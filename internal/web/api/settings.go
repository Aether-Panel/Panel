package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/offsite"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/update"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

// masterNodeIPKey is the setting that holds the master node's address
const masterNodeIPKey = "panel.settings.masterNodeIp"

// loopbackIP is the fallback when no local address is found
const loopbackIP = "127.0.0.1"

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

	g.Handle("POST", "/test/s3", middleware.RequiresPermission(scopes.ScopeSettingsEdit), sendTestS3)
	g.Handle("OPTIONS", "/test/s3", response.CreateOptions("POST"))
	g.Handle("POST", "/test/cloudflare", middleware.RequiresPermission(scopes.ScopeSettingsEdit), sendTestCloudflare)

	g.Handle("POST", "/license/activate", middleware.RequiresPermission(scopes.ScopeSettingsEdit), activateLicense)
	g.Handle("OPTIONS", "/license/activate", response.CreateOptions("POST"))

	g.Handle("POST", "/update-panel", middleware.RequiresPermission(scopes.ScopeSettingsEdit), updatePanel)
	g.Handle("OPTIONS", "/update-panel", response.CreateOptions("POST"))

	g.Handle("GET", "/update-check", middleware.RequiresPermission(scopes.ScopeSettingsEdit), updateCheck)
	g.Handle("OPTIONS", "/update-check", response.CreateOptions("GET"))

	g.Handle("GET", "/update-status", middleware.RequiresPermission(scopes.ScopeSettingsEdit), updateStatus)
	g.Handle("OPTIONS", "/update-status", response.CreateOptions("GET"))

	g.Handle("GET", "/domain-status", middleware.RequiresPermission(scopes.ScopeSettingsEdit), domainStatus)
	g.Handle("OPTIONS", "/domain-status", response.CreateOptions("GET"))
}

// @Summary Domain set-up status
// @Description What the server did with the Base URL: Nginx, the HTTPS certificate and the firewall
// @Success 200 {object} services.DomainStatus
// @Tags Settings
// @Router /api/settings/domain-status [get]
// @Security OAuth2Application[settings.edit]
func domainStatus(c *gin.Context) {
	status, err := services.ReadDomainStatus()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.JSON(http.StatusOK, status)
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
			c.JSON(http.StatusOK, models.Setting{Value: maskedSettingValue(key, v.Value())})
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

	// the page sent the mask back: the saved secret stays as it is
	if isSecretSetting(key) && cast.ToString(model.Value) == secretMask {
		c.Status(http.StatusNoContent)
		return
	}

	db := middleware.GetDatabase(c)
	if key == masterNodeIPKey {
		previousHost := localNodeHost(db)
		err = saveMasterNodeIP(db, model.Value)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
		followNodeAddress(models.LocalNode.ID, previousHost, localNodeHost(db))
		c.Status(http.StatusNoContent)
		return
	}

	if response.HandleError(c, validateStringSetting(key, cast.ToString(model.Value)), http.StatusBadRequest) {
		return
	}

	previousLanguage := services.NotificationLanguage()
	previousDNS, previousHost := prepareDNSSettingsChange(db, key)
	if err = applyEditableSetting(key, model.Value); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	services.SyncNodeToConfig()
	pushNotificationLanguageIfChanged(previousLanguage)
	followDNSSettingsChange(db, previousDNS, previousHost)

	c.Status(http.StatusNoContent)
}

// saveMasterNodeIP stores the global master node ip, which lives in the
// database instead of the config file.
func saveMasterNodeIP(db *gorm.DB, value interface{}) error {
	return db.Save(&models.PanelSetting{
		Key:   "master_node_ip",
		Value: cast.ToString(value),
	}).Error
}

// applyEditableSetting writes the value to the editable config entry with
// the given key, converted to that entry's type. Unknown keys are ignored.
func applyEditableSetting(key string, value interface{}) error {
	text := strings.TrimSpace(cast.ToString(value))
	if key == config.MasterURL.Key() {
		// links in emails, password resets and the panel's identity need a full address
		text = services.NormalizePanelURL(text)
	}
	if err := setMatchingEntries(editableStringEntries, key, text); err != nil {
		return err
	}
	if err := setMatchingEntries(editableBoolEntries, key, cast.ToBool(value)); err != nil {
		return err
	}
	return setMatchingEntries(editableIntEntries, key, cast.ToInt(value))
}

// settableEntry is a config entry the settings API can change.
type settableEntry[T any] interface {
	Key() string
	Set(value T, save bool) error
}

// setMatchingEntries saves the value to every entry with the given key.
func setMatchingEntries[T any, E settableEntry[T]](entries []E, key string, value T) error {
	for _, v := range entries {
		if v.Key() != key {
			continue
		}
		if err := v.Set(value, true); err != nil {
			return err
		}
	}
	return nil
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
	dropMaskedSecrets(*settings)

	// check everything first, so a bad value does not leave the settings half saved
	for key, value := range *settings {
		if response.HandleError(c, validateStringSetting(key, cast.ToString(value)), http.StatusBadRequest) {
			return
		}
	}

	previousLanguage := services.NotificationLanguage()
	keys := make([]string, 0, len(*settings))
	for key := range *settings {
		keys = append(keys, key)
	}
	previousDNS, previousHost := prepareDNSSettingsChange(db, keys...)
	for key, value := range *settings {
		// Intercept the custom global setting
		if key == masterNodeIPKey {
			_ = saveMasterNodeIP(db, value)
			continue
		}

		if err = applyEditableSetting(key, value); response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
	}

	services.SyncNodeToConfig()
	pushNotificationLanguageIfChanged(previousLanguage)
	followDNSSettingsChange(db, previousDNS, previousHost)

	c.Status(http.StatusNoContent)
}

// prepareDNSSettingsChange notes what the subdomains depend on before a settings save: the DNS
// settings, after writing them into the servers whose subdomain predates that (so a new base
// domain can still find their records), and the panel node's address
func prepareDNSSettingsChange(db *gorm.DB, keys ...string) (dnsSettings, string) {
	previous := currentDNSSettings()
	if touchesDNSSettings(keys...) {
		if err := rememberSubdomainHome(db, previous); err != nil {
			logging.Error.Printf("keeping where the subdomain records live: %v", err)
		}
	}
	return previous, localNodeHost(db)
}

// followDNSSettingsChange moves or updates the subdomains after a settings save changed their
// base domain, zone or DNS target, or the panel node's address (master node IP, Base URL)
func followDNSSettingsChange(db *gorm.DB, previous dnsSettings, previousHost string) {
	followDNSSettings(previous)
	followNodeAddress(models.LocalNode.ID, previousHost, localNodeHost(db))
}

// pushNotificationLanguageIfChanged sends a changed notification language to the remote nodes
// now: the status poll carries it, and would otherwise only run within the next 30 seconds
func pushNotificationLanguageIfChanged(previous string) {
	if services.NotificationLanguage() != previous {
		servers.PollRemoteNodesNow()
	}
}

func getSettings(c *gin.Context) {
	settings := make(map[string]interface{})

	for _, v := range editableStringEntries {
		settings[v.Key()] = maskedSettingValue(v.Key(), v.Value())
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
		settings[masterNodeIPKey] = masterSetting.Value
	} else {
		settings[masterNodeIPKey] = ""
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

	var body map[string]interface{}
	_ = c.BindJSON(&body)
	dropMaskedSecrets(body)

	getStr := func(key string) string {
		if v, ok := body[key].(string); ok {
			return v
		}
		return ""
	}

	entries := []config.StringEntry{
		config.EmailProvider,
		config.EmailFrom,
		config.EmailHost,
		config.EmailUsername,
		config.EmailPassword,
		config.EmailDomain,
		config.EmailKey,
	}

	saved := make([]string, len(entries))
	for i, entry := range entries {
		saved[i] = entry.Value()
		if value := getStr(entry.Key()); value != "" {
			_ = entry.Set(value, false)
		}
	}

	defer func() {
		for i, entry := range entries {
			_ = entry.Set(saved[i], false)
		}
	}()

	es := services.GetEmailService()
	err := es.SendEmail(user.Email, "test", nil, false)
	// 422 rather than 500: the panel hides the text of 5xx errors, and the admin needs the
	// mail server's reason (wrong password, port, TLS) to fix the settings
	if err != nil {
		response.HandleError(c, fmt.Errorf("the test email could not be sent: %w", err), http.StatusUnprocessableEntity)
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

	// webhooks typed in the form but not saved yet are tested too, like the email test does
	var overrides map[string]interface{}
	_ = c.ShouldBindJSON(&overrides)
	dropMaskedSecrets(overrides)

	fields := []services.DiscordEmbedField{
		{Name: services.NotificationText("field.type"), Value: services.NotificationText("discordTest.type"), Inline: true},
		{Name: services.NotificationText("field.status"), Value: services.NotificationText("discordTest.status"), Inline: true},
	}

	title := services.NotificationText("discordTest.title")
	description := services.NotificationText("discordTest.description")
	color := 0x0099FF

	// send to every configured webhook and report the ones that failed: a test that always
	// says OK hides a wrong or deleted webhook until a real alert goes missing. The names are
	// the ones ErrDiscordTestFailed carries, which the page shows as its section titles.
	targets := []struct {
		name  string
		entry config.StringEntry
		title string
	}{
		{"alerts", config.DiscordWebhook, title},
		{"reports", config.DiscordWebhookSystem, title + " (" + services.NotificationText("discordTest.system") + ")"},
		{"status", config.DiscordWebhookNode, title + " (" + services.NotificationText("discordTest.node") + ")"},
		{"transfers", config.DiscordWebhookExTransfer, title + " (" + services.NotificationText("discordTest.transfers") + ")"},
	}

	var failed []string
	sent := 0
	for _, target := range targets {
		hook := target.entry.Value()
		if v, ok := overrides[target.entry.Key()].(string); ok {
			hook = strings.TrimSpace(v)
			if err := services.ValidateDiscordWebhook(hook); err != nil {
				response.HandleError(c, fmt.Errorf("webhook %s: %w", target.name, err), http.StatusBadRequest)
				return
			}
		}
		if hook == "" {
			continue
		}
		sent++
		if err := ds.SendWebhookToURL(hook, target.title, description, color, fields); err != nil {
			logging.Error.Printf("Error sending the test to the %s webhook: %v", target.name, err)
			failed = append(failed, target.name)
		}
	}

	if sent == 0 {
		response.HandleError(c, skypanel.ErrDiscordNoWebhooks, http.StatusBadRequest)
		return
	}
	if len(failed) > 0 {
		// 422 rather than 502: the panel hides the text of 5xx errors, and the admin needs it
		response.HandleError(c, skypanel.ErrDiscordTestFailed(failed), http.StatusUnprocessableEntity)
		return
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
			ip = loopbackIP
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
		return loopbackIP, err
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}

	return loopbackIP, fmt.Errorf("no local IP found")
}

// discordWebhookEntries only take Discord webhook URLs; anything else would be refused when a
// message is sent, so it is refused when saved instead
var discordWebhookEntries = []config.StringEntry{
	config.DiscordWebhook,
	config.DiscordWebhookSystem,
	config.DiscordWebhookNode,
	config.DiscordWebhookExTransfer,
}

func validateStringSetting(key, value string) error {
	for _, v := range discordWebhookEntries {
		if v.Key() == key {
			return services.ValidateDiscordWebhook(strings.TrimSpace(value))
		}
	}
	if key == config.NotificationLanguage.Key() && !services.IsNotificationLanguage(strings.TrimSpace(value)) {
		return skypanel.ErrNotificationLanguage
	}
	return nil
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
	config.NotificationLanguage,
	// the node's key for CurseForge modpacks (on remote nodes it is set in their own config)
	config.CurseForgeKey,
	// the OAuth App users connect their GitHub account through
	config.GitHubClientID,
	config.DiscordWebhook,
	config.DiscordWebhookSystem,
	config.DiscordWebhookNode,
	config.DiscordWebhookExTransfer,
	config.LicenseKey,
	config.LicenseStatus,
	config.LicenseServerID,
	config.LicenseServerIP,
	config.BackupsS3Endpoint,
	config.BackupsS3Region,
	config.BackupsS3Bucket,
	config.BackupsS3AccessKey,
	config.BackupsS3SecretKey,
	config.BackupsS3Prefix,
	config.CloudflareAPIToken,
	config.CloudflareZoneID,
	config.DNSBaseDomain,
	config.DNSTarget,
	config.TurnstileSiteKey,
	config.TurnstileSecretKey,
}
var editableBoolEntries = []config.BoolEntry{
	config.RegistrationEnabled,
	config.RegistrationApproval,
	config.HideAIAnalysis,
	config.AIFileTools,
	config.BackupsS3Enabled,
	config.BackupsS3Insecure,
	config.DNSAutoAssign,
	config.ExperimentalVersionSwitch,
	config.ExperimentalPropertiesEditor,
	config.ExperimentalServerClone,
	config.TurnstileEnabled,
	config.CaptchaBuiltin,
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

// updateBranchPattern is a branch name safe to put in the GitHub URL
var updateBranchPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]{1,100}$`)

// defaultUpdateBranch is the stable branch, used when the panel's repository has no branch
// checked out (a detached HEAD) or none can be read
const defaultUpdateBranch = "main"

// currentGitBranch is the branch the panel's repository has checked out, which is the one
// tools/panelUpdate/panelUpdate.sh updates from
func currentGitBranch() string {
	head, err := os.ReadFile(gitRepoGitDir + "/HEAD")
	if err != nil {
		return defaultUpdateBranch
	}
	ref := strings.TrimSpace(string(head))
	if !strings.HasPrefix(ref, "ref:") {
		return defaultUpdateBranch
	}
	branch := strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(ref, "ref:")), "refs/heads/")
	if !updateBranchPattern.MatchString(branch) || strings.Contains(branch, "..") {
		return defaultUpdateBranch
	}
	return branch
}

// @Summary Check for panel updates
// @Description Compares the deployed commit with the latest commit of the branch the panel is installed from (main when it cannot be read).
// @Success 200 {object} nil
// @Tags Panel Settings
// @Router /api/settings/update-check [get]
// @Security OAuth2Application[settings.edit]
func updateCheck(c *gin.Context) {
	current := currentGitCommit()
	// the branch the updater pulls from: comparing with another one announced updates that the
	// update would not bring
	branch := currentGitBranch()
	result := gin.H{"current": current, "latest": "", "version": skypanel.Version, "branch": branch, "updateAvailable": false, "behindBy": 0}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var tip struct {
		Sha string `json:"sha"`
	}
	if reason := githubJSON(ctx, "commits/"+url.PathEscape(branch), &tip); reason != "" {
		result["reason"] = reason
		c.JSON(http.StatusOK, result)
		return
	}
	latest := tip.Sha
	result["latest"] = latest
	if current == "" || latest == "" || current == latest {
		c.JSON(http.StatusOK, result)
		return
	}

	// What the branch has that this install lacks. A merge on GitHub makes a new commit even
	// when it changes no file, so a different commit alone is not an update: one is available
	// only when the branch is ahead and some file differs.
	var compare struct {
		AheadBy int               `json:"ahead_by"`
		Files   []json.RawMessage `json:"files"`
	}
	var updateAvailable bool
	if updateCommitPattern.MatchString(current) && updateCommitPattern.MatchString(latest) && githubJSON(ctx, "compare/"+current+"..."+latest, &compare) == "" {
		result["behindBy"] = compare.AheadBy
		updateAvailable = compare.AheadBy > 0 && len(compare.Files) > 0
	} else {
		// The install's commit is not on GitHub (local commits): compare ancestry in the local
		// repository instead
		updateAvailable = true
		if contains, err := isGitAncestor(latest, current); err == nil {
			updateAvailable = !contains
		}
	}

	// Every install that follows the branch updates from its tip, so it is offered only once
	// GitHub's checks on it have passed: not while they run, never when one failed
	if updateAvailable && updateCommitPattern.MatchString(latest) {
		if verification := commitVerification(ctx, latest); verification != "" {
			result["verification"] = verification
			updateAvailable = false
		}
	}
	result["updateAvailable"] = updateAvailable
	c.JSON(http.StatusOK, result)
}

// updateWorkflowPath is the workflow whose run on a commit decides whether installs may update
// to it: the panel's own CI. Other runs on the same commit (Dependabot, CodeQL, scanners) say
// nothing about the panel, and a Dependabot update that cannot be applied fails on every commit.
const updateWorkflowPath = ".github/workflows/ci.yml"

// commitVerification says how the panel's CI went on a commit: "pending" while it runs,
// "failed" when it failed, "" when it passed. A commit without a CI run, or a GitHub that does
// not answer, gives "": the update was already confirmed and is not held back.
func commitVerification(ctx context.Context, sha string) string {
	var answer struct {
		WorkflowRuns []struct {
			Path       string `json:"path"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		} `json:"workflow_runs"`
	}
	if githubJSON(ctx, "actions/runs?head_sha="+url.QueryEscape(sha)+"&per_page=100", &answer) != "" {
		return ""
	}
	pending, failed := false, false
	for _, run := range answer.WorkflowRuns {
		if run.Path != updateWorkflowPath {
			continue
		}
		switch {
		case run.Status != "completed":
			pending = true
		case run.Conclusion == "success":
			// a passing run (a re-run after a flaky one included) is enough
			return ""
		case run.Conclusion == "failure", run.Conclusion == "timed_out", run.Conclusion == "startup_failure", run.Conclusion == "action_required":
			failed = true
		}
	}
	if pending {
		return "pending"
	}
	if failed {
		return "failed"
	}
	return ""
}

// githubRepoAPI is the panel's repository on GitHub's API (a variable so tests can point it at
// a local server)
var githubRepoAPI = "https://api.github.com/repos/Aether-Panel/Panel/"

// updateCommitPattern is a full commit hash, safe to put in the GitHub URL
var updateCommitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// githubJSON reads one GitHub API answer about the panel's repository into target. It returns
// "" on success, or why it could not: "github_rate_limited" (GitHub allows 60 anonymous
// requests an hour per IP), "github_error" or "github_unreachable".
func githubJSON(ctx context.Context, path string, target interface{}) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubRepoAPI+path, nil)
	if err != nil {
		return "github_error"
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Aether-Panel")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logging.Error.Printf("Update check failed to reach GitHub: %v", err)
		return "github_unreachable"
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		logging.Error.Printf("Update check: GitHub refused the request (%d), probably its rate limit", resp.StatusCode)
		return "github_rate_limited"
	case resp.StatusCode != http.StatusOK:
		logging.Error.Printf("Update check got unexpected GitHub response for %s: %d", path, resp.StatusCode)
		return "github_error"
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(target); err != nil {
		logging.Error.Printf("Update check failed to decode GitHub response: %v", err)
		return "github_error"
	}
	return ""
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

// @Summary Off-site backup test
// @Description Checks the S3 settings by writing and deleting a small object in the bucket. The values sent override the saved ones, so they can be tried before saving.
// @Success 204 {object} nil
// @Tags Panel Settings
// @Router /api/settings/test/s3 [post]
// @Security OAuth2Application[settings.edit]
func sendTestS3(c *gin.Context) {
	cfg, _ := offsite.FromSettings()
	var overrides map[string]interface{}
	_ = c.ShouldBindJSON(&overrides)
	dropMaskedSecrets(overrides)
	if v, ok := overrides[config.BackupsS3Endpoint.Key()]; ok {
		cfg.Endpoint = strings.TrimSpace(cast.ToString(v))
	}
	if v, ok := overrides[config.BackupsS3Region.Key()]; ok {
		cfg.Region = strings.TrimSpace(cast.ToString(v))
	}
	if v, ok := overrides[config.BackupsS3Bucket.Key()]; ok {
		cfg.Bucket = strings.TrimSpace(cast.ToString(v))
	}
	if v, ok := overrides[config.BackupsS3AccessKey.Key()]; ok {
		cfg.AccessKey = strings.TrimSpace(cast.ToString(v))
	}
	if v, ok := overrides[config.BackupsS3SecretKey.Key()]; ok {
		cfg.SecretKey = cast.ToString(v)
	}
	if v, ok := overrides[config.BackupsS3Prefix.Key()]; ok {
		cfg.Prefix = strings.Trim(strings.TrimSpace(cast.ToString(v)), "/")
	}
	if v, ok := overrides[config.BackupsS3Insecure.Key()]; ok {
		cfg.Insecure = cast.ToBool(v)
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	if err := offsite.Test(ctx, cfg); err != nil {
		// 422 rather than 502: the panel hides the text of 5xx errors, and the admin needs
		// to read what the storage answered (wrong key, missing bucket...)
		status := http.StatusUnprocessableEntity
		if errors.Is(err, offsite.ErrNotConfigured) {
			status = http.StatusBadRequest
		}
		response.HandleError(c, err, status)
		return
	}
	c.Status(http.StatusNoContent)
}
