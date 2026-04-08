package client

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	sls "github.com/alibabacloud-go/sls-20201230/v6/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"

	"github.com/alibabacloud-observability-mcp-server-go/internal/config"
	"github.com/alibabacloud-observability-mcp-server-go/internal/stability"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/endpoint"
)

// SLSClient is the interface for interacting with Alibaba Cloud Simple Log Service.
type SLSClient interface {
	// Query executes a log query against the specified logstore.
	// limit controls max rows returned (1-100, 0 means use server default); offset is for pagination; reverse returns logs in reverse chronological order.
	Query(ctx context.Context, region, project, logstore, query string, from, to int64, limit, offset int, reverse bool) ([]map[string]interface{}, error)
	// GetContextLogs retrieves context logs around an anchor log identified by pack_id and pack_meta.
	GetContextLogs(ctx context.Context, region, project, logstore, packID, packMeta string, backLines, forwardLines int) (map[string]interface{}, error)
	// ListProjects returns all SLS project names in the given region.
	ListProjects(ctx context.Context, region string) ([]string, error)
	// ListProjectsWithFilter returns SLS project names with optional name filter and limit.
	ListProjectsWithFilter(ctx context.Context, region, projectName string, limit int) ([]map[string]interface{}, error)
	// ListLogStores returns all logstore names within a project.
	ListLogStores(ctx context.Context, region, project string) ([]string, error)
	// ListLogStoresWithFilter returns logstore names with optional filter, limit, and type.
	ListLogStoresWithFilter(ctx context.Context, region, project, logStoreName string, limit int, isMetricStore bool) ([]string, error)
	// ListMetricStores returns all metric store names within a project.
	ListMetricStores(ctx context.Context, region, project string) ([]string, error)
	// TextToSQL converts a natural language question into an SQL query for the given logstore.
	TextToSQL(ctx context.Context, region, project, logstore, question string) (string, error)
}

// SLSClientImpl implements SLSClient with connection pooling, retry, and circuit breaker.
type SLSClientImpl struct {
	credential CredentialProvider
	resolver   *endpoint.Resolver
	config     *config.Config
	cb         *stability.CircuitBreaker
	cmsClient  CMSClient // Used for TextToSQL via CMS Chat API
}

// NewSLSClient creates a new SLSClientImpl with retry and circuit breaker support.
func NewSLSClient(cred CredentialProvider, cfg *config.Config) *SLSClientImpl {
	retryWait := time.Duration(cfg.Network.RetryWaitSeconds) * time.Second
	cb := stability.NewCircuitBreaker("sls", 5, retryWait*5)

	return &SLSClientImpl{
		credential: cred,
		resolver:   endpoint.NewSLSResolver(cfg.Endpoints.SLS),
		config:     cfg,
		cb:         cb,
		cmsClient:  nil, // Will be set via SetCMSClient
	}
}

// SetCMSClient sets the CMS client for TextToSQL delegation.
func (c *SLSClientImpl) SetCMSClient(cmsClient CMSClient) {
	c.cmsClient = cmsClient
}

// retryConfig builds a RetryConfig from the current configuration.
func (c *SLSClientImpl) retryConfig() stability.RetryConfig {
	retryWait := time.Duration(c.config.Network.RetryWaitSeconds) * time.Second
	return stability.RetryConfig{
		MaxAttempts: c.config.Network.MaxRetry,
		WaitTime:    retryWait,
		BackoffFunc: stability.DefaultBackoff(retryWait),
	}
}

// executeWithResilience wraps fn with retry and circuit breaker.
func (c *SLSClientImpl) executeWithResilience(ctx context.Context, fn func(ctx context.Context) error) error {
	return stability.Retry(ctx, c.retryConfig(), func(ctx context.Context) error {
		return c.cb.Execute(ctx, fn)
	})
}

// createClient creates an SLS SDK client for the given region.
func (c *SLSClientImpl) createClient(region string) (*sls.Client, error) {
	ep, err := c.resolver.Resolve(region)
	if err != nil {
		return nil, fmt.Errorf("sls: resolve endpoint: %w", err)
	}

	accessKeyID, err := c.credential.GetAccessKeyID()
	if err != nil {
		return nil, fmt.Errorf("sls: get access key id: %w", err)
	}

	accessKeySecret, err := c.credential.GetAccessKeySecret()
	if err != nil {
		return nil, fmt.Errorf("sls: get access key secret: %w", err)
	}

	cfg := &openapi.Config{
		AccessKeyId:     tea.String(accessKeyID),
		AccessKeySecret: tea.String(accessKeySecret),
		Endpoint:        tea.String(ep),
	}

	// Add security token if available (for STS)
	token, _ := c.credential.GetSecurityToken()
	if token != "" {
		cfg.SecurityToken = tea.String(token)
	}

	client, err := sls.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("sls: create client: %w", err)
	}

	return client, nil
}

// runtimeOptions returns the runtime options for SDK calls.
func (c *SLSClientImpl) runtimeOptions() *util.RuntimeOptions {
	return &util.RuntimeOptions{
		ConnectTimeout: tea.Int(c.config.Network.ConnectTimeoutMs),
		ReadTimeout:    tea.Int(c.config.Network.ReadTimeoutMs),
	}
}

// Query executes a log query against the specified logstore.
// limit controls the max number of rows returned (0 or negative means use the server default of 100).
// offset is used for pagination; reverse returns logs in reverse chronological order.
func (c *SLSClientImpl) Query(ctx context.Context, region, project, logstore, query string, from, to int64, limit, offset int, reverse bool) ([]map[string]interface{}, error) {
	client, err := c.createClient(region)
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}

	err = c.executeWithResilience(ctx, func(ctx context.Context) error {
		slog.DebugContext(ctx, "sls: query",
			"region", region,
			"project", project,
			"logstore", logstore,
			"from", from,
			"to", to,
			"limit", limit,
			"offset", offset,
			"reverse", reverse,
		)

		request := &sls.GetLogsRequest{
			From:    tea.Int32(int32(from)),
			To:      tea.Int32(int32(to)),
			Query:   tea.String(query),
			Offset:  tea.Int64(int64(offset)),
			Reverse: tea.Bool(reverse),
		}
		// Only set Line if a positive limit is specified; otherwise the server uses its default (100).
		if limit > 0 {
			request.Line = tea.Int64(int64(limit))
		}

		resp, err := client.GetLogsWithOptions(tea.String(project), tea.String(logstore), request, map[string]*string{}, c.runtimeOptions())
		if err != nil {
			return fmt.Errorf("sls api error: %w", err)
		}

		if resp.Body == nil {
			results = []map[string]interface{}{}
			return nil
		}

		results = make([]map[string]interface{}, 0, len(resp.Body))
		for _, log := range resp.Body {
			logMap := make(map[string]interface{})
			for k, v := range log {
				if strPtr, ok := v.(*string); ok && strPtr != nil {
					logMap[k] = tea.StringValue(strPtr)
				} else if v != nil {
					logMap[k] = v
				}
			}
			results = append(results, logMap)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("sls: query %s/%s: %w", project, logstore, err)
	}
	return results, nil
}

// GetContextLogs retrieves context logs around an anchor log identified by pack_id and pack_meta.
func (c *SLSClientImpl) GetContextLogs(ctx context.Context, region, project, logstore, packID, packMeta string, backLines, forwardLines int) (map[string]interface{}, error) {
	client, err := c.createClient(region)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}

	err = c.executeWithResilience(ctx, func(ctx context.Context) error {
		slog.DebugContext(ctx, "sls: get context logs",
			"region", region,
			"project", project,
			"logstore", logstore,
			"pack_id", packID,
			"back_lines", backLines,
			"forward_lines", forwardLines,
		)

		request := &sls.GetContextLogsRequest{
			PackId:       tea.String(packID),
			PackMeta:     tea.String(packMeta),
			BackLines:    tea.Int64(int64(backLines)),
			ForwardLines: tea.Int64(int64(forwardLines)),
		}

		resp, err := client.GetContextLogsWithOptions(tea.String(project), tea.String(logstore), request, map[string]*string{}, c.runtimeOptions())
		if err != nil {
			return fmt.Errorf("sls api error: %w", err)
		}

		if resp.Body == nil {
			result = map[string]interface{}{
				"logs":          []map[string]interface{}{},
				"total_lines":   int64(0),
				"back_lines":    int64(0),
				"forward_lines": int64(0),
				"progress":      "",
			}
			return nil
		}

		body := resp.Body
		logs := make([]map[string]interface{}, 0)
		if body.Logs != nil {
			for _, log := range body.Logs {
				logMap := make(map[string]interface{})
				for k, v := range log {
					logMap[k] = v
				}
				logs = append(logs, logMap)
			}
		}

		result = map[string]interface{}{
			"logs":          logs,
			"total_lines":   tea.Int64Value(body.TotalLines),
			"back_lines":    tea.Int64Value(body.BackLines),
			"forward_lines": tea.Int64Value(body.ForwardLines),
			"progress":      tea.StringValue(body.Progress),
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("sls: get context logs %s/%s: %w", project, logstore, err)
	}
	return result, nil
}

// ListProjects returns all SLS project names in the given region.
func (c *SLSClientImpl) ListProjects(ctx context.Context, region string) ([]string, error) {
	projects, err := c.ListProjectsWithFilter(ctx, region, "", 100)
	if err != nil {
		return nil, err
	}
	// Extract just the project names
	names := make([]string, 0, len(projects))
	for _, p := range projects {
		if name, ok := p["project_name"].(string); ok {
			names = append(names, name)
		}
	}
	return names, nil
}

// ListProjectsWithFilter returns SLS project names with optional name filter and limit.
func (c *SLSClientImpl) ListProjectsWithFilter(ctx context.Context, region, projectName string, limit int) ([]map[string]interface{}, error) {
	client, err := c.createClient(region)
	if err != nil {
		return nil, err
	}

	var projects []map[string]interface{}

	err = c.executeWithResilience(ctx, func(ctx context.Context) error {
		slog.DebugContext(ctx, "sls: list projects with filter",
			"region", region,
			"projectName", projectName,
			"limit", limit,
		)

		request := &sls.ListProjectRequest{
			Size: tea.Int32(int32(limit)),
		}

		// Add project name filter if provided (fuzzy search)
		if projectName != "" {
			request.ProjectName = tea.String(projectName)
		}

		resp, err := client.ListProjectWithOptions(request, map[string]*string{}, c.runtimeOptions())
		if err != nil {
			return fmt.Errorf("sls api error: %w", err)
		}

		if resp.Body == nil || resp.Body.Projects == nil {
			projects = []map[string]interface{}{}
			return nil
		}

		projects = make([]map[string]interface{}, 0, len(resp.Body.Projects))
		for _, p := range resp.Body.Projects {
			if p.ProjectName != nil {
				projectInfo := map[string]interface{}{
					"project_name": tea.StringValue(p.ProjectName),
					"description":  tea.StringValue(p.Description),
					"region_id":    tea.StringValue(p.Region),
				}
				projects = append(projects, projectInfo)
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("sls: list projects in %s: %w", region, err)
	}
	return projects, nil
}

// ListLogStores returns all logstore names within a project.
func (c *SLSClientImpl) ListLogStores(ctx context.Context, region, project string) ([]string, error) {
	return c.ListLogStoresWithFilter(ctx, region, project, "", 500, false)
}

// ListLogStoresWithFilter returns logstore names with optional filter, limit, and type.
func (c *SLSClientImpl) ListLogStoresWithFilter(ctx context.Context, region, project, logStoreName string, limit int, isMetricStore bool) ([]string, error) {
	client, err := c.createClient(region)
	if err != nil {
		return nil, err
	}

	var logstores []string

	err = c.executeWithResilience(ctx, func(ctx context.Context) error {
		slog.DebugContext(ctx, "sls: list logstores with filter",
			"region", region,
			"project", project,
			"logStoreName", logStoreName,
			"limit", limit,
			"isMetricStore", isMetricStore,
		)

		request := &sls.ListLogStoresRequest{
			Size: tea.Int32(int32(limit)),
		}

		// Add logstore name filter if provided
		if logStoreName != "" {
			request.LogstoreName = tea.String(logStoreName)
		}

		// Add telemetry type filter for metric stores
		if isMetricStore {
			request.TelemetryType = tea.String("Metrics")
		}

		resp, err := client.ListLogStoresWithOptions(tea.String(project), request, map[string]*string{}, c.runtimeOptions())
		if err != nil {
			return fmt.Errorf("sls api error: %w", err)
		}

		if resp.Body == nil || resp.Body.Logstores == nil {
			logstores = []string{}
			return nil
		}

		logstores = make([]string, 0, len(resp.Body.Logstores))
		for _, ls := range resp.Body.Logstores {
			if ls != nil {
				logstores = append(logstores, tea.StringValue(ls))
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("sls: list logstores in %s/%s: %w", region, project, err)
	}
	return logstores, nil
}

// ListMetricStores returns all metric store names within a project.
func (c *SLSClientImpl) ListMetricStores(ctx context.Context, region, project string) ([]string, error) {
	client, err := c.createClient(region)
	if err != nil {
		return nil, err
	}

	var metricStores []string

	err = c.executeWithResilience(ctx, func(ctx context.Context) error {
		slog.DebugContext(ctx, "sls: list metric stores",
			"region", region,
			"project", project,
		)

		request := &sls.ListMetricStoresRequest{
			Size: tea.Int32(500),
		}

		resp, err := client.ListMetricStoresWithOptions(tea.String(project), request, map[string]*string{}, c.runtimeOptions())
		if err != nil {
			return fmt.Errorf("sls api error: %w", err)
		}

		if resp.Body == nil || resp.Body.Metricstores == nil {
			metricStores = []string{}
			return nil
		}

		metricStores = make([]string, 0, len(resp.Body.Metricstores))
		for _, ms := range resp.Body.Metricstores {
			if ms != nil {
				metricStores = append(metricStores, tea.StringValue(ms))
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("sls: list metric stores in %s/%s: %w", region, project, err)
	}
	return metricStores, nil
}

// TextToSQL converts a natural language question into an SQL query.
// This delegates to the CMS Chat API (CreateThread + CreateChatWithSSE) which is the
// same approach used by the Python implementation.
func (c *SLSClientImpl) TextToSQL(ctx context.Context, region, project, logstore, question string) (string, error) {
	if c.cmsClient == nil {
		return "", fmt.Errorf("sls: text to sql: CMS client not configured")
	}

	slog.DebugContext(ctx, "sls: text to sql (delegating to CMS)",
		"region", region,
		"project", project,
		"logstore", logstore,
		"question", question,
	)

	return c.cmsClient.TextToSQL(ctx, region, project, logstore, question)
}
