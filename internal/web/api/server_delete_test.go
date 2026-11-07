package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const deleteTestServer = "del-00001"

type deleteTestEnv struct {
	db           *gorm.DB
	clientID     uint
	transferUUID string
}

// setupServerDeleteTest stores a server with a MySQL database, an OAuth client with a session
// and a federated transfer with a log line
func setupServerDeleteTest(t *testing.T) *deleteTestEnv {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "delete_test.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Role{}, &models.Permissions{}, &models.Client{}, &models.Session{},
		&models.Node{}, &models.Server{}, &models.Backup{}, &models.UptimeStatus{}, &models.DatabaseHost{}, &models.Database{},
		&models.ExTransferSession{}, &models.ExTransferLog{}))

	id := deleteTestServer
	require.NoError(t, db.Omit(clause.Associations).Create(&models.Server{Identifier: id, Name: "survival", Type: "generic"}).Error)

	host := &models.DatabaseHost{Name: "mysql", Host: "127.0.0.1", Port: 1, Username: "root", Password: "unused"}
	require.NoError(t, db.Create(host).Error)
	require.NoError(t, db.Omit(clause.Associations).Create(&models.Database{
		ServerID: id, DatabaseHostID: host.ID, DatabaseName: "s_del_world", Username: "u_world", Password: "unused",
	}).Error)

	client := &models.Client{ClientID: "client-del", HashedClientSecret: "x", UserID: 1, ServerID: &id}
	require.NoError(t, db.Omit(clause.Associations).Create(client).Error)
	userID := uint(1)
	require.NoError(t, db.Omit(clause.Associations).Create(&models.Session{
		Token: "client-session", ExpirationTime: time.Now().Add(time.Hour), ClientID: &client.ID, UserID: &userID,
	}).Error)

	uuid := "00000000-0000-0000-0000-000000000001"
	require.NoError(t, db.Omit(clause.Associations).Create(&models.ExTransferSession{
		SessionUUID: uuid, ServerID: id, UserID: 1, TokenHash: "hash", ExpiresAt: time.Now().Add(time.Hour),
	}).Error)
	require.NoError(t, db.Omit(clause.Associations).Create(&models.ExTransferLog{SessionID: uuid, Action: "CREATED", IPAddress: "127.0.0.1"}).Error)

	return &deleteTestEnv{db: db, clientID: client.ID, transferUUID: uuid}
}

// deleteWithoutNode runs the delete handler with ?skipNode, so no node is called
func (env *deleteTestEnv) deleteWithoutNode(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	server, err := (&services.Server{DB: env.db}).Get(deleteTestServer)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.DELETE("/servers/:id", func(c *gin.Context) {
		c.Set("db", env.db)
		c.Set("server", server)
	}, deleteServer)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/servers/"+deleteTestServer+"?skipNode", nil))
	return w
}

func (env *deleteTestEnv) count(t *testing.T, model interface{}, query string, args ...interface{}) int64 {
	t.Helper()
	var n int64
	require.NoError(t, env.db.Model(model).Where(query, args...).Count(&n).Error)
	return n
}

// useDatabaseDrop replaces the drop of the server's MySQL databases for this test
func useDatabaseDrop(t *testing.T, drop func(db *gorm.DB, serverID string) ([]services.LeftoverDatabase, error)) {
	previous := dropServerDatabases
	dropServerDatabases = drop
	t.Cleanup(func() { dropServerDatabases = previous })
}

func TestDeleteServer_DropsTheDatabasesWhileTheirRowsExist(t *testing.T) {
	env := setupServerDeleteTest(t)
	var seenDatabases, seenServers int64
	called := false
	useDatabaseDrop(t, func(_ *gorm.DB, serverID string) ([]services.LeftoverDatabase, error) {
		called = true
		seenDatabases = env.count(t, &models.Database{}, "server_id = ?", serverID)
		seenServers = env.count(t, &models.Server{}, "identifier = ?", serverID)
		return nil, nil
	})

	w := env.deleteWithoutNode(t)

	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	require.True(t, called, "the server's databases must be dropped from their hosts")
	assert.EqualValues(t, 1, seenDatabases, "the drop needs the database rows to know what to drop")
	assert.EqualValues(t, 1, seenServers)
	assert.Zero(t, env.count(t, &models.Database{}, "server_id = ?", deleteTestServer))
	assert.Zero(t, env.count(t, &models.Server{}, "identifier = ?", deleteTestServer))
}

func TestDeleteServer_ADatabaseDropErrorDeletesNothing(t *testing.T) {
	env := setupServerDeleteTest(t)
	useDatabaseDrop(t, func(*gorm.DB, string) ([]services.LeftoverDatabase, error) {
		return nil, errors.New("database listing failed")
	})

	w := env.deleteWithoutNode(t)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.EqualValues(t, 1, env.count(t, &models.Server{}, "identifier = ?", deleteTestServer))
	assert.EqualValues(t, 1, env.count(t, &models.Database{}, "server_id = ?", deleteTestServer))
	assert.EqualValues(t, 1, env.count(t, &models.Client{}, "id = ?", env.clientID))
}

func TestDeleteServer_RemovesClientSessionsAndTransfers(t *testing.T) {
	env := setupServerDeleteTest(t)
	useDatabaseDrop(t, func(*gorm.DB, string) ([]services.LeftoverDatabase, error) { return nil, nil })

	w := env.deleteWithoutNode(t)

	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.Zero(t, env.count(t, &models.Session{}, "client_id = ?", env.clientID), "the sessions of the server's OAuth clients")
	assert.Zero(t, env.count(t, &models.Client{}, "id = ?", env.clientID))
	assert.Zero(t, env.count(t, &models.ExTransferSession{}, "session_uuid = ?", env.transferUUID))
	assert.Zero(t, env.count(t, &models.ExTransferLog{}, "session_id = ?", env.transferUUID))
}

// A database host that is down does not stop the delete, but the answer lists what it left
// behind, for the person deleting the server to remove by hand
func TestDeleteServer_ReportsTheDatabasesAHostCouldNotDrop(t *testing.T) {
	env := setupServerDeleteTest(t) // its database host is 127.0.0.1:1, where nothing listens

	w := env.deleteWithoutNode(t)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var answer ServerDeleteResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &answer))
	require.Len(t, answer.LeftoverDatabases, 1)
	left := answer.LeftoverDatabases[0]
	assert.Equal(t, "mysql", left.HostName)
	assert.NotZero(t, left.HostID)
	assert.Equal(t, "s_del_world", left.Database)
	assert.Equal(t, "u_world", left.Username)
	assert.NotContains(t, w.Body.String(), "unused", "never the passwords")
	assert.Zero(t, env.count(t, &models.Server{}, "identifier = ?", deleteTestServer), "the server is deleted anyway")
	assert.Zero(t, env.count(t, &models.Database{}, "server_id = ?", deleteTestServer))
}
