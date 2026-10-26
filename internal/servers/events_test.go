package servers

import (
	"testing"

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
