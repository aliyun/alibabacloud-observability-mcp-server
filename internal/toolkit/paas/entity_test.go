package paas

import (
	"context"
	"fmt"
	"testing"

	"github.com/alibabacloud-observability-mcp-server-go/internal/client"
)

// mockCMSClient implements client.CMSClient for testing.
type mockCMSClient struct {
	executeSPLFn     func(ctx context.Context, region, workspace, query string, from, to int64, limit int) (map[string]interface{}, error)
	dataAgentQueryFn func(ctx context.Context, region, workspace, query string, fromTime, toTime int64) (*client.DataAgentResult, error)
}

func (m *mockCMSClient) ExecuteSPL(ctx context.Context, region, workspace, query string, from, to int64, limit int) (map[string]interface{}, error) {
	if m.executeSPLFn != nil {
		return m.executeSPLFn(ctx, region, workspace, query, from, to, limit)
	}
	return map[string]interface{}{"data": []interface{}{}}, nil
}

func (m *mockCMSClient) ListWorkspaces(ctx context.Context, region string) ([]map[string]interface{}, error) {
	return nil, nil
}

func (m *mockCMSClient) QueryMetric(ctx context.Context, region, namespace, metricName string, dimensions map[string]string, from, to int64) ([]map[string]interface{}, error) {
	return nil, nil
}

func (m *mockCMSClient) TextToSQL(ctx context.Context, region, project, logStore, text string) (string, error) {
	return "", nil
}

func (m *mockCMSClient) DataAgentQuery(ctx context.Context, region, workspace, query string, fromTime, toTime int64) (*client.DataAgentResult, error) {
	if m.dataAgentQueryFn != nil {
		return m.dataAgentQueryFn(ctx, region, workspace, query, fromTime, toTime)
	}
	return &client.DataAgentResult{}, nil
}

// ---------------------------------------------------------------------------
// EntityTools returns correct number of tools
// ---------------------------------------------------------------------------

func TestEntityTools_Count(t *testing.T) {
	tools := EntityTools(&mockCMSClient{})
	if got := len(tools); got != 3 {
		t.Fatalf("EntityTools() returned %d tools, want 3", got)
	}
}

func TestEntityTools_Names(t *testing.T) {
	tools := EntityTools(&mockCMSClient{})
	expected := []string{
		"umodel_get_entities",
		"umodel_get_neighbor_entities",
		"umodel_search_entities",
	}
	for i, tool := range tools {
		if tool.Name != expected[i] {
			t.Errorf("tool[%d].Name = %q, want %q", i, tool.Name, expected[i])
		}
	}
}

func TestEntityTools_AllHaveUmodelPrefix(t *testing.T) {
	tools := EntityTools(&mockCMSClient{})
	for _, tool := range tools {
		if len(tool.Name) < 7 || tool.Name[:7] != "umodel_" {
			t.Errorf("tool %q does not have umodel_ prefix", tool.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// Helper tests
// ---------------------------------------------------------------------------

func TestBuildEntityIDsParam(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"id1", ", ids=['id1']"},
		{"id1,id2,id3", ", ids=['id1','id2','id3']"},
		{" id1 , id2 ", ", ids=['id1','id2']"},
	}
	for _, tt := range tests {
		got := buildEntityIDsParam(tt.input)
		if got != tt.want {
			t.Errorf("buildEntityIDsParam(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildEntityFilterParam(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"name=payment", `, query=` + "`" + `"name"='payment'` + "`"},
		{"name=payment and status!=inactive", `, query=` + "`" + `"name"='payment' and "status"!='inactive'` + "`"},
	}
	for _, tt := range tests {
		got := buildEntityFilterParam(tt.input)
		if got != tt.want {
			t.Errorf("buildEntityFilterParam(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestConvertToSQLSyntax(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"name=payment", `"name"='payment'`},
		{"status!=inactive", `"status"!='inactive'`},
		{"name=payment and status!=inactive", `"name"='payment' and "status"!='inactive'`},
	}
	for _, tt := range tests {
		got := convertToSQLSyntax(tt.input)
		if got != tt.want {
			t.Errorf("convertToSQLSyntax(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseEntityIDsToSPLParam(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"id1", "['id1']"},
		{"id1,id2", "['id1','id2']"},
		{" id1 , id2 , id3 ", "['id1','id2','id3']"},
	}
	for _, tt := range tests {
		got := parseEntityIDsToSPLParam(tt.input)
		if got != tt.want {
			t.Errorf("parseEntityIDsToSPLParam(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseStringToSPLParam(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "''"},
		{"apm", "'apm'"},
		{" k8s ", "'k8s'"},
	}
	for _, tt := range tests {
		got := parseStringToSPLParam(tt.input)
		if got != tt.want {
			t.Errorf("parseStringToSPLParam(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseDirectionToSPLParam(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "'both'"},
		{"in", "'in'"},
		{"out", "'out'"},
		{"both", "'both'"},
	}
	for _, tt := range tests {
		got := parseDirectionToSPLParam(tt.input)
		if got != tt.want {
			t.Errorf("parseDirectionToSPLParam(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Handler tests with mock CMS client
// ---------------------------------------------------------------------------

func TestHandleGetEntities_MissingParams(t *testing.T) {
	tools := EntityTools(&mockCMSClient{})
	handler := tools[0].Handler // umodel_get_entities

	result, err := handler(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}
	if resp["error"] != true {
		t.Error("expected error=true for missing params")
	}
}

func TestHandleGetEntities_RequiresEntityIDsOrFilter(t *testing.T) {
	tools := EntityTools(&mockCMSClient{})
	handler := tools[0].Handler // umodel_get_entities

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
		t.Error("expected error=true when neither entity_ids nor entity_filter provided")
	}
}

func TestHandleGetEntities_WithEntityIDs(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{"data": []interface{}{}}, nil
		},
	}

	tools := EntityTools(mock)
	handler := tools[0].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"domain":          "apm",
		"entity_set_name": "apm.service",
		"workspace":       "test-ws",
		"regionId":        "cn-hangzhou",
		"entity_ids":      "svc-1,svc-2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	if !contains(capturedQuery, "ids=['svc-1','svc-2']") {
		t.Errorf("query should contain entity IDs, got %q", capturedQuery)
	}
}

func TestHandleGetEntities_WildcardDomainRejected(t *testing.T) {
	tools := EntityTools(&mockCMSClient{})
	handler := tools[0].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"domain":          "*",
		"entity_set_name": "apm.service",
		"workspace":       "test-ws",
		"regionId":        "cn-hangzhou",
		"entity_ids":      "svc-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true for wildcard domain")
	}
}

func TestHandleGetNeighborEntities_InvalidDirection(t *testing.T) {
	tools := EntityTools(&mockCMSClient{})
	handler := tools[1].Handler // umodel_get_neighbor_entities

	result, err := handler(context.Background(), map[string]interface{}{
		"workspace":         "test-ws",
		"src_entity_domain": "apm",
		"src_name":          "apm.service",
		"src_entity_ids":    "svc-1",
		"direction":         "invalid",
		"regionId":          "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true for invalid direction")
	}
}

func TestHandleGetNeighborEntities_Success(t *testing.T) {
	var capturedQuery string
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			capturedQuery = query
			return map[string]interface{}{"data": []interface{}{}}, nil
		},
	}

	tools := EntityTools(mock)
	handler := tools[1].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"workspace":         "test-ws",
		"src_entity_domain": "apm",
		"src_name":          "apm.service",
		"src_entity_ids":    "svc-1,svc-2",
		"direction":         "out",
		"regionId":          "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	if !contains(capturedQuery, "get_neighbor_entities") {
		t.Errorf("query should contain get_neighbor_entities, got %q", capturedQuery)
	}
	if !contains(capturedQuery, "'out'") {
		t.Errorf("query should contain direction 'out', got %q", capturedQuery)
	}
}

func TestHandleSearchEntities_Success(t *testing.T) {
	callCount := 0
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, query string, _, _ int64, _ int) (map[string]interface{}, error) {
			callCount++
			return map[string]interface{}{"data": []interface{}{}}, nil
		},
	}

	tools := EntityTools(mock)
	handler := tools[2].Handler // umodel_search_entities

	result, err := handler(context.Background(), map[string]interface{}{
		"workspace":   "test-ws",
		"search_text": "payment",
		"domain":      "apm",
		"regionId":    "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != false {
		t.Errorf("expected error=false, got message: %v", resp["message"])
	}
	// Should make 2 SPL calls: stats + detail
	if callCount != 2 {
		t.Errorf("expected 2 SPL calls (stats + detail), got %d", callCount)
	}
	// Data should contain statistics and detail
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("data should be a map with statistics and detail")
	}
	if _, ok := data["statistics"]; !ok {
		t.Error("data should contain 'statistics' key")
	}
	if _, ok := data["detail"]; !ok {
		t.Error("data should contain 'detail' key")
	}
}

func TestHandleSearchEntities_SPLError(t *testing.T) {
	mock := &mockCMSClient{
		executeSPLFn: func(_ context.Context, _, _, _ string, _, _ int64, _ int) (map[string]interface{}, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	tools := EntityTools(mock)
	handler := tools[2].Handler

	result, err := handler(context.Background(), map[string]interface{}{
		"workspace":   "test-ws",
		"search_text": "payment",
		"regionId":    "cn-hangzhou",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp := result.(map[string]interface{})
	if resp["error"] != true {
		t.Error("expected error=true when SPL call fails")
	}
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
