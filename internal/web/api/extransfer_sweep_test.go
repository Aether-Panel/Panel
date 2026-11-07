package api

import (
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
	removeOriginArchive = func(_ *gorm.DB, serverID string) {
		mu.Lock()
		removed = append(removed, serverID)
		mu.Unlock()
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
	assert.Empty(t, removed())
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

// fakeArchiveNode is a node holding the given files of srv01, answering deletes like the daemon:
// 204 when the file was there, 404 when not
func fakeArchiveNode(t *testing.T, files map[string]bool) (*models.Node, func() map[string]bool) {
	t.Helper()
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		name, ok := strings.CutPrefix(r.URL.Path, "/daemon/server/"+exTestServer+"/file/")
		if r.Method != http.MethodDelete || !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if !files[name] {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		delete(files, name)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	node := &models.Node{Name: "remote", PublicHost: "127.0.0.1", PrivateHost: "127.0.0.1", PublicPort: uint16(port), PrivatePort: uint16(port), SFTPPort: 5658, Secret: "node-secret-0123456789"}
	return node, func() map[string]bool {
		mu.Lock()
		defer mu.Unlock()
		left := make(map[string]bool, len(files))
		for k, v := range files {
			left[k] = v
		}
		return left
	}
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
			// setupExTransferTest swaps the removal out; this test needs the real one
			realRemoval := removeOriginArchive
			db := setupExTransferTest(t)
			removeOriginArchive = realRemoval
			node, left := fakeArchiveNode(t, tc.files)
			require.NoError(t, db.Create(node).Error)
			require.NoError(t, db.Omit(clause.Associations).Create(&models.Server{Identifier: exTestServer, Name: exTestServer, Type: "generic", NodeID: node.ID}).Error)
			pub, _ := newExTestKey(t)
			stuck := newExTestSession(t, db, models.StatusMigrating, pub, expiredAgo(time.Minute))

			_, err := sweepExpiredExTransfers(db, time.Now())

			require.NoError(t, err)
			assert.Equal(t, models.StatusFailed, reloadExSession(t, db, stuck).Status)
			assert.Equal(t, tc.left, left())
		})
	}
}
