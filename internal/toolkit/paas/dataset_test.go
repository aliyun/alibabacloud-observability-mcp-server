package paas

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// DatasetTools returns correct number of tools
// ---------------------------------------------------------------------------

func TestDatasetTools_Count(t *testing.T) {
	tools := DatasetTools(&mockCMSClient{})
	if got := len(tools); got != 3 {
		t.Fatalf("DatasetTools() returned %d tools, want 3", got)
	}
}

func TestDatasetTools_Names(t *testing.T) {
	tools := DatasetTools(&mockCMSClient{})
	expected := []string{
		"umodel_list_data_set",
		"umodel_search_entity_set",
		"umodel_list_related_entity_set",
	}
	for i, tool := range tools {
		if tool.Name != expected[i] {
			t.Errorf("tool[%d].Name = %q, want %q", i, tool.Name, expected[i])
		}
	}
}

func TestDatasetTools_AllHaveUmodelPrefix(t *testing.T) {
	tools := DatasetTools(&mockCMSClient{})
	for _, tool := range tools {
		if !strings.HasPrefix(tool.Name, "umodel_") {
			t.Errorf("tool %q does not have umodel_ prefix", tool.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// umodel_list_data_set handler tests
// ---------------------------------------------------------------------------

func TestHandleListDataSet_MissingParams(t *testing.T) {
	tools := DatasetTools(&mockCMSClient{})
	handler := tools[0].Handler

	result, err := handler(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true for missing params")
	}
}

func TestHandleListDataSet_Success(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"name": "metric_set_1", "type": "metric_set"},
				},
			}, nil
		},
	}

	tools := DatasetTools(mock)
	handler := tools[0].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"workspace":       "test-ws",
		"domain":          "apm",
		"entity_set_name": "apm.service",
		"regionId":        "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	if !strings.Contains(capturedQuery, "domain='apm'") {
		t.Errorf("query should contain domain='apm', got %q", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "name='apm.service'") {
		t.Errorf("query should contain name='apm.service', got %q", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "list_data_set([]") {
		t.Errorf("query should contain list_data_set([]) for no type filter, got %q", capturedQuery)
	}
}

func TestHandleListDataSet_WithDataSetTypes(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{"data": []interface{}{}}, nil
		},
	}

	tools := DatasetTools(mock)
	handler := tools[0].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"workspace":       "test-ws",
		"domain":          "apm",
		"entity_set_name": "apm.service",
		"data_set_types":  "metric_set,log_set",
		"regionId":        "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	if !strings.Contains(capturedQuery, "['metric_set','log_set']") {
		t.Errorf("query should contain type filter, got %q", capturedQuery)
	}
}

func TestHandleListDataSet_SPLError(t *testing.T) {
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, _ string, _, _ int64, _ int) (map[string]interface{}, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	tools := DatasetTools(mock)
	handler := tools[0].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"workspace":       "test-ws",
		"domain":          "apm",
		"entity_set_name": "apm.service",
		"regionId":        "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true when SPL call fails")
	}
}

// ---------------------------------------------------------------------------
// umodel_search_entity_set handler tests
// ---------------------------------------------------------------------------

func TestHandleSearchEntitySet_MissingParams(t *testing.T) {
	tools := DatasetTools(&mockCMSClient{})
	handler := tools[1].Handler

	result, err := handler(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true for missing params")
	}
}

func TestHandleSearchEntitySet_Success(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"name": "apm.service"},
				},
			}, nil
		},
	}

	tools := DatasetTools(mock)
	handler := tools[1].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"search_text": "service",
		"workspace":   "test-ws",
		"regionId":    "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	if !strings.Contains(capturedQuery, "strpos(metadata, 'service')") {
		t.Errorf("query should contain search text filter, got %q", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "entity_set") {
		t.Errorf("query should filter for entity_set kind, got %q", capturedQuery)
	}
}

func TestHandleSearchEntitySet_WithDomainFilter(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{"data": []interface{}{}}, nil
		},
	}

	tools := DatasetTools(mock)
	handler := tools[1].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"search_text":     "host",
		"workspace":       "test-ws",
		"domain":          "infrastructure",
		"entity_set_name": "host.instance",
		"regionId":        "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	if !strings.Contains(capturedQuery, "'$.domain') = 'infrastructure'") {
		t.Errorf("query should contain domain filter, got %q", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "'$.name') = 'host.instance'") {
		t.Errorf("query should contain entity_set_name filter, got %q", capturedQuery)
	}
}

// ---------------------------------------------------------------------------
// umodel_list_related_entity_set handler tests
// ---------------------------------------------------------------------------

func TestHandleListRelatedEntitySet_MissingParams(t *testing.T) {
	tools := DatasetTools(&mockCMSClient{})
	handler := tools[2].Handler

	result, err := handler(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true for missing params")
	}
}

func TestHandleListRelatedEntitySet_Success(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"name": "apm.endpoint"},
				},
			}, nil
		},
	}

	tools := DatasetTools(mock)
	handler := tools[2].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"domain":          "apm",
		"entity_set_name": "apm.service",
		"workspace":       "test-ws",
		"regionId":        "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	if !strings.Contains(capturedQuery, "list_related_entity_set") {
		t.Errorf("query should contain list_related_entity_set, got %q", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "'both'") {
		t.Errorf("query should contain default direction 'both', got %q", capturedQuery)
	}
}

func TestHandleListRelatedEntitySet_WithOptions(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{"data": []interface{}{}}, nil
		},
	}

	tools := DatasetTools(mock)
	handler := tools[2].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"domain":          "apm",
		"entity_set_name": "apm.service",
		"workspace":       "test-ws",
		"relation_type":   "calls",
		"direction":       "out",
		"detail":          true,
		"regionId":        "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	if !strings.Contains(capturedQuery, "'calls'") {
		t.Errorf("query should contain relation_type 'calls', got %q", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "'out'") {
		t.Errorf("query should contain direction 'out', got %q", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "true") {
		t.Errorf("query should contain detail=true, got %q", capturedQuery)
	}
}

func TestHandleListRelatedEntitySet_SPLError(t *testing.T) {
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, _ string, _, _ int64, _ int) (map[string]interface{}, error) {
			return nil, fmt.Errorf("timeout")
		},
	}

	tools := DatasetTools(mock)
	handler := tools[2].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"domain":          "apm",
		"entity_set_name": "apm.service",
		"workspace":       "test-ws",
		"regionId":        "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true when SPL call fails")
	}
}
