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

func TestRestrictSubServerCreation_RejectsHostEnvironments(t *testing.T) {
	ps, user := setupSubServerTest(t)

	for _, env := range []string{"host", "tty", "standard", "bubblewrap", ""} {
		err := restrictSubServerCreation(ps, user, subServerBody("parent01", skypanel.MetadataType{Type: env}))
		assert.Error(t, err, "environment %q must be rejected", env)
	}
}

func TestRestrictSubServerCreation_StripsHostAccessFromDocker(t *testing.T) {
	ps, user := setupSubServerTest(t)
	body := subServerBody("parent01", skypanel.MetadataType{
		Type: "docker",
		Metadata: map[string]interface{}{
			"image":        "eclipse-temurin:21",
			"portBindings": []interface{}{"25565:25565"},
			"hostConfig":   map[string]interface{}{"Privileged": true},
			"bindings":     map[string]interface{}{"/": "/host"},
			"networkName":  "host",
		},
	})

	err := restrictSubServerCreation(ps, user, body)

	assert.NoError(t, err)
	assert.Equal(t, "docker", body.Environment.Type)
	assert.Equal(t, "eclipse-temurin:21", body.Environment.Metadata["image"])
	assert.Contains(t, body.Environment.Metadata, "portBindings")
	assert.NotContains(t, body.Environment.Metadata, "hostConfig")
	assert.NotContains(t, body.Environment.Metadata, "bindings")
	assert.NotContains(t, body.Environment.Metadata, "networkName")
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
