package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm/clause"
)

func TestAssignServerPermissions_ASplitKeepsEachMembersOwnScopes(t *testing.T) {
	ps, owner := setupSubServerTest(t)
	viewer := &models.User{Username: "viewer", Email: "viewer@example.com", HashedPassword: "x"}
	assert.NoError(t, ps.DB.Create(viewer).Error)
	parent := "parent01"
	consoleOnly := []*scopes.Scope{scopes.ScopeServerView, scopes.ScopeServerConsole}
	assert.NoError(t, ps.UpdatePermissions(&models.Permissions{UserID: &viewer.ID, ServerIdentifier: &parent, Scopes: consoleOnly}))

	assert.NoError(t, assignServerPermissions(ps, "child01", []*models.User{owner, viewer}, &parent))

	row, err := ps.GetForUserAndServer(viewer.ID, "child01")
	assert.NoError(t, err)
	assert.ElementsMatch(t, consoleOnly, row.Scopes, "no more than on the parent")
	allowed, err := ps.HasPermission(viewer.ID, "child01", scopes.ScopeServerFileEdit)
	assert.NoError(t, err)
	assert.False(t, allowed)
}

func TestOwnerOfNewServer_ASplitOfAServerWithoutOwnerHasNone(t *testing.T) {
	ps, member := setupSubServerTest(t)
	parent := &models.Server{Identifier: "parent01", Name: "parent01", Type: "generic"}
	assert.NoError(t, ps.DB.Omit(clause.Associations).Create(parent).Error)
	admin := &models.User{Username: "admin", Email: "admin@example.com", HashedPassword: "x"}
	assert.NoError(t, ps.DB.Create(admin).Error)

	owner, err := ownerOfNewServer(ps.DB, admin, []*models.User{member}, &parent.Identifier)
	assert.NoError(t, err)
	assert.Nil(t, owner, "a parent member picked in no set order is not made the owner")
}

func TestRequestServer_ACallThatNamesNoServerIsABadRequest(t *testing.T) {
	ps, _ := setupSubServerTest(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	server, ok := requestServer(c, ps.DB, ProvisionActionRequest{}, &services.Server{DB: ps.DB})

	assert.False(t, ok)
	assert.Nil(t, server)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRequestServer_AnUnknownServerIsNotFound(t *testing.T) {
	ps, _ := setupSubServerTest(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	_, ok := requestServer(c, ps.DB, ProvisionActionRequest{ServiceID: "whmcs-42"}, &services.Server{DB: ps.DB})

	assert.False(t, ok)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
