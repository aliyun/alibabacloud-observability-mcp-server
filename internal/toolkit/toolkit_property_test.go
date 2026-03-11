package toolkit_test

import (
	"context"
	"strings"
	"testing"

	"github.com/alibabacloud-observability-mcp-server-go/internal/client"
	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit"
	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit/iaas"
	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit/paas"
	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit/shared"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// --- stub clients (no-op, only needed to construct toolkits) ---

type stubCMS struct{}

func (s *stubCMS) ExecuteSPL(_ context.Context, _, _, _ string, _, _ int64, _ int) (map[string]interface{}, error) {
	return nil, nil
}
func (s *stubCMS) ListWorkspaces(_ context.Context, _ string) ([]map[string]interface{}, error) {
	return nil, nil
}
func (s *stubCMS) QueryMetric(_ context.Context, _, _ string, _ string, _ map[string]string, _, _ int64) ([]map[string]interface{}, error) {
	return nil, nil
}

func (s *stubCMS) TextToSQL(_ context.Context, _, _, _, _ string) (string, error) {
	return "", nil
}

func (s *stubCMS) DataAgentQuery(_ context.Context, _, _, _ string, _, _ int64) (*client.DataAgentResult, error) {
	return &client.DataAgentResult{}, nil
}

var _ client.CMSClient = (*stubCMS)(nil)

type stubSLS struct{}

func (s *stubSLS) Query(_ context.Context, _, _, _, _ string, _, _ int64) ([]map[string]interface{}, error) {
	return nil, nil
}
func (s *stubSLS) GetContextLogs(_ context.Context, _, _, _, _, _ string, _, _ int) (map[string]interface{}, error) {
	return nil, nil
}
func (s *stubSLS) ListProjects(_ context.Context, _ string) ([]string, error) { return nil, nil }
func (s *stubSLS) ListProjectsWithFilter(_ context.Context, _, _ string, _ int) ([]map[string]interface{}, error) {
	return nil, nil
}
func (s *stubSLS) ListLogStores(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}
func (s *stubSLS) ListLogStoresWithFilter(_ context.Context, _, _, _ string, _ int, _ bool) ([]string, error) {
	return nil, nil
}
func (s *stubSLS) ListMetricStores(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}
func (s *stubSLS) TextToSQL(_ context.Context, _, _, _, _ string) (string, error) {
	return "", nil
}

var _ client.SLSClient = (*stubSLS)(nil)

// --- helpers ---

func newPaaS() toolkit.Toolkit  { return paas.NewPaaSToolkit(&stubCMS{}) }
func newIaaS() toolkit.Toolkit  { return iaas.NewIaaSToolkit(&stubSLS{}, &stubCMS{}) }
func newShared() toolkit.Toolkit { return shared.New(&stubCMS{}) }

// sharedToolNames returns the set of tool names from the Shared toolkit.
func sharedToolNames() map[string]bool {
	m := make(map[string]bool)
	for _, t := range newShared().Tools() {
		m[t.Name] = true
	}
	return m
}

// allToolNames returns the set of tool names from all three toolkits.
func allToolNames() map[string]bool {
	m := make(map[string]bool)
	for _, t := range newPaaS().Tools() {
		m[t.Name] = true
	}
	for _, t := range newIaaS().Tools() {
		m[t.Name] = true
	}
	for _, t := range newShared().Tools() {
		m[t.Name] = true
	}
	return m
}

// TestProperty_ToolkitPrefixConsistency verifies that every PaaS tool name
// starts with "umodel_" and every IaaS tool name starts with "sls_" or "cms_".
// Note: cms_natural_language_query is an alias in PaaS for Python compatibility.
//
// Feature: go-mcp-server-rewrite, Property 1: 工具集前缀一致性
// Validates: Requirements 3.2, 3.3
func TestProperty_ToolkitPrefixConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	paasTools := newPaaS().Tools()
	iaasTools := newIaaS().Tools()

	// Generator: random index into PaaS tools slice
	genPaaSIdx := gen.IntRange(0, len(paasTools)-1)
	// Generator: random index into IaaS tools slice
	genIaaSIdx := gen.IntRange(0, len(iaasTools)-1)

	properties.Property("PaaS tool names start with umodel_ or are known aliases", prop.ForAll(
		func(idx int) bool {
			name := paasTools[idx].Name
			// Allow umodel_ prefix or known aliases for Python compatibility
			return strings.HasPrefix(name, "umodel_") || name == "cms_natural_language_query"
		},
		genPaaSIdx,
	))

	properties.Property("IaaS tool names start with sls_ or cms_", prop.ForAll(
		func(idx int) bool {
			name := iaasTools[idx].Name
			return strings.HasPrefix(name, "sls_") || strings.HasPrefix(name, "cms_")
		},
		genIaaSIdx,
	))

	properties.TestingRun(t)
}

// TestProperty_ScopeToolRegistration verifies that for any valid
// MCP_TOOLKIT_SCOPE value, the registered tool set exactly matches the
// expected combination:
//   - "paas"       → PaaS + Shared tools only
//   - "iaas"       → IaaS + Shared tools only
//   - "all" or ""  → PaaS + IaaS + Shared tools
//
// Feature: go-mcp-server-rewrite, Property 2: 作用域工具注册正确性
// Validates: Requirements 3.5, 3.6, 3.7
func TestProperty_ScopeToolRegistration(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	paasToolkit := newPaaS()
	iaasToolkit := newIaaS()
	sharedToolkit := newShared()

	// Pre-compute expected tool name sets for each scope.
	paasNames := make(map[string]bool)
	for _, t := range paasToolkit.Tools() {
		paasNames[t.Name] = true
	}
	iaasNames := make(map[string]bool)
	for _, t := range iaasToolkit.Tools() {
		iaasNames[t.Name] = true
	}
	sharedNames := sharedToolNames()
	allNames := allToolNames()

	// expectedNames returns the expected tool name set for a given scope.
	expectedNames := func(scope string) map[string]bool {
		switch scope {
		case "paas":
			m := make(map[string]bool)
			for n := range paasNames {
				m[n] = true
			}
			for n := range sharedNames {
				m[n] = true
			}
			return m
		case "iaas":
			m := make(map[string]bool)
			for n := range iaasNames {
				m[n] = true
			}
			for n := range sharedNames {
				m[n] = true
			}
			return m
		default: // "all" or ""
			return allNames
		}
	}

	// setsEqual checks two string sets are identical.
	setsEqual := func(a, b map[string]bool) bool {
		if len(a) != len(b) {
			return false
		}
		for k := range a {
			if !b[k] {
				return false
			}
		}
		return true
	}

	// Generator: one of the four valid scope values.
	genScope := gen.OneConstOf("all", "paas", "iaas", "")

	properties.Property("registered tools match expected set for scope", prop.ForAll(
		func(scope string) bool {
			r := toolkit.NewRegistry()
			toolkit.RegisterToolkits(r, scope, nil, paasToolkit, iaasToolkit, sharedToolkit)

			registered := make(map[string]bool)
			for _, tool := range r.List() {
				registered[tool.Name] = true
			}

			expected := expectedNames(scope)
			return setsEqual(registered, expected)
		},
		genScope,
	))

	properties.TestingRun(t)
}
