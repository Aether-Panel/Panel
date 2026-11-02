package toolkit

import (
	"fmt"
	"sort"
	"sync"
)

// Registry holds the tools by name. It is filled once at start-up and read by every
// conversation, so lookups are safe from many goroutines.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry returns an empty registry
func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

// Register adds a tool. Two tools with one name would make the model's call ambiguous, so a
// duplicate (or a nameless or nil tool) is a programming error and panics.
func (r *Registry) Register(t Tool) {
	if t == nil {
		panic("toolkit: Register called with a nil tool")
	}
	name := t.Name()
	if name == "" {
		panic("toolkit: Register called with a tool without a name")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[name]; exists {
		panic(fmt.Sprintf("toolkit: a tool named %q is already registered", name))
	}
	r.tools[name] = t
}

// Get returns the tool with that name
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// Available returns the tools whose Scope is among scopes, sorted by name. Tools the user
// has no permission for are left out, so they are never offered to the model.
func (r *Registry) Available(scopes []string) []Tool {
	allowed := make(map[string]bool, len(scopes))
	for _, s := range scopes {
		allowed[s] = true
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	tools := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		if allowed[t.Scope()] {
			tools = append(tools, t)
		}
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name() < tools[j].Name() })
	return tools
}
