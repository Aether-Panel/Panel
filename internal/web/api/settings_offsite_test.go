package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useScratchConfig points the settings at an empty config file of this test, so saving them
// writes there
func useScratchConfig(t *testing.T) {
	previousFile := viper.ConfigFileUsed()
	file := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(file, []byte("{}"), 0600))
	viper.SetConfigFile(file)
	hours, keep := config.BackupsS3PeriodicHours.Value(), config.BackupsS3PeriodicKeep.Value()
	t.Cleanup(func() {
		_ = config.BackupsS3PeriodicHours.Set(hours, false)
		_ = config.BackupsS3PeriodicKeep.Set(keep, false)
		viper.SetConfigFile(previousFile)
	})
	require.NoError(t, config.BackupsS3PeriodicHours.Set(0, false))
	require.NoError(t, config.BackupsS3PeriodicKeep.Set(4, false))
}

func TestSettings_RejectsAnInvalidPeriodicInterval(t *testing.T) {
	useScratchConfig(t)
	hours := config.BackupsS3PeriodicHours.Key()
	keep := config.BackupsS3PeriodicKeep.Key()

	assert.ErrorIs(t, validateSetting(hours, 25), skypanel.ErrOffsitePeriodicHours)
	assert.ErrorIs(t, validateSetting(hours, float64(25)), skypanel.ErrOffsitePeriodicHours, "a number from JSON")
	assert.ErrorIs(t, validateSetting(hours, "abc"), skypanel.ErrOffsitePeriodicHours, "not a number is not 0")
	assert.ErrorIs(t, validateSetting(hours, 24.5), skypanel.ErrOffsitePeriodicHours)
	assert.ErrorIs(t, validateSetting(keep, 0), skypanel.ErrOffsiteKeep)
	assert.ErrorIs(t, validateSetting(keep, float64(51)), skypanel.ErrOffsiteKeep)
	assert.Equal(t, 0, config.BackupsS3PeriodicHours.Value(), "nothing is saved")

	require.NoError(t, validateSetting(hours, float64(48)))
	require.NoError(t, applyEditableSetting(hours, float64(48)))
	require.NoError(t, validateSetting(keep, "10"))
	require.NoError(t, applyEditableSetting(keep, "10"))
	assert.Equal(t, 48, config.BackupsS3PeriodicHours.Value())
	assert.Equal(t, 10, config.BackupsS3PeriodicKeep.Value())

	saved := viper.New()
	saved.SetConfigFile(viper.ConfigFileUsed())
	require.NoError(t, saved.ReadInConfig())
	assert.Equal(t, 48, saved.GetInt(hours), "written to the config file")
	assert.Equal(t, 10, saved.GetInt(keep))
}

func TestSettings_PeriodicValuesAreEditableIntegers(t *testing.T) {
	useScratchConfig(t)
	require.NoError(t, config.BackupsS3PeriodicHours.Set(72, false))
	require.NoError(t, config.BackupsS3PeriodicKeep.Set(7, false))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "key", Value: config.BackupsS3PeriodicHours.Key()}}
	getSetting(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"value":72}`, w.Body.String())

	editable := map[string]bool{}
	for _, v := range editableIntEntries {
		editable[v.Key()] = true
	}
	assert.True(t, editable[config.BackupsS3PeriodicHours.Key()])
	assert.True(t, editable[config.BackupsS3PeriodicKeep.Key()])
}

func TestSetSetting_AnInvalidPeriodicIntervalIsA400(t *testing.T) {
	useScratchConfig(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "key", Value: config.BackupsS3PeriodicHours.Key()}}
	body, _ := json.Marshal(map[string]interface{}{"value": 25})
	c.Request = httptest.NewRequest(http.MethodPut, "/api/settings/x", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setSetting(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "ErrOffsitePeriodicHours")
	assert.Equal(t, 0, config.BackupsS3PeriodicHours.Value())
}
