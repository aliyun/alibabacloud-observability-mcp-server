package client

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	cms "github.com/alibabacloud-go/cms-20240330/v6/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/dara"

	"github.com/alibabacloud-observability-mcp-server-go/internal/config"
	"github.com/alibabacloud-observability-mcp-server-go/internal/stability"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/endpoint"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/timeparse"
)

// CMSClient is the interface for interacting with Alibaba Cloud Monitor Service.
type CMSClient interface {
	// ExecuteSPL executes an SPL query against the specified workspace.
	ExecuteSPL(ctx context.Context, region, workspace, query string, from, to int64, limit int) (map[string]interface{}, error)
	// ListWorkspaces returns all CMS workspaces in the given region.
	ListWorkspaces(ctx context.Context, region string) ([]map[string]interface{}, error)
	// QueryMetric queries metric data from CMS.
	QueryMetric(ctx context.Context, region, namespace, metricName string, dimensions map[string]string, from, to int64) ([]map[string]interface{}, error)
	// TextToSQL converts natural language to SQL query using CMS Chat API.
	TextToSQL(ctx context.Context, region, project, logstore, text string) (string, error)
	// CMSClient interface 中添加：
	// DataAgentQuery performs a natural language data query using CMS CreateThread + CreateChatWithSSE API.
	DataAgentQuery(ctx context.Context, region, workspace, query string, fromTime, toTime int64) (*DataAgentResult, error)
}

// DataAgentResult holds the result of a data-agent natural language query.
// DataAgentResult holds the result of a data-agent natural language query.
type DataAgentResult struct {
	QueryResults []interface{}            // collected data items (entity_list, metric_set_query, etc.)
	ToolResults  []map[string]interface{} // tool call results
	GeneratedSQL string                   // extracted SQL if any
	Message      string                   // AI-generated explanation text
	TraceID      string                   // trace ID for debugging
}

// CMSClientImpl implements CMSClient using raw HTTP API calls.
// The Go SDK (cms-20240330 v1.0.0) doesn't include ListWorkspaces and GetEntityStoreData APIs,
// so we implement them using direct HTTP calls with Alibaba Cloud Signature V3.
type CMSClientImpl struct {
	credential CredentialProvider
	resolver   *endpoint.Resolver
	config     *config.Config
	cb         *stability.CircuitBreaker
	httpClient *http.Client
}

// NewCMSClient creates a new CMSClientImpl with retry and circuit breaker support.
func NewCMSClient(cred CredentialProvider, cfg *config.Config) *CMSClientImpl {
	retryWait := time.Duration(cfg.Network.RetryWaitSeconds) * time.Second
	cb := stability.NewCircuitBreaker("cms", 5, retryWait*5)

	return &CMSClientImpl{
		credential: cred,
		resolver:   endpoint.NewCMSResolver(cfg.Endpoints.CMS),
		config:     cfg,
		cb:         cb,
		httpClient: &http.Client{
			Timeout: cfg.GetReadTimeout(),
		},
	}
}

// retryConfig builds a RetryConfig from the current configuration.
func (c *CMSClientImpl) retryConfig() stability.RetryConfig {
	retryWait := time.Duration(c.config.Network.RetryWaitSeconds) * time.Second
	return stability.RetryConfig{
		MaxAttempts: c.config.Network.MaxRetry,
		WaitTime:    retryWait,
		BackoffFunc: stability.DefaultBackoff(retryWait),
	}
}

// executeWithResilience wraps fn with retry and circuit breaker.
func (c *CMSClientImpl) executeWithResilience(ctx context.Context, fn func(ctx context.Context) error) error {
	return stability.Retry(ctx, c.retryConfig(), func(ctx context.Context) error {
		return c.cb.Execute(ctx, fn)
	})
}

// resolveCredential returns the per-request credential from ctx if available,
// otherwise falls back to the client's default credential provider.
func (c *CMSClientImpl) resolveCredential(ctx context.Context) CredentialProvider {
	if cred := CredentialFromContext(ctx); cred != nil {
		return cred
	}
	return c.credential
}

// signRequest signs an HTTP request using Alibaba Cloud Signature Version 3.
// Reference: https://help.aliyun.com/document_detail/315526.html
func (c *CMSClientImpl) signRequest(ctx context.Context, req *http.Request, body []byte, action string) error {
	cred := c.resolveCredential(ctx)

	accessKeyID, err := cred.GetAccessKeyID()
	if err != nil {
		return fmt.Errorf("get access key id: %w", err)
	}

	accessKeySecret, err := cred.GetAccessKeySecret()
	if err != nil {
		return fmt.Errorf("get access key secret: %w", err)
	}

	// Set required headers
	now := time.Now().UTC()
	req.Header.Set("x-acs-date", now.Format("2006-01-02T15:04:05Z"))
	req.Header.Set("x-acs-version", "2024-03-30")
	req.Header.Set("x-acs-action", action)
	req.Header.Set("x-acs-signature-nonce", fmt.Sprintf("%d", time.Now().UnixNano()))

	// Add security token if available (for STS)
	token, _ := cred.GetSecurityToken()
	if token != "" {
		req.Header.Set("x-acs-security-token", token)
	}

	// Calculate content hash
	hash := sha256.Sum256(body)
	contentHash := hex.EncodeToString(hash[:])
	req.Header.Set("x-acs-content-sha256", contentHash)

	// Build canonical request
	canonicalRequest := c.buildCanonicalRequest(req, contentHash)

	// Build string to sign
	stringToSign := "ACS3-HMAC-SHA256\n" + hashSHA256(canonicalRequest)

	// Calculate signature
	signature := hmacSHA256([]byte(accessKeySecret), []byte(stringToSign))
	signatureHex := hex.EncodeToString(signature)

	// Build authorization header
	signedHeaders := c.getSignedHeaders(req)
	auth := fmt.Sprintf("ACS3-HMAC-SHA256 Credential=%s,SignedHeaders=%s,Signature=%s",
		accessKeyID, signedHeaders, signatureHex)
	req.Header.Set("Authorization", auth)

	return nil
}

// buildCanonicalRequest builds the canonical request string for signing.
func (c *CMSClientImpl) buildCanonicalRequest(req *http.Request, contentHash string) string {
	canonicalURI := req.URL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	canonicalQueryString := c.buildCanonicalQueryString(req.URL.Query())
	canonicalHeaders, signedHeaders := c.buildCanonicalHeaders(req)

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		contentHash)
}

// buildCanonicalQueryString builds the canonical query string.
func (c *CMSClientImpl) buildCanonicalQueryString(query url.Values) string {
	if len(query) == 0 {
		return ""
	}

	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		values := query[k]
		sort.Strings(values)
		for _, v := range values {
			pairs = append(pairs, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}
	return strings.Join(pairs, "&")
}

// buildCanonicalHeaders builds canonical headers and returns signed header names.
func (c *CMSClientImpl) buildCanonicalHeaders(req *http.Request) (string, string) {
	headersToSign := make(map[string]string)
	headersToSign["host"] = req.Host

	for k, v := range req.Header {
		lowerKey := strings.ToLower(k)
		if strings.HasPrefix(lowerKey, "x-acs-") || lowerKey == "content-type" {
			headersToSign[lowerKey] = strings.TrimSpace(v[0])
		}
	}

	keys := make([]string, 0, len(headersToSign))
	for k := range headersToSign {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var canonicalHeaders strings.Builder
	for _, k := range keys {
		canonicalHeaders.WriteString(k)
		canonicalHeaders.WriteString(":")
		canonicalHeaders.WriteString(headersToSign[k])
		canonicalHeaders.WriteString("\n")
	}

	signedHeaders := strings.Join(keys, ";")
	return canonicalHeaders.String(), signedHeaders
}

// getSignedHeaders returns the signed headers string.
func (c *CMSClientImpl) getSignedHeaders(req *http.Request) string {
	headers := []string{"host"}
	for k := range req.Header {
		lowerKey := strings.ToLower(k)
		if strings.HasPrefix(lowerKey, "x-acs-") || lowerKey == "content-type" {
			headers = append(headers, lowerKey)
		}
	}
	sort.Strings(headers)
	return strings.Join(headers, ";")
}

func hashSHA256(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// ListWorkspaces returns all CMS workspaces in the given region.
// API: GET /workspace
func (c *CMSClientImpl) ListWorkspaces(ctx context.Context, region string) ([]map[string]interface{}, error) {
	ep, err := c.resolver.Resolve(region)
	if err != nil {
		return nil, fmt.Errorf("cms: resolve endpoint: %w", err)
	}

	var workspaces []map[string]interface{}

	err = c.executeWithResilience(ctx, func(ctx context.Context) error {
		slog.DebugContext(ctx, "cms: list workspaces", "region", region, "endpoint", ep)

		// Build URL with query parameters
		u, err := url.Parse(fmt.Sprintf("https://%s/workspace", ep))
		if err != nil {
			return fmt.Errorf("parse url: %w", err)
		}
		q := u.Query()
		q.Set("maxResults", "100")
		q.Set("region", region)
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Accept", "application/json")

		// Sign the request
		if err := c.signRequest(ctx, req, nil, "ListWorkspaces"); err != nil {
			return fmt.Errorf("sign request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("http request: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			slog.ErrorContext(ctx, "cms: list workspaces failed",
				"status", resp.StatusCode,
				"body", string(body))
			return fmt.Errorf("api error: status %d, body: %s", resp.StatusCode, string(body))
		}

		var result struct {
			Workspaces []map[string]interface{} `json:"workspaces"`
			Total      int                      `json:"total"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}

		workspaces = result.Workspaces
		if workspaces == nil {
			workspaces = []map[string]interface{}{}
		}

		slog.DebugContext(ctx, "cms: list workspaces success",
			"count", len(workspaces),
			"total", result.Total)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("cms: list workspaces in %s: %w", region, err)
	}
	return workspaces, nil
}

// ExecuteSPL executes an SPL query against the specified workspace.
// API: POST /workspace/{workspace}/entitiesAndRelations
func (c *CMSClientImpl) ExecuteSPL(ctx context.Context, region, workspace, query string, from, to int64, limit int) (map[string]interface{}, error) {
	ep, err := c.resolver.Resolve(region)
	if err != nil {
		return nil, fmt.Errorf("cms: resolve endpoint: %w", err)
	}

	var result map[string]interface{}

	err = c.executeWithResilience(ctx, func(ctx context.Context) error {
		slog.DebugContext(ctx, "cms: execute spl",
			"region", region,
			"workspace", workspace,
			"query", query,
			"from", from,
			"to", to,
			"limit", limit)

		// Build request body
		reqBody := map[string]interface{}{
			"query": query,
			"from":  from,
			"to":    to,
		}
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}

		// Build URL
		u := fmt.Sprintf("https://%s/workspace/%s/entitiesAndRelations", ep, url.PathEscape(workspace))

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(bodyBytes))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		// Sign the request
		if err := c.signRequest(ctx, req, bodyBytes, "GetEntityStoreData"); err != nil {
			return fmt.Errorf("sign request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("http request: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			slog.ErrorContext(ctx, "cms: execute spl failed",
				"status", resp.StatusCode,
				"body", string(body))
			return fmt.Errorf("api error: status %d, body: %s", resp.StatusCode, string(body))
		}

		var apiResp struct {
			Data   [][]interface{} `json:"data"`
			Header []string        `json:"header"`
		}
		if err := json.Unmarshal(body, &apiResp); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}

		// Convert header+data arrays to object maps, matching Python's
		// pd.DataFrame(data, columns=header).to_dict('records') behavior.
		// This ensures Go output structure matches Python for all SPL-based tools.
		converted := convertSPLRowsToMaps(apiResp.Header, apiResp.Data)

		result = map[string]interface{}{
			"data":   converted,
			"header": apiResp.Header,
		}

		slog.DebugContext(ctx, "cms: execute spl success",
			"rows", len(apiResp.Data),
			"columns", len(apiResp.Header))
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("cms: execute spl %s/%s: %w", workspace, query, err)
	}
	return result, nil
}

// convertSPLRowsToMaps converts header+data arrays from the CMS API into
// []map[string]interface{}, matching Python's pd.DataFrame(data, columns=header)
// behavior. If header is empty, returns the raw data rows as-is.
func convertSPLRowsToMaps(header []string, data [][]interface{}) []map[string]interface{} {
	if len(header) == 0 || len(data) == 0 {
		return make([]map[string]interface{}, 0)
	}
	result := make([]map[string]interface{}, 0, len(data))
	for _, row := range data {
		m := make(map[string]interface{}, len(header))
		for i, col := range header {
			if i < len(row) {
				m[col] = row[i]
			} else {
				m[col] = nil
			}
		}
		result = append(result, m)
	}
	return result
}

// QueryMetric queries metric data from CMS.
// Note: For CMS 2.0, metric queries typically go through the entity store SPL interface.
func (c *CMSClientImpl) QueryMetric(ctx context.Context, region, namespace, metricName string, dimensions map[string]string, from, to int64) ([]map[string]interface{}, error) {
	if region == "" {
		return nil, fmt.Errorf("cms: region must not be empty")
	}

	slog.DebugContext(ctx, "cms: query metric",
		"region", region,
		"namespace", namespace,
		"metricName", metricName,
		"dimensions", dimensions,
		"from", from,
		"to", to)

	// For CMS 2.0, metric queries should use ExecuteSPL with appropriate SPL query
	// This is a placeholder for direct metric queries if needed
	return []map[string]interface{}{}, nil
}

// createCMSSDKClient creates a CMS SDK client for the given region.
func (c *CMSClientImpl) createCMSSDKClient(ctx context.Context, region string) (*cms.Client, error) {
	ep, err := c.resolver.Resolve(region)
	if err != nil {
		return nil, fmt.Errorf("cms: resolve endpoint: %w", err)
	}

	cred := c.resolveCredential(ctx)

	accessKeyID, err := cred.GetAccessKeyID()
	if err != nil {
		return nil, fmt.Errorf("cms: get access key id: %w", err)
	}

	accessKeySecret, err := cred.GetAccessKeySecret()
	if err != nil {
		return nil, fmt.Errorf("cms: get access key secret: %w", err)
	}

	cfg := &openapi.Config{
		AccessKeyId:     dara.String(accessKeyID),
		AccessKeySecret: dara.String(accessKeySecret),
		Endpoint:        dara.String(ep),
	}

	// Add security token if available (for STS)
	token, _ := cred.GetSecurityToken()
	if token != "" {
		cfg.SecurityToken = dara.String(token)
	}

	client, err := cms.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("cms: create sdk client: %w", err)
	}

	return client, nil
}

// TextToSQL converts natural language to SQL query using CMS Chat API with SSE streaming.
// This matches the Python implementation that uses CreateThread + CreateChatWithSSE.
func (c *CMSClientImpl) TextToSQL(ctx context.Context, region, project, logstore, text string) (string, error) {
	client, err := c.createCMSSDKClient(ctx, region)
	if err != nil {
		return "", err
	}

	const digitalEmployeeName = "apsara-ops"
	currentTime := time.Now().Unix()
	fromTime := time.Now().Unix() - 900 // last 15 minutes
	toTime := currentTime

	language := os.Getenv("LANGUAGE")
	if language == "" {
		language = "zh"
	}
	timeZone := os.Getenv("TIMEZONE")
	if timeZone == "" {
		timeZone = "Asia/Shanghai"
	}

	slog.DebugContext(ctx, "cms: text to sql",
		"region", region,
		"project", project,
		"logstore", logstore,
		"text", text,
		"from_time", fromTime,
		"to_time", toTime,
	)

	// Step 1: Create a thread
	threadRequest := &cms.CreateThreadRequest{}
	threadRequest.SetTitle(fmt.Sprintf("text2sql-%d", time.Now().Unix()))
	threadVariables := &cms.CreateThreadRequestVariables{}
	threadVariables.SetProject(project)
	threadRequest.SetVariables(threadVariables)

	slog.DebugContext(ctx, "cms: creating thread", "digital_employee", digitalEmployeeName)

	threadResp, err := client.CreateThread(dara.String(digitalEmployeeName), threadRequest)
	if err != nil {
		return "", fmt.Errorf("cms: create thread: %w", err)
	}

	if threadResp.Body == nil || threadResp.Body.ThreadId == nil {
		return "", fmt.Errorf("cms: create thread: missing thread_id in response")
	}

	threadID := dara.StringValue(threadResp.Body.ThreadId)
	slog.DebugContext(ctx, "cms: thread created", "thread_id", threadID)

	// Step 2: Create chat request with SSE
	chatRequest := &cms.CreateChatRequest{}
	chatRequest.SetDigitalEmployeeName(digitalEmployeeName)
	chatRequest.SetAction("create")
	chatRequest.SetThreadId(threadID)

	// Build message content
	content := &cms.CreateChatRequestMessagesContents{}
	content.SetType("text")
	content.SetValue(text)

	message := &cms.CreateChatRequestMessages{}
	message.SetRole("user")
	message.SetContents([]*cms.CreateChatRequestMessagesContents{content})

	chatRequest.SetMessages([]*cms.CreateChatRequestMessages{message})

	// Build variables matching Python implementation
	userContext, _ := json.Marshal([]map[string]interface{}{
		{
			"type": "metadata",
			"data": map[string]interface{}{
				"from_time": fromTime,
				"to_time":   toTime,
				"fromTime":  fromTime,
				"toTime":    toTime,
			},
		},
	})

	configJSON, _ := json.Marshal(map[string]interface{}{
		"disableThreadData": false,
	})

	variables := map[string]interface{}{
		"region":      region,
		"project":     project,
		"language":    language,
		"timeZone":    timeZone,
		"timeStamp":   fmt.Sprintf("%d", time.Now().Unix()),
		"logstore":    logstore,
		"startTime":   fromTime,
		"endTime":     toTime,
		"skill_name":  "sql_generation",
		"userContext": string(userContext),
		"config":      string(configJSON),
		"skill":       "sql_generation",
	}
	chatRequest.SetVariables(variables)

	slog.DebugContext(ctx, "cms: sending chat request", "thread_id", threadID)

	// Step 3: Call SSE API and collect responses
	runtime := &dara.RuntimeOptions{}
	runtime.SetReadTimeout(120000)
	runtime.SetConnectTimeout(30000)

	responseChan := make(chan *cms.CreateChatResponse, 100)
	errChan := make(chan error, 1)

	go client.CreateChatWithSSE(chatRequest, map[string]*string{}, runtime, responseChan, errChan)

	var collectedSQL string
	var collectedExplanation strings.Builder
	var traceID string

	// Process SSE responses
	for resp := range responseChan {
		if resp.Body == nil {
			continue
		}

		if resp.Body.TraceId != nil && *resp.Body.TraceId != "" {
			traceID = dara.StringValue(resp.Body.TraceId)
		}

		for _, msg := range resp.Body.Messages {
			if msg == nil {
				continue
			}

			// Check for tool calls (QuerySLSLogs)
			for _, tool := range msg.Tools {
				if tool == nil {
					continue
				}
				jsonTool, _ := json.Marshal(tool)
				slog.DebugContext(ctx, "cms: extracted tools", "contents", string(jsonTool))
				toolName, _ := tool["name"].(string)
				if toolName == "" {
					toolName, _ = tool["id"].(string)
				}
				if toolName == "QuerySLSLogs" && tool["arguments"] != nil {
					if args, ok := tool["arguments"].(map[string]interface{}); ok && args["query"] != nil && tool["status"] != nil && tool["status"].(string) == "start" {
						// TODO : 查询的sql语句，与返回日志中的不匹配
						if query, ok := args["query"].(string); ok && args["time_range"] != nil {
							collectedSQL = query
							slog.DebugContext(ctx, "cms: extracted SQL", "sql", collectedSQL)
							if time_range, ok := args["time_range"]; ok {
								if strings.ContainsRune(time_range.(string), '~') {
									times := strings.Split(time_range.(string), "~")
									f, _ := time.Parse("2006-01-02 15:04:05", times[0])
									fromTime = f.Unix()
									t, _ := time.Parse("2006-01-02 15:04:05", times[1])
									toTime = t.Unix()
								} else if strings.Contains(time_range.(string), "last_") {
									ts, err := timeparse.ParseTimeExpression(time_range.(string), time.Now())
									if err != nil {
										slog.ErrorContext(ctx, "cms: time parse error", "error", err)
									}
									fromTime = ts
									toTime = time.Now().Unix()
								}

							}
						}
					}
				}
			}
			if len(msg.Artifacts) > 0 {
				artifacts, _ := json.Marshal(msg.Artifacts)
				slog.DebugContext(ctx, "cms: extracted artifacts", "artifacts", string(artifacts))

			}
			for _, artifact := range msg.Artifacts {
				if artifact != nil && artifact["name"] != nil && artifact["name"].(string) == "Result" && artifact["parts"] != nil {
					switch artifact["parts"].(type) {
					case []interface{}:
						for _, part := range artifact["parts"].([]interface{}) {
							if part != nil {
								partInstance, ok := part.(map[string]interface{})
								if ok && partInstance["text"] != nil {
									collectedExplanation.WriteString(partInstance["text"].(string))
								}
							}
						}
					}
				}
			}

			if len(msg.Contents) > 0 {
				contents, _ := json.Marshal(msg.Contents)
				slog.DebugContext(ctx, "cms: extracted contents", "contents", string(contents))
			}

			// // Check for text content (explanation)
			// for _, contentItem := range msg.Contents {
			// 	if contentItem == nil {
			// 		continue
			// 	}
			// 	if contentType, _ := contentItem["type"].(string); contentType == "text" {
			// 		if textValue, _ := contentItem["value"].(string); textValue != "" {
			// 			collectedExplanation.WriteString(textValue)
			// 		}
			// 	}
			// }
		}
	}

	// Check for errors from SSE
	select {
	case err := <-errChan:
		if err != nil {
			return "", fmt.Errorf("cms: chat sse error: %w", err)
		}
	default:
	}

	// Build response data in the same format as Python sls_text_to_sql
	explanation := collectedExplanation.String()

	if collectedSQL != "" {
		slog.InfoContext(ctx, "cms: text to sql success",
			"sql", collectedSQL,
			"trace_id", traceID,
		)

		dataObj := map[string]interface{}{
			"answer":    collectedSQL,
			"message":   explanation,
			"to_time":   toTime,
			"from_time": fromTime,
		}
		dataJSON, _ := json.Marshal(dataObj)
		return string(dataJSON), nil
	}

	slog.WarnContext(ctx, "cms: text to sql failed: no SQL generated",
		"trace_id", traceID,
	)

	dataObj := map[string]interface{}{
		"answer":    "",
		"message":   explanation,
		"to_time":   fmt.Sprintf("%d", toTime),
		"from_time": fmt.Sprintf("%d", fromTime),
	}
	if explanation == "" {
		dataObj["message"] = "No SQL query generated"
	}
	dataJSON, _ := json.Marshal(dataObj)
	return string(dataJSON), nil
}

// DataAgentQuery performs a natural language data query using CMS CreateThread + CreateChatWithSSE API.
// This matches the Python implementation's _data_agent_query function.
func (c *CMSClientImpl) DataAgentQuery(ctx context.Context, region, workspace, query string, fromTime, toTime int64) (*DataAgentResult, error) {
	client, err := c.createCMSSDKClient(ctx, region)
	if err != nil {
		return nil, err
	}

	const digitalEmployeeName = "apsara-ops"
	const skill = "data-agent"

	language := os.Getenv("LANGUAGE")
	if language == "" {
		language = "zh"
	}
	timeZone := os.Getenv("TIMEZONE")
	if timeZone == "" {
		timeZone = "Asia/Shanghai"
	}

	slog.DebugContext(ctx, "cms: data agent query",
		"region", region,
		"workspace", workspace,
		"query", query,
		"from_time", fromTime,
		"to_time", toTime,
	)

	// Step 1: Create a thread
	threadRequest := &cms.CreateThreadRequest{}
	threadRequest.SetTitle(fmt.Sprintf("data-query-%d", time.Now().Unix()))
	threadVariables := &cms.CreateThreadRequestVariables{}
	threadVariables.SetWorkspace(workspace)
	threadRequest.SetVariables(threadVariables)

	slog.DebugContext(ctx, "cms: creating data-agent thread", "digital_employee", digitalEmployeeName)

	threadResp, err := client.CreateThread(dara.String(digitalEmployeeName), threadRequest)
	if err != nil {
		return nil, fmt.Errorf("cms: create thread: %w", err)
	}

	if threadResp.Body == nil || threadResp.Body.ThreadId == nil {
		return nil, fmt.Errorf("cms: create thread: missing thread_id in response")
	}

	threadID := dara.StringValue(threadResp.Body.ThreadId)
	slog.DebugContext(ctx, "cms: data-agent thread created", "thread_id", threadID)

	// Step 2: Create chat request with SSE
	chatRequest := &cms.CreateChatRequest{}
	chatRequest.SetDigitalEmployeeName(digitalEmployeeName)
	chatRequest.SetAction("create")
	chatRequest.SetThreadId(threadID)

	// Build message content
	content := &cms.CreateChatRequestMessagesContents{}
	content.SetType("text")
	content.SetValue(query)

	message := &cms.CreateChatRequestMessages{}
	message.SetRole("user")
	message.SetContents([]*cms.CreateChatRequestMessagesContents{content})

	chatRequest.SetMessages([]*cms.CreateChatRequestMessages{message})

	// Build variables matching Python implementation
	userContext, _ := json.Marshal([]map[string]interface{}{
		{
			"type": "metadata",
			"data": map[string]interface{}{
				"from_time": fromTime,
				"to_time":   toTime,
				"fromTime":  fromTime,
				"toTime":    toTime,
			},
		},
	})

	configJSON, _ := json.Marshal(map[string]interface{}{
		"disableThreadData": false,
	})

	variables := map[string]interface{}{
		"region":      region,
		"workspace":   workspace,
		"language":    language,
		"timeZone":    timeZone,
		"timeStamp":   fmt.Sprintf("%d", time.Now().Unix()),
		"startTime":   fromTime,
		"endTime":     toTime,
		"skill_name":  skill,
		"userContext": string(userContext),
		"config":      string(configJSON),
		"skill":       skill,
	}
	chatRequest.SetVariables(variables)

	slog.DebugContext(ctx, "cms: sending data-agent chat request", "thread_id", threadID)

	// Step 3: Call SSE API and collect responses
	runtime := &dara.RuntimeOptions{}
	runtime.SetReadTimeout(180000) // 3 minutes for data-agent (can be slow)
	runtime.SetConnectTimeout(30000)

	responseChan := make(chan *cms.CreateChatResponse, 100)
	errChan := make(chan error, 1)

	go client.CreateChatWithSSE(chatRequest, map[string]*string{}, runtime, responseChan, errChan)

	var collectedData []interface{}
	var collectedText strings.Builder
	var collectedToolResults []map[string]interface{}
	var collectedSQL string
	var traceID string

	// Process SSE responses
	for resp := range responseChan {
		if resp.Body == nil {
			continue
		}

		if resp.Body.TraceId != nil && *resp.Body.TraceId != "" {
			traceID = dara.StringValue(resp.Body.TraceId)
		}

		for _, msg := range resp.Body.Messages {
			if msg == nil {
				continue
			}

			msgRole := ""
			if msg.Role != nil {
				msgRole = *msg.Role
			}
			msgType := ""
			if msg.Type != nil {
				msgType = *msg.Type
			}

			slog.DebugContext(ctx, "cms: data-agent SSE message",
				"role", msgRole,
				"type", msgType,
				"tools_count", len(msg.Tools),
				"contents_count", len(msg.Contents),
				"artifacts_count", len(msg.Artifacts),
				"events_count", len(msg.Events),
			)

			// Process tool calls
			for i, tool := range msg.Tools {
				if tool == nil {
					continue
				}

				toolJSON, _ := json.Marshal(tool)
				slog.DebugContext(ctx, "cms: data-agent raw tool",
					"index", i,
					"raw", string(toolJSON))

				toolName, _ := tool["name"].(string)
				if toolName == "" {
					toolName, _ = tool["id"].(string)
				}
				toolStatus, _ := tool["status"].(string)

				slog.DebugContext(ctx, "cms: data-agent tool call",
					"name", toolName, "status", toolStatus)

				// Extract SQL if QuerySLSLogs tool
				if toolName == "QuerySLSLogs" {
					if args, ok := tool["arguments"].(map[string]interface{}); ok {
						if q, ok := args["query"].(string); ok {
							collectedSQL = q
							slog.DebugContext(ctx, "cms: data-agent extracted SQL", "sql", collectedSQL)
						}
					}
				}

				// Process tool contents (data-agent returned data)
				// Check both "success" and "end" statuses, as some tools report data at "end"
				if toolStatus == "success" || toolStatus == "end" {
					if toolContents, ok := tool["contents"].([]interface{}); ok {
						for j, tc := range toolContents {
							tcMap, ok := tc.(map[string]interface{})
							if !ok {
								slog.DebugContext(ctx, "cms: data-agent tool content not map",
									"index", j, "type", fmt.Sprintf("%T", tc))
								continue
							}
							tcValue := tcMap["value"]
							tcType, _ := tcMap["type"].(string)
							if tcValue == nil {
								continue
							}

							slog.DebugContext(ctx, "cms: data-agent tool content",
								"tool", toolName,
								"content_type", tcType,
								"value_type", fmt.Sprintf("%T", tcValue),
								"value_preview", truncateStr(fmt.Sprintf("%v", tcValue), 300))

							// Try to parse as JSON if string
							var parsed interface{}
							if strVal, ok := tcValue.(string); ok {
								if err := json.Unmarshal([]byte(strVal), &parsed); err != nil {
									slog.DebugContext(ctx, "cms: data-agent JSON parse failed, trying as raw",
										"error", err.Error(),
										"value_preview", truncateStr(strVal, 200))
									continue
								}
							} else {
								parsed = tcValue
							}

							// Collect typed data (entity_list, metric_set_query, data_query, etc.)
							switch p := parsed.(type) {
							case map[string]interface{}:
								if p["type"] != nil {
									collectedData = append(collectedData, p)
									slog.DebugContext(ctx, "cms: data-agent collected typed data",
										"data_type", p["type"])
								} else {
									// Even without "type", collect if it has "data" field
									if p["data"] != nil {
										collectedData = append(collectedData, p)
										slog.DebugContext(ctx, "cms: data-agent collected data (no type field)")
									}
								}
							case []interface{}:
								// Array of results - check each item
								for _, item := range p {
									if m, ok := item.(map[string]interface{}); ok && m["type"] != nil {
										collectedData = append(collectedData, m)
									}
								}
							}
						}
					} else {
						// Log if contents exists but isn't []interface{}
						if tool["contents"] != nil {
							contentsJSON, _ := json.Marshal(tool["contents"])
							slog.DebugContext(ctx, "cms: data-agent tool contents unexpected type",
								"tool", toolName,
								"contents_type", fmt.Sprintf("%T", tool["contents"]),
								"contents_raw", truncateStr(string(contentsJSON), 300))
						}
					}
				}

				// Collect tool results
				if toolResult := tool["result"]; toolResult != nil {
					collectedToolResults = append(collectedToolResults, map[string]interface{}{
						"tool":      toolName,
						"result":    toolResult,
						"arguments": tool["arguments"],
						"status":    toolStatus,
					})
				}
			}

			// Process text contents
			for i, contentItem := range msg.Contents {
				if contentItem == nil {
					continue
				}
				contentType, _ := contentItem["type"].(string)

				slog.DebugContext(ctx, "cms: data-agent content item",
					"index", i,
					"type", contentType,
					"keys", mapKeys(contentItem))

				// Handle "value" as either string or other types
				var contentStr string
				var contentRaw interface{}
				switch v := contentItem["value"].(type) {
				case string:
					contentStr = v
				default:
					contentRaw = v
				}

				if contentType == "text" && strings.TrimSpace(contentStr) != "" {
					collectedText.WriteString(contentStr)
				}
				if contentType == "data" {
					if contentStr != "" {
						var dataValue interface{}
						if err := json.Unmarshal([]byte(contentStr), &dataValue); err == nil {
							collectedData = append(collectedData, dataValue)
							slog.DebugContext(ctx, "cms: data-agent collected content data (string)")
						}
					} else if contentRaw != nil {
						collectedData = append(collectedData, contentRaw)
						slog.DebugContext(ctx, "cms: data-agent collected content data (raw)")
					}
				}
			}

			// Process events (may contain data in some responses)
			for _, event := range msg.Events {
				if event == nil {
					continue
				}
				eventJSON, _ := json.Marshal(event)
				slog.DebugContext(ctx, "cms: data-agent event", "raw", truncateStr(string(eventJSON), 300))

				// Check if event contains data
				if eventData, ok := event["data"]; ok && eventData != nil {
					switch ed := eventData.(type) {
					case map[string]interface{}:
						if ed["type"] != nil {
							collectedData = append(collectedData, ed)
						}
					case string:
						var parsed interface{}
						if err := json.Unmarshal([]byte(ed), &parsed); err == nil {
							if m, ok := parsed.(map[string]interface{}); ok && m["type"] != nil {
								collectedData = append(collectedData, m)
							}
						}
					}
				}
			}

			// Process artifacts
			for _, artifact := range msg.Artifacts {
				if artifact == nil {
					continue
				}

				artifactJSON, _ := json.Marshal(artifact)
				slog.DebugContext(ctx, "cms: data-agent artifact", "raw", truncateStr(string(artifactJSON), 300))

				artifactValue := artifact["value"]
				if artifactValue == nil {
					// Also check "parts" field (used in some responses)
					if parts, ok := artifact["parts"].([]interface{}); ok {
						for _, part := range parts {
							if partMap, ok := part.(map[string]interface{}); ok {
								if textVal, ok := partMap["text"].(string); ok {
									var parsed interface{}
									if err := json.Unmarshal([]byte(textVal), &parsed); err == nil {
										if m, ok := parsed.(map[string]interface{}); ok && m["type"] != nil {
											collectedData = append(collectedData, m)
										}
									}
								}
							}
						}
					}
					continue
				}
				var artifactData interface{}
				switch v := artifactValue.(type) {
				case string:
					if err := json.Unmarshal([]byte(v), &artifactData); err != nil {
						continue
					}
				default:
					artifactData = v
				}

				switch ad := artifactData.(type) {
				case []interface{}:
					for _, item := range ad {
						if m, ok := item.(map[string]interface{}); ok && m["type"] != nil {
							collectedData = append(collectedData, m)
						}
					}
				case map[string]interface{}:
					if ad["type"] != nil {
						collectedData = append(collectedData, ad)
					}
				}
			}
		}
	}

	// Check for errors from SSE
	select {
	case err := <-errChan:
		if err != nil {
			return nil, fmt.Errorf("cms: data-agent sse error: %w", err)
		}
	default:
	}

	slog.InfoContext(ctx, "cms: data-agent query complete",
		"trace_id", traceID,
		"data_count", len(collectedData),
		"text_length", collectedText.Len(),
		"tool_results_count", len(collectedToolResults),
		"has_sql", collectedSQL != "",
	)

	return &DataAgentResult{
		QueryResults: collectedData,
		ToolResults:  collectedToolResults,
		GeneratedSQL: collectedSQL,
		Message:      collectedText.String(),
		TraceID:      traceID,
	}, nil
}

// truncateStr truncates a string to maxLen characters, appending "..." if truncated.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// mapKeys returns the keys of a map[string]interface{} as a sorted slice.
func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
