// Package gemini is the Gemini function-calling client of the toolkit. It talks
// to the REST generateContent endpoint directly (no SDK) and implements
// agent.Generator.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/agent"
)

// DefaultEndpoint is the generateContent endpoint used when Client.Endpoint is empty.
const DefaultEndpoint = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent"

const (
	defaultTemperature     = 0.4
	defaultMaxOutputTokens = 2048
)

// maxResponseBytes caps the body read from the service.
const maxResponseBytes = 4 << 20

// maxErrorBytes caps the body read from a failed answer, and maxDetail the part of its
// message kept for the log.
const (
	maxErrorBytes = 64 << 10
	maxDetail     = 300
)

// defaultBackoff is how long Generate waits before each new attempt when the service is
// busy or could not be reached: three attempts in all.
var defaultBackoff = []time.Duration{time.Second, 3 * time.Second}

// maxRetryAfter is the longest wait the service may ask for (on a 429) that Generate still
// waits out; a longer one is reported at once instead of keeping the user waiting.
const maxRetryAfter = 10 * time.Second

// defaultHTTP is used when Client.HTTP is nil. The timeout is a backstop; the
// runner's context normally ends the request earlier.
var defaultHTTP = &http.Client{Timeout: 90 * time.Second}

// Errors returned by Generate. Their messages are safe to show to users: they
// never contain the API key, the endpoint or the service's own error text.
var (
	ErrNotConfigured = errors.New("the AI service is not configured")
	ErrUnreachable   = errors.New("could not reach the AI service")
	ErrUnreadable    = errors.New("the AI service sent an answer that could not be read")
	ErrEmpty         = errors.New("the AI service sent an empty answer")
	ErrBlocked       = errors.New("the AI service refused to answer this request")
	ErrMalformedCall = errors.New("the AI service sent an operation that could not be read")
	ErrTooLong       = errors.New("the answer was too long for the AI service; ask for a smaller part")

	// Kinds of a ServiceError, the service's answer to a request it did not accept.
	ErrKeyRejected = errors.New("the AI service rejected the API key; an administrator must check it in Settings")
	ErrQuota       = errors.New("the API key reached its usage limit on the AI service; wait a moment or check its plan")
	ErrUnavailable = errors.New("the AI service is overloaded or unavailable right now")
	ErrRejected    = errors.New("the AI service rejected the request")
)

// ServiceError is an answer of the service other than 200. Error is the safe text of its
// Kind (one of ErrKeyRejected, ErrQuota, ErrUnavailable, ErrRejected); Detail carries the
// service's own status and message, for the panel's log only.
type ServiceError struct {
	Kind       error
	StatusCode int
	Detail     string
	// RetryAfter is the wait the service asked for, when it said one
	RetryAfter time.Duration
}

func (e *ServiceError) Error() string { return e.Kind.Error() }
func (e *ServiceError) Unwrap() error { return e.Kind }

// Client calls Gemini with function calling.
type Client struct {
	APIKey          string
	Endpoint        string       // DefaultEndpoint when empty
	HTTP            *http.Client // a client with a 90 s timeout when nil
	Temperature     float64      // 0.4 when zero
	MaxOutputTokens int          // 2048 when zero
	// ThinkingBudget caps the tokens the model spends thinking, which Gemini 2.5 takes out
	// of MaxOutputTokens: without a cap, thinking can leave no room for the answer. Zero
	// leaves the model's own default.
	ThinkingBudget int
	// Backoff holds the waits before each new attempt when the service is busy or could
	// not be reached; nil uses one second, then three.
	Backoff []time.Duration
}

var _ agent.Generator = (*Client)(nil)

// Wire types of the REST API (only the fields used here).

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type part struct {
	Text             string            `json:"text,omitempty"`
	Thought          bool              `json:"thought,omitempty"`
	ThoughtSignature string            `json:"thoughtSignature,omitempty"`
	FunctionCall     *functionCall     `json:"functionCall,omitempty"`
	FunctionResponse *functionResponse `json:"functionResponse,omitempty"`
}

type functionCall struct {
	ID   string         `json:"id,omitempty"`
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type functionResponse struct {
	ID       string         `json:"id,omitempty"`
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type functionDeclaration struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type toolDeclarations struct {
	FunctionDeclarations []functionDeclaration `json:"functionDeclarations"`
}

type toolConfig struct {
	FunctionCallingConfig struct {
		Mode string `json:"mode"`
	} `json:"functionCallingConfig"`
}

type thinkingConfig struct {
	ThinkingBudget int `json:"thinkingBudget"`
}

type generationConfig struct {
	Temperature     float64         `json:"temperature"`
	MaxOutputTokens int             `json:"maxOutputTokens"`
	ThinkingConfig  *thinkingConfig `json:"thinkingConfig,omitempty"`
}

type request struct {
	SystemInstruction *content           `json:"systemInstruction,omitempty"`
	Contents          []content          `json:"contents"`
	Tools             []toolDeclarations `json:"tools,omitempty"`
	ToolConfig        *toolConfig        `json:"toolConfig,omitempty"`
	GenerationConfig  generationConfig   `json:"generationConfig"`
}

type response struct {
	Candidates []struct {
		Content      content `json:"content"`
		FinishReason string  `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
}

// Generate sends the conversation and the tools and returns the model's message. When the
// service is busy (500, 502, 503, 504), cannot be reached, or asks to wait a few seconds
// (429 with a short retry delay), it tries again, up to three attempts in all.
func (c *Client) Generate(ctx context.Context, system string, history []agent.Message, tools []toolkit.Tool) (agent.Message, error) {
	if c.APIKey == "" {
		return agent.Message{}, ErrNotConfigured
	}
	payload, err := json.Marshal(c.buildRequest(system, history, tools))
	if err != nil {
		return agent.Message{}, errors.New("the request for the AI service could not be built")
	}
	backoff := c.Backoff
	if backoff == nil {
		backoff = defaultBackoff
	}
	for attempt := 0; ; attempt++ {
		msg, err := c.send(ctx, payload)
		wait, again := retryWait(err, attempt, backoff)
		if !again {
			return msg, err
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return agent.Message{}, ctx.Err()
		case <-timer.C:
		}
	}
}

// retryWait says whether a failed attempt is tried again, and after how long.
func retryWait(err error, attempt int, backoff []time.Duration) (time.Duration, bool) {
	if err == nil || attempt >= len(backoff) {
		return 0, false
	}
	wait := backoff[attempt]
	var service *ServiceError
	switch {
	case errors.Is(err, ErrUnavailable), errors.Is(err, ErrUnreachable):
		return wait, true
	case errors.As(err, &service) && errors.Is(err, ErrQuota):
		// a per-minute limit that frees up in a few seconds is worth waiting for; a daily one is not
		if service.RetryAfter <= 0 || service.RetryAfter > maxRetryAfter {
			return 0, false
		}
		return max(wait, service.RetryAfter), true
	}
	return 0, false
}

// send makes one attempt.
func (c *Client) send(ctx context.Context, payload []byte) (agent.Message, error) {
	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return agent.Message{}, ErrNotConfigured
	}
	req.Header.Set("Content-Type", "application/json")
	// The key goes in a header, never in the URL, so it cannot end up in logs.
	req.Header.Set("x-goog-api-key", c.APIKey)
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = defaultHTTP
	}
	res, err := httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return agent.Message{}, ctx.Err()
		}
		return agent.Message{}, ErrUnreachable
	}
	defer func() { _ = res.Body.Close() }()
	if err := CheckResponse(res); err != nil {
		return agent.Message{}, err
	}
	var decoded response
	if err := json.NewDecoder(io.LimitReader(res.Body, maxResponseBytes)).Decode(&decoded); err != nil {
		return agent.Message{}, ErrUnreadable
	}
	return parseResponse(decoded)
}

// serviceErrorBody is the error Google APIs answer with.
type serviceErrorBody struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
		Details []struct {
			Reason     string `json:"reason"`
			RetryDelay string `json:"retryDelay"`
		} `json:"details"`
	} `json:"error"`
}

// CheckResponse returns nil for a 200 answer and a *ServiceError for any other, classified
// by what went wrong. It reads the body of a failed answer, never of a good one.
func CheckResponse(res *http.Response) error {
	if res.StatusCode == http.StatusOK {
		return nil
	}
	var body serviceErrorBody
	_ = json.NewDecoder(io.LimitReader(res.Body, maxErrorBytes)).Decode(&body)
	reason := ""
	var retryAfter time.Duration
	for _, d := range body.Error.Details {
		if d.Reason != "" && reason == "" {
			reason = d.Reason
		}
		if wait, err := time.ParseDuration(d.RetryDelay); err == nil && wait > 0 {
			retryAfter = wait
		}
	}
	return &ServiceError{
		Kind:       serviceErrorKind(res.StatusCode, reason, body.Error.Message),
		StatusCode: res.StatusCode,
		Detail:     serviceErrorDetail(res.StatusCode, body.Error.Status, reason, body.Error.Message),
		RetryAfter: retryAfter,
	}
}

func serviceErrorKind(status int, reason, message string) error {
	switch {
	case status == http.StatusUnauthorized, status == http.StatusForbidden,
		strings.HasPrefix(reason, "API_KEY_"), strings.Contains(strings.ToLower(message), "api key"):
		return ErrKeyRejected
	case status == http.StatusTooManyRequests:
		return ErrQuota
	case status >= http.StatusInternalServerError:
		return ErrUnavailable
	default:
		return ErrRejected
	}
}

func serviceErrorDetail(status int, code, reason, message string) string {
	detail := fmt.Sprintf("status %d", status)
	for _, extra := range []string{code, reason} {
		if extra != "" {
			detail += " " + extra
		}
	}
	message = strings.Join(strings.Fields(message), " ")
	if len(message) > maxDetail {
		message = message[:maxDetail] + "…"
	}
	if message != "" {
		detail += ": " + message
	}
	return detail
}

func (c *Client) buildRequest(system string, history []agent.Message, tools []toolkit.Tool) request {
	req := request{Contents: make([]content, 0, len(history))}
	if system != "" {
		req.SystemInstruction = &content{Parts: []part{{Text: system}}}
	}
	for _, msg := range history {
		if converted, ok := toContent(msg); ok {
			req.Contents = append(req.Contents, converted)
		}
	}
	if len(tools) > 0 {
		decls := make([]functionDeclaration, 0, len(tools))
		for _, t := range tools {
			decls = append(decls, functionDeclaration{Name: t.Name(), Description: t.Description(), Parameters: t.Parameters()})
		}
		req.Tools = []toolDeclarations{{FunctionDeclarations: decls}}
		req.ToolConfig = &toolConfig{}
		req.ToolConfig.FunctionCallingConfig.Mode = "AUTO"
	}
	req.GenerationConfig = generationConfig{Temperature: c.Temperature, MaxOutputTokens: c.MaxOutputTokens}
	if req.GenerationConfig.Temperature == 0 {
		req.GenerationConfig.Temperature = defaultTemperature
	}
	if req.GenerationConfig.MaxOutputTokens <= 0 {
		req.GenerationConfig.MaxOutputTokens = defaultMaxOutputTokens
	}
	if c.ThinkingBudget > 0 {
		req.GenerationConfig.ThinkingConfig = &thinkingConfig{ThinkingBudget: c.ThinkingBudget}
	}
	return req
}

// toContent maps one agent message to a Gemini content. Model messages keep
// their function calls; tool answers go back as role "user" functionResponse
// parts, in call order (the Gemini REST convention).
func toContent(msg agent.Message) (content, bool) {
	role := "user"
	if msg.Role == agent.RoleModel {
		role = "model"
	}
	out := content{Role: role}
	for _, p := range msg.Parts {
		switch {
		case p.Call != nil:
			args := p.Call.Args
			if args == nil {
				args = map[string]any{}
			}
			out.Parts = append(out.Parts, part{FunctionCall: &functionCall{ID: modelID(p.Call.ID), Name: p.Call.Name, Args: args}, ThoughtSignature: p.Signature})
		case p.Response != nil:
			resp := p.Response.Response
			if resp == nil {
				resp = map[string]any{}
			}
			out.Parts = append(out.Parts, part{FunctionResponse: &functionResponse{ID: modelID(p.Response.ID), Name: p.Response.Name, Response: resp}})
		case p.Text != "" || p.Signature != "":
			out.Parts = append(out.Parts, part{Text: p.Text, ThoughtSignature: p.Signature})
		}
	}
	return out, len(out.Parts) > 0
}

// modelID keeps only the IDs Gemini itself produced; the ones the runner made
// up are not sent.
func modelID(id string) string {
	if agent.IsGeneratedID(id) {
		return ""
	}
	return id
}

func parseResponse(res response) (agent.Message, error) {
	if len(res.Candidates) == 0 {
		if res.PromptFeedback.BlockReason != "" {
			return agent.Message{}, ErrBlocked
		}
		return agent.Message{}, ErrEmpty
	}
	candidate := res.Candidates[0]
	switch candidate.FinishReason {
	case "SAFETY", "RECITATION", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII", "IMAGE_SAFETY":
		return agent.Message{}, ErrBlocked
	case "MALFORMED_FUNCTION_CALL", "UNEXPECTED_TOOL_CALL":
		return agent.Message{}, ErrMalformedCall
	}
	msg := agent.Message{Role: agent.RoleModel}
	useful := false
	for _, p := range candidate.Content.Parts {
		switch {
		case p.FunctionCall != nil:
			if p.FunctionCall.Name == "" {
				return agent.Message{}, ErrMalformedCall
			}
			msg.Parts = append(msg.Parts, agent.Part{Call: &agent.FunctionCall{
				ID:   p.FunctionCall.ID,
				Name: p.FunctionCall.Name,
				Args: p.FunctionCall.Args,
			}, Signature: p.ThoughtSignature})
			useful = true
		case p.Thought:
			// Thinking summaries are not part of the answer.
		case p.Text != "" || p.ThoughtSignature != "":
			// a signature goes back with the history, so the model keeps its reasoning between rounds
			msg.Parts = append(msg.Parts, agent.Part{Text: p.Text, Signature: p.ThoughtSignature})
			useful = useful || strings.TrimSpace(p.Text) != ""
		}
	}
	if !useful {
		if candidate.FinishReason == "MAX_TOKENS" {
			return agent.Message{}, ErrTooLong
		}
		return agent.Message{}, ErrEmpty
	}
	return msg, nil
}
