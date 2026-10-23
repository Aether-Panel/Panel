package servers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
)

// Notifications are a server's own Discord alerts, set by its owner. They live in a
// separate 0600 file rather than in the server definition, which more users can read.
type Notifications struct {
	DiscordWebhook string `json:"discordWebhook"`
	// Crash: the server died on its own (with the last console lines)
	Crash bool `json:"crash"`
	// Backup: a backup finished or failed
	Backup bool `json:"backup"`
	// Resources: CPU or memory stayed high
	Resources bool `json:"resources"`
	// Power: the server was switched on or off
	Power bool `json:"power"`
}

// DefaultNotifications is what a server starts with: no webhook, and the useful events on
// for when one is added
func DefaultNotifications() Notifications {
	return Notifications{Crash: true, Backup: true, Resources: true}
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
	n.DiscordWebhook = strings.TrimSpace(n.DiscordWebhook)
	if err := services.ValidateDiscordWebhook(n.DiscordWebhook); err != nil {
		return err
	}

	notificationsLock.Lock()
	defer notificationsLock.Unlock()
	data, err := json.Marshal(n)
	if err != nil {
		return err
	}
	// the webhook URL is a secret: anyone holding it can post to the owner's channel
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

// Embed colours, matching the panel's success, warning and destructive tokens
const (
	colorCrash   = 0xF2726B
	colorWarning = 0xF5A524
	colorSuccess = 0x34D399
	colorInfo    = 0x3B9EF5
)

// notify sends an event to the server's webhook if the owner turned that event on. It runs
// in the background so a slow Discord never holds up the server.
func (p *Server) notify(enabled func(Notifications) bool, embed services.DiscordEmbed) {
	n, err := p.Notifications()
	if err != nil || n.DiscordWebhook == "" || !enabled(n) {
		return
	}
	if embed.Footer == nil {
		embed.Footer = &services.DiscordEmbedFooter{Text: "Aether Panel · " + p.ID()}
	}
	go func() {
		if err := services.GetDiscordService().SendEmbedToURL(n.DiscordWebhook, embed); err != nil {
			p.Log(logging.Error, "Error sending Discord notification: %s", err)
		}
	}()
}

func (p *Server) displayName() string {
	if p.Server.Display != "" {
		return p.Server.Display
	}
	return p.ID()
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
	log := strings.Join(tail, "\n")
	// Discord caps an embed description at 4096 characters
	if len(log) > 3500 {
		log = log[len(log)-3500:]
	}
	description := fmt.Sprintf("**%s** se ha caído (código de salida %d).\n%s", p.displayName(), report.ExitCode, status)
	if log != "" {
		description += "\n```\n" + strings.ReplaceAll(log, "```", "ʼʼʼ") + "\n```"
	}
	p.notify(func(n Notifications) bool { return n.Crash }, services.DiscordEmbed{
		Title:       "💥 Caída del servidor",
		Description: description,
		Color:       colorCrash,
	})
}

func (p *Server) notifyBackup(success bool) {
	embed := services.DiscordEmbed{
		Title:       "✅ Copia de seguridad completada",
		Description: fmt.Sprintf("La copia de seguridad de **%s** se ha guardado correctamente.", p.displayName()),
		Color:       colorSuccess,
	}
	if !success {
		embed.Title = "❌ La copia de seguridad falló"
		embed.Description = fmt.Sprintf("No se pudo completar la copia de seguridad de **%s**. Revisa la consola del servidor.", p.displayName())
		embed.Color = colorCrash
	}
	p.notify(func(n Notifications) bool { return n.Backup }, embed)
}

func (p *Server) notifyResource(resource string, value, threshold float64) {
	p.notify(func(n Notifications) bool { return n.Resources }, services.DiscordEmbed{
		Title:       "⚠️ Uso alto de " + resource,
		Description: fmt.Sprintf("**%s** está usando %.0f%% de %s (aviso a partir de %.0f%%).", p.displayName(), value, strings.ToLower(resource), threshold),
		Color:       colorWarning,
	})
}

func (p *Server) notifyPower(running bool) {
	embed := services.DiscordEmbed{
		Title:       "🟢 Servidor encendido",
		Description: fmt.Sprintf("**%s** está en línea.", p.displayName()),
		Color:       colorSuccess,
	}
	if !running {
		embed.Title = "⚪ Servidor apagado"
		embed.Description = fmt.Sprintf("**%s** se ha detenido.", p.displayName())
		embed.Color = colorInfo
	}
	p.notify(func(n Notifications) bool { return n.Power }, embed)
}

// SendTestNotification posts a sample message so the owner can check the webhook
func (p *Server) SendTestNotification() error {
	n, err := p.Notifications()
	if err != nil {
		return err
	}
	if n.DiscordWebhook == "" {
		return services.ErrInvalidDiscordWebhook
	}
	return services.GetDiscordService().SendEmbedToURL(n.DiscordWebhook, services.DiscordEmbed{
		Title:       "🔔 Notificaciones conectadas",
		Description: fmt.Sprintf("A partir de ahora recibirás aquí los avisos de **%s**.", p.displayName()),
		Color:       colorInfo,
		Footer:      &services.DiscordEmbedFooter{Text: "Aether Panel · " + p.ID()},
	})
}
