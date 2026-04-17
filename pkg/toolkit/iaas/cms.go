package iaas

import (
	"context"
	"fmt"
	"log/slog"

	sls "github.com/alibabacloud-go/sls-20201230/v6/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/client"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/toolkit"
)

// CMSTools returns all CMS tools backed by the given CMS client.
func CMSTools(cmsClient client.CMSClient, slsClient client.SLSClient) []toolkit.Tool {
	h := &cmsHandler{cmsClient: cmsClient, slsClient: slsClient}
	return []toolkit.Tool{
		h.executePromQLTool(),
		h.textToPromQLTool(),
	}
}

// cmsHandler holds the CMS client and provides tool constructors and handlers.
type cmsHandler struct {
	cmsClient client.CMSClient
	slsClient client.SLSClient
}

// ===========================================================================
// Tool 1: cms_execute_promql
// ===========================================================================

func (h *cmsHandler) executePromQLTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "cms_execute_promql",
		Description: `Execute a PromQL metric query.

## Overview

Execute a PromQL query on a specified SLS project and metric store, returning time-series metric data.

## Use Cases

- Query time-series metric data
- Analyze system performance metrics
- Monitor business metric trends

## Query Syntax

The query must use valid PromQL syntax.

## Time Range

- from_time: Start time, supports Unix timestamp (seconds/milliseconds) or relative time expressions
- to_time: End time, supports Unix timestamp (seconds/milliseconds) or relative time expressions`,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"project": map[string]any{
					"type":        "string",
					"description": "SLS project name",
				},
				"metricStore": map[string]any{
					"type":        "string",
					"description": "SLS metric store name",
				},
				"query": map[string]any{
					"type":        "string",
					"description": "PromQL query statement",
				},
				"from_time": map[string]any{
					"type":        "string",
					"description": "Start time, supports Unix timestamp or relative time e.g. 'now-5m'",
					"default":     "now-5m",
				},
				"to_time": map[string]any{
					"type":        "string",
					"description": "End time, supports Unix timestamp or relative time e.g. 'now'",
					"default":     "now",
				},
				"regionId": map[string]any{
					"type":        "string",
					"description": "Alibaba Cloud region ID, e.g. 'cn-hongkong'",
				},
			},
			"required": []string{"project", "metricStore", "query", "regionId"},
		},
		Handler: h.handleExecutePromQL,
	}
}

func (h *cmsHandler) handleExecutePromQL(ctx context.Context, params map[string]any) (any, error) {
	project := paramString(params, "project", "")
	metricStore := paramString(params, "metricStore", "")
	query := paramString(params, "query", "")
	regionID := paramString(params, "regionId", "")

	if project == "" || metricStore == "" || query == "" || regionID == "" {
		return buildResponse(nil, true, "project, metricStore, query and regionId are required"), nil
	}

	fromTS, err := parseTimeParam(params, "from_time", "now-5m")
	if err != nil {
		return buildResponse(nil, true, fmt.Sprintf("invalid from_time: %s", err)), nil
	}
	toTS, err := parseTimeParam(params, "to_time", "now")
	if err != nil {
		return buildResponse(nil, true, fmt.Sprintf("invalid to_time: %s", err)), nil
	}

	slog.InfoContext(ctx, "cms_execute_promql",
		"project", project, "metricStore", metricStore, "region", regionID,
		"from", fromTS, "to", toTS)

	// Wrap PromQL in SPL template for execution via SLS.
	// The SPL keyword is always ".metricstore" (literal), not the actual store name.
	// The store name is passed as the logstore parameter to the API.
	// Includes .set directives required by the SLS PromQL engine.
	splQuery := fmt.Sprintf(`.set "sql.session.velox_support_row_constructor_enabled" = 'true';
.set "sql.session.presto_velox_mix_run_not_check_linked_agg_enabled" = 'true';
.set "sql.session.presto_velox_mix_run_support_complex_type_enabled" = 'true';
.set "sql.session.velox_sanity_limit_enabled" = 'false';
.metricstore with(promql_query='%s',range='1m')
| extend latest_ts = element_at(__ts__,cardinality(__ts__)),
         latest_val = element_at(__value__,cardinality(__value__))
| stats arr_ts = array_agg(__ts__),
        arr_val = array_agg(__value__),
        title_agg = array_agg(json_format(cast(__labels__ as json))),
        cnt = count(*),
        latest_ts = array_agg(latest_ts),
        latest_val = array_agg(latest_val)
| project title_agg, cnt, latest_ts, latest_val`, query)

	results, err := h.slsClient.Query(ctx, regionID, project, metricStore, &sls.GetLogsRequest{
		Query: tea.String(splQuery),
		From:  tea.Int32(int32(fromTS)),
		To:    tea.Int32(int32(toTS)),
	})
	if err != nil {
		slog.ErrorContext(ctx, "cms_execute_promql failed", "error", err)
		if isMetricStoreNotFoundError(err) {
			msg := fmt.Sprintf(
				"Metric store '%s' does not exist in project '%s'. "+
					"Please check available metric stores.",
				metricStore, project,
			)
			return buildResponse(nil, true, msg), nil
		}
		return buildResponse(nil, true, fmt.Sprintf("PromQL query failed: %s", err)), nil
	}

	return buildResponse(map[string]any{
		"data":  results,
		"query": splQuery,
	}, false, ""), nil
}

// ===========================================================================
// Tool 2: cms_text_to_promql
// ===========================================================================

func (h *cmsHandler) textToPromQLTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "cms_text_to_promql",
		Description: `Convert natural language to PromQL query statements.

## Overview

This tool converts natural language descriptions into valid PromQL query statements,
allowing users to express query requirements in plain language.

## Use Cases

- When users are unfamiliar with PromQL query syntax
- When quickly building complex queries
- When extracting query intent from natural language descriptions

## Limitations

- Only supports generating PromQL queries
- Generates query statements, not query results

## Best Practices

- Provide clear and concise natural language descriptions
- Do not include project or metric store names in the description
- The first generated query may not fully meet requirements; multiple attempts may be needed

## Query Examples

- "Generate a PromQL query for XXX"
- "Query the number of Pods per namespace"`,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{
					"type":        "string",
					"description": "Natural language text for generating PromQL queries",
				},
				"project": map[string]any{
					"type":        "string",
					"description": "SLS project name",
				},
				"metricStore": map[string]any{
					"type":        "string",
					"description": "SLS metric store name",
				},
				"regionId": map[string]any{
					"type":        "string",
					"description": "Alibaba Cloud region ID, e.g. 'cn-hongkong'",
				},
			},
			"required": []string{"text", "project", "metricStore", "regionId"},
		},
		Handler: h.handleTextToPromQL,
	}
}

func (h *cmsHandler) handleTextToPromQL(ctx context.Context, params map[string]any) (any, error) {
	text := paramString(params, "text", "")
	project := paramString(params, "project", "")
	metricStore := paramString(params, "metricStore", "")
	regionID := paramString(params, "regionId", "")

	if text == "" || project == "" || metricStore == "" || regionID == "" {
		return buildResponse(nil, true, "text, project, metricStore and regionId are required"), nil
	}

	slog.InfoContext(ctx, "cms_text_to_promql",
		"project", project, "metricStore", metricStore, "region", regionID)

	// Uses SLS CallAiTools API with text_to_promql tool
	promql, err := h.slsClient.TextToPromQL(ctx, regionID, project, metricStore, text)
	if err != nil {
		slog.ErrorContext(ctx, "cms_text_to_promql failed", "error", err)
		return buildResponse(nil, true, fmt.Sprintf("Text to PromQL failed: %s", err)), nil
	}

	return buildResponse(map[string]interface{}{
		"query": promql,
	}, false, ""), nil
}
