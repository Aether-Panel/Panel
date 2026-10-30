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
