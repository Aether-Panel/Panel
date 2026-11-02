package api

import (
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func setupSubServerTest(t *testing.T) (*services.Permission, *models.User) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "api_test.db")), &gorm.Config{})
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	if !assert.NoError(t, db.AutoMigrate(&models.User{}, &models.Role{}, &models.Permissions{}, &models.Client{}, &models.Server{}, &models.Node{})) {
		t.FailNow()
	}

	user := &models.User{Username: "owner", Email: "owner@example.com", HashedPassword: "x"}
	if !assert.NoError(t, db.Create(user).Error) {
		t.FailNow()
	}

	ps := &services.Permission{DB: db}
	parent := "parent01"
	err = ps.UpdatePermissions(&models.Permissions{
		UserID:           &user.ID,
		ServerIdentifier: &parent,
		Scopes:           []*scopes.Scope{scopes.ScopeServerView},
	})
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	return ps, user
}

// ownParent makes the test user the owner of parent01: splitting is the owner's call
func ownParent(t *testing.T, ps *services.Permission, user *models.User) {
	parent := &models.Server{Identifier: "parent01", Name: "parent01", Type: "generic", OwnerID: &user.ID}
	if !assert.NoError(t, ps.DB.Omit(clause.Associations).Create(parent).Error) {
		t.FailNow()
	}
}

func subServerBody(parent string, env skypanel.MetadataType) *models.ServerCreation {
	body := &models.ServerCreation{}
	if parent != "" {
		body.ParentServerID = &parent
	}
	body.Environment = env
	return body
}

func TestRestrictSubServerCreation_RequiresParent(t *testing.T) {
	ps, user := setupSubServerTest(t)

	err := restrictSubServerCreation(ps, user, subServerBody("", skypanel.MetadataType{Type: "docker"}))

	assert.Error(t, err)
}

func TestRestrictSubServerCreation_RequiresAccessToParent(t *testing.T) {
	ps, user := setupSubServerTest(t)

	err := restrictSubServerCreation(ps, user, subServerBody("notmine", skypanel.MetadataType{Type: "docker"}))

	assert.Error(t, err)
}

func TestRestrictSubServerCreation_OnlyTheParentsOwnerSplitsIt(t *testing.T) {
	ps, user := setupSubServerTest(t)
	parent := &models.Server{Identifier: "parent01", Name: "parent01", Type: "generic"}
	if !assert.NoError(t, ps.DB.Omit(clause.Associations).Create(parent).Error) {
		t.FailNow()
	}

	err := restrictSubServerCreation(ps, user, subServerBody("parent01", skypanel.MetadataType{Type: "docker"}))

	assert.Error(t, err, "a sub-user of the parent cannot carve servers out of its plan")
}

func TestRestrictSubServerCreation_RejectsHostEnvironments(t *testing.T) {
	ps, user := setupSubServerTest(t)
	ownParent(t, ps, user)

	for _, env := range []string{"host", "tty", "standard", "bubblewrap", ""} {
		err := restrictSubServerCreation(ps, user, subServerBody("parent01", skypanel.MetadataType{Type: env}))
		assert.Error(t, err, "environment %q must be rejected", env)
	}
}

func TestRestrictSubServerCreation_StripsHostAccessFromDocker(t *testing.T) {
	ps, user := setupSubServerTest(t)
	ownParent(t, ps, user)
	body := subServerBody("parent01", skypanel.MetadataType{
		Type: "docker",
		Metadata: map[string]interface{}{
			"image":        "eclipse-temurin:21",
			"portBindings": []interface{}{"0.0.0.0:${port}:${port}/tcp", "${queryport}:${queryport}/udp"},
			"hostConfig":   map[string]interface{}{"Privileged": true},
			"bindings":     map[string]interface{}{"/": "/host"},
			"networkName":  "host",
			"labels":       map[string]interface{}{"traefik.http.routers.x.rule": "Host(`panel.example.com`)"},
			"config":       map[string]interface{}{"User": "0:0", "Env": []interface{}{"A=1"}},
		},
	})
	body.Variables = map[string]skypanel.Variable{"port": {Value: 25565}, "queryport": {Value: 25566}}

	err := restrictSubServerCreation(ps, user, body)

	assert.NoError(t, err)
	assert.Equal(t, "docker", body.Environment.Type)
	assert.Equal(t, "eclipse-temurin:21", body.Environment.Metadata["image"])
	assert.Contains(t, body.Environment.Metadata, "portBindings")
	assert.NotContains(t, body.Environment.Metadata, "hostConfig")
	assert.NotContains(t, body.Environment.Metadata, "bindings")
	assert.NotContains(t, body.Environment.Metadata, "networkName")
	assert.NotContains(t, body.Environment.Metadata, "labels")
	assert.Equal(t, map[string]interface{}{"Env": []interface{}{"A=1"}}, body.Environment.Metadata["config"], "the game never picks its user")
}

func TestRestrictSubServerCreation_PublishesOnlyItsPortVariables(t *testing.T) {
	ps, user := setupSubServerTest(t)
	ownParent(t, ps, user)

	for _, spec := range []string{"0.0.0.0:25565:25565", "25565:25565/udp", "0.0.0.0:${motd}:25565", "127.0.0.1:${port}:${port}"} {
		body := subServerBody("parent01", skypanel.MetadataType{Type: "docker", Metadata: map[string]interface{}{
			"portBindings": []interface{}{spec},
		}})
		body.Variables = map[string]skypanel.Variable{"port": {Value: 25570}, "motd": {Value: "hi"}}
		assert.Error(t, restrictSubServerCreation(ps, user, body), "%q must be refused", spec)
	}
}

func TestLockVariables_OwnersKeepTheirLimitsAndPorts(t *testing.T) {
	current := map[string]skypanel.Variable{
		"memory": {Value: 4096, UserEditable: true}, // a template that opened it by mistake
		"port":   {Value: 25565, UserEditable: true},
		"motd":   {Value: "hello", UserEditable: true},
		"jar":    {Value: "server.jar"},
	}
	wanted := map[string]skypanel.Variable{
		"memory":    {Value: -1, UserEditable: true},
		"port":      {Value: 25570, UserEditable: true},
		"motd":      {Value: "mine", UserEditable: true},
		"jar":       {Value: "evil.jar"},
		"queryport": {Value: 3000},
	}

	got := lockVariables(current, wanted)

	assert.Equal(t, 4096, got["memory"].Value)
	assert.Equal(t, 25565, got["port"].Value)
	assert.Equal(t, "server.jar", got["jar"].Value, "not editable by the owner")
	assert.Equal(t, "mine", got["motd"].Value)
	assert.NotContains(t, got, "queryport", "no new ports either")
}

func TestFilterUptimeToServers(t *testing.T) {
	stats := map[string]map[string]interface{}{
		"mine":   {"uptime": 1},
		"theirs": {"uptime": 2},
	}

	got := filterUptimeToServers(stats, []*models.Server{{Identifier: "mine"}})

	assert.Len(t, got, 1)
	assert.Contains(t, got, "mine")
	assert.NotContains(t, got, "theirs")
}

func TestCheckParentServerLimits_WorksOnSQLite(t *testing.T) {
	ps, _ := setupSubServerTest(t)
	db := ps.DB
	parent := &models.Server{Identifier: "parent01", Name: "parent01", Type: "generic", TotalCPU: 200, TotalMemory: 4096, TotalDisk: 10240}
	if !assert.NoError(t, db.Omit(clause.Associations).Create(parent).Error) {
		t.FailNow()
	}
	parentID := parent.Identifier

	var cpu int
	var memory, disk int64
	child := &models.Server{Identifier: "child01", ParentServerID: &parentID, TotalCPU: 100, TotalMemory: 1024, TotalDisk: 2048}
	err := db.Transaction(func(tx *gorm.DB) error {
		return checkParentServerLimits(tx, child, &cpu, &memory, &disk)
	})

	assert.NoError(t, err, "the raw FOR UPDATE query used to fail with a syntax error on SQLite")
	assert.Equal(t, 100, cpu)
	assert.Equal(t, int64(3072), memory)

	tooBig := &models.Server{Identifier: "child02", ParentServerID: &parentID, TotalCPU: 300}
	err = db.Transaction(func(tx *gorm.DB) error {
		return checkParentServerLimits(tx, tooBig, &cpu, &memory, &disk)
	})
	assert.Error(t, err, "limits are still enforced")
}
