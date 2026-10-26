package api

import (
	"errors"
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"gorm.io/gorm"
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
// @Success 200 {object} models.UserSearchResponse
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param body body models.UserSearch true "Filters to search on"
// @Tags Users
// @Router /api/users [get]
// @Security OAuth2Application[users.info.search]
func searchUsers(c *gin.Context) {
	var err error
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}

	search := newUserSearch()
	err = c.ShouldBind(search)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	if search.PageLimit > MaxPageSize {
		search.PageLimit = MaxPageSize
	}

	var results []*models.User
	var total int64
	if results, total, err = us.Search(search.Username, search.Email, search.PageLimit, search.Page); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, &models.UserSearchResponse{
		Users: models.FromUsers(results),
		Metadata: &skypanel.Metadata{Paging: &skypanel.Paging{
			Page:    search.Page,
			Size:    search.PageLimit,
			MaxSize: MaxPageSize,
			Total:   total,
		}},
	})
}

// @Summary Create user
// @Success 200 {object} models.UserView
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param body body models.UserView true "New user information"
// @Tags Users
// @Router /api/users [post]
// @Security OAuth2Application[users.info.edit]
func createUser(c *gin.Context) {
	var err error
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}

	var viewModel models.UserView
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

	user := &models.User{}
	viewModel.CopyToModel(user)
	if callerIsAdmin(c) {
		viewModel.CopyRoleToModel(user)
	}

	if err = us.Create(user); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	middleware.RecordActivity(c, models.ActivityRegister, user)

	resultModel := models.FromUser(user)

	c.JSON(http.StatusOK, resultModel)
}

// @Summary Get a user
// @Success 200 {object} models.UserView
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
	us := &services.User{DB: db}

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

	c.JSON(http.StatusOK, models.FromUser(user))
}

// @Summary Update user
// @Success 204 {object} nil
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param id path uint true "User ID"
// @Param body body models.UserView true "New user information"
// @Tags Users
// @Router /api/users/{id} [post]
// @Security OAuth2Application[users.info.edit]
func updateUser(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}

	var err error
	var id uint
	if id, err = cast.ToUintE(c.Param("id")); err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}

	var viewModel models.UserView
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

	isAdmin := callerIsAdmin(c)
	if !isAdmin && denyEditingAdmin(c, db, user) {
		return
	}

	viewModel.CopyToModel(user)
	if isAdmin {
		viewModel.CopyRoleToModel(user)
	}

	if err = us.Update(user); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	if viewModel.Password != "" {
		ss := &services.Session{DB: db}
		if err = ss.ExpireAllForUser(user.ID, ""); response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
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
	us := &services.User{DB: db}

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

	if !callerIsAdmin(c) && denyEditingAdmin(c, db, user) {
		return
	}

	if err = us.Delete(user); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Gets user permissions
// @Success 200 {object} models.PermissionView
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
	us := &services.User{DB: db}
	ps := &services.Permission{DB: db}

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

	c.JSON(http.StatusOK, models.FromPermission(perms))
}

// @Summary Sets user permissions
// @Success 204 {object} nil
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param id path uint true "User ID"
// @Param body body models.PermissionView true "New permissions"
// @Tags Users
// @Router /api/users/{id}/perms [put]
// @Security OAuth2Application[users.perms.edit]
func setUserPerms(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}
	ps := &services.Permission{DB: db}

	var err error
	var id uint
	if id, err = cast.ToUintE(c.Param("id")); err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}

	viewModel := &models.PermissionView{}
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

	// get the current user's scopes, including the ones from their role
	editorScopes := callerScopes(c)

	// admins can override, so skip our comparers
	if scopes.ContainsScope(editorScopes, scopes.ScopeAdmin) {
		perms.Scopes = viewModel.Scopes
	} else {
		if denyEditingAdmin(c, db, user) {
			return
		}
		// server scopes in the global row apply on every server; only an administrator may
		// grant those, or a user with this permission could open every customer's server
		editable := panelWideScopes(editorScopes)
		allowedScopes := utils.Union(viewModel.Scopes, editable)
		// update perms to match this "setup", but not stomp over what the user can't change
		replacement := scopes.UpdateScopesWhereGranted(perms.Scopes, allowedScopes, editable)
		perms.Scopes = replacement
	}

	err = ps.UpdatePermissions(perms)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.Status(http.StatusNoContent)
}

func newUserSearch() *models.UserSearch {
	return &models.UserSearch{
		Username:  "*",
		Email:     "*",
		PageLimit: DefaultPageSize,
		Page:      1,
	}
}

// callerScopes returns the scopes checkPermission resolved for this request (role included)
func callerScopes(c *gin.Context) []*scopes.Scope {
	if v, ok := c.Get("scopes"); ok {
		if granted, ok := v.([]*scopes.Scope); ok {
			return granted
		}
	}
	return []*scopes.Scope{}
}

// panelWideScopes leaves out the scopes that act on a server
func panelWideScopes(list []*scopes.Scope) []*scopes.Scope {
	out := make([]*scopes.Scope, 0, len(list))
	for _, s := range list {
		if !s.ForServer {
			out = append(out, s)
		}
	}
	return out
}

func callerIsAdmin(c *gin.Context) bool {
	return scopes.ContainsScope(callerScopes(c), scopes.ScopeAdmin)
}

// denyEditingAdmin stops non-admins from changing an administrator's account, which would
// let them take it over (new email or password). It aborts and returns true when denied.
func denyEditingAdmin(c *gin.Context, db *gorm.DB, target *models.User) bool {
	isAdmin, err := userIsAdmin(db, target)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return true
	}

	if isAdmin {
		c.AbortWithStatusJSON(http.StatusForbidden, &skypanel.ErrorResponse{Error: skypanel.FromError(errors.New("only administrators can modify an administrator"))})
		return true
	}
	return false
}

// userIsAdmin reports whether the user holds the admin scope, globally or through their role
// userIsPrivileged is true for an account with a panel-wide power beyond a customer's
// (admin, settings, users, nodes, templates…), directly or through its role. Sign-in links
// from the billing API are refused for these: a leaked billing key must not open them.
func userIsPrivileged(db *gorm.DB, user *models.User) (bool, error) {
	ps := &services.Permission{DB: db}
	perms, err := ps.GetForUserAndServer(user.ID, "")
	if err != nil {
		return false, err
	}
	held := append([]*scopes.Scope{}, perms.Scopes...)
	if user.RoleID != nil {
		rs := &services.Role{DB: db}
		role, err := rs.Get(*user.RoleID)
		if err != nil {
			return false, err
		}
		for _, s := range role.Scopes {
			held = scopes.AddScope(held, scopes.GetScope(s))
		}
	}
	for _, s := range held {
		if isPrivilegedScope(s) {
			return true, nil
		}
	}
	return false, nil
}

// privilegedScopes change the panel or other people's accounts. Customers hold read-only
// panel-wide scopes too (templates.view, uptime.view), so the test is this list, not "any
// scope that is not tied to a server".
var privilegedScopes = []*scopes.Scope{
	scopes.ScopeAdmin,
	scopes.ScopeSettingsEdit,
	scopes.ScopeNodesCreate, scopes.ScopeNodesEdit, scopes.ScopeNodesDelete, scopes.ScopeNodesDeploy,
	scopes.ScopeTemplatesLocalEdit, scopes.ScopeTemplatesRepoCreate, scopes.ScopeTemplatesRepoDelete,
	scopes.ScopeUserInfoEdit, scopes.ScopeUserPermsEdit,
	scopes.ScopeOAuth2Auth,
}

// isPrivilegedScope is a panel-wide power a customer does not get. Compared one by one:
// ContainsScope would count admin in the list as a match for anything.
func isPrivilegedScope(s *scopes.Scope) bool {
	if s == nil {
		return false
	}
	for _, p := range privilegedScopes {
		if p.Is(s.Value) {
			return true
		}
	}
	return false
}

func userIsAdmin(db *gorm.DB, user *models.User) (bool, error) {
	ps := &services.Permission{DB: db}
	perms, err := ps.GetForUserAndServer(user.ID, "")
	if err != nil {
		return false, err
	}
	userScopes := append([]*scopes.Scope{}, perms.Scopes...)

	if user.RoleID != nil {
		rs := &services.Role{DB: db}
		role, err := rs.Get(*user.RoleID)
		if err == nil && role != nil {
			for _, s := range role.Scopes {
				userScopes = scopes.AddScope(userScopes, scopes.GetScope(s))
			}
		}
	}

	return scopes.ContainsScope(userScopes, scopes.ScopeAdmin), nil
}
