package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubTool struct{ name string }

func (s stubTool) Name() string        { return s.name }
func (s stubTool) Description() string { return "does " + s.name }
func (s stubTool) Kind() toolkit.Kind  { return toolkit.KindRead }
func (s stubTool) Scope() string       { return toolkit.ScopeFilesView }

func (s stubTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"path": map[string]any{"type": "string"}},
		"required":   []string{"path"},
	}
}

func (s stubTool) Describe(_ context.Context, _ toolkit.ExecContext, _ map[string]any) (toolkit.Summary, error) {
	return toolkit.Summary{}, nil
}

func (s stubTool) Run(_ context.Context, _ toolkit.ExecContext, _ map[string]any) (toolkit.Result, error) {
	return toolkit.Result{}, nil
}

// capture starts a server that records the request and answers with body.
func capture(t *testing.T, status int, body string) (*Client, *map[string]any, *http.Header) {
	t.Helper()
	var got map[string]any
	var header http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Clone()
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return &Client{APIKey: "s3cret", Endpoint: server.URL, HTTP: server.Client(), Backoff: []time.Duration{time.Millisecond, time.Millisecond}}, &got, &header
}

func conversation() []agent.Message {
	return []agent.Message{
		{Role: agent.RoleUser, Parts: []agent.Part{{Text: "read both"}}},
		{Role: agent.RoleModel, Parts: []agent.Part{
			{Text: "Reading."},
			{Call: &agent.FunctionCall{ID: "g-1", Name: "read_file", Args: map[string]any{"path": "a"}}},
			{Call: &agent.FunctionCall{ID: agent.GeneratedIDPrefix + "1-2", Name: "read_file", Args: map[string]any{"path": "b"}}},
		}},
		{Role: agent.RoleTool, Parts: []agent.Part{
			{Response: &agent.FunctionResponse{ID: "g-1", Name: "read_file", Response: map[string]any{"content": "A"}}},
			{Response: &agent.FunctionResponse{ID: agent.GeneratedIDPrefix + "1-2", Name: "read_file", Response: map[string]any{"error": "file not found"}}},
		}},
	}
}

func TestGenerate_SendsTheRequestShape(t *testing.T) {
	c, got, header := capture(t, http.StatusOK, `{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`)
	_, err := c.Generate(context.Background(), "be careful", conversation(), []toolkit.Tool{stubTool{"read_file"}, stubTool{"list_directory"}})
	require.NoError(t, err)

	assert.Equal(t, "s3cret", header.Get("x-goog-api-key"))
	assert.Equal(t, "application/json", header.Get("Content-Type"))

	body := *got
	assert.Equal(t, map[string]any{"parts": []any{map[string]any{"text": "be careful"}}}, body["systemInstruction"])
	tools := body["tools"].([]any)
	require.Len(t, tools, 1)
	decls := tools[0].(map[string]any)["functionDeclarations"].([]any)
	require.Len(t, decls, 2)
	first := decls[0].(map[string]any)
	assert.Equal(t, "read_file", first["name"])
	assert.Equal(t, "does read_file", first["description"])
	assert.Equal(t, "object", first["parameters"].(map[string]any)["type"])
	assert.Equal(t, map[string]any{"functionCallingConfig": map[string]any{"mode": "AUTO"}}, body["toolConfig"])
	gen := body["generationConfig"].(map[string]any)
	assert.InDelta(t, 0.4, gen["temperature"], 0.0001)
	assert.InDelta(t, 2048, gen["maxOutputTokens"], 0.0001)

	contents := body["contents"].([]any)
	require.Len(t, contents, 3)
	user := contents[0].(map[string]any)
	assert.Equal(t, "user", user["role"])
	assert.Equal(t, []any{map[string]any{"text": "read both"}}, user["parts"])

	model := contents[1].(map[string]any)
	assert.Equal(t, "model", model["role"])
	modelParts := model["parts"].([]any)
	require.Len(t, modelParts, 3)
	assert.Equal(t, map[string]any{"text": "Reading."}, modelParts[0])
	assert.Equal(t, map[string]any{"functionCall": map[string]any{"id": "g-1", "name": "read_file", "args": map[string]any{"path": "a"}}}, modelParts[1])
	assert.Equal(t, map[string]any{"functionCall": map[string]any{"name": "read_file", "args": map[string]any{"path": "b"}}}, modelParts[2],
		"ids the runner made up are not sent")

	answers := contents[2].(map[string]any)
	assert.Equal(t, "user", answers["role"], "function responses go back as role user")
	answerParts := answers["parts"].([]any)
	require.Len(t, answerParts, 2)
	assert.Equal(t, map[string]any{"functionResponse": map[string]any{"id": "g-1", "name": "read_file", "response": map[string]any{"content": "A"}}}, answerParts[0])
	assert.Equal(t, map[string]any{"functionResponse": map[string]any{"name": "read_file", "response": map[string]any{"error": "file not found"}}}, answerParts[1])
}

func TestGenerate_WithoutToolsOmitsToolsAndToolConfig(t *testing.T) {
	c, got, _ := capture(t, http.StatusOK, `{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`)
	c.Temperature = 0.1
	c.MaxOutputTokens = 100
	_, err := c.Generate(context.Background(), "", conversation()[:1], nil)
	require.NoError(t, err)
	body := *got
	assert.NotContains(t, body, "tools")
	assert.NotContains(t, body, "toolConfig")
	assert.NotContains(t, body, "systemInstruction")
	gen := body["generationConfig"].(map[string]any)
	assert.InDelta(t, 0.1, gen["temperature"], 0.0001)
	assert.InDelta(t, 100, gen["maxOutputTokens"], 0.0001)
}

func TestGenerate_ParsesTextAndFunctionCalls(t *testing.T) {
	c, _, _ := capture(t, http.StatusOK, `{"candidates":[{"content":{"role":"model","parts":[
		{"text":"thinking...","thought":true},
		{"text":"I will read it."},
		{"functionCall":{"id":"abc","name":"read_file","args":{"path":"server.properties"}}},
		{"functionCall":{"name":"list_directory","args":{"dir":"plugins","depth":2}}}
	]},"finishReason":"STOP"}]}`)
	msg, err := c.Generate(context.Background(), "", conversation()[:1], []toolkit.Tool{stubTool{"read_file"}})
	require.NoError(t, err)
	assert.Equal(t, agent.RoleModel, msg.Role)
	require.Len(t, msg.Parts, 3)
	assert.Equal(t, "I will read it.", msg.Parts[0].Text)
	require.NotNil(t, msg.Parts[1].Call)
	assert.Equal(t, agent.FunctionCall{ID: "abc", Name: "read_file", Args: map[string]any{"path": "server.properties"}}, *msg.Parts[1].Call)
	require.NotNil(t, msg.Parts[2].Call)
	assert.Equal(t, "", msg.Parts[2].Call.ID)
	assert.Equal(t, "list_directory", msg.Parts[2].Call.Name)
	assert.Equal(t, map[string]any{"dir": "plugins", "depth": float64(2)}, msg.Parts[2].Call.Args)
}

func TestGenerate_ReportsFailuresSafely(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
		want   error
		text   string
	}{
		"quota":           {status: http.StatusTooManyRequests, body: `{"error":{"message":"key s3cret over quota"}}`, want: ErrQuota},
		"server error":    {status: http.StatusInternalServerError, body: `oops`, want: ErrUnavailable},
		"overloaded":      {status: http.StatusServiceUnavailable, body: `{"error":{"code":503,"message":"The model is overloaded.","status":"UNAVAILABLE"}}`, want: ErrUnavailable},
		"invalid key":     {status: http.StatusBadRequest, body: `{"error":{"code":400,"message":"API key not valid. Please pass a valid API key.","status":"INVALID_ARGUMENT","details":[{"reason":"API_KEY_INVALID"}]}}`, want: ErrKeyRejected},
		"no permission":   {status: http.StatusForbidden, body: `{"error":{"code":403,"message":"Generative Language API has not been used","status":"PERMISSION_DENIED"}}`, want: ErrKeyRejected},
		"bad request":     {status: http.StatusBadRequest, body: `{"error":{"code":400,"message":"Invalid JSON payload","status":"INVALID_ARGUMENT"}}`, want: ErrRejected},
		"too long":        {status: http.StatusOK, body: `{"candidates":[{"content":{"parts":[]},"finishReason":"MAX_TOKENS"}]}`, want: ErrTooLong},
		"bad json":        {status: http.StatusOK, body: `{"candidates":`, want: ErrUnreadable},
		"no candidates":   {status: http.StatusOK, body: `{"candidates":[]}`, want: ErrEmpty},
		"prompt blocked":  {status: http.StatusOK, body: `{"promptFeedback":{"blockReason":"SAFETY"}}`, want: ErrBlocked},
		"safety finish":   {status: http.StatusOK, body: `{"candidates":[{"content":{"parts":[]},"finishReason":"SAFETY"}]}`, want: ErrBlocked},
		"malformed call":  {status: http.StatusOK, body: `{"candidates":[{"content":{"parts":[]},"finishReason":"MALFORMED_FUNCTION_CALL"}]}`, want: ErrMalformedCall},
		"empty parts":     {status: http.StatusOK, body: `{"candidates":[{"content":{"parts":[]},"finishReason":"STOP"}]}`, want: ErrEmpty},
		"call no name":    {status: http.StatusOK, body: `{"candidates":[{"content":{"parts":[{"functionCall":{"args":{}}}]}}]}`, want: ErrMalformedCall},
		"only thoughts":   {status: http.StatusOK, body: `{"candidates":[{"content":{"parts":[{"text":"hmm","thought":true}]}}]}`, want: ErrEmpty},
		"empty text part": {status: http.StatusOK, body: `{"candidates":[{"content":{"parts":[{"text":""}]}}]}`, want: ErrEmpty},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, _, _ := capture(t, tc.status, tc.body)
			_, err := c.Generate(context.Background(), "", conversation()[:1], nil)
			require.Error(t, err)
			if tc.want != nil {
				assert.ErrorIs(t, err, tc.want)
			} else {
				assert.Equal(t, tc.text, err.Error())
			}
			assert.NotContains(t, err.Error(), "s3cret")
		})
	}
}

func TestGenerate_NeedsAKeyAndHandlesUnreachableService(t *testing.T) {
	_, err := (&Client{}).Generate(context.Background(), "", nil, nil)
	assert.ErrorIs(t, err, ErrNotConfigured)

	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()
	_, err = (&Client{APIKey: "s3cret", Endpoint: url, Backoff: []time.Duration{time.Millisecond}}).Generate(context.Background(), "", conversation()[:1], nil)
	assert.ErrorIs(t, err, ErrUnreachable)
	assert.False(t, strings.Contains(err.Error(), url), "the endpoint is not leaked")
}

func TestGenerate_LimitsTheResponseBody(t *testing.T) {
	huge := `{"candidates":[{"content":{"parts":[{"text":"` + strings.Repeat("a", maxResponseBytes) + `"}]}}]}`
	c, _, _ := capture(t, http.StatusOK, huge)
	_, err := c.Generate(context.Background(), "", conversation()[:1], nil)
	assert.ErrorIs(t, err, ErrUnreadable)
}

func TestGenerate_ServiceErrorKeepsTheCauseForTheLog(t *testing.T) {
	c, _, _ := capture(t, http.StatusTooManyRequests, `{"error":{"code":429,"message":"Quota exceeded for metric generate_content_free_tier_requests","status":"RESOURCE_EXHAUSTED","details":[{"retryDelay":"45s"}]}}`)
	_, err := c.Generate(context.Background(), "", conversation()[:1], nil)
	var service *ServiceError
	require.ErrorAs(t, err, &service)
	assert.Equal(t, http.StatusTooManyRequests, service.StatusCode)
	assert.Equal(t, 45*time.Second, service.RetryAfter)
	assert.Contains(t, service.Detail, "RESOURCE_EXHAUSTED")
	assert.Contains(t, service.Detail, "Quota exceeded")
	assert.Equal(t, ErrQuota.Error(), err.Error(), "the user only sees the safe text")
}

// sequence answers with the given statuses in turn, then 200
func sequence(t *testing.T, statuses ...int) (*Client, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := int(calls.Add(1))
		if n <= len(statuses) {
			w.WriteHeader(statuses[n-1])
			if statuses[n-1] == http.StatusTooManyRequests {
				_, _ = w.Write([]byte(`{"error":{"code":429,"status":"RESOURCE_EXHAUSTED","details":[{"retryDelay":"0.001s"}]}}`))
			}
			return
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`))
	}))
	t.Cleanup(server.Close)
	return &Client{APIKey: "k", Endpoint: server.URL, Backoff: []time.Duration{time.Millisecond, time.Millisecond}}, &calls
}

func TestGenerate_RetriesWhileTheServiceIsBusy(t *testing.T) {
	c, calls := sequence(t, http.StatusServiceUnavailable, http.StatusInternalServerError)
	msg, err := c.Generate(context.Background(), "", conversation()[:1], nil)
	require.NoError(t, err)
	assert.Equal(t, "ok", msg.Parts[0].Text)
	assert.Equal(t, int32(3), calls.Load())

	c, calls = sequence(t, http.StatusServiceUnavailable, http.StatusServiceUnavailable, http.StatusServiceUnavailable)
	_, err = c.Generate(context.Background(), "", conversation()[:1], nil)
	assert.ErrorIs(t, err, ErrUnavailable, "three attempts in all")
	assert.Equal(t, int32(3), calls.Load())

	c, calls = sequence(t, http.StatusTooManyRequests)
	_, err = c.Generate(context.Background(), "", conversation()[:1], nil)
	require.NoError(t, err, "a short per-minute limit is waited out")
	assert.Equal(t, int32(2), calls.Load())
}

func TestGenerate_DoesNotRetryWhatCannotGetBetter(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound} {
		c, calls := sequence(t, status)
		_, err := c.Generate(context.Background(), "", conversation()[:1], nil)
		require.Error(t, err)
		assert.Equal(t, int32(1), calls.Load(), "status %d", status)
	}
	c, _, _ := capture(t, http.StatusTooManyRequests, `{"error":{"details":[{"retryDelay":"3600s"}]}}`)
	start := time.Now()
	_, err := c.Generate(context.Background(), "", conversation()[:1], nil)
	assert.ErrorIs(t, err, ErrQuota)
	assert.Less(t, time.Since(start), time.Second, "a daily limit is reported at once")
}

func TestGenerate_StopsRetryingWhenTheTaskEnds(t *testing.T) {
	c, _ := sequence(t, http.StatusServiceUnavailable, http.StatusServiceUnavailable)
	c.Backoff = []time.Duration{time.Hour, time.Hour}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := c.Generate(ctx, "", conversation()[:1], nil)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
}

func TestGenerate_CapsThinkingAndKeepsThoughtSignatures(t *testing.T) {
	c, got, _ := capture(t, http.StatusOK, `{"candidates":[{"content":{"parts":[
		{"functionCall":{"name":"read_file","args":{"path":"a"}},"thoughtSignature":"c2ln"}
	]}}]}`)
	c.ThinkingBudget = 1024
	msg, err := c.Generate(context.Background(), "", conversation()[:1], []toolkit.Tool{stubTool{"read_file"}})
	require.NoError(t, err)
	gen := (*got)["generationConfig"].(map[string]any)
	assert.Equal(t, map[string]any{"thinkingBudget": float64(1024)}, gen["thinkingConfig"])
	require.Len(t, msg.Parts, 1)
	assert.Equal(t, "c2ln", msg.Parts[0].Signature)

	// the signature goes back with the call in the next request
	history := []agent.Message{conversation()[0], msg}
	_, err = c.Generate(context.Background(), "", history, nil)
	require.NoError(t, err)
	model := (*got)["contents"].([]any)[1].(map[string]any)
	assert.Equal(t, "c2ln", model["parts"].([]any)[0].(map[string]any)["thoughtSignature"])
}
