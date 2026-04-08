package toolkit

import (
	"context"
	"sort"
	"testing"
)

// --- helpers ---

// stubToolkit is a minimal Toolkit implementation for testing.
type stubToolkit struct {
	name  string
	tools []Tool
}

func (s *stubToolkit) Name() string  { return s.name }
func (s *stubToolkit) Tools() []Tool { return s.tools }

func dummyHandler(_ context.Context, _ map[string]interface{}) (interface{}, error) {
	return nil, nil
}

func makeTool(name string) Tool {
	return Tool{
		Name:        name,
		Description: name + " description",
		InputSchema: map[string]interface{}{"type": "object"},
		Handler:     dummyHandler,
	}
}

func makeToolkit(name string, toolNames ...string) *stubToolkit {
	tools := make([]Tool, len(toolNames))
	for i, n := range toolNames {
		tools[i] = makeTool(n)
	}
	return &stubToolkit{name: name, tools: tools}
}

// --- Registry unit tests ---

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if got := len(r.List()); got != 0 {
		t.Fatalf("new registry should be empty, got %d tools", got)
	}
}

func TestRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	tk := makeToolkit("test", "tool_a", "tool_b")
	r.Register(tk)

	tool, ok := r.Get("tool_a")
	if !ok {
		t.Fatal("expected tool_a to be found")
	}
	if tool.Name != "tool_a" {
		t.Fatalf("expected name tool_a, got %s", tool.Name)
	}

	_, ok = r.Get("nonexistent")
	if ok {
		t.Fatal("expected nonexistent tool to not be found")
	}
}

func TestRegisterOverwrites(t *testing.T) {
	r := NewRegistry()
	tk1 := &stubToolkit{name: "first", tools: []Tool{
		{Name: "dup", Description: "first"},
	}}
	tk2 := &stubToolkit{name: "second", tools: []Tool{
		{Name: "dup", Description: "second"},
	}}
	r.Register(tk1)
	r.Register(tk2)

	tool, _ := r.Get("dup")
	if tool.Description != "second" {
		t.Fatalf("expected overwritten description 'second', got %q", tool.Description)
	}
}

func TestListSortedByName(t *testing.T) {
	r := NewRegistry()
	tk := makeToolkit("unsorted", "zebra", "alpha", "middle")
	r.Register(tk)

	list := r.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(list))
	}
	names := make([]string, len(list))
	for i, tool := range list {
		names[i] = tool.Name
	}
	if !sort.StringsAreSorted(names) {
		t.Fatalf("List() should return tools sorted by name, got %v", names)
	}
}

// --- Scope filtering unit tests ---

func TestRegisterToolkits_ScopePaaS(t *testing.T) {
	r := NewRegistry()
	paas := makeToolkit("paas", "umodel_list")
	iaas := makeToolkit("iaas", "sls_query")
	shared := makeToolkit("shared", "list_workspace")

	RegisterToolkits(r, ScopePaaS, nil, paas, iaas, shared)

	if _, ok := r.Get("umodel_list"); !ok {
		t.Error("paas tool should be registered under paas scope")
	}
	if _, ok := r.Get("list_workspace"); !ok {
		t.Error("shared tool should be registered under paas scope")
	}
	if _, ok := r.Get("sls_query"); ok {
		t.Error("iaas tool should NOT be registered under paas scope")
	}
}

func TestRegisterToolkits_ScopeIaaS(t *testing.T) {
	r := NewRegistry()
	paas := makeToolkit("paas", "umodel_list")
	iaas := makeToolkit("iaas", "sls_query")
	shared := makeToolkit("shared", "list_workspace")

	RegisterToolkits(r, ScopeIaaS, nil, paas, iaas, shared)

	if _, ok := r.Get("sls_query"); !ok {
		t.Error("iaas tool should be registered under iaas scope")
	}
	if _, ok := r.Get("list_workspace"); !ok {
		t.Error("shared tool should be registered under iaas scope")
	}
	if _, ok := r.Get("umodel_list"); ok {
		t.Error("paas tool should NOT be registered under iaas scope")
	}
}

func TestRegisterToolkits_ScopeAll(t *testing.T) {
	r := NewRegistry()
	paas := makeToolkit("paas", "umodel_list")
	iaas := makeToolkit("iaas", "sls_query")
	shared := makeToolkit("shared", "list_workspace")

	RegisterToolkits(r, ScopeAll, nil, paas, iaas, shared)

	for _, name := range []string{"umodel_list", "sls_query", "list_workspace"} {
		if _, ok := r.Get(name); !ok {
			t.Errorf("tool %s should be registered under all scope", name)
		}
	}
}

func TestRegisterToolkits_ScopeEmpty(t *testing.T) {
	r := NewRegistry()
	paas := makeToolkit("paas", "umodel_list")
	iaas := makeToolkit("iaas", "sls_query")
	shared := makeToolkit("shared", "list_workspace")

	RegisterToolkits(r, "", nil, paas, iaas, shared)

	for _, name := range []string{"umodel_list", "sls_query", "list_workspace"} {
		if _, ok := r.Get(name); !ok {
			t.Errorf("tool %s should be registered when scope is empty (defaults to all)", name)
		}
	}
}

// --- Concurrency safety test ---

func TestRegistryConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	done := make(chan struct{})

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			tk := makeToolkit("w", "concurrent_tool")
			r.Register(tk)
		}
		done <- struct{}{}
	}()

	// Reader goroutines
	for i := 0; i < 3; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				r.Get("concurrent_tool")
				r.List()
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 4; i++ {
		<-done
	}
}

func TestFilterByNames(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	tk := makeToolkit("all", "tool_a", "tool_b", "tool_c")
	r.Register(tk)

	r.FilterByNames([]string{"tool_a", "tool_c"})

	if _, ok := r.Get("tool_a"); !ok {
		t.Error("tool_a should be kept")
	}
	if _, ok := r.Get("tool_c"); !ok {
		t.Error("tool_c should be kept")
	}
	if _, ok := r.Get("tool_b"); ok {
		t.Error("tool_b should be removed")
	}
	if got := len(r.List()); got != 2 {
		t.Errorf("expected 2 tools after filter, got %d", got)
	}
}

func TestFilterByNames_EmptyList(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	tk := makeToolkit("all", "tool_a", "tool_b")
	r.Register(tk)

	r.FilterByNames([]string{})

	if got := len(r.List()); got != 0 {
		t.Errorf("expected 0 tools after filtering with empty list, got %d", got)
	}
}

func TestRegisterToolkits_EnabledToolsFilter(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	p := makeToolkit("paas", "umodel_get_entities", "umodel_get_metrics")
	i := makeToolkit("iaas", "sls_query_logstore", "cms_query_metric")
	s := makeToolkit("shared", "list_workspace", "introduction")

	RegisterToolkits(r, ScopeAll, []string{"umodel_get_entities", "sls_query_logstore", "list_workspace"}, p, i, s)

	for _, name := range []string{"umodel_get_entities", "sls_query_logstore", "list_workspace"} {
		if _, ok := r.Get(name); !ok {
			t.Errorf("tool %s should be registered", name)
		}
	}
	for _, name := range []string{"umodel_get_metrics", "cms_query_metric", "introduction"} {
		if _, ok := r.Get(name); ok {
			t.Errorf("tool %s should NOT be registered when not in enabled_tools", name)
		}
	}
	if got := len(r.List()); got != 3 {
		t.Errorf("expected 3 tools, got %d", got)
	}
}

func TestRegisterToolkits_NilEnabledToolsKeepsAll(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	p := makeToolkit("paas", "umodel_list")
	i := makeToolkit("iaas", "sls_query")
	s := makeToolkit("shared", "list_workspace")

	RegisterToolkits(r, ScopeAll, nil, p, i, s)

	if got := len(r.List()); got != 3 {
		t.Errorf("expected 3 tools when enabled_tools is nil, got %d", got)
	}
}
