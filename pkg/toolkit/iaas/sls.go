// Package iaas implements the IaaS Toolkit, providing tools for direct access
// to SLS and CMS underlying APIs. SLS tool names use the `sls_` prefix.
package iaas

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/alibabacloud-observability-mcp-server-go/pkg/client"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/timeparse"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/toolkit"
)

// SLSTools returns all SLS tools backed by the given SLS and CMS clients.
// The CMS client is needed for sls_execute_spl when using workspace parameter.
func SLSTools(slsClient client.SLSClient, cmsClient client.CMSClient) []toolkit.Tool {
	h := &slsHandler{slsClient: slsClient, cmsClient: cmsClient}
	return []toolkit.Tool{
		h.listProjectsTool(),
		h.listLogstoresTool(),
		h.textToSQLTool(),
		h.textToSQLOldTool(), // Deprecated alias for Python compatibility
		h.textToPromQLTool(),
		h.sopTool(),
		h.executeSQLTool(),
		h.executeSPLTool(),
		h.getContextLogsTool(),
		h.textToSPLTool(),
		h.logExploreTool(),
		h.logCompareTool(),
	}
}

// slsHandler holds the SLS and CMS clients and provides tool constructors and handlers.
type slsHandler struct {
	slsClient client.SLSClient
	cmsClient client.CMSClient
}

// ---------------------------------------------------------------------------
// Helper: parameter extraction (same pattern as paas package)
// ---------------------------------------------------------------------------

func paramString(params map[string]interface{}, key, defaultVal string) string {
	v, ok := params[key]
	if !ok || v == nil {
		return defaultVal
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return defaultVal
	}
	return s
}

func paramInt(params map[string]interface{}, key string, defaultVal int) int {
	v, ok := params[key]
	if !ok || v == nil {
		return defaultVal
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return defaultVal
}

func paramBool(params map[string]interface{}, key string, defaultVal bool) bool {
	v, ok := params[key]
	if !ok || v == nil {
		return defaultVal
	}
	b, ok := v.(bool)
	if !ok {
		return defaultVal
	}
	return b
}

// buildResponse builds a standard tool response.
func buildResponse(data interface{}, isError bool, message string) map[string]interface{} {
	if message == "" {
		if isError {
			message = "Query failed"
		} else if data == nil {
			message = "No data found"
		} else {
			message = "success"
		}
	}
	return map[string]interface{}{
		"error":   isError,
		"data":    data,
		"message": message,
	}
}

// parseTimeParam parses a time parameter (string or numeric) into a Unix timestamp.
// parseTimeParamAt 与 parseTimeParam 相同，但使用调用方提供的 now 时刻，
// 避免多次调用之间因 time.Now() 不同导致的时间漂移。
func parseTimeParamAt(params map[string]interface{}, key, defaultVal string, now time.Time) (int64, error) {
	v, ok := params[key]
	if !ok || v == nil {
		return timeparse.ParseTimeExpression(defaultVal, now)
	}
	switch val := v.(type) {
	case string:
		if val == "" {
			return timeparse.ParseTimeExpression(defaultVal, now)
		}
		return timeparse.ParseTimeExpression(val, now)
	case float64:
		ts := int64(val)
		if ts > 9999999999 {
			ts = ts / 1000
		}
		return ts, nil
	case int64:
		if val > 9999999999 {
			val = val / 1000
		}
		return val, nil
	case int:
		ts := int64(val)
		if ts > 9999999999 {
			ts = ts / 1000
		}
		return ts, nil
	default:
		return 0, fmt.Errorf("unsupported type %T for parameter %s", v, key)
	}
}

func parseTimeParam(params map[string]interface{}, key, defaultVal string) (int64, error) {
	return parseTimeParamAt(params, key, defaultVal, time.Now())
}

// isMetricStoreNotFoundError checks whether the error indicates the metric store does not exist.
func isMetricStoreNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "LogStoreNotExist") ||
		strings.Contains(msg, "MetricStoreNotExist") ||
		strings.Contains(msg, "not exist")
}

// ===========================================================================
// Tool 1: sls_list_projects
// ===========================================================================

func (h *slsHandler) listProjectsTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "sls_list_projects",
		Description: `List all projects in Alibaba Cloud Log Service.

## Overview

Lists all SLS projects in a specified region, with optional fuzzy search by project name.
If no project name is provided, all projects in the region are returned.

## Use Cases

- Check whether a specific project exists
- List all available SLS projects in a region
- Search for projects by partial name match

## Response Structure

Each project includes:
- project_name: Project name
- description: Project description
- region_id: Region where the project resides

## Examples

- "Is there a project named XXX"
- "List all SLS projects"`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"projectName": map[string]interface{}{
					"type":        "string",
					"description": "Project name query string, supports fuzzy search",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results, range 1-100, default 50",
					"default":     50,
					"minimum":     1,
					"maximum":     100,
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID, e.g. 'cn-hongkong'",
				},
			},
			"required": []string{"regionId"},
		},
		Handler: h.handleListProjects,
	}
}

func (h *slsHandler) handleListProjects(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	regionID := paramString(params, "regionId", "")
	projectName := paramString(params, "projectName", "")
	limit := paramInt(params, "limit", 50)

	if regionID == "" {
		return buildResponse(nil, true, "regionId is required"), nil
	}

	slog.InfoContext(ctx, "sls_list_projects", "region", regionID, "projectName", projectName, "limit", limit)

	projects, err := h.slsClient.ListProjectsWithFilter(ctx, regionID, projectName, limit)
	if err != nil {
		slog.ErrorContext(ctx, "sls_list_projects failed", "error", err)
		return buildResponse(nil, true, fmt.Sprintf("Failed to list projects: %s", err)), nil
	}

	return buildResponse(map[string]interface{}{
		"projects": projects,
		"message":  fmt.Sprintf("Currently limited to %d projects to prevent excessive response size. To find more projects, provide a keyword to filter by project name.", limit),
	}, false, ""), nil
}

// ===========================================================================
// Tool 4: sls_list_logstores
// ===========================================================================

func (h *slsHandler) listLogstoresTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "sls_list_logstores",
		Description: `List logstores in an SLS project.

## Overview

Lists all logstores in a specified SLS project. Defaults to logstore type if not specified.
Supports fuzzy search by logstore name. If no logstore name is provided, all logstores in the project are returned.

## Use Cases

- Check whether a specific logstore exists in a project
- List all available logstores in a project
- Search for logstores by partial name match
- If the project parameter is not available from context, use sls_list_projects to get the project list (unless the user explicitly asks to iterate)

## Metric Store

To search for metric or time-series stores, set the isMetricStore parameter to true.

## Examples

- "Is there a logstore named XXX"
- "What logstores does a project have"`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS project name, must be an exact match, cannot contain Chinese characters",
				},
				"logStore": map[string]interface{}{
					"type":        "string",
					"description": "Logstore name, supports fuzzy search",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results, range 1-100, default 10",
					"default":     10,
					"minimum":     1,
					"maximum":     100,
				},
				"isMetricStore": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to query metric stores, default false. Set to true only when searching for metric stores",
					"default":     false,
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID, e.g. 'cn-hongkong'",
				},
			},
			"required": []string{"project", "regionId"},
		},
		Handler: h.handleListLogstores,
	}
}

func (h *slsHandler) handleListLogstores(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	project := paramString(params, "project", "")
	logStore := paramString(params, "logStore", "")
	regionID := paramString(params, "regionId", "")
	limit := paramInt(params, "limit", 10)
	isMetricStore := paramBool(params, "isMetricStore", false)

	if project == "" {
		return buildResponse(map[string]interface{}{
			"total":     0,
			"logstores": []interface{}{},
			"message":   "Please specify the project name, if you want to list all projects, please use sls_list_projects tool",
		}, false, ""), nil
	}

	if regionID == "" {
		return buildResponse(nil, true, "regionId is required"), nil
	}

	slog.InfoContext(ctx, "sls_list_logstores", "project", project, "logStore", logStore, "region", regionID, "limit", limit, "isMetricStore", isMetricStore)

	// Use the extended ListLogStores method with filter parameters
	logstores, err := h.slsClient.ListLogStoresWithFilter(ctx, regionID, project, logStore, limit, isMetricStore)
	if err != nil {
		slog.ErrorContext(ctx, "sls_list_logstores failed", "error", err)
		return buildResponse(nil, true, fmt.Sprintf("Failed to list logstores: %s", err)), nil
	}

	total := len(logstores)
	message := fmt.Sprintf("Currently limited to %d logstores to prevent excessive response size. To find more logstores, provide a keyword to filter by logstore name.", limit)
	if total == 0 {
		message = "Sorry not found logstore, please make sure your project and region or logstore name is correct, if you want to find metric store, please check isMetricStore parameter"
	}

	return buildResponse(map[string]interface{}{
		"total":     total,
		"logstores": logstores,
		"message":   message,
	}, false, ""), nil
}

// ===========================================================================
// Tool 3: sls_text_to_sql
// ===========================================================================

func (h *slsHandler) textToSQLTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "sls_text_to_sql",
		Description: `Convert natural language to an SLS query statement. When the user has a clear logstore query requirement, this tool must be used first to generate the query.

## Overview

Converts a natural language description into a valid SLS query statement, allowing users to express query requirements in plain language.

## Use Cases

- When the user is unfamiliar with SLS query syntax
- When a complex query needs to be built quickly
- When query intent needs to be extracted from a natural language description

## Limitations

- Only supports generating SLS queries, not SQL for other databases
- Generates a query statement, not query results; use with sls_query_logstore to execute

## Best Practices

- Provide a clear and concise natural language description
- Do not include project or logstore names in the description

## Examples

- "Generate a log query for XXX"
- "Find error logs in the last hour"`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text": map[string]interface{}{
					"type":        "string",
					"description": "Natural language text for query generation",
				},
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS project name",
				},
				"logStore": map[string]interface{}{
					"type":        "string",
					"description": "SLS logstore name",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID, e.g. 'cn-hongkong'",
				},
			},
			"required": []string{"text", "project", "logStore", "regionId"},
		},
		Handler: h.handleTextToSQL,
	}
}

func (h *slsHandler) handleTextToSQL(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	text := paramString(params, "text", "")
	project := paramString(params, "project", "")
	logStore := paramString(params, "logStore", "")
	regionID := paramString(params, "regionId", "")

	if text == "" || project == "" || logStore == "" || regionID == "" {
		return buildResponse(nil, true, "text, project, logStore and regionId are required"), nil
	}

	slog.InfoContext(ctx, "sls_text_to_sql",
		"project", project, "logStore", logStore, "region", regionID)

	sql, err := h.slsClient.TextToSQL(ctx, regionID, project, logStore, text)
	if err != nil {
		slog.ErrorContext(ctx, "sls_text_to_sql failed", "error", err)
		return buildResponse(nil, true, fmt.Sprintf("Text to SQL failed: %s", err)), nil
	}

	return buildResponse(map[string]interface{}{
		"query": sql,
	}, false, ""), nil
}

// ===========================================================================
// Tool 6b: sls_text_to_sql_old (Deprecated alias for Python compatibility)
// ===========================================================================

func (h *slsHandler) textToSQLOldTool() toolkit.Tool {
	// Get the base tool and modify its name and description
	baseTool := h.textToSQLTool()
	return toolkit.Tool{
		Name: "sls_text_to_sql_old",
		Description: `⚠️ [Deprecated] Convert natural language to an SLS query statement (legacy API).

## Deprecation Notice

This tool is deprecated. Please use sls_text_to_sql instead.

The new sls_text_to_sql tool uses the CMS Chat API, providing smarter SQL generation
with better context understanding, more accurate query generation, and more detailed explanations.

## Overview

This tool uses the legacy SLS CallAiTools API to convert natural language descriptions into valid SLS query statements.

## Use Cases

- Only as a fallback when the new sls_text_to_sql tool is unavailable

## Limitations

- Only supports generating SLS queries, not SQL for other databases
- Generates a query statement, not query results; use with sls_query_logstore to execute
- Requires the target logstore to have index configuration`,
		InputSchema: baseTool.InputSchema,
		Handler:     h.handleTextToSQL, // Reuse the same handler
	}
}

// ===========================================================================
// Tool 7: sls_text_to_promql
// ===========================================================================

func (h *slsHandler) textToPromQLTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "sls_text_to_promql",
		Description: `Convert natural language to a PromQL query statement.

## Overview

Converts a natural language description into a valid PromQL query statement, allowing users to express query requirements in plain language.

## Use Cases

- When the user is unfamiliar with PromQL query syntax
- When a complex query needs to be built quickly

## Limitations

- Only supports generating PromQL queries
- Generates a query statement, not query results

## Best Practices

- Provide a clear and concise natural language description
- Do not include project or metric store names in the description

## Examples

- "Generate a PromQL query for XXX"
- "Query the number of Pods per namespace"`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text": map[string]interface{}{
					"type":        "string",
					"description": "Natural language text for PromQL generation",
				},
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS project name",
				},
				"metricStore": map[string]interface{}{
					"type":        "string",
					"description": "SLS metric store name",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID, e.g. 'cn-hongkong'",
				},
			},
			"required": []string{"text", "project", "metricStore", "regionId"},
		},
		Handler: h.handleTextToPromQL,
	}
}

func (h *slsHandler) handleTextToPromQL(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	text := paramString(params, "text", "")
	project := paramString(params, "project", "")
	metricStore := paramString(params, "metricStore", "")
	regionID := paramString(params, "regionId", "")

	if text == "" || project == "" || metricStore == "" || regionID == "" {
		return buildResponse(nil, true, "text, project, metricStore and regionId are required"), nil
	}

	slog.InfoContext(ctx, "sls_text_to_promql",
		"project", project, "metricStore", metricStore, "region", regionID)

	// Uses the same TextToSQL underlying mechanism for PromQL generation
	promql, err := h.slsClient.TextToSQL(ctx, regionID, project, metricStore, text)
	if err != nil {
		slog.ErrorContext(ctx, "sls_text_to_promql failed", "error", err)
		return buildResponse(nil, true, fmt.Sprintf("Text to PromQL failed: %s", err)), nil
	}

	return buildResponse(map[string]interface{}{
		"query": promql,
	}, false, ""), nil
}

// ===========================================================================
// Tool 8: sls_sop
// ===========================================================================

func (h *slsHandler) sopTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "sls_sop",
		Description: `SLS SOP (Standard Operating Procedure) intelligent operations assistant.

## Overview

An intelligent assistant that answers questions about SLS usage, features, and operational procedures.

## Use Cases

- When the user does not know how to use a specific SLS feature
- When the user needs to understand SLS concepts or terminology
- When the user encounters operational issues and needs guidance

## Examples

- "How to create a new data transformation"
- "What is a Project"
- "How to configure alerts"`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text": map[string]interface{}{
					"type":        "string",
					"description": "User question about SLS usage or SOP",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID",
				},
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS project name (optional)",
				},
				"logStore": map[string]interface{}{
					"type":        "string",
					"description": "SLS logstore name (optional)",
				},
			},
			"required": []string{"text", "regionId"},
		},
		Handler: h.handleSOP,
	}
}

func (h *slsHandler) handleSOP(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	text := paramString(params, "text", "")
	regionID := paramString(params, "regionId", "")
	project := paramString(params, "project", "")
	logStore := paramString(params, "logStore", "")

	if text == "" || regionID == "" {
		return buildResponse(nil, true, "text and regionId are required"), nil
	}

	slog.InfoContext(ctx, "sls_sop",
		"region", regionID, "project", project, "logStore", logStore)

	// SOP uses CMS Chat API with "sop" skill for operations Q&A
	answer, err := h.cmsClient.ChatWithSkill(ctx, regionID, project, logStore, text, "sop")
	if err != nil {
		slog.ErrorContext(ctx, "sls_sop failed", "error", err)
		return buildResponse(nil, true, fmt.Sprintf("SOP query failed: %s", err)), nil
	}

	return buildResponse(map[string]interface{}{
		"answer": answer,
	}, false, ""), nil
}

// ===========================================================================
// Tool 9: sls_execute_sql
// ===========================================================================

func (h *slsHandler) executeSQLTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "sls_execute_sql",
		Description: `Execute an SLS log query.

## Overview

Executes a query on a specified SLS project and logstore, and returns the results.

## Use Cases

- Query log data based on specific conditions
- Analyze log information within a specific time range
- Search for specific events or errors in logs
- Compute aggregated statistics from log data

## Query Syntax

Queries must use valid SLS query syntax, not natural language. If you are unfamiliar with the logstore schema, use sls_list_logstores first to retrieve index information.

## Time Range

- from_time: Start time. Supports Unix timestamps (seconds/milliseconds) or relative expressions (e.g. 'now-1h')
- to_time: End time. Supports Unix timestamps (seconds/milliseconds) or relative expressions (e.g. 'now')`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS project name",
				},
				"logStore": map[string]interface{}{
					"type":        "string",
					"description": "SLS logstore name",
				},
				"query": map[string]interface{}{
					"type":        "string",
					"description": "SLS query statement",
				},
				"from_time": map[string]interface{}{
					"type":        "string",
					"description": "Start time. Unix timestamp or relative expression, e.g. 'now-5m'",
					"default":     "now-5m",
				},
				"to_time": map[string]interface{}{
					"type":        "string",
					"description": "End time. Unix timestamp or relative expression, e.g. 'now'",
					"default":     "now",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results, range 1-100, default 10",
					"default":     10,
				},
				"offset": map[string]interface{}{
					"type":        "integer",
					"description": "Query start offset for pagination, default 0",
					"default":     0,
				},
				"reverse": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to return results in descending timestamp order, default false",
					"default":     false,
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID, e.g. 'cn-hongkong'",
				},
			},
			"required": []string{"project", "logStore", "query", "regionId"},
		},
		Handler: h.handleExecuteSQL,
	}
}

func (h *slsHandler) handleExecuteSQL(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	project := paramString(params, "project", "")
	logStore := paramString(params, "logStore", "")
	query := paramString(params, "query", "")
	regionID := paramString(params, "regionId", "")
	limit := paramInt(params, "limit", 10)
	offset := paramInt(params, "offset", 0)
	reverse := paramBool(params, "reverse", false)

	if project == "" || logStore == "" || query == "" || regionID == "" {
		return buildResponse(nil, true, "project, logStore, query and regionId are required"), nil
	}

	fromTS, err := parseTimeParam(params, "from_time", "now-5m")
	if err != nil {
		return buildResponse(nil, true, fmt.Sprintf("invalid from_time: %s", err)), nil
	}
	toTS, err := parseTimeParam(params, "to_time", "now")
	if err != nil {
		return buildResponse(nil, true, fmt.Sprintf("invalid to_time: %s", err)), nil
	}

	slog.InfoContext(ctx, "sls_execute_sql",
		"project", project, "logStore", logStore, "region", regionID,
		"limit", limit, "offset", offset, "reverse", reverse)

	results, err := h.slsClient.Query(ctx, regionID, project, logStore, query, fromTS, toTS)
	if err != nil {
		slog.ErrorContext(ctx, "sls_execute_sql failed", "error", err)
		return buildResponse(nil, true, fmt.Sprintf("Query failed: %s", err)), nil
	}

	return buildResponse(results, false, ""), nil
}

// ===========================================================================
// Tool 10: sls_execute_spl
// ===========================================================================

func (h *slsHandler) executeSPLTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "sls_execute_spl",
		Description: `Execute a native SPL query statement.

## Overview

Allows direct execution of native SPL (Search Processing Language) query statements via CMS workspace,
providing maximum flexibility and functionality for advanced users. Supports complex data operations and analysis.

## Use Cases

- Complex data analysis and statistical computation beyond standard API coverage
- Custom data aggregation and transformation operations
- Cross-entity-set join queries and correlation analysis
- Advanced data mining and machine learning analysis

## Notes

- Requires familiarity with SPL syntax
- Ensure query correctness; invalid queries may return no results or errors
- Complex queries may consume significant compute resources and time`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Native SPL query statement",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS workspace name, obtainable via list_workspace",
				},
				"from_time": map[string]interface{}{
					"type":        "string",
					"description": "Start time. Unix timestamp or relative expression, e.g. 'now-5m'",
					"default":     "now-5m",
				},
				"to_time": map[string]interface{}{
					"type":        "string",
					"description": "End time. Unix timestamp or relative expression, e.g. 'now'",
					"default":     "now",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID, e.g. 'cn-hongkong'",
				},
			},
			"required": []string{"query", "workspace", "regionId"},
		},
		Handler: h.handleExecuteSPL,
	}
}

func (h *slsHandler) handleExecuteSPL(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	query := paramString(params, "query", "")
	workspace := paramString(params, "workspace", "")
	regionID := paramString(params, "regionId", "")

	if query == "" || workspace == "" || regionID == "" {
		return buildResponse(nil, true, "query, workspace and regionId are required"), nil
	}

	fromTS, err := parseTimeParam(params, "from_time", "now-5m")
	if err != nil {
		return buildResponse(nil, true, fmt.Sprintf("invalid from_time: %s", err)), nil
	}
	toTS, err := parseTimeParam(params, "to_time", "now")
	if err != nil {
		return buildResponse(nil, true, fmt.Sprintf("invalid to_time: %s", err)), nil
	}

	slog.InfoContext(ctx, "sls_execute_spl",
		"workspace", workspace, "region", regionID,
		"from", fromTS, "to", toTS)

	result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, 1000)
	if err != nil {
		slog.ErrorContext(ctx, "sls_execute_spl failed", "error", err)
		return buildResponse(nil, true, fmt.Sprintf("SPL query failed: %s", err)), nil
	}

	data := result["data"]

	return map[string]interface{}{
		"error":   false,
		"data":    data,
		"message": "SPL query executed successfully",
		"query":   query,
		"time_range": map[string]interface{}{
			"from":          fromTS,
			"to":            toTS,
			"from_readable": time.Unix(fromTS, 0).Format(time.RFC3339),
			"to_readable":   time.Unix(toTS, 0).Format(time.RFC3339),
			"expression":    fmt.Sprintf("%s ~ %s", paramString(params, "from_time", "now-5m"), paramString(params, "to_time", "now")),
		},
	}, nil
}

// ===========================================================================
// Tool 11: sls_get_context_logs
// ===========================================================================

func (h *slsHandler) getContextLogsTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "sls_get_context_logs",
		Description: `Query context logs around a specified log entry.

## Overview

Queries logs before (backward) and after (forward) a given starting log entry, using its pack_id and pack_meta.

Note: The time range for context queries is fixed to one day before and after the starting log (enforced by the SLS service).

## How to Obtain pack_id and pack_meta

First use sls_execute_sql to query the target log, appending |with_pack_meta to the query statement.
The results will include internal fields:
- __pack_id__: corresponds to this tool's pack_id
- __pack_meta__: corresponds to this tool's pack_meta

Then select the log entry you want as the starting point and pass these two field values to this tool.

Note: Some logstores (e.g. monitoring logstores) may not return __pack_id__. In that case, context log queries are not supported for that logstore.

## Parameters

- back_lines / forward_lines: Range 0-100, and at least one must be greater than 0.

## Response

The response structure matches the SLS OpenAPI. Each log in the logs array includes:
- __index_number__: Position relative to the starting log (negative = before, 0 = starting log, positive = after)`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS project name",
				},
				"logStore": map[string]interface{}{
					"type":        "string",
					"description": "SLS logstore name",
				},
				"pack_id": map[string]interface{}{
					"type":        "string",
					"description": "pack_id of the starting log entry",
				},
				"pack_meta": map[string]interface{}{
					"type":        "string",
					"description": "pack_meta of the starting log entry",
				},
				"back_lines": map[string]interface{}{
					"type":        "integer",
					"description": "Number of log lines to query backward, range 0-100, default 10",
					"default":     10,
				},
				"forward_lines": map[string]interface{}{
					"type":        "integer",
					"description": "Number of log lines to query forward, range 0-100, default 10",
					"default":     10,
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID, e.g. 'cn-hongkong'",
				},
			},
			"required": []string{"project", "logStore", "pack_id", "pack_meta", "regionId"},
		},
		Handler: h.handleGetContextLogs,
	}
}

// isInvalidPackValue checks whether a pack_id or pack_meta value is obviously
// invalid — e.g. placeholder strings, test values, or values that are too short
// to be real SLS pack identifiers.
func isInvalidPackValue(v string) bool {
	if len(v) < 5 {
		return true
	}
	lower := strings.ToLower(v)
	placeholders := []string{"test", "placeholder", "example", "dummy", "fake", "mock", "sample"}
	for _, p := range placeholders {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// packValueHint is the user-facing message returned when pack_id / pack_meta
// look invalid.
const packValueHint = "pack_id and pack_meta must come from a previous log query result. " +
	"First use sls_execute_sql or sls_query_logstore to query logs, " +
	"appending |with_pack_meta at the end of the query statement. " +
	"The results will contain __pack_id__ and __pack_meta__ fields. " +
	"Pass those field values to this tool."

func (h *slsHandler) handleGetContextLogs(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	project := paramString(params, "project", "")
	logStore := paramString(params, "logStore", "")
	packID := paramString(params, "pack_id", "")
	packMeta := paramString(params, "pack_meta", "")
	regionID := paramString(params, "regionId", "")
	backLines := paramInt(params, "back_lines", 10)
	forwardLines := paramInt(params, "forward_lines", 10)

	if project == "" || logStore == "" || packID == "" || packMeta == "" || regionID == "" {
		return buildResponse(nil, true, "project, logStore, pack_id, pack_meta and regionId are required"), nil
	}

	// Validate pack_id and pack_meta — reject obviously placeholder values.
	if isInvalidPackValue(packID) || isInvalidPackValue(packMeta) {
		return buildResponse(nil, true, fmt.Sprintf(
			"Invalid pack_id or pack_meta value (current: pack_id=%q, pack_meta=%q). %s",
			packID, packMeta, packValueHint)), nil
	}

	if backLines == 0 && forwardLines == 0 {
		return buildResponse(nil, true, "back_lines and forward_lines cannot both be 0; at least one must be greater than 0"), nil
	}

	slog.InfoContext(ctx, "sls_get_context_logs",
		"project", project, "logStore", logStore, "region", regionID,
		"pack_id", packID, "back_lines", backLines, "forward_lines", forwardLines)

	// Use the dedicated GetContextLogs API (matching Python implementation)
	result, err := h.slsClient.GetContextLogs(ctx, regionID, project, logStore, packID, packMeta, backLines, forwardLines)
	if err != nil {
		slog.ErrorContext(ctx, "sls_get_context_logs failed", "error", err)
		return buildResponse(nil, true, fmt.Sprintf(
			"Get context logs failed: %s\n\nHint: %s", err, packValueHint)), nil
	}

	return buildResponse(result, false, ""), nil
}

// ===========================================================================
// Tool 12: sls_text_to_spl
// ===========================================================================

func (h *slsHandler) textToSPLTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "sls_text_to_spl",
		Description: `Convert natural language to an SLS SPL query statement.

## Overview

Converts a natural language description into a valid SLS SPL query statement.
Note: SPL (Search Processing Language) is a pipeline-style query language in SLS, primarily used for data transformation, filtering, and extraction, distinct from standard SQL.

## Use Cases

- When the user needs to perform structured extraction from log data
- When fields need to be parsed from unstructured text

## Notes

- This tool executes the generated SPL against the provided data_sample and returns the results
- The response includes the generated query and execution results (data) based on the sample data`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text": map[string]interface{}{
					"type":        "string",
					"description": "Natural language description for SPL generation",
				},
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS project name",
				},
				"logStore": map[string]interface{}{
					"type":        "string",
					"description": "SLS logstore name",
				},
				"data_sample": map[string]interface{}{
					"type":        "array",
					"description": "Sample log data for SPL generation",
					"items": map[string]interface{}{
						"type": "object",
					},
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID, e.g. 'cn-hongkong'",
				},
			},
			"required": []string{"text", "project", "logStore", "data_sample", "regionId"},
		},
		Handler: h.handleTextToSPL,
	}
}

func (h *slsHandler) handleTextToSPL(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	text := paramString(params, "text", "")
	project := paramString(params, "project", "")
	logStore := paramString(params, "logStore", "")
	regionID := paramString(params, "regionId", "")

	if text == "" || project == "" || logStore == "" || regionID == "" {
		return buildResponse(nil, true, "text, project, logStore and regionId are required"), nil
	}

	slog.InfoContext(ctx, "sls_text_to_spl",
		"project", project, "logStore", logStore, "region", regionID)

	// Use CMS Chat API with "spl_intent_recognition" skill for SPL generation
	spl, err := h.cmsClient.ChatWithSkill(ctx, regionID, project, logStore, text, "spl_intent_recognition")
	if err != nil {
		slog.ErrorContext(ctx, "sls_text_to_spl failed", "error", err)
		return buildResponse(nil, true, fmt.Sprintf("Text to SPL failed: %s", err)), nil
	}

	return buildResponse(map[string]interface{}{
		"query": spl,
	}, false, ""), nil
}

// ===========================================================================
// Tool 13: sls_log_explore
// ===========================================================================

func (h *slsHandler) logExploreTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "sls_log_explore",
		Description: `View aggregated analysis results of log data in an Alibaba Cloud Log Service logstore, providing a log data overview.

## Overview

Provides an overview of log data in a specified logstore, including typical log patterns and the distribution of log counts per pattern.

## Use Cases

- When you need to view log overview information and data distribution in a logstore

## Examples

- "Query the log data distribution in a specific project's logstore"
- "What are the logs at different severity levels in a specific project's logstore"`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS project name, must be an exact match, cannot contain Chinese characters",
				},
				"logStore": map[string]interface{}{
					"type":        "string",
					"description": "SLS logstore name, must be an exact match, cannot contain Chinese characters",
				},
				"from_time": map[string]interface{}{
					"type":        "string",
					"description": "Start time: Unix timestamp (seconds/milliseconds) or relative expression (now-5m)",
					"default":     "now-1h",
				},
				"to_time": map[string]interface{}{
					"type":        "string",
					"description": "End time: Unix timestamp (seconds/milliseconds) or relative expression (now)",
					"default":     "now",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID",
				},
				"logField": map[string]interface{}{
					"type":        "string",
					"description": "Field name containing the log message",
				},
				"filter_query": map[string]interface{}{
					"type":        "string",
					"description": "Filter query statement, must be a valid SLS query, used to filter log data",
				},
				"groupField": map[string]interface{}{
					"type":        "string",
					"description": "Field name containing the log message group identifier",
				},
			},
			"required": []string{"project", "logStore", "regionId", "logField"},
		},
		Handler: h.handleLogExplore,
	}
}

func (h *slsHandler) handleLogExplore(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	project := paramString(params, "project", "")
	logStore := paramString(params, "logStore", "")
	logField := paramString(params, "logField", "")
	filterQuery := paramString(params, "filter_query", "")
	groupField := paramString(params, "groupField", "")
	regionID := paramString(params, "regionId", "")

	if project == "" || logStore == "" || regionID == "" || logField == "" {
		return buildResponse(nil, true, "project, logStore, regionId and logField are required"), nil
	}

	fromTS, err := parseTimeParam(params, "from_time", "now-1h")
	if err != nil {
		return buildResponse(nil, true, fmt.Sprintf("invalid from_time: %s", err)), nil
	}
	toTS, err := parseTimeParam(params, "to_time", "now")
	if err != nil {
		return buildResponse(nil, true, fmt.Sprintf("invalid to_time: %s", err)), nil
	}

	slog.InfoContext(ctx, "sls_log_explore",
		"project", project, "logStore", logStore, "region", regionID,
		"logField", logField, "groupField", groupField,
		"from", fromTS, "to", toTS)

	// Build filter query
	baseQuery := "*"
	if filterQuery != "" {
		baseQuery = filterQuery
	}

	// Step 1: Get total count to determine sampling rate
	countQuery := fmt.Sprintf("%s | select count(1) as cnt", baseQuery)
	countResult, err := h.slsClient.Query(ctx, regionID, project, logStore, countQuery, fromTS, toTS)
	if err != nil {
		slog.ErrorContext(ctx, "sls_log_explore count query failed", "error", err)
		return map[string]interface{}{
			"patterns": []interface{}{},
			"message":  fmt.Sprintf("Failed to count logs: %s", err),
		}, nil
	}

	totalCount := extractCount(countResult)
	if totalCount == 0 {
		return map[string]interface{}{
			"patterns": []interface{}{},
			"message":  fmt.Sprintf("Failed to do log explore because no log data found in the specified time range (%s ~ %s)", paramString(params, "from_time", "now-1h"), paramString(params, "to_time", "now")),
		}, nil
	}

	// Calculate sampling rate (target ~50000 logs for pattern generation)
	samplingRate := int(50000 * 100 / totalCount)
	if samplingRate < 1 {
		samplingRate = 1
	}
	if samplingRate > 100 {
		samplingRate = 100
	}

	// Step 2: Generate log patterns using SPL
	var splQuery string
	if groupField == "" {
		splQuery = fmt.Sprintf(`%s and "%s": *
    | sample -method='bernoulli' %d
    | where "%s" != '' and "%s" is not null
    | stats content_arr = array_agg("%s")
    | extend ret = get_log_patterns(content_arr, ARRAY[',', ' ', '''', '"', ';', '!', '=', '(', ')', '[', ']', '{', '}', '?', ':', '', '\t', '\n'], cast(null as array(varchar)), cast(null as array(varchar)), '{"threshold": 3, "tolerance": 0.1, "maxDigitRatio": 0.1}')
    | extend model_id = ret.model_id, error_msg = ret.error_msg
    | project model_id, error_msg
    | limit 50000`, baseQuery, logField, samplingRate, logField, logField, logField)
	} else {
		splQuery = fmt.Sprintf(`%s and "%s": *
    | sample -method='bernoulli' %d
    | where "%s" != '' and "%s" is not null
    | extend label_concat = coalesce(cast("%s" as varchar), '')
    | stats content_arr = array_agg("%s"), label_arr = array_agg(label_concat)
    | extend ret = get_log_patterns(content_arr, ARRAY[',', ' ', '''', '"', ';', '!', '=', '(', ')', '[', ']', '{', '}', '?', ':', '', '\t', '\n'], cast(null as array(varchar)), label_arr, '{"threshold": 3, "tolerance": 0.1, "maxDigitRatio": 0.1}')
    | extend model_id = ret.model_id, error_msg = ret.error_msg
    | project model_id, error_msg
    | limit 50000`, baseQuery, logField, samplingRate, logField, logField, groupField, logField)
	}

	modelResult, err := h.slsClient.Query(ctx, regionID, project, logStore, splQuery, fromTS, toTS)
	if err != nil {
		slog.ErrorContext(ctx, "sls_log_explore pattern generation failed", "error", err)
		return map[string]interface{}{
			"patterns": []interface{}{},
			"message":  "Failed to do log explore because pattern model creation failed",
		}, nil
	}

	modelID := extractModelID(modelResult)
	if modelID == "" {
		errorMsg := extractErrorMsg(modelResult)
		return map[string]interface{}{
			"patterns": []interface{}{},
			"message":  fmt.Sprintf("Failed to do log explore because pattern model creation failed: %s", errorMsg),
		}, nil
	}

	// Step 3: Match logs against patterns
	matchSamplingRate := int(200000 * 100 / totalCount)
	if matchSamplingRate < 1 {
		matchSamplingRate = 1
	}
	if matchSamplingRate > 100 {
		matchSamplingRate = 100
	}

	timeBucketSize := (toTS - fromTS) / 10
	if timeBucketSize < 1 {
		timeBucketSize = 1
	}

	var matchQuery string
	if groupField == "" {
		matchQuery = fmt.Sprintf(`%s and "%s": *
| sample -method='bernoulli' %d
| where "%s" != '' and "%s" is not null
| extend ret = match_log_patterns('%s', "%s")
| extend is_matched = ret.is_matched, pattern_id = ret.pattern_id, pattern = ret.pattern, pattern_regexp = ret.regexp, variables = ret.variables, time_bucket_id = __time__ - __time__ %% %d
| stats pattern = arbitrary(pattern), pattern_regexp = arbitrary(pattern_regexp), variables_arr = array_agg(variables), earliest_ts = min(__time__), latest_ts = max(__time__), event_num = count(1), hist = histogram(time_bucket_id), data_sample = arbitrary("%s") by pattern_id
| extend var_summary = summary_log_variables(variables_arr, '{"topk": 10}')
| project pattern_id, pattern, pattern_regexp, var_summary, earliest_ts, latest_ts, event_num, hist, data_sample
| sort event_num desc
| limit 200000`, baseQuery, logField, matchSamplingRate, logField, logField, modelID, logField, timeBucketSize, logField)
	} else {
		matchQuery = fmt.Sprintf(`%s and "%s": *
| sample -method='bernoulli' %d
| where "%s" != '' and "%s" is not null
| extend label_concat = coalesce(cast("%s" as varchar), '')
| extend ret = match_log_patterns('%s', "%s", label_concat)
| extend is_matched = ret.is_matched, pattern_id = ret.pattern_id, pattern = ret.pattern, pattern_regexp = ret.regexp, variables = ret.variables, time_bucket_id = __time__ - __time__ %% %d
| stats pattern = arbitrary(pattern), pattern_regexp = arbitrary(pattern_regexp), variables_arr = array_agg(variables), earliest_ts = min(__time__), latest_ts = max(__time__), event_num = count(1), hist = histogram(time_bucket_id), data_sample = arbitrary("%s") by pattern_id, label_concat
| extend var_summary = summary_log_variables(variables_arr, '{"topk": 10}')
| project pattern_id, pattern, label_concat, pattern_regexp, var_summary, earliest_ts, latest_ts, event_num, hist, data_sample
| sort event_num desc
| limit 200000`, baseQuery, logField, matchSamplingRate, logField, logField, groupField, modelID, logField, timeBucketSize, logField)
	}

	matchResult, err := h.slsClient.Query(ctx, regionID, project, logStore, matchQuery, fromTS, toTS)
	if err != nil {
		slog.ErrorContext(ctx, "sls_log_explore pattern matching failed", "error", err)
		return map[string]interface{}{
			"patterns": []interface{}{},
			"message":  "Failed to do log explore because match log data failed",
		}, nil
	}

	// Step 4: Format results
	patterns := formatLogExploreResults(matchResult, timeBucketSize)

	slog.InfoContext(ctx, "sls_log_explore completed",
		"project", project, "logStore", logStore,
		"patterns_found", len(patterns))

	return map[string]interface{}{
		"patterns": patterns,
		"message":  "success",
	}, nil
}

// LogPattern represents a discovered log pattern
type LogPattern struct {
	Pattern    string   `json:"pattern"`
	Count      int      `json:"count"`
	Percentage float64  `json:"percentage"`
	Examples   []string `json:"examples"`
}

// LogExploreResult contains the results of log exploration analysis
type LogExploreResult struct {
	Patterns       []LogPattern   `json:"patterns"`
	TotalLogs      int            `json:"total_logs"`
	UniquePatterns int            `json:"unique_patterns"`
	Distribution   map[string]int `json:"distribution"`
}

// extractCount extracts count from query result
func extractCount(result []map[string]interface{}) int64 {
	if len(result) == 0 {
		return 0
	}
	if cnt, ok := result[0]["cnt"]; ok {
		switch v := cnt.(type) {
		case float64:
			return int64(v)
		case int64:
			return v
		case int:
			return int64(v)
		case string:
			var n int64
			_, _ = fmt.Sscanf(v, "%d", &n)
			return n
		}
	}
	return 0
}

// extractModelID extracts model_id from query result.
func extractModelID(result []map[string]interface{}) string {
	if len(result) == 0 {
		return ""
	}
	if modelID, ok := result[0]["model_id"].(string); ok {
		return modelID
	}
	return ""
}

// extractErrorMsg extracts error_msg from query result.
func extractErrorMsg(result []map[string]interface{}) string {
	if len(result) == 0 {
		return ""
	}
	if errMsg, ok := result[0]["error_msg"].(string); ok {
		return errMsg
	}
	return ""
}

// formatLogExploreResults formats the log explore query results.
func formatLogExploreResults(result []map[string]interface{}, timeBucketSize int64) []map[string]interface{} {
	patterns := make([]map[string]interface{}, 0)

	for _, row := range result {

		pattern := make(map[string]interface{})
		pattern["pattern"] = row["pattern"]
		pattern["pattern_regexp"] = row["pattern_regexp"]
		pattern["event_num"] = row["event_num"]
		if group, ok := row["label_concat"]; ok {
			pattern["group"] = group
		}

		// Parse histogram if present
		if histStr, ok := row["hist"].(string); ok && histStr != "" {
			var hist map[string]interface{}
			if err := json.Unmarshal([]byte(histStr), &hist); err == nil {
				histogram := make([]map[string]interface{}, 0)
				for ts, count := range hist {
					var tsInt int64
					_, _ = fmt.Sscanf(ts, "%d", &tsInt)
					histogram = append(histogram, map[string]interface{}{
						"from_timestamp": tsInt,
						"to_timestamp":   tsInt + timeBucketSize,
						"count":          count,
					})
				}
				pattern["histogram"] = histogram
			}
		}

		// Parse variable summary if present
		if varSummaryStr, ok := row["var_summary"].(string); ok && varSummaryStr != "" {
			var varSummary []interface{}
			if err := json.Unmarshal([]byte(varSummaryStr), &varSummary); err == nil && len(varSummary) >= 4 {
				variables := make([]map[string]interface{}, 0)
				varCandidates, _ := varSummary[0].([]interface{})
				varCandidatesCount, _ := varSummary[1].([]interface{})
				varCandidatesType, _ := varSummary[2].([]interface{})
				varCandidatesFormat, _ := varSummary[3].([]interface{})

				for i := 0; i < len(varCandidatesType); i++ {
					varInfo := map[string]interface{}{
						"index":  i,
						"type":   varCandidatesType[i],
						"format": varCandidatesFormat[i],
					}
					candidates := make(map[string]interface{})
					if i < len(varCandidates) && i < len(varCandidatesCount) {
						candList, _ := varCandidates[i].([]interface{})
						countList, _ := varCandidatesCount[i].([]interface{})
						for j := 0; j < len(candList) && j < len(countList); j++ {
							if candStr, ok := candList[j].(string); ok {
								candidates[candStr] = countList[j]
							}
						}
					}
					varInfo["candidates"] = candidates
					variables = append(variables, varInfo)
				}
				pattern["variables"] = variables
			}
		}

		patterns = append(patterns, pattern)
	}

	return patterns
}

// formatLogCompareResults formats the log compare query results.
func formatLogCompareResults(result []map[string]interface{}) []map[string]interface{} {
	// Group results by (group, pattern) key
	type pairKey struct {
		group   string
		pattern string
	}
	pairs := make(map[pairKey]map[string]map[string]interface{})

	for _, row := range result {

		// Format individual item
		formattedItem := map[string]interface{}{
			"pattern":         row["pattern"],
			"pattern_regexp":  row["pattern_regexp"],
			"event_num":       row["event_num"],
			"group":           row["label_concat"],
			"test_or_control": row["group_id"],
		}

		// Parse variable summary if present
		if varSummaryStr, ok := row["var_summary"].(string); ok && varSummaryStr != "" {
			var varSummary []interface{}
			if err := json.Unmarshal([]byte(varSummaryStr), &varSummary); err == nil && len(varSummary) >= 4 {
				variables := make([]map[string]interface{}, 0)
				varCandidates, _ := varSummary[0].([]interface{})
				varCandidatesCount, _ := varSummary[1].([]interface{})
				varCandidatesType, _ := varSummary[2].([]interface{})
				varCandidatesFormat, _ := varSummary[3].([]interface{})

				for i := 0; i < len(varCandidatesType); i++ {
					varInfo := map[string]interface{}{
						"index":  i,
						"type":   varCandidatesType[i],
						"format": varCandidatesFormat[i],
					}
					candidates := make(map[string]interface{})
					if i < len(varCandidates) && i < len(varCandidatesCount) {
						candList, _ := varCandidates[i].([]interface{})
						countList, _ := varCandidatesCount[i].([]interface{})
						for j := 0; j < len(candList) && j < len(countList); j++ {
							if candStr, ok := candList[j].(string); ok {
								candidates[candStr] = countList[j]
							}
						}
					}
					varInfo["candidates"] = candidates
					variables = append(variables, varInfo)
				}
				formattedItem["variables"] = variables
			}
		}

		// Get key values
		groupVal := ""
		if g, ok := formattedItem["group"].(string); ok {
			groupVal = g
		}
		patternVal := ""
		if p, ok := formattedItem["pattern"].(string); ok {
			patternVal = p
		}
		testOrControl := ""
		if t, ok := formattedItem["test_or_control"].(string); ok {
			testOrControl = t
		}

		key := pairKey{group: groupVal, pattern: patternVal}
		if _, exists := pairs[key]; !exists {
			pairs[key] = make(map[string]map[string]interface{})
		}
		pairs[key][testOrControl] = formattedItem
	}

	// Merge test and control items
	results := make([]map[string]interface{}, 0)
	for _, pair := range pairs {
		test := pair["test"]
		control := pair["control"]

		if test == nil && control == nil {
			continue
		}

		merged := make(map[string]interface{})
		if control != nil {
			merged["pattern"] = control["pattern"]
			merged["pattern_regexp"] = control["pattern_regexp"]
			merged["group"] = control["group"]
		} else {
			merged["pattern"] = test["pattern"]
			merged["pattern_regexp"] = test["pattern_regexp"]
			merged["group"] = test["group"]
		}

		if test != nil {
			merged["test_event_num"] = test["event_num"]
			merged["test_variables"] = test["variables"]
		} else {
			merged["test_event_num"] = 0
			merged["test_variables"] = []interface{}{}
		}

		if control != nil {
			merged["control_event_num"] = control["event_num"]
			merged["control_variables"] = control["variables"]
		} else {
			merged["control_event_num"] = 0
			merged["control_variables"] = []interface{}{}
		}

		results = append(results, merged)
	}

	return results
}

// ===========================================================================
// Tool 14: sls_log_compare
// ===========================================================================

func (h *slsHandler) logCompareTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "sls_log_compare",
		Description: `View comparison results of log data between two time ranges in an Alibaba Cloud Log Service logstore.

## Overview

Compares the log data distribution between two time ranges, enabling quick analysis of log data changes.

## Use Cases

- When you need to analyze differences in log data distribution between two time ranges

## Examples

- "What are the differences between yesterday's logs and today's"
- "What changed in the logs before and after a service deployment"`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS project name, must be an exact match, cannot contain Chinese characters",
				},
				"logStore": map[string]interface{}{
					"type":        "string",
					"description": "SLS logstore name, must be an exact match, cannot contain Chinese characters",
				},
				"test_from_time": map[string]interface{}{
					"type":        "string",
					"description": "Test group start time: Unix timestamp (seconds/milliseconds) or relative expression (now-5m)",
					"default":     "now-1h",
				},
				"test_to_time": map[string]interface{}{
					"type":        "string",
					"description": "Test group end time: Unix timestamp (seconds/milliseconds) or relative expression (now)",
					"default":     "now",
				},
				"control_from_time": map[string]interface{}{
					"type":        "string",
					"description": "Control group start time: Unix timestamp (seconds/milliseconds) or relative expression (now-5m)",
					"default":     "now-1h",
				},
				"control_to_time": map[string]interface{}{
					"type":        "string",
					"description": "Control group end time: Unix timestamp (seconds/milliseconds) or relative expression (now)",
					"default":     "now",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID",
				},
				"logField": map[string]interface{}{
					"type":        "string",
					"description": "Field name containing the log message",
				},
				"filter_query": map[string]interface{}{
					"type":        "string",
					"description": "Filter query statement, must be a valid SLS query, used to filter log data",
				},
				"groupField": map[string]interface{}{
					"type":        "string",
					"description": "Field name containing the log message group identifier",
				},
			},
			"required": []string{"project", "logStore", "regionId", "logField"},
		},
		Handler: h.handleLogCompare,
	}
}

func (h *slsHandler) handleLogCompare(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	project := paramString(params, "project", "")
	logStore := paramString(params, "logStore", "")
	logField := paramString(params, "logField", "")
	filterQuery := paramString(params, "filter_query", "")
	groupField := paramString(params, "groupField", "")
	regionID := paramString(params, "regionId", "")

	if project == "" || logStore == "" || regionID == "" || logField == "" {
		return buildResponse(nil, true, "project, logStore, regionId and logField are required"), nil
	}

	// 使用同一个 now 时刻解析所有时间参数，避免多次 time.Now() 调用之间的
	// 微小时间差导致相邻时间范围（如 test=now-1h~now, control=now-2h~now-1h）
	// 被误判为重叠。
	now := time.Now()

	// Parse test time range
	testFromTS, err := parseTimeParamAt(params, "test_from_time", "now-1h", now)
	if err != nil {
		return buildResponse(nil, true, fmt.Sprintf("invalid test_from_time: %s", err)), nil
	}
	testToTS, err := parseTimeParamAt(params, "test_to_time", "now", now)
	if err != nil {
		return buildResponse(nil, true, fmt.Sprintf("invalid test_to_time: %s", err)), nil
	}

	// Parse control time range
	controlFromTS, err := parseTimeParamAt(params, "control_from_time", "now-1h", now)
	if err != nil {
		return buildResponse(nil, true, fmt.Sprintf("invalid control_from_time: %s", err)), nil
	}
	controlToTS, err := parseTimeParamAt(params, "control_to_time", "now", now)
	if err != nil {
		return buildResponse(nil, true, fmt.Sprintf("invalid control_to_time: %s", err)), nil
	}

	// Check for time range overlap (adjacent ranges where one ends exactly where the other begins are allowed)
	if !(testToTS <= controlFromTS || controlToTS <= testFromTS) {
		return map[string]interface{}{
			"patterns": []interface{}{},
			"message":  fmt.Sprintf("Failed to do log compare because test group (%s ~ %s) and control group (%s ~ %s) data time range overlap", paramString(params, "test_from_time", "now-1h"), paramString(params, "test_to_time", "now"), paramString(params, "control_from_time", "now-1h"), paramString(params, "control_to_time", "now")),
		}, nil
	}

	slog.InfoContext(ctx, "sls_log_compare",
		"project", project, "logStore", logStore, "region", regionID,
		"logField", logField, "groupField", groupField,
		"test_from", testFromTS, "test_to", testToTS,
		"control_from", controlFromTS, "control_to", controlToTS)

	// Build filter query
	baseQuery := "*"
	if filterQuery != "" {
		baseQuery = filterQuery
	}

	// Step 1: Get total count to determine sampling rate
	countQuery := fmt.Sprintf("%s | select count(1) as cnt", baseQuery)
	countResult, err := h.slsClient.Query(ctx, regionID, project, logStore, countQuery, testFromTS, testToTS)
	if err != nil {
		slog.ErrorContext(ctx, "sls_log_compare count query failed", "error", err)
		return map[string]interface{}{
			"patterns": []interface{}{},
			"message":  fmt.Sprintf("Failed to count logs: %s", err),
		}, nil
	}

	totalCount := extractCount(countResult)
	if totalCount == 0 {
		return map[string]interface{}{
			"patterns": []interface{}{},
			"message":  fmt.Sprintf("Failed to do log compare because no log data found in the specified time range (%s ~ %s)", paramString(params, "test_from_time", "now-1h"), paramString(params, "test_to_time", "now")),
		}, nil
	}

	// Calculate sampling rate (target ~50000 logs for pattern generation)
	samplingRate := int(50000 * 100 / totalCount)
	if samplingRate < 1 {
		samplingRate = 1
	}
	if samplingRate > 100 {
		samplingRate = 100
	}

	// Step 2: Generate log patterns for test group
	var splQuery string
	if groupField == "" {
		splQuery = fmt.Sprintf(`%s and "%s": *
    | sample -method='bernoulli' %d
    | where "%s" != '' and "%s" is not null
    | stats content_arr = array_agg("%s")
    | extend ret = get_log_patterns(content_arr, ARRAY[',', ' ', '''', '"', ';', '!', '=', '(', ')', '[', ']', '{', '}', '?', ':', '', '\t', '\n'], cast(null as array(varchar)), cast(null as array(varchar)), '{"threshold": 3, "tolerance": 0.1, "maxDigitRatio": 0.1}')
    | extend model_id = ret.model_id, error_msg = ret.error_msg
    | project model_id, error_msg
    | limit 50000`, baseQuery, logField, samplingRate, logField, logField, logField)
	} else {
		splQuery = fmt.Sprintf(`%s and "%s": *
    | sample -method='bernoulli' %d
    | where "%s" != '' and "%s" is not null
    | extend label_concat = coalesce(cast("%s" as varchar), '')
    | stats content_arr = array_agg("%s"), label_arr = array_agg(label_concat)
    | extend ret = get_log_patterns(content_arr, ARRAY[',', ' ', '''', '"', ';', '!', '=', '(', ')', '[', ']', '{', '}', '?', ':', '', '\t', '\n'], cast(null as array(varchar)), label_arr, '{"threshold": 3, "tolerance": 0.1, "maxDigitRatio": 0.1}')
    | extend model_id = ret.model_id, error_msg = ret.error_msg
    | project model_id, error_msg
    | limit 50000`, baseQuery, logField, samplingRate, logField, logField, groupField, logField)
	}

	testModelResult, err := h.slsClient.Query(ctx, regionID, project, logStore, splQuery, testFromTS, testToTS)
	if err != nil {
		slog.ErrorContext(ctx, "sls_log_compare test pattern generation failed", "error", err)
		return map[string]interface{}{
			"patterns": []interface{}{},
			"message":  "Failed to do log compare because test group pattern model creation failed",
		}, nil
	}

	testModelID := extractModelID(testModelResult)
	if testModelID == "" {
		errorMsg := extractErrorMsg(testModelResult)
		return map[string]interface{}{
			"patterns": []interface{}{},
			"message":  fmt.Sprintf("Failed to do log compare because test group pattern model creation failed: %s", errorMsg),
		}, nil
	}

	// Step 3: Generate log patterns for control group
	controlModelResult, err := h.slsClient.Query(ctx, regionID, project, logStore, splQuery, controlFromTS, controlToTS)
	if err != nil {
		slog.ErrorContext(ctx, "sls_log_compare control pattern generation failed", "error", err)
		return map[string]interface{}{
			"patterns": []interface{}{},
			"message":  "Failed to do log compare because control group pattern model creation failed",
		}, nil
	}

	controlModelID := extractModelID(controlModelResult)
	if controlModelID == "" {
		errorMsg := extractErrorMsg(controlModelResult)
		return map[string]interface{}{
			"patterns": []interface{}{},
			"message":  fmt.Sprintf("Failed to do log compare because control group pattern model creation failed: %s", errorMsg),
		}, nil
	}

	// Step 4: Merge test and control group models
	mergeQuery := fmt.Sprintf(`%s and "%s": *
| where "%s" != '' and "%s" is not null
| extend ret = merge_log_patterns('%s', '%s')
| limit 1
| extend model_id = ret.model_id, error_msg = ret.error_msg
| project model_id, error_msg`, baseQuery, logField, logField, logField, testModelID, controlModelID)

	mergeResult, err := h.slsClient.Query(ctx, regionID, project, logStore, mergeQuery, testFromTS, testToTS)
	if err != nil {
		slog.ErrorContext(ctx, "sls_log_compare merge models failed", "error", err)
		return map[string]interface{}{
			"patterns": []interface{}{},
			"message":  "Failed to do log compare because merge test and control group model failed",
		}, nil
	}

	modelID := extractModelID(mergeResult)
	if modelID == "" {
		errorMsg := extractErrorMsg(mergeResult)
		return map[string]interface{}{
			"patterns": []interface{}{},
			"message":  fmt.Sprintf("Failed to do log compare because merge test and control group model failed: %s", errorMsg),
		}, nil
	}

	// Step 5: Match logs against merged patterns
	matchSamplingRate := int(200000 * 100 / totalCount)
	if matchSamplingRate < 1 {
		matchSamplingRate = 1
	}
	if matchSamplingRate > 100 {
		matchSamplingRate = 100
	}

	fromTimestamp := testFromTS
	if controlFromTS < fromTimestamp {
		fromTimestamp = controlFromTS
	}
	toTimestamp := testToTS
	if controlToTS > toTimestamp {
		toTimestamp = controlToTS
	}

	var matchQuery string
	if groupField == "" {
		matchQuery = fmt.Sprintf(`%s and "%s": * 
| extend group_id = if(__time__ >= %d and __time__ < %d, 'test', if(__time__ >= %d and __time__ < %d, 'control', 'null'))
| where group_id != 'null'
| sample -method='bernoulli' %d
| where "%s" != '' and "%s" is not null
| extend ret = match_log_patterns('%s', "%s")
| extend is_matched = ret.is_matched, pattern_id = ret.pattern_id, pattern = ret.pattern, pattern_regexp = ret.regexp, variables = ret.variables
| stats pattern = arbitrary(pattern), pattern_regexp = arbitrary(pattern_regexp), variables_arr = array_agg(variables), earliest_ts = min(__time__), latest_ts = max(__time__), event_num = count(1), data_sample = arbitrary("%s") by pattern_id, group_id
| extend var_summary = summary_log_variables(variables_arr, '{"topk": 10}')
| project pattern_id, pattern, pattern_regexp, var_summary, earliest_ts, latest_ts, event_num, data_sample, group_id
| sort event_num desc
| limit 200000`, baseQuery, logField, testFromTS, testToTS, controlFromTS, controlToTS, matchSamplingRate, logField, logField, modelID, logField, logField)
	} else {
		matchQuery = fmt.Sprintf(`%s and "%s": * and "%s": *
| extend group_id = if(__time__ >= %d and __time__ < %d, 'test', if(__time__ >= %d and __time__ < %d, 'control', 'null'))
| where group_id != 'null'
| sample -method='bernoulli' %d
| where "%s" != '' and "%s" is not null
| extend label_concat = coalesce(cast("%s" as varchar), '')
| extend ret = match_log_patterns('%s', "%s", label_concat)
| extend is_matched = ret.is_matched, pattern_id = ret.pattern_id, pattern = ret.pattern, pattern_regexp = ret.regexp, variables = ret.variables
| stats pattern = arbitrary(pattern), pattern_regexp = arbitrary(pattern_regexp), variables_arr = array_agg(variables), earliest_ts = min(__time__), latest_ts = max(__time__), event_num = count(1), data_sample = arbitrary("%s") by pattern_id, label_concat, group_id
| extend var_summary = summary_log_variables(variables_arr, '{"topk": 10}')
| project pattern_id, pattern, label_concat, pattern_regexp, var_summary, earliest_ts, latest_ts, event_num, data_sample, group_id
| sort event_num desc
| limit 200000`, baseQuery, logField, groupField, testFromTS, testToTS, controlFromTS, controlToTS, matchSamplingRate, logField, logField, groupField, modelID, logField, logField)
	}

	matchResult, err := h.slsClient.Query(ctx, regionID, project, logStore, matchQuery, fromTimestamp, toTimestamp)
	if err != nil {
		slog.ErrorContext(ctx, "sls_log_compare pattern matching failed", "error", err)
		return map[string]interface{}{
			"patterns": []interface{}{},
			"message":  "Failed to do log compare because match log data failed",
		}, nil
	}

	// Step 6: Format and merge results
	patterns := formatLogCompareResults(matchResult)

	slog.InfoContext(ctx, "sls_log_compare completed",
		"project", project, "logStore", logStore,
		"patterns_found", len(patterns))

	return map[string]interface{}{
		"patterns": patterns,
		"message":  "success",
	}, nil
}
