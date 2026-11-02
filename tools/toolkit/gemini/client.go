// Package gemini is the Gemini function-calling client of the toolkit. It talks
// to the REST generateContent endpoint directly (no SDK) and implements
// agent.Model.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
)

// Client calls Gemini with function calling.
type Client struct {
	APIKey          string
	Endpoint        string       // DefaultEndpoint when empty
	HTTP            *http.Client // a client with a 90 s timeout when nil
	Temperature     float64      // 0.4 when zero
	MaxOutputTokens int          // 2048 when zero
}

var _ agent.Model = (*Client)(nil)

// Wire types of the REST API (only the fields used here).

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type part struct {
	Text             string            `json:"text,omitempty"`
	Thought          bool              `json:"thought,omitempty"`
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

type generationConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
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

// Generate sends the conversation and the tools and returns the model's message.
func (c *Client) Generate(ctx context.Context, system string, history []agent.Message, tools []toolkit.Tool) (agent.Message, error) {
	if c.APIKey == "" {
		return agent.Message{}, ErrNotConfigured
	}
	payload, err := json.Marshal(c.buildRequest(system, history, tools))
	if err != nil {
		return agent.Message{}, errors.New("the request for the AI service could not be built")
	}
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
	if res.StatusCode != http.StatusOK {
		// Only the status: the body may echo the request.
		return agent.Message{}, fmt.Errorf("the AI service answered with status %d", res.StatusCode)
	}
	var decoded response
	if err := json.NewDecoder(io.LimitReader(res.Body, maxResponseBytes)).Decode(&decoded); err != nil {
		return agent.Message{}, ErrUnreadable
	}
	return parseResponse(decoded)
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
			out.Parts = append(out.Parts, part{FunctionCall: &functionCall{ID: modelID(p.Call.ID), Name: p.Call.Name, Args: args}})
		case p.Response != nil:
			resp := p.Response.Response
			if resp == nil {
				resp = map[string]any{}
			}
			out.Parts = append(out.Parts, part{FunctionResponse: &functionResponse{ID: modelID(p.Response.ID), Name: p.Response.Name, Response: resp}})
		case p.Text != "":
			out.Parts = append(out.Parts, part{Text: p.Text})
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
			}})
		case p.Thought:
			// Thinking summaries are not part of the answer.
		case p.Text != "":
			msg.Parts = append(msg.Parts, agent.Part{Text: p.Text})
		}
	}
	if len(msg.Parts) == 0 {
		return agent.Message{}, ErrEmpty
	}
	return msg, nil
}
