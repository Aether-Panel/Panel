package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useScratchNodeSettings points the config at an empty file of this test and puts every synced
// setting back afterwards, so tests that save them leave nothing behind
func useScratchNodeSettings(t *testing.T) string {
	previousFile := viper.ConfigFileUsed()
	previous := NodeSettingsValues()
	file := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(file, []byte("{}"), 0600))
	viper.SetConfigFile(file)
	t.Cleanup(func() {
		viperLock.Lock()
		for key, value := range previous {
			viper.Set(key, value)
		}
		viperLock.Unlock()
		viper.SetConfigFile(previousFile)
	})
	return file
}

// a full set of valid values, different from every default
func sampleNodeSettings() map[string]interface{} {
	return map[string]interface{}{
		BackupsS3Enabled.Key():       true,
		BackupsS3Endpoint.Key():      "s3.example.com",
		BackupsS3Region.Key():        "eu-west-1",
		BackupsS3Bucket.Key():        "copies",
		BackupsS3AccessKey.Key():     "AKIA-TEST",
		BackupsS3SecretKey.Key():     "very-secret",
		BackupsS3Prefix.Key():        "panel-backups",
		BackupsS3Insecure.Key():      true,
		BackupsS3PeriodicHours.Key(): 24,
		BackupsS3PeriodicKeep.Key():  7,
		CurseForgeKey.Key():          "cf-key",
	}
}

func TestSyncedNodeSettings_AreTheS3AndCurseForgeKeys(t *testing.T) {
	keys := make([]string, 0, len(SyncedNodeSettings))
	for _, setting := range SyncedNodeSettings {
		keys = append(keys, setting.Key())
	}
	assert.ElementsMatch(t, []string{
		"daemon.backups.s3.enabled",
		"daemon.backups.s3.endpoint",
		"daemon.backups.s3.region",
		"daemon.backups.s3.bucket",
		"daemon.backups.s3.accessKey",
		"daemon.backups.s3.secretKey",
		"daemon.backups.s3.prefix",
		"daemon.backups.s3.insecure",
		"daemon.backups.s3.periodicHours",
		"daemon.backups.s3.periodicKeep",
		"daemon.curseforge.key",
	}, keys)

	assert.True(t, IsSyncedNodeSetting("daemon.backups.s3.secretKey"))
	assert.False(t, IsSyncedNodeSetting("panel.settings.companyName"))
}

func TestNodeSettingsValues_ReadsEveryKeyWithItsType(t *testing.T) {
	useScratchNodeSettings(t)
	require.NoError(t, ApplyNodeSettings(sampleNodeSettings()))

	assert.Equal(t, sampleNodeSettings(), NodeSettingsValues())
}

func TestNodeSettingsHash_StableAndFollowsEveryValue(t *testing.T) {
	values := sampleNodeSettings()
	hash := NodeSettingsHash(values)
	assert.Len(t, hash, 64, "sha256 in hex")

	// the same values built in another order give the same hash
	reordered := map[string]interface{}{}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	for i := len(keys) - 1; i >= 0; i-- {
		reordered[keys[i]] = values[keys[i]]
	}
	for i := 0; i < 20; i++ {
		assert.Equal(t, hash, NodeSettingsHash(reordered))
	}

	// and any one value changing changes it
	changed := map[string]interface{}{
		BackupsS3Enabled.Key():       false,
		BackupsS3Endpoint.Key():      "other.example.com",
		BackupsS3Region.Key():        "us-east-1",
		BackupsS3Bucket.Key():        "other",
		BackupsS3AccessKey.Key():     "AKIA-OTHER",
		BackupsS3SecretKey.Key():     "other-secret",
		BackupsS3Prefix.Key():        "other-prefix",
		BackupsS3Insecure.Key():      false,
		BackupsS3PeriodicHours.Key(): 48,
		BackupsS3PeriodicKeep.Key():  8,
		CurseForgeKey.Key():          "cf-other",
	}
	for key, value := range changed {
		edited := sampleNodeSettings()
		edited[key] = value
		assert.NotEqual(t, hash, NodeSettingsHash(edited), key)
	}
}

// the hash of what a node decoded from JSON matches the hash of the panel's typed values, so
// a node that took the panel's settings reports the panel's own hash
func TestNodeSettingsHash_SameAfterAJSONRoundTrip(t *testing.T) {
	values := sampleNodeSettings()
	decoded := map[string]interface{}{}
	for key, value := range values {
		if n, ok := value.(int); ok {
			decoded[key] = float64(n)
			continue
		}
		decoded[key] = value
	}
	parsed, err := ParseNodeSettings(decoded)
	require.NoError(t, err)
	assert.Equal(t, NodeSettingsHash(values), NodeSettingsHash(parsed))
}

func TestParseNodeSettings_RefusesAnythingButTheFullList(t *testing.T) {
	var settingErr *NodeSettingError

	unknown := sampleNodeSettings()
	unknown["panel.settings.companyName"] = "x"
	_, err := ParseNodeSettings(unknown)
	require.True(t, errors.As(err, &settingErr), "unknown key")
	assert.Equal(t, "panel.settings.companyName", settingErr.Key)

	missing := sampleNodeSettings()
	delete(missing, BackupsS3Bucket.Key())
	_, err = ParseNodeSettings(missing)
	require.True(t, errors.As(err, &settingErr), "missing key")
	assert.Equal(t, BackupsS3Bucket.Key(), settingErr.Key)

	for key, wrong := range map[string]interface{}{
		BackupsS3Enabled.Key():       "yes",
		BackupsS3Endpoint.Key():      12,
		BackupsS3PeriodicHours.Key(): 24.5,
		BackupsS3PeriodicKeep.Key():  true,
		CurseForgeKey.Key():          nil,
	} {
		bad := sampleNodeSettings()
		bad[key] = wrong
		_, err = ParseNodeSettings(bad)
		require.True(t, errors.As(err, &settingErr), key)
		assert.Equal(t, key, settingErr.Key)
		assert.NotContains(t, err.Error(), "very-secret", "the message never carries a value")
	}

	parsed, err := ParseNodeSettings(sampleNodeSettings())
	require.NoError(t, err)
	assert.Equal(t, sampleNodeSettings(), parsed)
}

func TestApplyNodeSettings_SavesToTheConfigFile(t *testing.T) {
	file := useScratchNodeSettings(t)
	require.NoError(t, ApplyNodeSettings(sampleNodeSettings()))

	saved := viper.New()
	saved.SetConfigFile(file)
	require.NoError(t, saved.ReadInConfig())
	assert.Equal(t, "copies", saved.GetString(BackupsS3Bucket.Key()))
	assert.Equal(t, 24, saved.GetInt(BackupsS3PeriodicHours.Key()))
	assert.True(t, saved.GetBool(BackupsS3Enabled.Key()))

	// a key outside the list is refused before anything is set
	bad := sampleNodeSettings()
	bad[BackupsS3Bucket.Key()] = "changed"
	bad["panel.token"] = "x"
	assert.Error(t, ApplyNodeSettings(bad))
	assert.Equal(t, "copies", BackupsS3Bucket.Value())
}

func TestWholeNumber_ReadsNumbersAsJSONSendsThem(t *testing.T) {
	for value, want := range map[interface{}]int{7: 7, float64(48): 48, "10": 10, " 010 ": 10} {
		n, ok := WholeNumber(value)
		assert.True(t, ok, "%v", value)
		assert.Equal(t, want, n, "%v", value)
	}
	for _, value := range []interface{}{24.5, "abc", true, nil} {
		_, ok := WholeNumber(value)
		assert.False(t, ok, "%v", value)
	}
}
