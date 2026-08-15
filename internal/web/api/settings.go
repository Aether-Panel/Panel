package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/update"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

func registerSettings(g *gin.RouterGroup) {
	g.Handle("GET", "", middleware.RequiresPermission(scopes.ScopeSettingsEdit), getSettings)
	g.Handle("POST", "", middleware.RequiresPermission(scopes.ScopeSettingsEdit), setSettings)
	g.Handle("OPTIONS", "", response.CreateOptions("GET", "POST"))

	g.Handle("GET", "/:key", middleware.RequiresPermission(scopes.ScopeSettingsEdit), getSetting)
	g.Handle("PUT", "/:key", middleware.RequiresPermission(scopes.ScopeSettingsEdit), setSetting)
	g.Handle("OPTIONS", "/:key", response.CreateOptions("GET", "PUT"))

	g.Handle("POST", "/test/email", middleware.RequiresPermission(scopes.ScopeSettingsEdit), sendTestEmail)
	g.Handle("OPTIONS", "/test/email", response.CreateOptions("POST"))

	g.Handle("POST", "/test/discord", middleware.RequiresPermission(scopes.ScopeSettingsEdit), sendTestDiscord)
	g.Handle("OPTIONS", "/test/discord", response.CreateOptions("POST"))

	g.Handle("POST", "/license/activate", middleware.RequiresPermission(scopes.ScopeSettingsEdit), activateLicense)
	g.Handle("OPTIONS", "/license/activate", response.CreateOptions("POST"))

	g.Handle("POST", "/update-panel", middleware.RequiresPermission(scopes.ScopeSettingsEdit), updatePanel)
	g.Handle("OPTIONS", "/update-panel", response.CreateOptions("POST"))

	g.Handle("GET", "/update-check", middleware.RequiresPermission(scopes.ScopeSettingsEdit), updateCheck)
	g.Handle("OPTIONS", "/update-check", response.CreateOptions("GET"))

	g.Handle("GET", "/update-status", middleware.RequiresPermission(scopes.ScopeSettingsEdit), updateStatus)
	g.Handle("OPTIONS", "/update-status", response.CreateOptions("GET"))
}

// @Summary Value a panel setting
// @Description Gets the value currently being used for the specified config key
// @Success 200 {object} models.Setting
// @Param key path string true "The config key"
// @Tags Panel Settings
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
// @Tags Panel Settings
// @Router /api/settings/{key} [put]
// @Security OAuth2Application[settings.edit]
func setSetting(c *gin.Context) {
	key := c.Param("key")

	var model models.Setting
	var err error
	if err = c.BindJSON(&model); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	if key == "panel.settings.masterNodeIp" {
		db := middleware.GetDatabase(c)
		err = db.Save(&models.PanelSetting{
			Key:   "master_node_ip",
			Value: cast.ToString(model.Value),
		}).Error
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
		c.Status(http.StatusNoContent)
		return
	}

	for _, v := range editableStringEntries {
		if v.Key() == key {
			err = v.Set(cast.ToString(model.Value), true)
			if response.HandleError(c, err, http.StatusInternalServerError) {
				return
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
// @Tags Panel Settings
// @Router /api/settings [post]
// @Security OAuth2Application[settings.edit]
func setSettings(c *gin.Context) {
	var settings *models.ChangeMultipleSettings
	var err error
	if err = c.BindJSON(&settings); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	db := middleware.GetDatabase(c)

	for key, value := range *settings {
		// Intercept the custom global setting
		if key == "panel.settings.masterNodeIp" {
			db.Save(&models.PanelSetting{
				Key:   "master_node_ip",
				Value: cast.ToString(value),
			})
			continue
		}

		for _, v := range editableStringEntries {
			if v.Key() == key {
				err = v.Set(cast.ToString(value), true)
				if response.HandleError(c, err, http.StatusInternalServerError) {
					return
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

	c.Status(http.StatusNoContent)
}

func getSettings(c *gin.Context) {
	settings := make(map[string]interface{})

	for _, v := range editableStringEntries {
		settings[v.Key()] = v.Value()
	}

	for _, v := range editableBoolEntries {
		settings[v.Key()] = v.Value()
	}

	for _, v := range editableIntEntries {
		settings[v.Key()] = v.Value()
	}

	// Fetch the global master node ip
	db := middleware.GetDatabase(c)
	var masterSetting models.PanelSetting
	if err := db.Where("`key` = ?", "master_node_ip").First(&masterSetting).Error; err == nil {
		settings["panel.settings.masterNodeIp"] = masterSetting.Value
	} else {
		settings["panel.settings.masterNodeIp"] = ""
	}

	c.JSON(http.StatusOK, settings)
}

// @Summary Email test
// @Description Tests email settings by sending an email
// @Success 204 {object} nil
// @Tags Panel Settings
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
// @Tags Panel Settings
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

// @Summary Activate license
// @Description Activates and verifies a license key with the external license server
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param body body map[string]string true "License key"
// @Tags Panel Settings
// @Router /api/settings/license/activate [post]
// @Security OAuth2Application[settings.edit]
func activateLicense(c *gin.Context) {
	var requestBody map[string]string
	if err := c.BindJSON(&requestBody); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	licenseKey, ok := requestBody["key"]
	if !ok || licenseKey == "" {
		response.HandleError(c, fmt.Errorf("license key is required"), http.StatusBadRequest)
		return
	}

	// Normalizar la clave de licencia (remover guiones y convertir a mayúsculas para comparación)
	normalizedKey := strings.ReplaceAll(strings.ToUpper(licenseKey), "-", "")
	if len(normalizedKey) != 16 {
		response.HandleError(c, fmt.Errorf("invalid license key format"), http.StatusBadRequest)
		return
	}

	// Obtener el servicio de licencias
	licenseService := services.GetLicenseService()

	// Primero verificar la licencia (GET)
	verifyResp, err := licenseService.VerifyLicense(licenseKey)
	if err != nil {
		logging.Error.Printf("Error verifying license: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("Error verifying license: %s", err.Error()),
		})
		return
	}

	if !verifyResp.Valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "License is not valid",
		})
		return
	}

	// Obtener identificador del servidor y IP
	serverID := config.LicenseServerID.Value()
	serverIP := config.LicenseServerIP.Value()

	// Si no tenemos serverID o serverIP guardados, generarlos
	if serverID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		serverID = hostname
		_ = config.LicenseServerID.Set(serverID, true)
	}

	if serverIP == "" {
		// Intentar obtener la IP pública del hostname
		ip, err := getServerIP()
		if err != nil {
			// Usar la IP privada como fallback
			ip = "127.0.0.1"
		}
		serverIP = ip
		_ = config.LicenseServerIP.Set(serverIP, true)
	}

	// Vincular la licencia con el servidor (POST)
	bindResp, err := licenseService.BindLicense(licenseKey, serverID, serverIP)
	if err != nil {
		logging.Error.Printf("Error binding license: %s", err.Error())
		// Aun si falla el bind, guardamos la licencia como válida
		// porque ya verificamos que es válida
	}

	// Determinar el tipo de licencia
	licenseType := licenseService.GetLicenseType(verifyResp)

	// Guardar la información de la licencia
	_ = config.LicenseKey.Set(licenseKey, true)
	_ = config.LicenseStatus.Set(licenseType, true)

	// Extraer permisos
	permissions := licenseService.ExtractPermissions(verifyResp)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"type":    licenseType,
		"license": gin.H{
			"key":           verifyResp.License.Key,
			"plan":          verifyResp.License.Plan,
			"maxServers":    verifyResp.License.MaxServers,
			"usedServers":   verifyResp.License.UsedServers,
			"expiryDate":    verifyResp.License.ExpiryDate,
			"daysRemaining": verifyResp.License.DaysRemaining,
			"billingCycle":  verifyResp.License.BillingCycle,
		},
		"permissions": permissions,
		"bound":       bindResp != nil && bindResp.Success,
		"message":     "License activated successfully",
	})
}

// getServerIP obtiene la IP del servidor
func getServerIP() (string, error) {
	// Primero intentar obtener la IP pública del hostname
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}

	// Intentar resolver el hostname a IP
	ips, err := net.LookupIP(hostname)
	if err != nil {
		// Si no se puede resolver, intentar obtener IP de interfaces de red
		return getLocalIP()
	}

	// Buscar una IP IPv4 que no sea localhost
	for _, ip := range ips {
		if ip.To4() != nil && !ip.IsLoopback() {
			return ip.String(), nil
		}
	}

	// Si no encontramos una IP pública, usar local
	return getLocalIP()
}

// getLocalIP obtiene una IP local de las interfaces de red
func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1", err
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}

	return "127.0.0.1", fmt.Errorf("no local IP found")
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
	config.MasterURL,
	config.NodeIP,
	config.GeminiAPIKey,
	config.DiscordWebhook,
	config.DiscordWebhookSystem,
	config.DiscordWebhookNode,
	config.LicenseKey,
	config.LicenseStatus,
	config.LicenseServerID,
	config.LicenseServerIP,
}
var editableBoolEntries = []config.BoolEntry{
	config.RegistrationEnabled,
	config.HideAIAnalysis,
	config.HeaderDecorations,
}
var editableIntEntries = []config.IntEntry{}

// @Summary Update panel automatically
// @Description Spawns an ephemeral container on the host to run the panelUpdate script, and triggers the update on every registered node.
// @Success 200 {object} nil
// @Failure 500 {object} ErrorResponse
// @Tags Panel Settings
// @Router /api/settings/update-panel [post]
// @Security OAuth2Application[settings.edit]
func updatePanel(c *gin.Context) {
	logging.Info.Println("Update panel requested via API")

	db := middleware.GetDatabase(c)
	ns := &services.Node{DB: db}

	nodes, err := ns.GetAll()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	results := make([]gin.H, 0, len(nodes))
	localUpdated := false

	for _, node := range nodes {
		if node.IsLocal() {
			if localUpdated {
				continue
			}
			localUpdated = true
			containerID, err := update.Trigger()
			if err != nil {
				logging.Error.Printf("Failed to trigger local update: %v", err)
				update.RecordFailure(err.Error())
				results = append(results, gin.H{"name": node.Name, "local": true, "error": err.Error()})
			} else {
				logging.Info.Printf("Local update container started with ID %s", containerID)
				results = append(results, gin.H{"name": node.Name, "local": true})
			}
			continue
		}

		resp, err := ns.CallNode(node, http.MethodPost, "/daemon/update", http.NoBody, nil)
		if err != nil {
			logging.Error.Printf("Failed to trigger update on node %s: %v", node.Name, err)
			results = append(results, gin.H{"name": node.Name, "error": err.Error()})
			continue
		}
		if resp.StatusCode >= 400 {
			logging.Error.Printf("Node %s returned status %d", node.Name, resp.StatusCode)
			results = append(results, gin.H{"name": node.Name, "error": fmt.Sprintf("node returned status %d", resp.StatusCode)})
			continue
		}
		logging.Info.Printf("Update triggered on node %s", node.Name)
		results = append(results, gin.H{"name": node.Name})
	}

	c.JSON(http.StatusOK, gin.H{"message": "Update initiated on the panel and all nodes.", "nodes": results})
}

// gitRepoGitDir is the location of the repository's .git directory, mounted
// read-only from the host at build/runtime (see docker-compose.yml).
const gitRepoGitDir = "/repo/.git"

// currentGitCommit returns the full SHA of the commit the repository currently
// points to, or an empty string if it cannot be determined.
func currentGitCommit() string {
	head, err := os.ReadFile(gitRepoGitDir + "/HEAD")
	if err != nil {
		return ""
	}

	ref := strings.TrimSpace(string(head))
	if strings.HasPrefix(ref, "ref:") {
		path := gitRepoGitDir + "/" + strings.TrimSpace(strings.TrimPrefix(ref, "ref:"))
		sha, err := os.ReadFile(path)
		if err == nil {
			if s := strings.TrimSpace(string(sha)); s != "" {
				return s
			}
		}

		// Ref could be packed (e.g. after git gc). Look it up in packed-refs.
		packed, err := os.ReadFile(gitRepoGitDir + "/packed-refs")
		if err != nil {
			return ""
		}
		for _, line := range strings.Split(string(packed), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") || trimmed == "" {
				continue
			}
			parts := strings.SplitN(trimmed, " ", 2)
			if len(parts) == 2 && parts[1] == strings.TrimSpace(strings.TrimPrefix(ref, "ref:")) {
				return parts[0]
			}
		}
		return ""
	}

	return ref
}

// isGitAncestor reports whether ancestor is an ancestor of (or equal to)
// descendant in the repository mounted at gitRepoGitDir. It runs git either
// inside an isolated container (preferred) or through a fixed absolute binary
// path, so executable resolution never relies on a mutable PATH. Any git error
// (missing binary, unknown object, shallow history...) is returned so callers
// can fall back to the plain SHA comparison.
func isGitAncestor(ancestor, descendant string) (bool, error) {
	code, err := runGit("--git-dir="+gitRepoGitDir, "merge-base", "--is-ancestor", ancestor, descendant)
	if err != nil {
		return false, err
	}
	switch code {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		return false, fmt.Errorf("git merge-base --is-ancestor exited with unexpected code %d", code)
	}
}

// @Summary Check for panel updates
// @Description Compares the deployed commit with the latest commit on the dev2.0 branch.
// @Success 200 {object} nil
// @Tags Panel Settings
// @Router /api/settings/update-check [get]
// @Security OAuth2Application[settings.edit]
func updateCheck(c *gin.Context) {
	current := currentGitCommit()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/Aether-Panel/Panel/commits/dev2.0", nil)
	if err != nil {
		response.HandleError(c, fmt.Errorf("failed to build update check request: %v", err), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Aether-Panel")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logging.Error.Printf("Update check failed to reach GitHub: %v", err)
		c.JSON(http.StatusOK, gin.H{"current": current, "latest": "", "version": skypanel.Version, "updateAvailable": false})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logging.Error.Printf("Update check got unexpected GitHub response: %d", resp.StatusCode)
		c.JSON(http.StatusOK, gin.H{"current": current, "latest": "", "version": skypanel.Version, "updateAvailable": false})
		return
	}

	var payload struct {
		Sha string `json:"sha"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		logging.Error.Printf("Update check failed to decode GitHub response: %v", err)
		c.JSON(http.StatusOK, gin.H{"current": current, "latest": "", "version": skypanel.Version, "updateAvailable": false})
		return
	}

	latest := payload.Sha

	// Consider an update available only when the remote tip contains commits
	// not yet deployed. A plain SHA comparison produces false positives when
	// the host repository has local commits (e.g. merge commits created by
	// git pull after local edits), so compare ancestry instead.
	updateAvailable := current != "" && latest != "" && current != latest
	if updateAvailable {
		contains, err := isGitAncestor(latest, current)
		if err == nil {
			updateAvailable = !contains
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"current":         current,
		"latest":          latest,
		"version":         skypanel.Version,
		"updateAvailable": updateAvailable,
	})
}

// nodeUpdateStatus describes the update status of a single node of the fleet.
type nodeUpdateStatus struct {
	Name        string    `json:"name"`
	Address     string    `json:"address"`
	Local       bool      `json:"local"`
	Online      bool      `json:"online"`
	Running     bool      `json:"running"`
	ContainerID string    `json:"containerId"`
	StartedAt   time.Time `json:"startedAt"`
	FinishedAt  time.Time `json:"finishedAt"`
	ExitCode    int       `json:"exitCode"`
	Log         string    `json:"log"`
	Error       string    `json:"error"`
}

// @Summary Get panel update status
// @Description Returns the update status, exit code and log tail of the panel and every registered node.
// @Success 200 {object} nil
// @Tags Panel Settings
// @Router /api/settings/update-status [get]
// @Security OAuth2Application[settings.edit]
func updateStatus(c *gin.Context) {
	db := middleware.GetDatabase(c)
	ns := &services.Node{DB: db}

	nodes, err := ns.GetAll()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	statuses := make([]nodeUpdateStatus, 0, len(nodes))

	for _, node := range nodes {
		status := nodeUpdateStatus{
			Name:    node.Name,
			Address: fmt.Sprintf("%s:%d", node.PrivateHost, node.PrivatePort),
			Local:   node.IsLocal(),
		}

		if node.IsLocal() {
			state := update.Status()
			status.Online = true
			status.Running = state.Running
			status.ContainerID = state.ContainerID
			status.StartedAt = state.StartedAt
			status.FinishedAt = state.FinishedAt
			status.ExitCode = state.ExitCode
			status.Log = state.LogTail
			status.Error = state.Error
		} else {
			resp, err := ns.CallNode(node, http.MethodGet, "/daemon/update-status", http.NoBody, nil)
			if err != nil {
				status.Error = fmt.Sprintf("unable to reach node: %v", err)
				statuses = append(statuses, status)
				continue
			}

			if resp.StatusCode != http.StatusOK {
				status.Error = fmt.Sprintf("node returned status %d", resp.StatusCode)
				statuses = append(statuses, status)
				continue
			}

			var state update.State
			if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
				status.Error = "invalid status response from node"
				statuses = append(statuses, status)
				continue
			}

			resp.Body.Close()
			status.Online = true
			status.Running = state.Running
			status.ContainerID = state.ContainerID
			status.StartedAt = state.StartedAt
			status.FinishedAt = state.FinishedAt
			status.ExitCode = state.ExitCode
			status.Log = state.LogTail
			status.Error = state.Error
		}

		statuses = append(statuses, status)
	}

	c.JSON(http.StatusOK, gin.H{"nodes": statuses})
}
