// Package toolkit defines the core Toolkit interface, Tool type, and Registry
// for managing MCP tool registration, lookup, and scope-based filtering.
package toolkit

import (
	"context"
	"sort"
	"sync"
)

// ToolHandler is the function signature for executing a tool.
type ToolHandler func(ctx context.Context, params map[string]interface{}) (interface{}, error)

// Tool represents a single MCP tool with its metadata and handler.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
	Handler     ToolHandler
}

// Toolkit is the interface that groups related tools together.
type Toolkit interface {
	// Name returns the toolkit's name (e.g. "paas", "iaas", "shared").
	Name() string
	// Tools returns all tools provided by this toolkit.
	Tools() []Tool
}

// Scope constants for toolkit filtering.
const (
	ScopeAll  = "all"
	ScopePaaS = "paas"
	ScopeIaaS = "iaas"
)

// Registry is a thread-safe tool registration center that supports
// registration, lookup, and listing of tools.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds all tools from the given Toolkit into the registry.
// If a tool name conflicts with an existing entry, the new tool overwrites it.
func (r *Registry) Register(tk Toolkit) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range tk.Tools() {
		r.tools[t.Name] = t
	}
}

// Get returns the tool with the given name and true, or a zero Tool and false
// if no tool with that name is registered.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tools sorted by name for deterministic output.
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

// RegisterToolkits registers toolkits into the registry based on the given scope.
// If enabledTools is non-empty, only the listed tools are kept after registration
// (scope is used to determine which toolkits to load, then the list filters further).
//   - "paas": registers paas + shared
//   - "iaas": registers iaas + shared
//   - "all" or "": registers paas + iaas + shared
func RegisterToolkits(r *Registry, scope string, enabledTools []string, paas, iaas, shared Toolkit) {
	switch scope {
	case ScopePaaS:
		r.Register(paas)
		r.Register(shared)
	case ScopeIaaS:
		r.Register(iaas)
		r.Register(shared)
	default: // "all" or empty
		r.Register(paas)
		r.Register(iaas)
		r.Register(shared)
	}

	if len(enabledTools) > 0 {
		r.FilterByNames(enabledTools)
	}
}

// FilterByNames removes all tools from the registry whose names are not in
// the given allow-list.
func (r *Registry) FilterByNames(names []string) {
	allowed := make(map[string]struct{}, len(names))
	for _, n := range names {
		allowed[n] = struct{}{}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for name := range r.tools {
		if _, ok := allowed[name]; !ok {
			delete(r.tools, name)
		}
	}
}
