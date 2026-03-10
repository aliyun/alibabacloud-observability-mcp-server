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
	executeSPLResult   map[string]interface{}
	executeSPLErr      error
	listWorkspacesResult []map[string]interface{}
	listWorkspacesErr    error
	queryMetricResult  []map[string]interface{}
	queryMetricErr     error
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

func (m *mockCMSClient) DataAgentQuery(_ context.Context, _, _, _ string, _, _ int64) (*client.DataAgentResult, error) {
	return &client.DataAgentResult{}, nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestCMSTools_Count(t *testing.T) {
	tools := CMSTools(&mockCMSClient{}, &mockSLSClient{})
	// 4 main tools + 2 aliases (cms_execute_promql, cms_text_to_promql)
	if got := len(tools); got != 6 {
		t.Errorf("CMSTools() returned %d tools, want 6", got)
	}
}

func TestCMSTools_NamesPrefix(t *testing.T) {
	tools := CMSTools(&mockCMSClient{}, &mockSLSClient{})
	for _, tool := range tools {
		// sls_query_metricstore is the PromQL tool which uses sls_ prefix
		if !strings.HasPrefix(tool.Name, "cms_") && !strings.HasPrefix(tool.Name, "sls_") {
			t.Errorf("tool %q does not have cms_ or sls_ prefix", tool.Name)
		}
	}
}

func TestCMSTools_ExpectedNames(t *testing.T) {
	tools := CMSTools(&mockCMSClient{}, &mockSLSClient{})
	expected := map[string]bool{
		"cms_query_metric":      false,
		"cms_list_metrics":      false,
		"cms_list_namespaces":   false,
		"sls_query_metricstore": false,
		"cms_execute_promql":    false, // alias for Python compatibility
		"cms_text_to_promql":    false, // alias for Python compatibility
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

func TestQueryMetric_Success(t *testing.T) {
	mock := &mockCMSClient{
		queryMetricResult: []map[string]interface{}{
			{"timestamp": float64(1234567890), "Average": 42.5, "instanceId": "i-xxx"},
		},
	}
	tools := CMSTools(mock, &mockSLSClient{})
	ctx := context.Background()

	var handler func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "cms_query_metric" {
			handler = tt.Handler
			break
		}
	}

	result, err := handler(ctx, map[string]interface{}{
		"namespace":  "acs_ecs_dashboard",
		"metricName": "CPUUtilization",
		"regionId":   "cn-hangzhou",
		"from_time":  "now-1h",
		"to_time":    "now",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"].(bool) {
		t.Errorf("expected error=false, got true: %s", resp["message"])
	}
	data := resp["data"].(map[string]interface{})
	datapoints := data["datapoints"].([]map[string]interface{})
	if len(datapoints) != 1 {
		t.Errorf("expected 1 datapoint, got %d", len(datapoints))
	}
}

func TestQueryMetric_WithDimensions(t *testing.T) {
	mock := &mockCMSClient{
		queryMetricResult: []map[string]interface{}{
			{"timestamp": float64(1234567890), "Average": 55.0},
		},
	}
	tools := CMSTools(mock, &mockSLSClient{})
	ctx := context.Background()

	var handler func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "cms_query_metric" {
			handler = tt.Handler
			break
		}
	}

	result, err := handler(ctx, map[string]interface{}{
		"namespace":  "acs_ecs_dashboard",
		"metricName": "CPUUtilization",
		"dimensions": `{"instanceId":"i-abc123"}`,
		"regionId":   "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"].(bool) {
		t.Errorf("expected error=false, got true: %s", resp["message"])
	}
}

func TestQueryMetric_InvalidDimensions(t *testing.T) {
	mock := &mockCMSClient{}
	tools := CMSTools(mock, &mockSLSClient{})
	ctx := context.Background()

	var handler func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "cms_query_metric" {
			handler = tt.Handler
			break
		}
	}

	result, err := handler(ctx, map[string]interface{}{
		"namespace":  "acs_ecs_dashboard",
		"metricName": "CPUUtilization",
		"dimensions": "not-valid-json",
		"regionId":   "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if !resp["error"].(bool) {
		t.Errorf("expected error=true for invalid dimensions JSON")
	}
	msg := resp["message"].(string)
	if !strings.Contains(msg, "invalid dimensions JSON") {
		t.Errorf("expected error message about invalid dimensions, got %q", msg)
	}
}

func TestQueryMetric_ClientError(t *testing.T) {
	mock := &mockCMSClient{
		queryMetricErr: fmt.Errorf("connection refused"),
	}
	tools := CMSTools(mock, &mockSLSClient{})
	ctx := context.Background()

	var handler func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "cms_query_metric" {
			handler = tt.Handler
			break
		}
	}

	result, err := handler(ctx, map[string]interface{}{
		"namespace":  "acs_ecs_dashboard",
		"metricName": "CPUUtilization",
		"regionId":   "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if !resp["error"].(bool) {
		t.Errorf("expected error=true for client error")
	}
	msg := resp["message"].(string)
	if !strings.Contains(msg, "connection refused") {
		t.Errorf("expected error message to contain 'connection refused', got %q", msg)
	}
}

func TestListMetrics_Success(t *testing.T) {
	mock := &mockCMSClient{
		queryMetricResult: []map[string]interface{}{
			{"metricName": "CPUUtilization", "namespace": "acs_ecs_dashboard"},
			{"metricName": "memory_usedutilization", "namespace": "acs_ecs_dashboard"},
		},
	}
	tools := CMSTools(mock, &mockSLSClient{})
	ctx := context.Background()

	var handler func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "cms_list_metrics" {
			handler = tt.Handler
			break
		}
	}

	result, err := handler(ctx, map[string]interface{}{
		"namespace": "acs_ecs_dashboard",
		"regionId":  "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"].(bool) {
		t.Errorf("expected error=false, got true: %s", resp["message"])
	}
	data := resp["data"].(map[string]interface{})
	metrics := data["metrics"].([]map[string]interface{})
	if len(metrics) != 2 {
		t.Errorf("expected 2 metrics, got %d", len(metrics))
	}
}

func TestListMetrics_ClientError(t *testing.T) {
	mock := &mockCMSClient{
		queryMetricErr: fmt.Errorf("service unavailable"),
	}
	tools := CMSTools(mock, &mockSLSClient{})
	ctx := context.Background()

	var handler func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "cms_list_metrics" {
			handler = tt.Handler
			break
		}
	}

	result, err := handler(ctx, map[string]interface{}{
		"namespace": "acs_ecs_dashboard",
		"regionId":  "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if !resp["error"].(bool) {
		t.Errorf("expected error=true for client error")
	}
}

func TestListNamespaces_Success(t *testing.T) {
	mock := &mockCMSClient{
		queryMetricResult: []map[string]interface{}{
			{"namespace": "acs_ecs_dashboard", "description": "ECS"},
			{"namespace": "acs_rds_dashboard", "description": "RDS"},
		},
	}
	tools := CMSTools(mock, &mockSLSClient{})
	ctx := context.Background()

	var handler func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "cms_list_namespaces" {
			handler = tt.Handler
			break
		}
	}

	result, err := handler(ctx, map[string]interface{}{
		"regionId": "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"].(bool) {
		t.Errorf("expected error=false, got true: %s", resp["message"])
	}
	data := resp["data"].(map[string]interface{})
	namespaces := data["namespaces"].([]map[string]interface{})
	if len(namespaces) != 2 {
		t.Errorf("expected 2 namespaces, got %d", len(namespaces))
	}
}

func TestListNamespaces_ClientError(t *testing.T) {
	mock := &mockCMSClient{
		queryMetricErr: fmt.Errorf("timeout"),
	}
	tools := CMSTools(mock, &mockSLSClient{})
	ctx := context.Background()

	var handler func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "cms_list_namespaces" {
			handler = tt.Handler
			break
		}
	}

	result, err := handler(ctx, map[string]interface{}{
		"regionId": "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if !resp["error"].(bool) {
		t.Errorf("expected error=true for client error")
	}
	msg := resp["message"].(string)
	if !strings.Contains(msg, "timeout") {
		t.Errorf("expected error message to contain 'timeout', got %q", msg)
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
		"regionId":    "cn-hangzhou",
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
				"regionId":    "cn-hangzhou",
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
			if !strings.Contains(msg, "sls_list_metricstores") {
				t.Errorf("expected suggestion to use sls_list_metricstores, got: %s", msg)
			}
			if !strings.Contains(msg, "cms_query_metric") {
				t.Errorf("expected suggestion to use cms_query_metric, got: %s", msg)
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
		"regionId":    "cn-hangzhou",
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
