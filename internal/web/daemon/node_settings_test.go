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
	keys := make([]interface{}, 0, len(config.SyncedNodeSettings))
	for _, setting := range config.SyncedNodeSettings {
		keys = append(keys, setting.Key())
	}
	assert.Equal(t, map[string]interface{}{
		"hash": config.NodeSettingsHash(config.NodeSettingsValues()),
		"keys": keys,
	}, body)
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
	var state skypanel.NodeSettingsState
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &state))
	assert.Equal(t, config.NodeSettingsHashFor(panelNodeSettings(), state.Keys), state.Hash)
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
		"wrong type": {func(p map[string]interface{}) { p[config.BackupsS3Enabled.Key()] = "yes" }, "ErrNodeSettingInvalid"},
		"hours 25":   {func(p map[string]interface{}) { p[config.BackupsS3PeriodicHours.Key()] = 25 }, "ErrOffsitePeriodicHours"},
		"keep 0":     {func(p map[string]interface{}) { p[config.BackupsS3PeriodicKeep.Key()] = 0 }, "ErrOffsiteKeep"},
	}
	for name, tc := range cases {
		payload := panelNodeSettings()
		// next to a key this node does not know, which alone would be ignored
		payload["daemon.future.setting"] = "x"
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

// a newer panel sends keys this node does not know yet: they are ignored and the rest applied
func TestNodeSettings_PutIgnoresKeysThisNodeDoesNotKnow(t *testing.T) {
	router := nodeSettingsRouter(t)
	payload := panelNodeSettings()
	payload["daemon.future.setting"] = "x"
	payload["panel.token"] = "not-for-nodes"

	w := callNodeSettings(router, http.MethodPut, panelToken(t, nil), payload)

	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.Equal(t, "copies", config.BackupsS3Bucket.Value())
	assert.Equal(t, 24, config.BackupsS3PeriodicHours.Value())
	assert.Nil(t, viper.Get("daemon.future.setting"), "an unknown key is not stored")
	assert.NotEqual(t, "not-for-nodes", config.PrivateKey.Value())
}

// an older panel sends fewer keys: only those change, the rest stay as they are
func TestNodeSettings_PutAppliesOnlyTheKeysSent(t *testing.T) {
	router := nodeSettingsRouter(t)
	token := panelToken(t, nil)
	require.NoError(t, config.CurseForgeKey.Set("node-own-key", false))
	require.NoError(t, config.BackupsS3PeriodicKeep.Set(9, false))

	w := callNodeSettings(router, http.MethodPut, token, map[string]interface{}{
		config.BackupsS3Bucket.Key():        "copies",
		config.BackupsS3PeriodicHours.Key(): 48,
	})

	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.Equal(t, "copies", config.BackupsS3Bucket.Value())
	assert.Equal(t, 48, config.BackupsS3PeriodicHours.Value())
	assert.Equal(t, "node-own-key", config.CurseForgeKey.Value(), "a key not sent is left alone")
	assert.Equal(t, 9, config.BackupsS3PeriodicKeep.Value())

	// nothing this node knows: nothing changes, and that is not an error
	for _, body := range []map[string]interface{}{{}, {"daemon.future.setting": "x"}} {
		w = callNodeSettings(router, http.MethodPut, token, body)
		assert.Equal(t, http.StatusNoContent, w.Code, "%v", body)
	}
	assert.Equal(t, "copies", config.BackupsS3Bucket.Value())
	assert.Equal(t, "node-own-key", config.CurseForgeKey.Value())
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
