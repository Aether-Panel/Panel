package web

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/client/frontend"
	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/web/api"
	"github.com/SkyPanel/SkyPanel/v3/internal/web/auth"
	"github.com/SkyPanel/SkyPanel/v3/internal/web/daemon"
	"github.com/SkyPanel/SkyPanel/v3/internal/web/oauth2"
	_ "github.com/SkyPanel/SkyPanel/v3/internal/web/swagger" // swagger docs init side effect
	_ "github.com/alecthomas/template"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	_ "github.com/swaggo/swag" // swagger docs init side effect
)

var noHTMLRedirectOn404 = []string{"/api/", "/oauth2/", "/daemon/"}
var clientFiles fs.FS

// RegisterRoutes Registers all routes
func RegisterRoutes(e *gin.Engine) {
	// Configuración de CORS Global
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOriginFunc = func(_ string) bool {
		return true
	}
	corsConfig.AllowCredentials = true
	corsConfig.AddAllowHeaders("Authorization", "Content-Type", "Accept", "Origin")
	corsConfig.AddAllowMethods("GET", "POST", "PUT", "DELETE", "OPTIONS")
	e.Use(cors.New(corsConfig))

	e.Use(func(c *gin.Context) {
		middleware.Recover(c)
	})

	e.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.DefaultModelsExpandDepth(0), ginSwagger.DeepLinking(false)))

	if config.DaemonEnabled.Value() {
		daemon.RegisterDaemonRoutes(e.Group("/daemon"))
	}

	if config.PanelEnabled.Value() {

		api.RegisterRoutes(e.Group("/api"))
		oauth2.RegisterRoutes(e.Group("/oauth2"))
		auth.RegisterRoutes(e.Group("/auth"))

		sub, err := fs.Sub(frontend.ClientFiles, "dist")
		if err != nil {
			panic(err)
		}
		clientFiles = sub

		if config.WebRoot.Value() != "" {
			clientFiles = files.NewMergedFS(os.DirFS(config.WebRoot.Value()), clientFiles)
		}

		// Generic static file serving (Optimized for Astro)
		e.Use(func(c *gin.Context) {
			reqPath := c.Request.URL.Path

			// Skip for API, OAuth2, and Daemon routes
			for _, prefix := range noHTMLRedirectOn404 {
				if strings.HasPrefix(reqPath, prefix) {
					return
				}
			}

			// Use path.Clean to prevent path traversal attacks
			cleanedPath := path.Clean(reqPath)

			// Clean path for fs.FS (no leading slash, "." for root)
			fPath := strings.TrimPrefix(cleanedPath, "/")
			if fPath == "" || fPath == "." {
				fPath = "."
			} else {
				// fs.FS.Open doesn't like trailing slashes for directory names
				fPath = strings.TrimSuffix(fPath, "/")
			}

			// Try to open the path in our embedded FS
			f, err := clientFiles.Open(fPath)
			if err != nil {
				// If not found, let it fall through to handle404 (SPA)
				return
			}
			defer f.Close()

			stat, err := f.Stat()
			if err != nil {
				return
			}

			if stat.IsDir() {
				// If it's a directory, we REQUIRE a trailing slash in the URL
				if !strings.HasSuffix(reqPath, "/") {
					c.Redirect(http.StatusMovedPermanently, reqPath+"/")
					c.Abort()
					return
				}

				// Look for index.html inside the directory
				indexPath := fPath
				if indexPath == "." {
					indexPath = "index.html"
				} else {
					indexPath += "/index.html"
				}

				// Verify index.html exists
				if ifile, ierr := clientFiles.Open(indexPath); ierr == nil {
					_ = ifile.Close()
					// Serve the index.html content directly to bypass Gin's smart redirects
					content, rerr := fs.ReadFile(clientFiles, indexPath)
					if rerr == nil {
						c.Data(http.StatusOK, "text/html; charset=utf-8", content)
						c.Abort()
						return
					}
				}
				// If directory but no index.html, fall through
				return
			}

			// If it's a regular file, serve it normally
			c.FileFromFS(fPath, http.FS(clientFiles))
			c.Abort()
		})

		// Favicon principal: devolver un SVG embebido si no existe archivo físico
		e.GET("/favicon.ico", func(c *gin.Context) {
			// Intentar servir desde los archivos del cliente si existe
			f, err := clientFiles.Open("favicon.ico")
			if err == nil {
				_ = f.Close()
				c.FileFromFS("favicon.ico", http.FS(clientFiles))
				return
			}

			// Fallback: pequeño favicon SVG integrado para evitar 404
			c.Header("Content-Type", "image/svg+xml")
			c.String(http.StatusOK, `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">
  <defs>
    <linearGradient id="g" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#4f46e5"/>
      <stop offset="100%" stop-color="#22c55e"/>
    </linearGradient>
  </defs>
  <rect x="4" y="4" width="56" height="56" rx="14" fill="url(#g)"/>
  <path d="M20 40L28 24L36 40L44 24" fill="none" stroke="#ffffff" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"/>
</svg>`)
		})

		// Para manifest.json
		e.GET("/manifest.json", func(c *gin.Context) {
			webManifest(c)
		})

		e.StaticFileFS("/favicon.png", "favicon.png", http.FS(clientFiles))
		e.NoRoute(handle404)
	}
}

func handle404(c *gin.Context) {
	for _, v := range noHTMLRedirectOn404 {
		if strings.HasPrefix(c.Request.URL.Path, v) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
	}

	if c.Request.Method == http.MethodConnect {
		c.AbortWithStatus(http.StatusMethodNotAllowed)
		return
	}

	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	file, err := fs.ReadFile(clientFiles, "index.html")
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Data(http.StatusOK, binding.MIMEHTML, file)
}

func webManifest(c *gin.Context) {
	iconSizes := []int{72, 96, 128, 144, 152, 192, 384, 512}
	icons := make([]map[string]interface{}, len(iconSizes))

	for i, s := range iconSizes {
		icons[i] = map[string]interface{}{
			"src":   fmt.Sprintf("img/appicons/%d.png", s),
			"sizes": fmt.Sprintf("%dx%d", s, s),
			"type":  "image/png",
		}
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"name":             config.CompanyName.Value(),
		"short_name":       config.CompanyName.Value(),
		"background_color": "#fff",
		"display":          "standalone",
		"scope":            "/",
		"start_url":        "/servers",
		"icons":            icons,
	})
}
