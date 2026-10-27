package servers

import (
	"errors"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

// Reattach connects the daemon again to the server's process when it kept running across a
// daemon restart (a container does): the console streams again, commands and stop work, and
// its exit goes through afterExit like any run. It does nothing when the server is stopped or
// already attached, and when it cannot attach the server stays as it was: reachable again
// once it is restarted.
func (p *Server) Reattach() error {
	p.attachMu.Lock()
	defer p.attachMu.Unlock()

	env := p.RunningEnvironment
	// every start and every re-attach sets a console; only a freshly loaded server has none
	if env == nil || env.Console != nil {
		return nil
	}
	running, err := env.IsRunning()
	if err != nil || !running {
		return err
	}

	command, err := p.resolveCommand()
	if err != nil {
		return err
	}
	if command.StdIn.Type == "" {
		command.StdIn = p.Execution.Stdin
	}

	p.stopRequested.Store(false)
	p.exitHandled.Store(false)
	// the run began before the restart; counting from now only delays forgiving old crashes
	p.startedAt.Store(time.Now().UnixNano())
	if p.KeepAlive.Frequency != "" && p.KeepAlive.Command != "" {
		p.startKeepAlive()
	}

	err = env.Reattach(skypanel.ExecutionData{
		Variables:   p.DataToMap(),
		Callback:    p.afterExit,
		StdInConfig: command.StdIn,
	})
	if err != nil {
		p.stopKeepAlive()
		// a host process cannot be reached again; nothing to report for those
		if !errors.Is(err, skypanel.ErrNotImplemented) {
			p.Log(logging.Error, "Could not reconnect to the running server: %s", err)
		}
		return err
	}

	p.Log(logging.Info, "Reconnected to running server %s", p.ID())
	env.DisplayToConsole(true, "Reconnected to the running server after a daemon restart\n")
	return nil
}

// reattachRunningServers reconnects to every server that kept running while the daemon was
// down, so none of them waits for a command to get its console back
func reattachRunningServers() {
	for _, p := range GetAll() {
		go func(p *Server) {
			_ = p.Reattach()
		}(p)
	}
}
