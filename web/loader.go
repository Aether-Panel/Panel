package web

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/client/frontend/dist"
	"github.com/SkyPanel/SkyPanel/v3/config"
	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/logging"
	"github.com/SkyPanel/SkyPanel/v3/middleware"
	"github.com/SkyPanel/SkyPanel/v3/web/api"
	"github.com/SkyPanel/SkyPanel/v3/web/auth"
	"github.com/SkyPanel/SkyPanel/v3/web/daemon"
	"github.com/SkyPanel/SkyPanel/v3/web/oauth2"
	_ "github.com/SkyPanel/SkyPanel/v3/web/swagger"
	_ "github.com/alecthomas/template"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	_ "github.com/swaggo/swag"
)

var noHtmlRedirectOn404 = []string{"/api/", "/oauth2/", "/daemon/"}
var clientFiles fs.ReadFileFS

// RegisterRoutes Registers all routes
// @title SkyPanel API
// @version 3.0
// @description SkyPanel API interface for both the panel and daemon.
// @contact.name SkyPanel
// @contact.url https://SkyPanel.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @Accept json
// @Produce json
// @Produce json
// @securitydefinitions.oauth2.application OAuth2Application
// @tokenUrl /oauth2/token
// @scope.none No scope needed
// @scope.admin Administrator, full rights to all actions
// @scope.login Allows logging into the panel
// @scope.oauth2.auth Scope to validate another OAuth2 credential
// @scope.nodes.view Allows viewing nodes
// @scope.nodes.create Allows creating nodes
// @scope.nodes.delete Allows for deleting nodes
// @scope.nodes.edit Allows editing of node connection information
// @scope.nodes.deploy Allows getting the config of a node for deployment
// @scope.self.edit Allows editing of personal account
// @scope.self.clients Allows creating OAuth2 clients under the account
// @scope.server.admin Admin access to a server (full permissions)
// @scope.server.view Allows viewing a server
// @scope.server.create Allows creating servers
// @scope.server.delete Allows deleting servers
// @scope.server.definition.edit Allows editing a server's definition
// @scope.server.data.edit Allows editing the values of variables
// @scope.server.flags.edit Allows changing flags on the server
// @scope.server.name.edit Allows editing of a server name
// @scope.server.definition.view Allows viewing a server's definition
// @scope.server.data.view Allows viewing a server's variables
// @scope.server.flags.view Allows viewing a server's flags
// @scope.server.clients.view Allows viewing OAuth2 clients associated to a server
// @scope.server.clients.edit Allows editing OAuth2 clients associated to a server
// @scope.server.clients.create Allows adding a new OAuth2 client to a server
// @scope.server.clients.delete Allows deleting OAuth2 clients associated to a server
// @scope.server.users.view Allows viewing users associated to a server
// @scope.server.users.edit Allows editing user permissions to a server
// @scope.server.users.create Allows adding a new user to a server
// @scope.server.users.delete Allows removing users from to a server
// @scope.server.tasks.view Allows viewing tasks associated to a server
// @scope.server.tasks.edit Allows editing tasks associated to a server
// @scope.server.tasks.add Allows adding a new tasks to a server
// @scope.server.tasks.delete Allows deleting tasks from to a server
// @scope.server.tasks.run Allows for running tasks on a server
// @scope.server.reload Allows reloading of a server's definition from disk
// @scope.server.start Allow starting a server
// @scope.server.stop Allows stopping a server
// @scope.server.kill Allows killing a server
// @scope.server.install Allows using the "Install" button for a server
// @scope.server.files.view Allows viewing and downloading files for a server through the File Manager
// @scope.server.files.edit Allows editing files for a server through the File Manager
// @scope.server.sftp Allows connection to a server over SFTP
// @scope.server.console Allows viewing the console of a server
// @scope.server.console.send Allows sending commands to a server's console
// @scope.server.stats Allows getting stats of a server like CPU and memory usage
// @scope.server.status Allows getting the status of a server
// @scope.settings.edit Allows for editing of panel settings
// @scope.templates.view Allows viewing templates
// @scope.templates.local.edit Allows editing of templates in the local repo
// @scope.templates.repo.create Allows adding a new template repo
// @scope.templates.repo.delete Allows deleting of a template repo
// @scope.users.info.search Allows for searching for users
// @scope.users.info.view Allows for viewing a user's info
// @scope.users.info.edit Allows for editing a user's info
// @scope.users.perms.view Allows for viewing a user's global permissions
// @scope.users.perms.edit Allows for editing a user's global permissions
func RegisterRoutes(e *gin.Engine) {
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

		// Rutas para hacer proxy a Gatus en el puerto 8081
		// Siempre intentar hacer proxy primero, si falla, mostrar error
		gatusHandler := func(c *gin.Context) {
			// Intentar hacer proxy directamente
			// Si Gatus no está corriendo, el proxy fallará con un error claro
			gatusProxy(c)
		}

		gatusGroup := e.Group("/gatus")
		{
			gatusGroup.Any("", gatusHandler)
			gatusGroup.Any("/*path", gatusHandler)
		}
		uptimeGroup := e.Group("/uptime")
		{
			uptimeGroup.Any("", gatusHandler)
			uptimeGroup.Any("/*path", gatusHandler)
		}

		clientFiles = dist.ClientFiles
		if config.WebRoot.Value() != "" {
			clientFiles = files.NewMergedFS(os.DirFS(config.WebRoot.Value()), clientFiles)
		}

		css := e.Group("/css")
		{
			css.Use(gzip.Gzip(gzip.DefaultCompression))
			css.Use(setContentType("text/css"))
			f, err := fs.Sub(clientFiles, "css")
			if err != nil {
				panic(err)
			}
			css.StaticFS("", http.FS(f))
		}
		fonts := e.Group("/fonts")
		{
			fonts.Use(gzip.Gzip(gzip.DefaultCompression))
			f, err := fs.Sub(clientFiles, "fonts")
			if err != nil {
				panic(err)
			}
			fonts.StaticFS("", http.FS(f))
		}
		img := e.Group("/img")
		{
			f, err := fs.Sub(clientFiles, "img")
			if err != nil {
				panic(err)
			}
			img.StaticFS("", http.FS(f))
		}
		js := e.Group("/js")
		{
			js.Use(gzip.Gzip(gzip.DefaultCompression))
			js.Use(setContentType("application/javascript"))
			f, err := fs.Sub(clientFiles, "js")
			if err != nil {
				panic(err)
			}
			js.StaticFS("", http.FS(f))
		}
		wasm := e.Group("/wasm")
		{
			wasm.Use(gzip.Gzip(gzip.DefaultCompression))
			wasm.Use(setContentType("application/wasm"))
			f, err := fs.Sub(clientFiles, "wasm")
			if err != nil {
				panic(err)
			}
			wasm.StaticFS("", http.FS(f))
		}
		theme := e.Group("/theme")
		{
			theme.Use(setContentType("application/x-tar"))
			f, err := fs.Sub(clientFiles, "theme")
			if err != nil {
				panic(err)
			}
			theme.StaticFS("", http.FS(f))
		}

		// Para manifest.json, verificar si viene de Gatus antes de usar el de SkyPanel
		e.GET("/manifest.json", func(c *gin.Context) {
			referer := c.Request.Header.Get("Referer")
			if strings.Contains(referer, "/uptime/") || strings.Contains(referer, "/gatus/") {
				gatusProxy(c)
				return
			}
			webManifest(c)
		})

		// Archivos de la raíz que pueden ser de Gatus
		e.GET("/favicon-16x16.png", func(c *gin.Context) {
			referer := c.Request.Header.Get("Referer")
			if strings.Contains(referer, "/uptime/") || strings.Contains(referer, "/gatus/") {
				gatusProxy(c)
				return
			}
			c.AbortWithStatus(http.StatusNotFound)
		})
		e.GET("/favicon-32x32.png", func(c *gin.Context) {
			referer := c.Request.Header.Get("Referer")
			if strings.Contains(referer, "/uptime/") || strings.Contains(referer, "/gatus/") {
				gatusProxy(c)
				return
			}
			c.AbortWithStatus(http.StatusNotFound)
		})
		e.GET("/apple-touch-icon.png", func(c *gin.Context) {
			referer := c.Request.Header.Get("Referer")
			if strings.Contains(referer, "/uptime/") || strings.Contains(referer, "/gatus/") {
				gatusProxy(c)
				return
			}
			c.AbortWithStatus(http.StatusNotFound)
		})
		e.GET("/logo-192x192.png", func(c *gin.Context) {
			referer := c.Request.Header.Get("Referer")
			if strings.Contains(referer, "/uptime/") || strings.Contains(referer, "/gatus/") {
				gatusProxy(c)
				return
			}
			c.AbortWithStatus(http.StatusNotFound)
		})
		e.GET("/logo-512x512.png", func(c *gin.Context) {
			referer := c.Request.Header.Get("Referer")
			if strings.Contains(referer, "/uptime/") || strings.Contains(referer, "/gatus/") {
				gatusProxy(c)
				return
			}
			c.AbortWithStatus(http.StatusNotFound)
		})

		e.StaticFileFS("/favicon.png", "favicon.png", http.FS(clientFiles))
		e.StaticFileFS("/favicon.ico", "favicon.ico", http.FS(clientFiles))
		e.NoRoute(handle404)
	}
}

func handle404(c *gin.Context) {
	for _, v := range noHtmlRedirectOn404 {
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
	file, err := clientFiles.ReadFile("index.html")
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

func setContentType(contentType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", contentType)
	}
}

// redirectToGatus redirige las peticiones a Gatus en el puerto 8081
func redirectToGatus(c *gin.Context) {
	path := c.Request.URL.Path

	// Quitar prefijos /gatus o /uptime
	for _, prefix := range []string{"/gatus", "/uptime"} {
		if strings.HasPrefix(path, prefix) {
			path = strings.TrimPrefix(path, prefix)
			break
		}
	}

	// Construir la URL de destino
	targetURL := "http://127.0.0.1:8081" + path
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	logging.Info.Printf("Gatus redirect: redirecting %s %s to %s", c.Request.Method, c.Request.URL.Path, targetURL)
	c.Redirect(http.StatusTemporaryRedirect, targetURL)
}

// gatusProxy crea un proxy reverso para Gatus
func gatusProxy(c *gin.Context) {
	logging.Info.Printf("Gatus proxy: proxying request %s %s to Gatus", c.Request.Method, c.Request.URL.Path)
	// Crear URL objetivo (Gatus en puerto interno)
	targetURL, err := url.Parse("http://127.0.0.1:8081")
	if err != nil {
		logging.Error.Printf("Gatus proxy: failed to parse URL: %s", err.Error())
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse Gatus URL"})
		return
	}

	// Capturar valores del contexto antes de crear el Director
	clientHost := c.Request.Host
	clientIP := c.ClientIP()
	isTLS := c.Request.TLS != nil
	origin := c.Request.Header.Get("Origin")
	referer := c.Request.Header.Get("Referer")

	// Crear proxy reverso con director personalizado
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Guardar el director original
	originalDirector := proxy.Director

	// Configurar director personalizado
	proxy.Director = func(req *http.Request) {
		// Llamar al director original
		originalDirector(req)

		// Modificar la ruta para quitar el prefijo /gatus o /uptime
		// Si la ruta ya es /api/v1/*, no necesita modificación porque viene directa de Gatus
		path := req.URL.Path
		logging.Debug.Printf("Gatus proxy: original path: %s", path)
		if !strings.HasPrefix(path, "/api/v1") {
			// Solo quitar prefijos si no es una ruta de API de Gatus
			for _, prefix := range []string{"/gatus", "/uptime"} {
				if strings.HasPrefix(path, prefix) {
					path = strings.TrimPrefix(path, prefix)
					logging.Debug.Printf("Gatus proxy: removed prefix %s, new path: %s", prefix, path)
					break
				}
			}
		}
		req.URL.Path = path
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}

		// Actualizar Host header
		req.Host = targetURL.Host
		logging.Debug.Printf("Gatus proxy: final path: %s, host: %s", req.URL.Path, req.Host)

		// Actualizar headers de forwarding
		req.Header.Set("X-Forwarded-Host", clientHost)
		req.Header.Set("X-Forwarded-For", clientIP)
		if isTLS {
			req.Header.Set("X-Forwarded-Proto", "https")
		} else {
			req.Header.Set("X-Forwarded-Proto", "http")
		}

		// Mantener headers originales importantes
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if referer != "" {
			req.Header.Set("Referer", referer)
		}

		// Copiar headers importantes del request original
		for key, values := range c.Request.Header {
			keyLower := strings.ToLower(key)
			// Copiar headers importantes pero no modificar los que ya configuramos
			if keyLower != "host" &&
				keyLower != "x-forwarded-host" &&
				keyLower != "x-forwarded-for" &&
				keyLower != "x-forwarded-proto" {
				req.Header[key] = values
			}
		}
	}

	// Configurar ErrorHandler para manejar errores del proxy
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logging.Error.Printf("Gatus proxy error: %s", err.Error())
		// Si es un error de conexión, intentar redirigir
		if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "connect: connection refused") {
			path := r.URL.Path
			// Quitar prefijos /gatus o /uptime
			for _, prefix := range []string{"/gatus", "/uptime"} {
				if strings.HasPrefix(path, prefix) {
					path = strings.TrimPrefix(path, prefix)
					break
				}
			}
			if path == "" {
				path = "/"
			}
			targetURL := "http://localhost:8081" + path
			if r.URL.RawQuery != "" {
				targetURL += "?" + r.URL.RawQuery
			}
			logging.Info.Printf("Gatus not accessible via proxy, redirecting to %s", targetURL)
			http.Redirect(w, r, targetURL, http.StatusTemporaryRedirect)
			return
		}
		http.Error(w, "Error proxying to Gatus: "+err.Error(), http.StatusBadGateway)
	}

	// Modificar la respuesta para agregar headers CORS y asegurar que se sirvan correctamente
	proxy.ModifyResponse = func(resp *http.Response) error {
		// Agregar headers CORS
		resp.Header.Set("Access-Control-Allow-Origin", "*")
		resp.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, HEAD")
		resp.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		resp.Header.Set("Access-Control-Allow-Credentials", "true")
		resp.Header.Set("Access-Control-Max-Age", "86400")

		// Asegurar que los headers de contenido se mantengan
		contentType := resp.Header.Get("Content-Type")
		if contentType == "" && resp.StatusCode == 200 {
			// Intentar determinar el content type desde la ruta
			path := resp.Request.URL.Path
			if strings.HasPrefix(path, "/api/v1") {
				resp.Header.Set("Content-Type", "application/json")
			} else if strings.HasSuffix(path, ".html") || path == "/" {
				resp.Header.Set("Content-Type", "text/html; charset=utf-8")
			}
		}

		logging.Info.Printf("Gatus proxy: response status %d Content-Type: %s for %s %s",
			resp.StatusCode, resp.Header.Get("Content-Type"), resp.Request.Method, resp.Request.URL.Path)
		return nil
	}

	// Servir la petición
	proxy.ServeHTTP(c.Writer, c.Request)
}
