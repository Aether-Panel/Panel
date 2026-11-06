package servers

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/offsite"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

const (
	// periodicSeries is the upload series of the periodic copies, apart from every task's
	periodicSeries = "periodic"
	// periodicFirstPass lets the node finish starting its servers before the first copies
	periodicFirstPass = 5 * time.Minute
	// periodicInterval is how often the worker looks for servers whose copy is due
	periodicInterval = 10 * time.Minute
	// periodicMaxBackoff caps the wait after failures: a bucket down for days still gets
	// tried once a day
	periodicMaxBackoff = 24 * time.Hour
)

// periodicState is what the node remembers of one server's periodic off-site copies. It is
// kept in memory only: after a restart it is read again from the bucket.
type periodicState struct {
	// last is when the server's newest periodic copy was made
	last time.Time
	// seeded is whether last has been read from the bucket, which happens once
	seeded bool
	// failures in a row; each one doubles the wait before the next try
	failures int
	// retryAt is when the next try may run after a failure
	retryAt time.Time
	// settingsError is the last error reading the server's own setting, so it is logged once
	// rather than on every pass
	settingsError string
}

// periodic holds the state of every server, by server id
var periodic = struct {
	sync.Mutex
	states map[string]*periodicState
}{states: map[string]*periodicState{}}

// LastPeriodic is when the server's last periodic off-site copy was made; false when the
// node does not know of one
func LastPeriodic(serverID string) (time.Time, bool) {
	s := periodicOf(serverID)
	if s.last.IsZero() {
		return time.Time{}, false
	}
	return s.last, true
}

// periodicOf is a copy of the server's state; the zero state when it has none
func periodicOf(serverID string) periodicState {
	periodic.Lock()
	defer periodic.Unlock()
	if s := periodic.states[serverID]; s != nil {
		return *s
	}
	return periodicState{}
}

// updatePeriodic changes the server's state, creating it when needed
func updatePeriodic(serverID string, change func(*periodicState)) {
	periodic.Lock()
	defer periodic.Unlock()
	s := periodic.states[serverID]
	if s == nil {
		s = &periodicState{}
		periodic.states[serverID] = s
	}
	change(s)
}

// forgetPeriodic drops the state of a deleted server, so one created again with its id
// starts afresh
func forgetPeriodic(serverID string) {
	periodic.Lock()
	defer periodic.Unlock()
	delete(periodic.states, serverID)
}

// StartOffsitePeriodic starts the node's only periodic copy worker in the background. It
// goes through the servers one at a time, so they never all back up at once.
func StartOffsitePeriodic() {
	go func() {
		time.Sleep(periodicFirstPass)
		periodicPass(time.Now())
		ticker := time.NewTicker(periodicInterval)
		defer ticker.Stop()
		for range ticker.C {
			periodicPass(time.Now())
		}
	}()
}

// periodicPass makes the periodic copy of every server that is due, in order
func periodicPass(now time.Time) {
	for _, p := range GetAll() {
		p.runPeriodic(now)
	}
}

// runPeriodic makes the server's periodic copy if it is due, uploads it and prunes the
// bucket, leaving nothing on the node. It reports whether a copy was made, whatever happened
// to its upload. Only failures are announced; after one the server waits 1, 2, 4... up to 24
// hours before trying again.
func (p *Server) runPeriodic(now time.Time) bool {
	if !bucket.Configured() {
		return false
	}
	hours, keep, ok := p.periodicPlan()
	if !ok || hours == 0 {
		return false
	}
	state := periodicOf(p.ID())
	if now.Before(state.retryAt) || p.busyForPeriodic() {
		return false
	}
	if !state.seeded {
		last, err := p.lastPeriodicInBucket()
		if err != nil {
			p.periodicUnavailable(now, err)
			return false
		}
		updatePeriodic(p.ID(), func(s *periodicState) {
			s.last = last
			s.seeded = true
		})
		state.last = last
	}
	if now.Sub(state.last) < time.Duration(hours)*time.Hour {
		return false
	}
	// a quick check first, so a bucket that is down does not cost a whole archive
	ctx, cancel := context.WithTimeout(context.Background(), offsiteListTimeout)
	err := bucket.Reachable(ctx)
	cancel()
	if err != nil {
		p.periodicUnavailable(now, err)
		return false
	}
	return p.makePeriodicCopy(now, keep)
}

// makePeriodicCopy writes the archive, uploads it while the periodic series is claimed and
// removes it from the node, whether the upload worked or not, so retries never fill the disk
func (p *Server) makePeriodicCopy(now time.Time, keep int) bool {
	if !startSeries(p.ID(), periodicSeries) {
		return false
	}
	defer endSeries(p.ID(), periodicSeries)

	fileName, file, err := p.beginBackup(OffsiteBackupPrefix)
	if errors.Is(err, skypanel.ErrBackupInProgress) {
		// someone started a backup since the check; the next pass tries again
		return false
	}
	if err != nil {
		// beginBackup has already announced the failure
		p.Log(logging.Error, "Error starting the periodic backup: %s", err)
		recordPeriodicFailure(p.ID(), now)
		return false
	}
	defer func() {
		if err := p.deleteLocalBackup(fileName); err != nil {
			p.Log(logging.Error, "Error removing the local periodic backup %s: %s", fileName, err)
		}
	}()

	if err := p.writeArchive(file); err != nil {
		p.Log(logging.Error, "Error creating backup file: %s", err)
		p.RunningEnvironment.DisplayToConsole(true, "Failed to create backup file")
		p.endBackupQuiet(false)
		recordPeriodicFailure(p.ID(), now)
		return false
	}
	p.endBackupQuiet(true)

	if p.uploadOffsite(fileName, file) != nil {
		p.alertBackup(false)
		recordPeriodicFailure(p.ID(), now)
		return true
	}
	updatePeriodic(p.ID(), func(s *periodicState) {
		s.last = now
		s.failures = 0
		s.retryAt = time.Time{}
	})
	p.pruneOffsite(keep, periodicObjects)
	return true
}

// periodicPlan is the server's interval and copies to keep. It reports false when the
// server's own setting cannot be read: it may have turned the copy off, so the panel's value
// is no safe guess. The same error is logged only once.
func (p *Server) periodicPlan() (hours, keep int, ok bool) {
	hours, keep, err := p.periodicSettings()
	message := ""
	if err != nil {
		message = err.Error()
	}
	periodic.Lock()
	s := periodic.states[p.ID()]
	changed := (s == nil && message != "") || (s != nil && s.settingsError != message)
	if changed {
		if s == nil {
			s = &periodicState{}
			periodic.states[p.ID()] = s
		}
		s.settingsError = message
	}
	periodic.Unlock()
	if err != nil {
		if changed {
			p.Log(logging.Error, "Error reading the off-site settings, no periodic copy until it is fixed: %s", err)
		}
		return 0, 0, false
	}
	return hours, keep, true
}

// busyForPeriodic says whether the server must be left alone this pass: being moved to
// another node, installed, backed up or restored. A running game is no reason; the copy is
// taken hot, as one made by hand.
func (p *Server) busyForPeriodic() bool {
	if p.Scheduler != nil && p.Scheduler.IsPaused() {
		return true
	}
	if p.GetEnvironment().IsInstalling() {
		return true
	}
	return p.IsBackingUp() || p.IsRestoring()
}

// lastPeriodicInBucket is when the server's newest periodic copy in the bucket was made; zero
// when there is none
func (p *Server) lastPeriodicInBucket() (time.Time, error) {
	ctx, cancel := context.WithTimeout(context.Background(), offsiteListTimeout)
	defer cancel()
	objects, err := bucket.List(ctx, p.ID())
	if err != nil {
		return time.Time{}, err
	}
	var last time.Time
	for _, o := range periodicObjects(objects) {
		if o.ModTime.After(last) {
			last = o.ModTime
		}
	}
	return last, nil
}

// periodicUnavailable records and announces a bucket that could not be reached or listed
func (p *Server) periodicUnavailable(now time.Time, err error) {
	p.Log(logging.Error, "Periodic off-site backup skipped, the bucket is not available: %s", err)
	p.RunningEnvironment.DisplayToConsole(true, "Periodic off-site backup skipped, the bucket is not available: %s", err)
	p.alertBackup(false)
	recordPeriodicFailure(p.ID(), now)
}

// recordPeriodicFailure doubles the wait before the server's next try, up to a day
func recordPeriodicFailure(serverID string, now time.Time) {
	updatePeriodic(serverID, func(s *periodicState) {
		s.failures++
		// 2^5 hours is already past the cap, and a larger shift would overflow
		backoff := min(time.Hour<<min(s.failures-1, 5), periodicMaxBackoff)
		s.retryAt = now.Add(backoff)
	})
}

// periodicObjects keeps the periodic copies; those of the tasks and made by hand are not its
func periodicObjects(objects []offsite.Object) []offsite.Object {
	own := make([]offsite.Object, 0, len(objects))
	for _, o := range objects {
		if strings.HasPrefix(o.FileName, OffsiteBackupPrefix) {
			own = append(own, o)
		}
	}
	return own
}
