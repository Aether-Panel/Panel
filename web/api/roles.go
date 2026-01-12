package api

import (
	"github.com/gin-gonic/gin"
	"github.com/SkyPanel/SkyPanel/v3/middleware"
	"github.com/SkyPanel/SkyPanel/v3/models"
	"github.com/SkyPanel/SkyPanel/v3/response"
	"github.com/SkyPanel/SkyPanel/v3/scopes"
	"github.com/SkyPanel/SkyPanel/v3/services"
	"github.com/spf13/cast"
	"net/http"
)

func registerRoles(g *gin.RouterGroup) {
	g.Handle("GET", "", middleware.RequiresPermission(scopes.ScopeAdmin), listRoles)
	g.Handle("POST", "", middleware.RequiresPermission(scopes.ScopeAdmin), createRole)
	g.Handle("OPTIONS", "", response.CreateOptions("GET", "POST"))

	g.Handle("GET", "/:id", middleware.RequiresPermission(scopes.ScopeAdmin), getRole)
	g.Handle("POST", "/:id", middleware.RequiresPermission(scopes.ScopeAdmin), updateRole)
	g.Handle("DELETE", "/:id", middleware.RequiresPermission(scopes.ScopeAdmin), deleteRole)
	g.Handle("OPTIONS", "/:id", response.CreateOptions("GET", "POST", "DELETE"))
}

// @Summary List roles
// @Success 200 {array} models.Role
// @Failure 403 {object} SkyPanel.ErrorResponse
// @Failure 500 {object} SkyPanel.ErrorResponse
// @Router /api/roles [get]
// @Security OAuth2Application[admin]
func listRoles(c *gin.Context) {
	db := middleware.GetDatabase(c)
	rs := &services.Role{DB: db}

	roles, err := rs.List()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, roles)
}

// @Summary Create role
// @Success 200 {object} models.Role
// @Failure 400 {object} SkyPanel.ErrorResponse
// @Failure 403 {object} SkyPanel.ErrorResponse
// @Failure 500 {object} SkyPanel.ErrorResponse
// @Param body body models.Role true "New role information"
// @Router /api/roles [post]
// @Security OAuth2Application[admin]
func createRole(c *gin.Context) {
	db := middleware.GetDatabase(c)
	rs := &services.Role{DB: db}

	var role models.Role
	if err := c.BindJSON(&role); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	if err := rs.Create(&role); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, role)
}

// @Summary Get role
// @Success 200 {object} models.Role
// @Failure 400 {object} SkyPanel.ErrorResponse
// @Failure 403 {object} SkyPanel.ErrorResponse
// @Failure 404 {object} SkyPanel.ErrorResponse
// @Failure 500 {object} SkyPanel.ErrorResponse
// @Param id path uint true "Role ID"
// @Router /api/roles/{id} [get]
// @Security OAuth2Application[admin]
func getRole(c *gin.Context) {
	db := middleware.GetDatabase(c)
	rs := &services.Role{DB: db}

	var err error
	var id uint
	if id, err = cast.ToUintE(c.Param("id")); err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}

	role, err := rs.Get(id)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, role)
}

// @Summary Update role
// @Success 200 {object} models.Role
// @Failure 400 {object} SkyPanel.ErrorResponse
// @Failure 403 {object} SkyPanel.ErrorResponse
// @Failure 404 {object} SkyPanel.ErrorResponse
// @Failure 500 {object} SkyPanel.ErrorResponse
// @Param id path uint true "Role ID"
// @Param body body models.Role true "Updated role information"
// @Router /api/roles/{id} [post]
// @Security OAuth2Application[admin]
func updateRole(c *gin.Context) {
	db := middleware.GetDatabase(c)
	rs := &services.Role{DB: db}

	var err error
	var id uint
	if id, err = cast.ToUintE(c.Param("id")); err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}

	var role models.Role
	if err := c.BindJSON(&role); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	role.ID = id
	if err := rs.Update(&role); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, role)
}

// @Summary Delete role
// @Success 204 {object} nil
// @Failure 400 {object} SkyPanel.ErrorResponse
// @Failure 403 {object} SkyPanel.ErrorResponse
// @Failure 404 {object} SkyPanel.ErrorResponse
// @Failure 500 {object} SkyPanel.ErrorResponse
// @Param id path uint true "Role ID"
// @Router /api/roles/{id} [delete]
// @Security OAuth2Application[admin]
func deleteRole(c *gin.Context) {
	db := middleware.GetDatabase(c)
	rs := &services.Role{DB: db}

	var err error
	var id uint
	if id, err = cast.ToUintE(c.Param("id")); err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}

	if err := rs.Delete(id); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.Status(http.StatusNoContent)
}

