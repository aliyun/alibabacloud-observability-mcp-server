package iaas

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/alibabacloud-observability-mcp-server-go/internal/client"
)

// ---------------------------------------------------------------------------
// Mock CMS Client
// ---------------------------------------------------------------------------

// Compile-time check that mockCMSClient implements client.CMSClient.
var _ client.CMSClient = (*mockCMSClient)(nil)

type mockCMSClient struct {
	executeSPLResult     map[string]interface{}
	executeSPLErr        error
	listWorkspacesResult []map[string]interface{}
	listWorkspacesErr    error
	queryMetricResult    []map[string]interface{}
	queryMetricErr       error
	chatWithSkillResult  string
	chatWithSkillErr     error
}

func (m *mockCMSClient) ExecuteSPL(_ context.Context, _, _, _ string, _, _ int64, _ int) (map[string]interface{}, error) {
	return m.executeSPLResult, m.executeSPLErr
}

func (m *mockCMSClient) ListWorkspaces(_ context.Context, _ string) ([]map[string]interface{}, error) {
	return m.listWorkspacesResult, m.listWorkspacesErr
}

func (m *mockCMSClient) QueryMetric(_ context.Context, _, _, _ string, _ map[string]string, _, _ int64) ([]map[string]interface{}, error) {
	return m.queryMetricResult, m.queryMetricErr
}

func (m *mockCMSClient) TextToSQL(_ context.Context, _, _, _, _ string) (string, error) {
	return "", nil
}

func (m *mockCMSClient) ChatWithSkill(_ context.Context, _, _, _, _, _ string) (string, error) {
	return m.chatWithSkillResult, m.chatWithSkillErr
}

func (m *mockCMSClient) DataAgentQuery(_ context.Context, _, _, _ string, _, _ int64) (*client.DataAgentResult, error) {
	return &client.DataAgentResult{}, nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestCMSTools_Count(t *testing.T) {
	tools := CMSTools(&mockCMSClient{}, &mockSLSClient{})
	// 2 tools: cms_execute_promql + cms_text_to_promql
	if got := len(tools); got != 2 {
		t.Errorf("CMSTools() returned %d tools, want 6", got)
	}
}

func TestCMSTools_NamesPrefix(t *testing.T) {
	tools := CMSTools(&mockCMSClient{}, &mockSLSClient{})
	for _, tool := range tools {
		if !strings.HasPrefix(tool.Name, "cms_") {
			t.Errorf("tool %q does not have cms_ prefix", tool.Name)
		}
	}
}

func TestCMSTools_ExpectedNames(t *testing.T) {
	tools := CMSTools(&mockCMSClient{}, &mockSLSClient{})
	expected := map[string]bool{
		"cms_execute_promql": false,
		"cms_text_to_promql": false,
	}
	for _, tool := range tools {
		if _, ok := expected[tool.Name]; !ok {
			t.Errorf("unexpected tool name: %q", tool.Name)
		}
		expected[tool.Name] = true
	}
	for name, found := range expected {
		if !found {
			t.Errorf("expected tool %q not found", name)
		}
	}
}

func TestCMSTools_MissingParams(t *testing.T) {
	mock := &mockCMSClient{}
	tools := CMSTools(mock, &mockSLSClient{})
	ctx := context.Background()

	for _, tool := range tools {
		t.Run(tool.Name+"_empty_params", func(t *testing.T) {
			result, err := tool.Handler(ctx, map[string]interface{}{})
			if err != nil {
				t.Fatalf("handler returned error: %v", err)
			}
			resp, ok := result.(map[string]interface{})
			if !ok {
				t.Fatalf("expected map response, got %T", result)
			}
			isErr, _ := resp["error"].(bool)
			if !isErr {
				t.Errorf("expected error=true for empty params, got false")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// cms_execute_promql / handleExecutePromQL tests
// ---------------------------------------------------------------------------

func TestExecutePromQL_Success(t *testing.T) {
	slsMock := &mockSLSClient{
		queryResult: []map[string]interface{}{
			{"__name__": "cpu_total", "__value__": 0.85, "__timestamp__": float64(1700000000)},
		},
	}
	tools := CMSTools(&mockCMSClient{}, slsMock)
	ctx := context.Background()

	var handler func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "cms_execute_promql" {
			handler = tt.Handler
			break
		}
	}

	result, err := handler(ctx, map[string]interface{}{
		"project":     "my-project",
		"metricStore": "my-metrics",
		"query":       "up",
		"regionId":    "cn-hongkong",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"].(bool) {
		t.Errorf("expected error=false, got true: %s", resp["message"])
	}
	data := resp["data"].(map[string]interface{})
	if data["query"] == nil {
		t.Error("expected 'query' field in data")
	}
}

func TestExecutePromQL_MetricStoreNotFound(t *testing.T) {
	tests := []struct {
		name   string
		errMsg string
	}{
		{"LogStoreNotExist", "LogStoreNotExist: metricstore my-metrics does not exist"},
		{"MetricStoreNotExist", "MetricStoreNotExist: my-metrics"},
		{"not exist", "The specified logstore my-metrics does not exist"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			slsMock := &mockSLSClient{
				queryErr: fmt.Errorf("%s", tc.errMsg),
			}
			tools := CMSTools(&mockCMSClient{}, slsMock)
			ctx := context.Background()

			var handler func(context.Context, map[string]interface{}) (interface{}, error)
			for _, tt := range tools {
				if tt.Name == "cms_execute_promql" {
					handler = tt.Handler
					break
				}
			}

			result, err := handler(ctx, map[string]interface{}{
				"project":     "my-project",
				"metricStore": "my-metrics",
				"query":       "up",
				"regionId":    "cn-hongkong",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			resp := result.(map[string]interface{})
			if !resp["error"].(bool) {
				t.Fatal("expected error=true, got false")
			}
			msg := resp["message"].(string)
			if !strings.Contains(msg, "does not exist in project") {
				t.Errorf("expected friendly not-found message, got: %s", msg)
			}
			if !strings.Contains(msg, "does not exist") {
				t.Errorf("expected not-found message, got: %s", msg)
			}
		})
	}
}

func TestExecutePromQL_GenericError(t *testing.T) {
	slsMock := &mockSLSClient{
		queryErr: fmt.Errorf("connection timeout"),
	}
	tools := CMSTools(&mockCMSClient{}, slsMock)
	ctx := context.Background()

	var handler func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "cms_execute_promql" {
			handler = tt.Handler
			break
		}
	}

	result, err := handler(ctx, map[string]interface{}{
		"project":     "my-project",
		"metricStore": "my-metrics",
		"query":       "up",
		"regionId":    "cn-hongkong",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if !resp["error"].(bool) {
		t.Fatal("expected error=true, got false")
	}
	msg := resp["message"].(string)
	if !strings.Contains(msg, "PromQL query failed") {
		t.Errorf("expected 'PromQL query failed' message, got: %s", msg)
	}
	if strings.Contains(msg, "sls_list_metricstores") {
		t.Errorf("generic error should not suggest sls_list_metricstores, got: %s", msg)
	}
}
