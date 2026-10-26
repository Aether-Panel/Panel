package servers

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
)

// Notifications are a server's own alerts, set by its owner: which events, and where they go
// (a Discord channel, a Telegram chat, a JSON webhook, or several). They live in a separate
// 0600 file rather than in the server definition, which more users can read.
type Notifications struct {
	DiscordWebhook string `json:"discordWebhook"`
	// TelegramBotToken and TelegramChatID: the bot from @BotFather and the chat it posts to
	TelegramBotToken string `json:"telegramBotToken"`
	TelegramChatID   string `json:"telegramChatId"`
	// WebhookURL receives every event as JSON; WebhookSecret, when set, signs the body
	WebhookURL    string `json:"webhookUrl"`
	WebhookSecret string `json:"webhookSecret"`

	// Crash: the server died on its own (with the last console lines)
	Crash bool `json:"crash"`
	// Backup: a backup finished or failed
	Backup bool `json:"backup"`
	// Resources: CPU or memory stayed high
	Resources bool `json:"resources"`
	// Power: the server was switched on or off
	Power bool `json:"power"`
}

// DefaultNotifications is what a server starts with: no channel, and the useful events on
// for when one is added
func DefaultNotifications() Notifications {
	return Notifications{Crash: true, Backup: true, Resources: true}
}

func (n Notifications) hasChannel() bool {
	return n.DiscordWebhook != "" || n.TelegramBotToken != "" || n.WebhookURL != ""
}

// Validate checks every channel the owner filled in
func (n *Notifications) Validate() error {
	n.DiscordWebhook = strings.TrimSpace(n.DiscordWebhook)
	n.TelegramBotToken = strings.TrimSpace(n.TelegramBotToken)
	n.TelegramChatID = strings.TrimSpace(n.TelegramChatID)
	n.WebhookURL = strings.TrimSpace(n.WebhookURL)
	if err := services.ValidateDiscordWebhook(n.DiscordWebhook); err != nil {
		return err
	}
	if err := services.ValidateTelegram(n.TelegramBotToken, n.TelegramChatID); err != nil {
		return err
	}
	return services.ValidateWebhook(n.WebhookURL)
}

var notificationsLock sync.Mutex

func notificationsFile(serverID string) string {
	return filepath.Join(config.ServersFolder.Value(), serverID+".notify")
}

func (p *Server) Notifications() (Notifications, error) {
	notificationsLock.Lock()
	defer notificationsLock.Unlock()
	return readNotifications(p.ID())
}

// SetNotifications validates and saves the server's notification settings
func (p *Server) SetNotifications(n Notifications) error {
	if err := n.Validate(); err != nil {
		return err
	}

	notificationsLock.Lock()
	defer notificationsLock.Unlock()
	data, err := json.Marshal(n)
	if err != nil {
		return err
	}
	// webhook URLs and bot tokens are secrets: anyone holding them can post as the owner
	return os.WriteFile(notificationsFile(p.ID()), data, 0600)
}

func readNotifications(serverID string) (Notifications, error) {
	data, err := os.ReadFile(notificationsFile(serverID))
	if os.IsNotExist(err) {
		return DefaultNotifications(), nil
	}
	if err != nil {
		return Notifications{}, err
	}
	n := DefaultNotifications()
	if err := json.Unmarshal(data, &n); err != nil {
		return Notifications{}, err
	}
	return n, nil
}

// Colours, matching the panel's success, warning and destructive tokens
const (
	colorCrash   = 0xF2726B
	colorWarning = 0xF5A524
	colorSuccess = 0x34D399
	colorInfo    = 0x3B9EF5
)

// notificationEvent is one alert, written once and rendered for each channel
type notificationEvent struct {
	Kind    string
	Title   string
	Message string
	Color   int
	Lines   []string
	Extra   map[string]interface{}
}

func (p *Server) displayName() string {
	if p.Server.Display != "" {
		return p.Server.Display
	}
	return p.ID()
}

// discordEmbed renders the event for Discord, with the server name in bold
func (p *Server) discordEmbed(e notificationEvent) services.DiscordEmbed {
	description := strings.Replace(e.Message, p.displayName(), "**"+p.displayName()+"**", 1)
	if len(e.Lines) > 0 {
		log := strings.Join(e.Lines, "\n")
		// Discord caps an embed description at 4096 characters
		if len(log) > 3500 {
			log = log[len(log)-3500:]
		}
		description += "\n```\n" + strings.ReplaceAll(log, "```", "ʼʼʼ") + "\n```"
	}
	return services.DiscordEmbed{
		Title:       e.Title,
		Description: description,
		Color:       e.Color,
		Footer:      &services.DiscordEmbedFooter{Text: "Aether Panel · " + p.ID()},
	}
}

// telegramText renders the event for Telegram's HTML mode
func (p *Server) telegramText(e notificationEvent) string {
	message := services.TelegramHTML(e.Message)
	name := services.TelegramHTML(p.displayName())
	message = strings.Replace(message, name, "<b>"+name+"</b>", 1)
	text := "<b>" + services.TelegramHTML(e.Title) + "</b>\n" + message
	if len(e.Lines) > 0 {
		log := strings.Join(e.Lines, "\n")
		// Telegram caps a message at 4096 characters
		if len(log) > 3000 {
			log = log[len(log)-3000:]
		}
		text += "\n<pre>" + services.TelegramHTML(log) + "</pre>"
	}
	return text + "\n<i>Aether Panel · " + services.TelegramHTML(p.ID()) + "</i>"
}

// webhookPayload renders the event as the JSON a webhook receives
func (p *Server) webhookPayload(e notificationEvent) map[string]interface{} {
	payload := map[string]interface{}{
		"event":     e.Kind,
		"title":     e.Title,
		"message":   e.Message,
		"server":    map[string]string{"id": p.ID(), "name": p.displayName()},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if len(e.Lines) > 0 {
		payload["lines"] = e.Lines
	}
	for k, v := range e.Extra {
		payload[k] = v
	}
	return payload
}

// deliver sends the event to every channel configured, returning each channel's error
func (p *Server) deliver(n Notifications, e notificationEvent) map[string]error {
	results := make(map[string]error)
	ds := services.GetDiscordService()
	if n.DiscordWebhook != "" {
		results["discord"] = ds.SendEmbedToURL(n.DiscordWebhook, p.discordEmbed(e))
	}
	if n.TelegramBotToken != "" {
		results["telegram"] = services.SendTelegram(n.TelegramBotToken, n.TelegramChatID, p.telegramText(e))
	}
	if n.WebhookURL != "" {
		results["webhook"] = services.SendWebhookJSON(n.WebhookURL, n.WebhookSecret, p.webhookPayload(e))
	}
	return results
}

// EventSink, when set, also receives every server event, whatever the owner's alerts are.
// The panel hands them to the installed extensions; it stays nil on a node without a panel.
var EventSink func(event string, data map[string]interface{})

// extensionEvents names each alert kind as the extensions know it
var extensionEvents = map[string]string{
	"crash":     "server.crashed",
	"backup":    "server.backup",
	"resources": "server.resources",
}

// publish passes the event on to EventSink
func (p *Server) publish(e notificationEvent) {
	sink := EventSink
	if sink == nil {
		return
	}
	name, ok := extensionEvents[e.Kind]
	if e.Kind == "power" {
		name, ok = "server.stopped", true
		if running, _ := e.Extra["running"].(bool); running {
			name = "server.started"
		}
	}
	if !ok {
		return
	}
	data := p.webhookPayload(e)
	delete(data, "event")
	delete(data, "timestamp")
	sink(name, data)
}

// notify sends an event if the owner turned it on. It runs in the background, so a slow
// channel never holds up the server.
func (p *Server) notify(enabled func(Notifications) bool, e notificationEvent) {
	p.publish(e)
	n, err := p.Notifications()
	if err != nil || !n.hasChannel() || !enabled(n) {
		return
	}
	go func() {
		for channel, err := range p.deliver(n, e) {
			if err != nil {
				p.Log(logging.Error, "Error sending %s notification: %s", channel, err)
			}
		}
	}()
}

func (p *Server) notifyCrash(report CrashReport) {
	status := "No se reinició automáticamente."
	if report.Restarted {
		status = fmt.Sprintf("Se está reiniciando solo (intento %d de %d).", report.Attempt, report.Limit)
	}
	tail := report.Lines
	if len(tail) > 12 {
		tail = tail[len(tail)-12:]
	}
	p.notify(func(n Notifications) bool { return n.Crash }, notificationEvent{
		Kind:    "crash",
		Title:   "💥 Caída del servidor",
		Message: fmt.Sprintf("%s se ha caído (código de salida %d). %s", p.displayName(), report.ExitCode, status),
		Color:   colorCrash,
		Lines:   tail,
		Extra:   map[string]interface{}{"exitCode": report.ExitCode, "restarted": report.Restarted},
	})
}

func (p *Server) notifyBackup(success bool) {
	e := notificationEvent{
		Kind:    "backup",
		Title:   "✅ Copia de seguridad completada",
		Message: fmt.Sprintf("La copia de seguridad de %s se ha guardado correctamente.", p.displayName()),
		Color:   colorSuccess,
		Extra:   map[string]interface{}{"success": true},
	}
	if !success {
		e.Title = "❌ La copia de seguridad falló"
		e.Message = fmt.Sprintf("No se pudo completar la copia de seguridad de %s. Revisa la consola del servidor.", p.displayName())
		e.Color = colorCrash
		e.Extra["success"] = false
	}
	p.notify(func(n Notifications) bool { return n.Backup }, e)
}

func (p *Server) notifyResource(resource string, value, threshold float64) {
	p.notify(func(n Notifications) bool { return n.Resources }, notificationEvent{
		Kind:    "resources",
		Title:   "⚠️ Uso alto de " + strings.ToLower(resource),
		Message: fmt.Sprintf("%s está usando el %.0f %% de %s (aviso a partir del %.0f %%).", p.displayName(), value, strings.ToLower(resource), threshold),
		Color:   colorWarning,
		Extra:   map[string]interface{}{"resource": resource, "value": value, "threshold": threshold},
	})
}

func (p *Server) notifyPower(running bool) {
	e := notificationEvent{
		Kind:    "power",
		Title:   "🟢 Servidor encendido",
		Message: fmt.Sprintf("%s está en línea.", p.displayName()),
		Color:   colorSuccess,
		Extra:   map[string]interface{}{"running": true},
	}
	if !running {
		e.Title = "⚪ Servidor apagado"
		e.Message = fmt.Sprintf("%s se ha detenido.", p.displayName())
		e.Color = colorInfo
		e.Extra["running"] = false
	}
	p.notify(func(n Notifications) bool { return n.Power }, e)
}

// ErrNoNotificationChannel is returned by a test when the owner has not set any channel yet
var ErrNoNotificationChannel = errors.New("add a Discord webhook, a Telegram bot or a webhook URL first")

// SendTestNotification posts a sample message to every channel, so the owner can check them.
// It returns each channel's result ("ok" or the reason it failed).
func (p *Server) SendTestNotification() (map[string]string, error) {
	n, err := p.Notifications()
	if err != nil {
		return nil, err
	}
	if !n.hasChannel() {
		return nil, ErrNoNotificationChannel
	}
	results := make(map[string]string)
	for channel, err := range p.deliver(n, notificationEvent{
		Kind:    "test",
		Title:   "🔔 Notificaciones conectadas",
		Message: fmt.Sprintf("A partir de ahora recibirás aquí los avisos de %s.", p.displayName()),
		Color:   colorInfo,
	}) {
		if err != nil {
			results[channel] = err.Error()
		} else {
			results[channel] = "ok"
		}
	}
	return results, nil
}
