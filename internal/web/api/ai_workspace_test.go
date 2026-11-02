package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/security"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeWorkspace_WorksOnTheServersFiles(t *testing.T) {
	d := newFakeDaemon(t, "srv1")
	folder := d.folders["srv1"]
	ws := d.workspaceFor("srv1")
	ctx := context.Background()
	writeServerFile(t, folder, "server.properties", "motd=hello\nmax-players=20\n")
	writeServerFile(t, folder, "config/paper.yml", "view-distance: 10\n")

	// create, and create again: nothing is replaced
	require.NoError(t, ws.Write(ctx, "notes/new file #1?.txt", []byte("first"), false))
	err := ws.Write(ctx, "notes/new file #1?.txt", []byte("second"), false)
	assert.ErrorIs(t, err, workspace.ErrExists)
	data, err := ws.Read(ctx, "notes/new file #1?.txt", 0)
	require.NoError(t, err)
	assert.Equal(t, "first", string(data), "the name keeps its # and ?")

	// overwrite
	require.NoError(t, ws.Write(ctx, "notes/new file #1?.txt", []byte("second"), true))
	data, err = ws.Read(ctx, "notes/new file #1?.txt", 0)
	require.NoError(t, err)
	assert.Equal(t, "second", string(data))

	// list: the root and a folder, without the file manager's ".."
	entries, err := ws.List(ctx, "")
	require.NoError(t, err)
	names := map[string]workspace.Entry{}
	for _, e := range entries {
		names[e.Name] = e
	}
	assert.True(t, names["config"].IsDir)
	assert.Equal(t, "server.properties", names["server.properties"].Path)
	assert.EqualValues(t, 26, names["server.properties"].Size)
	entries, err = ws.List(ctx, "config")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, workspace.Entry{Name: "paper.yml", Path: "config/paper.yml", Size: 18, ModTime: entries[0].ModTime}, entries[0])
	_, err = ws.List(ctx, "server.properties")
	assert.ErrorIs(t, err, workspace.ErrNotDir)
	_, err = ws.Read(ctx, "config", 0)
	assert.ErrorIs(t, err, workspace.ErrIsDir)

	// stat
	e, err := ws.Stat(ctx, "config/paper.yml")
	require.NoError(t, err)
	assert.EqualValues(t, 18, e.Size)
	_, err = ws.Stat(ctx, "missing.txt")
	assert.ErrorIs(t, err, workspace.ErrNotFound)

	// folders
	require.NoError(t, ws.Mkdir(ctx, "world/region"))
	assert.DirExists(t, filepath.Join(folder, "world", "region"))
	require.NoError(t, ws.Mkdir(ctx, "world"), "an existing folder is fine")
	assert.ErrorIs(t, ws.Mkdir(ctx, "server.properties"), workspace.ErrExists, "a file is in the way")

	// copy and move
	require.NoError(t, ws.Copy(ctx, "config", "backup/config"))
	assert.FileExists(t, filepath.Join(folder, "backup", "config", "paper.yml"))
	assert.ErrorIs(t, ws.Copy(ctx, "config", "backup/config"), workspace.ErrExists)
	require.NoError(t, ws.Move(ctx, "backup/config/paper.yml", "old/deep/paper.yml"))
	assert.FileExists(t, filepath.Join(folder, "old", "deep", "paper.yml"), "missing parents are made")
	assert.ErrorIs(t, ws.Move(ctx, "server.properties", "config/paper.yml"), workspace.ErrExists)
	assert.ErrorIs(t, ws.Move(ctx, "missing.txt", "x.txt"), workspace.ErrNotFound)

	// search
	matches, err := ws.Search(ctx, "", workspace.SearchOptions{Query: "max-players", InContent: true})
	require.NoError(t, err)
	assert.Equal(t, []workspace.Match{{Path: "server.properties", Line: 2, Text: "max-players=20"}}, matches)
	matches, err = ws.Search(ctx, "", workspace.SearchOptions{Query: "paper", MaxResults: 1})
	require.NoError(t, err)
	assert.Len(t, matches, 1, "a smaller limit is applied on top of the node's")

	// delete
	require.NoError(t, ws.Delete(ctx, "old"))
	assert.NoDirExists(t, filepath.Join(folder, "old"))
	assert.ErrorIs(t, ws.Delete(ctx, "old"), workspace.ErrNotFound)

	// every call went to this server's routes
	for _, call := range d.callLog() {
		assert.True(t, strings.Contains(call, " /daemon/server/srv1/"), call)
	}
}

func TestNodeWorkspace_RefusesBadPathsWithoutCallingTheNode(t *testing.T) {
	d := newFakeDaemon(t, "srv1", "srv2")
	writeServerFile(t, d.folders["srv2"], "secret.txt", "srv2's")
	ws := d.workspaceFor("srv1")
	ctx := context.Background()

	for _, p := range []string{"../srv2/secret.txt", "/etc/passwd", `..\secret.txt`, "a/../../secret.txt", "C:/x"} {
		_, err := ws.Read(ctx, p, 0)
		assert.ErrorIs(t, err, security.ErrOutsideRoot, p)
		assert.ErrorIs(t, ws.Delete(ctx, p), security.ErrOutsideRoot, p)
		assert.ErrorIs(t, ws.Copy(ctx, p, "copy.txt"), security.ErrOutsideRoot, p)
		assert.ErrorIs(t, ws.Move(ctx, "x", p), security.ErrOutsideRoot, p)
	}
	_, err := ws.Read(ctx, "", 0)
	assert.ErrorIs(t, err, security.ErrInvalidPath, "a file is needed, not the server folder")
	assert.Empty(t, d.callLog(), "nothing reached the node")
	assert.FileExists(t, filepath.Join(d.folders["srv2"], "secret.txt"))
}

// A link inside the server folder that points out of it is refused by the node's confined
// file server, through the nodeWorkspace as through the workspace itself
func TestNodeWorkspace_LinksOutOfTheServerAreRefused(t *testing.T) {
	d := newFakeDaemon(t, "srv1", "srv2")
	writeServerFile(t, d.folders["srv2"], "secret.txt", "srv2's")
	require.NoError(t, os.Symlink(d.folders["srv2"], filepath.Join(d.folders["srv1"], "other")))
	ws := d.workspaceFor("srv1")
	ctx := context.Background()

	_, err := ws.Stat(ctx, "other/secret.txt")
	assert.Error(t, err)
	assert.ErrorIs(t, ws.Copy(ctx, "other", "mine"), workspace.ErrSymlink)
	matches, err := ws.Search(ctx, "", workspace.SearchOptions{Query: "srv2", InContent: true})
	require.NoError(t, err)
	assert.Empty(t, matches)
	require.NoError(t, ws.Delete(ctx, "other"), "the link itself can go")
	assert.FileExists(t, filepath.Join(d.folders["srv2"], "secret.txt"))
}

// endless is a file body that never ends and counts what was read of it
type endless struct{ read int }

func (e *endless) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'a'
	}
	e.read += len(p)
	return len(p), nil
}

func (e *endless) Close() error { return nil }

func TestNodeWorkspace_ReadStopsAtTheLimit(t *testing.T) {
	body := &endless{}
	ws := &nodeWorkspace{prefix: daemonServerPrefix + "srv1", node: "fake", call: func(context.Context, string, string, io.Reader, http.Header) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, ContentLength: -1, Header: http.Header{"Content-Type": {"application/octet-stream"}}, Body: body}, nil
	}}

	_, err := ws.Read(context.Background(), "huge.log", 1024)
	assert.ErrorIs(t, err, workspace.ErrTooLarge)
	assert.LessOrEqual(t, body.read, 1025, "only the limit and one byte are read")

	announced := &nodeWorkspace{prefix: daemonServerPrefix + "srv1", node: "fake", call: func(context.Context, string, string, io.Reader, http.Header) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, ContentLength: 1 << 30, Header: http.Header{"Content-Type": {"application/octet-stream"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	}}
	_, err = announced.Read(context.Background(), "huge.log", 1024)
	assert.ErrorIs(t, err, workspace.ErrTooLarge, "a size the node announces is checked first")
}

func TestNodeWorkspace_NodeFailuresAreSafe(t *testing.T) {
	answer := func(status int, header http.Header, body string) *nodeWorkspace {
		return &nodeWorkspace{prefix: daemonServerPrefix + "srv1", node: "fake", call: func(context.Context, string, string, io.Reader, http.Header) (*http.Response, error) {
			return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body))}, nil
		}}
	}
	ctx := context.Background()

	// the node refuses the panel's token: a safe error, not a 401 for the browser
	err := answer(http.StatusUnauthorized, http.Header{}, "").Delete(ctx, "x.txt")
	assert.ErrorIs(t, err, workspace.ErrFailed)
	assert.Contains(t, err.Error(), "credentials")

	// the file manager's routes: read by status, their own text is not passed on
	err = answer(http.StatusInternalServerError, http.Header{}, `{"error":{"msg":"open /var/lib/panel/servers/srv1/x: boom","code":"ErrGeneric"}}`).Delete(ctx, "x.txt")
	assert.ErrorIs(t, err, workspace.ErrFailed)
	assert.NotContains(t, err.Error(), "/var/lib")
	err = answer(http.StatusBadRequest, http.Header{}, `{"error":{"msg":"invalid file access","code":"ErrIllegalFileAccess"}}`).Delete(ctx, "x.txt")
	assert.ErrorIs(t, err, workspace.ErrOutsideRoot)
	err = answer(http.StatusConflict, http.Header{}, `{"error":{"msg":"the server has used all of its disk space","code":"ErrDiskLimitExceeded"}}`).Write(ctx, "x.txt", []byte("x"), true)
	assert.ErrorIs(t, err, workspace.ErrNoSpace)
	err = answer(http.StatusNotFound, http.Header{}, "").Delete(ctx, "x.txt")
	assert.ErrorIs(t, err, workspace.ErrNotFound)

	// the node cannot be reached: its address stays in the log
	unreachable := &nodeWorkspace{prefix: daemonServerPrefix + "srv1", node: "fake", call: func(context.Context, string, string, io.Reader, http.Header) (*http.Response, error) {
		return nil, errors.New("dial tcp 10.0.0.7:8080: connection refused")
	}}
	_, err = unreachable.Stat(ctx, "x.txt")
	assert.ErrorIs(t, err, workspace.ErrFailed)
	assert.NotContains(t, err.Error(), "10.0.0.7")
}

// The workspace the panel builds goes through CallNodeContext, with the panel's token, to the
// route's server on its node; a node that refuses the token gives a safe error
func TestNewNodeWorkspace_CallsTheServersNode(t *testing.T) {
	d := newFakeDaemon(t, "srv1")
	writeServerFile(t, d.folders["srv1"], "eula.txt", "eula=true\n")
	server := &models.Server{Identifier: "srv1", Node: d.remoteNode(t)}
	ws := newNodeWorkspace(&services.Node{}, server)

	data, err := ws.Read(context.Background(), "eula.txt", 0)
	require.NoError(t, err)
	assert.Equal(t, "eula=true\n", string(data))
	require.NoError(t, ws.Write(context.Background(), "notes.txt", []byte("hi"), false))
	assert.FileExists(t, filepath.Join(d.folders["srv1"], "notes.txt"))

	refusing := &models.Server{Identifier: "srv1", Node: *fakeRemoteNode(t, http.StatusUnauthorized, http.Header{})}
	_, err = newNodeWorkspace(&services.Node{}, refusing).Stat(context.Background(), "eula.txt")
	assert.ErrorIs(t, err, workspace.ErrFailed)
	assert.Contains(t, err.Error(), "credentials")
}

// A node from before the assistant's routes answers 404 with no error code: that must read as
// "update the node", never as "the file is not there", or create_file would go ahead and an old
// node would write a file named "name?create=true"
func TestAssistantRouteError_OldNodeIsNotNotFound(t *testing.T) {
	plain404 := func() *http.Response {
		rec := httptest.NewRecorder()
		rec.WriteHeader(http.StatusNotFound)
		return rec.Result()
	}

	err := assistantRouteError("/daemon/server/abc/fs/stat?path=a", plain404(), "a")
	assert.ErrorIs(t, err, errNodeOutdated)
	assert.NotErrorIs(t, err, workspace.ErrNotFound)

	err = assistantRouteError("/daemon/server/abc/file/a?create=true", plain404(), "a")
	assert.ErrorIs(t, err, errNodeOutdated)

	// the file manager's own routes keep reading a 404 as not found
	err = assistantRouteError("/daemon/server/abc/file/a", plain404(), "a")
	assert.ErrorIs(t, err, workspace.ErrNotFound)

	// a node that has the routes names the error, and a missing file stays not found
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", "application/json")
	rec.WriteHeader(http.StatusNotFound)
	_, _ = rec.WriteString(`{"error":{"code":"ErrFileNotFound","msg":"not found: a"}}`)
	err = assistantRouteError("/daemon/server/abc/fs/stat?path=a", rec.Result(), "a")
	assert.ErrorIs(t, err, workspace.ErrNotFound)
}
