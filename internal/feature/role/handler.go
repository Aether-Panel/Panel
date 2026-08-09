package role

import (
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/internal/domain"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/scopes"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

func registerRoles(g *gin.RouterGroup) {
	g.Handle("GET", "", middleware.RequiresAnyPermission(scopes.ScopeAdmin, scopes.ScopeUserInfoView, scopes.ScopeUserInfoEdit), listRoles)
	g.Handle("POST", "", middleware.RequiresPermission(scopes.ScopeAdmin), createRole)
	g.Handle("OPTIONS", "", response.CreateOptions("GET", "POST"))

	g.Handle("GET", "/:id", middleware.RequiresPermission(scopes.ScopeAdmin), getRole)
	g.Handle("POST", "/:id", middleware.RequiresPermission(scopes.ScopeAdmin), updateRole)
	g.Handle("DELETE", "/:id", middleware.RequiresPermission(scopes.ScopeAdmin), deleteRole)
	g.Handle("OPTIONS", "/:id", response.CreateOptions("GET", "POST", "DELETE"))
}

// @Summary List roles
// @Success 200 {array} domain.Role
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Tags Roles
// @Router /api/roles [get]
// @Security OAuth2Application[admin]
func listRoles(c *gin.Context) {
	db := middleware.GetDatabase(c)
	rs := &role.RoleRepo{DB: db}

	roles, err := rs.List()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, roles)
}

// @Summary Create role
// @Success 200 {object} domain.Role
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param body body domain.Role true "New role information"
// @Tags Roles
// @Router /api/roles [post]
// @Security OAuth2Application[admin]
func createRole(c *gin.Context) {
	db := middleware.GetDatabase(c)
	rs := &role.RoleRepo{DB: db}

	var role domain.Role
	if err := c.BindJSON(&role); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	if err := rs.Create(&role); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, role)
}

// @Summary Get role
// @Success 200 {object} domain.Role
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param id path uint true "Role ID"
// @Tags Roles
// @Router /api/roles/{id} [get]
// @Security OAuth2Application[admin]
func getRole(c *gin.Context) {
	db := middleware.GetDatabase(c)
	rs := &role.RoleRepo{DB: db}

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
// @Success 200 {object} domain.Role
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param id path uint true "Role ID"
// @Param body body domain.Role true "Updated role information"
// @Tags Roles
// @Router /api/roles/{id} [post]
// @Security OAuth2Application[admin]
func updateRole(c *gin.Context) {
	db := middleware.GetDatabase(c)
	rs := &role.RoleRepo{DB: db}

	var err error
	var id uint
	if id, err = cast.ToUintE(c.Param("id")); err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}

	var role domain.Role
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
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param id path uint true "Role ID"
// @Tags Roles
// @Router /api/roles/{id} [delete]
// @Security OAuth2Application[admin]
func deleteRole(c *gin.Context) {
	db := middleware.GetDatabase(c)
	rs := &role.RoleRepo{DB: db}

	var err error
	var id uint
	if id, err = cast.ToUintE(c.Param("id")); err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}

	if err := rs.Delete(id); err != nil {
		switch {
		case err.Error() == "cannot delete the admin role" || err.Error() == "cannot delete a default role":
			response.HandleError(c, err, http.StatusBadRequest)
		case gorm.ErrRecordNotFound == err:
			c.AbortWithStatus(http.StatusNotFound)
		default:
			response.HandleError(c, err, http.StatusInternalServerError)
		}
		return
	}

	c.Status(http.StatusNoContent)
}
