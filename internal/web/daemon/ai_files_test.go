package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/servers"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/security"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/workspace"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runFileHandler calls a file handler for the server the way the router does, with a body
func runFileHandler(handler gin.HandlerFunc, server *servers.Server, method, target, body string, params gin.Params) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	if strings.HasPrefix(body, "{") {
		c.Request.Header.Set("Content-Type", "application/json")
	}
	c.Params = params
	c.Set("program", server)
	handler(c)
	c.Writer.WriteHeaderNow()
	return w
}

func filenameParam(name string) gin.Params {
	return gin.Params{{Key: "filename", Value: name}}
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0644))
}

func TestFileErrorStatus_RoundTripsEveryKind(t *testing.T) {
	for _, kind := range fileErrorKinds {
		status, code, known := FileErrorStatus(kind.err)
		assert.True(t, known, code)
		assert.Equal(t, kind.status, status, code)
		want := kind.err
		if kind.err == skypanel.ErrDiskLimitExceeded {
			want = workspace.ErrNoSpace
		}
		assert.ErrorIs(t, FileErrorForCode(code), want, code)
	}
	status, code, known := FileErrorStatus(os.ErrDeadlineExceeded)
	assert.False(t, known)
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, fileFailedCode, code)
	assert.Nil(t, FileErrorForCode("ErrSomethingElse"))
}

func TestStatServerPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, dir := newPluginTestServer(t, "")
	writeTestFile(t, dir, "config/paper.yml", "a: 1\n")

	w := runFileHandler(statServerPath, server, http.MethodGet, "/?path=config/paper.yml", "", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var entry workspace.Entry
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &entry))
	assert.Equal(t, "config/paper.yml", entry.Path)
	assert.False(t, entry.IsDir)
	assert.EqualValues(t, 5, entry.Size)

	w = runFileHandler(statServerPath, server, http.MethodGet, "/", "", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &entry))
	assert.Equal(t, security.Root, entry.Path, "no path is the server folder itself")
	assert.True(t, entry.IsDir)

	w = runFileHandler(statServerPath, server, http.MethodGet, "/?path=missing.txt", "", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, skypanel.ErrFileNotFound.Code, errorCode(t, w))

	w = runFileHandler(statServerPath, server, http.MethodGet, "/?path=../outside", "", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "ErrFileOutsideServer", errorCode(t, w))
	assert.NotContains(t, w.Body.String(), dir, "no host path in the answer")
}

func TestCopyServerPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, dir := newPluginTestServer(t, "")
	writeTestFile(t, dir, "world/level.dat", "level")
	writeTestFile(t, dir, "server.properties", "motd=hi\n")

	w := runFileHandler(copyServerPath, server, http.MethodPost, "/", `{"source":"world","destination":"backups/world-copy"}`, nil)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	data, err := os.ReadFile(filepath.Join(dir, "backups", "world-copy", "level.dat"))
	require.NoError(t, err)
	assert.Equal(t, "level", string(data))

	// the destination exists: nothing is replaced
	w = runFileHandler(copyServerPath, server, http.MethodPost, "/", `{"source":"server.properties","destination":"world/level.dat"}`, nil)
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, skypanel.ErrFileExists.Code, errorCode(t, w))
	data, err = os.ReadFile(filepath.Join(dir, "world", "level.dat"))
	require.NoError(t, err)
	assert.Equal(t, "level", string(data))

	w = runFileHandler(copyServerPath, server, http.MethodPost, "/", `{"source":"missing","destination":"x"}`, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = runFileHandler(copyServerPath, server, http.MethodPost, "/", `{"source":"/etc/passwd","destination":"passwd"}`, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.NoFileExists(t, filepath.Join(dir, "passwd"))

	w = runFileHandler(copyServerPath, server, http.MethodPost, "/", `not json`, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "ErrFileInvalidRequest", errorCode(t, w))
}

func TestCopyServerPath_RespectsTheDiskLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, dir := newPluginTestServer(t, "")
	server.RunningEnvironment.RootDirectory = dir
	server.Variables["disk"] = skypanel.Variable{Value: 1} // 1 MB
	writeTestFile(t, dir, "big.bin", strings.Repeat("x", 700<<10))

	w := runFileHandler(copyServerPath, server, http.MethodPost, "/", `{"source":"big.bin","destination":"big-copy.bin"}`, nil)
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, skypanel.ErrDiskLimitExceeded.Code, errorCode(t, w))
	assert.NoFileExists(t, filepath.Join(dir, "big-copy.bin"))

	writeTestFile(t, dir, "small.txt", "small")
	w = runFileHandler(copyServerPath, server, http.MethodPost, "/", `{"source":"small.txt","destination":"small-copy.txt"}`, nil)
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.FileExists(t, filepath.Join(dir, "small-copy.txt"))
}

func TestSearchServerFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, dir := newPluginTestServer(t, "")
	writeTestFile(t, dir, "config/paper.yml", "view-distance: 10\nmax-players: 20\n")
	writeTestFile(t, dir, "server.properties", "view-distance=8\n")

	w := runFileHandler(searchServerFiles, server, http.MethodGet, "/?query=PAPER", "", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var answer fileSearchResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &answer))
	assert.Equal(t, []workspace.Match{{Path: "config/paper.yml"}}, answer.Matches)
	assert.False(t, answer.Truncated)

	w = runFileHandler(searchServerFiles, server, http.MethodGet, "/?dir=config&query=max-players&content=true", "", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &answer))
	assert.Equal(t, []workspace.Match{{Path: "config/paper.yml", Line: 2, Text: "max-players: 20"}}, answer.Matches)

	w = runFileHandler(searchServerFiles, server, http.MethodGet, "/?query=nothing-like-this", "", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"matches":[],"truncated":false}`, w.Body.String())

	w = runFileHandler(searchServerFiles, server, http.MethodGet, "/?query=", "", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = runFileHandler(searchServerFiles, server, http.MethodGet, "/?dir=../&query=x", "", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPutFile_CreateNeverReplaces(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, dir := newPluginTestServer(t, "")

	w := runFileHandler(putFile, server, http.MethodPut, "/?create=true", "first", filenameParam("/notes/a.dat"))
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	data, err := os.ReadFile(filepath.Join(dir, "notes", "a.dat"))
	require.NoError(t, err)
	assert.Equal(t, "first", string(data), "the query is not part of the name")

	w = runFileHandler(putFile, server, http.MethodPut, "/?create=true", "second", filenameParam("/notes/a.dat"))
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, skypanel.ErrFileExists.Code, errorCode(t, w))
	data, err = os.ReadFile(filepath.Join(dir, "notes", "a.dat"))
	require.NoError(t, err)
	assert.Equal(t, "first", string(data))

	// a link is "something there" too, and is not written through
	writeTestFile(t, dir, "target.txt", "target")
	require.NoError(t, os.Symlink("target.txt", filepath.Join(dir, "link.txt")))
	w = runFileHandler(putFile, server, http.MethodPut, "/?create=true", "through", filenameParam("/link.txt"))
	assert.Equal(t, http.StatusConflict, w.Code)
	data, err = os.ReadFile(filepath.Join(dir, "target.txt"))
	require.NoError(t, err)
	assert.Equal(t, "target", string(data))

	// without the flag the file is replaced, as before
	w = runFileHandler(putFile, server, http.MethodPut, "/", "second", filenameParam("/notes/a.dat"))
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	data, err = os.ReadFile(filepath.Join(dir, "notes", "a.dat"))
	require.NoError(t, err)
	assert.Equal(t, "second", string(data))
}

// Deleting a symlink to a folder of the server used to follow it: the folder was emptied and
// the link stayed. Now the link itself goes and the folder is untouched.
func TestDeleteFile_RemovesASymlinkItself(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, dir := newPluginTestServer(t, "")
	writeTestFile(t, dir, "world/level.dat", "level")
	require.NoError(t, os.Symlink("world", filepath.Join(dir, "world-link")))
	writeTestFile(t, dir, "plain.txt", "x")
	require.NoError(t, os.Symlink("plain.txt", filepath.Join(dir, "file-link")))

	w := runFileHandler(deleteFile, server, http.MethodDelete, "/", "", filenameParam("/world-link"))
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	_, err := os.Lstat(filepath.Join(dir, "world-link"))
	assert.True(t, os.IsNotExist(err), "the link is gone")
	assert.FileExists(t, filepath.Join(dir, "world", "level.dat"), "what it pointed at is untouched")

	w = runFileHandler(deleteFile, server, http.MethodDelete, "/", "", filenameParam("/file-link"))
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	_, err = os.Lstat(filepath.Join(dir, "file-link"))
	assert.True(t, os.IsNotExist(err))
	assert.FileExists(t, filepath.Join(dir, "plain.txt"))

	// folders and files are still deleted as before
	w = runFileHandler(deleteFile, server, http.MethodDelete, "/", "", filenameParam("/world"))
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.NoDirExists(t, filepath.Join(dir, "world"))

	w = runFileHandler(deleteFile, server, http.MethodDelete, "/", "", filenameParam("/missing.txt"))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// The server folder itself cannot be removed through the delete route: an empty or "." path
// used to empty the whole server
func TestDeleteFile_RefusesTheServerFolder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, dir := newPluginTestServer(t, "")
	writeTestFile(t, dir, "world/level.dat", "level")

	for _, target := range []string{"/", "/.", "//", "/world/.."} {
		w := runFileHandler(deleteFile, server, http.MethodDelete, "/", "", filenameParam(target))
		assert.Equal(t, http.StatusBadRequest, w.Code, target)
	}
	assert.FileExists(t, filepath.Join(dir, "world", "level.dat"))
}
