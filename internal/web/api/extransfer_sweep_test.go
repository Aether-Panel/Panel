package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// recordArchiveRemovals swaps the node call for a record of the servers whose archive was removed
func recordArchiveRemovals(t *testing.T) func() []string {
	t.Helper()
	var mu sync.Mutex
	var removed []string
	previous := removeOriginArchive
	removeOriginArchive = func(_ *gorm.DB, serverID string) bool {
		mu.Lock()
		removed = append(removed, serverID)
		mu.Unlock()
		return true
	}
	t.Cleanup(func() { removeOriginArchive = previous })
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), removed...)
	}
}

func expiredAgo(d time.Duration) func(*models.ExTransferSession) {
	return func(s *models.ExTransferSession) { s.ExpiresAt = time.Now().Add(-d) }
}

func TestSweepExpiredExTransfers_EndsTheExpiredSessionAndRemovesItsArchive(t *testing.T) {
	db := setupExTransferTest(t)
	removed := recordArchiveRemovals(t)
	pub, _ := newExTestKey(t)
	stuck := newExTestSession(t, db, models.StatusMigrating, pub, expiredAgo(time.Minute))

	ended, err := sweepExpiredExTransfers(db, time.Now())

	require.NoError(t, err)
	assert.Equal(t, 1, ended)
	assert.Equal(t, models.StatusFailed, reloadExSession(t, db, stuck).Status)
	assert.EqualValues(t, 1, countExLogs(t, db, stuck.SessionUUID, "EXPIRE"))
	assert.Equal(t, []string{exTestServer}, removed())
	assert.EqualValues(t, 1, countExLogs(t, db, stuck.SessionUUID, "ARCHIVE_REMOVED"))

	// a second sweep finds nothing left to end
	ended, err = sweepExpiredExTransfers(db, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 0, ended)
	assert.Len(t, removed(), 1)
}

func TestSweepExpiredExTransfers_LeavesActiveAndFinishedSessionsAlone(t *testing.T) {
	db := setupExTransferTest(t)
	removed := recordArchiveRemovals(t)
	pub, _ := newExTestKey(t)
	active := newExTestSession(t, db, models.StatusMigrating, pub, func(s *models.ExTransferSession) {
		now := time.Now()
		s.LastHeartbeatAt = &now
		s.ExpiresAt = now.Add(exTransferHeartbeatExtension)
	})
	completed := newExTestSession(t, db, models.StatusCompleted, pub, expiredAgo(time.Hour))
	cancelled := newExTestSession(t, db, models.StatusCancelled, pub, expiredAgo(time.Hour))
	failed := newExTestSession(t, db, models.StatusFailed, pub, expiredAgo(time.Hour))

	ended, err := sweepExpiredExTransfers(db, time.Now())

	require.NoError(t, err)
	assert.Equal(t, 0, ended)
	assert.Equal(t, models.StatusMigrating, reloadExSession(t, db, active).Status)
	assert.Equal(t, models.StatusCompleted, reloadExSession(t, db, completed).Status)
	assert.Equal(t, models.StatusCancelled, reloadExSession(t, db, cancelled).Status)
	assert.Equal(t, models.StatusFailed, reloadExSession(t, db, failed).Status)
	assert.Empty(t, removed(), "no archive is touched")
}

// An unused token expires too, but nothing was archived for it.
func TestSweepExpiredExTransfers_AnUnusedTokenLeftNoArchive(t *testing.T) {
	db := setupExTransferTest(t)
	removed := recordArchiveRemovals(t)
	pub, _ := newExTestKey(t)
	created := newExTestSession(t, db, models.StatusCreated, nil, expiredAgo(time.Minute))
	validated := newExTestSession(t, db, models.StatusValidated, pub, expiredAgo(time.Minute))

	ended, err := sweepExpiredExTransfers(db, time.Now())

	require.NoError(t, err)
	assert.Equal(t, 2, ended)
	assert.Equal(t, models.StatusFailed, reloadExSession(t, db, created).Status)
	assert.Equal(t, models.StatusFailed, reloadExSession(t, db, validated).Status)
	assert.EqualValues(t, 1, countExLogs(t, db, created.SessionUUID, "EXPIRE_UNUSED"))
	assert.Empty(t, removed())

	_, err = sweepExpiredExTransfers(db, time.Now())
	require.NoError(t, err)
	assert.Empty(t, removed(), "nothing is waiting to be removed")
}

// The archive has the same name for every transfer of the server: while another one is
// sending or receiving it, removing it would break that transfer.
func TestSweepExpiredExTransfers_KeepsTheArchiveOfAServerStillInATransfer(t *testing.T) {
	db := setupExTransferTest(t)
	removed := recordArchiveRemovals(t)
	pub, _ := newExTestKey(t)
	stuck := newExTestSession(t, db, models.StatusMigrating, pub, expiredAgo(time.Minute))
	newExTestSession(t, db, models.StatusMigrating, pub, nil)

	_, err := sweepExpiredExTransfers(db, time.Now())
	require.NoError(t, err)
	assert.Equal(t, models.StatusFailed, reloadExSession(t, db, stuck).Status)
	assert.Empty(t, removed(), "a newer session is sending the archive")

	// a pull into the same server running on this panel writes the same file
	const pulling = "srv02"
	other := newExTestSession(t, db, models.StatusMigrating, pub, func(s *models.ExTransferSession) {
		s.ServerID = pulling
		s.ExpiresAt = time.Now().Add(-time.Minute)
	})
	_, finish, ok := startPull(pulling)
	require.True(t, ok)
	defer finish()

	_, err = sweepExpiredExTransfers(db, time.Now())
	require.NoError(t, err)
	assert.Equal(t, models.StatusFailed, reloadExSession(t, db, other).Status)
	assert.Empty(t, removed(), "a pull is writing the archive")
}

func TestSweepExpiredExTransfers_OnlyTheExpiredServersArchive(t *testing.T) {
	db := setupExTransferTest(t)
	removed := recordArchiveRemovals(t)
	pub, _ := newExTestKey(t)
	newExTestSession(t, db, models.StatusMigrating, pub, expiredAgo(time.Minute))
	newExTestSession(t, db, models.StatusMigrating, pub, func(s *models.ExTransferSession) { s.ServerID = "srv03" })

	_, err := sweepExpiredExTransfers(db, time.Now())

	require.NoError(t, err)
	assert.Equal(t, []string{exTestServer}, removed())
}

// A heartbeat that lands between the sweep reading the session and ending it wins.
func TestExpireExTransferSession_AHeartbeatInBetweenKeepsItAlive(t *testing.T) {
	db := setupExTransferTest(t)
	pub, _ := newExTestKey(t)
	session := newExTestSession(t, db, models.StatusMigrating, pub, expiredAgo(time.Second))
	read := *reloadExSession(t, db, session)
	require.NoError(t, db.Model(&models.ExTransferSession{}).Where("id = ?", session.ID).Update("expires_at", time.Now().Add(time.Minute)).Error)

	ended, err := expireExTransferSession(db, &read, time.Now())

	require.NoError(t, err)
	assert.False(t, ended)
	assert.Equal(t, models.StatusMigrating, reloadExSession(t, db, session).Status)
}

// archiveNode is a fake node holding files of srv01 that answers like the daemon: a delete gets
// 204 when the file was there, 404 when not, and every route a bare 404 while the server is
// not loaded. Setting status makes it answer that instead, and hang makes it never answer.
type archiveNode struct {
	mu       sync.Mutex
	files    map[string]bool
	unloaded bool
	status   int
	hang     bool
	deletes  int
}

func (n *archiveNode) serve(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	n.mu.Lock()
	unloaded := n.unloaded
	n.mu.Unlock()
	if unloaded {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/daemon/server/"+exTestServer+"/status" {
		_, _ = w.Write([]byte(`{"running":false}`))
		return
	}
	name, ok := strings.CutPrefix(r.URL.Path, "/daemon/server/"+exTestServer+"/file/")
	if r.Method != http.MethodDelete || !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	n.mu.Lock()
	n.deletes++
	hang, status := n.hang, n.status
	n.mu.Unlock()
	if hang {
		<-r.Context().Done()
		return
	}
	if status != 0 {
		w.WriteHeader(status)
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.files[name] {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	delete(n.files, name)
	w.WriteHeader(http.StatusNoContent)
}

func (n *archiveNode) setLoaded(loaded bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.unloaded = !loaded
}

func (n *archiveNode) set(status int, hang bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.status, n.hang = status, hang
}

func (n *archiveNode) state() (left map[string]bool, deletes int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	left = make(map[string]bool, len(n.files))
	for k, v := range n.files {
		left[k] = v
	}
	return left, n.deletes
}

// setupArchiveNode stores srv01 on a fake node holding the files, with the real removal in place
func setupArchiveNode(t *testing.T, files map[string]bool) (*gorm.DB, *archiveNode) {
	t.Helper()
	// setupExTransferTest swaps the removal out; these tests need the real one
	realRemoval := removeOriginArchive
	db := setupExTransferTest(t)
	removeOriginArchive = realRemoval

	fake := &archiveNode{files: files}
	srv := httptest.NewServer(http.HandlerFunc(fake.serve))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	node := &models.Node{Name: "remote", PublicHost: "127.0.0.1", PrivateHost: "127.0.0.1", PublicPort: uint16(port), PrivatePort: uint16(port), SFTPPort: 5658, Secret: "node-secret-0123456789"}
	require.NoError(t, db.Create(node).Error)
	require.NoError(t, db.Omit(clause.Associations).Create(&models.Server{Identifier: exTestServer, Name: exTestServer, Type: "generic", NodeID: node.ID}).Error)
	return db, fake
}

// The real removal against a node: the archive goes, the other files stay, and a server
// whose folder has no archive is no error.
func TestSweepExpiredExTransfers_RemovesTheArchiveOnTheNode(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]bool
		left  map[string]bool
	}{
		{"with an archive", map[string]bool{originArchive: true, "world.dat": true}, map[string]bool{"world.dat": true}},
		{"with half an archive", map[string]bool{originArchivePartial: true}, map[string]bool{}},
		{"without an archive", map[string]bool{}, map[string]bool{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, node := setupArchiveNode(t, tc.files)
			pub, _ := newExTestKey(t)
			stuck := newExTestSession(t, db, models.StatusMigrating, pub, expiredAgo(time.Minute))

			_, err := sweepExpiredExTransfers(db, time.Now())

			require.NoError(t, err)
			assert.Equal(t, models.StatusFailed, reloadExSession(t, db, stuck).Status)
			left, _ := node.state()
			assert.Equal(t, tc.left, left)
			assert.EqualValues(t, 1, countExLogs(t, db, stuck.SessionUUID, "ARCHIVE_REMOVED"))
		})
	}
}

// A node that answers an error (it just rebooted, or refuses the panel) keeps the archive:
// every run tries again until the node confirms it is gone, and then stops.
func TestSweepExpiredExTransfers_RetriesAFailedRemoval(t *testing.T) {
	db, node := setupArchiveNode(t, map[string]bool{originArchive: true})
	pub, _ := newExTestKey(t)
	stuck := newExTestSession(t, db, models.StatusMigrating, pub, expiredAgo(time.Minute))

	for _, status := range []int{http.StatusInternalServerError, http.StatusUnauthorized} {
		node.set(status, false)
		_, err := sweepExpiredExTransfers(db, time.Now())
		require.NoError(t, err)
		left, _ := node.state()
		assert.True(t, left[originArchive], "the node answered %d", status)
		assert.EqualValues(t, 0, countExLogs(t, db, stuck.SessionUUID, "ARCHIVE_REMOVED"))
	}
	assert.Equal(t, models.StatusFailed, reloadExSession(t, db, stuck).Status)
	assert.EqualValues(t, 1, countExLogs(t, db, stuck.SessionUUID, "EXPIRE"), "the session is expired once")

	node.set(0, false)
	_, err := sweepExpiredExTransfers(db, time.Now())
	require.NoError(t, err)
	left, deletes := node.state()
	assert.Empty(t, left)
	assert.EqualValues(t, 1, countExLogs(t, db, stuck.SessionUUID, "ARCHIVE_REMOVED"))

	_, err = sweepExpiredExTransfers(db, time.Now())
	require.NoError(t, err)
	_, after := node.state()
	assert.Equal(t, deletes, after, "a removed archive is not asked for again")
}

// A node that never answers must not hold up the sweep, nor the server's archive lock.
func TestSweepExpiredExTransfers_AHangingNodeTimesOut(t *testing.T) {
	db, node := setupArchiveNode(t, map[string]bool{originArchive: true})
	node.set(0, true)
	previous := originArchiveRemoveTimeout
	originArchiveRemoveTimeout = 200 * time.Millisecond
	t.Cleanup(func() { originArchiveRemoveTimeout = previous })
	pub, _ := newExTestKey(t)
	stuck := newExTestSession(t, db, models.StatusMigrating, pub, expiredAgo(time.Minute))

	start := time.Now()
	_, err := sweepExpiredExTransfers(db, time.Now())

	require.NoError(t, err)
	assert.Less(t, time.Since(start), 10*time.Second)
	assert.EqualValues(t, 0, countExLogs(t, db, stuck.SessionUUID, "ARCHIVE_REMOVED"), "tried again next run")
	unlock := lockOriginArchive(exTestServer)
	unlock()
}

// A session that cannot be ended is reported, and the others still are, archives included.
func TestSweepExpiredExTransfers_OneFailingSessionDoesNotStopTheOthers(t *testing.T) {
	db := setupExTransferTest(t)
	removed := recordArchiveRemovals(t)
	pub, _ := newExTestKey(t)
	broken := newExTestSession(t, db, models.StatusMigrating, pub, expiredAgo(time.Minute))
	stuck := newExTestSession(t, db, models.StatusMigrating, pub, func(s *models.ExTransferSession) {
		s.ServerID = "srv03"
		s.ExpiresAt = time.Now().Add(-time.Minute)
	})
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:fail_one_log", func(tx *gorm.DB) {
		if row, ok := tx.Statement.Dest.(*models.ExTransferLog); ok && row.SessionID == broken.SessionUUID {
			_ = tx.AddError(errors.New("disk full"))
		}
	}))

	ended, err := sweepExpiredExTransfers(db, time.Now())

	require.ErrorContains(t, err, broken.SessionUUID)
	assert.Equal(t, 1, ended)
	assert.Equal(t, models.StatusMigrating, reloadExSession(t, db, broken).Status, "rolled back, ended next run")
	assert.Equal(t, models.StatusFailed, reloadExSession(t, db, stuck).Status)
	assert.Equal(t, []string{"srv03"}, removed())
}

// A node that has not loaded the server answers its file delete with the same bare 404 as a
// missing file; nothing was removed then, so the sweep tries again.
func TestSweepExpiredExTransfers_AServerTheNodeHasNotLoadedIsNotRemoved(t *testing.T) {
	db, node := setupArchiveNode(t, map[string]bool{originArchive: true})
	node.setLoaded(false)
	pub, _ := newExTestKey(t)
	stuck := newExTestSession(t, db, models.StatusMigrating, pub, expiredAgo(time.Minute))

	_, err := sweepExpiredExTransfers(db, time.Now())
	require.NoError(t, err)
	assert.EqualValues(t, 0, countExLogs(t, db, stuck.SessionUUID, "ARCHIVE_REMOVED"))

	node.setLoaded(true)
	_, err = sweepExpiredExTransfers(db, time.Now())
	require.NoError(t, err)
	left, _ := node.state()
	assert.Empty(t, left)
	assert.EqualValues(t, 1, countExLogs(t, db, stuck.SessionUUID, "ARCHIVE_REMOVED"))
}

// switchArchiveRemoval swaps the node call for one that answers what removable says, and
// counts the calls
func switchArchiveRemoval(t *testing.T) (setRemovable func(bool), calls func() int) {
	t.Helper()
	var mu sync.Mutex
	removable, n := false, 0
	previous := removeOriginArchive
	removeOriginArchive = func(*gorm.DB, string) bool {
		mu.Lock()
		defer mu.Unlock()
		n++
		return removable
	}
	t.Cleanup(func() { removeOriginArchive = previous })
	setRemovable = func(v bool) {
		mu.Lock()
		defer mu.Unlock()
		removable = v
	}
	calls = func() int {
		mu.Lock()
		defer mu.Unlock()
		return n
	}
	return setRemovable, calls
}

// Confirm and cancel remove the archive once; when the node does not confirm it, the sweep
// takes over until it does, and never while the server is in a live transfer.
func TestSessionArchiveRemoval_TheSweepRetriesWhatTheNodeDidNotConfirm(t *testing.T) {
	db := setupExTransferTest(t)
	setRemovable, calls := switchArchiveRemoval(t)
	pub, priv := newExTestKey(t)
	confirmed := newExTestSession(t, db, models.StatusMigrating, pub, nil)

	code, _ := callExSigned(t, db, confirmExTransfer, confirmed.SessionUUID, exSign(priv, "CONFIRM", confirmed.SessionUUID))
	require.Equal(t, http.StatusOK, code)
	assert.Eventually(t, func() bool {
		return countExLogs(t, db, confirmed.SessionUUID, "ARCHIVE_PENDING") == 1
	}, 5*time.Second, 10*time.Millisecond)

	// a newer transfer of the server is sending its own archive: left alone
	live := newExTestSession(t, db, models.StatusMigrating, pub, nil)
	setRemovable(true)
	before := calls()
	_, err := sweepExpiredExTransfers(db, time.Now())
	require.NoError(t, err)
	assert.Equal(t, before, calls(), "a live transfer's archive is never touched")
	assert.EqualValues(t, 0, countExLogs(t, db, confirmed.SessionUUID, "ARCHIVE_REMOVED"))

	require.NoError(t, db.Model(&models.ExTransferSession{}).Where("id = ?", live.ID).Update("status", models.StatusCancelled).Error)
	_, err = sweepExpiredExTransfers(db, time.Now())
	require.NoError(t, err)
	assert.Equal(t, before+1, calls())
	assert.EqualValues(t, 1, countExLogs(t, db, confirmed.SessionUUID, "ARCHIVE_REMOVED"))

	_, err = sweepExpiredExTransfers(db, time.Now())
	require.NoError(t, err)
	assert.Equal(t, before+1, calls(), "done once the node confirmed")
}

func TestSessionArchiveRemoval_NothingToRetryWhenTheNodeConfirms(t *testing.T) {
	db := setupExTransferTest(t)
	setRemovable, calls := switchArchiveRemoval(t)
	setRemovable(true)
	pub, _ := newExTestKey(t)
	cancelled := newExTestSession(t, db, models.StatusCancelled, pub, nil)

	sessionArchiveRemoval(db, exTestServer, cancelled.SessionUUID)()

	assert.Equal(t, 1, calls())
	assert.EqualValues(t, 0, countExLogs(t, db, cancelled.SessionUUID, "ARCHIVE_PENDING"))
	_, err := sweepExpiredExTransfers(db, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 1, calls())
}

// The removal right after a confirm goes through the sweep's guards: while another session of
// the server is still sending its own archive (same file names), it waits for the sweep.
func TestSessionArchiveRemoval_LeavesTheArchiveOfALiveTransfer(t *testing.T) {
	db := setupExTransferTest(t)
	setRemovable, calls := switchArchiveRemoval(t)
	setRemovable(true)
	pub, priv := newExTestKey(t)
	a := newExTestSession(t, db, models.StatusMigrating, pub, nil)
	b := newExTestSession(t, db, models.StatusMigrating, pub, nil)

	code, _ := callExSigned(t, db, confirmExTransfer, a.SessionUUID, exSign(priv, "CONFIRM", a.SessionUUID))
	require.Equal(t, http.StatusOK, code)
	assert.Eventually(t, func() bool {
		return countExLogs(t, db, a.SessionUUID, "ARCHIVE_PENDING") == 1
	}, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, 0, calls(), "b's archive was not touched")

	// without b the confirm removes it at once
	code, _ = callExSigned(t, db, confirmExTransfer, b.SessionUUID, exSign(priv, "CONFIRM", b.SessionUUID))
	require.Equal(t, http.StatusOK, code)
	assert.Eventually(t, func() bool { return calls() == 1 }, 5*time.Second, 10*time.Millisecond)
	assert.EqualValues(t, 0, countExLogs(t, db, b.SessionUUID, "ARCHIVE_PENDING"))
}
