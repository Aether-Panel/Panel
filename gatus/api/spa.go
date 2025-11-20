package api

import (
	_ "embed"
	"html/template"
	"os"

	"github.com/TwiN/gatus/v5/config"
	"github.com/TwiN/gatus/v5/config/ui"
	static "github.com/TwiN/gatus/v5/web"
	"github.com/TwiN/logr"
	"github.com/gofiber/fiber/v2"
)

func SinglePageApplication(uiConfig *ui.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Intentar leer el config actualizado del archivo para obtener el nombre de la empresa más reciente
		configPath := os.Getenv("GATUS_CONFIG_PATH")
		if configPath == "" {
			configPath = "config/config.yaml"
		}
		
		// Usar el uiConfig pasado como parámetro por defecto
		currentUIConfig := uiConfig
		
		// Intentar leer el config del archivo para obtener actualizaciones dinámicas
		if _, err := os.Stat(configPath); err == nil {
			cfg, err := config.LoadConfiguration(configPath)
			if err == nil && cfg != nil && cfg.UI != nil {
				// Usar el config actualizado del archivo
				currentUIConfig = cfg.UI
			}
		}
		
		vd := ui.ViewData{UI: currentUIConfig}
		{
			themeFromCookie := string(c.Request().Header.Cookie("theme"))
			if len(themeFromCookie) > 0 {
				if themeFromCookie == "dark" {
					vd.Theme = "dark"
				}
			} else if uiConfig.IsDarkMode() { // Since there's no theme cookie, we'll rely on ui.DarkMode
				vd.Theme = "dark"
			}
		}
		t, err := template.ParseFS(static.FileSystem, static.IndexPath)
		if err != nil {
			// This should never happen, because ui.ValidateAndSetDefaults validates that the template works.
			logr.Errorf("[api.SinglePageApplication] Failed to parse template. This should never happen, because the template is validated on start. Error: %s", err.Error())
			return c.Status(500).SendString("Failed to parse template. This should never happen, because the template is validated on start.")
		}
		c.Set("Content-Type", "text/html")
		err = t.Execute(c, vd)
		if err != nil {
			// This should never happen, because ui.ValidateAndSetDefaults validates that the template works.
			logr.Errorf("[api.SinglePageApplication] Failed to execute template. This should never happen, because the template is validated on start. Error: %s", err.Error())
			return c.Status(500).SendString("Failed to parse template. This should never happen, because the template is validated on start.")
		}
		return c.SendStatus(200)
	}
}
