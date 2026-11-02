// Package agent runs the function-calling loop between a language model and the
// toolkit's tools: it offers the tools the user may use, runs reads and creates
// straight away, and stops for the user's confirmation before anything that
// changes or removes existing data.
package agent

import (
	"context"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit"
)

// Message roles. RoleTool holds the answers to the model's function calls; the
// model client decides how to send them (Gemini wants them as role "user").
const (
	RoleUser  = "user"
	RoleModel = "model"
	RoleTool  = "tool"
)

// Step statuses.
const (
	StatusDone     = "done"
	StatusFailed   = "failed"
	StatusPending  = "pending"
	StatusDeclined = "declined"
	StatusRefused  = "refused"
)

// Message is one turn of the conversation, in a model-neutral shape.
type Message struct {
	Role  string // "user" | "model" | "tool"
	Parts []Part
}

// Part is one piece of a message: text, a function call from the model or the
// answer to one.
type Part struct {
	Text     string            `json:",omitempty"`
	Call     *FunctionCall     `json:",omitempty"` // from the model
	Response *FunctionResponse `json:",omitempty"` // to the model
}

// FunctionCall is a tool call requested by the model.
type FunctionCall struct {
	ID   string
	Name string
	Args map[string]any
}

// FunctionResponse is the answer to a FunctionCall with the same ID.
type FunctionResponse struct {
	ID       string
	Name     string
	Response map[string]any
}

// Generator is the language model: it generates the next message of the conversation.
type Generator interface {
	Generate(ctx context.Context, system string, history []Message, tools []toolkit.Tool) (Message, error)
}

// Step is one tool call as shown to the user.
type Step struct {
	ID      string       `json:"id"`
	Tool    string       `json:"tool"`
	Kind    toolkit.Kind `json:"kind"`
	Summary string       `json:"summary"`
	Paths   []string     `json:"paths,omitempty"`
	Status  string       `json:"status"` // "done" | "failed" | "pending" | "declined" | "refused"
	Error   string       `json:"error,omitempty"`
}

// Limits bound one task so a confused model cannot loop forever or touch too much.
type Limits struct {
	MaxRounds        int
	MaxCallsPerRound int
	MaxCalls         int
	Timeout          time.Duration
}

// DefaultLimits are the limits used when a Runner leaves a field at zero.
func DefaultLimits() Limits {
	return Limits{MaxRounds: 10, MaxCallsPerRound: 8, MaxCalls: 30, Timeout: 120 * time.Second}
}

// withDefaults fills the zero fields with the defaults.
func (l Limits) withDefaults() Limits {
	d := DefaultLimits()
	if l.MaxRounds <= 0 {
		l.MaxRounds = d.MaxRounds
	}
	if l.MaxCallsPerRound <= 0 {
		l.MaxCallsPerRound = d.MaxCallsPerRound
	}
	if l.MaxCalls <= 0 {
		l.MaxCalls = d.MaxCalls
	}
	if l.Timeout <= 0 {
		l.Timeout = d.Timeout
	}
	return l
}

// GeneratedIDPrefix marks the call IDs the runner made up because the model sent
// none. A model client must not send these back to its API as if the model had
// produced them.
const GeneratedIDPrefix = "local-"

// IsGeneratedID reports whether id was made up by the runner.
func IsGeneratedID(id string) bool {
	return strings.HasPrefix(id, GeneratedIDPrefix)
}
