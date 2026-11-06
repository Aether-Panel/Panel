package services

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSettingsNode is a remote node's daemon that answers GET and PUT /daemon/settings the way
// the test sets it up, and records what the panel sent it
type fakeSettingsNode struct {
	mu        sync.Mutex
	hash      string
	keys      []string
	getStatus int
	putStatus int
	putAnswer string
	// hold, when set, keeps every GET waiting until it is closed
	hold     chan struct{}
	gets     int
	inFlight int
	most     int
	puts     []string
	noToken  bool
}

func newFakeSettingsNode(t *testing.T, id uint) (*fakeSettingsNode, *models.Node) {
	fake := &fakeSettingsNode{getStatus: http.StatusOK, putStatus: http.StatusNoContent, keys: config.NodeSettingsKeys()}
	srv := httptest.NewServer(http.HandlerFunc(fake.serve))
	t.Cleanup(srv.Close)
	return fake, nodeAt(t, id, srv.URL)
}

func nodeAt(t *testing.T, id uint, address string) *models.Node {
	u, err := url.Parse(address)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	return &models.Node{ID: id, Name: "node-" + strconv.Itoa(int(id)), PublicHost: "127.0.0.1", PrivateHost: "127.0.0.1", PublicPort: uint16(port), PrivatePort: uint16(port)}
}

func (f *fakeSettingsNode) serve(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		// the panel's probe for TLS
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.URL.Path != "/daemon/settings" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	f.mu.Lock()
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		f.noToken = true
	}
	f.mu.Unlock()
	switch r.Method {
	case http.MethodGet:
		f.answerGet(w)
	case http.MethodPut:
		f.answerPut(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (f *fakeSettingsNode) answerGet(w http.ResponseWriter) {
	f.mu.Lock()
	f.gets++
	f.inFlight++
	if f.inFlight > f.most {
		f.most = f.inFlight
	}
	hold, status := f.hold, f.getStatus
	state := skypanel.NodeSettingsState{Hash: f.hash, Keys: f.keys}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.inFlight--
		f.mu.Unlock()
	}()
	if hold != nil {
		<-hold
	}
	if status != http.StatusOK {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

func (f *fakeSettingsNode) answerPut(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	_ = json.NewDecoder(r.Body).Decode(&body)
	sent, _ := json.Marshal(body)
	f.mu.Lock()
	f.puts = append(f.puts, string(sent))
	status, answer := f.putStatus, f.putAnswer
	f.mu.Unlock()
	if answer != "" {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(status)
	_, _ = w.Write([]byte(answer))
}

func (f *fakeSettingsNode) sent() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.puts...)
}

func (f *fakeSettingsNode) getCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.gets
}

// syncTo is a sync of its own (not the panel's) that sends to the given nodes
func syncTo(nodes ...*models.Node) *nodeSettingsSync {
	s := newNodeSettingsSync(&Node{}, func() ([]*models.Node, error) { return nodes, nil })
	s.nodeTimeout = 5 * time.Second
	return s
}

func statusOf(t *testing.T, s *nodeSettingsSync, node *models.Node) NodeSyncStatus {
	statuses := s.statusesOf([]*models.Node{node})
	require.Len(t, statuses, 1)
	return statuses[0]
}

func panelPackage(t *testing.T) string {
	data, err := json.Marshal(config.NodeSettingsValues())
	require.NoError(t, err)
	return string(data)
}

func TestNodeSettingsSync_SameHashSendsNothing(t *testing.T) {
	fake, node := newFakeSettingsNode(t, 1)
	fake.hash = config.NodeSettingsHashFor(config.NodeSettingsValues(), fake.keys)
	s := syncTo(node)

	s.pass()

	assert.Empty(t, fake.sent(), "a node that already has the settings gets no PUT")
	status := statusOf(t, s, node)
	assert.Equal(t, NodeSyncSynced, status.State)
	assert.Empty(t, status.Error)
	assert.NotNil(t, status.CheckedAt)
	assert.False(t, fake.noToken, "every call carries the panel's token")
}

func TestNodeSettingsSync_OtherHashSendsTheWholePackage(t *testing.T) {
	fake, node := newFakeSettingsNode(t, 1)
	fake.hash = "an older package"
	s := syncTo(node)

	s.pass()

	sent := fake.sent()
	require.Len(t, sent, 1)
	assert.JSONEq(t, panelPackage(t), sent[0], "every synced key with the panel's value")
	assert.Equal(t, NodeSyncSynced, statusOf(t, s, node).State)
}

// The node hashes only the keys it lists, so a node that knows fewer keys (an older version)
// is still in line when those keys match
func TestNodeSettingsSync_ComparesOnlyTheKeysTheNodeKnows(t *testing.T) {
	fake, node := newFakeSettingsNode(t, 1)
	fake.keys = config.NodeSettingsKeys()[:3]
	fake.hash = config.NodeSettingsHashFor(config.NodeSettingsValues(), fake.keys)
	s := syncTo(node)

	s.pass()

	assert.Empty(t, fake.sent())
	assert.Equal(t, NodeSyncSynced, statusOf(t, s, node).State)
}

func TestNodeSettingsSync_ANodeWithoutTheRouteIsOutdated(t *testing.T) {
	fake, node := newFakeSettingsNode(t, 1)
	fake.getStatus = http.StatusNotFound
	s := syncTo(node)

	s.pass()

	assert.Empty(t, fake.sent())
	assert.Equal(t, NodeSyncOutdated, statusOf(t, s, node).State)
}

func TestNodeSettingsSync_ARefusedPackageFailsWithTheNodesMessage(t *testing.T) {
	fake, node := newFakeSettingsNode(t, 1)
	fake.hash = "stale"
	fake.putStatus = http.StatusBadRequest
	fake.putAnswer = `{"error":{"code":"ErrNodeSettingInvalid","msg":"node setting daemon.backups.s3.bucket must be text"}}`
	s := syncTo(node)

	s.pass()

	status := statusOf(t, s, node)
	assert.Equal(t, NodeSyncFailed, status.State)
	assert.Equal(t, "node setting daemon.backups.s3.bucket must be text", status.Error)
}

func TestNodeSettingsSync_ARefusedTokenFailsWithAClearMessage(t *testing.T) {
	fake, node := newFakeSettingsNode(t, 1)
	fake.getStatus = http.StatusUnauthorized
	s := syncTo(node)

	s.pass()

	status := statusOf(t, s, node)
	assert.Equal(t, NodeSyncFailed, status.State)
	assert.Equal(t, skypanel.ErrNodeAuthFailed.Message, status.Error)
}

func TestNodeSettingsSync_ANodeThatDoesNotAnswerIsPending(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := "http://" + listener.Addr().String()
	require.NoError(t, listener.Close())
	node := nodeAt(t, 1, address)
	s := syncTo(node)

	s.pass()

	status := statusOf(t, s, node)
	assert.Equal(t, NodeSyncPending, status.State)
	assert.Empty(t, status.Error, "nothing for the user to fix: it is retried")
	assert.NotNil(t, status.CheckedAt)
}

// A node that hangs is given up on after the time each node gets, so it cannot hold the pass
func TestNodeSettingsSync_ASlowNodeIsPendingAfterItsTime(t *testing.T) {
	fake, node := newFakeSettingsNode(t, 1)
	fake.hold = make(chan struct{})
	// runs before the server's own cleanup, which waits for the held request
	t.Cleanup(func() { close(fake.hold) })
	s := syncTo(node)
	s.nodeTimeout = 200 * time.Millisecond

	started := time.Now()
	s.pass()

	assert.Less(t, time.Since(started), 3*time.Second)
	assert.Equal(t, NodeSyncPending, statusOf(t, s, node).State)
}

// A node that could not save its config (5xx) is tried again on the next pass
func TestNodeSettingsSync_ANodeErrorIsPending(t *testing.T) {
	fake, node := newFakeSettingsNode(t, 1)
	fake.hash = "stale"
	fake.putStatus = http.StatusInternalServerError
	s := syncTo(node)

	s.pass()

	assert.Equal(t, NodeSyncPending, statusOf(t, s, node).State)
}

func TestNodeSettingsSync_SkipsThePanelsOwnNode(t *testing.T) {
	var localCalls atomic.Int32
	useEngine(t, func(e *gin.Engine) {
		e.Any("/daemon/settings", func(c *gin.Context) {
			localCalls.Add(1)
			c.Status(http.StatusNotFound)
		})
	})
	fake, node := newFakeSettingsNode(t, 1)
	fake.hash = "stale"
	s := syncTo(models.LocalNode, node)

	s.pass()

	assert.Zero(t, localCalls.Load(), "the panel's node shares the panel's config.json")
	assert.Len(t, fake.sent(), 1)
	statuses := s.statusesOf([]*models.Node{models.LocalNode, node})
	require.Len(t, statuses, 1)
	assert.Equal(t, uint(1), statuses[0].ID)
}

func TestNodeSettingsSync_ANodeNotCheckedYetIsPending(t *testing.T) {
	s := syncTo()
	node := &models.Node{ID: 4, Name: "new"}

	status := statusOf(t, s, node)

	assert.Equal(t, NodeSyncStatus{ID: 4, Name: "new", State: NodeSyncPending}, status)
}

func TestNodeSettingsSync_ForgetsRemovedNodes(t *testing.T) {
	fake, node := newFakeSettingsNode(t, 1)
	fake.hash = "stale"
	nodes := []*models.Node{node}
	s := newNodeSettingsSync(&Node{}, func() ([]*models.Node, error) { return nodes, nil })
	s.pass()
	nodes = nil

	s.pass()

	s.mu.Lock()
	defer s.mu.Unlock()
	assert.Empty(t, s.statuses)
}

// A save asks for a pass and goes on at once; asking again during a pass runs exactly one more
// after it, never two passes at the same time
func TestNodeSettingsSync_RequestsNeverOverlap(t *testing.T) {
	fake, node := newFakeSettingsNode(t, 1)
	fake.hold = make(chan struct{})
	s := syncTo(node)

	returned := make(chan struct{})
	go func() {
		s.request()
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("asking for a pass waited for the pass")
	}
	require.Eventually(t, func() bool { return fake.getCount() == 1 }, 5*time.Second, 10*time.Millisecond)

	var asking sync.WaitGroup
	for i := 0; i < 5; i++ {
		asking.Add(1)
		go func() {
			defer asking.Done()
			s.request()
		}()
	}
	asking.Wait()
	close(fake.hold)

	require.Eventually(t, func() bool { return !s.busy() }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, 2, fake.getCount(), "the requests during the pass made one more pass")
	fake.mu.Lock()
	assert.Equal(t, 1, fake.most, "passes never overlap")
	fake.mu.Unlock()
}

func TestNodeSettingsSync_RequestBeforeStartDoesNothing(t *testing.T) {
	s := newNodeSettingsSync(nil, nil)
	s.request()
	assert.False(t, s.busy())
}

func TestNodeSettingsSync_WorkerPassesAgainAndAgain(t *testing.T) {
	fake, node := newFakeSettingsNode(t, 1)
	fake.hash = config.NodeSettingsHashFor(config.NodeSettingsValues(), fake.keys)
	s := syncTo(node)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.run(10*time.Millisecond, 20*time.Millisecond, stop)
	}()

	assert.Eventually(t, func() bool { return fake.getCount() >= 3 }, 5*time.Second, 10*time.Millisecond)
	close(stop)
	<-done
}

func TestNodeSyncStatus_JSONHasNoSettingValues(t *testing.T) {
	now := time.Now()
	data, err := json.Marshal(NodeSyncStatus{ID: 2, Name: "n", State: NodeSyncFailed, Error: "boom", CheckedAt: &now})
	require.NoError(t, err)
	var fields map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &fields))
	assert.ElementsMatch(t, []string{"id", "name", "state", "error", "checkedAt"}, keysOf(fields))

	data, err = json.Marshal(NodeSyncStatus{ID: 2, Name: "n", State: NodeSyncPending})
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":2,"name":"n","state":"pending","checkedAt":null}`, string(data))
}
