package skypanel

import (
	"bytes"
	"encoding/json"
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

// "secure" in a template's console picks wss:// (the default, as before the field) or ws://
func TestCreateConsoleStdinProxy_RCONWSSecure(t *testing.T) {
	for raw, wantNoTLS := range map[string]bool{
		`{"type":"rconws","port":"28016"}`:                  false,
		`{"type":"rconws","port":"28016","secure":true}`:    false,
		`{"type":"rconws","port":"28016","secure":false}`:   true,
		`{"type":"rconws","port":"${port}","secure":false}`: true,
	} {
		var config StdinConsoleConfiguration
		if !assert.NoError(t, json.Unmarshal([]byte(raw), &config), raw) {
			continue
		}
		// the variables are filled in before the console is made; secure must survive that
		config = config.Replace(map[string]interface{}{"port": "28016"})

		e := &Environment{ServerID: "consoletest"}
		e.CreateConsoleStdinProxy(config, discardCloser{io.Discard})

		console, ok := e.Console.(*connections.RCONWSConnection)
		if assert.True(t, ok, raw) {
			assert.Equal(t, wantNoTLS, console.NoTLS, raw)
			assert.Equal(t, "28016", console.Port, raw)
		}
	}
}
