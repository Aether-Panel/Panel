package skypanel

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

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

// waitEnvironment is a process that already looks stopped while its exit is still handled
type waitEnvironment struct {
	killed chan struct{}
}

func (w *waitEnvironment) ExecuteAsyncImpl(*Environment, ExecutionData) error { return nil }
func (w *waitEnvironment) KillImpl(e *Environment) error {
	close(w.killed)
	e.Wait.Done()
	return nil
}
func (w *waitEnvironment) GetStatsImpl(*Environment) (*ServerStats, error) {
	return &ServerStats{}, nil
}
func (w *waitEnvironment) SendCodeImpl(*Environment, int) error     { return nil }
func (w *waitEnvironment) GetUIDImpl(*Environment) int              { return 0 }
func (w *waitEnvironment) GetGidImpl(*Environment) int              { return 0 }
func (w *waitEnvironment) IsRunningImpl(*Environment) (bool, error) { return false, nil }

func TestWaitForMainProcess_WaitsForTheExitToBeHandled(t *testing.T) {
	e := &Environment{Implementation: &waitEnvironment{killed: make(chan struct{})}, Wait: &sync.WaitGroup{}}
	e.Wait.Add(1)
	handled := make(chan struct{})
	go func() {
		time.Sleep(150 * time.Millisecond)
		close(handled)
		e.Wait.Done()
	}()

	start := time.Now()
	assert.NoError(t, e.WaitForMainProcess())
	select {
	case <-handled:
	default:
		t.Fatal("returned before the exit was handled, although the process no longer ran")
	}
	assert.GreaterOrEqual(t, time.Since(start), 140*time.Millisecond)
}

func TestWaitForMainProcessFor_KillsAfterTheTimeout(t *testing.T) {
	impl := &waitEnvironment{killed: make(chan struct{})}
	e := &Environment{Implementation: impl, Wait: &sync.WaitGroup{}}
	e.Wait.Add(1)

	assert.NoError(t, e.WaitForMainProcessFor(50*time.Millisecond))
	select {
	case <-impl.killed:
	default:
		t.Fatal("the process was not killed")
	}
}

func TestWaitForMainProcess_NothingToWaitFor(t *testing.T) {
	e := &Environment{Implementation: &waitEnvironment{killed: make(chan struct{})}, Wait: &sync.WaitGroup{}}
	done := make(chan struct{})
	go func() { _ = e.WaitForMainProcess(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("blocked with no process")
	}
}
