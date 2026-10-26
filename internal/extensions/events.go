package extensions

import (
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"gorm.io/gorm"
)

// retryDelays are the waits before the 2nd and 3rd attempt of a delivery that failed
var retryDelays = []time.Duration{5 * time.Second, 30 * time.Second}

// maxDeliveries caps how many events are in flight at once, so a slow extension cannot pile
// up goroutines; past it, events are dropped (and logged) rather than queued forever
const maxDeliveries = 32

// send posts one event; tests replace it
var send = services.SendWebhookJSON

type subscriber struct {
	id, webhook, secret string
	manifest            *Manifest
}

var (
	activeMu sync.RWMutex
	loaded   bool
	active   []subscriber
	inFlight = make(chan struct{}, maxDeliveries)
)

// Reload reads which extensions are switched on, after any change to them
func Reload(db *gorm.DB) {
	var list []models.Extension
	if err := db.Where("enabled = ?", true).Find(&list).Error; err != nil {
		logging.Error.Printf("Could not load the extensions: %s", err)
		return
	}
	subs := make([]subscriber, 0, len(list))
	for i := range list {
		m, err := ManifestOf(&list[i])
		if err != nil {
			logging.Error.Printf("Extension %s has an unreadable manifest: %s", list[i].ID, err)
			continue
		}
		subs = append(subs, subscriber{id: list[i].ID, webhook: list[i].WebhookURL, secret: list[i].Secret, manifest: m})
	}
	activeMu.Lock()
	active, loaded = subs, true
	activeMu.Unlock()
}

func subscribers(event string) []subscriber {
	activeMu.RLock()
	ready := loaded
	activeMu.RUnlock()
	if !ready {
		db, err := database.GetConnection()
		if err != nil {
			return nil
		}
		Reload(db)
	}

	activeMu.RLock()
	defer activeMu.RUnlock()
	var out []subscriber
	for _, s := range active {
		if s.webhook != "" && s.manifest.Subscribed(event) {
			out = append(out, s)
		}
	}
	return out
}

// Publish sends the event to every switched-on extension that subscribed to it, in the
// background. data is added to the body next to "event", "extension" and "timestamp".
//
// Nothing runs on the caller's goroutine: callers are often inside a database transaction,
// and with SQLite's single connection, looking up the subscribers there would wait forever.
func Publish(event string, data map[string]interface{}) {
	go dispatch(event, data)
}

func dispatch(event string, data map[string]interface{}) {
	for _, s := range subscribers(event) {
		body := map[string]interface{}{}
		for k, v := range data {
			body[k] = v
		}
		body["event"] = event
		body["extension"] = s.id
		body["timestamp"] = time.Now().UTC().Format(time.RFC3339)

		select {
		case inFlight <- struct{}{}:
			go func(s subscriber) {
				defer func() { <-inFlight }()
				deliver(s, body)
			}(s)
		default:
			logging.Error.Printf("Extension %s: too many events waiting, %s dropped", s.id, event)
		}
	}
}

// deliver posts the event, retrying after retryDelays; a last failure is only logged
func deliver(s subscriber, body map[string]interface{}) {
	for attempt := 0; ; attempt++ {
		err := send(s.webhook, s.secret, body)
		if err == nil {
			return
		}
		if attempt >= len(retryDelays) {
			logging.Error.Printf("Extension %s: event %v not delivered: %s", s.id, body["event"], err)
			return
		}
		time.Sleep(retryDelays[attempt])
	}
}

// SendTest posts a "test" event to the extension's webhook now and returns the result, so an
// administrator can check the address and the signature
func SendTest(ext *models.Extension) error {
	body := map[string]interface{}{
		"event":     "test",
		"extension": ext.ID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	return send(ext.WebhookURL, ext.Secret, body)
}

// ServerData is the "server" object events carry
func ServerData(id, name string) map[string]interface{} {
	return map[string]interface{}{"server": map[string]string{"id": id, "name": name}}
}
