package user

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/domain"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/scopes"
	"github.com/gin-gonic/gin"
	"net/http"
)

func registerUserSettings(g *gin.RouterGroup) {
	g.Handle("GET", "", middleware.RequiresPermission(scopes.ScopeLogin), getUserSettings)
	g.Handle("PUT", "/:key", middleware.RequiresPermission(scopes.ScopeLogin), setUserSetting)
	g.Handle("OPTIONS", "", response.CreateOptions("GET", "PUT"))
}

// @Summary Get a user setting
// @Description Gets all settings specific to the current user
// @Success 200 {object} domain.UserSettingsView
// @Failure 500 {object} skypanel.ErrorResponse
// @Tags User Settings
// @Router /api/usersettings [get]
// @Security OAuth2Application[login]
func getUserSettings(c *gin.Context) {
	db := middleware.GetDatabase(c)
	uss := &services.UserSettings{DB: db}

	user := c.MustGet("user").(*domain.User)

	results, err := uss.GetAllForUser(user.ID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, results)
}

// @Summary Update a user setting
// @Description Updates the value of a user setting
// @Success 204 {object} nil
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param key path string true "The config key"
// @Param value body domain.ChangeUserSetting true "The new value for the setting"
// @Tags User Settings
// @Router /api/usersettings/{key} [PUT]
// @Security OAuth2Application[login]
func setUserSetting(c *gin.Context) {
	key := c.Param("key")
	db := middleware.GetDatabase(c)
	uss := &services.UserSettings{DB: db}

	user := c.MustGet("user").(*domain.User)

	var model domain.ChangeUserSetting
	if err := c.BindJSON(&model); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	err := uss.Update(&domain.UserSetting{
		Key:    key,
		UserID: user.ID,
		Value:  model.Value,
	})

	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.Status(http.StatusNoContent)
}
