package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/agent"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/filesystem"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/gemini"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/workspace"
	"github.com/gin-gonic/gin"
)

// The assistant's agent mode: with the file tools on, a chat runs the toolkit's agent loop,
// where the model can read and change the files of the server of the route (and only that
// one) through the tools the user's permissions allow. Changes to existing data stop for the
// user's confirmation, answered on /ai/chat/confirm.

const (
	// aiConfirmMaxActions caps the call IDs a confirmation may approve
	aiConfirmMaxActions = 64
	// aiConfirmMaxLength caps the token and each call ID of a confirmation
	aiConfirmMaxLength = 128
)

// aiAgentShared holds the tools and the pending confirmations, one of each per panel
// process; aiRunner hands out copies of it bound to a request's model
var (
	aiAgentOnce   sync.Once
	aiAgentShared *agent.Runner
)

// aiAgentModel makes the model of a request. The API key is read on every request, because an
// administrator can change it at any time. Tests swap it for a scripted model.
var aiAgentModel = func(apiKey string) agent.Generator {
	return &gemini.Client{APIKey: apiKey}
}

// aiAgentWorkspace makes the workspace of the route's server, reached through its node. Tests
// swap it for a local folder.
var aiAgentWorkspace = func(c *gin.Context, server *models.Server) workspace.Workspace {
	return newNodeWorkspace(&services.Node{DB: middleware.GetDatabase(c)}, server)
}

// aiRunner returns the runner for one request: the shared registry and confirmation store
// with this request's model. A copy, so concurrent requests never share a model.
func aiRunner(apiKey string) *agent.Runner {
	aiAgentOnce.Do(func() {
		registry := toolkit.NewRegistry()
		filesystem.Register(registry)
		aiAgentShared = &agent.Runner{Registry: registry, Store: agent.NewConfirmationStore()}
	})
	runner := *aiAgentShared
	runner.Model = aiAgentModel(apiKey)
	return &runner
}

// aiConfirmRequest answers the changes the assistant is waiting on
type aiConfirmRequest struct {
	// Token is the pending confirmation's token
	Token string `json:"token"`
	// Approved are the IDs of the pending actions to run; the others are declined
	Approved []string `json:"approved"`
	// Language is the reader's language ("es" or "en")
	Language string `json:"language"`
} // @name AIConfirmRequest

// aiAgentResponse is what the chat and the confirmation answer in agent mode. Steps is always
// a list and Pending is null when nothing waits for the user.
type aiAgentResponse struct {
	Reply   string         `json:"reply"`
	Sources []string       `json:"sources"`
	Steps   []agent.Step   `json:"steps"`
	Pending *agent.Pending `json:"pending"`
} // @name AIAgentResponse

// aiUser is the signed-in user of the request, or nil
func aiUser(c *gin.Context) *models.User {
	if v, ok := c.Get("user"); ok {
		if user, isUser := v.(*models.User); isUser && user != nil && user.ID != 0 {
			return user
		}
	}
	return nil
}

// aiFileToolScopes turns the caller's scopes for this server (already bounded by an OAuth2
// client's) into the toolkit's scope names. Read on every request, so a permission taken away
// counts at once.
func aiFileToolScopes(c *gin.Context) []string {
	granted := callerScopes(c)
	out := make([]string, 0, 2)
	if scopes.ContainsScope(granted, scopes.ScopeServerFileView) {
		out = append(out, toolkit.ScopeFilesView)
	}
	if scopes.ContainsScope(granted, scopes.ScopeServerFileEdit) {
		out = append(out, toolkit.ScopeFilesEdit)
	}
	return out
}

// aiAgentScopes returns the toolkit scopes of the request and whether the chat runs in agent
// mode: the setting is on, a user is signed in, and they may at least see the server's files
func aiAgentScopes(c *gin.Context) ([]string, bool) {
	if !config.AIFileTools.Value() || aiUser(c) == nil {
		return nil, false
	}
	toolScopes := aiFileToolScopes(c)
	return toolScopes, slices.Contains(toolScopes, toolkit.ScopeFilesView)
}

// aiSession is who runs the task and on which server, all taken from the authenticated
// request: the workspace is bound to the route's server here, never by the model
func aiSession(c *gin.Context, server *models.Server, user *models.User, toolScopes []string, language string) agent.Session {
	return agent.Session{
		ServerID: server.Identifier,
		UserID:   user.ID,
		Scopes:   toolScopes,
		Exec: toolkit.ExecContext{
			ServerID:  server.Identifier,
			UserID:    user.ID,
			Workspace: aiAgentWorkspace(c, server),
		},
		Language: language,
	}
}

// aiHistory turns the chat messages into the agent's
func aiHistory(messages []aiChatMessage) []agent.Message {
	history := make([]agent.Message, 0, len(messages))
	for _, m := range messages {
		role := agent.RoleUser
		if m.Role == "assistant" {
			role = agent.RoleModel
		}
		history = append(history, agent.Message{Role: role, Parts: []agent.Part{{Text: m.Content}}})
	}
	return history
}

// runAssistantAgent answers a chat in agent mode
func runAssistantAgent(c *gin.Context, apiKey string, server *models.Server, req aiChatRequest, serverContext string, sources, toolScopes []string) {
	user := aiUser(c)
	session := aiSession(c, server, user, toolScopes, req.Language)
	out, err := aiRunner(apiKey).Start(c.Request.Context(), session, agentInstruction(server, req.Language, serverContext), aiHistory(req.Messages))
	answerAgent(c, server, user, out, err, sources)
}

// @Summary Answer the server assistant's pending changes
// @Description Runs the changes the assistant is waiting on that the user approved and declines the rest, then lets the assistant
// @Description continue. The token is single use, expires after 10 minutes and only works for the user and the server it was given for.
// @Description The scopes are checked again: without server.files.edit an approved edit is refused.
// @Success 200 {object} aiAgentResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} skypanel.ErrorResponse
// @Failure 409 {object} skypanel.ErrorResponse "ErrAIConfirmationInvalid: expired, already answered, or for another user or server"
// @Failure 429 {object} map[string]interface{}
// @Failure 502 {object} map[string]interface{}
// @Param id path string true "Server ID"
// @Param body body aiConfirmRequest true "The token and the approved action IDs"
// @Tags Servers
// @Router /api/servers/{id}/ai/chat/confirm [post]
// @Security OAuth2Application[server.console]
func confirmAssistantChanges(c *gin.Context) {
	apiKey := config.GeminiAPIKey.Value()
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gemini API Key is not configured. Please add it in Settings."})
		return
	}

	var req aiConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}
	if err := validateConfirm(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !aiAllowed(c) {
		return
	}

	user := aiUser(c)
	if user == nil {
		c.AbortWithStatusJSON(http.StatusForbidden, &skypanel.ErrorResponse{Error: skypanel.ErrNoPermission})
		return
	}
	if !config.AIFileTools.Value() {
		// the file tools were turned off meanwhile: what was waiting cannot run any more
		c.AbortWithStatusJSON(http.StatusConflict, &skypanel.ErrorResponse{Error: skypanel.ErrAIConfirmationInvalid})
		return
	}
	toolScopes := aiFileToolScopes(c)
	if !slices.Contains(toolScopes, toolkit.ScopeFilesView) {
		c.AbortWithStatusJSON(http.StatusForbidden, &skypanel.ErrorResponse{Error: skypanel.ErrNoPermission})
		return
	}

	server := getServerFromGin(c)
	session := aiSession(c, server, user, toolScopes, req.Language)
	out, err := aiRunner(apiKey).Resume(c.Request.Context(), session, req.Token, req.Approved)
	if errors.Is(err, agent.ErrConfirmationInvalid) {
		c.AbortWithStatusJSON(http.StatusConflict, &skypanel.ErrorResponse{Error: skypanel.ErrAIConfirmationInvalid})
		return
	}
	answerAgent(c, server, user, out, err, []string{})
}

func validateConfirm(req *aiConfirmRequest) error {
	if req.Token == "" || len(req.Token) > aiConfirmMaxLength {
		return errors.New("the confirmation token is missing or invalid")
	}
	if len(req.Approved) > aiConfirmMaxActions {
		return fmt.Errorf("at most %d actions can be approved at once", aiConfirmMaxActions)
	}
	for _, id := range req.Approved {
		if id == "" || len(id) > aiConfirmMaxLength {
			return errors.New("an approved action ID is invalid")
		}
	}
	return nil
}

// answerAgent records the changes that were made and answers with the outcome. A failed task
// still lists the steps that ran before it failed.
func answerAgent(c *gin.Context, server *models.Server, user *models.User, out agent.Outcome, err error, sources []string) {
	steps := out.Steps
	if steps == nil {
		steps = []agent.Step{}
	}
	recordAgentActivity(c, server, user, steps)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": aiAgentErrorMessage(err), "steps": steps, "pending": nil})
		return
	}
	if sources == nil {
		sources = []string{}
	}
	c.JSON(http.StatusOK, aiAgentResponse{Reply: out.Reply, Sources: sources, Steps: steps, Pending: out.Pending})
}

// aiAgentErrorMessage is the user-safe text for a failed task. The Gemini client's errors are
// written to be shown; anything else may carry details for the log only.
func aiAgentErrorMessage(err error) string {
	for _, known := range []error{gemini.ErrNotConfigured, gemini.ErrUnreachable, gemini.ErrUnreadable, gemini.ErrEmpty, gemini.ErrBlocked, gemini.ErrMalformedCall} {
		if errors.Is(err, known) {
			return known.Error()
		}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "the assistant took too long to answer, try again"
	}
	if strings.HasPrefix(err.Error(), "the AI service answered with status ") {
		// the client's own text: only the status, never what the service sent
		return err.Error()
	}
	logging.Error.Printf("The server assistant failed: %s", err)
	return "the assistant could not answer, try again"
}

// recordAgentActivity writes the changes the assistant made to the server's activity log,
// under the user who asked for them: ai.<tool> with the paths as detail. Reads are not
// recorded (they would drown the log) and only go to the debug log.
func recordAgentActivity(c *gin.Context, server *models.Server, user *models.User, steps []agent.Step) {
	db := serverActivityDB(c)
	for _, step := range steps {
		if step.Status != agent.StatusDone {
			continue
		}
		if step.Kind == toolkit.KindRead {
			logging.Debug.Printf("[%s] The assistant ran %s on %s", server.Identifier, step.Tool, strings.Join(step.Paths, ", "))
			continue
		}
		if db == nil {
			continue
		}
		entry := &models.ServerActivity{
			ServerID: server.Identifier,
			Action:   "ai." + step.Tool,
			Detail:   strings.Join(step.Paths, " → "),
			IP:       c.ClientIP(),
		}
		if user != nil {
			id := user.ID
			entry.UserID = &id
			entry.Username = user.Username
		}
		sa := &services.ServerActivity{DB: db}
		serverActivityWrite(func() {
			if err := sa.Record(entry); err != nil {
				logging.Error.Printf("Failed to record activity %s on server %s: %s", entry.Action, entry.ServerID, err.Error())
			}
		})
	}
}
