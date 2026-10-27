package api

import (
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sign-in links from the billing API are refused for any account that can change the panel
// or other accounts, not only for those holding the literal admin scope. A customer's own
// panel-wide scopes (templates.view, uptime.view, server.create) do not count.
func TestUserIsPrivileged(t *testing.T) {
	db := setupCloneTest(t)
	ps := &services.Permission{DB: db}

	customerRole := &models.Role{Name: "Usuario", Scopes: []string{"login", "self.edit", "server.create", "templates.view", "uptime.view", "server.view", "server.files.edit"}}
	require.NoError(t, db.Create(customerRole).Error)

	newUser := func(name string, roleID *uint, global ...*scopes.Scope) *models.User {
		u := &models.User{Username: name, Email: name + "@example.com", HashedPassword: "x", RoleID: roleID}
		require.NoError(t, db.Create(u).Error)
		if len(global) > 0 {
			require.NoError(t, ps.UpdatePermissions(&models.Permissions{UserID: &u.ID, Scopes: global}))
		}
		return u
	}

	cases := map[string]struct {
		user *models.User
		want bool
	}{
		"customer with the default role": {newUser("customer", &customerRole.ID, scopes.ScopeLogin), false},
		"staff who can edit settings":    {newUser("staff", &customerRole.ID, scopes.ScopeLogin, scopes.ScopeSettingsEdit), true},
		"staff who can edit permissions": {newUser("perms", nil, scopes.ScopeLogin, scopes.ScopeUserPermsEdit), true},
		"administrator":                  {newUser("admin", nil, scopes.ScopeAdmin), true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := userIsPrivileged(db, tc.user)
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// Docker treats 0 as "no limit": a parent whose plan is fully split keeps the smallest real
// limit instead of getting the whole node.
func TestParentLimitsData_NeverZero(t *testing.T) {
	assert.Equal(t, map[string]interface{}{"cpu": 1, "memory": int64(1), "disk": int64(1)}, parentLimitsData(0, -5, 0))
	assert.Equal(t, map[string]interface{}{"cpu": 150, "memory": int64(2048), "disk": int64(4096)}, parentLimitsData(150, 2048, 4096))
}

// A parent edited by an admin keeps only what its sub-servers leave free on its node, and
// only the fields that were sent are rewritten.
func TestLimitsForParentNode(t *testing.T) {
	body := map[string]interface{}{"memory": 4096, "motd": "hi"}
	limitsForParentNode(body, &models.Server{TotalCPU: 200, TotalMemory: 4096, TotalDisk: 10240}, &resourceUse{cpu: 100, memory: 1024, disk: 2048})
	assert.Equal(t, int64(3072), body["memory"])
	assert.Equal(t, "hi", body["motd"])
	_, hasCPU := body["cpu"]
	assert.False(t, hasCPU, "a field that was not sent is not added")
}

func TestChildrenResourceUse(t *testing.T) {
	db := setupCloneTest(t)
	parent := "parent01"
	for _, child := range []*models.Server{
		{Name: "c1", Identifier: "child01", Type: "generic", ParentServerID: &parent, TotalCPU: 50, TotalMemory: 512, TotalDisk: 1024},
		{Name: "c2", Identifier: "child02", Type: "generic", ParentServerID: &parent, TotalCPU: 25, TotalMemory: 256, TotalDisk: 512},
	} {
		require.NoError(t, db.Create(child).Error)
	}

	use, err := childrenResourceUse(db, parent)
	require.NoError(t, err)
	assert.Equal(t, &resourceUse{cpu: 75, memory: 768, disk: 1536}, use)

	none, err := childrenResourceUse(db, "lonely01")
	require.NoError(t, err)
	assert.Nil(t, none)
}
