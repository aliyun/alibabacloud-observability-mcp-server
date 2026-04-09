package paas

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/alibabacloud-observability-mcp-server-go/pkg/client"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/timeparse"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/toolkit"
)

// DataAgentTools returns the natural language data query tool backed by the given CMS client.
func DataAgentTools(cmsClient client.CMSClient) []toolkit.Tool {
	h := &dataAgentHandler{cmsClient: cmsClient}
	return []toolkit.Tool{
		h.dataAgentQueryTool(),
	}
}

// dataAgentHandler holds the CMS client and provides tool constructors and handlers.
type dataAgentHandler struct {
	cmsClient client.CMSClient
}

// ===========================================================================
// Tool: cms_natural_language_query
// ===========================================================================

func (h *dataAgentHandler) dataAgentQueryTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "cms_natural_language_query",
		Description: `Query observability data using natural language.

## Overview

Describe the data you want to query in natural language. The system interprets your intent
and returns matching results via the CMS data-agent skill.

## Use Cases

- Quickly query observability data without knowing specific APIs
- Perform complex data analysis without writing query statements
- Retrieve service performance overviews, error analysis, slow request statistics, etc.

## Supported Query Types

- Metrics: e.g. "CPU usage of service A", "Pods with highest memory usage"
- Logs: e.g. "Count of error logs", "Logs containing Exception"
- Traces: e.g. "Requests with latency over 1s", "Service call topology"
- Aggregations: e.g. "Request count grouped by service", "Average response time"

## Response Structure

- data: Query result data, including query_results (entity lists, metric data, etc.)
- message: AI-generated explanation text
- error: Whether an error occurred (true/false)
- time_range: The queried time range

## Query Examples

- "Top 10 services by request volume"
- "Services with error rate above 1%"
- "Services with response time over 1s"
- "Request count per service"`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Natural language query describing the observability data to retrieve. Examples: 'Top 10 services by request volume', 'Services with error rate above 1%', 'Services with response time over 1s'",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS workspace name, obtainable via list_workspace",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID, e.g. 'cn-hongkong'",
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": timeRangeDescription,
					"default":     "last_15m",
				},
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "Optional entity domain filter, e.g. 'apm', 'infrastructure', to narrow the query scope",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "Optional entity type filter, e.g. 'apm.service', to narrow the query scope",
				},
				"entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "Optional comma-separated entity IDs to restrict the query scope",
				},
			},
			"required": []string{"query", "workspace", "regionId"},
		},
		Handler: h.handleDataAgentQuery,
	}
}

func (h *dataAgentHandler) handleDataAgentQuery(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	query := paramString(params, "query", "")
	workspace := paramString(params, "workspace", "")
	regionID := paramString(params, "regionId", "")
	timeRange := paramString(params, "time_range", "last_15m")

	if query == "" {
		return buildDataAgentErrorResponse("query is required", timeRange), nil
	}
	if workspace == "" || regionID == "" {
		return buildDataAgentErrorResponse("workspace and regionId are required", timeRange), nil
	}

	fromTS, toTS, err := parseTimeRange(timeRange)
	if err != nil {
		return buildDataAgentErrorResponse(err.Error(), timeRange), nil
	}

	slog.InfoContext(ctx, "cms_natural_language_query",
		"workspace", workspace,
		"query", query,
		"region", regionID,
		"from", fromTS,
		"to", toTS,
	)

	// Call CMS CreateThread + CreateChatWithSSE API (matching Python implementation)
	result, err := h.cmsClient.DataAgentQuery(ctx, regionID, workspace, query, fromTS, toTS)
	if err != nil {
		slog.ErrorContext(ctx, "cms_natural_language_query failed", "error", err)
		return buildDataAgentErrorResponse(fmt.Sprintf("Query failed: %s", err), timeRange), nil
	}


	now := time.Now()
	return map[string]interface{}{
		"message":  result.Message,
		"trace_id": result.TraceID,
		"error":    false,
		"timestamp": now.Unix(),
	}, nil
}

// buildDataAgentErrorResponse constructs an error response for data-agent queries.
func buildDataAgentErrorResponse(message, timeRange string) map[string]interface{} {
	if timeRange == "" {
		timeRange = "last_15m"
	}
	now := time.Now()
	fromTS := now.Add(-15 * time.Minute).Unix()
	toTS := now.Unix()
	return map[string]interface{}{
		"data":     nil,
		"message":  message,
		"trace_id": "",
		"error":    true,
		"time_range": map[string]interface{}{
			"from":          fromTS,
			"to":            toTS,
			"from_readable": timeparse.FormatTimestamp(fromTS),
			"to_readable":   timeparse.FormatTimestamp(toTS),
			"expression":    timeRange,
		},
		"timestamp": now.Unix(),
	}
}
