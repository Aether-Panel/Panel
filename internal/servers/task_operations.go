package servers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/offsite"
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

// offsiteListTimeout bounds listing the bucket, and the deletions of a prune that follow it
const offsiteListTimeout = time.Minute

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
	opts AutoBackupOptions
}

func (o backupOperation) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	p, err := serverFromArgs(args)
	if err != nil {
		return skypanel.OperationResult{Error: err}
	}
	_, err = p.RunAutoBackup(o.opts)
	return skypanel.OperationResult{Error: err}
}

// taskIDArg is the step's "taskId" argument, which only the scheduler sets; "" when the step
// runs outside a scheduled task
func taskIDArg(v interface{}) (string, error) {
	if v == nil {
		return "", nil
	}
	id, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("invalid task id %v", v)
	}
	return id, nil
}

// copyTaskBackupOffsite uploads a task's backup in the background when the task's frequency
// says this run is due, then prunes the task's own older copies in the bucket. It never makes
// the task fail: the backup is already safe on the node, and the next run tries again.
func (p *Server) copyTaskBackupOffsite(fileName, file string, opts AutoBackupOptions) {
	if opts.Offsite == OffsiteNever || opts.TaskID == "" || !bucket.Configured() {
		return
	}
	// while the task's previous copy is still uploading it is not in the bucket yet, so this
	// run would upload again; it stays on the node instead
	if !startSeries(p.ID(), opts.TaskID) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), offsiteListTimeout)
	objects, err := bucket.List(ctx, p.ID())
	cancel()
	if err != nil {
		endSeries(p.ID(), opts.TaskID)
		p.Log(logging.Error, "Error listing off-site backups: %s", err)
		p.RunningEnvironment.DisplayToConsole(true, "Off-site copy skipped: %s", err)
		return
	}
	var last time.Time
	for _, o := range taskObjects(objects, opts.TaskID) {
		if o.ModTime.After(last) {
			last = o.ModTime
		}
	}
	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	if !offsiteDue(opts.Offsite, last, now().In(p.Scheduler.Location())) {
		endSeries(p.ID(), opts.TaskID)
		return
	}

	go func() {
		defer endSeries(p.ID(), opts.TaskID)
		if p.uploadOffsite(fileName, file) != nil {
			return
		}
		p.pruneOffsite(opts.OffsiteKeep, func(objects []offsite.Object) []offsite.Object {
			return taskObjects(objects, opts.TaskID)
		})
	}()
}

// uploadOffsite copies a finished backup to the bucket, telling the console how it went
func (p *Server) uploadOffsite(fileName, file string) error {
	p.RunningEnvironment.DisplayToConsole(true, "Uploading backup to off-site storage")
	ctx, cancel := context.WithTimeout(context.Background(), offsiteTimeout)
	defer cancel()
	if err := bucket.Upload(ctx, p.ID(), fileName, file); err != nil {
		p.Log(logging.Error, "Error uploading backup off-site: %s", err)
		p.RunningEnvironment.DisplayToConsole(true, "Off-site upload failed: %s", err)
		return err
	}
	p.RunningEnvironment.DisplayToConsole(true, "Backup uploaded to off-site storage")
	return nil
}

// pruneOffsite deletes the oldest copies of one series in the bucket beyond the newest keep.
// series picks the series' own copies out of the listing, so copies of other tasks, of the
// periodic schedule or made by hand are never counted nor deleted. A server deleted while its
// copy uploaded prunes nothing: its copies in the bucket are all that is left of it, and a
// server created again with its id would find them.
func (p *Server) pruneOffsite(keep int, series func([]offsite.Object) []offsite.Object) {
	if keep < 1 || !p.loaded() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), offsiteListTimeout)
	defer cancel()
	objects, err := bucket.List(ctx, p.ID())
	if err != nil {
		p.Log(logging.Error, "Error listing off-site backups: %s", err)
		return
	}
	for _, name := range newestBeyond(series(objects), keep) {
		if err := bucket.Delete(ctx, p.ID(), name); err != nil {
			p.Log(logging.Error, "Error deleting off-site backup %s: %s", name, err)
		}
	}
}

// taskObjects keeps the task's own copies. The name is parsed rather than its start compared,
// since task "a" would otherwise claim the copies of task "a-b".
func taskObjects(objects []offsite.Object, taskID string) []offsite.Object {
	own := make([]offsite.Object, 0, len(objects))
	for _, o := range objects {
		if id, ok := taskIDFromBackupName(o.FileName); ok && id == taskID {
			own = append(own, o)
		}
	}
	return own
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
		frequency, err := ParseOffsiteFrequency(op.OperationArgs["offsite"])
		if err != nil {
			return nil, err
		}
		offsiteKeep, err := OffsiteKeep(op.OperationArgs["offsiteKeep"])
		if err != nil {
			return nil, err
		}
		taskID, err := taskIDArg(op.OperationArgs["taskId"])
		if err != nil {
			return nil, err
		}
		return backupOperation{opts: AutoBackupOptions{Keep: keep, TaskID: taskID, Offsite: frequency, OffsiteKeep: offsiteKeep}}, nil
	}},
}
