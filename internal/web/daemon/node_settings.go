package daemon

import (
	"errors"
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
)

// @Summary Get the hash of the panel's node settings
// @Description The settings the panel hands to every node (off-site S3 backups and the CurseForge key) that this node knows, as keys, and the sha256 (hex) of the canonical JSON of their current values. The panel hashes its own values for the keys both know and compares, so a panel and a node of different versions still agree on the settings they share. The values themselves are never answered: they include secrets.
// @Success 200 {object} skypanel.NodeSettingsState
// @Tags Daemon Root
// @Router /daemon/settings [get]
// @Security OAuth2Application[none]
func getNodeSettings(c *gin.Context) {
	keys := config.NodeSettingsKeys()
	c.JSON(http.StatusOK, skypanel.NodeSettingsState{Hash: config.NodeSettingsHashFor(config.NodeSettingsValues(), keys), Keys: keys})
}

// @Summary Take the panel's node settings
// @Description Saves the settings the panel hands to every node (off-site S3 backups and the CurseForge key) to this node's config.json and applies them without a restart. The body holds any of those keys, each with its value; a key left out stays as it is, so an older panel that sends fewer keys still works. A key this node does not know (from a newer panel) is ignored. A known key with a value of the wrong type, or a periodic value the panel does not offer, is refused with 400 and nothing is applied. An object with no known keys changes nothing and answers 204.
// @Success 204 {object} nil
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param data body map[string]interface{} true "Synced settings, keyed by their config key"
// @Tags Daemon Root
// @Router /daemon/settings [put]
// @Security OAuth2Application[none]
func putNodeSettings(c *gin.Context) {
	var raw map[string]interface{}
	if err := c.ShouldBindJSON(&raw); err != nil || raw == nil {
		response.HandleError(c, skypanel.ErrNodeSettingInvalid("package", "must be an object of settings"), http.StatusBadRequest)
		return
	}

	values, ignored, err := checkNodeSettings(raw)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	if response.HandleError(c, config.ApplyNodeSettings(values), http.StatusInternalServerError) {
		return
	}

	// only counts: the values include secrets
	logging.Info.Printf("Took the panel's node settings (%d keys, %d unknown to this node ignored)", len(values), ignored)
	c.Status(http.StatusNoContent)
}

// checkNodeSettings checks the whole package before anything is saved, with the rules the
// panel applies on its settings page, so a bad value never leaves this node half set. ignored
// counts the keys this node does not know, which a newer panel may send.
func checkNodeSettings(raw map[string]interface{}) (values map[string]interface{}, ignored int, err error) {
	values, ignored, err = config.ParseNodeSettings(raw)
	var settingErr *config.NodeSettingError
	if errors.As(err, &settingErr) {
		return nil, 0, skypanel.ErrNodeSettingInvalid(settingErr.Key, settingErr.Problem)
	}
	if err != nil {
		return nil, 0, err
	}
	for key, value := range values {
		if invalid := servers.ValidateOffsiteSetting(key, value); invalid != nil {
			return nil, 0, invalid
		}
	}
	return values, ignored, nil
}
