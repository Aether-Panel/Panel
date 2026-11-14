package command

import (
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFactoryReadsStdin(t *testing.T) {
	op, err := Factory.Create(skypanel.CreateOperation{OperationArgs: map[string]interface{}{
		"commands": []interface{}{"./start.sh"},
		"stdin":    map[string]interface{}{"type": "rcon", "ip": "127.0.0.1", "port": "25575", "password": "secret"},
	}})
	require.NoError(t, err, "a command with stdin could not be created at all")

	cmd := op.(Command)
	assert.Equal(t, []string{"./start.sh"}, cmd.Commands)
	assert.Equal(t, skypanel.StdinConsoleConfiguration{Type: "rcon", IP: "127.0.0.1", Port: "25575", Password: "secret"}, cmd.StdIn)
}

func TestFactoryWithoutStdin(t *testing.T) {
	op, err := Factory.Create(skypanel.CreateOperation{OperationArgs: map[string]interface{}{"commands": []interface{}{"echo hi"}}})
	require.NoError(t, err)
	assert.Equal(t, skypanel.StdinConsoleConfiguration{}, op.(Command).StdIn)
}
