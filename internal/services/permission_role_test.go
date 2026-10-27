package services

import (
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/stretchr/testify/assert"
)

func TestHasPermission_AnAdministratorByRoleIsOneEverywhere(t *testing.T) {
	db := setupOtpTestDB(t)
	user := createOtpTestUser(t, db, "", false)
	role := &models.Role{Name: "Administrador", Scopes: []string{scopes.ScopeAdmin.String(), scopes.ScopeLogin.String()}}
	assert.NoError(t, db.Create(role).Error)
	assert.NoError(t, db.Model(user).Update("role_id", role.ID).Error)
	ps := &Permission{DB: db}

	allowed, err := ps.HasPermission(user.ID, "srv1", scopes.ScopeServerSftp)
	assert.NoError(t, err)
	assert.True(t, allowed, "SFTP on a server they are not a member of")

	allowed, err = ps.MaySignIn(user.ID)
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func TestHasPermission_ARolesServerScopesDoNotApplyOnServers(t *testing.T) {
	db := setupOtpTestDB(t)
	user := createOtpTestUser(t, db, "", false)
	role := &models.Role{Name: "Usuario", Scopes: []string{scopes.ScopeLogin.String(), scopes.ScopeServerSftp.String()}}
	assert.NoError(t, db.Create(role).Error)
	assert.NoError(t, db.Model(user).Update("role_id", role.ID).Error)
	ps := &Permission{DB: db}

	allowed, err := ps.HasPermission(user.ID, "srv1", scopes.ScopeServerSftp)
	assert.NoError(t, err)
	assert.False(t, allowed, "only the owner or the server's own row give access to a server")

	allowed, err = ps.MaySignIn(user.ID)
	assert.NoError(t, err)
	assert.True(t, allowed, "the role's panel-wide login counts")
}

func TestMaySignIn_FalseOnceTheLoginPermissionIsTakenAway(t *testing.T) {
	db := setupOtpTestDB(t)
	user := createOtpTestUser(t, db, "", false)
	ps := &Permission{DB: db}
	server := "srv1"
	assert.NoError(t, ps.UpdatePermissions(&models.Permissions{UserID: &user.ID, ServerIdentifier: &server, Scopes: []*scopes.Scope{scopes.ScopeServerSftp}}))

	allowed, err := ps.MaySignIn(user.ID)
	assert.NoError(t, err)
	assert.False(t, allowed, "a server row alone does not let anyone in")

	assert.NoError(t, ps.UpdatePermissions(&models.Permissions{UserID: &user.ID, Scopes: []*scopes.Scope{scopes.ScopeLogin}}))
	allowed, err = ps.MaySignIn(user.ID)
	assert.NoError(t, err)
	assert.True(t, allowed)
}
