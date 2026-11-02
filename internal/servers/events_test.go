package servers

import (
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestPublish_ServerEventsReachTheSinkUnderTheirExtensionNames(t *testing.T) {
	p, _ := newBackupTestServer(t)
	p.Server.Display = "Survival"

	var events []string
	var last map[string]interface{}
	previous := EventSink
	EventSink = func(event string, data map[string]interface{}) {
		events = append(events, event)
		last = data
	}
	t.Cleanup(func() { EventSink = previous })

	p.publish(notificationEvent{Kind: "power", Extra: map[string]interface{}{"running": true}})
	p.publish(notificationEvent{Kind: "power", Extra: map[string]interface{}{"running": false}})
	p.publish(notificationEvent{Kind: "crash", Extra: map[string]interface{}{"exitCode": 1}})
	p.publish(notificationEvent{Kind: "test"})

	assert.Equal(t, []string{"server.started", "server.stopped", "server.crashed"}, events)
	assert.Equal(t, map[string]string{"id": "backuptest", "name": "Survival"}, last["server"])
	assert.Equal(t, 1, last["exitCode"])
	assert.NotContains(t, last, "event", "the sink names the event itself")
}

// The title and message an extension (and the owner's channels) receive follow the panel's
// notification language; the resource in the data stays the value it always was
func TestServerEvents_FollowTheNotificationLanguage(t *testing.T) {
	p, _ := newBackupTestServer(t)
	p.Server.Display = "Survival"

	var last map[string]interface{}
	previous := EventSink
	EventSink = func(_ string, data map[string]interface{}) { last = data }
	t.Cleanup(func() {
		EventSink = previous
		_ = config.NotificationLanguage.Set("es", false)
	})

	_ = config.NotificationLanguage.Set("es", false)
	p.notifyResource("Memoria", 95, 90)
	assert.Equal(t, "⚠️ Uso alto de memoria", last["title"])
	assert.Equal(t, "Survival está usando el 95 % de memoria (aviso a partir del 90 %).", last["message"])

	_ = config.NotificationLanguage.Set("en", false)
	p.notifyResource("Memoria", 95, 90)
	assert.Equal(t, "⚠️ High memory usage", last["title"])
	assert.Equal(t, "Survival is using 95% of its memory (the alert starts at 90%).", last["message"])
	assert.Equal(t, "Memoria", last["resource"])

	p.notifyPower(false)
	assert.Equal(t, "⚪ Server stopped", last["title"])
	assert.Equal(t, "Survival has stopped.", last["message"])
}

func TestResourceLabel(t *testing.T) {
	t.Cleanup(func() { _ = config.NotificationLanguage.Set("es", false) })
	_ = config.NotificationLanguage.Set("es", false)
	assert.Equal(t, "Memoria", resourceLabel("Memoria", false))
	assert.Equal(t, "cpu", resourceLabel("CPU", true))
	_ = config.NotificationLanguage.Set("en", false)
	assert.Equal(t, "CPU", resourceLabel("CPU", true))
	assert.Equal(t, "memory", resourceLabel("Memoria", false))
}
