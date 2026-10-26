package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
)

// @Summary Get config
// @Description Gets the editable config entries for the panel
// @Success 200 {object} EditableConfig
// @Tags Config
// @Router /api/config [get]
// @Security OAuth2Application[none]
func panelConfig(c *gin.Context) {
	themes := make([]string, 0)
	files, err := os.ReadDir(config.WebRoot.Value() + "/theme")
	if err == nil {
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".tar") {
				themes = append(themes, f.Name()[:len(f.Name())-4])
			}
		}
	}

	c.JSON(http.StatusOK, EditableConfig{
		Themes: ThemeConfig{
			Active:    config.DefaultTheme.Value(),
			Settings:  config.ThemeSettings.Value(),
			Available: themes,
		},
		Branding: BrandingConfig{
			Name: config.CompanyName.Value(),
		},
		RegistrationEnabled: config.RegistrationEnabled.Value(),
		TurnstileSiteKey:    config.TurnstileSiteKey.Value(),
		TurnstileEnabled:    config.TurnstileEnabled.Value(),
		Experimental: ExperimentalConfig{
			VersionSwitch:    config.ExperimentalVersionSwitch.Value(),
			PropertiesEditor: config.ExperimentalPropertiesEditor.Value(),
			ServerClone:      config.ExperimentalServerClone.Value(),
		},
		Version: skypanel.Version,
	})
}

type EditableConfig struct {
	Themes              ThemeConfig        `json:"themes"`
	Branding            BrandingConfig     `json:"branding"`
	RegistrationEnabled bool               `json:"registrationEnabled"`
	TurnstileSiteKey    string             `json:"turnstileSiteKey"`
	TurnstileEnabled    bool               `json:"turnstileEnabled"`
	Experimental        ExperimentalConfig `json:"experimental"`
	Version             string             `json:"version"`
} // @name EditableConfigSettings

// ExperimentalConfig says which experimental features an admin has turned on
type ExperimentalConfig struct {
	VersionSwitch    bool `json:"versionSwitch"`
	PropertiesEditor bool `json:"propertiesEditor"`
	ServerClone      bool `json:"serverClone"`
} // @name ExperimentalConfig

// requiresExperimental refuses a request while the experimental feature behind it is off,
// so turning a feature off in the settings stops it, not just hides its button
func requiresExperimental(feature config.BoolEntry) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !feature.Value() {
			c.AbortWithStatusJSON(http.StatusForbidden, &skypanel.ErrorResponse{Error: skypanel.ErrFeatureDisabled})
			return
		}
		c.Next()
	}
}

type ThemeConfig struct {
	Active    string   `json:"active" example:"default"`
	Settings  string   `json:"settings" example:"{}"`
	Available []string `json:"available" example:"alternativeTheme"`
} // @name ThemeConfig

type BrandingConfig struct {
	Name string `json:"name" example:"SkyPanel"`
} // @name BrandingConfig
