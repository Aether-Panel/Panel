package servers

import (
	"errors"
	"fmt"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cast"
)

// Operations that act on the server itself, so scheduled tasks can start, stop, restart and
// back it up. They live here rather than in internal/operations because they need *Server.

// stopTimeout is how long a task waits for the game to shut down before killing it, so a
// hung server cannot block the rest of the task forever
const stopTimeout = 5 * time.Minute

// DefaultAutoBackupKeep is how many automatic backups are kept when a task does not say
const DefaultAutoBackupKeep = 5

// MaxAutoBackupKeep caps retention so one task cannot fill the node's disk
const MaxAutoBackupKeep = 50

var errNotAServer = errors.New("this operation can only run on a server")

func serverFromArgs(args skypanel.RunOperatorArgs) (*Server, error) {
	p, ok := args.Server.(*Server)
	if !ok || p == nil {
		return nil, errNotAServer
	}
	return p, nil
}

// Restart stops the server, waits for it to exit and starts it again
func (p *Server) Restart() error {
	if err := p.Stop(); err != nil {
		return err
	}
	if err := p.GetEnvironment().WaitForMainProcess(); err != nil {
		return err
	}
	return p.Start()
}

type startOperation struct{}

func (startOperation) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	p, err := serverFromArgs(args)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}
	if running, err := p.IsRunning(); err != nil || running {
		return skypanel.OperationResult{Error: err}
	}
	return skypanel.OperationResult{Error: p.Start()}
}

type stopOperation struct{}

func (stopOperation) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	p, err := serverFromArgs(args)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}
	if err := p.Stop(); err != nil {
		return skypanel.OperationResult{Error: err}
	}
	return skypanel.OperationResult{Error: p.GetEnvironment().WaitForMainProcessFor(stopTimeout)}
}

// restartOperation only restarts a running server: a scheduled restart must not bring back
// a server its owner switched off
type restartOperation struct{}

func (restartOperation) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	p, err := serverFromArgs(args)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}
	running, err := p.IsRunning()
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}
	if !running {
		p.RunningEnvironment.DisplayToConsole(true, "Server is not running, skipping the restart\n")
		return skypanel.OperationResult{}
	}
	if err := p.Stop(); err != nil {
		return skypanel.OperationResult{Error: err}
	}
	if err := p.GetEnvironment().WaitForMainProcessFor(stopTimeout); err != nil {
		return skypanel.OperationResult{Error: err}
	}
	return skypanel.OperationResult{Error: p.Start()}
}

type backupOperation struct {
	keep int
}

func (o backupOperation) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	p, err := serverFromArgs(args)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}
	_, err = p.RunAutoBackup(o.keep)
	return skypanel.OperationResult{Error: err}
}

type serverOperationFactory struct {
	key    string
	create func(op skypanel.CreateOperation) (skypanel.Operation, error)
}

func (f serverOperationFactory) Create(op skypanel.CreateOperation) (skypanel.Operation, error) {
	return f.create(op)
}

func (f serverOperationFactory) Key() string {
	return f.key
}

// AutoBackupKeep reads the "keep" argument of a backup operation, defaulting and capping it
func AutoBackupKeep(value interface{}) (int, error) {
	if value == nil {
		return DefaultAutoBackupKeep, nil
	}
	keep, err := cast.ToIntE(value)
	if err != nil {
		return 0, err
	}
	if keep < 1 || keep > MaxAutoBackupKeep {
		return 0, fmt.Errorf("keep must be between 1 and %d", MaxAutoBackupKeep)
	}
	return keep, nil
}

var serverOperationFactories = []skypanel.OperationFactory{
	serverOperationFactory{key: "start", create: func(skypanel.CreateOperation) (skypanel.Operation, error) {
		return startOperation{}, nil
	}},
	serverOperationFactory{key: "stop", create: func(skypanel.CreateOperation) (skypanel.Operation, error) {
		return stopOperation{}, nil
	}},
	serverOperationFactory{key: "restart", create: func(skypanel.CreateOperation) (skypanel.Operation, error) {
		return restartOperation{}, nil
	}},
	serverOperationFactory{key: "backup", create: func(op skypanel.CreateOperation) (skypanel.Operation, error) {
		keep, err := AutoBackupKeep(op.OperationArgs["keep"])
		if err != nil {
			return nil, err
		}
		return backupOperation{keep: keep}, nil
	}},
}
