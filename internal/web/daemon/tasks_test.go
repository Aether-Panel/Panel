package daemon

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// While a server is reloaded or deleted its scheduler is stopped: a task change then is
// refused with 409 and a clear error, not a 500
func TestTaskHandlers_AStoppedSchedulerAnswersConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := config.ServersFolder.Value()
	require.NoError(t, config.ServersFolder.Set(t.TempDir(), false))
	t.Cleanup(func() { _ = config.ServersFolder.Set(previous, false) })

	server, _ := newPluginTestServer(t, "")
	server.Scheduler = servers.NewDefaultScheduler(server.ID()) // never started, as once stopped
	server.Scheduler.Tasks["restart"] = skypanel.Task{Name: "Restart", CronSchedule: "0 5 * * *"}
	task := `{"name":"Restart","cronSchedule":"0 6 * * *","operations":[{"type":"restart"}]}`
	params := gin.Params{{Key: "taskID", Value: "restart"}}

	for name, w := range map[string]interface{ Result() *http.Response }{
		"delete": runFileHandler(deleteServerTask, server, http.MethodDelete, "/", "", params),
		"edit":   runFileHandler(editServerTask, server, http.MethodPut, "/", task, params),
		"run":    runFileHandler(runServerTask, server, http.MethodPost, "/", "", params),
	} {
		res := w.Result()
		assert.Equal(t, http.StatusConflict, res.StatusCode, name)
		var body skypanel.ErrorResponse
		if assert.NoError(t, json.NewDecoder(res.Body).Decode(&body), name) && assert.NotNil(t, body.Error, name) {
			assert.Equal(t, "ErrSchedulerStopped", body.Error.Code, name)
		}
		_ = res.Body.Close()
	}
	assert.NoFileExists(t, filepath.Join(config.ServersFolder.Value(), server.ID()+".cron"))
}
