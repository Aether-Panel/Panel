package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type activityHarness struct {
	db      *gorm.DB
	router  *gin.Engine
	user    *models.User
	server  *models.Server
	status  int
	scopes  []*scopes.Scope
	gotBody string
}

// newActivityHarness wires recordServerActivity in front of stub handlers on the same
// paths the panel registers, so the recorded entries can be checked without a node
func newActivityHarness(t *testing.T) *activityHarness {
	gin.SetMode(gin.TestMode)

	previous := serverActivityWrite
	serverActivityWrite = func(fn func()) { fn() }
	t.Cleanup(func() { serverActivityWrite = previous })

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "activity_api.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Role{}, &models.Permissions{}, &models.Node{}, &models.Server{}, &models.Backup{}, &models.ServerActivity{}))

	h := &activityHarness{db: db, status: http.StatusNoContent}
	h.user = &models.User{Username: "alice", Email: "alice@example.com", HashedPassword: "x"}
	require.NoError(t, db.Create(h.user).Error)
	h.server = &models.Server{Identifier: "srv1", Name: "srv1", Type: "generic"}
	require.NoError(t, db.Omit(clause.Associations).Create(h.server).Error)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("user", h.user)
		c.Next()
	})
	g := r.Group("/api/servers")
	g.Use(recordServerActivity)

	stub := func(c *gin.Context) {
		body, _ := io.ReadAll(c.Request.Body)
		h.gotBody = string(body)
		c.Status(h.status)
	}
	for _, route := range []string{
		"PUT ", "POST /suspend", "PUT /name/:name", "POST /start", "POST /stop", "POST /restart",
		"POST /kill", "POST /install", "POST /reload", "POST /console", "GET /console",
		"PUT /file/*filename", "POST /file/*filename", "DELETE /file/*filename", "GET /file/*filename",
		"POST /archive/*filename", "POST /extract/*filename", "POST /plugins/:pluginId", "DELETE /plugins",
		"POST /backup/create", "POST /backup/restore/:backupID", "DELETE /backup/:backupID",
		"PUT /user/:email", "DELETE /user/:email", "POST /data", "PUT /port-settings", "PUT /definition",
		"POST /flags", "PUT /notifications", "POST /notifications/test", "POST /ai/analyze",
		"PUT /tasks/:taskId", "DELETE /tasks/:taskId", "POST /tasks/:taskId/run",
		"POST /extransfer/create", "POST /dns/refresh",
	} {
		method, path, _ := strings.Cut(route, " ")
		g.Handle(method, "/:serverId"+path, stub)
	}
	g.DELETE("/:serverId", func(c *gin.Context) {
		h.db.Where("identifier = ?", c.Param("serverId")).Delete(&models.Server{})
		c.Status(h.status)
	})
	g.GET("/:serverId/activity", func(c *gin.Context) {
		c.Set("server", h.server)
		c.Set("scopes", h.scopes)
	}, getServerActivity)

	h.router = r
	return h
}

func (h *activityHarness) do(t *testing.T, method, target, body, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.RemoteAddr = "203.0.113.9:4000"
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func (h *activityHarness) entries(t *testing.T) []models.ServerActivity {
	var out []models.ServerActivity
	require.NoError(t, h.db.Order("id").Find(&out).Error)
	return out
}

func TestRecordServerActivity_MapsRoutesToActions(t *testing.T) {
	h := newActivityHarness(t)
	require.NoError(t, h.db.Create(&models.Backup{Name: "before update", FileName: "b.tar.gz", ServerID: "srv1"}).Error)
	other := &models.User{Username: "bob", Email: "bob@example.com", HashedPassword: "x"}
	require.NoError(t, h.db.Create(other).Error)
	srv := "srv1"
	require.NoError(t, h.db.Omit(clause.Associations).Create(&models.Permissions{UserID: &other.ID, ServerIdentifier: &srv, RawScopes: "server.view"}).Error)

	cases := []struct {
		method, target, body, contentType string
		action, detail                    string
	}{
		{"POST", "/api/servers/srv1/start", "", "", "server.start", ""},
		{"POST", "/api/servers/srv1/stop", "", "", "server.stop", ""},
		{"POST", "/api/servers/srv1/restart", "", "", "server.restart", ""},
		{"POST", "/api/servers/srv1/kill", "", "", "server.kill", ""},
		{"POST", "/api/servers/srv1/install", "", "", "server.install", ""},
		{"POST", "/api/servers/srv1/reload", "", "", "server.reload", ""},
		{"PUT", "/api/servers/srv1/name/Lobby", "", "", "server.rename", "Lobby"},
		{"POST", "/api/servers/srv1/suspend", "", "", "server.suspend", ""},
		{"PUT", "/api/servers/srv1/file/config/server.properties", "motd=hi", "text/plain", "file.write", "config/server.properties"},
		{"PUT", "/api/servers/srv1/file/world?folder", "", "", "file.mkdir", "world"},
		{"PUT", "/api/servers/srv1/file/mods/a.jar", "--x\r\n", "multipart/form-data; boundary=x", "file.upload", "mods/a.jar"},
		{"POST", "/api/servers/srv1/file/old.txt?destination=new.txt", "", "", "file.rename", "old.txt → new.txt"},
		{"DELETE", "/api/servers/srv1/file/logs/latest.log", "", "", "file.delete", "logs/latest.log"},
		{"POST", "/api/servers/srv1/archive/backup.zip", "[]", "application/json", "file.archive", "backup.zip"},
		{"POST", "/api/servers/srv1/extract/backup.zip?destination=.", "", "", "file.extract", "backup.zip"},
		{"POST", "/api/servers/srv1/plugins/essentialsx", "", "", "plugin.install", "essentialsx"},
		{"DELETE", "/api/servers/srv1/plugins?name=Essentials.jar", "", "", "plugin.delete", "Essentials.jar"},
		{"POST", "/api/servers/srv1/backup/create?name=nightly", "", "", "backup.create", "nightly"},
		{"POST", "/api/servers/srv1/backup/restore/1", "", "", "backup.restore", "before update"},
		{"DELETE", "/api/servers/srv1/backup/99", "", "", "backup.delete", "#99"},
		{"PUT", "/api/servers/srv1/user/new@example.com", "{}", "application/json", "user.add", "new@example.com"},
		{"PUT", "/api/servers/srv1/user/bob@example.com", "{}", "application/json", "user.edit", "bob@example.com"},
		{"DELETE", "/api/servers/srv1/user/bob@example.com", "", "", "user.remove", "bob@example.com"},
		{"POST", "/api/servers/srv1/data", `{"data":{"password":{"value":"hunter2"}}}`, "application/json", "settings.data", ""},
		{"PUT", "/api/servers/srv1/port-settings", "{}", "application/json", "settings.ports", ""},
		{"PUT", "/api/servers/srv1/definition", "{}", "application/json", "settings.definition", ""},
		{"POST", "/api/servers/srv1/flags", "{}", "application/json", "settings.flags", ""},
		{"PUT", "/api/servers/srv1/notifications", `{"webhook":"https://discord.com/api/webhooks/secret"}`, "application/json", "settings.notifications", ""},
		{"PUT", "/api/servers/srv1/tasks/task-1", `{"name":"Nightly restart"}`, "application/json", "task.save", "Nightly restart"},
		{"DELETE", "/api/servers/srv1/tasks/task-1", "", "", "task.delete", "task-1"},
		{"POST", "/api/servers/srv1/tasks/task-1/run", "", "", "task.run", "task-1"},
		{"POST", "/api/servers/srv1/extransfer/create", "{}", "application/json", "extransfer.create", ""},
		{"POST", "/api/servers/srv1/dns/refresh", "", "", "server.change", "POST /dns/refresh"},
		{"PUT", "/api/servers/srv1", "{}", "application/json", "server.create", ""},
	}

	for _, tc := range cases {
		before := len(h.entries(t))
		rec := h.do(t, tc.method, tc.target, tc.body, tc.contentType)
		require.Equal(t, http.StatusNoContent, rec.Code, tc.target)

		all := h.entries(t)
		if !assert.Len(t, all, before+1, "%s %s", tc.method, tc.target) {
			continue
		}
		got := all[len(all)-1]
		assert.Equal(t, tc.action, got.Action, "%s %s", tc.method, tc.target)
		assert.Equal(t, tc.detail, got.Detail, "%s %s", tc.method, tc.target)
		assert.Equal(t, "srv1", got.ServerID)
		assert.Equal(t, "alice", got.Username)
		require.NotNil(t, got.UserID)
		assert.Equal(t, h.user.ID, *got.UserID)
		assert.Equal(t, "203.0.113.9", got.IP)
	}
}

func TestRecordServerActivity_SkipsReadsFailuresAndNoOps(t *testing.T) {
	h := newActivityHarness(t)

	h.do(t, "GET", "/api/servers/srv1/console", "", "")
	h.do(t, "GET", "/api/servers/srv1/file/server.properties", "", "")
	h.do(t, "POST", "/api/servers/srv1/ai/analyze", "", "")
	h.do(t, "POST", "/api/servers/srv1/notifications/test", "", "")

	h.status = http.StatusForbidden
	h.do(t, "POST", "/api/servers/srv1/start", "", "")
	h.status = http.StatusInternalServerError
	h.do(t, "PUT", "/api/servers/srv1/file/a.txt", "x", "text/plain")

	assert.Empty(t, h.entries(t))
}

func TestRecordServerActivity_CapturesConsoleCommand(t *testing.T) {
	h := newActivityHarness(t)

	h.do(t, "POST", "/api/servers/srv1/console", "say hola\n", "text/plain")
	assert.Equal(t, "say hola\n", h.gotBody, "the handler still receives the whole body")

	long := "say " + strings.Repeat("a", 500)
	h.do(t, "POST", "/api/servers/srv1/console", long, "text/plain")
	assert.Equal(t, long, h.gotBody)

	all := h.entries(t)
	require.Len(t, all, 2)
	assert.Equal(t, "console.command", all[0].Action)
	assert.Equal(t, "say hola", all[0].Detail)
	assert.Equal(t, consoleCommandMax, len([]rune(all[1].Detail)))
	assert.True(t, strings.HasPrefix(all[1].Detail, "say aaa"))
}

func TestRecordServerActivity_ServerDeletionClearsItsLog(t *testing.T) {
	h := newActivityHarness(t)
	h.do(t, "POST", "/api/servers/srv1/start", "", "")
	require.NoError(t, h.db.Create(&models.ServerActivity{ServerID: "other", Action: "server.start"}).Error)

	h.do(t, "DELETE", "/api/servers/srv1", "", "")

	all := h.entries(t)
	require.Len(t, all, 1)
	assert.Equal(t, "other", all[0].ServerID)
}

func TestGetServerActivity_PaginatesAndHidesIP(t *testing.T) {
	h := newActivityHarness(t)
	now := time.Now()
	for i := 0; i < 5; i++ {
		action := "server.start"
		if i%2 == 1 {
			action = "file.write"
		}
		require.NoError(t, h.db.Create(&models.ServerActivity{ServerID: "srv1", Action: action, Username: "alice", UserID: &h.user.ID, IP: "198.51.100.1", CreatedAt: now}).Error)
	}
	require.NoError(t, h.db.Create(&models.ServerActivity{ServerID: "srv2", Action: "server.stop"}).Error)

	fetch := func(query string) (map[string]json.RawMessage, []map[string]any) {
		rec := h.do(t, "GET", "/api/servers/srv1/activity"+query, "", "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
		var items []map[string]any
		require.NoError(t, json.Unmarshal(raw["items"], &items))
		return raw, items
	}

	h.scopes = []*scopes.Scope{scopes.ScopeServerUserView}
	raw, items := fetch("?limit=2")
	require.Len(t, items, 2)
	assert.Equal(t, float64(5), items[0]["id"])
	assert.NotContains(t, items[0], "ip", "non-admins never see the IP")
	assert.Equal(t, "alice", items[0]["username"])
	var cursor uint
	require.NoError(t, json.Unmarshal(raw["nextCursor"], &cursor))
	assert.Equal(t, uint(4), cursor)

	_, items = fetch("?limit=2&before=4")
	require.Len(t, items, 2)
	assert.Equal(t, float64(3), items[0]["id"])

	raw, items = fetch("?limit=2&before=2")
	require.Len(t, items, 1)
	assert.Equal(t, "null", string(raw["nextCursor"]))

	_, items = fetch("?type=files")
	require.Len(t, items, 2)
	for _, item := range items {
		assert.Equal(t, "file.write", item["action"])
	}

	h.scopes = []*scopes.Scope{scopes.ScopeAdmin}
	_, items = fetch("?limit=1")
	require.Len(t, items, 1)
	assert.Equal(t, "198.51.100.1", items[0]["ip"])

	rec := h.do(t, "GET", "/api/servers/srv1/activity?limit=abc", "", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Len(t, h.entries(t), 6, "reading the log records nothing")
}
