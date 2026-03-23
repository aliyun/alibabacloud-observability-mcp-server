package shared

import (
	"context"
	"errors"
	"testing"

	"github.com/alibabacloud-observability-mcp-server-go/internal/client"
	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit"
)

// mockCMSClient is a test double for client.CMSClient.
type mockCMSClient struct {
	listWorkspacesFunc func(ctx context.Context, region string) ([]map[string]interface{}, error)
	executeSPLFunc     func(ctx context.Context, region, workspace, query string, from, to int64, limit int) (map[string]interface{}, error)
	queryMetricFunc    func(ctx context.Context, region, namespace, metricName string, dimensions map[string]string, from, to int64) ([]map[string]interface{}, error)
}

func (m *mockCMSClient) ListWorkspaces(ctx context.Context, region string) ([]map[string]interface{}, error) {
	if m.listWorkspacesFunc != nil {
		return m.listWorkspacesFunc(ctx, region)
	}
	return nil, nil
}

func (m *mockCMSClient) ExecuteSPL(ctx context.Context, region, workspace, query string, from, to int64, limit int) (map[string]interface{}, error) {
	if m.executeSPLFunc != nil {
		return m.executeSPLFunc(ctx, region, workspace, query, from, to, limit)
	}
	return nil, nil
}

func (m *mockCMSClient) QueryMetric(ctx context.Context, region, namespace, metricName string, dimensions map[string]string, from, to int64) ([]map[string]interface{}, error) {
	if m.queryMetricFunc != nil {
		return m.queryMetricFunc(ctx, region, namespace, metricName, dimensions, from, to)
	}
	return nil, nil
}

func (m *mockCMSClient) TextToSQL(ctx context.Context, region, project, logStore, text string) (string, error) {
	return "", nil
}

func (m *mockCMSClient) ChatWithSkill(_ context.Context, _, _, _, _, _ string) (string, error) {
	return "", nil
}

func (m *mockCMSClient) DataAgentQuery(_ context.Context, _, _, _ string, _, _ int64) (*client.DataAgentResult, error) {
	return &client.DataAgentResult{}, nil
}

func TestSharedToolkit_Name(t *testing.T) {
	tk := New(&mockCMSClient{})
	if got := tk.Name(); got != "shared" {
		t.Errorf("Name() = %q, want %q", got, "shared")
	}
}

func TestSharedToolkit_Tools(t *testing.T) {
	tk := New(&mockCMSClient{})
	tools := tk.Tools()

	want := map[string]bool{
		"list_workspace": false,
		"list_domains":   false,
		"introduction":   false,
	}

	if len(tools) != len(want) {
		t.Fatalf("Tools() returned %d tools, want %d", len(tools), len(want))
	}

	for _, tool := range tools {
		if _, ok := want[tool.Name]; !ok {
			t.Errorf("unexpected tool %q", tool.Name)
		}
		want[tool.Name] = true
		if tool.Handler == nil {
			t.Errorf("tool %q has nil handler", tool.Name)
		}
		if tool.Description == "" {
			t.Errorf("tool %q has empty description", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Errorf("tool %q has nil InputSchema", tool.Name)
		}
	}

	for name, found := range want {
		if !found {
			t.Errorf("missing expected tool %q", name)
		}
	}
}

func TestHandleListWorkspace_Success(t *testing.T) {
	mock := &mockCMSClient{
		listWorkspacesFunc: func(_ context.Context, region string) ([]map[string]interface{}, error) {
			return []map[string]interface{}{
				{"name": "ws-1", "id": "123"},
				{"name": "ws-2", "id": "456"},
			}, nil
		},
	}
	tk := New(mock)
	ctx := context.Background()

	result, err := tk.handleListWorkspace(ctx, map[string]interface{}{
		"regionId": "cn-hongkong",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]interface{})
	if m["error"] != false {
		t.Error("expected error=false")
	}
	if m["total_count"] != 2 {
		t.Errorf("total_count = %v, want 2", m["total_count"])
	}
	if m["region"] != "cn-hongkong" {
		t.Errorf("region = %v, want cn-hongkong", m["region"])
	}
}

func TestHandleListWorkspace_MissingRegion(t *testing.T) {
	tk := New(&mockCMSClient{})
	ctx := context.Background()

	result, err := tk.handleListWorkspace(ctx, map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]interface{})
	if m["error"] != true {
		t.Error("expected error=true for missing regionId")
	}
}

func TestHandleListWorkspace_CMSError(t *testing.T) {
	mock := &mockCMSClient{
		listWorkspacesFunc: func(_ context.Context, _ string) ([]map[string]interface{}, error) {
			return nil, errors.New("connection refused")
		},
	}
	tk := New(mock)
	ctx := context.Background()

	result, err := tk.handleListWorkspace(ctx, map[string]interface{}{
		"regionId": "cn-beijing",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]interface{})
	if m["error"] != true {
		t.Error("expected error=true on CMS failure")
	}
}

func TestHandleListDomains_Success(t *testing.T) {
	mock := &mockCMSClient{
		executeSPLFunc: func(_ context.Context, region, workspace, query string, from, to int64, limit int) (map[string]interface{}, error) {
			if workspace != "my-ws" {
				t.Errorf("workspace = %q, want %q", workspace, "my-ws")
			}
			if region != "cn-hongkong" {
				t.Errorf("region = %q, want %q", region, "cn-hongkong")
			}
			return map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"__domain__": "apm", "cnt": 42},
				},
			}, nil
		},
	}
	tk := New(mock)
	ctx := context.Background()

	result, err := tk.handleListDomains(ctx, map[string]interface{}{
		"workspace": "my-ws",
		"regionId":  "cn-hongkong",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]interface{})
	if m["error"] != false {
		t.Error("expected error=false")
	}
	if m["workspace"] != "my-ws" {
		t.Errorf("workspace = %v, want my-ws", m["workspace"])
	}
	if m["query"] != listDomainsQuery {
		t.Errorf("query mismatch")
	}
}

func TestHandleListDomains_MissingParams(t *testing.T) {
	tk := New(&mockCMSClient{})
	ctx := context.Background()

	tests := []struct {
		name   string
		params map[string]interface{}
	}{
		{"missing both", map[string]interface{}{}},
		{"missing workspace", map[string]interface{}{"regionId": "cn-hongkong"}},
		{"missing regionId", map[string]interface{}{"workspace": "ws"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tk.handleListDomains(ctx, tt.params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			m := result.(map[string]interface{})
			if m["error"] != true {
				t.Error("expected error=true for missing params")
			}
		})
	}
}

func TestHandleListDomains_CMSError(t *testing.T) {
	mock := &mockCMSClient{
		executeSPLFunc: func(_ context.Context, _, _, _ string, _, _ int64, _ int) (map[string]interface{}, error) {
			return nil, errors.New("spl execution failed")
		},
	}
	tk := New(mock)
	ctx := context.Background()

	result, err := tk.handleListDomains(ctx, map[string]interface{}{
		"workspace": "ws",
		"regionId":  "cn-hongkong",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]interface{})
	if m["error"] != true {
		t.Error("expected error=true on CMS failure")
	}
}

func TestHandleIntroduction(t *testing.T) {
	result, err := handleIntroduction(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := result.(map[string]interface{})
	if m["name"] != "Alibaba Cloud Observability MCP Server" {
		t.Errorf("name = %v", m["name"])
	}
	if m["version"] != "1.0.0" {
		t.Errorf("version = %v", m["version"])
	}

	// Verify key sections exist
	if _, ok := m["capabilities"]; !ok {
		t.Error("missing capabilities")
	}
	if _, ok := m["tool_layers"]; !ok {
		t.Error("missing tool_layers")
	}
	if _, ok := m["important_notes"]; !ok {
		t.Error("missing important_notes")
	}
	if _, ok := m["references"]; !ok {
		t.Error("missing references")
	}

	// Verify tool_layers has all three layers
	layers := m["tool_layers"].(map[string]interface{})
	for _, layer := range []string{"paas", "iaas", "shared"} {
		if _, ok := layers[layer]; !ok {
			t.Errorf("missing tool_layer %q", layer)
		}
	}
}

// TestSharedToolkit_ImplementsToolkit verifies the interface contract.
func TestSharedToolkit_ImplementsToolkit(t *testing.T) {
	var _ interface {
		Name() string
		Tools() []toolkit.Tool
	} = New(&mockCMSClient{})
}
