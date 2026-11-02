package services

import (
	"fmt"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

// setupSFTPKeyLogin is a user who may sign in, holds SFTP on srv1 and has one SSH key
func setupSFTPKeyLogin(t *testing.T) (*gorm.DB, *models.User, ssh.PublicKey) {
	db, user := setupSSHKeyTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Server{}))
	ps := &Permission{DB: db}
	server := "srv1"
	require.NoError(t, ps.UpdatePermissions(&models.Permissions{UserID: &user.ID, Scopes: []*scopes.Scope{scopes.ScopeLogin}}))
	require.NoError(t, ps.UpdatePermissions(&models.Permissions{UserID: &user.ID, ServerIdentifier: &server, Scopes: []*scopes.Scope{scopes.ScopeServerSftp}}))

	line, key := newSSHKeyLine(t, "ed25519", "laptop")
	_, err := (&SSHKey{DB: db}).Add(user.ID, "", line)
	require.NoError(t, err)

	limitKey := fmt.Sprintf("sshkey:user:%d", user.ID)
	LoginAttempts.Reset(limitKey)
	t.Cleanup(func() { LoginAttempts.Reset(limitKey) })
	return db, user, key
}

func TestValidateSFTPKey_LetsTheKeyIn(t *testing.T) {
	db, user, key := setupSFTPKeyLogin(t)

	perms, err := validateSFTPKey(db, user.Email, "srv1", key)

	require.NoError(t, err)
	assert.Equal(t, "srv1", perms.Extensions["server_id"])

	_, other := newSSHKeyLine(t, "ed25519", "")
	_, err = validateSFTPKey(db, user.Email, "srv1", other)
	assert.ErrorIs(t, err, errSFTPRefused, "a key that is not the user's")
}

// The panel's own node limits failed key logins like /auth/sftp-key does for remote nodes,
// with the same budget, which a success clears
func TestValidateSFTPKey_LimitsFailedAttempts(t *testing.T) {
	db, user, key := setupSFTPKeyLogin(t)
	limitKey := fmt.Sprintf("sshkey:user:%d", user.ID)

	for i := 0; i < 9; i++ {
		_, wrong := newSSHKeyLine(t, "ed25519", "")
		_, err := validateSFTPKey(db, user.Email, "srv1", wrong)
		require.ErrorIs(t, err, errSFTPRefused)
	}
	_, err := validateSFTPKey(db, user.Email, "srv1", key)
	require.NoError(t, err, "the tenth attempt still fits")
	assert.True(t, LoginAttempts.Allowed(limitKey), "a success clears the misses")

	for i := 0; i < 10; i++ {
		_, wrong := newSSHKeyLine(t, "ed25519", "")
		_, err = validateSFTPKey(db, user.Email, "srv1", wrong)
		require.ErrorIs(t, err, errSFTPRefused)
	}
	_, err = validateSFTPKey(db, user.Email, "srv1", key)
	assert.ErrorIs(t, err, skypanel.ErrTooManyAttempts, "once the budget is spent even the right key waits")
	assert.True(t, LoginAttempts.Allowed(fmt.Sprintf("password:user:%d", user.ID)), "key misses do not lock the password login")
}
