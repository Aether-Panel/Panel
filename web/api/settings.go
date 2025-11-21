package api

import (
	"fmt"
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/config"
	"github.com/SkyPanel/SkyPanel/v3/logging"
	"github.com/SkyPanel/SkyPanel/v3/middleware"
	"github.com/SkyPanel/SkyPanel/v3/models"
	"github.com/SkyPanel/SkyPanel/v3/response"
	"github.com/SkyPanel/SkyPanel/v3/scopes"
	"github.com/SkyPanel/SkyPanel/v3/services"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

func registerSettings(g *gin.RouterGroup) {
	g.Handle("POST", "", middleware.RequiresPermission(scopes.ScopeSettingsEdit), setSettings)
	g.Handle("OPTIONS", "", response.CreateOptions("POST"))

	g.Handle("GET", "/:key", middleware.RequiresPermission(scopes.ScopeSettingsEdit), getSetting)
	g.Handle("PUT", "/:key", middleware.RequiresPermission(scopes.ScopeSettingsEdit), setSetting)
	g.Handle("OPTIONS", "/:key", response.CreateOptions("GET", "PUT"))

	g.Handle("POST", "/test/email", middleware.RequiresPermission(scopes.ScopeSettingsEdit), sendTestEmail)
	g.Handle("OPTIONS", "/test/email", response.CreateOptions("POST"))

	g.Handle("POST", "/test/discord", middleware.RequiresPermission(scopes.ScopeSettingsEdit), sendTestDiscord)
	g.Handle("OPTIONS", "/test/discord", response.CreateOptions("POST"))
}

// @Summary Value a panel setting
// @Description Gets the value currently being used for the specified config key
// @Success 200 {object} models.Setting
// @Param key path string true "The config key"
// @Router /api/settings/{key} [get]
// @Security OAuth2Application[settings.edit]
func getSetting(c *gin.Context) {
	key := c.Param("key")

	for _, v := range editableStringEntries {
		if v.Key() == key {
			c.JSON(http.StatusOK, models.Setting{Value: v.Value()})
			return
		}
	}

	for _, v := range editableBoolEntries {
		if v.Key() == key {
			c.JSON(http.StatusOK, models.Setting{Value: v.Value()})
			return
		}
	}

	for _, v := range editableIntEntries {
		if v.Key() == key {
			c.JSON(http.StatusOK, models.Setting{Value: v.Value()})
			return
		}
	}

	c.Status(http.StatusNoContent)
}

// @Summary Update a panel setting
// @Description Updates the value of a panel setting
// @Success 204 {object} nil
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Param key path string true "The config key"
// @Param value body models.Setting true "The new value for the setting"
// @Router /api/settings/{key} [put]
// @Security OAuth2Application[settings.edit]
func setSetting(c *gin.Context) {
	key := c.Param("key")

	var model models.Setting
	var err error
	if err = c.BindJSON(&model); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	companyNameChanged := false
	for _, v := range editableStringEntries {
		if v.Key() == key {
			err = v.Set(cast.ToString(model.Value), true)
			if response.HandleError(c, err, http.StatusInternalServerError) {
				return
			}
			// Si cambió el nombre de la empresa, sincronizar con Gatus
			if key == "panel.settings.companyName" {
				companyNameChanged = true
			}
		}
	}

	for _, v := range editableBoolEntries {
		if v.Key() == key {
			err = v.Set(cast.ToBool(model.Value), true)
			if response.HandleError(c, err, http.StatusInternalServerError) {
				return
			}
		}
	}

	for _, v := range editableIntEntries {
		if v.Key() == key {
			err = v.Set(cast.ToInt(model.Value), true)
			if response.HandleError(c, err, http.StatusInternalServerError) {
				return
			}
		}
	}

	services.SyncNodeToConfig()

	// Sincronizar el nombre de la empresa con Gatus si cambió
	if companyNameChanged {
		if err := services.SyncCompanyNameToGatus(); err != nil {
			// No es crítico, solo loguear el error
			logging.Error.Printf("Error syncing company name to Gatus: %s", err.Error())
		}
	}

	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Update multiple panel setting
// @Description Updates multiple panel settings at once
// @Success 204 {object} nil
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Param data body models.ChangeMultipleSettings true "Config data to apply"
// @Router /api/settings [post]
// @Security OAuth2Application[settings.edit]
func setSettings(c *gin.Context) {
	var settings *models.ChangeMultipleSettings
	var err error
	if err = c.BindJSON(&settings); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	companyNameChanged := false
	for key, value := range *settings {
		for _, v := range editableStringEntries {
			if v.Key() == key {
				err = v.Set(cast.ToString(value), true)
				if response.HandleError(c, err, http.StatusInternalServerError) {
					return
				}
				// Si cambió el nombre de la empresa, sincronizar con Gatus
				if key == "panel.settings.companyName" {
					companyNameChanged = true
				}
			}
		}

		for _, v := range editableBoolEntries {
			if v.Key() == key {
				err = v.Set(cast.ToBool(value), true)
				if response.HandleError(c, err, http.StatusInternalServerError) {
					return
				}
			}
		}

		for _, v := range editableIntEntries {
			if v.Key() == key {
				err = v.Set(cast.ToInt(value), true)
				if response.HandleError(c, err, http.StatusInternalServerError) {
					return
				}
			}
		}
	}

	services.SyncNodeToConfig()

	// Sincronizar el nombre de la empresa con Gatus si cambió
	if companyNameChanged {
		if err := services.SyncCompanyNameToGatus(); err != nil {
			// No es crítico, solo loguear el error
			logging.Error.Printf("Error syncing company name to Gatus: %s", err.Error())
		}
	}

	c.Status(http.StatusNoContent)
}

// @Summary Email test
// @Description Tests email settings by sending an email
// @Success 204 {object} nil
// @Router /api/settings/test/email [post]
// @Security OAuth2Application[settings.edit]
func sendTestEmail(c *gin.Context) {
	user := c.MustGet("user").(*models.User)

	es := services.GetEmailService()
	err := es.SendEmail(user.Email, "test", nil, false)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Discord webhook test
// @Description Tests Discord webhook settings by sending a test message to all configured webhooks
// @Success 204 {object} nil
// @Router /api/settings/test/discord [post]
// @Security OAuth2Application[settings.edit]
func sendTestDiscord(c *gin.Context) {
	ds := services.GetDiscordService()

	fields := []services.DiscordEmbedField{
		{Name: "Tipo", Value: "Mensaje de Prueba", Inline: true},
		{Name: "Estado", Value: "✅ Configuración Correcta", Inline: true},
	}

	title := "🧪 Test de Webhook Discord"
	description := "Este es un mensaje de prueba para verificar que el webhook de Discord está configurado correctamente."
	color := 0x0099FF

	// Enviar a los 3 webhooks si están configurados
	var errors []string

	// Webhook principal
	if config.DiscordWebhook.Value() != "" {
		err := ds.SendWebhookToURL(config.DiscordWebhook.Value(), title, description, color, fields)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Webhook principal: %v", err))
			logging.Error.Printf("Error enviando test al webhook principal: %v", err)
		}
	}

	// Webhook del sistema
	if config.DiscordWebhookSystem.Value() != "" {
		err := ds.SendWebhookToURL(config.DiscordWebhookSystem.Value(), title+" (Sistema)", description+" Este es el webhook del sistema.", color, fields)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Webhook sistema: %v", err))
			logging.Error.Printf("Error enviando test al webhook del sistema: %v", err)
		}
	}

	// Webhook del nodo
	if config.DiscordWebhookNode.Value() != "" {
		err := ds.SendWebhookToURL(config.DiscordWebhookNode.Value(), title+" (Nodo)", description+" Este es el webhook del nodo.", color, fields)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Webhook nodo: %v", err))
			logging.Error.Printf("Error enviando test al webhook del nodo: %v", err)
		}
	}

	// Si hay errores y no se envió a ningún webhook, retornar error
	if len(errors) > 0 {
		if config.DiscordWebhook.Value() == "" && config.DiscordWebhookSystem.Value() == "" && config.DiscordWebhookNode.Value() == "" {
			response.HandleError(c, fmt.Errorf("no hay webhooks configurados"), http.StatusBadRequest)
			return
		}
		// Si hay al menos un webhook configurado pero falló, loguear pero no fallar
		logging.Info.Printf("Algunos webhooks fallaron al enviar test: %v", errors)
	}

	c.Status(http.StatusNoContent)
}

var editableStringEntries = []config.StringEntry{
	config.EmailDomain,
	config.EmailFrom,
	config.EmailHost,
	config.EmailKey,
	config.EmailPassword,
	config.EmailProvider,
	config.EmailUsername,
	config.CompanyName,
	config.DefaultTheme,
	config.ThemeSettings,
	config.MasterUrl,
	config.DiscordWebhook,
	config.DiscordWebhookSystem,
	config.DiscordWebhookNode,
}
var editableBoolEntries = []config.BoolEntry{
	config.RegistrationEnabled,
}
var editableIntEntries = []config.IntEntry{}
