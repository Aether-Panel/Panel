package servers

import (
	"errors"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
)

func TestNewNodeSnapshot_KeepsTheNodesOwnServers(t *testing.T) {
	previous := GetAll()
	allServersLock.Lock()
	allServers = nil
	allServersLock.Unlock()
	t.Cleanup(func() {
		allServersLock.Lock()
		allServers = previous
		allServersLock.Unlock()
	})
	local := CreateProgram()
	local.Identifier = "local1"
	addToCache(local)

	snapshot := newNodeSnapshot([]string{"remote1", "remote2", "remote3", "local1"}, map[string]skypanel.ServerRunning{
		"remote1": {Running: true},
		"remote2": {Installing: true},
		"local1":  {Running: true},
		"other1":  {Running: true},
	}, nil)

	assert.True(t, snapshot.reachable)
	assert.Equal(t, 4, snapshot.total)
	assert.Equal(t, map[string]bool{"remote1": true, "remote2": false}, snapshot.running,
		"servers of other nodes, servers the node left out and servers this process runs are not taken")
	online, offline := snapshot.onlineOffline()
	assert.Equal(t, 1, online)
	assert.Equal(t, 1, offline)
}

// A node that does not answer has no states at all, which RecordNodeStatuses takes as
// "record nothing", not as every server stopped
func TestNewNodeSnapshot_UnreachableNode(t *testing.T) {
	snapshot := newNodeSnapshot([]string{"remote1", "remote2"}, nil, errors.New("connection refused"))

	assert.False(t, snapshot.reachable)
	assert.Equal(t, 2, snapshot.total)
	assert.Nil(t, snapshot.running)
	online, offline := snapshot.onlineOffline()
	assert.Zero(t, online)
	assert.Zero(t, offline)
}

// Every status poll tells the node the panel's notification language
func TestStatusPollHeaders_CarryTheNotificationLanguage(t *testing.T) {
	t.Cleanup(func() { _ = config.NotificationLanguage.Set("es", false) })

	_ = config.NotificationLanguage.Set("en", false)
	assert.Equal(t, "en", statusPollHeaders().Get(services.NotificationLanguageHeader))

	_ = config.NotificationLanguage.Set("es", false)
	assert.Equal(t, "es", statusPollHeaders().Get(services.NotificationLanguageHeader))
}
