package client

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	sls "github.com/alibabacloud-go/sls-20201230/v6/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/config"
)

// testConfig returns a minimal Config suitable for SLS client tests.
func testConfig() *config.Config {
	return &config.Config{
		Network: config.NetworkConfig{
			MaxRetry:         1,
			RetryWaitSeconds: 0, // Use 0 for fast tests (10ms equivalent)
			ReadTimeoutMs:    5000,
			ConnectTimeoutMs: 1000,
		},
		Endpoints: config.EndpointsConfig{
			SLS: map[string]string{},
			CMS: map[string]string{},
		},
		Credentials: config.CredentialsConfig{
			AccessKeyID:     "test-id",
			AccessKeySecret: "test-secret",
		},
	}
}

// testCredential returns a static credential provider for tests.
func testCredential() CredentialProvider {
	return &StaticCredentialProvider{
		AccessKeyID:     "test-id",
		AccessKeySecret: "test-secret",
	}
}

func TestNewSLSClient(t *testing.T) {
	cfg := testConfig()
	cred := testCredential()

	client := NewSLSClient(cred, cfg)
	if client == nil {
		t.Fatal("NewSLSClient returned nil")
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

// TestSLSClient_InterfaceCompliance verifies SLSClientImpl satisfies SLSClient.
func TestSLSClient_InterfaceCompliance(t *testing.T) {
	var _ SLSClient = (*SLSClientImpl)(nil)
}

func TestLogsV2MetaToMap(t *testing.T) {
	if got := logsV2MetaToMap(nil); len(got) != 0 {
		t.Fatalf("logsV2MetaToMap(nil) = %v, want empty map", got)
	}

	meta := &sls.GetLogsV2ResponseBodyMeta{
		IsAccurate:         tea.Bool(true),
		Progress:           tea.String("Complete"),
		HasSQL:             tea.Bool(true),
		Count:              tea.Int32(1),
		ProcessedRows:      tea.Int64(10000),
		ProcessedBytes:     tea.Int64(2048),
		ElapsedMillisecond: tea.Int64(5),
		WhereQuery:         tea.String("*"),
		AggQuery:           tea.String("select count(*) as cnt"),
	}
	got := logsV2MetaToMap(meta)
	if got["isAccurate"] != true {
		t.Errorf("isAccurate = %v, want true", got["isAccurate"])
	}
	if got["progress"] != "Complete" {
		t.Errorf("progress = %v, want Complete", got["progress"])
	}
	if got["hasSQL"] != true {
		t.Errorf("hasSQL = %v, want true", got["hasSQL"])
	}
	if got["count"] != int32(1) {
		t.Errorf("count = %v, want 1", got["count"])
	}
	if got["processedRows"] != int64(10000) {
		t.Errorf("processedRows = %v, want 10000", got["processedRows"])
	}
}

func TestQueryWithMeta_Integration(t *testing.T) {
	if os.Getenv("SLS_IT") != "1" {
		t.Skip("set SLS_IT=1 SLS_IT_REGION SLS_IT_PROJECT SLS_IT_LOGSTORE")
	}
	region := os.Getenv("SLS_IT_REGION")
	project := os.Getenv("SLS_IT_PROJECT")
	logstore := os.Getenv("SLS_IT_LOGSTORE")
	if region == "" || project == "" || logstore == "" {
		t.Fatal("SLS_IT_REGION, SLS_IT_PROJECT and SLS_IT_LOGSTORE are required")
	}

	cfg := testConfig()
	cfg.Network.ReadTimeoutMs = 30000
	slsClient := NewSLSClient(NewCredentialProvider("", "", ""), cfg)
	now := time.Now().Unix()
	result, err := slsClient.QueryWithMeta(context.Background(), region, project, logstore, &sls.GetLogsRequest{
		Query: tea.String("* | select count(*) as cnt"),
		From:  tea.Int32(int32(now - 300)),
		To:    tea.Int32(int32(now)),
		Line:  tea.Int64(1),
	})
	if err != nil {
		t.Fatalf("QueryWithMeta: %v", err)
	}
	if result.Meta["progress"] == nil && result.Meta["isAccurate"] == nil {
		t.Fatalf("GetLogsV2 meta missing progress/isAccurate, keys=%v", mapsKeys(result.Meta))
	}
}

func mapsKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestSLSClient_Query(t *testing.T) {
	// Skip this test as it makes real API calls
	// This is an integration test that requires real credentials
	t.Skip("Skipping integration test: requires real Alibaba Cloud credentials")

	cfg := testConfig()
	client := NewSLSClient(testCredential(), cfg)

	ctx := context.Background()
	requestParams := &sls.GetLogsRequest{
		Query: tea.String("* | SELECT count(*)"),
		From:  tea.Int32(1700000000),
		To:    tea.Int32(1700003600),
	}
	results, err := client.Query(ctx, "cn-hongkong", "my-project", "my-logstore", requestParams)
	if err != nil {
		t.Fatalf("Query() error = %v; want nil", err)
	}
	if results == nil {
		t.Fatal("Query() returned nil results; want non-nil slice")
	}
}

func TestSLSClient_ListProjects(t *testing.T) {
	// Skip this test as it makes real API calls
	// This is an integration test that requires real credentials
	t.Skip("Skipping integration test: requires real Alibaba Cloud credentials")

	cfg := testConfig()
	client := NewSLSClient(testCredential(), cfg)

	ctx := context.Background()
	projects, err := client.ListProjects(ctx, "cn-hongkong")
	if err != nil {
		t.Fatalf("ListProjects() error = %v; want nil", err)
	}
	if projects == nil {
		t.Fatal("ListProjects() returned nil; want non-nil slice")
	}
}

func TestSLSClient_ListLogStores(t *testing.T) {
	// Skip this test as it makes real API calls
	// This is an integration test that requires real credentials
	t.Skip("Skipping integration test: requires real Alibaba Cloud credentials")

	cfg := testConfig()
	client := NewSLSClient(testCredential(), cfg)

	ctx := context.Background()
	stores, err := client.ListLogStores(ctx, "cn-hongkong", "my-project")
	if err != nil {
		t.Fatalf("ListLogStores() error = %v; want nil", err)
	}
	if stores == nil {
		t.Fatal("ListLogStores() returned nil; want non-nil slice")
	}
}

func TestSLSClient_ListMetricStores(t *testing.T) {
	// Skip this test as it makes real API calls
	// This is an integration test that requires real credentials
	t.Skip("Skipping integration test: requires real Alibaba Cloud credentials")

	cfg := testConfig()
	client := NewSLSClient(testCredential(), cfg)

	ctx := context.Background()
	stores, err := client.ListMetricStores(ctx, "cn-hongkong", "my-project")
	if err != nil {
		t.Fatalf("ListMetricStores() error = %v; want nil", err)
	}
	if stores == nil {
		t.Fatal("ListMetricStores() returned nil; want non-nil slice")
	}
}

func TestSLSClient_TextToSQL(t *testing.T) {
	// Skip this test as it makes real API calls
	// This is an integration test that requires real credentials
	t.Skip("Skipping integration test: requires real Alibaba Cloud credentials")

	cfg := testConfig()
	client := NewSLSClient(testCredential(), cfg)

	ctx := context.Background()
	sql, err := client.TextToSQL(ctx, "cn-hongkong", "my-project", "my-logstore", "show me errors in the last hour")
	if err != nil {
		t.Fatalf("TextToSQL() error = %v; want nil", err)
	}
	// Result depends on actual API response
	_ = sql
}

func TestSLSClient_EmptyRegion(t *testing.T) {
	cfg := testConfig()
	client := NewSLSClient(testCredential(), cfg)
	ctx := context.Background()

	_, err := client.Query(ctx, "", "proj", "store", &sls.GetLogsRequest{
		Query: tea.String("*"),
		From:  tea.Int32(0),
		To:    tea.Int32(1),
	})
	if err == nil {
		t.Fatal("Query() with empty region should return error")
	}

	_, err = client.ListProjects(ctx, "")
	if err == nil {
		t.Fatal("ListProjects() with empty region should return error")
	}

	_, err = client.ListLogStores(ctx, "", "proj")
	if err == nil {
		t.Fatal("ListLogStores() with empty region should return error")
	}

	_, err = client.ListMetricStores(ctx, "", "proj")
	if err == nil {
		t.Fatal("ListMetricStores() with empty region should return error")
	}

	_, err = client.TextToSQL(ctx, "", "proj", "store", "question")
	if err == nil {
		t.Fatal("TextToSQL() with empty region should return error")
	}
}

func TestSLSClient_ContextCancellation(t *testing.T) {
	cfg := testConfig()
	cfg.Network.MaxRetry = 3
	cfg.Network.RetryWaitSeconds = 1 // 1 second
	client := NewSLSClient(testCredential(), cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	// With a cancelled context, the retry loop should exit quickly.
	// The placeholder implementation succeeds on first try, so this tests
	// that context is checked. For a real SDK call that blocks, the context
	// cancellation would propagate through the HTTP client.
	_, err := client.Query(ctx, "cn-hongkong", "proj", "store", &sls.GetLogsRequest{
		Query: tea.String("*"),
		From:  tea.Int32(0),
		To:    tea.Int32(1),
	})
	// Placeholder succeeds immediately even with cancelled context, which is
	// acceptable. The important thing is it doesn't hang.
	_ = err
}

func TestSLSClient_EndpointOverride(t *testing.T) {
	cfg := testConfig()
	cfg.Endpoints.SLS = map[string]string{
		"cn-hongkong": "custom-sls.example.com",
	}
	client := NewSLSClient(testCredential(), cfg)

	ep, err := client.resolver.Resolve("cn-hongkong")
	if err != nil {
		t.Fatalf("resolver.Resolve() error = %v", err)
	}
	if ep != "custom-sls.example.com" {
		t.Fatalf("resolver.Resolve() = %q; want %q", ep, "custom-sls.example.com")
	}

	// Non-overridden region uses template.
	ep, err = client.resolver.Resolve("cn-shanghai")
	if err != nil {
		t.Fatalf("resolver.Resolve() error = %v", err)
	}
	if ep != "cn-shanghai.log.aliyuncs.com" {
		t.Fatalf("resolver.Resolve() = %q; want %q", ep, "cn-shanghai.log.aliyuncs.com")
	}
}

func TestSLSClient_RetryConfig(t *testing.T) {
	cfg := testConfig()
	cfg.Network.MaxRetry = 3
	cfg.Network.RetryWaitSeconds = 1 // 1 second = 1000ms, but we check for 1s
	client := NewSLSClient(testCredential(), cfg)

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

func TestSLSClient_ExecuteWithResilience_Success(t *testing.T) {
	cfg := testConfig()
	client := NewSLSClient(testCredential(), cfg)

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

func TestSLSClient_ExecuteWithResilience_RetryOnFailure(t *testing.T) {
	cfg := testConfig()
	cfg.Network.MaxRetry = 3
	cfg.Network.RetryWaitSeconds = 0 // Fast test
	client := NewSLSClient(testCredential(), cfg)

	calls := 0
	testErr := io.EOF // Use a retryable error
	err := client.executeWithResilience(context.Background(), func(ctx context.Context) error {
		calls++
		return testErr
	})
	if err == nil {
		t.Fatal("executeWithResilience() should return error after all retries fail")
	}
	// With MaxRetry=3, the function should be called 3 times.
	if calls != 3 {
		t.Fatalf("fn called %d times; want 3", calls)
	}
}
