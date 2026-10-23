package api

import (
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
)

func registerActivity(g *gin.RouterGroup) {
	g.Handle("GET", "", middleware.RequiresPermission(scopes.ScopeUserInfoSearch), listActivity)
	g.Handle("OPTIONS", "", response.CreateOptions("GET"))
}

// @Summary List access activity
// @Description Gets the recorded sign-in and account-creation events, newest first. Readable by anyone who can search users.
// @Success 200 {object} models.ActivitySearchResponse
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param event query string false "Only return this event kind (login, login_failed, register)"
// @Param page query int false "Page number (default: 1)"
// @Param limit query int false "Number of events per page (default: 10, max: 100)"
// @Tags Activity
// @Router /api/activity [get]
// @Security OAuth2Application[users.info.search]
func listActivity(c *gin.Context) {
	db := middleware.GetDatabase(c)
	activity := &services.Activity{DB: db}

	search := &models.ActivitySearch{}
	if err := c.ShouldBindQuery(search); err != nil {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}

	limit := int(search.Limit)
	if limit < 1 {
		limit = 10
	}
	if limit > MaxPageSize {
		limit = MaxPageSize
	}

	page := int(search.Page)
	if page < 1 {
		page = 1
	}

	entries, total, err := activity.List(limit, page, search.Event)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	views := make([]*models.ActivityView, 0, len(entries))
	for _, entry := range entries {
		views = append(views, models.FromActivity(entry))
	}

	c.JSON(http.StatusOK, &models.ActivitySearchResponse{
		Activity: views,
		Metadata: &skypanel.Metadata{
			Paging: &skypanel.Paging{
				Page:    uint(page),
				Size:    uint(limit),
				MaxSize: MaxPageSize,
				Total:   total,
			},
		},
	})
}
