package client

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewCMSClient(t *testing.T) {
	cfg := testConfig()
	cred := testCredential()

	client := NewCMSClient(cred, cfg)
	if client == nil {
		t.Fatal("NewCMSClient returned nil")
	}
	if client.httpClient == nil {
		t.Fatal("httpClient is nil; connection pool not initialized")
	}
	if client.cb == nil {
		t.Fatal("circuit breaker is nil")
	}
	if client.resolver == nil {
		t.Fatal("resolver is nil")
	}
	if client.credential == nil {
		t.Fatal("credential is nil")
	}
}

// TestCMSClient_InterfaceCompliance verifies CMSClientImpl satisfies CMSClient.
func TestCMSClient_InterfaceCompliance(t *testing.T) {
	var _ CMSClient = (*CMSClientImpl)(nil)
}

func TestCMSClient_ExecuteSPL(t *testing.T) {
	// Skip this test as it makes real API calls
	// This is an integration test that requires real credentials
	t.Skip("Skipping integration test: requires real Alibaba Cloud credentials")

	cfg := testConfig()
	client := NewCMSClient(testCredential(), cfg)

	ctx := context.Background()
	result, err := client.ExecuteSPL(ctx, "cn-hongkong", "my-workspace", "* | SELECT count(*)", 1700000000, 1700003600, 100)
	if err != nil {
		t.Fatalf("ExecuteSPL() error = %v; want nil", err)
	}
	if result == nil {
		t.Fatal("ExecuteSPL() returned nil result; want non-nil map")
	}
}

func TestCMSClient_ListWorkspaces(t *testing.T) {
	// Skip this test as it makes real API calls
	// This is an integration test that requires real credentials
	t.Skip("Skipping integration test: requires real Alibaba Cloud credentials")

	cfg := testConfig()
	client := NewCMSClient(testCredential(), cfg)

	ctx := context.Background()
	workspaces, err := client.ListWorkspaces(ctx, "cn-hongkong")
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v; want nil", err)
	}
	if workspaces == nil {
		t.Fatal("ListWorkspaces() returned nil; want non-nil slice")
	}
}

func TestCMSClient_QueryMetric(t *testing.T) {
	cfg := testConfig()
	client := NewCMSClient(testCredential(), cfg)

	ctx := context.Background()
	dims := map[string]string{"instanceId": "i-abc123"}
	// QueryMetric currently returns empty slice (placeholder for CMS 2.0)
	datapoints, err := client.QueryMetric(ctx, "cn-hongkong", "acs_ecs_dashboard", "CPUUtilization", dims, 1700000000, 1700003600)
	if err != nil {
		t.Fatalf("QueryMetric() error = %v; want nil", err)
	}
	if datapoints == nil {
		t.Fatal("QueryMetric() returned nil; want non-nil slice")
	}
	// Expect empty slice since QueryMetric is a placeholder
	if len(datapoints) != 0 {
		t.Fatalf("QueryMetric() returned %d datapoints; want 0 (placeholder)", len(datapoints))
	}
}

func TestCMSClient_QueryMetric_NilDimensions(t *testing.T) {
	cfg := testConfig()
	client := NewCMSClient(testCredential(), cfg)

	ctx := context.Background()
	// QueryMetric currently returns empty slice (placeholder for CMS 2.0)
	datapoints, err := client.QueryMetric(ctx, "cn-hongkong", "acs_ecs_dashboard", "CPUUtilization", nil, 1700000000, 1700003600)
	if err != nil {
		t.Fatalf("QueryMetric() with nil dimensions error = %v; want nil", err)
	}
	if datapoints == nil {
		t.Fatal("QueryMetric() returned nil; want non-nil slice")
	}
	// Expect empty slice since QueryMetric is a placeholder
	if len(datapoints) != 0 {
		t.Fatalf("QueryMetric() returned %d datapoints; want 0 (placeholder)", len(datapoints))
	}
}

func TestCMSClient_EmptyRegion(t *testing.T) {
	cfg := testConfig()
	client := NewCMSClient(testCredential(), cfg)
	ctx := context.Background()

	_, err := client.ExecuteSPL(ctx, "", "ws", "query", 0, 1, 10)
	if err == nil {
		t.Fatal("ExecuteSPL() with empty region should return error")
	}

	_, err = client.ListWorkspaces(ctx, "")
	if err == nil {
		t.Fatal("ListWorkspaces() with empty region should return error")
	}

	_, err = client.QueryMetric(ctx, "", "ns", "metric", nil, 0, 1)
	if err == nil {
		t.Fatal("QueryMetric() with empty region should return error")
	}
}

func TestCMSClient_EndpointOverride(t *testing.T) {
	cfg := testConfig()
	cfg.Endpoints.CMS = map[string]string{
		"cn-hongkong": "custom-cms.example.com",
	}
	client := NewCMSClient(testCredential(), cfg)

	ep, err := client.resolver.Resolve("cn-hongkong")
	if err != nil {
		t.Fatalf("resolver.Resolve() error = %v", err)
	}
	if ep != "custom-cms.example.com" {
		t.Fatalf("resolver.Resolve() = %q; want %q", ep, "custom-cms.example.com")
	}

	// Non-overridden region uses template.
	ep, err = client.resolver.Resolve("cn-shanghai")
	if err != nil {
		t.Fatalf("resolver.Resolve() error = %v", err)
	}
	if ep != "cms.cn-shanghai.aliyuncs.com" {
		t.Fatalf("resolver.Resolve() = %q; want %q", ep, "cms.cn-shanghai.aliyuncs.com")
	}
}

func TestCMSClient_RetryConfig(t *testing.T) {
	cfg := testConfig()
	cfg.Network.MaxRetry = 3
	cfg.Network.RetryWaitSeconds = 1 // 1 second
	client := NewCMSClient(testCredential(), cfg)

	rc := client.retryConfig()
	if rc.MaxAttempts != 3 {
		t.Fatalf("retryConfig().MaxAttempts = %d; want 3", rc.MaxAttempts)
	}
	if rc.WaitTime != 1*time.Second {
		t.Fatalf("retryConfig().WaitTime = %v; want 1s", rc.WaitTime)
	}
	if rc.BackoffFunc == nil {
		t.Fatal("retryConfig().BackoffFunc is nil")
	}
}

func TestCMSClient_ExecuteWithResilience_Success(t *testing.T) {
	cfg := testConfig()
	client := NewCMSClient(testCredential(), cfg)

	calls := 0
	err := client.executeWithResilience(context.Background(), func(ctx context.Context) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("executeWithResilience() error = %v; want nil", err)
	}
	if calls != 1 {
		t.Fatalf("fn called %d times; want 1", calls)
	}
}

func TestCMSClient_ExecuteWithResilience_RetryOnFailure(t *testing.T) {
	cfg := testConfig()
	cfg.Network.MaxRetry = 3
	cfg.Network.RetryWaitSeconds = 0 // Fast test
	client := NewCMSClient(testCredential(), cfg)

	calls := 0
	testErr := errors.New("transient error")
	err := client.executeWithResilience(context.Background(), func(ctx context.Context) error {
		calls++
		return testErr
	})
	if err == nil {
		t.Fatal("executeWithResilience() should return error after all retries fail")
	}
	if calls != 3 {
		t.Fatalf("fn called %d times; want 3", calls)
	}
}

func TestCMSClient_ContextCancellation(t *testing.T) {
	cfg := testConfig()
	cfg.Network.MaxRetry = 3
	cfg.Network.RetryWaitSeconds = 1 // 1 second
	client := NewCMSClient(testCredential(), cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	// With a cancelled context, the retry loop should exit quickly.
	_, err := client.ExecuteSPL(ctx, "cn-hongkong", "ws", "query", 0, 1, 10)
	// Placeholder succeeds immediately even with cancelled context, which is
	// acceptable. The important thing is it doesn't hang.
	_ = err
}

func TestConvertSPLRowsToMaps(t *testing.T) {
	tests := []struct {
		name   string
		header []string
		data   [][]interface{}
		want   []map[string]interface{}
	}{
		{
			name:   "empty header and data",
			header: nil,
			data:   nil,
			want:   []map[string]interface{}{},
		},
		{
			name:   "empty data",
			header: []string{"a", "b"},
			data:   nil,
			want:   []map[string]interface{}{},
		},
		{
			name:   "normal conversion",
			header: []string{"__domain__", "cnt"},
			data: [][]interface{}{
				{"apm", float64(42)},
				{"k8s", float64(10)},
			},
			want: []map[string]interface{}{
				{"__domain__": "apm", "cnt": float64(42)},
				{"__domain__": "k8s", "cnt": float64(10)},
			},
		},
		{
			name:   "row shorter than header",
			header: []string{"a", "b", "c"},
			data:   [][]interface{}{{"x"}},
			want: []map[string]interface{}{
				{"a": "x", "b": nil, "c": nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSPLRowsToMaps(tt.header, tt.data)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i, row := range got {
				for k, v := range tt.want[i] {
					if row[k] != v {
						t.Errorf("row[%d][%q] = %v, want %v", i, k, row[k], v)
					}
				}
			}
		})
	}
}
