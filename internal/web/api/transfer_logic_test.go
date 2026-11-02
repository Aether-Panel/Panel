package api

import (
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestServerPortsTaken_OnlyPortsHeldOnTheTargetNode(t *testing.T) {
	held := map[uint16]portHolder{25565: {reason: portBusyServer, by: "other"}, 8080: {reason: portBusyPanel}}

	server := &models.Server{Port: 25565, Ports: []uint16{25565, 25575, 8080}}
	assert.Equal(t, []uint16{8080, 25565}, serverPortsTaken(server, held), "each taken port once, in order")

	assert.Empty(t, serverPortsTaken(&models.Server{Port: 25570, Ports: []uint16{25570}}, held))
	assert.Empty(t, serverPortsTaken(&models.Server{}, held), "a server without ports")
	assert.Empty(t, serverPortsTaken(&models.Server{Port: 80, Ports: []uint16{80}}, held), "a low port it already had is not refused")
}
