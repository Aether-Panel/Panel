package daemon

import (
	"errors"
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/gin-gonic/gin"
)

func registerGitHubRoutes(l *gin.RouterGroup) {
	l.GET("/:serverId/github/install", middleware.ResolveServerNode, getGitHubInstall)
	l.POST("/:serverId/github/install", middleware.ResolveServerNode, postGitHubInstall)
	l.OPTIONS("/:serverId/github/install", response.CreateOptions("GET", "POST"))
}

// githubInstallErrorStatus: a bad request 400, a busy server 409, GitHub's refusals 422 (the
// panel shows the text of 4xx answers)
func githubInstallErrorStatus(err error) int {
	var upstream *servers.UpstreamError
	switch {
	case errors.Is(err, servers.ErrGitHubBadRequest):
		return http.StatusBadRequest
	case errors.Is(err, servers.ErrGitHubInstalling), errors.Is(err, servers.ErrModpackInstalling), errors.Is(err, servers.ErrServerInstalling),
		errors.Is(err, servers.ErrModpackServerBusy), errors.Is(err, servers.ErrVersionChanging):
		return http.StatusConflict
	case errors.As(err, &upstream):
		return http.StatusUnprocessableEntity
	}
	return http.StatusInternalServerError
}

// @Summary GitHub install status
// @Description The repository install that is running (step, bytes downloaded) or the last one
// @Success 200 {object} servers.GitHubInstallStatus
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /daemon/server/{id}/github/install [get]
// @Security OAuth2Application[server.install]
func getGitHubInstall(c *gin.Context) {
	c.JSON(http.StatusOK, getServerFromGin(c).GitHubInstallStatus())
}

// @Summary Install a GitHub repository
// @Description Downloads a branch's archive from the short-lived GitHub address the panel sends, then stops the server, takes a restore point, empties the folder and unpacks the branch into it, in the background
// @Success 202 {object} nil
// @Param id path string true "Server ID"
// @Tags Daemon Server
// @Router /daemon/server/{id}/github/install [post]
// @Security OAuth2Application[server.install]
func postGitHubInstall(c *gin.Context) {
	var body servers.GitHubInstallRequest
	if response.HandleError(c, c.ShouldBindJSON(&body), http.StatusBadRequest) {
		return
	}
	if err := getServerFromGin(c).InstallFromGitHub(body); err != nil {
		response.HandleError(c, err, githubInstallErrorStatus(err))
		return
	}
	c.Status(http.StatusAccepted)
}
