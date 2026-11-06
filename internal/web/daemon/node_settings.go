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
// @Description The sha256 (hex) of the canonical JSON of this node's current values of the settings the panel hands to every node (off-site S3 backups and the CurseForge key). The panel compares it with its own to know whether to send them again. The values themselves are never answered: they include secrets.
// @Success 200 {object} skypanel.NodeSettingsState
// @Tags Daemon Root
// @Router /daemon/settings [get]
// @Security OAuth2Application[none]
func getNodeSettings(c *gin.Context) {
	c.JSON(http.StatusOK, skypanel.NodeSettingsState{Hash: config.NodeSettingsHash(config.NodeSettingsValues())})
}

// @Summary Take the panel's node settings
// @Description Saves the settings the panel hands to every node (off-site S3 backups and the CurseForge key) to this node's config.json and applies them without a restart. The body holds exactly the keys of that list, each with its value: an unknown or missing key, a value of the wrong type or a periodic value the panel does not offer is refused with 400 and nothing is applied.
// @Success 204 {object} nil
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param data body map[string]interface{} true "Every synced setting, keyed by its config key"
// @Tags Daemon Root
// @Router /daemon/settings [put]
// @Security OAuth2Application[none]
func putNodeSettings(c *gin.Context) {
	var raw map[string]interface{}
	if err := c.ShouldBindJSON(&raw); err != nil || raw == nil {
		response.HandleError(c, skypanel.ErrNodeSettingInvalid("package", "must be an object of settings"), http.StatusBadRequest)
		return
	}

	values, err := checkNodeSettings(raw)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	if response.HandleError(c, config.ApplyNodeSettings(values), http.StatusInternalServerError) {
		return
	}

	// only the keys: the values include secrets
	logging.Info.Printf("Took the panel's node settings (%d keys)", len(values))
	c.Status(http.StatusNoContent)
}

// checkNodeSettings checks the whole package before anything is saved, with the rules the
// panel applies on its settings page, so a bad value never leaves this node half set
func checkNodeSettings(raw map[string]interface{}) (map[string]interface{}, error) {
	values, err := config.ParseNodeSettings(raw)
	var settingErr *config.NodeSettingError
	if errors.As(err, &settingErr) {
		return nil, skypanel.ErrNodeSettingInvalid(settingErr.Key, settingErr.Problem)
	}
	if err != nil {
		return nil, err
	}
	for key, value := range values {
		if err := servers.ValidateOffsiteSetting(key, value); err != nil {
			return nil, err
		}
	}
	return values, nil
}
