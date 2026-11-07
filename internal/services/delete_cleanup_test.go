package services

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// setupDeleteCleanupDB is a SQLite database with the tables a server or user delete touches
func setupDeleteCleanupDB(t *testing.T, extra ...interface{}) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "delete_cleanup.db")), &gorm.Config{})
	require.NoError(t, err)
	tables := []interface{}{&models.User{}, &models.Role{}, &models.Permissions{}, &models.Client{}, &models.Session{},
		&models.Node{}, &models.Server{}, &models.Backup{}, &models.UptimeStatus{}, &models.ExTransferSession{}, &models.ExTransferLog{}}
	require.NoError(t, db.AutoMigrate(append(tables, extra...)...))
	return db
}

// addCleanupServer stores a server with an OAuth client, a session made for that client and
// a federated transfer session with a log line
func addCleanupServer(t *testing.T, db *gorm.DB, id string, userID uint) (clientID uint, transferUUID string) {
	t.Helper()
	require.NoError(t, db.Omit(clause.Associations).Create(&models.Server{Identifier: id, Name: id, Type: "generic"}).Error)

	serverID := id
	client := &models.Client{ClientID: "client-" + id, HashedClientSecret: "x", UserID: userID, ServerID: &serverID}
	require.NoError(t, db.Omit(clause.Associations).Create(client).Error)
	require.NoError(t, db.Omit(clause.Associations).Create(&models.Session{
		Token: "token-" + id, ExpirationTime: time.Now().Add(time.Hour), ClientID: &client.ID, UserID: &userID,
	}).Error)

	transferUUID = addCleanupTransfer(t, db, id, userID)
	return client.ID, transferUUID
}

// addCleanupTransfer stores a federated transfer session of the server and user, with a log line
func addCleanupTransfer(t *testing.T, db *gorm.DB, serverID string, userID uint) string {
	t.Helper()
	uuid := "00000000-0000-0000-0000-0000000" + serverID[len(serverID)-5:]
	require.NoError(t, db.Omit(clause.Associations).Create(&models.ExTransferSession{
		SessionUUID: uuid, ServerID: serverID, UserID: userID, TokenHash: "hash-" + uuid, ExpiresAt: time.Now().Add(time.Hour),
	}).Error)
	require.NoError(t, db.Omit(clause.Associations).Create(&models.ExTransferLog{SessionID: uuid, Action: "CREATED", IPAddress: "127.0.0.1"}).Error)
	return uuid
}

func countRows(t *testing.T, db *gorm.DB, model interface{}, query string, args ...interface{}) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(model).Where(query, args...).Count(&n).Error)
	return n
}

func TestServerDelete_RemovesClientSessionsAndTransfers(t *testing.T) {
	db := setupDeleteCleanupDB(t, &models.Database{}, &models.DatabaseHost{})
	gone, goneTransfer := addCleanupServer(t, db, "gone-00001", 1)
	kept, keptTransfer := addCleanupServer(t, db, "kept-00002", 1)

	require.NoError(t, (&Server{DB: db}).Delete("gone-00001"))

	assert.Zero(t, countRows(t, db, &models.Session{}, "client_id = ?", gone), "the sessions of the server's clients")
	assert.Zero(t, countRows(t, db, &models.Client{}, "id = ?", gone))
	assert.Zero(t, countRows(t, db, &models.ExTransferSession{}, "session_uuid = ?", goneTransfer))
	assert.Zero(t, countRows(t, db, &models.ExTransferLog{}, "session_id = ?", goneTransfer))

	// another server's rows stay
	assert.EqualValues(t, 1, countRows(t, db, &models.Session{}, "client_id = ?", kept))
	assert.EqualValues(t, 1, countRows(t, db, &models.Client{}, "id = ?", kept))
	assert.EqualValues(t, 1, countRows(t, db, &models.ExTransferSession{}, "session_uuid = ?", keptTransfer))
	assert.EqualValues(t, 1, countRows(t, db, &models.ExTransferLog{}, "session_id = ?", keptTransfer))
}

func TestServerDelete_ADatabaseDropErrorDeletesNothing(t *testing.T) {
	// no databases table: listing the server's databases fails
	db := setupDeleteCleanupDB(t)
	clientID, _ := addCleanupServer(t, db, "gone-00001", 1)

	assert.Error(t, (&Server{DB: db}).Delete("gone-00001"))

	assert.EqualValues(t, 1, countRows(t, db, &models.Server{}, "identifier = ?", "gone-00001"))
	assert.EqualValues(t, 1, countRows(t, db, &models.Client{}, "id = ?", clientID), "nothing goes before the databases are dropped")
}

func TestUserDelete_RemovesTheirTransfersAndClientSessions(t *testing.T) {
	db := setupDeleteCleanupDB(t, &models.RecoveryCode{}, &models.PasswordReset{}, &models.SSHKey{}, &models.UserSetting{}, &models.GitHubAccount{})
	user := &models.User{Username: "gone", Email: "gone@example.com", HashedPassword: "x"}
	require.NoError(t, db.Create(user).Error)
	other := &models.User{Username: "kept", Email: "kept@example.com", HashedPassword: "x"}
	require.NoError(t, db.Create(other).Error)

	clientID, transfer := addCleanupServer(t, db, "srv-00001", user.ID)
	keptTransfer := addCleanupTransfer(t, db, "srv-00002", other.ID)

	require.NoError(t, (&User{DB: db}).Delete(user))

	assert.Zero(t, countRows(t, db, &models.ExTransferSession{}, "session_uuid = ?", transfer))
	assert.Zero(t, countRows(t, db, &models.ExTransferLog{}, "session_id = ?", transfer))
	assert.Zero(t, countRows(t, db, &models.Session{}, "client_id = ?", clientID))
	assert.Zero(t, countRows(t, db, &models.Client{}, "id = ?", clientID))

	assert.EqualValues(t, 1, countRows(t, db, &models.ExTransferSession{}, "session_uuid = ?", keptTransfer), "another user's transfer stays")
	assert.EqualValues(t, 1, countRows(t, db, &models.ExTransferLog{}, "session_id = ?", keptTransfer))
}
