package api

import (
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// a member who can see the Arma server, start it and read its console, and nothing else
func clientLimitsDB(t *testing.T) (*gorm.DB, *models.User) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "clients.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Node{}, &models.Server{}, &models.Role{}, &models.User{}, &models.Client{}, &models.Permissions{}))
	require.NoError(t, db.Omit(clause.Associations).Create(&models.Server{Identifier: "arma0001", Name: "Arma", Type: "armareforger"}).Error)
	require.NoError(t, db.Omit(clause.Associations).Create(&models.Server{Identifier: "other001", Name: "Other", Type: "standard"}).Error)
	user := &models.User{Username: "player", Email: "player@example.com", HashedPassword: "x"}
	require.NoError(t, db.Create(user).Error)
	server := "arma0001"
	require.NoError(t, db.Create(&models.Permissions{
		UserID: &user.ID, ServerIdentifier: &server,
		Scopes: []*scopes.Scope{scopes.ScopeServerStart, scopes.ScopeServerConsole},
	}).Error)
	return db, user
}

func TestClientLimits_OneServerAndWhatTheUserHoldsThere(t *testing.T) {
	db, user := clientLimitsDB(t)

	serverID, limitedTo, err := clientLimits(db, user, personalClientRequest{ServerID: "arma0001", Scopes: []string{"server.start", "server.start"}})
	require.NoError(t, err)
	require.NotNil(t, serverID)
	assert.Equal(t, "arma0001", *serverID)
	assert.Equal(t, "server.start server.view", limitedTo, "repeats are dropped and the server stays visible to the client")

	serverID, limitedTo, err = clientLimits(db, user, personalClientRequest{ServerID: "arma0001"})
	require.NoError(t, err)
	assert.Equal(t, "arma0001", *serverID)
	assert.Empty(t, limitedTo, "no scopes means whatever the user can do on that server")

	serverID, limitedTo, err = clientLimits(db, user, personalClientRequest{})
	require.NoError(t, err)
	assert.Nil(t, serverID)
	assert.Empty(t, limitedTo)
}

func TestClientLimits_RefusesWhatTheUserCannotGive(t *testing.T) {
	db, user := clientLimitsDB(t)

	_, _, err := clientLimits(db, user, personalClientRequest{ServerID: "other001"})
	assert.ErrorIs(t, err, skypanel.ErrServerNotFound, "a server the user cannot see looks like it does not exist")

	_, _, err = clientLimits(db, user, personalClientRequest{ServerID: "arma0001", Scopes: []string{"server.files.edit"}})
	assert.ErrorIs(t, err, skypanel.ErrNoPermission, "a client never gets more than its owner has there")

	_, _, err = clientLimits(db, user, personalClientRequest{ServerID: "arma0001", Scopes: []string{"users.info.edit"}})
	assert.ErrorIs(t, err, errClientScopes, "only server permissions can be given")

	_, _, err = clientLimits(db, user, personalClientRequest{ServerID: "arma0001", Scopes: []string{"made.up"}})
	assert.ErrorIs(t, err, errClientScopes)
}
