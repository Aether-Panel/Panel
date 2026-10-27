package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
)

func registerAPIKeys(g *gin.RouterGroup) {
	g.Handle("GET", "", middleware.RequiresPermission(scopes.ScopeAdmin), getAPIKeys)
	g.Handle("POST", "", middleware.RequiresPermission(scopes.ScopeAdmin), createAPIKey)
	g.Handle("OPTIONS", "", response.CreateOptions("GET", "POST"))

	g.Handle("DELETE", "/:id", middleware.RequiresPermission(scopes.ScopeAdmin), deleteAPIKey)
	g.Handle("OPTIONS", "/:id", response.CreateOptions("DELETE"))
}

func getAPIKeys(c *gin.Context) {
	db := middleware.GetDatabase(c)
	ss := &services.APIKeyService{DB: db}

	keys, err := ss.GetAll()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, keys)
}

func createAPIKey(c *gin.Context) {
	db := middleware.GetDatabase(c)
	ss := &services.APIKeyService{DB: db}

	var req struct {
		Name        string   `json:"name" binding:"required"`
		Permissions []string `json:"permissions"`
	}

	if err := c.ShouldBindJSON(&req); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	permissions, err := normalizeAPIKeyPermissions(req.Permissions)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	token, apiKey, err := ss.GenerateKey(req.Name, permissions)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// Devuelve el token sólo esta vez
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"key":   apiKey,
	})
}

func deleteAPIKey(c *gin.Context) {
	db := middleware.GetDatabase(c)
	ss := &services.APIKeyService{DB: db}

	id, err := strconv.Atoi(c.Param("id"))
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	err = ss.Delete(uint(id))
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.Status(http.StatusNoContent)
}

// normalizeAPIKeyPermissions keeps the known actions, once each. A key created without a list
// gets every action, as keys always did ("provision"), so existing callers keep working; a list
// that names nothing known is refused rather than turned into an all-powerful key.
func normalizeAPIKeyPermissions(requested []string) ([]string, error) {
	if len(requested) == 0 {
		return []string{models.APIKeyAll}, nil
	}
	known := map[string]bool{models.APIKeyAll: true}
	for _, a := range models.APIKeyActions {
		known[a] = true
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(requested))
	for _, p := range requested {
		p = strings.TrimSpace(p)
		if !known[p] || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, skypanel.ErrAPIKeyPermissions
	}
	return out, nil
}
