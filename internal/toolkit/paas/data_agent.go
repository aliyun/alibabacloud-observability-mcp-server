package paas

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/alibabacloud-observability-mcp-server-go/internal/client"
	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/timeparse"
)

// DataAgentTools returns the natural language data query tool backed by the given CMS client.
func DataAgentTools(cmsClient client.CMSClient) []toolkit.Tool {
	h := &dataAgentHandler{cmsClient: cmsClient}
	return []toolkit.Tool{
		h.dataAgentQueryTool(),
		h.naturalLanguageQueryAliasTool(), // Alias for Python compatibility
	}
}

// naturalLanguageQueryAliasTool returns an alias tool for umodel_data_agent_query
// to maintain compatibility with Python MCP server which uses "cms_natural_language_query"
func (h *dataAgentHandler) naturalLanguageQueryAliasTool() toolkit.Tool {
	tool := h.dataAgentQueryTool()
	tool.Name = "cms_natural_language_query"
	return tool
}

// dataAgentHandler holds the CMS client and provides tool constructors and handlers.
type dataAgentHandler struct {
	cmsClient client.CMSClient
}

// ===========================================================================
// Tool: umodel_data_agent_query
// ===========================================================================

func (h *dataAgentHandler) dataAgentQueryTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "umodel_data_agent_query",
		Description: `使用自然语言查询可观测数据。

## 功能概述

用户可以使用自然语言描述想要查询的数据，系统会自动理解意图并返回相应的数据结果。
通过调用 CMS 的 data-agent skill，实现智能数据查询功能。

## 使用场景

- 当需要快速查询可观测数据但不熟悉具体 API 时
- 当需要进行复杂的数据分析但不想编写复杂查询语句时
- 当需要获取服务性能概览、错误分析、慢请求统计等信息时

## 支持的查询类型

- 指标查询：如"查询服务A的CPU使用率"、"获取内存使用最高的Pod"
- 日志查询：如"统计错误日志数量"、"查找包含Exception的日志"
- 链路查询：如"查询延迟超过1秒的请求"、"获取服务调用拓扑"
- 聚合统计：如"按服务分组统计请求数量"、"计算平均响应时间"

## 返回数据结构

返回的数据包含：
- data: 查询结果数据，包含 query_results（实体列表、指标数据等）
- message: AI 生成的解释说明文本
- error: 是否发生错误（true/false）
- time_range: 查询的时间范围

## 查询示例

- "查询请求量最高的10个服务"
- "哪些服务的错误率超过1%"
- "查询响应时间超过1秒的服务"
- "统计各服务的请求数量"`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "自然语言查询文本，描述你想要查询的可观测数据。示例：'查询请求量最高的10个服务'、'哪些服务的错误率超过1%'、'查询响应时间超过1秒的服务'",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS工作空间名称，可通过 list_workspace 获取",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": timeRangeDescription,
					"default":     "last_15m",
				},
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "可选的实体域上下文，如 'apm'、'infrastructure'，用于缩小查询范围",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "可选的实体类型上下文，如 'apm.service'，用于缩小查询范围",
				},
				"entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "可选的逗号分隔实体ID列表，用于限定查询的实体范围",
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

	slog.InfoContext(ctx, "umodel_data_agent_query",
		"workspace", workspace,
		"query", query,
		"region", regionID,
		"from", fromTS,
		"to", toTS,
	)

	// Call CMS CreateThread + CreateChatWithSSE API (matching Python implementation)
	result, err := h.cmsClient.DataAgentQuery(ctx, regionID, workspace, query, fromTS, toTS)
	if err != nil {
		slog.ErrorContext(ctx, "umodel_data_agent_query failed", "error", err)
		return buildDataAgentErrorResponse(fmt.Sprintf("查询失败: %s", err), timeRange), nil
	}

	// Build response matching Python format
	resultData := map[string]interface{}{
		"query_results": result.QueryResults,
		"tool_results":  result.ToolResults,
	}
	if result.GeneratedSQL != "" {
		resultData["generated_sql"] = result.GeneratedSQL
	}

	// Try to extract main data from tool results (matching Python logic)
	for _, toolResult := range result.ToolResults {
		if toolResult["result"] == nil {
			continue
		}
		switch r := toolResult["result"].(type) {
		case map[string]interface{}:
			if r["data"] != nil {
				resultData["data"] = r["data"]
			}
		case []interface{}:
			resultData["data"] = r
		}
	}

	message := result.Message
	if message == "" {
		message = "查询完成"
	}

	now := time.Now()
	return map[string]interface{}{
		"data":     resultData,
		"message":  message,
		"trace_id": result.TraceID,
		"error":    false,
		"time_range": map[string]interface{}{
			"from":          fromTS,
			"to":            toTS,
			"from_readable": timeparse.FormatTimestamp(fromTS),
			"to_readable":   timeparse.FormatTimestamp(toTS),
			"expression":    timeRange,
		},
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
