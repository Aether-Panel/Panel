package services

import (
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/stretchr/testify/assert"
)

func TestServerOwner_HoldsOwnerScopesWhateverTheirRowSays(t *testing.T) {
	db := setupOtpTestDB(t)
	if !assert.NoError(t, db.AutoMigrate(&models.Server{})) {
		t.FailNow()
	}
	owner := createOtpTestUser(t, db, "", false)
	assert.NoError(t, db.Exec(`INSERT INTO servers (identifier, name, internal_id, type) VALUES ('srv1', 'Survival', 'id-srv1', 'generic')`).Error)
	ps := &Permission{DB: db}

	allowed, err := ps.HasPermission(owner.ID, "srv1", scopes.ScopeServerSftp)
	assert.NoError(t, err)
	assert.False(t, allowed, "not the owner yet, and no row")

	assert.NoError(t, ps.SetServerOwner("srv1", owner.ID))

	isOwner, err := ps.IsServerOwner(owner.ID, "srv1")
	assert.NoError(t, err)
	assert.True(t, isOwner)

	row, err := ps.GetForUserAndServer(owner.ID, "srv1")
	assert.NoError(t, err)
	assert.NotZero(t, row.ID, "the owner is a member, so the server is listed as theirs")

	for _, s := range []*scopes.Scope{scopes.ScopeServerSftp, scopes.ScopeServerSendCommand, scopes.ScopeServerUserEdit} {
		allowed, err = ps.HasPermission(owner.ID, "srv1", s)
		assert.NoError(t, err)
		assert.True(t, allowed, s.Value)
	}
	allowed, err = ps.HasPermission(owner.ID, "srv1", scopes.ScopeServerDelete)
	assert.NoError(t, err)
	assert.False(t, allowed, "deleting a server stays with administrators")
}
