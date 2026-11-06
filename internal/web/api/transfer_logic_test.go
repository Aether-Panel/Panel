package api

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestServerPortsTaken_OnlyPortsHeldOnTheTargetNode(t *testing.T) {
	held := map[uint16]portHolder{25565: {reason: portBusyServer, by: "other"}, 8080: {reason: portBusyPanel}}

	server := &models.Server{Port: 25565, Ports: []uint16{25565, 25575, 8080}}
	assert.Equal(t, []uint16{8080, 25565}, serverPortsTaken(server, held), "each taken port once, in order")

	assert.Empty(t, serverPortsTaken(&models.Server{Port: 25570, Ports: []uint16{25570}}, held))
	assert.Empty(t, serverPortsTaken(&models.Server{}, held), "a server without ports")
	assert.Empty(t, serverPortsTaken(&models.Server{Port: 80, Ports: []uint16{80}}, held), "a low port it already had is not refused")
}

func TestFamilyPortsTaken_CountsTheSubServersToo(t *testing.T) {
	held := map[uint16]portHolder{25565: {reason: portBusyServer, by: "other"}, 25600: {reason: portBusyServer, by: "other"}}
	family := []*models.Server{
		{Port: 25565, Ports: []uint16{25565}},
		{Port: 25600, Ports: []uint16{25600, 25565}},
		{Port: 25601, Ports: []uint16{25601}},
	}

	assert.Equal(t, []uint16{25565, 25600}, familyPortsTaken(family, held))
}

func TestTransferServer_RefusesASubServer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	parent := "parent01"
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"nodeId":1}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("server", &models.Server{Identifier: "child01", ParentServerID: &parent})

	transferServer(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "ErrTransferSubServer")
}

// newTransferTargetNode records the node servers are moved to in the tests
func newTransferTargetNode(t *testing.T, db *gorm.DB) models.Node {
	node := models.Node{ID: 7, Name: "b", PublicHost: "10.0.0.7", PrivateHost: "10.0.0.7", PublicPort: 8080, PrivatePort: 8080, SFTPPort: 5657, Secret: "secret"}
	require.NoError(t, db.Create(&node).Error)
	return node
}

func TestSubServersToMove_LeavesOnesAlreadyOnTheTargetNode(t *testing.T) {
	db := setupExTransferTest(t)
	parent := "parent01"
	create := func(s *models.Server) {
		require.NoError(t, db.Omit(clause.Associations).Create(s).Error)
	}
	create(&models.Server{Identifier: parent, Name: parent, Type: "generic"})
	create(&models.Server{Identifier: "child01", Name: "child01", Type: "generic", ParentServerID: &parent})
	target := newTransferTargetNode(t, db)
	create(&models.Server{Identifier: "child02", Name: "child02", Type: "generic", ParentServerID: &parent, NodeID: target.ID, Node: target})
	create(&models.Server{Identifier: "other01", Name: "other01", Type: "generic"})

	moving, err := subServersToMove(db, &models.Server{Identifier: parent}, &target)

	require.NoError(t, err)
	require.Len(t, moving, 1)
	assert.Equal(t, "child01", moving[0].Identifier)
}

func TestMoveServerRecordToNode_OnlyTheNodeChanges(t *testing.T) {
	db := setupExTransferTest(t)
	require.NoError(t, db.Omit(clause.Associations).Create(&models.Server{Identifier: "srv01", Name: "before", Type: "generic"}).Error)
	target := newTransferTargetNode(t, db)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return moveServerRecordToNode(tx, "srv01", &target)
	}))

	var moved models.Server
	require.NoError(t, db.Where("identifier = ?", "srv01").First(&moved).Error)
	require.NotNil(t, moved.RawNodeID)
	assert.Equal(t, uint(7), *moved.RawNodeID)
	assert.Equal(t, "before", moved.Name)

	require.NoError(t, moveServerRecordToNode(db, "srv01", models.LocalNode))
	moved = models.Server{}
	require.NoError(t, db.Where("identifier = ?", "srv01").First(&moved).Error)
	assert.Nil(t, moved.RawNodeID, "the panel's own node has no id in the record")
}

// fakeAnswer is what a fake node answers to one call; status 0 means it cannot be reached
type fakeAnswer struct {
	status int
	body   string
	stream io.Reader
}

// fakeTransferNode is one node of a transfer. It answers from fail (by prefix of the call),
// then answers (by the exact call), and 204 otherwise; it keeps whether the server runs and
// records every call and what each upload carried.
type fakeTransferNode struct {
	running bool
	answers map[string]fakeAnswer
	fail    map[string]int
	calls   []string
	uploads map[string]string
}

func (f *fakeTransferNode) call(method, path string, body io.ReadCloser, _ http.Header) (*http.Response, error) {
	call := method + " " + path
	f.calls = append(f.calls, call)
	if body != nil {
		data, err := io.ReadAll(body)
		_ = body.Close()
		if err != nil {
			// a real client fails the request when its body breaks
			return nil, err
		}
		if f.uploads == nil {
			f.uploads = map[string]string{}
		}
		f.uploads[call] = string(data)
	}

	answer, ok := f.answerFor(method, path, call)
	if !ok {
		answer = fakeAnswer{status: http.StatusNoContent}
	}
	if answer.status == 0 {
		return nil, errors.New("node unreachable")
	}
	contents := answer.stream
	if contents == nil {
		contents = strings.NewReader(answer.body)
	}
	return &http.Response{StatusCode: answer.status, Body: io.NopCloser(contents)}, nil
}

func (f *fakeTransferNode) answerFor(method, path, call string) (fakeAnswer, bool) {
	for prefix, status := range f.fail {
		if strings.HasPrefix(call, prefix) {
			return fakeAnswer{status: status, body: "node says no"}, true
		}
	}
	if answer, ok := f.answers[call]; ok {
		return answer, true
	}
	switch {
	case method == http.MethodGet && strings.HasSuffix(path, "/status"):
		if f.running {
			return fakeAnswer{status: http.StatusOK, body: `{"running":true}`}, true
		}
		return fakeAnswer{status: http.StatusOK, body: `{"running":false}`}, true
	case strings.HasSuffix(path, "/stop?wait=true"), strings.HasSuffix(path, "/kill"):
		f.running = false
	case strings.Contains(path, "/start"):
		f.running = true
	}
	return fakeAnswer{}, false
}

// newTransferSource is the old node of server id, with a definition, an archive and four
// backups: one local, one also off-site, one only off-site and one that failed
func newTransferSource(id string, running bool) *fakeTransferNode {
	prefix := "GET /daemon/server/" + id
	answers := map[string]fakeAnswer{}
	answers[prefix+"/definition"] = fakeAnswer{status: http.StatusOK, body: `{"type":"minecraft"}`}
	answers[prefix+"/tasks"] = fakeAnswer{status: http.StatusOK, body: `{"tasks":{"restart":{"name":"Daily restart","cronSchedule":"0 5 * * *"}}}`}
	answers[prefix+"/offsite"] = fakeAnswer{status: http.StatusOK, body: `{"periodicHours":null,"periodicKeep":null,"effectiveHours":24,"effectiveKeep":4,"configured":true,"lastPeriodic":null}`}
	answers[prefix+"/file/transfer.tar.gz"] = fakeAnswer{status: http.StatusOK, body: "ARCHIVE"}
	answers[prefix+"/backup"] = fakeAnswer{status: http.StatusOK, body: `[
		{"fileName":"a.tar.gz","location":"local","modTime":"2026-03-01T12:00:00Z"},
		{"fileName":"auto-b.tar.gz","location":"both"},
		{"fileName":"c.tar.gz","location":"offsite"},
		{"fileName":"d.tar.gz","location":"failed"}]`}
	answers[prefix+"/backup/download?fileName=a.tar.gz"] = fakeAnswer{status: http.StatusOK, body: "BACKUP A"}
	answers[prefix+"/backup/download?fileName=auto-b.tar.gz"] = fakeAnswer{status: http.StatusOK, body: "BACKUP B"}
	return &fakeTransferNode{running: running, answers: answers}
}

func noKillWait(t *testing.T) {
	previous := transferKillWait
	transferKillWait = 0
	t.Cleanup(func() { transferKillWait = previous })
}

func callsWithPrefix(calls []string, prefix string) []string {
	found := make([]string, 0)
	for _, c := range calls {
		if strings.HasPrefix(c, prefix) {
			found = append(found, c)
		}
	}
	return found
}

func TestMoveServers_CopiesFilesAndBackupsBeforeTheRecordMoves(t *testing.T) {
	noKillWait(t)
	source := newTransferSource("srv01", true)
	target := &fakeTransferNode{}
	committed := false
	commit := func() error {
		committed = true
		assert.NotContains(t, source.calls, "DELETE /daemon/server/srv01", "the old copy stays until the panel points at the new one")
		return nil
	}

	err := moveServers([]*serverMove{{id: "srv01", source: source.call, target: target.call}}, commit)

	require.NoError(t, err)
	assert.True(t, committed)
	assert.Equal(t, []string{
		"PUT /daemon/server/srv01",
		"POST /daemon/server/srv01/scheduler/pause",
		"PUT /daemon/server/srv01/tasks/restart",
		"PUT /daemon/server/srv01/file/transfer.tar.gz",
		"POST /daemon/server/srv01/extract/transfer.tar.gz?destination=.",
		"DELETE /daemon/server/srv01/file/transfer.tar.gz",
		"PUT /daemon/server/srv01/backup/file?fileName=a.tar.gz&modTime=2026-03-01T12%3A00%3A00Z",
		"PUT /daemon/server/srv01/backup/file?fileName=auto-b.tar.gz&modTime=0001-01-01T00%3A00%3A00Z",
	}, target.calls, "only the backups with an archive on the old node are copied")
	assert.Equal(t, "BACKUP A", target.uploads["PUT /daemon/server/srv01/backup/file?fileName=a.tar.gz&modTime=2026-03-01T12%3A00%3A00Z"])
	assert.Contains(t, target.uploads["PUT /daemon/server/srv01/file/transfer.tar.gz"], "ARCHIVE")
	assert.Equal(t, `{"type":"minecraft"}`, target.uploads["PUT /daemon/server/srv01"])
	assert.Equal(t, `{"name":"Daily restart","cronSchedule":"0 5 * * *"}`, target.uploads["PUT /daemon/server/srv01/tasks/restart"], "its tasks go with it")
	assert.Equal(t, "POST /daemon/server/srv01/scheduler/pause", source.calls[0], "no task runs on the old node while it moves")

	assert.Equal(t, "DELETE /daemon/server/srv01", source.calls[len(source.calls)-1], "removed from the old node with its backups, last")
	assert.Empty(t, callsWithPrefix(source.calls, "POST /daemon/server/srv01/start?wait=true"))
}

func TestServerMove_LeavesPeriodicCopiesToTheBucket(t *testing.T) {
	noKillWait(t)
	source := newTransferSource("srv01", false)
	// a periodic copy the old node is still uploading: its archive is there until the upload ends
	source.answers["GET /daemon/server/srv01/backup"] = fakeAnswer{status: http.StatusOK, body: `[
		{"fileName":"a.tar.gz","location":"local","modTime":"2026-03-01T12:00:00Z"},
		{"fileName":"` + servers.OffsiteBackupPrefix + `x.tar.gz","location":"local"},
		{"fileName":"` + servers.OffsiteBackupPrefix + `y.tar.gz","location":"both"}]`}
	target := &fakeTransferNode{}

	err := moveServers([]*serverMove{{id: "srv01", source: source.call, target: target.call}}, func() error { return nil })

	require.NoError(t, err)
	assert.Equal(t, []string{"PUT /daemon/server/srv01/backup/file?fileName=a.tar.gz&modTime=2026-03-01T12%3A00%3A00Z"},
		callsWithPrefix(target.calls, "PUT /daemon/server/srv01/backup/file"),
		"a periodic copy belongs in the bucket, which both nodes share, and could vanish mid-copy")
	assert.Empty(t, callsWithPrefix(source.calls, "GET /daemon/server/srv01/backup/download?fileName="+servers.OffsiteBackupPrefix))
}

func TestServerMove_CopiesOffsiteSettings(t *testing.T) {
	noKillWait(t)
	source := newTransferSource("srv01", false)
	source.answers["GET /daemon/server/srv01/offsite"] = fakeAnswer{status: http.StatusOK, body: `{"periodicHours":168,"periodicKeep":null,"effectiveHours":168,"effectiveKeep":4,"configured":true,"lastPeriodic":"2026-11-01T03:00:00Z"}`}
	target := &fakeTransferNode{}

	err := moveServers([]*serverMove{{id: "srv01", source: source.call, target: target.call}}, func() error { return nil })

	require.NoError(t, err)
	assert.Equal(t, []string{
		"PUT /daemon/server/srv01",
		"POST /daemon/server/srv01/scheduler/pause",
		"PUT /daemon/server/srv01/tasks/restart",
		"PUT /daemon/server/srv01/offsite",
	}, target.calls[:4], "its periodic off-site setting goes with its tasks")
	assert.JSONEq(t, `{"periodicHours":168,"periodicKeep":null}`, target.uploads["PUT /daemon/server/srv01/offsite"], "only what the server set, not what the old node reports")
}

func TestServerMove_AnOldNodeWithoutOffsiteSettingsStillMoves(t *testing.T) {
	noKillWait(t)
	source := newTransferSource("srv01", false)
	source.answers["GET /daemon/server/srv01/offsite"] = fakeAnswer{status: http.StatusNotFound, body: "404 page not found"}
	target := &fakeTransferNode{}
	committed := false

	err := moveServers([]*serverMove{{id: "srv01", source: source.call, target: target.call}}, func() error {
		committed = true
		return nil
	})

	require.NoError(t, err)
	assert.True(t, committed)
	assert.Contains(t, source.calls, "GET /daemon/server/srv01/offsite")
	assert.Empty(t, callsWithPrefix(target.calls, "PUT /daemon/server/srv01/offsite"), "nothing to copy")
}

func TestServerMove_NothingSetLeavesTheNewNodeOnItsDefaults(t *testing.T) {
	noKillWait(t)
	source := newTransferSource("srv01", false)
	target := &fakeTransferNode{}

	err := moveServers([]*serverMove{{id: "srv01", source: source.call, target: target.call}}, func() error { return nil })

	require.NoError(t, err)
	assert.Contains(t, source.calls, "GET /daemon/server/srv01/offsite")
	assert.Empty(t, callsWithPrefix(target.calls, "PUT /daemon/server/srv01/offsite"))
}

func TestServerMove_AnOffsiteSettingThatCannotBeReadStopsTheMove(t *testing.T) {
	noKillWait(t)
	source := newTransferSource("srv01", false)
	source.answers["GET /daemon/server/srv01/offsite"] = fakeAnswer{status: http.StatusInternalServerError, body: "disk error"}
	target := &fakeTransferNode{}

	err := moveServers([]*serverMove{{id: "srv01", source: source.call, target: target.call}}, func() error {
		t.Fatal("the panel must not change")
		return nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "off-site")
	assert.Contains(t, target.calls, "DELETE /daemon/server/srv01?keepBackups=true", "the copy begun on the new node goes")
}

func TestMoveServers_AFailedBackupCopyUndoesTheTransfer(t *testing.T) {
	noKillWait(t)
	source := newTransferSource("srv01", true)
	target := &fakeTransferNode{fail: map[string]int{"PUT /daemon/server/srv01/backup/file?fileName=auto-b.tar.gz": http.StatusInsufficientStorage}}
	committed := false

	err := moveServers([]*serverMove{{id: "srv01", source: source.call, target: target.call}}, func() error {
		committed = true
		return nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "auto-b.tar.gz")
	assert.False(t, committed, "the panel keeps pointing at the old node")
	assert.Equal(t, []string{
		"DELETE /daemon/server/srv01/backup/file?fileName=a.tar.gz",
		"DELETE /daemon/server/srv01/backup/file?fileName=auto-b.tar.gz",
		"DELETE /daemon/server/srv01?keepBackups=true",
	}, target.calls[len(target.calls)-3:], "the copies it made or began go, then the server")
	assert.Equal(t, "POST /daemon/server/srv01/scheduler/resume", source.calls[len(source.calls)-2], "its tasks run again where it is")
	assert.Equal(t, "POST /daemon/server/srv01/start?wait=true", source.calls[len(source.calls)-1], "it runs again where it was")
	assert.True(t, source.running)
	assert.NotContains(t, source.calls, "DELETE /daemon/server/srv01")
	assert.Contains(t, source.calls, "DELETE /daemon/server/srv01/file/transfer.tar.gz", "the archive does not stay in its files")
}

func TestMoveServers_AFailedArchiveRestartsTheServer(t *testing.T) {
	noKillWait(t)
	source := newTransferSource("srv01", true)
	source.fail = map[string]int{"POST /daemon/server/srv01/archive/": http.StatusInsufficientStorage}
	target := &fakeTransferNode{}

	err := moveServers([]*serverMove{{id: "srv01", source: source.call, target: target.call}}, func() error {
		t.Fatal("the panel must not change")
		return nil
	})

	require.Error(t, err)
	assert.Equal(t, []string{
		"PUT /daemon/server/srv01",
		"POST /daemon/server/srv01/scheduler/pause",
		"PUT /daemon/server/srv01/tasks/restart",
		"DELETE /daemon/server/srv01?keepBackups=true",
	}, target.calls)
	assert.Equal(t, "POST /daemon/server/srv01/start?wait=true", source.calls[len(source.calls)-1])
}

func TestMoveServers_AStoppedServerStaysStopped(t *testing.T) {
	source := newTransferSource("srv01", false)
	target := &fakeTransferNode{fail: map[string]int{"POST /daemon/server/srv01/extract/": http.StatusInternalServerError}}

	err := moveServers([]*serverMove{{id: "srv01", source: source.call, target: target.call}}, func() error { return nil })

	require.Error(t, err)
	assert.Contains(t, target.calls, "DELETE /daemon/server/srv01?keepBackups=true")
	assert.Empty(t, callsWithPrefix(source.calls, "POST /daemon/server/srv01/start?wait=true"), "it was not running")
	assert.Empty(t, callsWithPrefix(source.calls, "POST /daemon/server/srv01/stop"))
}

func TestMoveServers_NothingToUndoWhenTheDefinitionCannotBeRead(t *testing.T) {
	source := newTransferSource("srv01", true)
	source.fail = map[string]int{"GET /daemon/server/srv01/definition": 0}
	target := &fakeTransferNode{}

	err := moveServers([]*serverMove{{id: "srv01", source: source.call, target: target.call}}, func() error { return nil })

	require.Error(t, err)
	assert.Empty(t, target.calls)
	assert.Equal(t, []string{
		"POST /daemon/server/srv01/scheduler/pause",
		"GET /daemon/server/srv01/definition",
		"POST /daemon/server/srv01/scheduler/resume",
	}, source.calls, "it was never stopped, and its tasks run again")
	assert.True(t, source.running)
}

func TestMoveServers_ARefusedCreateLeavesNothingToRemove(t *testing.T) {
	source := newTransferSource("srv01", true)
	target := &fakeTransferNode{fail: map[string]int{"PUT /daemon/server/srv01": http.StatusBadRequest}}

	err := moveServers([]*serverMove{{id: "srv01", source: source.call, target: target.call}}, func() error { return nil })

	require.Error(t, err)
	assert.Equal(t, []string{"PUT /daemon/server/srv01"}, target.calls, "the node removed what it began itself")
	assert.True(t, source.running)
}

func TestMoveServers_AnUnreachableCreateIsRemoved(t *testing.T) {
	source := newTransferSource("srv01", false)
	target := &fakeTransferNode{answers: map[string]fakeAnswer{"PUT /daemon/server/srv01": {status: 0}}}

	err := moveServers([]*serverMove{{id: "srv01", source: source.call, target: target.call}}, func() error { return nil })

	require.Error(t, err)
	assert.Equal(t, []string{"PUT /daemon/server/srv01", "DELETE /daemon/server/srv01?keepBackups=true"}, target.calls,
		"a node that could not be heard may have created it")
}

func TestMoveServers_AFailedCommitUndoesEverything(t *testing.T) {
	noKillWait(t)
	source := newTransferSource("srv01", true)
	target := &fakeTransferNode{}

	err := moveServers([]*serverMove{{id: "srv01", source: source.call, target: target.call}}, func() error {
		return errors.New("database is gone")
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "database is gone")
	assert.Contains(t, target.calls, "DELETE /daemon/server/srv01/backup/file?fileName=a.tar.gz")
	assert.Contains(t, target.calls, "DELETE /daemon/server/srv01/backup/file?fileName=auto-b.tar.gz")
	assert.Contains(t, target.calls, "DELETE /daemon/server/srv01?keepBackups=true")
	assert.NotContains(t, source.calls, "DELETE /daemon/server/srv01")
	assert.True(t, source.running)
}

func TestMoveServers_ABackupAlreadyOnTheNewNodeIsNotTakenBack(t *testing.T) {
	noKillWait(t)
	source := newTransferSource("srv01", false)
	target := &fakeTransferNode{fail: map[string]int{"PUT /daemon/server/srv01/backup/file?fileName=a.tar.gz": http.StatusConflict}}

	err := moveServers([]*serverMove{{id: "srv01", source: source.call, target: target.call}}, func() error {
		return errors.New("database is gone")
	})

	require.Error(t, err)
	assert.NotContains(t, target.calls, "DELETE /daemon/server/srv01/backup/file?fileName=a.tar.gz", "it was there before the transfer")
	assert.Contains(t, target.calls, "DELETE /daemon/server/srv01/backup/file?fileName=auto-b.tar.gz")
}

func TestMoveServers_ASubServerThatFailsUndoesItsParent(t *testing.T) {
	noKillWait(t)
	parentSource := newTransferSource("srv01", true)
	childSource := newTransferSource("sub01", true)
	childSource.fail = map[string]int{"POST /daemon/server/sub01/archive/": http.StatusInsufficientStorage}
	target := &fakeTransferNode{}

	err := moveServers([]*serverMove{
		{id: "srv01", source: parentSource.call, target: target.call},
		{id: "sub01", source: childSource.call, target: target.call},
	}, func() error {
		t.Fatal("the panel must not change")
		return nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "sub01")
	assert.Contains(t, target.calls, "DELETE /daemon/server/sub01?keepBackups=true")
	assert.Contains(t, target.calls, "DELETE /daemon/server/srv01?keepBackups=true")
	assert.Contains(t, target.calls, "DELETE /daemon/server/srv01/backup/file?fileName=a.tar.gz")
	assert.True(t, parentSource.running)
	assert.True(t, childSource.running)
	assert.NotContains(t, parentSource.calls, "DELETE /daemon/server/srv01")
}

// brokenReader gives part of an archive, then fails
type brokenReader struct{ sent bool }

func (b *brokenReader) Read(p []byte) (int, error) {
	if b.sent {
		return 0, errors.New("connection lost")
	}
	b.sent = true
	return copy(p, "ARCH"), nil
}

func TestStreamArchive_ACopyCutShortFailsTheUpload(t *testing.T) {
	source := newTransferSource("srv01", false)
	source.answers["GET /daemon/server/srv01/file/transfer.tar.gz"] = fakeAnswer{status: http.StatusOK, stream: &brokenReader{}}
	target := &fakeTransferNode{}
	m := &serverMove{id: "srv01", source: source.call, target: target.call}

	err := m.streamArchive()

	require.Error(t, err, "half an archive is never uploaded as if it were whole")
	assert.Contains(t, err.Error(), "connection lost")
}

func TestResumeMovedServers_StartsOnlyThoseThatWereRunning(t *testing.T) {
	parent := &fakeTransferNode{}
	child := &fakeTransferNode{fail: map[string]int{"POST /daemon/server/sub01/start": http.StatusConflict}}
	stopped := &fakeTransferNode{}
	moves := []*serverMove{
		{id: "srv01", target: parent.call, wasRunning: true},
		{id: "sub01", target: child.call, wasRunning: true},
		{id: "sub02", target: stopped.call},
	}

	failed := resumeMovedServers(moves)

	assert.Equal(t, []string{
		"POST /daemon/server/srv01/scheduler/resume",
		"GET /daemon/server/srv01/status",
		"POST /daemon/server/srv01/start?wait=true",
	}, parent.calls)
	assert.Equal(t, []string{"POST /daemon/server/sub02/scheduler/resume"}, stopped.calls, "its tasks come back, but it stays stopped")
	require.Len(t, failed, 1, "a server that does not start is reported")
	assert.Contains(t, failed["sub01"].Error(), "node says no")
}

func TestResumeMovedServers_LeavesARunningServerAlone(t *testing.T) {
	target := &fakeTransferNode{running: true}

	assert.Empty(t, resumeMovedServers([]*serverMove{{id: "srv01", target: target.call, wasRunning: true}}))
	assert.Empty(t, callsWithPrefix(target.calls, "POST /daemon/server/srv01/start"), "started before the panel restarted")
}

func TestMoveServers_ARunningServerIsStartedOnTheNewNodeAfterwards(t *testing.T) {
	noKillWait(t)
	source := newTransferSource("srv01", true)
	target := &fakeTransferNode{}
	moves := []*serverMove{{id: "srv01", source: source.call, target: target.call}}

	require.NoError(t, moveServers(moves, func() error { return nil }))
	assert.Empty(t, resumeMovedServers(moves))

	assert.Equal(t, "POST /daemon/server/srv01/start?wait=true", target.calls[len(target.calls)-1])
	assert.True(t, target.running)
	assert.Empty(t, callsWithPrefix(source.calls, "POST /daemon/server/srv01/start"), "not on the old node")
}

func TestMoveServers_ABackupStartedMeanwhileStopsTheTransfer(t *testing.T) {
	noKillWait(t)
	source := newTransferSource("srv01", true)
	source.answers["GET /daemon/server/srv01/backup/busy"] = fakeAnswer{status: http.StatusOK, body: `{"busy":true}`}
	target := &fakeTransferNode{}

	err := moveServers([]*serverMove{{id: "srv01", source: source.call, target: target.call}}, func() error {
		t.Fatal("the panel must not change")
		return nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "backup")
	assert.Empty(t, callsWithPrefix(source.calls, "POST /daemon/server/srv01/archive/"), "nothing is archived under a backup")
	assert.Contains(t, target.calls, "DELETE /daemon/server/srv01?keepBackups=true")
	assert.True(t, source.running, "it runs again where it was")
}

func TestFirstWithBackupRunning_OnlyWhenTheNodeSaysSo(t *testing.T) {
	idle := &fakeTransferNode{answers: map[string]fakeAnswer{"GET /daemon/server/srv01/backup/busy": {status: http.StatusOK, body: `{"busy":false}`}}}
	busy := &fakeTransferNode{answers: map[string]fakeAnswer{"GET /daemon/server/sub01/backup/busy": {status: http.StatusOK, body: `{"busy":true}`}}}
	unknown := &fakeTransferNode{answers: map[string]fakeAnswer{"GET /daemon/server/sub02/backup/busy": {status: http.StatusNotFound}}}
	nodes := map[string]*fakeTransferNode{"srv01": idle, "sub01": busy, "sub02": unknown}
	callerFor := func(s *models.Server) nodeCaller { return nodes[s.Identifier].call }

	family := []*models.Server{{Identifier: "srv01"}, {Identifier: "sub02"}, {Identifier: "sub01"}}
	assert.Equal(t, "sub01", firstWithBackupRunning(family, callerFor))
	assert.Empty(t, firstWithBackupRunning(family[:2], callerFor), "a node that cannot tell is not taken as busy")
}

func TestClaimTransfer_AllOrNone(t *testing.T) {
	t.Cleanup(func() { releaseTransfer([]string{"srv01", "sub01", "sub02"}) })

	claim := func(ids ...string) bool {
		claimed, err := claimTransfer(nil, ids)
		require.NoError(t, err)
		return claimed
	}
	require.True(t, claim("srv01", "sub01"))
	assert.False(t, claim("sub02", "sub01"), "one of them is already being moved")
	assert.True(t, claim("sub02"), "a refused claim takes nothing")

	releaseTransfer([]string{"srv01", "sub01"})
	assert.True(t, claim("srv01"))
}

func TestTransferServer_RefusesASecondTransfer(t *testing.T) {
	db := setupExTransferTest(t)
	require.NoError(t, db.Omit(clause.Associations).Create(&models.Server{Identifier: "srv01", Name: "srv01", Type: "generic"}).Error)
	newTransferTargetNode(t, db)
	claimed, err := claimTransfer(nil, []string{"srv01"})
	require.NoError(t, err)
	require.True(t, claimed)
	t.Cleanup(func() { releaseTransfer([]string{"srv01"}) })

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"nodeId":7}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("db", db)
	c.Set("server", &models.Server{Identifier: "srv01", Node: *models.LocalNode})

	transferServer(c)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "ErrTransferRunning")
	claimed, err = claimTransfer(nil, []string{"srv01"})
	require.NoError(t, err)
	assert.False(t, claimed, "the refused request leaves the running transfer's claim alone")
}

func TestRecordTransferActivity_UnderWhoAskedForIt(t *testing.T) {
	db := setupExTransferTest(t)
	require.NoError(t, db.AutoMigrate(&models.ServerActivity{}))
	require.NoError(t, db.Omit(clause.Associations).Create(&models.Server{Identifier: "srv01", Name: "srv01", Type: "generic"}).Error)

	recordTransferActivity(db, "srv01", activityTransferFailed, "server srv01, uploading the archive: node unreachable", &models.User{ID: 4, Username: "alice"})
	recordTransferActivity(db, "srv01", activityTransferStartFailed, "starting it on the new node: port in use", nil)

	var entries []models.ServerActivity
	require.NoError(t, db.Order("id").Find(&entries).Error)
	require.Len(t, entries, 2)
	assert.Equal(t, "server.transferFailed", entries[0].Action)
	assert.Equal(t, "server srv01, uploading the archive: node unreachable", entries[0].Detail)
	assert.Equal(t, "alice", entries[0].Username)
	require.NotNil(t, entries[0].UserID)
	assert.Equal(t, uint(4), *entries[0].UserID)
	assert.Equal(t, "server.transferStartFailed", entries[1].Action)
	assert.Nil(t, entries[1].UserID)
}

// setupNodeTransferTest is a database with the tables a recorded transfer uses
func setupNodeTransferTest(t *testing.T) *gorm.DB {
	db := setupExTransferTest(t)
	require.NoError(t, db.AutoMigrate(&models.NodeTransfer{}, &models.ServerActivity{}))
	for _, id := range []string{"srv01", "sub01"} {
		require.NoError(t, db.Omit(clause.Associations).Create(&models.Server{Identifier: id, Name: id, Type: "generic"}).Error)
	}
	return db
}

func reloadNodeTransfer(t *testing.T, db *gorm.DB, id uint) models.NodeTransfer {
	var record models.NodeTransfer
	require.NoError(t, db.First(&record, id).Error)
	return record
}

func TestTransferRun_SavesHowFarEachServerGot(t *testing.T) {
	noKillWait(t)
	db := setupNodeTransferTest(t)
	family := []*models.Server{{Identifier: "srv01", NodeID: 3}}
	record, err := recordNodeTransfer(db, family, &models.Node{ID: 7}, &models.User{ID: 4})
	require.NoError(t, err)

	source := newTransferSource("srv01", true)
	target := &fakeTransferNode{}
	run := &transferRun{db: db, record: record}
	m := &serverMove{id: "srv01", sourceNodeID: 3, source: source.call, target: target.call, persist: run.save}
	run.moves = []*serverMove{m}

	err = moveServers(run.moves, func() error {
		saved := reloadNodeTransfer(t, db, record.ID)
		require.Len(t, saved.Moves, 1)
		assert.Equal(t, models.NodeTransferMove{
			ServerID:      "srv01",
			SourceNodeID:  3,
			Step:          stepCopied,
			WasRunning:    true,
			CopiedBackups: []string{"a.tar.gz", "auto-b.tar.gz"},
		}, saved.Moves[0], "what a panel that restarts now needs to undo it")
		assert.False(t, saved.Committed)
		require.NotNil(t, saved.UserID)
		assert.Equal(t, uint(4), *saved.UserID)
		return errors.New("database is gone")
	})

	require.Error(t, err)
}

func TestClaimTransfer_ARecordedTransferHoldsItsServers(t *testing.T) {
	db := setupNodeTransferTest(t)
	_, err := recordNodeTransfer(db, []*models.Server{{Identifier: "srv01"}, {Identifier: "sub01"}}, &models.Node{ID: 7}, nil)
	require.NoError(t, err)

	claimed, err := claimTransfer(db, []string{"sub01"})
	require.NoError(t, err)
	assert.False(t, claimed, "a transfer from before a restart still holds its sub-server")

	claimed, err = claimTransfer(db, []string{"other01"})
	require.NoError(t, err)
	assert.True(t, claimed)
	releaseTransfer([]string{"other01"})
}

// recoveryNodes answers for nodes 3 (the old one) and 7 (the new one)
func recoveryNodes(source, target *fakeTransferNode) func(uint) (nodeCaller, error) {
	return func(id uint) (nodeCaller, error) {
		switch id {
		case 3:
			return source.call, nil
		case 7:
			return target.call, nil
		}
		return nil, errors.New("no such node")
	}
}

func TestRecoverNodeTransfers_UndoesOneNotCommitted(t *testing.T) {
	db := setupNodeTransferTest(t)
	user := &models.User{Username: "alice", Email: "alice@example.com", HashedPassword: "x"}
	require.NoError(t, db.Create(user).Error)
	record := &models.NodeTransfer{ServerID: "srv01", TargetNodeID: 7, UserID: &user.ID, Moves: []models.NodeTransferMove{
		{ServerID: "srv01", SourceNodeID: 3, Step: stepCopyingBackups, WasRunning: true, CopiedBackups: []string{"a.tar.gz"}},
		{ServerID: "sub01", SourceNodeID: 3, Step: stepWaiting},
	}}
	require.NoError(t, db.Create(record).Error)
	source := &fakeTransferNode{}
	target := &fakeTransferNode{}

	recoverNodeTransfers(db, recoveryNodes(source, target))

	assert.Equal(t, []string{
		"DELETE /daemon/server/sub01?keepBackups=true",
		"DELETE /daemon/server/srv01/backup/file?fileName=a.tar.gz",
		"DELETE /daemon/server/srv01?keepBackups=true",
	}, target.calls, "what it may have left on the new node goes")
	assert.Equal(t, "POST /daemon/server/srv01/scheduler/resume", source.calls[len(source.calls)-2])
	assert.Equal(t, "POST /daemon/server/srv01/start?wait=true", source.calls[len(source.calls)-1], "it runs again where it was")
	assert.Contains(t, source.calls, "POST /daemon/server/sub01/scheduler/resume")
	assert.Empty(t, callsWithPrefix(source.calls, "POST /daemon/server/sub01/start"), "it was not running")
	assert.Empty(t, callsWithPrefix(source.calls, "DELETE /daemon/server/srv01?"), "the old node keeps it")

	var count int64
	require.NoError(t, db.Model(&models.NodeTransfer{}).Count(&count).Error)
	assert.Zero(t, count, "the transfer has ended")

	var entries []models.ServerActivity
	require.NoError(t, db.Where("server_id = ?", "srv01").Find(&entries).Error)
	require.Len(t, entries, 1)
	assert.Equal(t, activityTransferFailed, entries[0].Action)
	assert.Contains(t, entries[0].Detail, "panel restarted")
	assert.Contains(t, entries[0].Detail, stepCopyingBackups)
	assert.Equal(t, "alice", entries[0].Username)
}

func TestRecoverNodeTransfers_FinishesOneCommitted(t *testing.T) {
	db := setupNodeTransferTest(t)
	record := &models.NodeTransfer{ServerID: "srv01", TargetNodeID: 7, Committed: true, Moves: []models.NodeTransferMove{
		{ServerID: "srv01", SourceNodeID: 3, Step: stepCopied, WasRunning: true},
		{ServerID: "sub01", SourceNodeID: 3, Step: stepCopied},
	}}
	require.NoError(t, db.Create(record).Error)
	source := &fakeTransferNode{}
	target := &fakeTransferNode{}

	recoverNodeTransfers(db, recoveryNodes(source, target))

	assert.Equal(t, []string{"DELETE /daemon/server/srv01", "DELETE /daemon/server/sub01"}, source.calls, "only the old copies go")
	assert.Equal(t, []string{
		"POST /daemon/server/srv01/scheduler/resume",
		"GET /daemon/server/srv01/status",
		"POST /daemon/server/srv01/start?wait=true",
		"POST /daemon/server/sub01/scheduler/resume",
	}, target.calls, "the tasks come back on the new node, and what ran starts there")

	var count int64
	require.NoError(t, db.Model(&models.NodeTransfer{}).Count(&count).Error)
	assert.Zero(t, count)
	require.NoError(t, db.Model(&models.ServerActivity{}).Count(&count).Error)
	assert.Zero(t, count, "nothing failed")
}

func TestRecoverNodeTransfers_ANodeThatIsGoneStillEndsTheRecord(t *testing.T) {
	db := setupNodeTransferTest(t)
	record := &models.NodeTransfer{ServerID: "srv01", TargetNodeID: 9, Moves: []models.NodeTransferMove{{ServerID: "srv01", SourceNodeID: 3, Step: stepCreating}}}
	require.NoError(t, db.Create(record).Error)
	source := &fakeTransferNode{}

	recoverNodeTransfers(db, recoveryNodes(source, &fakeTransferNode{}))

	assert.Contains(t, source.calls, "POST /daemon/server/srv01/scheduler/resume", "the old node is still reached")
	var count int64
	require.NoError(t, db.Model(&models.NodeTransfer{}).Count(&count).Error)
	assert.Zero(t, count)
}
