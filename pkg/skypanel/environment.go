package skypanel

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/connections"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
)

type EnvironmentImpl interface {
	ExecuteAsyncImpl(environment *Environment, steps ExecutionData) error

	KillImpl(environment *Environment) error

	GetStatsImpl(environment *Environment) (*ServerStats, error)

	SendCodeImpl(environment *Environment, code int) error

	GetUIDImpl(environment *Environment) int

	GetGidImpl(environment *Environment) int

	IsRunningImpl(environment *Environment) (isRunning bool, err error)
}

type Environment struct {
	Type            string          `json:"type"`
	RootDirectory   string          `json:"root,omitempty"`
	BackupDirectory string          `json:"-"`
	ConsoleBuffer   *MemoryCache    `json:"-"`
	Wait            *sync.WaitGroup `json:"-"`
	ServerID        string          `json:"-"`
	LastExitCode    int             `json:"-"`
	Wrapper         io.Writer       `json:"-"` // our proxy back to the main
	ConsoleTracker  *Tracker        `json:"-"`
	StatusTracker   *Tracker        `json:"-"`
	StatsTracker    *Tracker        `json:"-"`
	Installing      bool            `json:"-"`
	BackingUp       bool            `json:"-"`
	Console         Console         `json:"-"`
	Server          Server          `json:"-"`
	Implementation  EnvironmentImpl `json:"-"`
}

type ExecutionData struct {
	Command          string
	Environment      map[string]string
	WorkingDirectory string
	Variables        map[string]interface{}
	Callback         func(exitCode int)
	StdInConfig      StdinConsoleConfiguration
	// DisableStdin     bool
	DisableQuery bool
	DisableStats bool
}

type ExecutionFunction func(steps ExecutionData) (err error)

func (e *Environment) Execute(steps ExecutionData) error {
	err := e.ExecuteAsync(steps)
	if err != nil {
		return err
	}
	return e.WaitForMainProcess()
}

func (e *Environment) ExecuteAsync(steps ExecutionData) (err error) {
	running, err := e.IsRunning()
	if err != nil {
		return
	}
	if running {
		err = ErrProcessRunning
		return
	}

	// update configs
	steps.StdInConfig = steps.StdInConfig.Replace(steps.Variables)

	return e.Implementation.ExecuteAsyncImpl(e, steps)
}

func (e *Environment) CreateConsoleStdinProxy(config StdinConsoleConfiguration, base io.WriteCloser) {
	switch config.Type {
	case "telnet":
		e.Console = &connections.TelnetConnection{
			IP:       config.IP,
			Port:     config.Port,
			Password: config.Password,
		}
	case "rcon":
		e.Console = &connections.RCONConnection{
			IP:       config.IP,
			Port:     config.Port,
			Password: config.Password,
		}
	case "rconws":
		e.Console = &connections.RCONWSConnection{
			IP:       config.IP,
			Port:     config.Port,
			Password: config.Password,
		}
	default:
		e.Console = &NoStartConsole{Base: base}
	}
}

func (e *Environment) GetRootDirectory() string {
	return e.RootDirectory
}

func (e *Environment) GetConsole() (console []byte, epoch int64) {
	console, epoch = e.ConsoleBuffer.Read()
	return
}

func (e *Environment) GetConsoleFrom(time int64) (console []byte, epoch int64) {
	console, epoch = e.ConsoleBuffer.ReadFrom(time)
	return
}

func (e *Environment) AddConsoleListener(ws *Socket) {
	e.ConsoleTracker.Register(ws)
}

func (e *Environment) AddStatsListener(ws *Socket) {
	e.StatsTracker.Register(ws)
}

func (e *Environment) AddStatusListener(ws *Socket) {
	e.StatusTracker.Register(ws)
}

func (e *Environment) GetStatsTracker() *Tracker {
	return e.StatsTracker
}

func (e *Environment) DisplayToConsole(daemon bool, msg string, data ...interface{}) {
	format := msg
	if daemon {
		if !strings.HasSuffix(format, "\n") {
			format += "\n"
		}
		format = "[DAEMON] " + format
	}
	if len(data) == 0 {
		_, _ = fmt.Fprint(e.ConsoleBuffer, format)
		_, _ = fmt.Fprint(e.ConsoleTracker, format)
	} else {
		_, _ = fmt.Fprintf(e.ConsoleBuffer, format, data...)
		_, _ = fmt.Fprintf(e.ConsoleTracker, format, data...)
	}
}

func (e *Environment) Update() error {
	return nil
}

func (e *Environment) validatedRoot() (string, error) {
	p, err := filepath.Abs(e.RootDirectory)
	if err != nil {
		return "", fmt.Errorf("invalid root directory: %s", e.RootDirectory)
	}
	p = filepath.Clean(p)
	base, err := filepath.Abs(config.ServersFolder.Value())
	if err != nil {
		return "", fmt.Errorf("invalid server data folder: %s", config.ServersFolder.Value())
	}
	rel, err := filepath.Rel(base, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("root directory %s is not within %s", p, base)
	}
	return p, nil
}

func (e *Environment) validatedPath() (string, error) {
	p, err := e.validatedRoot()
	if err != nil {
		return "", err
	}
	cleanP := filepath.Clean(p)
	base, absErr := filepath.Abs(config.ServersFolder.Value())
	if absErr != nil {
		return "", absErr
	}
	cleanBase := filepath.Clean(base)
	if !strings.HasPrefix(cleanP, cleanBase+string(filepath.Separator)) && cleanP != cleanBase {
		return "", fmt.Errorf("invalid root directory: %s", cleanP)
	}
	return cleanP, nil
}

func (e *Environment) Delete() (err error) {
	dir, err := e.validatedPath()
	if err != nil {
		return err
	}
	err = os.RemoveAll(dir)
	return
}

func (e *Environment) Create() error {
	dir, err := e.validatedPath()
	if err != nil {
		return err
	}
	err = os.Mkdir(dir, 0755)
	if os.IsExist(err) {
		return nil
	}
	return err
}

func (e *Environment) WaitForMainProcess() error {
	return e.WaitForMainProcessFor(0)
}

func (e *Environment) WaitForMainProcessFor(timeout time.Duration) (err error) {
	running, err := e.IsRunning()
	if err != nil {
		return
	}
	if running {
		if timeout > 0 {
			var timer = time.AfterFunc(timeout, func() {
				err = e.Kill()
			})
			e.Wait.Wait()
			timer.Stop()
		} else {
			e.Wait.Wait()
		}
	}
	return
}

func (e *Environment) CreateWrapper() {
	if config.ConsoleForward.Value() {
		// return io.MultiWriter(newLogger(e.ServerID).Writer(), e.ConsoleBuffer, e.ConsoleTracker)
		e.Wrapper = io.MultiWriter(logging.OriginalStdOut, e.ConsoleBuffer, e.ConsoleTracker)
	} else {
		e.Wrapper = io.MultiWriter(e.ConsoleBuffer, e.ConsoleTracker)
	}
}

func (e *Environment) GetLastExitCode() int {
	return e.LastExitCode
}

func (e *Environment) GetWrapper() io.Writer {
	return e.Wrapper
}

func (e *Environment) Log(l *log.Logger, format string, obj ...interface{}) {
	msg := fmt.Sprintf("[%s] ", e.ServerID) + format
	l.Printf(msg, obj...)
}

func (e *Environment) IsInstalling() bool {
	return e.Installing
}

func (e *Environment) SetInstalling(flag bool) {
	e.Installing = flag
	_ = e.StatusTracker.WriteMessage(Transmission{
		Message: ServerRunning{
			Installing: flag,
		},
		Type: MessageTypeStatus,
	})
}

func (e *Environment) ExecuteInMainProcess(cmd string) (err error) {
	running, err := e.IsRunning()
	if err != nil {
		return err
	}
	if !running {
		err = ErrServerOffline
		return
	}
	_, err = io.WriteString(e.Console, cmd+"\n")
	return
}

func (e *Environment) IsRunning() (isRunning bool, err error) {
	return e.Implementation.IsRunningImpl(e)
}

func (e *Environment) Kill() error {
	return e.Implementation.KillImpl(e)
}

func (e *Environment) GetStats() (*ServerStats, error) {
	return e.Implementation.GetStatsImpl(e)
}

func (e *Environment) SendCode(code int) error {
	return e.Implementation.SendCodeImpl(e, code)
}

func (e *Environment) GetUID() int {
	return e.Implementation.GetUIDImpl(e)
}

func (e *Environment) GetGid() int {
	return e.Implementation.GetGidImpl(e)
}
