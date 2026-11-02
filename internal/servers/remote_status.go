package servers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"gorm.io/gorm"
)

// Remote daemons run without the panel, so nothing records their servers' uptime on their side.
// The panel asks every remote node for the state of all its servers in one request and records
// the answers like the local tracker does.
const (
	// remoteStatusEvery is how often each remote node is asked. State changes are recorded
	// within this much of when they happened, at one light request per node.
	remoteStatusEvery = 30 * time.Second
	// remoteStatusTimeout bounds one node's answer, so a node that is down does not hold the
	// round up (rounds never overlap)
	remoteStatusTimeout = 10 * time.Second
	// remoteStatusStaleAfter is how long a server's open period may go unconfirmed before it is
	// closed at its last confirmation: a few missed polls (a node restarting, a network blip)
	// keep the period going, a real outage shows as a gap instead of uptime or downtime
	remoteStatusStaleAfter = 3 * time.Minute
	// remoteStatusMaxBody caps the node's answer; a few dozen bytes per server
	remoteStatusMaxBody = 4 << 20
)

// nodeServersStatus is a daemon's /daemon/servers/status answer
type nodeServersStatus struct {
	Servers map[string]skypanel.ServerRunning `json:"servers"`
}

// nodeStatusSnapshot is the last poll of a remote node, kept for the Discord node report
type nodeStatusSnapshot struct {
	at        time.Time
	reachable bool
	// total is how many servers the panel places on the node
	total int
	// running is the state of each of those servers the node answered for; nil when unreachable
	running map[string]bool
}

var (
	// nodeSnapshots is the latest poll of each remote node, by node ID
	nodeSnapshots = make(map[uint]nodeStatusSnapshot)
	// nodeSnapshotsLock guards nodeSnapshots: the poller writes it, the Discord report reads it
	nodeSnapshotsLock sync.RWMutex
	// remotePolling is set while a round runs, so a slow node or database never makes two
	// rounds record the same server at once
	remotePolling atomic.Bool
)

// processRemoteStatus polls the remote nodes now and then every remoteStatusEvery
func processRemoteStatus() {
	pollRemoteNodes()
	ticker := time.NewTicker(remoteStatusEvery)
	for range ticker.C {
		if !running {
			ticker.Stop()
			return
		}
		pollRemoteNodes()
	}
}

// pollRemoteNodes asks every remote node, all at once, for its servers' states and records them.
// Only where the panel runs: a daemon-only node has no panel database and no nodes to ask.
func pollRemoteNodes() {
	if !config.PanelEnabled.Value() || !remotePolling.CompareAndSwap(false, true) {
		return
	}
	defer remotePolling.Store(false)

	db, err := database.GetConnection()
	if err != nil {
		logging.Error.Printf("Error getting database connection for the node status poll: %s", err)
		return
	}
	ns := &services.Node{DB: db}
	us := &services.Uptime{DB: db}

	nodes, err := ns.GetAll()
	if err != nil {
		logging.Error.Printf("Error getting nodes for the node status poll: %s", err)
		return
	}

	var wg sync.WaitGroup
	for _, node := range nodes {
		// the panel's own servers are recorded every few seconds by trackUptimeForAllServers
		if node.IsLocal() {
			continue
		}
		wg.Add(1)
		go func(node *models.Node) {
			defer wg.Done()
			defer skypanel.Recover()
			pollRemoteNode(ns, us, node)
		}(node)
	}
	wg.Wait()
}

// pollRemoteNode asks one node for its servers' states, keeps the answer for the Discord report
// and records it as uptime
func pollRemoteNode(ns *services.Node, us *services.Uptime, node *models.Node) {
	serverIDs, err := remoteNodeServerIDs(ns.DB, node.ID)
	if err != nil {
		logging.Error.Printf("Error listing the servers of node %s: %s", node.Name, err)
		return
	}

	reported, err := fetchNodeServerStatuses(ns, node)
	snapshot := newNodeSnapshot(serverIDs, reported, err)
	// told once when the node stops or starts answering, not every poll
	previous, seen := storeNodeSnapshot(node.ID, snapshot)
	if !snapshot.reachable && (!seen || previous.reachable) {
		logging.Error.Printf("Node %s did not answer the server status poll, its uptime is not recorded until it does: %s", node.Name, err)
	} else if snapshot.reachable && seen && !previous.reachable {
		logging.Info.Printf("Node %s answers the server status poll again", node.Name)
	}

	if err := us.RecordNodeStatuses(serverIDs, snapshot.running, remoteStatusStaleAfter); err != nil {
		logging.Error.Printf("Error recording uptime for node %s: %s", node.Name, err)
	}
}

// remoteNodeServerIDs are the servers the panel places on the node. Servers of the panel's own
// node have no node_id, so they are never among them.
func remoteNodeServerIDs(db *gorm.DB, nodeID uint) ([]string, error) {
	var ids []string
	err := db.Model(&models.Server{}).Where("node_id = ?", nodeID).Pluck("identifier", &ids).Error
	return ids, err
}

// fetchNodeServerStatuses asks the node's daemon for the state of every server it hosts
func fetchNodeServerStatuses(ns *services.Node, node *models.Node) (map[string]skypanel.ServerRunning, error) {
	ctx, cancel := context.WithTimeout(context.Background(), remoteStatusTimeout)
	defer cancel()

	res, err := ns.CallNodeContext(ctx, node, "GET", "/daemon/servers/status", nil, nil)
	if err != nil {
		return nil, err
	}
	defer utils.CloseResponse(res)

	if res.StatusCode != http.StatusOK {
		// a node still on an older version answers 404: it needs updating to be polled
		return nil, fmt.Errorf("status %d", res.StatusCode)
	}
	var answer nodeServersStatus
	if err := json.NewDecoder(io.LimitReader(res.Body, remoteStatusMaxBody)).Decode(&answer); err != nil {
		return nil, err
	}
	return answer.Servers, nil
}

// newNodeSnapshot keeps, of the node's answer, the servers the panel places on that node and does
// not run itself (those the local tracker records). A failed call leaves the node unreachable.
func newNodeSnapshot(serverIDs []string, reported map[string]skypanel.ServerRunning, err error) nodeStatusSnapshot {
	snapshot := nodeStatusSnapshot{at: time.Now(), total: len(serverIDs)}
	if err != nil {
		return snapshot
	}

	snapshot.reachable = true
	snapshot.running = make(map[string]bool, len(serverIDs))
	for _, id := range serverIDs {
		state, ok := reported[id]
		if !ok || GetFromCache(id) != nil {
			continue
		}
		snapshot.running[id] = state.Running
	}
	return snapshot
}

// storeNodeSnapshot saves the node's latest poll and returns the one before it, if any
func storeNodeSnapshot(nodeID uint, snapshot nodeStatusSnapshot) (nodeStatusSnapshot, bool) {
	nodeSnapshotsLock.Lock()
	defer nodeSnapshotsLock.Unlock()
	previous, ok := nodeSnapshots[nodeID]
	nodeSnapshots[nodeID] = snapshot
	return previous, ok
}

// remoteNodeStatus is the node's latest poll for the Discord report. If the poller has not
// polled it lately (it just started, or the node was added a moment ago), the node is asked now;
// that answer is not recorded as uptime, the poller does that on its own schedule.
func remoteNodeStatus(ns *services.Node, node *models.Node) nodeStatusSnapshot {
	nodeSnapshotsLock.RLock()
	snapshot, ok := nodeSnapshots[node.ID]
	nodeSnapshotsLock.RUnlock()
	if ok && time.Since(snapshot.at) <= 2*remoteStatusEvery {
		return snapshot
	}

	serverIDs, err := remoteNodeServerIDs(ns.DB, node.ID)
	if err != nil {
		logging.Error.Printf("Error listing the servers of node %s: %s", node.Name, err)
	}
	reported, err := fetchNodeServerStatuses(ns, node)
	return newNodeSnapshot(serverIDs, reported, err)
}

// onlineOffline counts the servers of a snapshot by state
func (s nodeStatusSnapshot) onlineOffline() (online, offline int) {
	for _, isRunning := range s.running {
		if isRunning {
			online++
		} else {
			offline++
		}
	}
	return
}
