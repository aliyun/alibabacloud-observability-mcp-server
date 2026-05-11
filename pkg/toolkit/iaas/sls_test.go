package iaas

import (
	"context"
	"fmt"
	"strings"
	"testing"

	sls "github.com/alibabacloud-go/sls-20201230/v6/client"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/client"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/toolkit"
)

// ---------------------------------------------------------------------------
// Mock SLS Client
// ---------------------------------------------------------------------------

// Compile-time check that mockSLSClient implements client.SLSClient.
var _ client.SLSClient = (*mockSLSClient)(nil)

type mockSLSClient struct {
	queryResult                   []map[string]interface{}
	queryErr                      error
	getContextLogsResult          map[string]interface{}
	getContextLogsErr             error
	listProjectsResult            []string
	listProjectsErr               error
	listProjectsWithFilterResult  []map[string]interface{}
	listProjectsWithFilterErr     error
	listLogStoresResult           []string
	listLogStoresErr              error
	listLogStoresWithFilterResult []string
	listLogStoresWithFilterErr    error
	listMetricStoresResult        []string
	listMetricStoresErr           error
	textToSQLResult               string
	textToSQLErr                  error
}

func (m *mockSLSClient) Query(_ context.Context, _, _, _ string, _ *sls.GetLogsRequest) ([]map[string]interface{}, error) {
	return m.queryResult, m.queryErr
}

func (m *mockSLSClient) GetContextLogs(_ context.Context, _, _, _, _, _ string, _, _ int) (map[string]interface{}, error) {
	return m.getContextLogsResult, m.getContextLogsErr
}

func (m *mockSLSClient) ListProjects(_ context.Context, _ string) ([]string, error) {
	return m.listProjectsResult, m.listProjectsErr
}

func (m *mockSLSClient) ListProjectsWithFilter(_ context.Context, _, _ string, _ int) ([]map[string]interface{}, error) {
	return m.listProjectsWithFilterResult, m.listProjectsWithFilterErr
}

func (m *mockSLSClient) ListLogStores(_ context.Context, _, _ string) ([]string, error) {
	return m.listLogStoresResult, m.listLogStoresErr
}

func (m *mockSLSClient) ListLogStoresWithFilter(_ context.Context, _, _, _ string, _ int, _ bool) ([]string, error) {
	return m.listLogStoresWithFilterResult, m.listLogStoresWithFilterErr
}

func (m *mockSLSClient) ListMetricStores(_ context.Context, _, _ string) ([]string, error) {
	return m.listMetricStoresResult, m.listMetricStoresErr
}

func (m *mockSLSClient) TextToSQL(_ context.Context, _, _, _, _ string) (string, error) {
	return m.textToSQLResult, m.textToSQLErr
}

func (m *mockSLSClient) TextToPromQL(_ context.Context, _, _, _, _ string) (string, error) {
	return m.textToSQLResult, m.textToSQLErr
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestSLSTools_Count(t *testing.T) {
	tools := SLSTools(&mockSLSClient{}, &mockCMSClient{})
	if got := len(tools); got == 0 {
		t.Fatal("SLSTools() returned 0 tools")
	}
	// Verify count matches expected names list (single source of truth: TestSLSTools_ExpectedNames)
	expected := expectedSLSToolNames()
	if got := len(tools); got != len(expected) {
		t.Errorf("SLSTools() returned %d tools, want %d (expected names list)", got, len(expected))
	}
}

func TestSLSTools_NamesPrefix(t *testing.T) {
	tools := SLSTools(&mockSLSClient{}, &mockCMSClient{})
	for _, tool := range tools {
		if !strings.HasPrefix(tool.Name, "sls_") {
			t.Errorf("tool %q does not have sls_ prefix", tool.Name)
		}
	}
}

// expectedSLSToolNames returns the canonical set of SLS tool names.
// This is the single source of truth for tool-count and tool-name assertions.
func expectedSLSToolNames() map[string]bool {
	return map[string]bool{
		"sls_list_projects":    false,
		"sls_list_logstores":   false,
		"sls_text_to_sql":      false,
		"sls_text_to_sql_old":  false, // Deprecated alias for Python compatibility
		"sls_sop":              false,
		"sls_execute_sql":      false,
		"sls_execute_spl":      false,
		"sls_get_context_logs": false,
		"sls_text_to_spl":      false,
		"sls_log_explore":      false,
		"sls_log_compare":      false,
	}
}

func TestSLSTools_ExpectedNames(t *testing.T) {
	tools := SLSTools(&mockSLSClient{}, &mockCMSClient{})
	expected := expectedSLSToolNames()
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

func TestSLSTools_MissingParams(t *testing.T) {
	mock := &mockSLSClient{}
	tools := SLSTools(mock, &mockCMSClient{})
	ctx := context.Background()

	// Tools that return success with a message instead of error when params are missing
	// (to match Python behavior)
	skipTools := map[string]bool{
		"sls_list_logstores": true, // Returns success with message when project is empty
	}

	for _, tool := range tools {
		if skipTools[tool.Name] {
			continue
		}
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

func TestListProjects_Success(t *testing.T) {
	mock := &mockSLSClient{
		listProjectsWithFilterResult: []map[string]interface{}{
			{"project_name": "project-a", "description": "Project A", "region_id": "cn-hongkong"},
			{"project_name": "project-b", "description": "Project B", "region_id": "cn-hongkong"},
		},
	}
	tools := SLSTools(mock, &mockCMSClient{})
	ctx := context.Background()

	var tool func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_list_projects" {
			tool = tt.Handler
			break
		}
	}

	result, err := tool(ctx, map[string]interface{}{
		"regionId": "cn-hongkong",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"].(bool) {
		t.Errorf("expected error=false, got true")
	}
	data := resp["data"].(map[string]interface{})
	projects := data["projects"].([]map[string]interface{})
	if len(projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(projects))
	}
}

func TestListLogstores_Success(t *testing.T) {
	mock := &mockSLSClient{
		listLogStoresWithFilterResult: []string{"access-log", "error-log"},
	}
	tools := SLSTools(mock, &mockCMSClient{})
	ctx := context.Background()

	var tool func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_list_logstores" {
			tool = tt.Handler
			break
		}
	}

	result, err := tool(ctx, map[string]interface{}{
		"project":  "my-project",
		"regionId": "cn-hongkong",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"].(bool) {
		t.Errorf("expected error=false, got true")
	}
	data := resp["data"].(map[string]interface{})
	logstores := data["logstores"].([]string)
	if len(logstores) != 2 {
		t.Errorf("expected 2 logstores, got %d", len(logstores))
	}
}

func TestTextToSQL_Success(t *testing.T) {
	mock := &mockSLSClient{
		textToSQLResult: "SELECT * FROM log WHERE level = 'ERROR' LIMIT 10",
	}
	tools := SLSTools(mock, &mockCMSClient{})
	ctx := context.Background()

	var tool func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_text_to_sql" {
			tool = tt.Handler
			break
		}
	}

	result, err := tool(ctx, map[string]interface{}{
		"text":     "查找错误日志",
		"project":  "my-project",
		"logStore": "my-logstore",
		"regionId": "cn-hongkong",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"].(bool) {
		t.Errorf("expected error=false, got true")
	}
	data := resp["data"].(map[string]interface{})
	query := data["query"].(string)
	if query == "" {
		t.Error("expected non-empty query")
	}
}

// ---------------------------------------------------------------------------
// sls_text_to_sql_old (Deprecated alias) Tests
// ---------------------------------------------------------------------------

func TestTextToSQLOld_Name(t *testing.T) {
	tools := SLSTools(&mockSLSClient{}, &mockCMSClient{})
	var found bool
	for _, tool := range tools {
		if tool.Name == "sls_text_to_sql_old" {
			found = true
			break
		}
	}
	if !found {
		t.Error("sls_text_to_sql_old tool not found")
	}
}

func TestTextToSQLOld_SameInputSchema(t *testing.T) {
	tools := SLSTools(&mockSLSClient{}, &mockCMSClient{})
	var textToSQL, textToSQLOld toolkit.Tool
	for _, tool := range tools {
		if tool.Name == "sls_text_to_sql" {
			textToSQL = tool
		}
		if tool.Name == "sls_text_to_sql_old" {
			textToSQLOld = tool
		}
	}

	// Both tools should have the same input schema
	sqlSchema := textToSQL.InputSchema["properties"].(map[string]interface{})
	oldSchema := textToSQLOld.InputSchema["properties"].(map[string]interface{})

	// Check that both have the same required fields
	for key := range sqlSchema {
		if _, ok := oldSchema[key]; !ok {
			t.Errorf("sls_text_to_sql_old missing field %q from sls_text_to_sql", key)
		}
	}
}

func TestTextToSQLOld_SameHandlerBehavior(t *testing.T) {
	mock := &mockSLSClient{
		textToSQLResult: "SELECT * FROM log WHERE level = 'ERROR' LIMIT 10",
	}
	tools := SLSTools(mock, &mockCMSClient{})
	ctx := context.Background()

	var textToSQLHandler, textToSQLOldHandler func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_text_to_sql" {
			textToSQLHandler = tt.Handler
		}
		if tt.Name == "sls_text_to_sql_old" {
			textToSQLOldHandler = tt.Handler
		}
	}

	params := map[string]interface{}{
		"text":     "查找错误日志",
		"project":  "my-project",
		"logStore": "my-logstore",
		"regionId": "cn-hongkong",
	}

	// Both handlers should produce the same result
	result1, err1 := textToSQLHandler(ctx, params)
	result2, err2 := textToSQLOldHandler(ctx, params)

	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: err1=%v, err2=%v", err1, err2)
	}

	resp1 := result1.(map[string]interface{})
	resp2 := result2.(map[string]interface{})

	if resp1["error"] != resp2["error"] {
		t.Errorf("error mismatch: sls_text_to_sql=%v, sls_text_to_sql_old=%v", resp1["error"], resp2["error"])
	}

	data1 := resp1["data"].(map[string]interface{})
	data2 := resp2["data"].(map[string]interface{})

	if data1["query"] != data2["query"] {
		t.Errorf("query mismatch: sls_text_to_sql=%v, sls_text_to_sql_old=%v", data1["query"], data2["query"])
	}
}

func TestTextToSQLOld_DeprecatedDescription(t *testing.T) {
	tools := SLSTools(&mockSLSClient{}, &mockCMSClient{})
	for _, tool := range tools {
		if tool.Name == "sls_text_to_sql_old" {
			if !strings.Contains(tool.Description, "废弃") && !strings.Contains(tool.Description, "Deprecated") {
				t.Error("sls_text_to_sql_old description should mention deprecation")
			}
			return
		}
	}
	t.Error("sls_text_to_sql_old tool not found")
}

func TestSOP_Success(t *testing.T) {
	mock := &mockSLSClient{}
	cmsMock := &mockCMSClient{
		chatWithSkillResult: "要创建数据加工任务，请按以下步骤操作...",
	}
	tools := SLSTools(mock, cmsMock)
	ctx := context.Background()

	var tool func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_sop" {
			tool = tt.Handler
			break
		}
	}

	result, err := tool(ctx, map[string]interface{}{
		"text":     "如何创建数据加工",
		"regionId": "cn-hongkong",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"].(bool) {
		t.Errorf("expected error=false, got true")
	}
	data := resp["data"].(map[string]interface{})
	answer := data["answer"].(string)
	if answer == "" {
		t.Error("expected non-empty answer")
	}
}

func TestIsMetricStoreNotFoundError(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		expect bool
	}{
		{"nil error", nil, false},
		{"generic error", fmt.Errorf("timeout"), false},
		{"LogStoreNotExist", fmt.Errorf("LogStoreNotExist: store not found"), true},
		{"MetricStoreNotExist", fmt.Errorf("MetricStoreNotExist: my-store"), true},
		{"not exist", fmt.Errorf("The specified logstore does not exist"), true},
		{"connection refused", fmt.Errorf("connection refused"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isMetricStoreNotFoundError(tc.err)
			if got != tc.expect {
				t.Errorf("isMetricStoreNotFoundError(%v) = %v, want %v", tc.err, got, tc.expect)
			}
		})
	}
}

func TestExecuteSQL_Success(t *testing.T) {
	mock := &mockSLSClient{
		queryResult: []map[string]interface{}{
			{"__time__": "1234567890", "content": "test log 1"},
			{"__time__": "1234567891", "content": "test log 2"},
		},
	}
	tools := SLSTools(mock, &mockCMSClient{})
	ctx := context.Background()

	var tool func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_execute_sql" {
			tool = tt.Handler
			break
		}
	}

	result, err := tool(ctx, map[string]interface{}{
		"project":   "my-project",
		"logStore":  "my-logstore",
		"query":     "* | limit 10",
		"regionId":  "cn-hongkong",
		"from_time": "now-1h",
		"to_time":   "now",
		"limit":     10,
		"offset":    0,
		"reverse":   false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"].(bool) {
		t.Errorf("expected error=false, got true: %s", resp["message"])
	}
	data := resp["data"].([]map[string]interface{})
	if len(data) != 2 {
		t.Errorf("expected 2 results, got %d", len(data))
	}
}

func TestExecuteSQL_WithPagination(t *testing.T) {
	mock := &mockSLSClient{
		queryResult: []map[string]interface{}{
			{"__time__": "1234567890", "content": "test log"},
		},
	}
	tools := SLSTools(mock, &mockCMSClient{})
	ctx := context.Background()

	var tool func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_execute_sql" {
			tool = tt.Handler
			break
		}
	}

	result, err := tool(ctx, map[string]interface{}{
		"project":   "my-project",
		"logStore":  "my-logstore",
		"query":     "* | limit 10",
		"regionId":  "cn-hongkong",
		"from_time": "now-1h",
		"to_time":   "now",
		"limit":     5,
		"offset":    10,
		"reverse":   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"].(bool) {
		t.Errorf("expected error=false, got true: %s", resp["message"])
	}
}

func TestExecuteSPL_Success(t *testing.T) {
	mockCMS := &mockCMSClient{
		executeSPLResult: map[string]interface{}{
			"data": []map[string]interface{}{
				{"field1": "value1", "field2": "value2"},
			},
		},
	}
	tools := SLSTools(&mockSLSClient{}, mockCMS)
	ctx := context.Background()

	var tool func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_execute_spl" {
			tool = tt.Handler
			break
		}
	}

	result, err := tool(ctx, map[string]interface{}{
		"query":     ".entity_set with(domain='apm') | stats count(1) by __domain__",
		"workspace": "my-workspace",
		"regionId":  "cn-hongkong",
		"from_time": "now-5m",
		"to_time":   "now",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"].(bool) {
		t.Errorf("expected error=false, got true: %s", resp["message"])
	}
}

func TestGetContextLogs_Success(t *testing.T) {
	mock := &mockSLSClient{
		getContextLogsResult: map[string]interface{}{
			"logs": []map[string]interface{}{
				{"__index_number__": "-1", "content": "previous log"},
				{"__index_number__": "0", "content": "target log"},
				{"__index_number__": "1", "content": "next log"},
			},
			"total_lines":   int64(3),
			"back_lines":    int64(1),
			"forward_lines": int64(1),
			"progress":      "Complete",
		},
	}
	tools := SLSTools(mock, &mockCMSClient{})
	ctx := context.Background()

	var tool func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_get_context_logs" {
			tool = tt.Handler
			break
		}
	}

	result, err := tool(ctx, map[string]interface{}{
		"project":       "my-project",
		"logStore":      "my-logstore",
		"pack_id":       "ABCDE-12345-FGHIJ-67890",
		"pack_meta":     "logstore-1|MTY5MjAwMDAwMA==|12345|67890",
		"regionId":      "cn-hongkong",
		"back_lines":    10,
		"forward_lines": 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"].(bool) {
		t.Errorf("expected error=false, got true: %s", resp["message"])
	}
}

func TestGetContextLogs_InvalidParams(t *testing.T) {
	mock := &mockSLSClient{}
	tools := SLSTools(mock, &mockCMSClient{})
	ctx := context.Background()

	var tool func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_get_context_logs" {
			tool = tt.Handler
			break
		}
	}

	// Test with both back_lines and forward_lines set to 0
	result, err := tool(ctx, map[string]interface{}{
		"project":       "my-project",
		"logStore":      "my-logstore",
		"pack_id":       "ABCDE-12345-FGHIJ-67890",
		"pack_meta":     "logstore-1|MTY5MjAwMDAwMA==|12345|67890",
		"regionId":      "cn-hongkong",
		"back_lines":    0,
		"forward_lines": 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if !resp["error"].(bool) {
		t.Errorf("expected error=true when both back_lines and forward_lines are 0")
	}
}

func TestGetContextLogs_InvalidPackValues(t *testing.T) {
	mock := &mockSLSClient{}
	tools := SLSTools(mock, &mockCMSClient{})
	ctx := context.Background()

	var tool func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_get_context_logs" {
			tool = tt.Handler
			break
		}
	}

	tests := []struct {
		name     string
		packID   string
		packMeta string
	}{
		{"placeholder pack_id", "test-pack-id", "logstore-1|MTY5MjAwMDAwMA==|12345|67890"},
		{"placeholder pack_meta", "ABCDE-12345-FGHIJ-67890", "test-pack-meta"},
		{"short pack_id", "ab", "logstore-1|MTY5MjAwMDAwMA==|12345|67890"},
		{"short pack_meta", "ABCDE-12345-FGHIJ-67890", "cd"},
		{"example keyword", "example-value-here", "logstore-1|MTY5MjAwMDAwMA==|12345|67890"},
		{"dummy keyword", "ABCDE-12345-FGHIJ-67890", "dummy-meta-value"},
		{"fake keyword", "fake-pack-id-value", "logstore-1|MTY5MjAwMDAwMA==|12345|67890"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool(ctx, map[string]interface{}{
				"project":       "my-project",
				"logStore":      "my-logstore",
				"pack_id":       tc.packID,
				"pack_meta":     tc.packMeta,
				"regionId":      "cn-hongkong",
				"back_lines":    10,
				"forward_lines": 10,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			resp := result.(map[string]interface{})
			if !resp["error"].(bool) {
				t.Errorf("expected error=true for invalid pack values")
			}
			msg := resp["message"].(string)
			if !strings.Contains(msg, "pack_id") {
				t.Errorf("error message should mention pack_id, got: %s", msg)
			}
			if !strings.Contains(msg, "with_pack_meta") {
				t.Errorf("error message should explain how to get valid values, got: %s", msg)
			}
		})
	}
}

func TestGetContextLogs_APIErrorIncludesHint(t *testing.T) {
	mock := &mockSLSClient{
		getContextLogsErr: fmt.Errorf("PackIDNotExist: pack_id not found"),
	}
	tools := SLSTools(mock, &mockCMSClient{})
	ctx := context.Background()

	var tool func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_get_context_logs" {
			tool = tt.Handler
			break
		}
	}

	result, err := tool(ctx, map[string]interface{}{
		"project":       "my-project",
		"logStore":      "my-logstore",
		"pack_id":       "ABCDE-12345-FGHIJ-67890",
		"pack_meta":     "logstore-1|MTY5MjAwMDAwMA==|12345|67890",
		"regionId":      "cn-hongkong",
		"back_lines":    10,
		"forward_lines": 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if !resp["error"].(bool) {
		t.Errorf("expected error=true when API fails")
	}
	msg := resp["message"].(string)
	if !strings.Contains(msg, "PackIDNotExist") {
		t.Errorf("error message should contain original error, got: %s", msg)
	}
	if !strings.Contains(msg, "with_pack_meta") {
		t.Errorf("error message should include hint about how to get valid values, got: %s", msg)
	}
}

func TestIsInvalidPackValue(t *testing.T) {
	tests := []struct {
		value   string
		invalid bool
	}{
		{"", true},
		{"ab", true},
		{"abcd", true},
		{"test-pack-id", true},
		{"my-Test-value", true},
		{"placeholder-123", true},
		{"example-value", true},
		{"dummy-data", true},
		{"fake-id-here", true},
		{"mock-value-1", true},
		{"sample-pack", true},
		{"ABCDE-12345-FGHIJ-67890", false},
		{"logstore-1|MTY5MjAwMDAwMA==|12345|67890", false},
		{"fad234530fe80e7ecde594b738006090", false},
		{"hwx28v3j7p@1ce6b4ed38de9e9edd982", false},
	}

	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			got := isInvalidPackValue(tc.value)
			if got != tc.invalid {
				t.Errorf("isInvalidPackValue(%q) = %v, want %v", tc.value, got, tc.invalid)
			}
		})
	}
}

func TestTextToSPL_Success(t *testing.T) {
	mock := &mockSLSClient{}
	cmsMock := &mockCMSClient{
		chatWithSkillResult: "* | parse-json content | project field1, field2",
	}
	tools := SLSTools(mock, cmsMock)
	ctx := context.Background()

	var tool func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_text_to_spl" {
			tool = tt.Handler
			break
		}
	}

	result, err := tool(ctx, map[string]interface{}{
		"text":        "提取field1和field2字段",
		"project":     "my-project",
		"logStore":    "my-logstore",
		"data_sample": []interface{}{map[string]interface{}{"content": "{\"field1\":\"value1\"}"}},
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
	query := data["query"].(string)
	if query == "" {
		t.Error("expected non-empty query")
	}
}

func TestLogExplore_Success(t *testing.T) {
	mock := &mockSLSClient{
		queryResult: []map[string]interface{}{
			{"cnt": 100}, // First query returns count
		},
	}
	tools := SLSTools(mock, &mockCMSClient{})
	ctx := context.Background()

	var tool func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_log_explore" {
			tool = tt.Handler
			break
		}
	}

	result, err := tool(ctx, map[string]interface{}{
		"project":      "my-project",
		"logStore":     "my-logstore",
		"regionId":     "cn-hongkong",
		"logField":     "content",
		"from_time":    "now-1h",
		"to_time":      "now",
		"max_patterns": 10,
		"sample_size":  1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	// sls_log_explore returns {patterns: [], message: "..."} format
	if _, ok := resp["patterns"]; !ok {
		t.Error("expected patterns field in response")
	}
	if _, ok := resp["message"]; !ok {
		t.Error("expected message field in response")
	}
}

func TestLogCompare_Success(t *testing.T) {
	mock := &mockSLSClient{
		queryResult: []map[string]interface{}{
			{"cnt": 100}, // First query returns count
		},
	}
	tools := SLSTools(mock, &mockCMSClient{})
	ctx := context.Background()

	var tool func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_log_compare" {
			tool = tt.Handler
			break
		}
	}

	result, err := tool(ctx, map[string]interface{}{
		"project":           "my-project",
		"logStore":          "my-logstore",
		"regionId":          "cn-hongkong",
		"logField":          "content",
		"test_from_time":    "now-1h",
		"test_to_time":      "now",
		"control_from_time": "now-3h",
		"control_to_time":   "now-2h",
		"max_patterns":      10,
		"sample_size":       1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	// sls_log_compare returns {patterns: [], message: "..."} format
	if _, ok := resp["patterns"]; !ok {
		t.Error("expected patterns field in response")
	}
	if _, ok := resp["message"]; !ok {
		t.Error("expected message field in response")
	}
}

func TestLogCompare_MissingParams(t *testing.T) {
	mock := &mockSLSClient{}
	tools := SLSTools(mock, &mockCMSClient{})
	ctx := context.Background()

	var tool func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_log_compare" {
			tool = tt.Handler
			break
		}
	}

	result, err := tool(ctx, map[string]interface{}{
		"project":  "my-project",
		"logStore": "my-logstore",
		// Missing regionId
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if !resp["error"].(bool) {
		t.Errorf("expected error=true for missing regionId")
	}
}

func TestLogCompare_ClientError(t *testing.T) {
	mock := &mockSLSClient{
		queryErr: fmt.Errorf("connection refused"),
	}
	tools := SLSTools(mock, &mockCMSClient{})
	ctx := context.Background()

	var tool func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_log_compare" {
			tool = tt.Handler
			break
		}
	}

	result, err := tool(ctx, map[string]interface{}{
		"project":           "my-project",
		"logStore":          "my-logstore",
		"regionId":          "cn-hongkong",
		"logField":          "content",
		"test_from_time":    "now-1h",
		"test_to_time":      "now",
		"control_from_time": "now-3h",
		"control_to_time":   "now-2h",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	// sls_log_compare returns {patterns: [], message: "..."} format on error
	msg := resp["message"].(string)
	if !strings.Contains(msg, "connection refused") {
		t.Errorf("expected error message to contain 'connection refused', got %q", msg)
	}
}

func TestLogCompare_WithDifferentPatterns(t *testing.T) {
	// This test simulates different patterns in test vs control
	mock := &mockSLSClient{
		queryResult: []map[string]interface{}{
			{"cnt": 100}, // First query returns count
		},
	}

	tools := SLSTools(mock, &mockCMSClient{})
	ctx := context.Background()

	var tool func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_log_compare" {
			tool = tt.Handler
			break
		}
	}

	result, err := tool(ctx, map[string]interface{}{
		"project":           "my-project",
		"logStore":          "my-logstore",
		"regionId":          "cn-hongkong",
		"logField":          "content",
		"test_from_time":    "now-1h",
		"test_to_time":      "now",
		"control_from_time": "now-3h",
		"control_to_time":   "now-2h",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	// sls_log_compare returns {patterns: [], message: "..."} format
	if _, ok := resp["patterns"]; !ok {
		t.Error("expected patterns field in response")
	}
}

// ===========================================================================
// Tests for sls_execute_spl (workspace/CMS mode only)
// ===========================================================================

func TestExecuteSPL_CMSError(t *testing.T) {
	mockCMS := &mockCMSClient{
		executeSPLErr: fmt.Errorf("CMS service unavailable"),
	}
	tools := SLSTools(&mockSLSClient{}, mockCMS)
	ctx := context.Background()

	var tool func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_execute_spl" {
			tool = tt.Handler
			break
		}
	}

	result, err := tool(ctx, map[string]interface{}{
		"query":     ".entity_set with(domain='apm')",
		"workspace": "my-workspace",
		"regionId":  "cn-hongkong",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if !resp["error"].(bool) {
		t.Errorf("expected error=true for CMS error")
	}
	msg := resp["message"].(string)
	if !strings.Contains(msg, "CMS service unavailable") {
		t.Errorf("expected error message to contain 'CMS service unavailable', got %q", msg)
	}
}

func TestExecuteSPL_MissingWorkspace(t *testing.T) {
	tools := SLSTools(&mockSLSClient{}, &mockCMSClient{})
	ctx := context.Background()

	var tool func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_execute_spl" {
			tool = tt.Handler
			break
		}
	}

	result, err := tool(ctx, map[string]interface{}{
		"query":    ".entity_set with(domain='apm')",
		"regionId": "cn-hongkong",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if !resp["error"].(bool) {
		t.Errorf("expected error=true when workspace is missing")
	}
}

func TestExecuteSPL_MissingRequiredParams(t *testing.T) {
	tools := SLSTools(&mockSLSClient{}, &mockCMSClient{})
	ctx := context.Background()

	var tool func(context.Context, map[string]interface{}) (interface{}, error)
	for _, tt := range tools {
		if tt.Name == "sls_execute_spl" {
			tool = tt.Handler
			break
		}
	}

	testCases := []struct {
		name   string
		params map[string]interface{}
	}{
		{
			name: "missing query",
			params: map[string]interface{}{
				"workspace": "my-workspace",
				"regionId":  "cn-hongkong",
			},
		},
		{
			name: "missing regionId",
			params: map[string]interface{}{
				"query":     ".entity_set with(domain='apm')",
				"workspace": "my-workspace",
			},
		},
		{
			name: "empty query",
			params: map[string]interface{}{
				"query":     "",
				"workspace": "my-workspace",
				"regionId":  "cn-hongkong",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool(ctx, tc.params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			resp := result.(map[string]interface{})
			if !resp["error"].(bool) {
				t.Errorf("expected error=true for %s", tc.name)
			}
		})
	}
}
