package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
)

// notification text keys used by several reports
const (
	textFieldServer       = "field.server"
	textFieldStatus       = "field.status"
	textSystemStatusTotal = "systemStatus.total"
)

var discordServiceInstance *DiscordService

type DiscordService struct{}

type DiscordWebhookPayload struct {
	Content  string         `json:"content,omitempty"`
	Username string         `json:"username,omitempty"`
	Embeds   []DiscordEmbed `json:"embeds,omitempty"`
}

type DiscordEmbed struct {
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color,omitempty"`
	Fields      []DiscordEmbedField `json:"fields,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"`
	Footer      *DiscordEmbedFooter `json:"footer,omitempty"`
}

type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type DiscordEmbedFooter struct {
	Text string `json:"text"`
}

func GetDiscordService() *DiscordService {
	if discordServiceInstance == nil {
		discordServiceInstance = &DiscordService{}
	}
	return discordServiceInstance
}

func (ds *DiscordService) SendWebhook(title, description string, color int, fields []DiscordEmbedField) error {
	return ds.SendWebhookToURL(config.DiscordWebhook.Value(), title, description, color, fields)
}

// SendWebhookToURL envía un mensaje a un webhook específico
func (ds *DiscordService) SendWebhookToURL(webhookURL, title, description string, color int, fields []DiscordEmbedField) error {
	if webhookURL == "" {
		return nil // Silently skip if webhook is not configured
	}

	return ds.SendEmbedToURL(webhookURL, DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       color,
		Fields:      fields,
	})
}

// discordWebhookPattern is what a Discord webhook URL looks like. Server owners can set
// their own webhook, so anything else is refused rather than requested by the panel.
var discordWebhookPattern = regexp.MustCompile(`^https://(?:(?:ptb|canary)\.)?discord(?:app)?\.com/api/webhooks/\d+/[\w-]+$`)

// discordWebhookPattern and discordHTTPClient are swapped in tests, which post to a local server
var (
	defaultDiscordHTTPClient = utils.NewRestrictedHTTPClient
	discordHTTPClient        = defaultDiscordHTTPClient
)

var ErrInvalidDiscordWebhook = errors.New("the URL is not a Discord webhook (https://discord.com/api/webhooks/...)")

// ValidateDiscordWebhook accepts an empty URL (notifications off) or a Discord webhook URL
func ValidateDiscordWebhook(webhookURL string) error {
	if webhookURL == "" || discordWebhookPattern.MatchString(webhookURL) {
		return nil
	}
	return ErrInvalidDiscordWebhook
}

// SendEmbedToURL posts one embed to a Discord webhook. The URL must be a Discord webhook and
// the request goes through a client that refuses private addresses, so a webhook setting can
// never be used to reach the node's own network.
func (ds *DiscordService) SendEmbedToURL(webhookURL string, embed DiscordEmbed) error {
	if webhookURL == "" {
		return nil
	}
	if !discordWebhookPattern.MatchString(webhookURL) {
		return ErrInvalidDiscordWebhook
	}
	if embed.Timestamp == "" {
		embed.Timestamp = time.Now().Format(time.RFC3339)
	}
	if embed.Footer == nil {
		embed.Footer = &DiscordEmbedFooter{Text: "Aether Panel"}
	}

	jsonData, err := json.Marshal(DiscordWebhookPayload{Embeds: []DiscordEmbed{embed}})
	if err != nil {
		logging.Error.Printf("Error marshaling Discord webhook payload: %v", err)
		return err
	}

	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		logging.Error.Printf("Error creating Discord webhook request: %v", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := discordHTTPClient().Do(req)
	if err != nil {
		logging.Error.Printf("Error sending Discord webhook: %v", err)
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		logging.Error.Printf("Discord webhook returned status %d: %s", resp.StatusCode, string(body))
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// SendAlert envía una alerta genérica
func (ds *DiscordService) SendAlert(title, description string, fields []DiscordEmbedField) error {
	return ds.SendWebhook(title, description, 0xFF0000, fields) // Rojo para alertas
}

// SendServerOfflineAlert envía alerta cuando un servidor se desconecta
func (ds *DiscordService) SendServerOfflineAlert(serverName, serverID string) error {
	fields := []DiscordEmbedField{
		{Name: NotificationText(textFieldServer), Value: serverName, Inline: true},
		{Name: "ID", Value: serverID, Inline: true},
		{Name: NotificationText(textFieldStatus), Value: "🔴 Offline", Inline: true},
	}
	return ds.SendAlert(NotificationText("serverOffline.title"), NotificationText("serverOffline.description", serverName), fields)
}

// SendServerOnlineAlert envía alerta cuando un servidor se conecta
func (ds *DiscordService) SendServerOnlineAlert(serverName, serverID string) error {
	fields := []DiscordEmbedField{
		{Name: NotificationText(textFieldServer), Value: serverName, Inline: true},
		{Name: "ID", Value: serverID, Inline: true},
		{Name: NotificationText(textFieldStatus), Value: "🟢 Online", Inline: true},
	}
	return ds.SendWebhook(NotificationText("serverOnline.title"), NotificationText("serverOnline.description", serverName), 0x00FF00, fields) // Verde
}

// SendResourceAlert envía alerta cuando el uso de recursos es alto. resourceType is the
// resource's name already in the notification language.
func (ds *DiscordService) SendResourceAlert(serverName, serverID, resourceType string, currentValue, threshold float64) error {
	fields := []DiscordEmbedField{
		{Name: NotificationText(textFieldServer), Value: serverName, Inline: true},
		{Name: "ID", Value: serverID, Inline: true},
		{Name: NotificationText("field.resource"), Value: resourceType, Inline: true},
		{Name: NotificationText("field.currentUsage"), Value: fmt.Sprintf("%.1f%%", currentValue), Inline: true},
		{Name: NotificationText("field.threshold"), Value: fmt.Sprintf("%.1f%%", threshold), Inline: true},
	}

	title := NotificationText("resourceAlert.title", resourceType)
	description := NotificationText("resourceAlert.description", serverName, currentValue, resourceType, threshold)

	return ds.SendAlert(title, description, fields)
}

// SendBackupAlert envía alerta sobre backups
func (ds *DiscordService) SendBackupAlert(serverName, serverID, status string, isSuccess bool) error {
	fields := []DiscordEmbedField{
		{Name: NotificationText(textFieldServer), Value: serverName, Inline: true},
		{Name: "ID", Value: serverID, Inline: true},
		{Name: NotificationText(textFieldStatus), Value: status, Inline: true},
	}

	var title, description string
	var color int

	if isSuccess {
		title = NotificationText("backupAlert.successTitle")
		description = NotificationText("backupAlert.success", serverName)
		color = 0x00FF00 // Verde
	} else {
		title = NotificationText("backupAlert.failedTitle")
		description = NotificationText("backupAlert.failed", serverName, status)
		color = 0xFF0000 // Rojo
	}

	return ds.SendWebhook(title, description, color, fields)
}

// SendSystemStatus envía un resumen del estado completo del sistema
func (ds *DiscordService) SendSystemStatus(servers []ServerInfo) error {
	webhookURL := config.DiscordWebhookSystem.Value()
	if webhookURL == "" {
		return nil // Silently skip if webhook is not configured
	}

	if len(servers) == 0 {
		return nil // No hay servidores para reportar
	}

	description := systemStatusDescription(servers)
	fields := systemStatusFields(servers)

	color := 0x3498DB // Azul para status general

	embed := DiscordEmbed{
		Title:       NotificationText("systemStatus.title"),
		Description: description,
		Color:       color,
		Fields:      fields,
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &DiscordEmbedFooter{
			Text: NotificationText("systemStatus.footer"),
		},
	}

	payload := DiscordWebhookPayload{
		Embeds: []DiscordEmbed{embed},
	}

	return postDiscordPayload(webhookURL, payload)
}

// systemStatusDescription builds the summary line with online/offline counts and average usage
func systemStatusDescription(servers []ServerInfo) string {
	// Calcular estadísticas generales
	totalServers := len(servers)
	onlineServers := 0
	offlineServers := 0
	totalCPU := 0.0
	totalMemory := 0.0
	onlineCount := 0

	for _, s := range servers {
		if s.IsRunning {
			onlineServers++
			totalCPU += s.CPU
			totalMemory += s.Memory
			onlineCount++
		} else {
			offlineServers++
		}
	}

	avgCPU := 0.0
	avgMemory := 0.0
	if onlineCount > 0 {
		avgCPU = totalCPU / float64(onlineCount)
		avgMemory = totalMemory / float64(onlineCount)
	}

	// Crear descripción principal
	description := NotificationText(textSystemStatusTotal, totalServers) + "\n"
	description += fmt.Sprintf("🟢 **Online:** %d | 🔴 **Offline:** %d\n", onlineServers, offlineServers)

	if onlineCount > 0 {
		description += "\n" + NotificationText("systemStatus.averages") + "\n"
		description += fmt.Sprintf("• CPU: %.1f%%\n", avgCPU)
		description += NotificationText("systemStatus.memory", avgMemory/1024/1024) + "\n" // Convertir bytes a MB si es necesario
	}
	return description
}

// discordMaxEmbedFields is the most fields Discord accepts in one embed; with one more it
// refuses the whole message
const discordMaxEmbedFields = 25

// systemStatusFields builds one field per server. Above Discord's field limit it shows the
// first servers and spends the last field on a note saying how many are left out (the counts
// in the description still cover all of them); it used to add that note as a 26th field,
// which made Discord reject the report.
func systemStatusFields(servers []ServerInfo) []DiscordEmbedField {
	serversToShow := len(servers)
	if serversToShow > discordMaxEmbedFields {
		serversToShow = discordMaxEmbedFields - 1
	}

	fields := make([]DiscordEmbedField, 0, serversToShow+1)
	for i := 0; i < serversToShow; i++ {
		fields = append(fields, serverStatusField(servers[i]))
	}

	if serversToShow < len(servers) {
		fields = append(fields, DiscordEmbedField{
			Name:  NotificationText("field.note"),
			Value: NotificationText("systemStatus.showing", serversToShow, len(servers)),
		})
	}
	return fields
}

func serverStatusField(s ServerInfo) DiscordEmbedField {
	statusEmoji := "🔴"
	if s.IsRunning {
		statusEmoji = "🟢"
	}

	memoryStr := fmt.Sprintf("%.1f MB", s.Memory/1024/1024)
	if s.Memory < 1024*1024*10 { // Si es menor a 10 MB, mostrar como porcentaje
		memoryStr = fmt.Sprintf("%.1f%%", s.Memory)
	}

	statusText := "Offline"
	if s.IsRunning {
		statusText = "Online"
	}

	value := fmt.Sprintf("%s %s\nCPU: %.1f%% | RAM: %s",
		statusEmoji,
		statusText,
		s.CPU,
		memoryStr)

	return DiscordEmbedField{
		Name:   s.Name,
		Value:  value,
		Inline: true,
	}
}

// postDiscordPayload sends the system status payload to the webhook. Like SendEmbedToURL it
// only posts to a Discord webhook and through the client that refuses private addresses.
func postDiscordPayload(webhookURL string, payload DiscordWebhookPayload) error {
	if webhookURL == "" {
		return nil
	}
	if !discordWebhookPattern.MatchString(webhookURL) {
		return ErrInvalidDiscordWebhook
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		logging.Error.Printf("Error marshaling Discord webhook payload: %v", err)
		return err
	}

	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		logging.Error.Printf("Error creating Discord webhook request: %v", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := discordHTTPClient().Do(req)
	if err != nil {
		logging.Error.Printf("Error sending Discord webhook: %v", err)
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		logging.Error.Printf("Discord webhook returned status %d: %s", resp.StatusCode, string(body))
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// ServerInfo contiene la información de un servidor para el reporte
type ServerInfo struct {
	Name      string
	ID        string
	IsRunning bool
	CPU       float64
	Memory    float64
}

// formatBytes formatea bytes a formato legible (GB, MB, etc.)
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// SendNodeStatus envía información sobre el estado del nodo
func (ds *DiscordService) SendNodeStatus(totalServers, onlineServers, offlineServers int, nodeName string, publicHost string, publicPort uint16, cpuModel string, cpuCores int, cpuThreads int, cpuGhz float64, memTotal uint64, memUsed uint64, osName string) error {
	webhookURL := config.DiscordWebhookNode.Value()
	if webhookURL == "" {
		return nil // Silently skip if webhook is not configured
	}

	if nodeName == "" {
		nodeName = NotificationText("nodeStatus.localNode")
	}

	description := NotificationText("nodeStatus.node", nodeName) + "\n"
	if publicHost != "" {
		description += NotificationText("nodeStatus.address", nodeAddress(publicHost, publicPort)) + "\n"
	}
	description += NotificationText(textSystemStatusTotal, totalServers) + "\n"
	description += fmt.Sprintf("🟢 **Online:** %d | 🔴 **Offline:** %d\n", onlineServers, offlineServers)

	fields := []DiscordEmbedField{
		{Name: NotificationText(textFieldStatus), Value: NotificationText("nodeStatus.active"), Inline: true},
		{Name: NotificationText("field.totalServers"), Value: fmt.Sprintf("%d", totalServers), Inline: true},
		{Name: NotificationText("field.onlineServers"), Value: fmt.Sprintf("%d", onlineServers), Inline: true},
		{Name: NotificationText("field.offlineServers"), Value: fmt.Sprintf("%d", offlineServers), Inline: true},
	}

	if publicHost != "" {
		fields = append(fields, DiscordEmbedField{Name: "Host", Value: nodeAddress(publicHost, publicPort), Inline: true})
	}

	// Información del CPU
	if cpuModel != "Desconocido" {
		fields = append(fields, DiscordEmbedField{Name: "CPU", Value: cpuModel, Inline: false})
	}
	if cpuCores > 0 {
		fields = append(fields, DiscordEmbedField{Name: NotificationText("field.cores"), Value: fmt.Sprintf("%d", cpuCores), Inline: true})
	}
	if cpuThreads > 0 {
		fields = append(fields, DiscordEmbedField{Name: NotificationText("field.threads"), Value: fmt.Sprintf("%d", cpuThreads), Inline: true})
	}
	if cpuGhz > 0 {
		fields = append(fields, DiscordEmbedField{Name: NotificationText("field.frequency"), Value: fmt.Sprintf("%.2f GHz", cpuGhz), Inline: true})
	}

	// Información de RAM
	if memTotal > 0 {
		memUsagePercent := float64(memUsed) / float64(memTotal) * 100.0
		fields = append(fields, DiscordEmbedField{Name: NotificationText("field.ramTotal"), Value: formatBytes(memTotal), Inline: true})
		fields = append(fields, DiscordEmbedField{Name: NotificationText("field.ramUsed"), Value: fmt.Sprintf("%s (%.1f%%)", formatBytes(memUsed), memUsagePercent), Inline: true})
		fields = append(fields, DiscordEmbedField{Name: NotificationText("field.ramFree"), Value: formatBytes(memTotal - memUsed), Inline: true})
	}

	// Sistema operativo
	if osName != "" {
		fields = append(fields, DiscordEmbedField{Name: NotificationText("field.os"), Value: osName, Inline: true})
	}

	return ds.SendWebhookToURL(webhookURL, NotificationText("nodeStatus.title"), description, nodeStatusColor(onlineServers, offlineServers), fields)
}

// SendNodeUnreachable reports a node that did not answer the panel. Its servers' states and its
// machine are unknown, so only what the panel knows itself is shown: how many servers it places
// there. SendNodeStatus is for nodes that answered, and always says they are active.
func (ds *DiscordService) SendNodeUnreachable(totalServers int, nodeName string, publicHost string, publicPort uint16) error {
	webhookURL := config.DiscordWebhookNode.Value()
	if webhookURL == "" {
		return nil // Silently skip if webhook is not configured
	}

	description, fields := nodeUnreachableReport(totalServers, nodeName, publicHost, publicPort)
	return ds.SendWebhookToURL(webhookURL, NotificationText("nodeStatus.title"), description, 0xFF0000, fields)
}

// nodeUnreachableReport is the description and fields of SendNodeUnreachable
func nodeUnreachableReport(totalServers int, nodeName string, publicHost string, publicPort uint16) (string, []DiscordEmbedField) {
	description := NotificationText("nodeStatus.node", nodeName) + "\n"
	if publicHost != "" {
		description += NotificationText("nodeStatus.address", nodeAddress(publicHost, publicPort)) + "\n"
	}
	description += NotificationText(textSystemStatusTotal, totalServers) + "\n"
	description += NotificationText("nodeStatus.unreachable") + "\n"

	fields := []DiscordEmbedField{
		{Name: NotificationText(textFieldStatus), Value: NotificationText("nodeStatus.noAnswer"), Inline: true},
		{Name: NotificationText("field.totalServers"), Value: fmt.Sprintf("%d", totalServers), Inline: true},
	}
	if publicHost != "" {
		fields = append(fields, DiscordEmbedField{Name: "Host", Value: nodeAddress(publicHost, publicPort), Inline: true})
	}
	return description, fields
}

// nodeAddress is host:port, or just the host when no port is known
func nodeAddress(publicHost string, publicPort uint16) string {
	if publicPort > 0 {
		return fmt.Sprintf("%s:%d", publicHost, publicPort)
	}
	return publicHost
}

func nodeStatusColor(onlineServers, offlineServers int) int {
	color := 0x00FF00 // Verde para nodo activo
	if offlineServers > onlineServers && onlineServers == 0 {
		color = 0xFF0000 // Rojo si todos están offline
	} else if offlineServers > 0 {
		color = 0xFFA500 // Naranja si hay algunos offline
	}
	return color
}
