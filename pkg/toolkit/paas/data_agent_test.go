package paas

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/alibabacloud-observability-mcp-server-go/pkg/client"
)

// ---------------------------------------------------------------------------
// DataAgentTools returns correct number of tools
// ---------------------------------------------------------------------------

func TestDataAgentTools_Count(t *testing.T) {
	tools := DataAgentTools(&mockCMSClient{})
	// 1 tool: cms_natural_language_query
	if got := len(tools); got != 1 {
		t.Fatalf("DataAgentTools() returned %d tools, want 1", got)
	}
}

func TestDataAgentTools_Name(t *testing.T) {
	tools := DataAgentTools(&mockCMSClient{})
	if tools[0].Name != "cms_natural_language_query" {
		t.Errorf("tool name = %q, want %q", tools[0].Name, "cms_natural_language_query")
	}
}

func TestDataAgentTools_HasUmodelPrefix(t *testing.T) {
	tools := DataAgentTools(&mockCMSClient{})
	for _, tool := range tools {
		if !strings.HasPrefix(tool.Name, "cms_") {
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
		"regionId":  "cn-hongkong",
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
		"regionId": "cn-hongkong",
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
		"regionId":  "cn-hongkong",
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
	if capturedRegion != "cn-hongkong" {
		t.Errorf("region = %q, want %q", capturedRegion, "cn-hongkong")
	}
	if capturedQuery != "查询请求量最高的服务" {
		t.Errorf("query = %q, want %q", capturedQuery, "查询请求量最高的服务")
	}
	if resp["trace_id"] != "test-trace-123" {
		t.Errorf("trace_id = %v, want %q", resp["trace_id"], "test-trace-123")
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
		"regionId":  "cn-hongkong",
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
		"regionId":  "cn-hongkong",
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
		"regionId":  "cn-hongkong",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := result.(map[string]interface{})
	if resp["message"] != "查询完成" {
		t.Errorf("message = %v, want %q", resp["message"], "查询完成")
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
		"regionId":  "cn-hongkong",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := result.(map[string]interface{})

	// Verify all expected keys exist
	requiredKeys := []string{"error", "message", "trace_id", "timestamp"}
	for _, key := range requiredKeys {
		if _, ok := resp[key]; !ok {
			t.Errorf("response missing key %q", key)
		}
	}
}
