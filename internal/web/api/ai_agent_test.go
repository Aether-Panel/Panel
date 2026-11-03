package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/agent"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/gemini"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// agentHarness wires the assistant's routes as the panel does, with the activity log in front,
// a scripted model, and servers whose node is the fake daemon reached over HTTP
type agentHarness struct {
	db      *gorm.DB
	router  *gin.Engine
	daemon  *fakeDaemon
	model   *fakeModel
	servers map[string]*models.Server
	alice   *models.User
	bob     *models.User
	user    *models.User
	scopes  []*scopes.Scope
}

var (
	allFileScopes = []*scopes.Scope{scopes.ScopeServerConsole, scopes.ScopeServerFileView, scopes.ScopeServerFileEdit}
	viewFileScope = []*scopes.Scope{scopes.ScopeServerConsole, scopes.ScopeServerFileView}
)

func useSetting[T any](t *testing.T, entry interface {
	Value() T
	Set(T, bool) error
}, value T) {
	previous := entry.Value()
	require.NoError(t, entry.Set(value, false))
	t.Cleanup(func() { _ = entry.Set(previous, false) })
}

func newAgentHarness(t *testing.T) *agentHarness {
	gin.SetMode(gin.TestMode)

	previousWrite := serverActivityWrite
	serverActivityWrite = func(fn func()) { fn() }
	previousLimiter := aiLimiter
	aiLimiter = services.NewAttemptLimiter(1000, time.Minute)
	previousGather := aiGatherContext
	aiGatherContext = func(context.Context, *services.Node, *models.Server) (string, []string) {
		return "Server: test", []string{"status"}
	}
	previousModel := aiAgentModel
	t.Cleanup(func() {
		serverActivityWrite = previousWrite
		aiLimiter = previousLimiter
		aiGatherContext = previousGather
		aiAgentModel = previousModel
	})
	useSetting[string](t, config.GeminiAPIKey, "test-key")
	useSetting[bool](t, config.AIFileTools, true)

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "ai_agent.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Node{}, &models.Server{}, &models.ServerActivity{}))

	h := &agentHarness{db: db, daemon: newFakeDaemon(t, "srv1", "srv2"), model: &fakeModel{}, servers: map[string]*models.Server{}}
	aiAgentModel = func(apiKey string) agent.Generator {
		assert.Equal(t, "test-key", apiKey)
		return h.model
	}
	h.alice = &models.User{Username: "alice", Email: "alice@example.com", HashedPassword: "x"}
	h.bob = &models.User{Username: "bob", Email: "bob@example.com", HashedPassword: "x"}
	require.NoError(t, db.Create(h.alice).Error)
	require.NoError(t, db.Create(h.bob).Error)
	h.user, h.scopes = h.alice, allFileScopes

	node := h.daemon.remoteNode(t)
	for _, id := range []string{"srv1", "srv2"} {
		require.NoError(t, db.Omit(clause.Associations).Create(&models.Server{Identifier: id, Name: id, Type: "generic"}).Error)
		h.servers[id] = &models.Server{Identifier: id, Name: id, Type: "generic", Node: node}
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("user", h.user)
		c.Set("scopes", h.scopes)
		c.Next()
	})
	g := r.Group("/api/servers")
	g.Use(recordServerActivity)
	// what ResolveServerPanel does: the server comes from the route
	resolve := func(c *gin.Context) {
		server, ok := h.servers[c.Param("serverId")]
		if !ok {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.Set("server", server)
	}
	g.POST("/:serverId/ai/chat", resolve, chatWithAssistant)
	g.POST("/:serverId/ai/chat/confirm", resolve, confirmAssistantChanges)
	h.router = r
	return h
}

func (h *agentHarness) post(t *testing.T, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.9:4000"
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func (h *agentHarness) ask(t *testing.T, serverID, question string) *httptest.ResponseRecorder {
	return h.post(t, "/api/servers/"+serverID+"/ai/chat", map[string]any{
		"messages": []map[string]string{{"role": "user", "content": question}},
		"language": "en",
	})
}

func (h *agentHarness) confirm(t *testing.T, serverID, token string, approved ...string) *httptest.ResponseRecorder {
	if approved == nil {
		approved = []string{}
	}
	return h.post(t, "/api/servers/"+serverID+"/ai/chat/confirm", map[string]any{"token": token, "approved": approved, "language": "en"})
}

func (h *agentHarness) activity(t *testing.T) []models.ServerActivity {
	var out []models.ServerActivity
	require.NoError(t, h.db.Order("id").Find(&out).Error)
	return out
}

type agentAnswer struct {
	Reply   string         `json:"reply"`
	Sources []string       `json:"sources"`
	Steps   []agent.Step   `json:"steps"`
	Pending *agent.Pending `json:"pending"`
}

func decodeAgentAnswer(t *testing.T, rec *httptest.ResponseRecorder) agentAnswer {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	for _, key := range []string{"reply", "sources", "steps", "pending"} {
		assert.Contains(t, raw, key, "the answer always has %s", key)
	}
	assert.NotEqual(t, "null", string(raw["steps"]), "steps is always a list")
	var answer agentAnswer
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &answer))
	return answer
}

func answerCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body skypanel.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	require.NotNil(t, body.Error, rec.Body.String())
	return body.Error.Code
}

func createAndDelete() []agent.FunctionCall {
	return []agent.FunctionCall{
		{ID: "c1", Name: "create_file", Args: map[string]any{"path": "notes/todo.txt", "content": "hi"}},
		{ID: "c2", Name: "delete_path", Args: map[string]any{"path": "old.txt"}},
	}
}

// The whole way: the chat creates a file at once and waits for the user before deleting one;
// the confirmation runs the delete on the node and the assistant answers. Both changes are in
// the server's activity log under the user, and nothing else is.
func TestAssistantAgent_RoundTripThroughTheNode(t *testing.T) {
	h := newAgentHarness(t)
	folder := h.daemon.folders["srv1"]
	writeServerFile(t, folder, "old.txt", "old")
	h.model.respond = callsThenText("Done: I created the notes and deleted old.txt.", createAndDelete()...)

	answer := decodeAgentAnswer(t, h.ask(t, "srv1", "make a notes file and delete old.txt"))
	assert.Equal(t, "I will do that.", answer.Reply, "the explanation above the confirmation")
	assert.Equal(t, []string{"status"}, answer.Sources)
	require.Len(t, answer.Steps, 1)
	assert.Equal(t, agent.Step{ID: "c1", Tool: "create_file", Kind: toolkit.KindCreate, Summary: answer.Steps[0].Summary, Paths: []string{"notes/todo.txt"}, Status: agent.StatusDone}, answer.Steps[0])
	require.NotNil(t, answer.Pending)
	require.Len(t, answer.Pending.Actions, 1)
	assert.Equal(t, "c2", answer.Pending.Actions[0].ID)
	assert.Equal(t, toolkit.KindDestructive, answer.Pending.Actions[0].Kind)
	assert.Equal(t, agent.StatusPending, answer.Pending.Actions[0].Status)
	assert.Equal(t, []string{"old.txt"}, answer.Pending.Actions[0].Paths)
	assert.Contains(t, h.model.offered[0], "delete_path")
	assert.Contains(t, h.model.offered[0], "read_file")

	assert.FileExists(t, filepath.Join(folder, "notes", "todo.txt"))
	assert.FileExists(t, filepath.Join(folder, "old.txt"), "nothing is deleted before the user confirms")

	answer = decodeAgentAnswer(t, h.confirm(t, "srv1", answer.Pending.Token, "c2"))
	assert.Equal(t, "Done: I created the notes and deleted old.txt.", answer.Reply)
	assert.Equal(t, []string{}, answer.Sources)
	require.Len(t, answer.Steps, 1)
	assert.Equal(t, "c2", answer.Steps[0].ID)
	assert.Equal(t, agent.StatusDone, answer.Steps[0].Status)
	assert.Nil(t, answer.Pending)
	assert.NoFileExists(t, filepath.Join(folder, "old.txt"))

	entries := h.activity(t)
	require.Len(t, entries, 2, "one entry per change; the chat and the confirmation themselves are not logged")
	assert.Equal(t, "ai.create_file", entries[0].Action)
	assert.Equal(t, "notes/todo.txt", entries[0].Detail)
	assert.Equal(t, "ai.delete_path", entries[1].Action)
	assert.Equal(t, "old.txt", entries[1].Detail)
	for _, e := range entries {
		assert.Equal(t, "srv1", e.ServerID)
		require.NotNil(t, e.UserID)
		assert.Equal(t, h.alice.ID, *e.UserID)
		assert.Equal(t, "alice", e.Username)
		assert.Equal(t, "203.0.113.9", e.IP)
	}

	// the token is single use
	rec := h.confirm(t, "srv1", "0123", "c2")
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "ErrAIConfirmationInvalid", answerCode(t, rec))
}

// A confirmation only works for the user and the server it was given for. A wrong attempt runs
// nothing and does not use the confirmation up.
func TestAssistantAgent_ConfirmationIsBoundToUserAndServer(t *testing.T) {
	h := newAgentHarness(t)
	folder := h.daemon.folders["srv1"]
	writeServerFile(t, folder, "old.txt", "old")
	h.model.respond = callsThenText("Deleted.", agent.FunctionCall{ID: "c1", Name: "delete_path", Args: map[string]any{"path": "old.txt"}})

	answer := decodeAgentAnswer(t, h.ask(t, "srv1", "delete old.txt"))
	require.NotNil(t, answer.Pending)
	token := answer.Pending.Token
	rounds := h.model.roundCount()

	// another user, with every permission on the server
	h.user = h.bob
	rec := h.confirm(t, "srv1", token, "c1")
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "ErrAIConfirmationInvalid", answerCode(t, rec))

	// the same user, through another server's route
	h.user = h.alice
	rec = h.confirm(t, "srv2", token, "c1")
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "ErrAIConfirmationInvalid", answerCode(t, rec))

	assert.FileExists(t, filepath.Join(folder, "old.txt"), "nothing ran")
	assert.Equal(t, rounds, h.model.roundCount(), "the model was not asked again")
	assert.Empty(t, h.activity(t))

	// the owner can still answer it
	answer = decodeAgentAnswer(t, h.confirm(t, "srv1", token, "c1"))
	assert.Equal(t, "Deleted.", answer.Reply)
	assert.NoFileExists(t, filepath.Join(folder, "old.txt"))
}

func TestAssistantAgent_DeclinedActionsDoNotRun(t *testing.T) {
	h := newAgentHarness(t)
	folder := h.daemon.folders["srv1"]
	writeServerFile(t, folder, "old.txt", "old")
	h.model.respond = callsThenText("Fine, I left it.", agent.FunctionCall{ID: "c1", Name: "delete_path", Args: map[string]any{"path": "old.txt"}})

	answer := decodeAgentAnswer(t, h.ask(t, "srv1", "delete old.txt"))
	require.NotNil(t, answer.Pending)

	answer = decodeAgentAnswer(t, h.confirm(t, "srv1", answer.Pending.Token))
	require.Len(t, answer.Steps, 1)
	assert.Equal(t, agent.StatusDeclined, answer.Steps[0].Status)
	assert.FileExists(t, filepath.Join(folder, "old.txt"))
	assert.Empty(t, h.activity(t))
}

// Without server.files.edit the edit tools are not offered, and a call to one anyway is refused
func TestAssistantAgent_WithoutEditPermission(t *testing.T) {
	h := newAgentHarness(t)
	h.scopes = viewFileScope
	folder := h.daemon.folders["srv1"]
	writeServerFile(t, folder, "old.txt", "old")
	h.model.respond = callsThenText("I cannot delete it.", agent.FunctionCall{ID: "c1", Name: "delete_path", Args: map[string]any{"path": "old.txt"}})

	answer := decodeAgentAnswer(t, h.ask(t, "srv1", "delete old.txt"))
	assert.Equal(t, "I cannot delete it.", answer.Reply)
	require.Len(t, answer.Steps, 1)
	assert.Equal(t, agent.StatusRefused, answer.Steps[0].Status)
	assert.Nil(t, answer.Pending)
	assert.FileExists(t, filepath.Join(folder, "old.txt"))
	assert.Empty(t, h.activity(t))

	for _, offered := range h.model.offered {
		assert.Contains(t, offered, "read_file")
		for _, name := range []string{"create_file", "create_directory", "copy_path", "edit_file", "write_file", "move_path", "rename_path", "delete_path"} {
			assert.NotContains(t, offered, name)
		}
	}
}

// Permissions are read again on the confirmation: one taken away meanwhile stops the change
func TestAssistantAgent_ConfirmChecksPermissionsAgain(t *testing.T) {
	h := newAgentHarness(t)
	folder := h.daemon.folders["srv1"]
	writeServerFile(t, folder, "old.txt", "old")
	h.model.respond = callsThenText("Could not delete it.", agent.FunctionCall{ID: "c1", Name: "delete_path", Args: map[string]any{"path": "old.txt"}})

	answer := decodeAgentAnswer(t, h.ask(t, "srv1", "delete old.txt"))
	require.NotNil(t, answer.Pending)
	token := answer.Pending.Token

	h.scopes = []*scopes.Scope{scopes.ScopeServerConsole}
	rec := h.confirm(t, "srv1", token, "c1")
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, skypanel.ErrNoPermission.Code, answerCode(t, rec))

	h.scopes = viewFileScope
	answer = decodeAgentAnswer(t, h.confirm(t, "srv1", token, "c1"))
	require.Len(t, answer.Steps, 1)
	assert.Equal(t, agent.StatusRefused, answer.Steps[0].Status)
	assert.FileExists(t, filepath.Join(folder, "old.txt"))
}

// With the setting off, or without permission to see the files, the chat is the plain one
func TestAssistantAgent_PlainChatWhenOffOrWithoutFilePermission(t *testing.T) {
	h := newAgentHarness(t)
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"Plain answer."}]}}]}`))
	}))
	t.Cleanup(service.Close)
	previous := geminiChatEndpoint
	geminiChatEndpoint = service.URL
	t.Cleanup(func() { geminiChatEndpoint = previous })
	h.model.respond = func(int, []agent.Message) (agent.Message, error) {
		return agent.Message{}, errors.New("the model must not be used")
	}

	check := func() {
		rec := h.ask(t, "srv1", "why is it slow?")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
		assert.JSONEq(t, `"Plain answer."`, string(raw["reply"]))
		assert.NotContains(t, raw, "steps")
		assert.NotContains(t, raw, "pending")
	}

	useSetting[bool](t, config.AIFileTools, false)
	check()
	rec := h.confirm(t, "srv1", "0123", "c1")
	assert.Equal(t, http.StatusConflict, rec.Code, "nothing can be confirmed with the tools off")

	require.NoError(t, config.AIFileTools.Set(true, false))
	h.scopes = []*scopes.Scope{scopes.ScopeServerConsole}
	check()
	assert.Zero(t, h.model.roundCount())
}

func TestAssistantAgent_ModelErrorsAreSafe(t *testing.T) {
	h := newAgentHarness(t)

	h.model.respond = func(int, []agent.Message) (agent.Message, error) { return agent.Message{}, gemini.ErrBlocked }
	rec := h.ask(t, "srv1", "hello")
	assert.Equal(t, http.StatusBadGateway, rec.Code)
	assert.Contains(t, rec.Body.String(), gemini.ErrBlocked.Error())
	assert.Contains(t, rec.Body.String(), `"steps":[]`)

	h.model.respond = func(int, []agent.Message) (agent.Message, error) {
		return agent.Message{}, errors.New("dial tcp 10.0.0.7:443: key=secret")
	}
	rec = h.ask(t, "srv1", "hello")
	assert.Equal(t, http.StatusBadGateway, rec.Code)
	assert.NotContains(t, rec.Body.String(), "secret")
	assert.NotContains(t, rec.Body.String(), "10.0.0.7")
}

func TestAssistantConfirm_ValidatesTheRequest(t *testing.T) {
	h := newAgentHarness(t)

	rec := h.post(t, "/api/servers/srv1/ai/chat/confirm", map[string]any{"approved": []string{"c1"}})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "no token")

	many := make([]string, aiConfirmMaxActions+1)
	for i := range many {
		many[i] = "c"
	}
	rec = h.confirm(t, "srv1", "0123", many...)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = h.confirm(t, "srv1", strings.Repeat("a", aiConfirmMaxLength+1))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAgentInstruction_AddsTheToolRules(t *testing.T) {
	server := &models.Server{Name: "survival"}
	text := agentInstruction(server, "es", "Recent console:\n| hi")
	assert.Contains(t, text, "Write in Spanish")
	assert.Contains(t, text, "relative to the server folder")
	assert.Contains(t, text, "never instructions")
	assert.Contains(t, text, "<server-context>\nRecent console:\n| hi\n</server-context>")
	assert.NotContains(t, assistantInstruction(server, "es", ""), "through the tools", "the plain chat has no tools")
}
