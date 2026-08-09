package user

import (
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/internal/domain"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

func registerUsers(g *gin.RouterGroup) {
	g.Handle("GET", "", middleware.RequiresPermission(scopes.ScopeUserInfoSearch), searchUsers)
	g.Handle("POST", "", middleware.RequiresPermission(scopes.ScopeUserInfoEdit), createUser)
	g.Handle("OPTIONS", "", response.CreateOptions("GET", "POST"))

	g.Handle("GET", "/:id", middleware.RequiresPermission(scopes.ScopeUserInfoView), getUser)
	g.Handle("POST", "/:id", middleware.RequiresPermission(scopes.ScopeUserInfoEdit), updateUser)
	g.Handle("DELETE", "/:id", middleware.RequiresPermission(scopes.ScopeUserInfoEdit), deleteUser)
	g.Handle("OPTIONS", "/:id", response.CreateOptions("GET", "POST", "DELETE"))

	g.Handle("GET", "/:id/perms", middleware.RequiresPermission(scopes.ScopeUserPermsView), getUserPerms)
	g.Handle("PUT", "/:id/perms", middleware.RequiresPermission(scopes.ScopeUserPermsEdit), setUserPerms)
	g.Handle("OPTIONS", "/:id/perms", response.CreateOptions("PUT", "GET"))
}

// @Summary Get users
// @Description Gets users, and allowing for filtering of users. * is a wildcard that can be used for text inputs
// @Success 200 {object} domain.UserSearchResponse
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param body body domain.UserSearch true "Filters to search on"
// @Tags Users
// @Router /api/users [get]
// @Security OAuth2Application[users.info.search]
func searchUsers(c *gin.Context) {
	var err error
	db := middleware.GetDatabase(c)
	us := &user.UserRepo{DB: db}

	search := newUserSearch()
	err = c.ShouldBind(search)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	if search.PageLimit > MaxPageSize {
		search.PageLimit = MaxPageSize
	}

	var results []*domain.User
	var total int64
	if results, total, err = us.Search(search.Username, search.Email, search.PageLimit, search.Page); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, &domain.UserSearchResponse{
		Users: domain.FromUsers(results),
		Metadata: &skypanel.Metadata{Paging: &skypanel.Paging{
			Page:    search.Page,
			Size:    search.PageLimit,
			MaxSize: MaxPageSize,
			Total:   total,
		}},
	})
}

// @Summary Create user
// @Success 200 {object} domain.UserView
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param body body domain.UserView true "New user information"
// @Tags Users
// @Router /api/users [post]
// @Security OAuth2Application[users.info.edit]
func createUser(c *gin.Context) {
	var err error
	db := middleware.GetDatabase(c)
	us := &user.UserRepo{DB: db}

	var viewModel domain.UserView
	if err = c.ShouldBindJSON(&viewModel); err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}

	if err = viewModel.Valid(false); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if viewModel.Password == "" {
		response.HandleError(c, skypanel.ErrFieldRequired("password"), http.StatusBadRequest)
		return
	}

	user := &domain.User{}
	viewModel.CopyToModel(user)

	if err = us.Create(user); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	resultModel := domain.FromUser(user)

	c.JSON(http.StatusOK, resultModel)
}

// @Summary Get a user
// @Success 200 {object} domain.UserView
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param id path uint true "User ID"
// @Tags Users
// @Router /api/users/{id} [get]
// @Security OAuth2Application[users.info.view]
func getUser(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &user.UserRepo{DB: db}

	var err error
	var id uint
	if id, err = cast.ToUintE(c.Param("id")); err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}

	user, err := us.GetByID(id)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, domain.FromUser(user))
}

// @Summary Update user
// @Success 204 {object} nil
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param id path uint true "User ID"
// @Param body body domain.UserView true "New user information"
// @Tags Users
// @Router /api/users/{id} [post]
// @Security OAuth2Application[users.info.edit]
func updateUser(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &user.UserRepo{DB: db}

	var err error
	var id uint
	if id, err = cast.ToUintE(c.Param("id")); err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}

	var viewModel domain.UserView
	if err := c.ShouldBindJSON(&viewModel); err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}

	if err := viewModel.Valid(true); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := us.GetByID(id)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	viewModel.CopyToModel(user)

	if err = us.Update(user); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Delete user
// @Success 204 {object} nil
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param id path uint true "User ID"
// @Tags Users
// @Router /api/users/{id} [delete]
// @Security OAuth2Application[users.info.edit]
func deleteUser(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &user.UserRepo{DB: db}

	var err error
	var id uint
	if id, err = cast.ToUintE(c.Param("id")); err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}

	user, err := us.GetByID(id)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	if err = us.Delete(user); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Gets user permissions
// @Success 200 {object} domain.PermissionView
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param id path uint true "User ID"
// @Tags Users
// @Router /api/users/{id}/perms [get]
// @Security OAuth2Application[users.perms.view]
func getUserPerms(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &user.UserRepo{DB: db}
	ps := &permission.PermissionRepo{DB: db}

	var err error
	var id uint
	if id, err = cast.ToUintE(c.Param("id")); err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}

	user, err := us.GetByID(id)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	perms, err := ps.GetForUserAndServer(user.ID, "")
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, domain.FromPermission(perms))
}

// @Summary Sets user permissions
// @Success 204 {object} nil
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param id path uint true "User ID"
// @Param body body domain.PermissionView true "New permissions"
// @Tags Users
// @Router /api/users/{id}/perms [put]
// @Security OAuth2Application[users.perms.edit]
func setUserPerms(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &user.UserRepo{DB: db}
	ps := &permission.PermissionRepo{DB: db}

	var err error
	var id uint
	if id, err = cast.ToUintE(c.Param("id")); err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}

	viewModel := &domain.PermissionView{}
	err = c.BindJSON(viewModel)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	user, err := us.GetByID(id)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	perms, err := ps.GetForUserAndServer(user.ID, "")
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// get the current user's scopes
	editorUser := c.MustGet("user").(*domain.User)
	editorPerms, err := ps.GetForUserAndServer(editorUser.ID, "")
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// admins can override, so skip our comparers
	if scopes.ContainsScope(editorPerms.Scopes, scopes.ScopeAdmin) {
		perms.Scopes = viewModel.Scopes
	} else {
		allowedScopes := utils.Union(viewModel.Scopes, editorPerms.Scopes)
		// update perms to match this "setup", but not stomp over what the user can't change
		replacement := scopes.UpdateScopesWhereGranted(perms.Scopes, allowedScopes, editorPerms.Scopes)
		perms.Scopes = replacement
	}

	err = ps.UpdatePermissions(perms)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.Status(http.StatusNoContent)
}

func newUserSearch() *domain.UserSearch {
	return &domain.UserSearch{
		Username:  "*",
		Email:     "*",
		PageLimit: DefaultPageSize,
		Page:      1,
	}
}
