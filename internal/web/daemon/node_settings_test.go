package daemon

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nodeSettingsRouter serves the daemon's routes as a node does, writing its settings to an
// empty config file of this test; every synced setting is put back afterwards
func nodeSettingsRouter(t *testing.T) *gin.Engine {
	gin.SetMode(gin.TestMode)
	previousFile := viper.ConfigFileUsed()
	previous := config.NodeSettingsValues()
	file := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(file, []byte("{}"), 0600))
	viper.SetConfigFile(file)
	t.Cleanup(func() {
		viper.SetConfigFile(file)
		_ = config.ApplyNodeSettings(previous)
		viper.SetConfigFile(previousFile)
	})

	router := gin.New()
	RegisterDaemonRoutes(router.Group("/daemon"))
	return router
}

// panelToken is a token the panel signs for this node (the local one, as in a test there is
// no node secret configured)
func panelToken(t *testing.T, node *models.Node) string {
	ts, err := services.NewTokenService()
	require.NoError(t, err)
	token, err := ts.GenerateRequest(node)
	require.NoError(t, err)
	return token
}

func callNodeSettings(router *gin.Engine, method, token string, body interface{}) *httptest.ResponseRecorder {
	var data []byte
	if body != nil {
		data, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, "/daemon/settings", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// what the panel sends: every synced key, as JSON carries it
func panelNodeSettings() map[string]interface{} {
	return map[string]interface{}{
		config.BackupsS3Enabled.Key():       true,
		config.BackupsS3Endpoint.Key():      "s3.example.com",
		config.BackupsS3Region.Key():        "eu-west-1",
		config.BackupsS3Bucket.Key():        "copies",
		config.BackupsS3AccessKey.Key():     "AKIA-TEST",
		config.BackupsS3SecretKey.Key():     "very-secret",
		config.BackupsS3Prefix.Key():        "panel-backups",
		config.BackupsS3Insecure.Key():      false,
		config.BackupsS3PeriodicHours.Key(): 24,
		config.BackupsS3PeriodicKeep.Key():  7,
		config.CurseForgeKey.Key():          "cf-key",
	}
}

func TestNodeSettings_GetAnswersOnlyTheHash(t *testing.T) {
	router := nodeSettingsRouter(t)
	require.NoError(t, config.BackupsS3SecretKey.Set("very-secret", false))

	w := callNodeSettings(router, http.MethodGet, panelToken(t, nil), nil)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, map[string]interface{}{"hash": config.NodeSettingsHash(config.NodeSettingsValues())}, body)
	assert.NotContains(t, w.Body.String(), "very-secret")
}

func TestNodeSettings_PutAppliesAndSavesEveryValue(t *testing.T) {
	router := nodeSettingsRouter(t)
	token := panelToken(t, nil)

	w := callNodeSettings(router, http.MethodPut, token, panelNodeSettings())

	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.True(t, config.BackupsS3Enabled.Value())
	assert.Equal(t, "copies", config.BackupsS3Bucket.Value())
	assert.Equal(t, "very-secret", config.BackupsS3SecretKey.Value())
	assert.Equal(t, 24, config.BackupsS3PeriodicHours.Value())
	assert.Equal(t, 7, config.BackupsS3PeriodicKeep.Value())
	assert.Equal(t, "cf-key", config.CurseForgeKey.Value())

	saved := viper.New()
	saved.SetConfigFile(viper.ConfigFileUsed())
	require.NoError(t, saved.ReadInConfig())
	assert.Equal(t, "copies", saved.GetString(config.BackupsS3Bucket.Key()), "written to the node's config.json")

	// the node now reports the hash the panel computes from its own values
	w = callNodeSettings(router, http.MethodGet, token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"hash":"`+config.NodeSettingsHash(panelNodeSettings())+`"}`, w.Body.String())
}

func TestNodeSettings_PutRefusesTheWholePackageOnABadValue(t *testing.T) {
	router := nodeSettingsRouter(t)
	token := panelToken(t, nil)
	require.NoError(t, config.BackupsS3Bucket.Set("before", false))
	require.NoError(t, config.BackupsS3PeriodicHours.Set(0, false))
	require.NoError(t, config.BackupsS3PeriodicKeep.Set(4, false))

	cases := map[string]struct {
		edit func(map[string]interface{})
		code string
	}{
		"unknown key": {func(p map[string]interface{}) { p["panel.token"] = "x" }, "ErrNodeSettingInvalid"},
		"missing key": {func(p map[string]interface{}) { delete(p, config.CurseForgeKey.Key()) }, "ErrNodeSettingInvalid"},
		"wrong type":  {func(p map[string]interface{}) { p[config.BackupsS3Enabled.Key()] = "yes" }, "ErrNodeSettingInvalid"},
		"hours 25":    {func(p map[string]interface{}) { p[config.BackupsS3PeriodicHours.Key()] = 25 }, "ErrOffsitePeriodicHours"},
		"keep 0":      {func(p map[string]interface{}) { p[config.BackupsS3PeriodicKeep.Key()] = 0 }, "ErrOffsiteKeep"},
	}
	for name, tc := range cases {
		payload := panelNodeSettings()
		tc.edit(payload)

		w := callNodeSettings(router, http.MethodPut, token, payload)

		assert.Equal(t, http.StatusBadRequest, w.Code, name)
		var body skypanel.ErrorResponse
		if assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), name) && assert.NotNil(t, body.Error, name) {
			assert.Equal(t, tc.code, body.Error.Code, name)
		}
		assert.NotContains(t, w.Body.String(), "very-secret", name)
		assert.Equal(t, "before", config.BackupsS3Bucket.Value(), "%s: nothing applied", name)
		assert.Equal(t, 0, config.BackupsS3PeriodicHours.Value(), name)
		assert.Equal(t, 4, config.BackupsS3PeriodicKeep.Value(), name)
	}

	w := callNodeSettings(router, http.MethodPut, token, []string{"not", "an", "object"})
	assert.Equal(t, http.StatusBadRequest, w.Code, "not an object")
}

// only the panel, with a token signed for this node, reads or writes these settings
func TestNodeSettings_RefusesRequestsWithoutThePanelsToken(t *testing.T) {
	router := nodeSettingsRouter(t)
	require.NoError(t, config.BackupsS3Bucket.Set("before", false))
	otherNode := panelToken(t, &models.Node{ID: 5, Name: "other", Secret: "another-node-secret"})

	for _, token := range []string{"", "not-a-jwt", otherNode} {
		assert.Equal(t, http.StatusUnauthorized, callNodeSettings(router, http.MethodGet, token, nil).Code)
		assert.Equal(t, http.StatusUnauthorized, callNodeSettings(router, http.MethodPut, token, panelNodeSettings()).Code)
	}
	assert.Equal(t, "before", config.BackupsS3Bucket.Value())
}
