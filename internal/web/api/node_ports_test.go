package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNextPortFree_SkipsHeldAndAskedPorts(t *testing.T) {
	held := map[uint16]portHolder{8081: {reason: portBusyPanel}, 8082: {reason: portBusyServer}}
	asked := map[uint16]bool{8080: true, 8083: true}
	assert.Equal(t, uint16(8084), nextPortFree(8080, held, asked), "neither another holder's port nor one the form already uses")
	assert.Equal(t, uint16(0), nextPortFree(65535, held, asked), "no port after the last one")
}

func TestIsMainPortVariable(t *testing.T) {
	for key, want := range map[string]bool{
		"port": true, "port2": true, "port10": true,
		"queryport": false, "rcon_port": false, "portal": false, "porta1": false,
	} {
		assert.Equal(t, want, isMainPortVariable(key), key)
	}
}

func TestPortCheckResults_HoldersOnlyForAdministrators(t *testing.T) {
	held := map[uint16]portHolder{25565: {reason: portBusyServer, by: "someone's survival"}}
	ports := map[string]uint16{"port": 25565, "queryport": 80, "rcon": 25575}

	customer := portCheckResults(ports, held, false)
	assert.True(t, customer["port"].Busy)
	assert.Empty(t, customer["port"].By, "a customer does not learn whose server holds it")
	assert.NotZero(t, customer["port"].Suggestion)
	assert.Equal(t, portBusyRange, customer["queryport"].Reason)
	assert.False(t, customer["rcon"].Busy)

	admin := portCheckResults(ports, held, true)
	assert.Equal(t, "someone's survival", admin["port"].By)
}
