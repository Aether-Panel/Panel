package daemon

import (
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/gin-gonic/gin"
)

// registerWorldMapRoutes adds the world map install and viewer routes
func registerWorldMapRoutes(l *gin.RouterGroup) {
	l.GET("/:serverId/map", middleware.ResolveServerNode, getWorldMap)
	l.DELETE("/:serverId/map", middleware.ResolveServerNode, removeWorldMap)
	l.OPTIONS("/:serverId/map", response.CreateOptions("GET", "DELETE"))
	l.POST("/:serverId/map/install", middleware.ResolveServerNode, installWorldMap)
	l.OPTIONS("/:serverId/map/install", response.CreateOptions("POST"))
	l.GET("/:serverId/map/view/*path", middleware.ResolveServerNode, viewWorldMap)
	l.HEAD("/:serverId/map/view/*path", middleware.ResolveServerNode, viewWorldMap)
}

// @Summary Get world map
// @Description Whether this server can run BlueMap or Dynmap, which one is installed and whether its web map answers
// @Success 200 {object} servers.WorldMapStatus
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/map [get]
// @Security OAuth2Application[server.view]
func getWorldMap(c *gin.Context) {
	server := getServerFromGin(c)
	c.JSON(http.StatusOK, server.WorldMapStatus())
}

// @Summary Install world map
// @Description Downloads BlueMap or Dynmap from Modrinth into plugins/ or mods/, after a restore point, and restarts the server if it is running. Runs in the background; the console shows each step.
// @Success 202 {object} nil
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/map/install [post]
// @Security OAuth2Application[server.files.edit]
func installWorldMap(c *gin.Context) {
	server := getServerFromGin(c)

	var body struct {
		Kind           string `json:"kind" binding:"required"`
		AcceptDownload bool   `json:"acceptDownload"`
	}
	if response.HandleError(c, c.ShouldBindJSON(&body), http.StatusBadRequest) {
		return
	}
	err := server.InstallWorldMap(strings.ToLower(strings.TrimSpace(body.Kind)), body.AcceptDownload)
	switch {
	case errors.Is(err, servers.ErrMapKind), errors.Is(err, servers.ErrMapEULA), errors.Is(err, servers.ErrMapUnsupported):
		response.HandleError(c, err, http.StatusBadRequest)
	case err != nil:
		response.HandleError(c, err, http.StatusConflict)
	default:
		c.Status(http.StatusAccepted)
	}
}

// @Summary Remove world map
// @Description Removes the map's jar; the rendered map stays on disk
// @Success 204 {object} nil
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /api/servers/{id}/map [delete]
// @Security OAuth2Application[server.files.edit]
func removeWorldMap(c *gin.Context) {
	server := getServerFromGin(c)
	err := server.RemoveWorldMap()
	switch {
	case errors.Is(err, servers.ErrMapNotInstalled):
		response.HandleError(c, err, http.StatusNotFound)
	case errors.Is(err, servers.ErrMapInstalling):
		response.HandleError(c, err, http.StatusConflict)
	case err != nil:
		response.HandleError(c, err, http.StatusInternalServerError)
	default:
		c.Status(http.StatusNoContent)
	}
}

// mapTransport talks to the maps' web servers, which run next to the node
var mapTransport = &http.Transport{
	DialContext:           (&net.Dialer{Timeout: 3 * time.Second}).DialContext,
	ResponseHeaderTimeout: 30 * time.Second,
	MaxIdleConnsPerHost:   16,
	IdleConnTimeout:       60 * time.Second,
	// the browser's encoding is passed along as is; the panel unpacks what it must rewrite
	DisableCompression: true,
}

// mapRequestHeaders are the only request headers passed to the map
var mapRequestHeaders = []string{"Accept", "Accept-Encoding", "Accept-Language", "Range", "If-None-Match", "If-Modified-Since"}

// @Summary View world map
// @Description Reverse proxy to the map's own web server. The panel serves it to the browser under a signed link and adds the security headers.
// @Param id path string true "Server ID"
// @Param path path string true "Path inside the map"
// @Tags Daemon Server
// @Router /daemon/server/{id}/map/view/{path} [get]
func viewWorldMap(c *gin.Context) {
	server := getServerFromGin(c)
	target := c.Param("path")
	if !validMapPath(target) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}
	address, err := server.WorldMapTarget(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	query := c.Request.URL.RawQuery

	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.Out.URL.Scheme = "http"
			r.Out.URL.Host = address
			r.Out.URL.Path = target
			r.Out.URL.RawPath = ""
			r.Out.URL.RawQuery = query
			r.Out.Host = address
			headers := make(http.Header)
			for _, name := range mapRequestHeaders {
				if value := r.In.Header.Get(name); value != "" {
					headers.Set(name, value)
				}
			}
			r.Out.Header = headers
		},
		Transport: mapTransport,
		ModifyResponse: func(res *http.Response) error {
			res.Header.Del("Set-Cookie")
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			server.ForgetWorldMapTarget()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"the map's web server does not answer"}`))
		},
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

// validMapPath accepts an absolute path inside the map without ".." steps
func validMapPath(p string) bool {
	if !strings.HasPrefix(p, "/") || len(p) > 2048 || strings.ContainsAny(p, "\\\x00") {
		return false
	}
	for _, segment := range strings.Split(p, "/") {
		if segment == ".." {
			return false
		}
	}
	return true
}
