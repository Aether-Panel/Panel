package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// NodeSetting is a config entry the panel edits but every node's daemon uses. It is one of
// StringEntry, BoolEntry or IntEntry; the type decides how its value is read and checked.
type NodeSetting interface {
	Key() string
}

// SyncedNodeSettings are the settings the panel hands to every remote node, so a node needs
// none of them in its own config.json. The panel reads and packs this list and a node accepts
// nothing else from it: a new daemon setting edited in the panel only has to be added here.
var SyncedNodeSettings = []NodeSetting{
	BackupsS3Enabled,
	BackupsS3Endpoint,
	BackupsS3Region,
	BackupsS3Bucket,
	BackupsS3AccessKey,
	BackupsS3SecretKey,
	BackupsS3Prefix,
	BackupsS3Insecure,
	BackupsS3PeriodicHours,
	BackupsS3PeriodicKeep,
	CurseForgeKey,
}

// IsSyncedNodeSetting says whether key is one of SyncedNodeSettings, so saving it in the panel
// has to reach the nodes too
func IsSyncedNodeSetting(key string) bool {
	return syncedNodeSetting(key) != nil
}

func syncedNodeSetting(key string) NodeSetting {
	for _, setting := range SyncedNodeSettings {
		if setting.Key() == key {
			return setting
		}
	}
	return nil
}

// NodeSettingsValues is the current value of every synced setting, keyed by its config key and
// with the entry's own type (string, bool or int). Reading through the entries, and not viper,
// gives the same types whether a value came from the file, the environment or a default, so
// the panel and a node hash equal settings the same way.
func NodeSettingsValues() map[string]interface{} {
	values := make(map[string]interface{}, len(SyncedNodeSettings))
	for _, setting := range SyncedNodeSettings {
		switch entry := setting.(type) {
		case StringEntry:
			values[entry.Key()] = entry.Value()
		case BoolEntry:
			values[entry.Key()] = entry.Value()
		case IntEntry:
			values[entry.Key()] = entry.Value()
		}
	}
	return values
}

// NodeSettingsHash is the sha256 (hex) of the values as canonical JSON: encoding/json writes a
// map's keys sorted, so the hash does not depend on the order they were added in. A node
// answers it instead of the values, which include secrets, and the panel compares it with its
// own to know whether the node needs its settings again. Empty when a value cannot be encoded,
// which never matches a real hash, so the panel then sends the settings anyway.
func NodeSettingsHash(values map[string]interface{}) string {
	data, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// NodeSettingError refuses one key of the settings the panel sent. It names the key and the
// problem but never the value, which may be a secret.
type NodeSettingError struct {
	Key     string
	Problem string
}

func (e *NodeSettingError) Error() string {
	return fmt.Sprintf("node setting %s %s", e.Key, e.Problem)
}

// ParseNodeSettings checks the settings the panel sent a node, as decoded from JSON: exactly
// the keys of SyncedNodeSettings, each with a value of its entry's type. It answers them with
// the entries' own types (numbers as int), ready for ApplyNodeSettings. The first problem
// found is returned and nothing is applied, so a bad package never leaves a node half set.
func ParseNodeSettings(raw map[string]interface{}) (map[string]interface{}, error) {
	for key := range raw {
		if !IsSyncedNodeSetting(key) {
			return nil, &NodeSettingError{Key: key, Problem: "is not one the panel hands to its nodes"}
		}
	}
	values := make(map[string]interface{}, len(SyncedNodeSettings))
	for _, setting := range SyncedNodeSettings {
		value, ok := raw[setting.Key()]
		if !ok {
			return nil, &NodeSettingError{Key: setting.Key(), Problem: "is missing"}
		}
		typed, problem := parseNodeSetting(setting, value)
		if problem != "" {
			return nil, &NodeSettingError{Key: setting.Key(), Problem: problem}
		}
		values[setting.Key()] = typed
	}
	return values, nil
}

func parseNodeSetting(setting NodeSetting, value interface{}) (interface{}, string) {
	switch setting.(type) {
	case StringEntry:
		if text, ok := value.(string); ok {
			return text, ""
		}
		return nil, "must be text"
	case BoolEntry:
		if flag, ok := value.(bool); ok {
			return flag, ""
		}
		return nil, "must be true or false"
	case IntEntry:
		if n, ok := WholeNumber(value); ok {
			return n, ""
		}
		return nil, "must be a whole number"
	}
	return nil, "has a type the node cannot read"
}

// ApplyNodeSettings sets the values ParseNodeSettings checked, in memory first, so every
// reader sees them at once without a restart, and then writes the config file a single time.
// A key outside SyncedNodeSettings is refused before anything changes.
func ApplyNodeSettings(values map[string]interface{}) error {
	for key := range values {
		if !IsSyncedNodeSetting(key) {
			return &NodeSettingError{Key: key, Problem: "is not one the panel hands to its nodes"}
		}
	}
	viperLock.Lock()
	defer viperLock.Unlock()
	for key, value := range values {
		viper.Set(key, value)
	}
	return viper.WriteConfig()
}

// WholeNumber reads a number setting as JSON sends it (a number, or the text of one); false
// when it is not a whole number, so a typo is refused instead of turning into 0. Text is read
// in base 10, so "010" is 10 and not octal.
func WholeNumber(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case float64:
		n := int(v)
		return n, float64(n) == v
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		return n, err == nil
	}
	return 0, false
}
