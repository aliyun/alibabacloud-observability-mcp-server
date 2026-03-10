package paas

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// DataTools returns correct number of tools
// ---------------------------------------------------------------------------

func TestDataTools_Count(t *testing.T) {
	tools := DataTools(&mockCMSClient{})
	// 8 main tools + 1 alias (umodel_get_profiles)
	if got := len(tools); got != 9 {
		t.Fatalf("DataTools() returned %d tools, want 9", got)
	}
}

func TestDataTools_Names(t *testing.T) {
	tools := DataTools(&mockCMSClient{})
	expected := []string{
		"umodel_get_metrics",
		"umodel_get_golden_metrics",
		"umodel_get_relation_metrics",
		"umodel_get_logs",
		"umodel_get_events",
		"umodel_get_traces",
		"umodel_search_traces",
		"umodel_get_profiling",
		"umodel_get_profiles", // alias for umodel_get_profiling
	}
	for i, tool := range tools {
		if tool.Name != expected[i] {
			t.Errorf("tool[%d].Name = %q, want %q", i, tool.Name, expected[i])
		}
	}
}

func TestDataTools_AllHaveUmodelPrefix(t *testing.T) {
	tools := DataTools(&mockCMSClient{})
	for _, tool := range tools {
		if !strings.HasPrefix(tool.Name, "umodel_") {
			t.Errorf("tool %q does not have umodel_ prefix", tool.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// umodel_get_metrics handler tests
// ---------------------------------------------------------------------------

func TestHandleGetMetrics_MissingParams(t *testing.T) {
	tools := DataTools(&mockCMSClient{})
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

func TestHandleGetMetrics_Success(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"__name__": "cpu_usage", "__value__": 0.85},
				},
			}, nil
		},
	}

	tools := DataTools(mock)
	handler := tools[0].Handler

	// Use apm.metric.service which is compatible with apm.service
	result, err := handler(context.Background(), map[string]interface{}{
		"domain":             "apm",
		"entity_set_name":    "apm.service",
		"metric_domain_name": "apm.metric.service",
		"metric":             "cpu_usage",
		"workspace":          "test-ws",
		"regionId":           "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	if !strings.Contains(capturedQuery, "get_metric") {
		t.Errorf("query should contain get_metric, got %q", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "apm.metric.service") {
		t.Errorf("query should contain metric_domain_name, got %q", capturedQuery)
	}
}

func TestHandleGetMetrics_WithEntityIDs(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{"data": []interface{}{}}, nil
		},
	}

	tools := DataTools(mock)
	handler := tools[0].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"domain":             "apm",
		"entity_set_name":    "apm.service",
		"metric_domain_name": "apm.metric.service",
		"metric":             "cpu_usage",
		"workspace":          "test-ws",
		"regionId":           "cn-hangzhou",
		"entity_ids":         "svc-1,svc-2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	if !strings.Contains(capturedQuery, "ids=['svc-1','svc-2']") {
		t.Errorf("query should contain entity IDs, got %q", capturedQuery)
	}
}

func TestHandleGetMetrics_SPLError(t *testing.T) {
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, _ string, _, _ int64, _ int) (map[string]interface{}, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	tools := DataTools(mock)
	handler := tools[0].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"domain":             "apm",
		"entity_set_name":    "apm.service",
		"metric_domain_name": "apm.metric.service",
		"metric":             "cpu_usage",
		"workspace":          "test-ws",
		"regionId":           "cn-hangzhou",
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
// umodel_get_metrics: metric compatibility validation tests
// ---------------------------------------------------------------------------

func TestHandleGetMetrics_IncompatibleMetricDomain(t *testing.T) {
	// apm.metric.jvm is NOT compatible with apm.service — should be apm.instance
	mock := &mockCMSClient{}
	tools := DataTools(mock)
	handler := tools[0].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"domain":             "apm",
		"entity_set_name":    "apm.service",
		"metric_domain_name": "apm.metric.jvm",
		"metric":             "arms_jvm_mem_used_bytes",
		"workspace":          "test-ws",
		"regionId":           "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true for incompatible metric_domain_name")
	}
	msg, _ := resp["message"].(string)
	if !strings.Contains(msg, "not compatible") {
		t.Errorf("message should mention incompatibility, got %q", msg)
	}
	if !strings.Contains(msg, "apm.instance") {
		t.Errorf("message should suggest apm.instance, got %q", msg)
	}
}

func TestHandleGetMetrics_CompatibleCombinationPassesThrough(t *testing.T) {
	called := false
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, _ string, _, _ int64, _ int) (map[string]interface{}, error) {
			called = true
			return map[string]interface{}{"data": []interface{}{}}, nil
		},
	}
	tools := DataTools(mock)
	handler := tools[0].Handler

	// apm.instance + apm.metric.jvm is a valid combination
	result, err := handler(context.Background(), map[string]interface{}{
		"domain":             "apm",
		"entity_set_name":    "apm.instance",
		"metric_domain_name": "apm.metric.jvm",
		"metric":             "arms_jvm_mem_used_bytes",
		"workspace":          "test-ws",
		"regionId":           "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false for compatible combination, got message: %v", resp["message"])
	}
	if !called {
		t.Error("expected SPL to be executed for compatible combination")
	}
}

func TestHandleGetMetrics_UnknownEntitySetSkipsValidation(t *testing.T) {
	called := false
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, _ string, _, _ int64, _ int) (map[string]interface{}, error) {
			called = true
			return map[string]interface{}{"data": []interface{}{}}, nil
		},
	}
	tools := DataTools(mock)
	handler := tools[0].Handler

	// host.instance is not in the compatibility map — should pass through
	result, err := handler(context.Background(), map[string]interface{}{
		"domain":             "host",
		"entity_set_name":    "host.instance",
		"metric_domain_name": "host.metric.cpu",
		"metric":             "cpu_usage",
		"workspace":          "test-ws",
		"regionId":           "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false for unknown entity set, got message: %v", resp["message"])
	}
	if !called {
		t.Error("expected SPL to be executed for unknown entity set")
	}
}

func TestValidateMetricCompatibility(t *testing.T) {
	tests := []struct {
		name              string
		entitySetName     string
		metricDomainName  string
		wantEmpty         bool
		wantContains      string
	}{
		{
			name:             "apm.service with apm.metric.service is compatible",
			entitySetName:    "apm.service",
			metricDomainName: "apm.metric.service",
			wantEmpty:        true,
		},
		{
			name:             "apm.service with apm.metric.exception is compatible",
			entitySetName:    "apm.service",
			metricDomainName: "apm.metric.exception",
			wantEmpty:        true,
		},
		{
			name:             "apm.service with apm.metric.jvm is incompatible",
			entitySetName:    "apm.service",
			metricDomainName: "apm.metric.jvm",
			wantEmpty:        false,
			wantContains:     "apm.instance",
		},
		{
			name:             "apm.instance with apm.metric.jvm is compatible",
			entitySetName:    "apm.instance",
			metricDomainName: "apm.metric.jvm",
			wantEmpty:        true,
		},
		{
			name:             "k8s.pod with k8s.metric.pod is compatible",
			entitySetName:    "k8s.pod",
			metricDomainName: "k8s.metric.pod",
			wantEmpty:        true,
		},
		{
			name:             "k8s.pod with apm.metric.jvm is incompatible",
			entitySetName:    "k8s.pod",
			metricDomainName: "apm.metric.jvm",
			wantEmpty:        false,
			wantContains:     "apm.instance",
		},
		{
			name:             "unknown entity set skips validation",
			entitySetName:    "custom.entity",
			metricDomainName: "custom.metric",
			wantEmpty:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateMetricCompatibility(tt.entitySetName, tt.metricDomainName)
			if tt.wantEmpty && result != "" {
				t.Errorf("expected empty result, got %q", result)
			}
			if !tt.wantEmpty && result == "" {
				t.Error("expected non-empty result for incompatible combination")
			}
			if tt.wantContains != "" && !strings.Contains(result, tt.wantContains) {
				t.Errorf("expected result to contain %q, got %q", tt.wantContains, result)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// umodel_get_golden_metrics handler tests
// ---------------------------------------------------------------------------

func TestHandleGetGoldenMetrics_MissingParams(t *testing.T) {
	tools := DataTools(&mockCMSClient{})
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

func TestHandleGetGoldenMetrics_Success(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"metric": "latency", "__value__": []float64{100.5}},
				},
			}, nil
		},
	}

	tools := DataTools(mock)
	handler := tools[1].Handler

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
	if !strings.Contains(capturedQuery, "get_golden_metrics") {
		t.Errorf("query should contain get_golden_metrics, got %q", capturedQuery)
	}
}

func TestHandleGetGoldenMetrics_InvalidQueryType(t *testing.T) {
	tools := DataTools(&mockCMSClient{})
	handler := tools[1].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"domain":          "apm",
		"entity_set_name": "apm.service",
		"workspace":       "test-ws",
		"regionId":        "cn-hangzhou",
		"query_type":      "invalid",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true for invalid query_type")
	}
}

// ---------------------------------------------------------------------------
// umodel_get_relation_metrics handler tests
// ---------------------------------------------------------------------------

func TestHandleGetRelationMetrics_MissingParams(t *testing.T) {
	tools := DataTools(&mockCMSClient{})
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

func TestHandleGetRelationMetrics_Success(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"metric": "latency", "__value__": []float64{50.5}},
				},
			}, nil
		},
	}

	tools := DataTools(mock)
	handler := tools[2].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"src_domain":          "apm",
		"src_entity_set_name": "apm.service",
		"relation_type":       "calls",
		"metric_set_domain":   "apm",
		"metric":              "latency",
		"workspace":           "test-ws",
		"regionId":            "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	if !strings.Contains(capturedQuery, "get_relation_metric") {
		t.Errorf("query should contain get_relation_metric, got %q", capturedQuery)
	}
}

func TestHandleGetRelationMetrics_AutoGenerateMetricSetName(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{"data": []interface{}{}}, nil
		},
	}

	tools := DataTools(mock)
	handler := tools[2].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"src_domain":          "apm",
		"src_entity_set_name": "apm.service",
		"relation_type":       "calls",
		"metric_set_domain":   "apm",
		"metric":              "latency",
		"workspace":           "test-ws",
		"regionId":            "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	// metric_set_name should be auto-generated as "calls.apm.service"
	if !strings.Contains(capturedQuery, "calls.apm.service") {
		t.Errorf("query should contain auto-generated metric_set_name 'calls.apm.service', got %q", capturedQuery)
	}
}

func TestHandleGetRelationMetrics_WithEntityIDs(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{"data": []interface{}{}}, nil
		},
	}

	tools := DataTools(mock)
	handler := tools[2].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"src_domain":          "apm",
		"src_entity_set_name": "apm.service",
		"src_entity_ids":      "svc-1,svc-2",
		"relation_type":       "calls",
		"metric_set_domain":   "apm",
		"metric":              "latency",
		"workspace":           "test-ws",
		"regionId":            "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	if !strings.Contains(capturedQuery, "ids=['svc-1','svc-2']") {
		t.Errorf("query should contain entity IDs, got %q", capturedQuery)
	}
}

func TestHandleGetRelationMetrics_InvalidDirection(t *testing.T) {
	tools := DataTools(&mockCMSClient{})
	handler := tools[2].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"src_domain":          "apm",
		"src_entity_set_name": "apm.service",
		"relation_type":       "calls",
		"metric_set_domain":   "apm",
		"metric":              "latency",
		"workspace":           "test-ws",
		"regionId":            "cn-hangzhou",
		"direction":           "invalid",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true for invalid direction")
	}
}

func TestHandleGetRelationMetrics_InvalidQueryType(t *testing.T) {
	tools := DataTools(&mockCMSClient{})
	handler := tools[2].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"src_domain":          "apm",
		"src_entity_set_name": "apm.service",
		"relation_type":       "calls",
		"metric_set_domain":   "apm",
		"metric":              "latency",
		"workspace":           "test-ws",
		"regionId":            "cn-hangzhou",
		"query_type":          "invalid",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true for invalid query_type")
	}
}

func TestHandleGetRelationMetrics_SPLError(t *testing.T) {
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, _ string, _, _ int64, _ int) (map[string]interface{}, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	tools := DataTools(mock)
	handler := tools[2].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"src_domain":          "apm",
		"src_entity_set_name": "apm.service",
		"relation_type":       "calls",
		"metric_set_domain":   "apm",
		"metric":              "latency",
		"workspace":           "test-ws",
		"regionId":            "cn-hangzhou",
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
// umodel_get_logs handler tests
// ---------------------------------------------------------------------------

func TestHandleGetLogs_MissingParams(t *testing.T) {
	tools := DataTools(&mockCMSClient{})
	handler := tools[3].Handler

	result, err := handler(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true for missing params")
	}
}

func TestHandleGetLogs_Success(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"content": "error log message"},
				},
			}, nil
		},
	}

	tools := DataTools(mock)
	handler := tools[3].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"domain":          "apm",
		"entity_set_name": "apm.service",
		"log_set_domain":  "apm",
		"log_set_name":    "apm.log.apm.service",
		"workspace":       "test-ws",
		"regionId":        "cn-hangzhou",
		"entity_ids":      "svc-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	if !strings.Contains(capturedQuery, "get_log") {
		t.Errorf("query should contain get_log, got %q", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "apm.log.apm.service") {
		t.Errorf("query should contain log_set_name, got %q", capturedQuery)
	}
}

// ---------------------------------------------------------------------------
// umodel_get_events handler tests
// ---------------------------------------------------------------------------

func TestHandleGetEvents_MissingParams(t *testing.T) {
	tools := DataTools(&mockCMSClient{})
	handler := tools[4].Handler

	result, err := handler(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true for missing params")
	}
}

func TestHandleGetEvents_Success(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"event_type": "deployment"},
				},
			}, nil
		},
	}

	tools := DataTools(mock)
	handler := tools[4].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"domain":           "apm",
		"entity_set_name":  "apm.service",
		"event_set_domain": "default",
		"event_set_name":   "default.event.common",
		"workspace":        "test-ws",
		"regionId":         "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	if !strings.Contains(capturedQuery, "get_event") {
		t.Errorf("query should contain get_event, got %q", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "default.event.common") {
		t.Errorf("query should contain event_set_name, got %q", capturedQuery)
	}
}

// ---------------------------------------------------------------------------
// umodel_get_traces handler tests
// ---------------------------------------------------------------------------

func TestHandleGetTraces_MissingParams(t *testing.T) {
	tools := DataTools(&mockCMSClient{})
	handler := tools[5].Handler

	result, err := handler(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true for missing params")
	}
}

func TestHandleGetTraces_Success(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"traceId": "trace-1", "duration_ms": 150.5},
				},
			}, nil
		},
	}

	tools := DataTools(mock)
	handler := tools[5].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"domain":           "apm",
		"entity_set_name":  "apm.service",
		"trace_set_domain": "apm",
		"trace_set_name":   "apm.trace.common",
		"trace_ids":        "trace-1,trace-2",
		"workspace":        "test-ws",
		"regionId":         "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	if !strings.Contains(capturedQuery, "get_trace") {
		t.Errorf("query should contain get_trace, got %q", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "traceId='trace-1'") {
		t.Errorf("query should contain trace ID filter, got %q", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "traceId='trace-2'") {
		t.Errorf("query should contain second trace ID filter, got %q", capturedQuery)
	}
}

// ---------------------------------------------------------------------------
// umodel_search_traces handler tests
// ---------------------------------------------------------------------------

func TestHandleSearchTraces_MissingParams(t *testing.T) {
	tools := DataTools(&mockCMSClient{})
	handler := tools[6].Handler

	result, err := handler(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true for missing params")
	}
}

func TestHandleSearchTraces_Success(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"traceId": "trace-1", "duration_ms": 150.5, "span_count": 5, "error_span_count": 0},
				},
			}, nil
		},
	}

	tools := DataTools(mock)
	handler := tools[6].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"domain":           "apm",
		"entity_set_name":  "apm.service",
		"trace_set_domain": "apm",
		"trace_set_name":   "apm.trace.common",
		"workspace":        "test-ws",
		"regionId":         "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	if !strings.Contains(capturedQuery, "get_trace") {
		t.Errorf("query should contain get_trace, got %q", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "stats") {
		t.Errorf("query should contain stats aggregation, got %q", capturedQuery)
	}
}

func TestHandleSearchTraces_WithMinDuration(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{"data": []interface{}{}}, nil
		},
	}

	tools := DataTools(mock)
	handler := tools[6].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"domain":           "apm",
		"entity_set_name":  "apm.service",
		"trace_set_domain": "apm",
		"trace_set_name":   "apm.trace.common",
		"workspace":        "test-ws",
		"regionId":         "cn-hangzhou",
		"min_duration_ms":  float64(1000),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	// 1000ms = 1000000000 ns
	if !strings.Contains(capturedQuery, "cast(duration as bigint) > 1000000000") {
		t.Errorf("query should contain min_duration filter, got %q", capturedQuery)
	}
}

func TestHandleSearchTraces_WithHasError(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{"data": []interface{}{}}, nil
		},
	}

	tools := DataTools(mock)
	handler := tools[6].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"domain":           "apm",
		"entity_set_name":  "apm.service",
		"trace_set_domain": "apm",
		"trace_set_name":   "apm.trace.common",
		"workspace":        "test-ws",
		"regionId":         "cn-hangzhou",
		"has_error":        true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	if !strings.Contains(capturedQuery, "cast(statusCode as varchar) = '2'") {
		t.Errorf("query should contain has_error filter, got %q", capturedQuery)
	}
}

func TestHandleSearchTraces_WithEntityIDs(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{"data": []interface{}{}}, nil
		},
	}

	tools := DataTools(mock)
	handler := tools[6].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"domain":           "apm",
		"entity_set_name":  "apm.service",
		"trace_set_domain": "apm",
		"trace_set_name":   "apm.trace.common",
		"workspace":        "test-ws",
		"regionId":         "cn-hangzhou",
		"entity_ids":       "svc-1,svc-2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	if !strings.Contains(capturedQuery, "ids=['svc-1','svc-2']") {
		t.Errorf("query should contain entity IDs, got %q", capturedQuery)
	}
}

func TestHandleSearchTraces_SPLError(t *testing.T) {
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, _ string, _, _ int64, _ int) (map[string]interface{}, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	tools := DataTools(mock)
	handler := tools[6].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"domain":           "apm",
		"entity_set_name":  "apm.service",
		"trace_set_domain": "apm",
		"trace_set_name":   "apm.trace.common",
		"workspace":        "test-ws",
		"regionId":         "cn-hangzhou",
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
// umodel_get_profiling handler tests
// ---------------------------------------------------------------------------

func TestHandleGetProfiling_MissingParams(t *testing.T) {
	tools := DataTools(&mockCMSClient{})
	handler := tools[7].Handler

	result, err := handler(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true for missing params")
	}
}

func TestHandleGetProfiling_RequiresEntityIDs(t *testing.T) {
	tools := DataTools(&mockCMSClient{})
	handler := tools[7].Handler

	// All params except entity_ids
	result, err := handler(context.Background(), map[string]interface{}{
		"domain":             "apm",
		"entity_set_name":    "apm.service",
		"profile_set_domain": "default",
		"profile_set_name":   "default.profile.common",
		"workspace":          "test-ws",
		"regionId":           "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true when entity_ids is missing")
	}
}

func TestHandleGetProfiling_Success(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"profile_type": "cpu"},
				},
			}, nil
		},
	}

	tools := DataTools(mock)
	handler := tools[7].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"domain":             "apm",
		"entity_set_name":    "apm.service",
		"profile_set_domain": "default",
		"profile_set_name":   "default.profile.common",
		"workspace":          "test-ws",
		"entity_ids":         "svc-1",
		"regionId":           "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	if !strings.Contains(capturedQuery, "get_profile") {
		t.Errorf("query should contain get_profile, got %q", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "ids=['svc-1']") {
		t.Errorf("query should contain entity IDs, got %q", capturedQuery)
	}
}

func TestHandleGetProfiling_SPLError(t *testing.T) {
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, _ string, _, _ int64, _ int) (map[string]interface{}, error) {
			return nil, fmt.Errorf("timeout")
		},
	}

	tools := DataTools(mock)
	handler := tools[7].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"domain":             "apm",
		"entity_set_name":    "apm.service",
		"profile_set_domain": "default",
		"profile_set_name":   "default.profile.common",
		"workspace":          "test-ws",
		"entity_ids":         "svc-1",
		"regionId":           "cn-hangzhou",
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
// Response structure tests
// ---------------------------------------------------------------------------

func TestDataTools_ResponseContainsTimeRange(t *testing.T) {
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, _ string, _, _ int64, _ int) (map[string]interface{}, error) {
			return map[string]interface{}{"data": []interface{}{}}, nil
		},
	}

	tools := DataTools(mock)
	handler := tools[0].Handler // umodel_get_metrics

	result, err := handler(context.Background(), map[string]interface{}{
		"domain":             "apm",
		"entity_set_name":    "apm.service",
		"metric_domain_name": "apm.metric.service",
		"metric":             "cpu_usage",
		"workspace":          "test-ws",
		"regionId":           "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})

	// Verify response contains time_range
	tr, ok := resp["time_range"].(map[string]interface{})
	if !ok {
		t.Fatal("response should contain time_range map")
	}
	if _, ok := tr["from"]; !ok {
		t.Error("time_range should contain 'from'")
	}
	if _, ok := tr["to"]; !ok {
		t.Error("time_range should contain 'to'")
	}
	if _, ok := tr["from_readable"]; !ok {
		t.Error("time_range should contain 'from_readable'")
	}
	if _, ok := tr["to_readable"]; !ok {
		t.Error("time_range should contain 'to_readable'")
	}
	if _, ok := tr["expression"]; !ok {
		t.Error("time_range should contain 'expression'")
	}
}

// ---------------------------------------------------------------------------
// umodel_get_profiles alias tests
// ---------------------------------------------------------------------------

func TestGetProfilesAlias_Name(t *testing.T) {
	tools := DataTools(&mockCMSClient{})
	// Index 8 is umodel_get_profiles (alias)
	aliasTool := tools[8]
	if aliasTool.Name != "umodel_get_profiles" {
		t.Errorf("alias tool name = %q, want %q", aliasTool.Name, "umodel_get_profiles")
	}
}

func TestGetProfilesAlias_SameInputSchema(t *testing.T) {
	tools := DataTools(&mockCMSClient{})
	// Index 7 is umodel_get_profiling (original)
	// Index 8 is umodel_get_profiles (alias)
	originalTool := tools[7]
	aliasTool := tools[8]

	// Verify both have the same InputSchema structure
	originalSchema := originalTool.InputSchema
	aliasSchema := aliasTool.InputSchema

	// Check that both schemas have the same "type" field
	if originalSchema["type"] != aliasSchema["type"] {
		t.Errorf("InputSchema type mismatch: original=%v, alias=%v", originalSchema["type"], aliasSchema["type"])
	}

	// Check that both schemas have the same "required" fields
	originalRequired, _ := originalSchema["required"].([]string)
	aliasRequired, _ := aliasSchema["required"].([]string)
	if len(originalRequired) != len(aliasRequired) {
		t.Errorf("InputSchema required length mismatch: original=%d, alias=%d", len(originalRequired), len(aliasRequired))
	}

	// Check that both schemas have the same properties
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

func TestGetProfilesAlias_SameHandlerBehavior(t *testing.T) {
	// Test that both tools produce the same result for the same input
	var originalQuery, aliasQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			if originalQuery == "" {
				originalQuery = query
			} else {
				aliasQuery = query
			}
			return map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{"profile_type": "cpu"},
				},
			}, nil
		},
	}

	tools := DataTools(mock)
	originalHandler := tools[7].Handler // umodel_get_profiling
	aliasHandler := tools[8].Handler    // umodel_get_profiles

	params := map[string]interface{}{
		"domain":             "apm",
		"entity_set_name":    "apm.service",
		"profile_set_domain": "default",
		"profile_set_name":   "default.profile.common",
		"workspace":          "test-ws",
		"entity_ids":         "svc-1",
		"regionId":           "cn-hangzhou",
	}

	// Call original handler
	result1, err1 := originalHandler(context.Background(), params)
	if err1 != nil {
		t.Fatalf("original handler error: %v", err1)
	}

	// Call alias handler
	result2, err2 := aliasHandler(context.Background(), params)
	if err2 != nil {
		t.Fatalf("alias handler error: %v", err2)
	}

	// Both should produce the same query
	if originalQuery != aliasQuery {
		t.Errorf("handlers produced different queries:\noriginal: %q\nalias: %q", originalQuery, aliasQuery)
	}

	// Both should have the same error status
	resp1 := result1.(map[string]interface{})
	resp2 := result2.(map[string]interface{})
	if resp1["error"] != resp2["error"] {
		t.Errorf("handlers produced different error status: original=%v, alias=%v", resp1["error"], resp2["error"])
	}
}

func TestGetProfilesAlias_MissingParams(t *testing.T) {
	// Verify alias tool has the same validation behavior as original
	tools := DataTools(&mockCMSClient{})
	aliasHandler := tools[8].Handler // umodel_get_profiles

	result, err := aliasHandler(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true for missing params")
	}
}

