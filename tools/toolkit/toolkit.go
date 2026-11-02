// Package toolkit defines the tools an AI assistant can call on one server's files: the Tool
// contract, the kinds that decide whether a call needs the user's confirmation, and the
// Registry the agent loop picks the tools from. The tools themselves live in subpackages
// (filesystem); the files they reach come from a workspace.Workspace bound to one server.
package toolkit

import (
	"context"
	"errors"

	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/security"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/workspace"
)

// Kind says what a tool does to the server's files, which decides whether the user must
// confirm a call before it runs
type Kind string

const (
	// KindRead only looks: list, read, info, search
	KindRead Kind = "read"

	// KindCreate adds something new and never replaces what is there
	KindCreate Kind = "create"

	// KindModify changes or replaces existing data: edit, overwrite, move, rename
	KindModify Kind = "modify"

	// KindDestructive removes data
	KindDestructive Kind = "destructive"
)

// NeedsConfirmation reports whether calls of this kind wait for the user's approval
func (k Kind) NeedsConfirmation() bool {
	return k == KindModify || k == KindDestructive
}

// Scope names are plain strings, so the toolkit does not import internal/scopes
const (
	ScopeFilesView = "server.files.view"
	ScopeFilesEdit = "server.files.edit"
)

// Tool is one operation the model can call. Implementations validate their own arguments in
// both Describe and Run: Run may be called without Describe, and with arguments that
// changed since.
type Tool interface {
	// Name is the snake_case function name the model calls; unique in a Registry
	Name() string
	// Description tells the model what the tool does and when to use it
	Description() string
	// Parameters is a JSON schema object (type, properties, required) in the OpenAPI
	// subset Gemini accepts
	Parameters() map[string]any
	// Kind decides whether a call needs confirmation
	Kind() Kind
	// Scope is the permission a user needs for the tool: ScopeFilesView or ScopeFilesEdit
	Scope() string
	// Describe validates args and returns a short human summary of what the call would do,
	// for the step list and the confirmation card. It only reads the workspace (for example
	// to count occurrences) and is safe to call more than once.
	Describe(ctx context.Context, ec ExecContext, args map[string]any) (Summary, error)
	// Run performs the call
	Run(ctx context.Context, ec ExecContext, args map[string]any) (Result, error)
}

// Summary describes a call before it runs. Text is a human sentence without host paths;
// Paths are the server-relative paths involved.
type Summary struct {
	Text  string
	Paths []string
}

// ExecContext is what a tool runs against. ServerID and UserID are informational (logging);
// the Workspace is already bound to the server of the conversation.
type ExecContext struct {
	ServerID  string
	UserID    uint
	Workspace workspace.Workspace
}

// Result is what a call produced. Output is JSON-serialisable and goes back to the model
// as the function response; Summary is a human sentence for the step list; Changed lists
// the server-relative paths the call created or changed.
type Result struct {
	Output  any
	Summary string
	Changed []string
}

// Errors returned by tools are user-safe (no host paths) and wrap one of these with %w.
// Most are the workspace and security values themselves, so errors.Is matches whichever
// layer produced them.
var (
	ErrInvalidArgs = errors.New("invalid arguments")
	ErrNotFound    = workspace.ErrNotFound
	ErrExists      = workspace.ErrExists
	ErrTooLarge    = workspace.ErrTooLarge
	ErrProtected   = security.ErrProtected
	ErrOutsideRoot = security.ErrOutsideRoot
)
