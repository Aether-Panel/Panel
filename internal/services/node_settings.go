package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"gorm.io/gorm"
)

const (
	// nodeSettingsFirstPass waits for the panel to finish starting before the first pass
	nodeSettingsFirstPass = time.Minute
	// nodeSettingsEvery catches up nodes a save could not reach: one that was off, one that
	// just joined or one that was just updated
	nodeSettingsEvery = 10 * time.Minute
	// nodeSettingsNodeTimeout keeps a slow or dead node from holding the pass for the others
	nodeSettingsNodeTimeout = 10 * time.Second
	// nodeSettingsMaxAnswer bounds what is read of a node's answer
	nodeSettingsMaxAnswer = 64 << 10
	nodeSettingsPath      = "/daemon/settings"
	wwwAuthenticateHeader = "WWW-Authenticate"
)

// NodeSyncState says whether a remote node has the panel's node settings
type NodeSyncState string

const (
	// NodeSyncSynced: the node has the same values as the panel
	NodeSyncSynced NodeSyncState = "synced"
	// NodeSyncPending: not checked yet, or the node did not answer or could not save; the next
	// pass tries again by itself
	NodeSyncPending NodeSyncState = "pending"
	// NodeSyncOutdated: the node runs a version without /daemon/settings and needs updating
	NodeSyncOutdated NodeSyncState = "outdated"
	// NodeSyncFailed: the node refused the panel or its package; Error says why
	NodeSyncFailed NodeSyncState = "failed"
)

// NodeSyncStatus is what the settings page shows for one remote node. It never carries a
// setting's value, since they include secrets.
type NodeSyncStatus struct {
	ID    uint          `json:"id"`
	Name  string        `json:"name"`
	State NodeSyncState `json:"state"`
	// Error is the reason a node is failed, for the user to fix; empty otherwise
	Error string `json:"error,omitempty"`
	// CheckedAt is null until the node's first check
	CheckedAt *time.Time `json:"checkedAt"`
	// reason is why a node is pending, for the log only
	reason string
} // @name NodeSyncStatus

// nodeSettingsSync hands the panel's node settings (config.SyncedNodeSettings) to every remote
// node, one node at a time and one pass at a time, and remembers how each node answered
type nodeSettingsSync struct {
	nodeTimeout time.Duration

	mu    sync.Mutex
	ns    *Node
	nodes func() ([]*models.Node, error)
	// running is true while a pass goes on; again asks for one more pass after it
	running  bool
	again    bool
	statuses map[uint]NodeSyncStatus
}

func newNodeSettingsSync(ns *Node, nodes func() ([]*models.Node, error)) *nodeSettingsSync {
	return &nodeSettingsSync{
		nodeTimeout: nodeSettingsNodeTimeout,
		ns:          ns,
		nodes:       nodes,
		statuses:    make(map[uint]NodeSyncStatus),
	}
}

// nodeSettings is the panel's sync. It does nothing until StartNodeSettingsSync gives it the
// database, so a process that is only a node never sends anything.
var nodeSettings = newNodeSettingsSync(nil, nil)

// StartNodeSettingsSync sends the panel's node settings to the remote nodes a minute after the
// panel starts and every 10 minutes after, and lets RequestNodeSettingsSync send them at once
func StartNodeSettingsSync(db *gorm.DB) {
	nodeSettings.mu.Lock()
	nodeSettings.ns = &Node{DB: db}
	nodeSettings.nodes = func() ([]*models.Node, error) { return remoteNodes(db) }
	nodeSettings.mu.Unlock()
	go nodeSettings.run(nodeSettingsFirstPass, nodeSettingsEvery, nil)
}

// RequestNodeSettingsSync sends the panel's node settings to the remote nodes in the
// background, right after a save changed one. It never waits for the nodes: a pass already
// running is followed by one more, which reads the new values.
func RequestNodeSettingsSync() {
	nodeSettings.request()
}

// NodeSettingsSyncStatuses is how every remote node answered the last time, in id order. A
// node not checked yet (it just joined, or the panel just started) is pending.
func NodeSettingsSyncStatuses(db *gorm.DB) ([]NodeSyncStatus, error) {
	nodes, err := remoteNodes(db)
	if err != nil {
		return nil, err
	}
	return nodeSettings.statusesOf(nodes), nil
}

// remoteNodes are the nodes in the database. The panel's own node (models.LocalNode) is never
// a row: it shares the panel's config.json, so it already has the settings.
func remoteNodes(db *gorm.DB) ([]*models.Node, error) {
	var nodes []*models.Node
	err := db.Order("id").Find(&nodes).Error
	return nodes, err
}

func (s *nodeSettingsSync) run(first, every time.Duration, stop <-chan struct{}) {
	timer := time.NewTimer(first)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-stop:
		return
	}
	s.request()

	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.request()
		case <-stop:
			return
		}
	}
}

func (s *nodeSettingsSync) request() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ns == nil {
		return
	}
	if s.running {
		s.again = true
		return
	}
	s.running = true
	go s.drain()
}

// drain runs passes until nobody asked for another one during the last
func (s *nodeSettingsSync) drain() {
	for {
		s.safePass()
		s.mu.Lock()
		if !s.again {
			s.running = false
			s.mu.Unlock()
			return
		}
		s.again = false
		s.mu.Unlock()
	}
}

// safePass keeps a panic in one pass from leaving running set, which would stop every later one
func (s *nodeSettingsSync) safePass() {
	defer skypanel.Recover()
	s.pass()
}

func (s *nodeSettingsSync) busy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// pass checks every remote node in turn and sends the settings to those that differ. The
// values are read once, so every node of a pass gets the same package.
func (s *nodeSettingsSync) pass() {
	s.mu.Lock()
	ns, list := s.ns, s.nodes
	s.mu.Unlock()

	nodes, err := list()
	if err != nil {
		logging.Error.Printf("Error listing the nodes to send the node settings to: %s", err)
		return
	}
	values := config.NodeSettingsValues()
	seen := make(map[uint]bool, len(nodes))
	for _, node := range nodes {
		if node.ID == models.LocalNode.ID {
			continue
		}
		seen[node.ID] = true
		s.record(s.syncNode(ns, node, values))
	}
	s.forgetOthers(seen)
}

// syncNode asks the node for the hash of its settings and sends the whole package only when it
// differs. The node hashes only the keys it lists, so a node of another version compares the
// keys both know.
func (s *nodeSettingsSync) syncNode(ns *Node, node *models.Node, values map[string]interface{}) NodeSyncStatus {
	ctx, cancel := context.WithTimeout(context.Background(), s.nodeTimeout)
	defer cancel()

	status := NodeSyncStatus{ID: node.ID, Name: node.Name, State: NodeSyncSynced}
	state, problem := readNodeSettings(ctx, ns, node)
	if problem == nil && config.NodeSettingsHashFor(values, state.Keys) != state.Hash {
		problem = sendNodeSettings(ctx, ns, node, values)
	}
	if problem != nil {
		status.State = problem.state
		status.reason = problem.reason
		if problem.state == NodeSyncFailed {
			status.Error = problem.reason
		}
	}
	checked := time.Now()
	status.CheckedAt = &checked
	return status
}

// nodeSyncProblem is why a node is not in line, and in which state that leaves it
type nodeSyncProblem struct {
	state  NodeSyncState
	reason string
}

func readNodeSettings(ctx context.Context, ns *Node, node *models.Node) (skypanel.NodeSettingsState, *nodeSyncProblem) {
	var state skypanel.NodeSettingsState
	res, err := ns.CallNodeContext(ctx, node, http.MethodGet, nodeSettingsPath, nil, nil)
	defer utils.CloseResponse(res)
	if problem := nodeAnswerProblem(res, err); problem != nil {
		return state, problem
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, nodeSettingsMaxAnswer)).Decode(&state); err != nil {
		return state, &nodeSyncProblem{state: NodeSyncFailed, reason: "the node's answer about its settings could not be read"}
	}
	return state, nil
}

func sendNodeSettings(ctx context.Context, ns *Node, node *models.Node, values map[string]interface{}) *nodeSyncProblem {
	data, err := json.Marshal(values)
	if err != nil {
		return &nodeSyncProblem{state: NodeSyncFailed, reason: "the panel could not pack its node settings"}
	}
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	res, err := ns.CallNodeContext(ctx, node, http.MethodPut, nodeSettingsPath, io.NopCloser(bytes.NewReader(data)), headers)
	defer utils.CloseResponse(res)
	return nodeAnswerProblem(res, err)
}

// nodeAnswerProblem sorts a node's answer: no answer or a 5xx is retried by the next pass (a
// 5xx means the node could not save its config, though the values may already be live); a 404
// is a node too old to have the route; a refused token or a refused package is something the
// user has to fix, so it keeps the node's message.
func nodeAnswerProblem(res *http.Response, err error) *nodeSyncProblem {
	switch {
	case err != nil:
		return &nodeSyncProblem{state: NodeSyncPending, reason: err.Error()}
	case res.StatusCode == http.StatusNotFound:
		return &nodeSyncProblem{state: NodeSyncOutdated, reason: "the node has no /daemon/settings yet"}
	case NodeRefusedPanel(res):
		return &nodeSyncProblem{state: NodeSyncFailed, reason: skypanel.ErrNodeAuthFailed.Message}
	case res.StatusCode >= http.StatusInternalServerError:
		return &nodeSyncProblem{state: NodeSyncPending, reason: fmt.Sprintf("the node answered %d", res.StatusCode)}
	case res.StatusCode < 200 || res.StatusCode >= 300:
		return &nodeSyncProblem{state: NodeSyncFailed, reason: nodeErrorMessage(res)}
	}
	return nil
}

// nodeErrorMessage is the message of the node's error answer. A node names the key and the
// problem, never the value, so it is safe to show and log.
func nodeErrorMessage(res *http.Response) string {
	var answer skypanel.ErrorResponse
	if err := json.NewDecoder(io.LimitReader(res.Body, nodeSettingsMaxAnswer)).Decode(&answer); err == nil && answer.Error != nil && answer.Error.Message != "" {
		return answer.Error.Message
	}
	return fmt.Sprintf("the node answered %d", res.StatusCode)
}

// NodeRefusedPanel reports whether a node's answer is the node refusing the panel itself, not
// an answer about what was asked. A node checks only the panel's token, so a 401, or a 403
// that carries an authentication challenge, means the two do not trust each other: another
// secret, a clock that is off, or a proxy in front of the node asking for credentials. Any
// other 403 is the node's own answer.
func NodeRefusedPanel(res *http.Response) bool {
	if res == nil {
		return false
	}
	switch res.StatusCode {
	case http.StatusUnauthorized:
		return true
	case http.StatusForbidden:
		return res.Header.Get(wwwAuthenticateHeader) != ""
	}
	return false
}

// record keeps the node's status and logs it only when it changed, so a node that stays off
// does not fill the log every 10 minutes
func (s *nodeSettingsSync) record(status NodeSyncStatus) {
	s.mu.Lock()
	previous, seen := s.statuses[status.ID]
	s.statuses[status.ID] = status
	s.mu.Unlock()
	if seen && previous.State == status.State && previous.reason == status.reason {
		return
	}
	switch status.State {
	case NodeSyncSynced:
		logging.Info.Printf("Node %q has the panel's node settings", status.Name)
	case NodeSyncPending:
		logging.Info.Printf("Node %q did not take the panel's node settings yet, the next pass tries again: %s", status.Name, status.reason)
	case NodeSyncOutdated:
		logging.Info.Printf("Node %q needs updating to take the panel's node settings", status.Name)
	case NodeSyncFailed:
		logging.Error.Printf("Node %q did not take the panel's node settings: %s", status.Name, status.reason)
	}
}

// forgetOthers drops the statuses of nodes that are gone
func (s *nodeSettingsSync) forgetOthers(nodes map[uint]bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.statuses {
		if !nodes[id] {
			delete(s.statuses, id)
		}
	}
}

func (s *nodeSettingsSync) statusesOf(nodes []*models.Node) []NodeSyncStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	statuses := make([]NodeSyncStatus, 0, len(nodes))
	for _, node := range nodes {
		if node.ID == models.LocalNode.ID {
			continue
		}
		status, ok := s.statuses[node.ID]
		if !ok {
			status = NodeSyncStatus{State: NodeSyncPending}
		}
		// the name as it is now, even if the node was renamed since its check
		status.ID, status.Name = node.ID, node.Name
		status.reason = ""
		statuses = append(statuses, status)
	}
	return statuses
}
