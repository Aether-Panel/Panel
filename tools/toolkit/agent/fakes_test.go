package agent

import (
	"context"
	"errors"
	"sync"

	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit"
)

// fakeTool is a tool that only records what the runner asked of it.
type fakeTool struct {
	name        string
	kind        toolkit.Kind
	scope       string
	describeErr error
	runErr      error
	output      any

	mu        sync.Mutex
	describes int
	runs      []map[string]any
}

func (f *fakeTool) Name() string        { return f.name }
func (f *fakeTool) Description() string { return "fake " + f.name }
func (f *fakeTool) Kind() toolkit.Kind  { return f.kind }
func (f *fakeTool) Scope() string       { return f.scope }

func (f *fakeTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"path": map[string]any{"type": "string"}},
		"required":   []string{"path"},
	}
}

func (f *fakeTool) Describe(_ context.Context, _ toolkit.ExecContext, args map[string]any) (toolkit.Summary, error) {
	f.mu.Lock()
	f.describes++
	f.mu.Unlock()
	if f.describeErr != nil {
		return toolkit.Summary{}, f.describeErr
	}
	p, ok := args["path"].(string)
	if !ok || p == "" {
		return toolkit.Summary{}, errors.New("path is required")
	}
	return toolkit.Summary{Text: f.name + " " + p, Paths: []string{p}}, nil
}

func (f *fakeTool) Run(_ context.Context, _ toolkit.ExecContext, args map[string]any) (toolkit.Result, error) {
	f.mu.Lock()
	f.runs = append(f.runs, args)
	f.mu.Unlock()
	if f.runErr != nil {
		return toolkit.Result{}, f.runErr
	}
	p, _ := args["path"].(string)
	out := f.output
	if out == nil {
		out = map[string]any{"path": p}
	}
	return toolkit.Result{Output: out, Summary: "ran " + f.name + " " + p, Changed: []string{p}}, nil
}

func (f *fakeTool) runCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.runs)
}

func (f *fakeTool) describeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.describes
}

// kit is a registry with one fake tool of each kind.
type kit struct {
	reg    *toolkit.Registry
	read   *fakeTool
	create *fakeTool
	edit   *fakeTool
	delete *fakeTool
}

func newKit() *kit {
	k := &kit{
		reg:    toolkit.NewRegistry(),
		read:   &fakeTool{name: "read_file", kind: toolkit.KindRead, scope: toolkit.ScopeFilesView},
		create: &fakeTool{name: "create_file", kind: toolkit.KindCreate, scope: toolkit.ScopeFilesEdit},
		edit:   &fakeTool{name: "edit_file", kind: toolkit.KindModify, scope: toolkit.ScopeFilesEdit},
		delete: &fakeTool{name: "delete_path", kind: toolkit.KindDestructive, scope: toolkit.ScopeFilesEdit},
	}
	for _, t := range []*fakeTool{k.read, k.create, k.edit, k.delete} {
		k.reg.Register(t)
	}
	return k
}

// scriptedModel answers with turns in order and records what it was sent.
type scriptedModel struct {
	mu        sync.Mutex
	turns     []Message
	repeat    *Message // answered once the turns run out, when set
	histories [][]Message
	offered   [][]string
}

func (m *scriptedModel) Generate(_ context.Context, _ string, history []Message, tools []toolkit.Tool) (Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.histories = append(m.histories, append([]Message(nil), history...))
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name())
	}
	m.offered = append(m.offered, names)
	n := len(m.histories) - 1
	if n < len(m.turns) {
		return copyMessage(m.turns[n]), nil
	}
	if m.repeat != nil {
		return copyMessage(*m.repeat), nil
	}
	return Message{}, errors.New("the script has no more turns")
}

func (m *scriptedModel) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.histories)
}

func (m *scriptedModel) lastHistory() []Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.histories[len(m.histories)-1]
}

// copyMessage returns msg with fresh call structs, as a real client would.
func copyMessage(msg Message) Message {
	out := Message{Role: msg.Role, Parts: make([]Part, len(msg.Parts))}
	for i, p := range msg.Parts {
		out.Parts[i] = p
		if p.Call != nil {
			c := *p.Call
			out.Parts[i].Call = &c
		}
	}
	return out
}

// blockingModel waits until the context ends.
type blockingModel struct{}

func (blockingModel) Generate(ctx context.Context, _ string, _ []Message, _ []toolkit.Tool) (Message, error) {
	<-ctx.Done()
	return Message{}, ctx.Err()
}

func textMsg(s string) Message {
	return Message{Role: RoleModel, Parts: []Part{{Text: s}}}
}

func callsMsg(cs ...FunctionCall) Message {
	msg := Message{Role: RoleModel}
	for i := range cs {
		c := cs[i]
		msg.Parts = append(msg.Parts, Part{Call: &c})
	}
	return msg
}

func fcall(name, path string) FunctionCall {
	return FunctionCall{Name: name, Args: map[string]any{"path": path}}
}

// toolResponses returns the function responses of the last tool message in history.
func toolResponses(history []Message) []*FunctionResponse {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role != RoleTool {
			continue
		}
		var out []*FunctionResponse
		for _, p := range history[i].Parts {
			out = append(out, p.Response)
		}
		return out
	}
	return nil
}
