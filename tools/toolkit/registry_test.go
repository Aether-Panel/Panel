package toolkit

import (
	"context"
	"errors"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/security"
	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubTool struct {
	name  string
	kind  Kind
	scope string
}

func (s stubTool) Name() string               { return s.name }
func (s stubTool) Description() string        { return "stub" }
func (s stubTool) Parameters() map[string]any { return map[string]any{"type": "object"} }
func (s stubTool) Kind() Kind                 { return s.kind }
func (s stubTool) Scope() string              { return s.scope }

func (s stubTool) Describe(_ context.Context, _ ExecContext, _ map[string]any) (Summary, error) {
	return Summary{Text: s.name}, nil
}

func (s stubTool) Run(_ context.Context, _ ExecContext, _ map[string]any) (Result, error) {
	return Result{Summary: s.name}, nil
}

func TestKindNeedsConfirmation(t *testing.T) {
	assert.False(t, KindRead.NeedsConfirmation())
	assert.False(t, KindCreate.NeedsConfirmation())
	assert.True(t, KindModify.NeedsConfirmation())
	assert.True(t, KindDestructive.NeedsConfirmation())
}

func TestRegistry_GetAndAvailable(t *testing.T) {
	r := NewRegistry()
	r.Register(stubTool{name: "write_file", kind: KindModify, scope: ScopeFilesEdit})
	r.Register(stubTool{name: "read_file", kind: KindRead, scope: ScopeFilesView})
	r.Register(stubTool{name: "list_directory", kind: KindRead, scope: ScopeFilesView})
	r.Register(stubTool{name: "delete_path", kind: KindDestructive, scope: ScopeFilesEdit})

	tool, ok := r.Get("read_file")
	require.True(t, ok)
	assert.Equal(t, "read_file", tool.Name())
	_, ok = r.Get("run_command")
	assert.False(t, ok)

	names := func(tools []Tool) []string {
		out := []string{}
		for _, tool := range tools {
			out = append(out, tool.Name())
		}
		return out
	}
	assert.Equal(t, []string{"list_directory", "read_file"}, names(r.Available([]string{ScopeFilesView})))
	assert.Equal(t, []string{"delete_path", "list_directory", "read_file", "write_file"}, names(r.Available([]string{ScopeFilesEdit, ScopeFilesView, "server.console"})))
	assert.Equal(t, []string{"delete_path", "write_file"}, names(r.Available([]string{ScopeFilesEdit})), "edit does not imply view")
	assert.Empty(t, r.Available(nil))
}

func TestRegistry_RegisterPanicsOnMistakes(t *testing.T) {
	r := NewRegistry()
	r.Register(stubTool{name: "read_file", scope: ScopeFilesView})
	assert.Panics(t, func() { r.Register(stubTool{name: "read_file", scope: ScopeFilesEdit}) })
	assert.Panics(t, func() { r.Register(stubTool{name: ""}) })
	assert.Panics(t, func() { r.Register(nil) })
}

func TestErrorsAreSharedWithTheLowerLayers(t *testing.T) {
	assert.True(t, errors.Is(ErrNotFound, workspace.ErrNotFound))
	assert.True(t, errors.Is(ErrExists, workspace.ErrExists))
	assert.True(t, errors.Is(ErrTooLarge, workspace.ErrTooLarge))
	assert.True(t, errors.Is(ErrOutsideRoot, security.ErrOutsideRoot))
	assert.True(t, errors.Is(ErrProtected, security.ErrProtected))
	assert.True(t, errors.Is(ErrOutsideRoot, workspace.ErrOutsideRoot))
}
