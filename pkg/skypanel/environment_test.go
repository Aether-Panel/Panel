package skypanel

import (
	"bytes"
	"io"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/connections"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/stretchr/testify/assert"
)

type discardCloser struct{ io.Writer }

func (discardCloser) Close() error { return nil }

func TestCreateConsoleStdinProxy_PicksTheConsoleOfItsType(t *testing.T) {
	for consoleType, want := range map[string]Console{
		"":       &NoStartConsole{},
		"stdin":  &NoStartConsole{},
		"rcon":   &connections.RCONConnection{},
		"telnet": &connections.TelnetConnection{},
		"rconws": &connections.RCONWSConnection{},
	} {
		e := &Environment{ServerID: "consoletest"}
		e.CreateConsoleStdinProxy(StdinConsoleConfiguration{Type: consoleType, Port: "25575"}, discardCloser{io.Discard})
		assert.IsType(t, want, e.Console, consoleType)
	}
}

func TestCreateConsoleStdinProxy_WarnsAboutAnUnknownType(t *testing.T) {
	var logged bytes.Buffer
	previous := logging.Error.Writer()
	logging.Error.SetOutput(&logged)
	defer logging.Error.SetOutput(previous)

	e := &Environment{ServerID: "consoletest"}
	e.CreateConsoleStdinProxy(StdinConsoleConfiguration{Type: "rcon-ws"}, discardCloser{io.Discard})

	assert.IsType(t, &NoStartConsole{}, e.Console, "commands still reach the game")
	assert.Contains(t, logged.String(), `"rcon-ws"`)

	logged.Reset()
	e.CreateConsoleStdinProxy(StdinConsoleConfiguration{Type: "stdin"}, discardCloser{io.Discard})
	assert.Empty(t, logged.String(), "stdin is not a mistake")
}
