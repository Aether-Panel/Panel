package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/SkyPanel/SkyPanel/v3/config"
	"github.com/SkyPanel/SkyPanel/v3/logging"
	"io"
	"net/http"
	"time"
)

var discordServiceInstance *DiscordService

type DiscordService struct{}

type DiscordWebhookPayload struct {
	Content   string                `json:"content,omitempty"`
	Username  string                `json:"username,omitempty"`
	Embeds    []DiscordEmbed        `json:"embeds,omitempty"`
}

type DiscordEmbed struct {
	Title       string                `json:"title,omitempty"`
	Description string                `json:"description,omitempty"`
	Color       int                   `json:"color,omitempty"`
	Fields      []DiscordEmbedField   `json:"fields,omitempty"`
	Timestamp   string                `json:"timestamp,omitempty"`
	Footer      *DiscordEmbedFooter   `json:"footer,omitempty"`
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

	embed := DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       color,
		Fields:      fields,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	payload := DiscordWebhookPayload{
		Embeds: []DiscordEmbed{embed},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		logging.Error.Printf("Error marshaling Discord webhook payload: %v", err)
		return err
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		logging.Error.Printf("Error creating Discord webhook request: %v", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		logging.Error.Printf("Error sending Discord webhook: %v", err)
		return err
	}
	defer func() {
		if resp.Body != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
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
		{Name: "Servidor", Value: serverName, Inline: true},
		{Name: "ID", Value: serverID, Inline: true},
		{Name: "Estado", Value: "🔴 Offline", Inline: true},
	}
	return ds.SendAlert("⚠️ Servidor Desconectado", fmt.Sprintf("El servidor **%s** se ha desconectado o está offline.", serverName), fields)
}

// SendServerOnlineAlert envía alerta cuando un servidor se conecta
func (ds *DiscordService) SendServerOnlineAlert(serverName, serverID string) error {
	fields := []DiscordEmbedField{
		{Name: "Servidor", Value: serverName, Inline: true},
		{Name: "ID", Value: serverID, Inline: true},
		{Name: "Estado", Value: "🟢 Online", Inline: true},
	}
	return ds.SendWebhook("✅ Servidor Conectado", fmt.Sprintf("El servidor **%s** está ahora online.", serverName), 0x00FF00, fields) // Verde
}

// SendResourceAlert envía alerta cuando el uso de recursos es alto
func (ds *DiscordService) SendResourceAlert(serverName, serverID, resourceType string, currentValue, threshold float64) error {
	fields := []DiscordEmbedField{
		{Name: "Servidor", Value: serverName, Inline: true},
		{Name: "ID", Value: serverID, Inline: true},
		{Name: "Recurso", Value: resourceType, Inline: true},
		{Name: "Uso Actual", Value: fmt.Sprintf("%.1f%%", currentValue), Inline: true},
		{Name: "Umbral", Value: fmt.Sprintf("%.1f%%", threshold), Inline: true},
	}
	
	title := fmt.Sprintf("⚠️ Alto Uso de %s", resourceType)
	description := fmt.Sprintf("El servidor **%s** está usando **%.1f%%** de %s (umbral: %.1f%%)", serverName, currentValue, resourceType, threshold)
	
	return ds.SendAlert(title, description, fields)
}

// SendBackupAlert envía alerta sobre backups
func (ds *DiscordService) SendBackupAlert(serverName, serverID, status string, isSuccess bool) error {
	fields := []DiscordEmbedField{
		{Name: "Servidor", Value: serverName, Inline: true},
		{Name: "ID", Value: serverID, Inline: true},
		{Name: "Estado", Value: status, Inline: true},
	}
	
	var title, description string
	var color int
	
	if isSuccess {
		title = "✅ Backup Completado"
		description = fmt.Sprintf("El backup del servidor **%s** se completó exitosamente.", serverName)
		color = 0x00FF00 // Verde
	} else {
		title = "❌ Backup Fallido"
		description = fmt.Sprintf("El backup del servidor **%s** falló: %s", serverName, status)
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
	description := fmt.Sprintf("**Total de Servidores:** %d\n", totalServers)
	description += fmt.Sprintf("🟢 **Online:** %d | 🔴 **Offline:** %d\n", onlineServers, offlineServers)
	
	if onlineCount > 0 {
		description += fmt.Sprintf("\n**Promedio de Recursos (Servidores Online):**\n")
		description += fmt.Sprintf("• CPU: %.1f%%\n", avgCPU)
		description += fmt.Sprintf("• Memoria: %.1f MB\n", avgMemory/1024/1024) // Convertir bytes a MB si es necesario
	}

	// Crear campos para cada servidor (limitado a 25 por limitación de Discord)
	maxFields := 25
	serversToShow := len(servers)
	if serversToShow > maxFields {
		serversToShow = maxFields
	}

	fields := make([]DiscordEmbedField, 0)
	for i := 0; i < serversToShow; i++ {
		s := servers[i]
		
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
		
		fields = append(fields, DiscordEmbedField{
			Name:   s.Name,
			Value:  value,
			Inline: true,
		})
	}

	if len(servers) > maxFields {
		fields = append(fields, DiscordEmbedField{
			Name:  "Nota",
			Value: fmt.Sprintf("Mostrando %d de %d servidores", serversToShow, len(servers)),
		})
	}

	color := 0x3498DB // Azul para status general
	
	embed := DiscordEmbed{
		Title:       "📊 Estado del Sistema",
		Description: description,
		Color:       color,
		Fields:      fields,
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &DiscordEmbedFooter{
			Text: fmt.Sprintf("Actualizado cada minuto"),
		},
	}

	payload := DiscordWebhookPayload{
		Embeds: []DiscordEmbed{embed},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		logging.Error.Printf("Error marshaling Discord webhook payload: %v", err)
		return err
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		logging.Error.Printf("Error creating Discord webhook request: %v", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		logging.Error.Printf("Error sending Discord webhook: %v", err)
		return err
	}
	defer func() {
		if resp.Body != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
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
		nodeName = "Nodo Local"
	}

	description := fmt.Sprintf("**Nodo:** %s\n", nodeName)
	if publicHost != "" {
		if publicPort > 0 {
			description += fmt.Sprintf("**Dirección:** %s:%d\n", publicHost, publicPort)
		} else {
			description += fmt.Sprintf("**Dirección:** %s\n", publicHost)
		}
	}
	description += fmt.Sprintf("**Total de Servidores:** %d\n", totalServers)
	description += fmt.Sprintf("🟢 **Online:** %d | 🔴 **Offline:** %d\n", onlineServers, offlineServers)

	fields := []DiscordEmbedField{
		{Name: "Estado", Value: "🟢 Activo", Inline: true},
		{Name: "Servidores Totales", Value: fmt.Sprintf("%d", totalServers), Inline: true},
		{Name: "Servidores Online", Value: fmt.Sprintf("%d", onlineServers), Inline: true},
		{Name: "Servidores Offline", Value: fmt.Sprintf("%d", offlineServers), Inline: true},
	}

	if publicHost != "" {
		if publicPort > 0 {
			fields = append(fields, DiscordEmbedField{Name: "Host", Value: fmt.Sprintf("%s:%d", publicHost, publicPort), Inline: true})
		} else {
			fields = append(fields, DiscordEmbedField{Name: "Host", Value: publicHost, Inline: true})
		}
	}

	// Información del CPU
	if cpuModel != "Desconocido" {
		fields = append(fields, DiscordEmbedField{Name: "CPU", Value: cpuModel, Inline: false})
	}
	if cpuCores > 0 {
		fields = append(fields, DiscordEmbedField{Name: "Núcleos", Value: fmt.Sprintf("%d", cpuCores), Inline: true})
	}
	if cpuThreads > 0 {
		fields = append(fields, DiscordEmbedField{Name: "Hilos", Value: fmt.Sprintf("%d", cpuThreads), Inline: true})
	}
	if cpuGhz > 0 {
		fields = append(fields, DiscordEmbedField{Name: "Frecuencia", Value: fmt.Sprintf("%.2f GHz", cpuGhz), Inline: true})
	}

	// Información de RAM
	if memTotal > 0 {
		memUsagePercent := float64(memUsed) / float64(memTotal) * 100.0
		fields = append(fields, DiscordEmbedField{Name: "RAM Total", Value: formatBytes(memTotal), Inline: true})
		fields = append(fields, DiscordEmbedField{Name: "RAM Usada", Value: fmt.Sprintf("%s (%.1f%%)", formatBytes(memUsed), memUsagePercent), Inline: true})
		fields = append(fields, DiscordEmbedField{Name: "RAM Libre", Value: formatBytes(memTotal-memUsed), Inline: true})
	}

	// Sistema operativo
	if osName != "" {
		fields = append(fields, DiscordEmbedField{Name: "Sistema Operativo", Value: osName, Inline: true})
	}

	color := 0x00FF00 // Verde para nodo activo
	if offlineServers > onlineServers && onlineServers == 0 {
		color = 0xFF0000 // Rojo si todos están offline
	} else if offlineServers > 0 {
		color = 0xFFA500 // Naranja si hay algunos offline
	}

	return ds.SendWebhookToURL(webhookURL, "🖥️ Estado del Nodo", description, color, fields)
}

