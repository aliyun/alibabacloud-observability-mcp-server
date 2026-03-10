package paas

import (
	"context"
	"fmt"
	"testing"

	"github.com/alibabacloud-observability-mcp-server-go/internal/client"
)

// ---------------------------------------------------------------------------
// DataAgentTools returns correct number of tools
// ---------------------------------------------------------------------------

func TestDataAgentTools_Count(t *testing.T) {
	tools := DataAgentTools(&mockCMSClient{})
	// 1 main tool + 1 alias (cms_natural_language_query)
	if got := len(tools); got != 2 {
		t.Fatalf("DataAgentTools() returned %d tools, want 2", got)
	}
}

func TestDataAgentTools_Name(t *testing.T) {
	tools := DataAgentTools(&mockCMSClient{})
	if tools[0].Name != "umodel_data_agent_query" {
		t.Errorf("tool name = %q, want %q", tools[0].Name, "umodel_data_agent_query")
	}
}

func TestDataAgentTools_HasUmodelPrefix(t *testing.T) {
	tools := DataAgentTools(&mockCMSClient{})
	for _, tool := range tools {
		// Allow both umodel_ prefix and cms_natural_language_query alias
		if tool.Name != "cms_natural_language_query" && (len(tool.Name) < 7 || tool.Name[:7] != "umodel_") {
			t.Errorf("tool %q does not have umodel_ prefix", tool.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// Handler tests: missing params validation
// ---------------------------------------------------------------------------

func TestHandleDataAgentQuery_MissingQuery(t *testing.T) {
	tools := DataAgentTools(&mockCMSClient{})
	handler := tools[0].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"workspace": "test-ws",
		"regionId":  "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true for missing query")
	}
}

func TestHandleDataAgentQuery_MissingWorkspace(t *testing.T) {
	tools := DataAgentTools(&mockCMSClient{})
	handler := tools[0].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"query":    "test query",
		"regionId": "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true for missing workspace")
	}
}

func TestHandleDataAgentQuery_MissingRegionId(t *testing.T) {
	tools := DataAgentTools(&mockCMSClient{})
	handler := tools[0].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"query":     "test query",
		"workspace": "test-ws",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true for missing regionId")
	}
}

func TestHandleDataAgentQuery_AllParamsMissing(t *testing.T) {
	tools := DataAgentTools(&mockCMSClient{})
	handler := tools[0].Handler

	result, err := handler(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true for all params missing")
	}
}

// ---------------------------------------------------------------------------
// Handler tests: successful execution via DataAgentQuery API
// ---------------------------------------------------------------------------

func TestHandleDataAgentQuery_Success(t *testing.T) {
	var capturedQuery string
	var capturedWorkspace string
	var capturedRegion string
	mock := &mockCMSClient{
		dataAgentQueryFn: func(_ context.Context, region, workspace, query string, _, _ int64) (*client.DataAgentResult, error) {
			capturedQuery = query
			capturedWorkspace = workspace
			capturedRegion = region
			return &client.DataAgentResult{
				QueryResults: []interface{}{
					map[string]interface{}{"type": "entity_list", "data": []interface{}{
						map[string]interface{}{"service": "payment", "requests": 1000},
					}},
				},
				Message: "查询完成，找到1个服务",
				TraceID: "test-trace-123",
			}, nil
		},
	}

	tools := DataAgentTools(mock)
	handler := tools[0].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"query":     "查询请求量最高的服务",
		"workspace": "test-ws",
		"regionId":  "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	if capturedWorkspace != "test-ws" {
		t.Errorf("workspace = %q, want %q", capturedWorkspace, "test-ws")
	}
	if capturedRegion != "cn-hangzhou" {
		t.Errorf("region = %q, want %q", capturedRegion, "cn-hangzhou")
	}
	if capturedQuery != "查询请求量最高的服务" {
		t.Errorf("query = %q, want %q", capturedQuery, "查询请求量最高的服务")
	}
	if resp["trace_id"] != "test-trace-123" {
		t.Errorf("trace_id = %v, want %q", resp["trace_id"], "test-trace-123")
	}

	// Verify data structure contains query_results
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data should be a map")
	}
	if data["query_results"] == nil {
		t.Error("data should contain query_results")
	}
}

func TestHandleDataAgentQuery_APIError(t *testing.T) {
	mock := &mockCMSClient{
		dataAgentQueryFn: func(_ context.Context, _, _, _ string, _, _ int64) (*client.DataAgentResult, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	tools := DataAgentTools(mock)
	handler := tools[0].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"query":     "test query",
		"workspace": "test-ws",
		"regionId":  "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true when API call fails")
	}
	msg := resp["message"].(string)
	if !contains(msg, "connection refused") {
		t.Errorf("error message should contain cause, got %q", msg)
	}
}

func TestHandleDataAgentQuery_DefaultTimeRange(t *testing.T) {
	mock := &mockCMSClient{
		dataAgentQueryFn: func(_ context.Context, _, _, _ string, from, to int64) (*client.DataAgentResult, error) {
			// Verify time range is approximately 15 minutes (default)
			diff := to - from
			if diff < 800 || diff > 1000 {
				return nil, fmt.Errorf("unexpected time range diff: %d (expected ~900)", diff)
			}
			return &client.DataAgentResult{Message: "ok"}, nil
		},
	}

	tools := DataAgentTools(mock)
	handler := tools[0].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"query":     "test",
		"workspace": "test-ws",
		"regionId":  "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
}

func TestHandleDataAgentQuery_WithGeneratedSQL(t *testing.T) {
	mock := &mockCMSClient{
		dataAgentQueryFn: func(_ context.Context, _, _, _ string, _, _ int64) (*client.DataAgentResult, error) {
			return &client.DataAgentResult{
				QueryResults: []interface{}{
					map[string]interface{}{"type": "entity_list", "data": []interface{}{}},
				},
				GeneratedSQL: "SELECT * FROM logs WHERE level='ERROR'",
				Message:      "查询完成",
				TraceID:      "trace-456",
			}, nil
		},
	}

	tools := DataAgentTools(mock)
	handler := tools[0].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"query":     "查询错误日志",
		"workspace": "test-ws",
		"regionId":  "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := result.(map[string]interface{})
	data := resp["data"].(map[string]interface{})
	if data["generated_sql"] != "SELECT * FROM logs WHERE level='ERROR'" {
		t.Errorf("generated_sql = %v, want SQL string", data["generated_sql"])
	}
}

// ---------------------------------------------------------------------------
// Response structure tests
// ---------------------------------------------------------------------------

func TestHandleDataAgentQuery_ResponseStructure(t *testing.T) {
	mock := &mockCMSClient{
		dataAgentQueryFn: func(_ context.Context, _, _, _ string, _, _ int64) (*client.DataAgentResult, error) {
			return &client.DataAgentResult{
				QueryResults: []interface{}{"result1"},
				Message:      "done",
				TraceID:      "t-1",
			}, nil
		},
	}

	tools := DataAgentTools(mock)
	handler := tools[0].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"query":     "test",
		"workspace": "test-ws",
		"regionId":  "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := result.(map[string]interface{})

	// Verify all expected keys exist (matching Python response format)
	requiredKeys := []string{"error", "data", "message", "trace_id", "time_range", "timestamp"}
	for _, key := range requiredKeys {
		if _, ok := resp[key]; !ok {
			t.Errorf("response missing key %q", key)
		}
	}

	// Verify time_range structure
	tr, ok := resp["time_range"].(map[string]interface{})
	if !ok {
		t.Fatal("time_range should be a map")
	}
	trKeys := []string{"from", "to", "from_readable", "to_readable", "expression"}
	for _, key := range trKeys {
		if _, ok := tr[key]; !ok {
			t.Errorf("time_range missing key %q", key)
		}
	}
}

// ---------------------------------------------------------------------------
// cms_natural_language_query alias tests
// ---------------------------------------------------------------------------

func TestNaturalLanguageQueryAlias_Name(t *testing.T) {
	tools := DataAgentTools(&mockCMSClient{})
	aliasTool := tools[1]
	if aliasTool.Name != "cms_natural_language_query" {
		t.Errorf("alias tool name = %q, want %q", aliasTool.Name, "cms_natural_language_query")
	}
}

func TestNaturalLanguageQueryAlias_SameInputSchema(t *testing.T) {
	tools := DataAgentTools(&mockCMSClient{})
	originalTool := tools[0]
	aliasTool := tools[1]

	originalSchema := originalTool.InputSchema
	aliasSchema := aliasTool.InputSchema

	if originalSchema["type"] != aliasSchema["type"] {
		t.Errorf("InputSchema type mismatch: original=%v, alias=%v", originalSchema["type"], aliasSchema["type"])
	}

	originalRequired, _ := originalSchema["required"].([]string)
	aliasRequired, _ := aliasSchema["required"].([]string)
	if len(originalRequired) != len(aliasRequired) {
		t.Errorf("InputSchema required length mismatch: original=%d, alias=%d", len(originalRequired), len(aliasRequired))
	}

	originalProps, _ := originalSchema["properties"].(map[string]interface{})
	aliasProps, _ := aliasSchema["properties"].(map[string]interface{})
	if len(originalProps) != len(aliasProps) {
		t.Errorf("InputSchema properties count mismatch: original=%d, alias=%d", len(originalProps), len(aliasProps))
	}
	for key := range originalProps {
		if _, ok := aliasProps[key]; !ok {
			t.Errorf("alias InputSchema missing property %q", key)
		}
	}
}

func TestNaturalLanguageQueryAlias_SameHandlerBehavior(t *testing.T) {
	callCount := 0
	mock := &mockCMSClient{
		dataAgentQueryFn: func(_ context.Context, _, _, _ string, _, _ int64) (*client.DataAgentResult, error) {
			callCount++
			return &client.DataAgentResult{
				QueryResults: []interface{}{
					map[string]interface{}{"type": "entity_list", "data": []interface{}{}},
				},
				Message: "ok",
			}, nil
		},
	}

	tools := DataAgentTools(mock)
	originalHandler := tools[0].Handler
	aliasHandler := tools[1].Handler

	params := map[string]interface{}{
		"query":     "查询请求量最高的服务",
		"workspace": "test-ws",
		"regionId":  "cn-hangzhou",
	}

	result1, err1 := originalHandler(context.Background(), params)
	if err1 != nil {
		t.Fatalf("original handler error: %v", err1)
	}

	result2, err2 := aliasHandler(context.Background(), params)
	if err2 != nil {
		t.Fatalf("alias handler error: %v", err2)
	}

	// Both should call DataAgentQuery
	if callCount != 2 {
		t.Errorf("expected 2 DataAgentQuery calls, got %d", callCount)
	}

	resp1 := result1.(map[string]interface{})
	resp2 := result2.(map[string]interface{})
	if resp1["error"] != resp2["error"] {
		t.Errorf("handlers produced different error status: original=%v, alias=%v", resp1["error"], resp2["error"])
	}
}

func TestNaturalLanguageQueryAlias_MissingParams(t *testing.T) {
	tools := DataAgentTools(&mockCMSClient{})
	aliasHandler := tools[1].Handler

	result, err := aliasHandler(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true for missing params")
	}
}
