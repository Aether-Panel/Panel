package api

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupCloneTest(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "clone_test.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Role{}, &models.Permissions{}, &models.Client{}, &models.Node{}, &models.Server{}))
	return db
}

func cloneBool(b bool) *bool { return &b }

func TestCloneRequestNormalize(t *testing.T) {
	tests := []struct {
		name      string
		req       cloneServerRequest
		wantName  string
		wantFiles bool
		wantErr   bool
	}{
		{name: "trims and copies files by default", req: cloneServerRequest{Name: "  Survival (copia) "}, wantName: "Survival (copia)", wantFiles: true},
		{name: "files can be left out", req: cloneServerRequest{Name: "Test", CopyFiles: cloneBool(false)}, wantName: "Test", wantFiles: false},
		{name: "files asked explicitly", req: cloneServerRequest{Name: "Test", CopyFiles: cloneBool(true)}, wantName: "Test", wantFiles: true},
		{name: "empty name", req: cloneServerRequest{Name: "   "}, wantErr: true},
		{name: "40 characters fit", req: cloneServerRequest{Name: strings.Repeat("a", 40)}, wantName: strings.Repeat("a", 40), wantFiles: true},
		{name: "41 characters do not", req: cloneServerRequest{Name: strings.Repeat("a", 41)}, wantErr: true},
		{name: "accents are refused like on servers.name", req: cloneServerRequest{Name: "Mundo ñ"}, wantErr: true},
		{name: "control characters", req: cloneServerRequest{Name: "a\tb"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, files, err := tt.req.normalize()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, name)
			assert.Equal(t, tt.wantFiles, files)
		})
	}
}

func TestAssignClonePorts_AvoidsUsedPortsAndTheOriginal(t *testing.T) {
	db := setupCloneTest(t)
	source := &models.Server{Identifier: "src", Name: "src", Type: "minecraft-java", Port: 25565, Ports: []uint16{25565, 25570}}
	require.NoError(t, db.Create(source).Error)
	// the next two ports after the original are taken by another server (one only in Ports)
	require.NoError(t, db.Create(&models.Server{Identifier: "other", Name: "other", Type: "generic", Port: 25566, Ports: []uint16{25566, 25567}}).Error)

	variables := map[string]skypanel.Variable{
		"port":       {Value: float64(25565)},
		"port2":      {Value: "25570"},
		"query_port": {Value: float64(25565)},
		"rconport":   {Value: float64(25575)},
		"memory":     {Value: float64(2048)},
		"export":     {Value: float64(30000)},
	}
	node := &models.Node{ID: 0, PublicPort: 8080, PrivatePort: 8080, SFTPPort: 5657}

	mapping, err := assignClonePorts(db, node, variables)
	require.NoError(t, err)

	assert.Equal(t, 25568, variables["port"].Value, "first free port after the original's")
	assert.Equal(t, "25571", variables["port2"].Value, "text values stay text")
	assert.Equal(t, 25568, variables["query_port"].Value, "a port shared with the main one stays shared")
	assert.Equal(t, 25576, variables["rconport"].Value)
	assert.Equal(t, float64(2048), variables["memory"].Value, "other variables are untouched")
	assert.Equal(t, float64(30000), variables["export"].Value, "names that merely end in 'port' are not ports")
	assert.Equal(t, map[uint16]uint16{25565: 25568, 25570: 25571, 25575: 25576}, mapping)

	used, err := usedPortsOnNode(db, 0)
	require.NoError(t, err)
	for _, p := range mapping {
		assert.False(t, used[p], "port %d is already used on the node", p)
	}
}

func TestAssignClonePorts_SkipsTheNodesOwnPorts(t *testing.T) {
	db := setupCloneTest(t)
	variables := map[string]skypanel.Variable{"port": {Value: float64(8079)}}
	node := &models.Node{ID: 0, PublicPort: 8080, PrivatePort: 8081, SFTPPort: 8082}

	mapping, err := assignClonePorts(db, node, variables)
	require.NoError(t, err)
	assert.Equal(t, uint16(8083), mapping[8079])
}

func TestAssignClonePorts_LowPortsMoveIntoTheUserRange(t *testing.T) {
	db := setupCloneTest(t)
	variables := map[string]skypanel.Variable{"port": {Value: float64(80)}}

	mapping, err := assignClonePorts(db, &models.Node{}, variables)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, mapping[80], uint16(1024))
}

func TestRemapPortBindings(t *testing.T) {
	env := skypanel.MetadataType{Type: "docker", Metadata: map[string]interface{}{
		"portBindings": []interface{}{"0.0.0.0:25565:25565/tcp", "${port2}:${port2}/udp", "0.0.0.0:9000:9000"},
	}}
	remapPortBindings(&env, map[uint16]uint16{25565: 25568, 2: 9})

	assert.Equal(t, []interface{}{"0.0.0.0:25568:25568/tcp", "${port2}:${port2}/udp", "0.0.0.0:9000:9000"}, env.Metadata["portBindings"])
}

func TestRemapPortNotes(t *testing.T) {
	notes := remapPortNotes(map[string]string{"25565": "Java", "19132": "Bedrock", "x": "bad"}, map[uint16]uint16{25565: 25568})
	assert.Equal(t, map[string]string{"25568": "Java"}, notes)
	assert.Nil(t, remapPortNotes(nil, nil))
}

func TestCopyServerPermissions_KeepsEachUsersScopes(t *testing.T) {
	db := setupCloneTest(t)
	owner := &models.User{Username: "owner", Email: "owner@example.com", HashedPassword: "x"}
	helper := &models.User{Username: "helper", Email: "helper@example.com", HashedPassword: "x"}
	require.NoError(t, db.Create(owner).Error)
	require.NoError(t, db.Create(helper).Error)

	source := "src"
	clientID := uint(7)
	require.NoError(t, db.Create(&models.Permissions{UserID: &owner.ID, ServerIdentifier: &source, Scopes: []*scopes.Scope{scopes.ScopeServerView, scopes.ScopeServerStart, scopes.ScopeServerFileEdit}}).Error)
	require.NoError(t, db.Create(&models.Permissions{UserID: &helper.ID, ServerIdentifier: &source, Scopes: []*scopes.Scope{scopes.ScopeServerView, scopes.ScopeServerConsole}}).Error)
	require.NoError(t, db.Create(&models.Permissions{ClientID: &clientID, ServerIdentifier: &source, Scopes: []*scopes.Scope{scopes.ScopeServerSftp}}).Error)

	require.NoError(t, copyServerPermissions(db, source, "copy"))

	var copied []*models.Permissions
	require.NoError(t, db.Where("server_identifier = ?", "copy").Find(&copied).Error)
	require.Len(t, copied, 2, "only user rows are copied, not OAuth clients")

	byUser := map[uint][]string{}
	for _, p := range copied {
		require.NotNil(t, p.UserID)
		for _, s := range p.Scopes {
			byUser[*p.UserID] = append(byUser[*p.UserID], s.String())
		}
	}
	assert.ElementsMatch(t, []string{"server.view", "server.start", "server.files.edit"}, byUser[owner.ID])
	assert.ElementsMatch(t, []string{"server.view", "server.console"}, byUser[helper.ID])

	var original int64
	db.Model(&models.Permissions{}).Where("server_identifier = ?", source).Count(&original)
	assert.Equal(t, int64(3), original, "the original keeps its rows")
}

func TestNewCloneCreation_KeepsTheOriginalUntouched(t *testing.T) {
	parent := "parent01"
	source := &models.Server{Identifier: "src", Name: "Survival", NodeID: 3, ParentServerID: &parent, Subdomain: "survival", ExternalID: "svc-1"}
	definition := &skypanel.Server{Identifier: "src", Variables: map[string]skypanel.Variable{"port": {Value: float64(25565)}}}

	body := newCloneCreation(definition, source, "NEWID", "Survival (copia)")
	body.Variables["port"] = skypanel.Variable{Value: 1}

	assert.Equal(t, "NEWID", body.Identifier)
	assert.Equal(t, "Survival (copia)", body.Name)
	assert.Equal(t, uint(3), body.NodeID)
	require.NotNil(t, body.ParentServerID)
	assert.Equal(t, "parent01", *body.ParentServerID)
	assert.NotSame(t, source.ParentServerID, body.ParentServerID)
	assert.Empty(t, body.Users)
	assert.Equal(t, float64(25565), definition.Variables["port"].Value, "the definition read from the node is not changed")
}

func TestAuthorizeClone(t *testing.T) {
	db := setupCloneTest(t)
	ps := &services.Permission{DB: db}
	member := &models.User{Username: "member", Email: "member@example.com", HashedPassword: "x"}
	stranger := &models.User{Username: "stranger", Email: "stranger@example.com", HashedPassword: "x"}
	admin := &models.User{Username: "admin", Email: "admin@example.com", HashedPassword: "x"}
	for _, u := range []*models.User{member, stranger, admin} {
		require.NoError(t, db.Create(u).Error)
	}
	source := &models.Server{Identifier: "src", Name: "src", Type: "generic"}
	require.NoError(t, ps.UpdatePermissions(&models.Permissions{UserID: &member.ID, ServerIdentifier: &source.Identifier, Scopes: []*scopes.Scope{scopes.ScopeServerView}}))
	require.NoError(t, ps.UpdatePermissions(&models.Permissions{UserID: &admin.ID, Scopes: []*scopes.Scope{scopes.ScopeAdmin}}))

	isAdmin, err := authorizeClone(ps, admin, source)
	assert.NoError(t, err)
	assert.True(t, isAdmin)

	isAdmin, err = authorizeClone(ps, member, source)
	assert.NoError(t, err)
	assert.False(t, isAdmin)

	_, err = authorizeClone(ps, stranger, source)
	assert.ErrorIs(t, err, errCloneNotMember, "server.create alone does not open someone else's server")

	source.Suspended = true
	_, err = authorizeClone(ps, member, source)
	assert.ErrorIs(t, err, errCloneSuspended)
	_, err = authorizeClone(ps, admin, source)
	assert.NoError(t, err, "admins may still copy a suspended server")
}
