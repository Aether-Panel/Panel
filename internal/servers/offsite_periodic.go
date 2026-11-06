package servers

import (
	"sync"
	"time"
)

// periodicState is what the node remembers of one server's periodic off-site copies. It is
// kept in memory only: after a restart it is read again from the bucket.
type periodicState struct {
	// last is when the server's newest periodic copy was made
	last time.Time
}

// periodic holds the state of every server, by server id
var periodic = struct {
	sync.Mutex
	states map[string]*periodicState
}{states: map[string]*periodicState{}}

// LastPeriodic is when the server's last periodic off-site copy was made; false when the
// node does not know of one
func LastPeriodic(serverID string) (time.Time, bool) {
	periodic.Lock()
	defer periodic.Unlock()
	s := periodic.states[serverID]
	if s == nil || s.last.IsZero() {
		return time.Time{}, false
	}
	return s.last, true
}
