package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/files"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/web/daemon"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/agent"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/workspace"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// fakeDaemon serves the node routes the assistant uses over real folders, one per server. The
// assistant's routes (fs/stat, fs/copy, fs/search) work as the daemon's do, through the
// workspace over the confined file server and with the daemon's error codes; the file
// manager's routes answer the way the daemon's handlers do.
type fakeDaemon struct {
	folders map[string]string // server identifier → its folder
	engine  *gin.Engine

	mu    sync.Mutex
	calls []string
}

func newFakeDaemon(t *testing.T, ids ...string) *fakeDaemon {
	gin.SetMode(gin.TestMode)
	d := &fakeDaemon{folders: map[string]string{}}
	for _, id := range ids {
		d.folders[id] = t.TempDir()
	}

	r := gin.New()
	g := r.Group("/daemon/server/:serverId", d.resolve)
	g.GET("/fs/stat", func(c *gin.Context) {
		entry, err := d.workspace(c).Stat(c.Request.Context(), c.Query("path"))
		if err != nil {
			fakeFileError(c, err)
			return
		}
		c.JSON(http.StatusOK, entry)
	})
	g.POST("/fs/copy", func(c *gin.Context) {
		var req struct {
			Source      string `json:"source"`
			Destination string `json:"destination"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			fakeFileError(c, workspace.ErrInvalid)
			return
		}
		if err := d.workspace(c).Copy(c.Request.Context(), req.Source, req.Destination); err != nil {
			fakeFileError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	g.GET("/fs/search", func(c *gin.Context) {
		opts := workspace.SearchOptions{Query: c.Query("query"), InContent: c.Query("content") == "true"}
		matches, err := d.workspace(c).Search(c.Request.Context(), c.Query("dir"), opts)
		if err != nil {
			fakeFileError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"matches": matches, "truncated": false})
	})
	g.GET("/file/*filename", d.getFile)
	g.PUT("/file/*filename", d.putFile)
	g.DELETE("/file/*filename", func(c *gin.Context) {
		if err := d.workspace(c).Delete(c.Request.Context(), fileParam(c)); err != nil {
			fakeFileError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	g.POST("/file/*filename", func(c *gin.Context) {
		fsys := d.fileServer(c)
		destination := c.Query("destination")
		if _, err := fsys.Stat(destination); err == nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, &skypanel.ErrorResponse{Error: skypanel.ErrFileExists})
			return
		}
		if err := fsys.Rename(fileParam(c), destination); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, &skypanel.ErrorResponse{Error: skypanel.ErrInternal})
			return
		}
		c.Status(http.StatusNoContent)
	})
	d.engine = r
	return d
}

// resolve finds the server of the route, as ResolveServerNode does, and opens its folder
func (d *fakeDaemon) resolve(c *gin.Context) {
	d.mu.Lock()
	d.calls = append(d.calls, c.Request.Method+" "+c.Request.URL.RequestURI())
	d.mu.Unlock()
	folder, ok := d.folders[c.Param("serverId")]
	if !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	fsys, err := files.NewFileServer(folder, -1, -1)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	defer func() { _ = fsys.Close() }()
	c.Set("fs", fsys)
	c.Next()
}

func (d *fakeDaemon) fileServer(c *gin.Context) files.FileServer {
	return c.MustGet("fs").(files.FileServer)
}

func (d *fakeDaemon) workspace(c *gin.Context) workspace.Workspace {
	return workspace.NewFileServerWorkspace(d.fileServer(c))
}

func fileParam(c *gin.Context) string {
	return strings.TrimPrefix(c.Param("filename"), "/")
}

// getFile answers like the daemon's: a listing (".." first below the root) or the content
func (d *fakeDaemon) getFile(c *gin.Context) {
	fsys := d.fileServer(c)
	name := fileParam(c)
	info, err := fsys.Stat(name)
	if os.IsNotExist(err) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, &skypanel.ErrorResponse{Error: skypanel.ErrInternal})
		return
	}
	if info.IsDir() {
		items, _ := fsys.ReadDir(name)
		list := []skypanel.FileDesc{}
		if name != "" {
			list = append(list, skypanel.FileDesc{Name: ".."})
		}
		for _, item := range items {
			desc := skypanel.FileDesc{Name: item.Name(), File: !item.IsDir()}
			// stat by path like the real daemon (listDirectory): DirEntry.Info is not reliable
			// for entries of a subfolder of the file server
			if itemInfo, err := fsys.Stat(path.Join(name, item.Name())); err == nil && !item.IsDir() {
				desc.Size = itemInfo.Size()
				desc.Modified = itemInfo.ModTime().Unix()
			}
			list = append(list, desc)
		}
		c.JSON(http.StatusOK, list)
		return
	}
	f, err := fsys.Open(name)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	defer func() { _ = f.Close() }()
	c.DataFromReader(http.StatusOK, info.Size(), "application/octet-stream", f, nil)
}

// putFile answers like the daemon's: ?folder makes folders, ?create=true never replaces
func (d *fakeDaemon) putFile(c *gin.Context) {
	fsys := d.fileServer(c)
	name := fileParam(c)
	if _, folder := c.GetQuery("folder"); folder {
		if err := fsys.MkdirAll(name, 0755); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
		return
	}
	if err := fsys.MkdirAll(path.Dir(name), 0755); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	flags := os.O_CREATE | os.O_TRUNC | os.O_RDWR
	if _, create := c.GetQuery("create"); create {
		flags = os.O_CREATE | os.O_EXCL | os.O_WRONLY
	}
	f, err := fsys.OpenFile(name, flags, 0644)
	if os.IsExist(err) {
		c.AbortWithStatusJSON(http.StatusConflict, &skypanel.ErrorResponse{Error: skypanel.ErrFileExists})
		return
	}
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(f, c.Request.Body); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
}

func fakeFileError(c *gin.Context, err error) {
	status, code, known := daemon.FileErrorStatus(err)
	message := err.Error()
	if !known {
		message = workspace.ErrFailed.Error()
	}
	c.AbortWithStatusJSON(status, &skypanel.ErrorResponse{Error: skypanel.CreateError(message, code)})
}

// call reaches the fake daemon in process, as a node call would
func (d *fakeDaemon) call(ctx context.Context, method, target string, body io.Reader, header http.Header) (*http.Response, error) {
	req := httptest.NewRequest(method, target, body).WithContext(ctx)
	for k, v := range header {
		req.Header[k] = v
	}
	rec := httptest.NewRecorder()
	d.engine.ServeHTTP(rec, req)
	return rec.Result(), nil
}

// workspaceFor is a nodeWorkspace of one server of the fake daemon
func (d *fakeDaemon) workspaceFor(id string) *nodeWorkspace {
	return &nodeWorkspace{prefix: daemonServerPrefix + id, node: "fake", call: d.call}
}

// remoteNode serves the fake daemon over HTTP as a remote node, so the panel reaches it the
// way it reaches real nodes (CallNodeContext, with its token and TLS probe)
func (d *fakeDaemon) remoteNode(t *testing.T) models.Node {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			// the panel's probe for TLS
			w.WriteHeader(http.StatusOK)
			return
		}
		d.engine.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	return models.Node{Name: "remote", PublicHost: "127.0.0.1", PrivateHost: "127.0.0.1", PublicPort: uint16(port), PrivatePort: uint16(port)}
}

func (d *fakeDaemon) callLog() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.calls...)
}

// fakeModel answers each round with what respond gives and records the tools it was offered
type fakeModel struct {
	mu      sync.Mutex
	respond func(round int, history []agent.Message) (agent.Message, error)
	offered [][]string
	rounds  int
}

func (m *fakeModel) Generate(_ context.Context, _ string, history []agent.Message, tools []toolkit.Tool) (agent.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name())
	}
	m.offered = append(m.offered, names)
	m.rounds++
	return m.respond(m.rounds, history)
}

func (m *fakeModel) roundCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rounds
}

// callsThenText answers a new question with calls, and the tool answers with text
func callsThenText(text string, calls ...agent.FunctionCall) func(int, []agent.Message) (agent.Message, error) {
	return func(_ int, history []agent.Message) (agent.Message, error) {
		if len(history) > 0 && history[len(history)-1].Role == agent.RoleTool {
			return agent.Message{Role: agent.RoleModel, Parts: []agent.Part{{Text: text}}}, nil
		}
		msg := agent.Message{Role: agent.RoleModel, Parts: []agent.Part{{Text: "I will do that."}}}
		for i := range calls {
			call := calls[i]
			msg.Parts = append(msg.Parts, agent.Part{Call: &call})
		}
		return msg, nil
	}
}

func writeServerFile(t *testing.T, folder, name, content string) {
	t.Helper()
	full := path.Join(folder, name)
	require.NoError(t, os.MkdirAll(path.Dir(full), 0755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0644))
}
