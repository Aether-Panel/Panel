package api

import (
	"errors"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// A refused port is a coded error with the port in it, so the page shows it in the viewer's
// language instead of the text the panel wrote
func TestValidatePorts_CodedErrors(t *testing.T) {
	db := setupCloneTest(t)
	require.NoError(t, db.Create(&models.Server{Identifier: "other", Name: "other", Type: "generic", Port: 25566, Ports: []uint16{25566}}).Error)

	assertPortError := func(err error, code, port string) {
		t.Helper()
		var panelErr *skypanel.Error
		if assert.True(t, errors.As(err, &panelErr), "%v", err) {
			assert.Equal(t, code, panelErr.Code)
			assert.Equal(t, port, panelErr.Meta["port"])
		}
	}

	assertPortError(validatePortsAgainstNode(db, 0, "", []uint16{80}), "ErrPortOutOfRange", "80")
	assertPortError(validatePortsAgainstNode(db, 0, "", []uint16{30000, 30000}), "ErrPortDuplicated", "30000")
	assertPortError(validatePortsAgainstNode(db, 0, "", []uint16{25566}), "ErrPortTaken", "25566")
	assert.NoError(t, validatePortsAgainstNode(db, 0, "other", []uint16{25566}), "a server keeps its own port")

	err := validateOtherPorts(db, 0, map[string]skypanel.Variable{"rcon_port": {Value: float64(25566)}})
	assertPortError(err, "ErrPortTaken", "25566 (rcon_port)")
	assert.Equal(t, "port 25566 (rcon_port) is already used by another server on this node", err.Error())
}
