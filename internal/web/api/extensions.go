package api

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/extensions"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// extensionPageSandbox is the Content-Security-Policy of every extension file: its scripts
// run in an opaque origin, so a page cannot read the panel's cookies or call its API with
// the viewer's session, whoever wrote it
const extensionPageSandbox = "sandbox allow-scripts allow-forms allow-popups allow-modals allow-downloads"

const extensionFilesPath = "/api/extension-files/"

func registerExtensions(g *gin.RouterGroup) {
	g.Handle("GET", "", middleware.RequiresPermission(scopes.ScopeAdmin), getExtensions)
	g.Handle("POST", "", middleware.RequiresPermission(scopes.ScopeAdmin), installExtension)
	g.Handle("OPTIONS", "", response.CreateOptions("GET", "POST"))

	g.Handle("GET", "/active", middleware.RequiresPermission(scopes.ScopeLogin), getActiveExtensions)
	g.Handle("OPTIONS", "/active", response.CreateOptions("GET"))

	g.Handle("PUT", "/:id", middleware.RequiresPermission(scopes.ScopeAdmin), updateExtension)
	g.Handle("DELETE", "/:id", middleware.RequiresPermission(scopes.ScopeAdmin), uninstallExtension)
	g.Handle("OPTIONS", "/:id", response.CreateOptions("PUT", "DELETE"))

	g.Handle("POST", "/:id/test", middleware.RequiresPermission(scopes.ScopeAdmin), testExtension)
	g.Handle("OPTIONS", "/:id/test", response.CreateOptions("POST"))
}

// registerExtensionFiles serves the pages' files. They are public: a sandboxed frame does not
// send the session, and extension pages hold no secrets of the panel.
func registerExtensionFiles(g *gin.RouterGroup) {
	g.GET("/:id/*path", serveExtensionFile)
	g.HEAD("/:id/*path", serveExtensionFile)
}

// ExtensionView is an installed extension as administrators see it
type ExtensionView struct {
	models.Extension
	Pages  []ExtensionPageView `json:"pages"`
	Events []string            `json:"events"`
	// ManifestWebhook is the address the package asked for, to offer going back to it
	ManifestWebhook string `json:"manifestWebhook"`
} // @name Extension

// ExtensionPageView is a page with the address its frame loads
type ExtensionPageView struct {
	extensions.Page
	URL string `json:"url"`
} // @name ExtensionPage

// ActiveExtension is a switched-on extension's pages, for the menu and the server tabs
type ActiveExtension struct {
	ID    string              `json:"id"`
	Name  string              `json:"name"`
	Pages []ExtensionPageView `json:"pages"`
} // @name ActiveExtension

func pageViews(ext *models.Extension, m *extensions.Manifest, include func(extensions.Page) bool) []ExtensionPageView {
	pages := make([]ExtensionPageView, 0, len(m.Pages))
	for _, p := range m.Pages {
		if include(p) {
			pages = append(pages, ExtensionPageView{Page: p, URL: extensionFilesPath + ext.ID + "/" + p.Path})
		}
	}
	return pages
}

// @Summary List extensions
// @Description Every installed extension, with its pages, events and webhook secret
// @Success 200 {array} ExtensionView
// @Failure 403 {object} skypanel.ErrorResponse
// @Tags Extensions
// @Router /api/extensions [get]
// @Security OAuth2Application[admin]
func getExtensions(c *gin.Context) {
	db := middleware.GetDatabase(c)
	var list []models.Extension
	if err := db.Order("name").Find(&list).Error; response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	views := make([]ExtensionView, 0, len(list))
	for i := range list {
		m, err := extensions.ManifestOf(&list[i])
		if err != nil {
			m = &extensions.Manifest{}
		}
		events := m.Events
		if events == nil {
			events = []string{}
		}
		views = append(views, ExtensionView{
			Extension:       list[i],
			Pages:           pageViews(&list[i], m, func(extensions.Page) bool { return true }),
			Events:          events,
			ManifestWebhook: m.Webhook,
		})
	}
	c.JSON(http.StatusOK, views)
}

// @Summary Pages of the switched-on extensions
// @Description For the menu and the server tabs. Administration pages are only listed for administrators.
// @Success 200 {array} ActiveExtension
// @Tags Extensions
// @Router /api/extensions/active [get]
// @Security OAuth2Application[login]
func getActiveExtensions(c *gin.Context) {
	db := middleware.GetDatabase(c)
	var list []models.Extension
	if err := db.Where("enabled = ?", true).Order("name").Find(&list).Error; response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	isAdmin := callerIsAdmin(c)
	active := make([]ActiveExtension, 0, len(list))
	for i := range list {
		m, err := extensions.ManifestOf(&list[i])
		if err != nil {
			continue
		}
		pages := pageViews(&list[i], m, func(p extensions.Page) bool {
			return p.Placement == extensions.PlacementServer || isAdmin
		})
		if len(pages) > 0 {
			active = append(active, ActiveExtension{ID: list[i].ID, Name: list[i].Name, Pages: pages})
		}
	}
	c.JSON(http.StatusOK, active)
}

// @Summary Install or update an extension
// @Description Upload the extension's zip as the "file" form field. Installing an id that is
// @Description already there updates it; a new extension starts switched off.
// @Accept multipart/form-data
// @Success 200 {object} models.Extension
// @Failure 400 {object} skypanel.ErrorResponse
// @Tags Extensions
// @Router /api/extensions [post]
// @Security OAuth2Application[admin]
func installExtension(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, extensions.MaxPackageSize+1<<20)
	file, _, err := c.Request.FormFile("file")
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, extensions.MaxPackageSize+1))
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	ext, err := extensions.Install(middleware.GetDatabase(c), bytes.NewReader(data), int64(len(data)))
	if err != nil {
		// what is wrong with the package is for the administrator to read
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "ErrExtensionInvalid", "msg": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, ext)
}

// UpdateExtensionRequest switches an extension or changes where its events go
type UpdateExtensionRequest struct {
	Enabled    *bool   `json:"enabled"`
	WebhookURL *string `json:"webhookUrl"`
}

// @Summary Switch an extension on or off, or change its webhook
// @Param id path string true "Extension id"
// @Param body body UpdateExtensionRequest true "Fields to change"
// @Success 200 {object} models.Extension
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Tags Extensions
// @Router /api/extensions/{id} [put]
// @Security OAuth2Application[admin]
func updateExtension(c *gin.Context) {
	db := middleware.GetDatabase(c)
	var req UpdateExtensionRequest
	if err := c.ShouldBindJSON(&req); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	var ext models.Extension
	if err := db.First(&ext, "id = ?", c.Param("id")).Error; response.HandleError(c, err, http.StatusNotFound) {
		return
	}
	if req.WebhookURL != nil {
		webhook := strings.TrimSpace(*req.WebhookURL)
		if err := services.ValidateWebhook(webhook); response.HandleError(c, err, http.StatusBadRequest) {
			return
		}
		ext.WebhookURL = webhook
	}
	if req.Enabled != nil {
		ext.Enabled = *req.Enabled
	}
	if err := db.Save(&ext).Error; response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	extensions.Reload(db)
	c.JSON(http.StatusOK, ext)
}

// @Summary Uninstall an extension
// @Param id path string true "Extension id"
// @Success 204 {object} nil
// @Failure 404 {object} skypanel.ErrorResponse
// @Tags Extensions
// @Router /api/extensions/{id} [delete]
// @Security OAuth2Application[admin]
func uninstallExtension(c *gin.Context) {
	err := extensions.Uninstall(middleware.GetDatabase(c), c.Param("id"))
	if errors.Is(err, extensions.ErrNotFound) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Send a test event
// @Description Posts a signed "test" event to the extension's webhook and reports whether it was accepted
// @Param id path string true "Extension id"
// @Success 204 {object} nil
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Tags Extensions
// @Router /api/extensions/{id}/test [post]
// @Security OAuth2Application[admin]
func testExtension(c *gin.Context) {
	db := middleware.GetDatabase(c)
	var ext models.Extension
	if err := db.First(&ext, "id = ?", c.Param("id")).Error; response.HandleError(c, err, http.StatusNotFound) {
		return
	}
	if ext.WebhookURL == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "ErrExtensionNoWebhook", "msg": "this extension has no webhook"}})
		return
	}
	if err := extensions.SendTest(&ext); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "ErrExtensionWebhook", "msg": err.Error()}})
		return
	}
	c.Status(http.StatusNoContent)
}

// serveExtensionFile sends one file of a switched-on extension, always sandboxed
func serveExtensionFile(c *gin.Context) {
	db := middleware.GetDatabase(c)
	id := c.Param("id")
	name := strings.TrimPrefix(c.Param("path"), "/")
	if name == "" || path.Clean(name) != name || strings.HasPrefix(name, "..") || strings.Contains(name, "\\") {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	var ext models.Extension
	err := db.Select("id", "enabled").First(&ext, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && !ext.Enabled) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	root, err := os.OpenRoot(extensions.Folder(ext.ID))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	defer root.Close()
	f, err := root.Open(filepath.FromSlash(name))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.Header("Content-Security-Policy", extensionPageSandbox)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Referrer-Policy", "no-referrer")
	// the panel's own pages may frame it; nobody else
	c.Header("X-Frame-Options", "SAMEORIGIN")
	c.Header("Cache-Control", "no-cache")
	contentType := mime.TypeByExtension(path.Ext(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	http.ServeContent(c.Writer, c.Request, info.Name(), info.ModTime(), f)
}
