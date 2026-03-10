package iaas

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/alibabacloud-observability-mcp-server-go/internal/client"
	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit"
)

// CMSTools returns all CMS tools backed by the given CMS client.
func CMSTools(cmsClient client.CMSClient, slsClient client.SLSClient) []toolkit.Tool {
	h := &cmsHandler{cmsClient: cmsClient, slsClient: slsClient}
	return []toolkit.Tool{
		h.queryMetricTool(),
		h.listMetricsTool(),
		h.listNamespacesTool(),
		h.executePromQLTool(),
		h.cmsExecutePromQLTool(), // Alias for Python compatibility
		h.cmsTextToPromQLTool(),  // Alias for Python compatibility
	}
}

// cmsHandler holds the CMS client and provides tool constructors and handlers.
type cmsHandler struct {
	cmsClient client.CMSClient
	slsClient client.SLSClient
}

// ===========================================================================
// Tool 1: cms_query_metric
// ===========================================================================

func (h *cmsHandler) queryMetricTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "cms_query_metric",
		Description: `查询云监控指标数据。

## 功能概述

该工具用于查询阿里云云监控（CMS）中指定命名空间和指标名称的监控数据。

## 使用场景

- 当需要查询ECS、RDS等云产品的监控指标时
- 当需要分析特定资源的性能数据时
- 当需要获取特定维度的指标数据时

## 参数说明

- namespace: 云产品命名空间，如 'acs_ecs_dashboard'、'acs_rds_dashboard'
- metricName: 指标名称，如 'CPUUtilization'、'memory_usedutilization'
- dimensions: 维度过滤条件，JSON格式，如 '{"instanceId":"i-xxx"}'
- from_time: 开始时间，支持Unix时间戳（秒/毫秒）或相对时间表达式
- to_time: 结束时间，支持Unix时间戳（秒/毫秒）或相对时间表达式`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "云产品命名空间，如 'acs_ecs_dashboard'",
				},
				"metricName": map[string]interface{}{
					"type":        "string",
					"description": "指标名称，如 'CPUUtilization'",
				},
				"dimensions": map[string]interface{}{
					"type":        "string",
					"description": "维度过滤条件，JSON格式，如 '{\"instanceId\":\"i-xxx\"}'",
				},
				"from_time": map[string]interface{}{
					"type":        "string",
					"description": "开始时间，支持Unix时间戳或相对时间如 'now-1h'",
					"default":     "now-1h",
				},
				"to_time": map[string]interface{}{
					"type":        "string",
					"description": "结束时间，支持Unix时间戳或相对时间如 'now'",
					"default":     "now",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
				},
			},
			"required": []string{"namespace", "metricName", "regionId"},
		},
		Handler: h.handleQueryMetric,
	}
}

func (h *cmsHandler) handleQueryMetric(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	namespace := paramString(params, "namespace", "")
	metricName := paramString(params, "metricName", "")
	regionID := paramString(params, "regionId", "")

	if namespace == "" || metricName == "" || regionID == "" {
		return buildResponse(nil, true, "namespace, metricName and regionId are required"), nil
	}

	fromTS, err := parseTimeParam(params, "from_time", "now-1h")
	if err != nil {
		return buildResponse(nil, true, fmt.Sprintf("invalid from_time: %s", err)), nil
	}
	toTS, err := parseTimeParam(params, "to_time", "now")
	if err != nil {
		return buildResponse(nil, true, fmt.Sprintf("invalid to_time: %s", err)), nil
	}

	// Parse dimensions JSON string into map
	dimensions := make(map[string]string)
	dimStr := paramString(params, "dimensions", "")
	if dimStr != "" {
		if err := json.Unmarshal([]byte(dimStr), &dimensions); err != nil {
			return buildResponse(nil, true, fmt.Sprintf("invalid dimensions JSON: %s", err)), nil
		}
	}

	slog.InfoContext(ctx, "cms_query_metric",
		"namespace", namespace, "metricName", metricName, "region", regionID)

	datapoints, err := h.cmsClient.QueryMetric(ctx, regionID, namespace, metricName, dimensions, fromTS, toTS)
	if err != nil {
		slog.ErrorContext(ctx, "cms_query_metric failed", "error", err)
		return buildResponse(nil, true, fmt.Sprintf("Query metric failed: %s", err)), nil
	}

	return buildResponse(map[string]interface{}{
		"datapoints": datapoints,
	}, false, ""), nil
}

// ===========================================================================
// Tool 2: cms_list_metrics
// ===========================================================================

func (h *cmsHandler) listMetricsTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "cms_list_metrics",
		Description: `列出云监控命名空间下的可用指标。

## 功能概述

该工具用于列出指定云产品命名空间下所有可用的监控指标。

## 使用场景

- 当需要了解某个云产品有哪些可用的监控指标时
- 当需要查找特定指标的名称时
- 当需要在查询指标数据前确认指标是否存在时

## 参数说明

- namespace: 云产品命名空间，如 'acs_ecs_dashboard'
- regionId: 阿里云区域ID`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "云产品命名空间，如 'acs_ecs_dashboard'",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
				},
			},
			"required": []string{"namespace", "regionId"},
		},
		Handler: h.handleListMetrics,
	}
}

func (h *cmsHandler) handleListMetrics(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	namespace := paramString(params, "namespace", "")
	regionID := paramString(params, "regionId", "")

	if namespace == "" || regionID == "" {
		return buildResponse(nil, true, "namespace and regionId are required"), nil
	}

	slog.InfoContext(ctx, "cms_list_metrics",
		"namespace", namespace, "region", regionID)

	// Use QueryMetric with empty metric name and short time range to discover available metrics.
	// The CMS API returns metric metadata when queried this way.
	// We query with a minimal time range to get the metric list.
	datapoints, err := h.cmsClient.QueryMetric(ctx, regionID, namespace, "", nil, 0, 0)
	if err != nil {
		slog.ErrorContext(ctx, "cms_list_metrics failed", "error", err)
		return buildResponse(nil, true, fmt.Sprintf("List metrics failed: %s", err)), nil
	}

	return buildResponse(map[string]interface{}{
		"metrics": datapoints,
	}, false, ""), nil
}

// ===========================================================================
// Tool 3: cms_list_namespaces
// ===========================================================================

func (h *cmsHandler) listNamespacesTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "cms_list_namespaces",
		Description: `列出云监控可用的命名空间。

## 功能概述

该工具用于列出阿里云云监控中所有可用的命名空间（云产品监控项）。

## 使用场景

- 当需要了解有哪些云产品支持云监控时
- 当需要查找特定云产品的命名空间名称时
- 当需要在查询指标前确认命名空间是否存在时

## 返回数据

返回可用的命名空间列表，每个命名空间包含名称和描述信息。`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
				},
			},
			"required": []string{"regionId"},
		},
		Handler: h.handleListNamespaces,
	}
}

func (h *cmsHandler) handleListNamespaces(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	regionID := paramString(params, "regionId", "")

	if regionID == "" {
		return buildResponse(nil, true, "regionId is required"), nil
	}

	slog.InfoContext(ctx, "cms_list_namespaces", "region", regionID)

	// Use QueryMetric with empty namespace and metric to list available namespaces.
	datapoints, err := h.cmsClient.QueryMetric(ctx, regionID, "", "", nil, 0, 0)
	if err != nil {
		slog.ErrorContext(ctx, "cms_list_namespaces failed", "error", err)
		return buildResponse(nil, true, fmt.Sprintf("List namespaces failed: %s", err)), nil
	}

	return buildResponse(map[string]interface{}{
		"namespaces": datapoints,
	}, false, ""), nil
}

// ===========================================================================
// Tool 4: cms_execute_promql (sls_query_metricstore)
// ===========================================================================

func (h *cmsHandler) executePromQLTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "sls_query_metricstore",
		Description: `执行PromQL指标查询。

## 功能概述

该工具用于在指定的SLS项目和指标库上执行PromQL查询语句，并返回时序指标数据。

## 使用场景

- 当需要查询时序指标数据时
- 当需要分析系统性能指标时
- 当需要监控业务指标趋势时

## 查询语法

查询必须使用有效的PromQL语法。

## 时间范围

- from_time: 开始时间，支持Unix时间戳（秒/毫秒）或相对时间表达式
- to_time: 结束时间，支持Unix时间戳（秒/毫秒）或相对时间表达式`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS项目名称",
				},
				"metricStore": map[string]interface{}{
					"type":        "string",
					"description": "SLS指标库名称",
				},
				"query": map[string]interface{}{
					"type":        "string",
					"description": "PromQL查询语句",
				},
				"from_time": map[string]interface{}{
					"type":        "string",
					"description": "开始时间，支持Unix时间戳或相对时间如 'now-5m'",
					"default":     "now-5m",
				},
				"to_time": map[string]interface{}{
					"type":        "string",
					"description": "结束时间，支持Unix时间戳或相对时间如 'now'",
					"default":     "now",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
				},
			},
			"required": []string{"project", "metricStore", "query", "regionId"},
		},
		Handler: h.handleExecutePromQL,
	}
}

func (h *cmsHandler) handleExecutePromQL(ctx context.Context, params map[string]interface{}) (interface{}, error) {
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

	// Wrap PromQL in SPL template for execution via SLS
	// SPL format: .metricstore with(promql_query='<query>')
	splQuery := fmt.Sprintf(".%s with(promql_query='%s')", metricStore, query)

	results, err := h.slsClient.Query(ctx, regionID, project, metricStore, splQuery, fromTS, toTS)
	if err != nil {
		slog.ErrorContext(ctx, "cms_execute_promql failed", "error", err)
		if isMetricStoreNotFoundError(err) {
			msg := fmt.Sprintf(
				"Metric store '%s' does not exist in project '%s'. "+
					"Please use sls_list_metricstores to check available metric stores. "+
					"Alternatively, you can use cms_query_metric to query CMS metrics directly.",
				metricStore, project,
			)
			return buildResponse(nil, true, msg), nil
		}
		return buildResponse(nil, true, fmt.Sprintf("PromQL query failed: %s", err)), nil
	}

	return buildResponse(map[string]interface{}{
		"data":  results,
		"query": splQuery,
	}, false, ""), nil
}


// ===========================================================================
// Tool 5: cms_execute_promql (Alias for Python compatibility)
// ===========================================================================

func (h *cmsHandler) cmsExecutePromQLTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "cms_execute_promql",
		Description: `执行PromQL查询（与 sls_query_metricstore 功能相同）。

## 功能概述

该工具用于执行PromQL查询，查询云监控指标数据。
这是 sls_query_metricstore 的别名，为了与 Python 版本保持兼容。

## 使用场景

- 当需要查询云监控指标数据时
- 当需要使用 PromQL 语法分析时序数据时

## 查询语法

查询必须使用有效的PromQL语法。

## 时间范围

- from_time: 开始时间，支持Unix时间戳（秒/毫秒）或相对时间表达式（如 'now-5m'）
- to_time: 结束时间，支持Unix时间戳（秒/毫秒）或相对时间表达式（如 'now'）`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "项目名称",
				},
				"metricStore": map[string]interface{}{
					"type":        "string",
					"description": "指标存储名称",
				},
				"query": map[string]interface{}{
					"type":        "string",
					"description": "PromQL查询语句",
				},
				"from_time": map[string]interface{}{
					"type":        "string",
					"description": "查询开始时间，支持Unix时间戳或相对时间如 'now-5m'",
					"default":     "now-5m",
				},
				"to_time": map[string]interface{}{
					"type":        "string",
					"description": "查询结束时间，支持Unix时间戳或相对时间如 'now'",
					"default":     "now",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
				},
			},
			"required": []string{"project", "metricStore", "query", "regionId"},
		},
		Handler: h.handleExecutePromQL, // Reuse the same handler
	}
}


// ===========================================================================
// Tool 6: cms_text_to_promql (Alias for sls_text_to_promql for Python compatibility)
// ===========================================================================

func (h *cmsHandler) cmsTextToPromQLTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "cms_text_to_promql",
		Description: `将自然语言转换为PromQL查询语句。

## 功能概述

该工具可以将自然语言描述转换为有效的PromQL查询语句，便于用户使用自然语言表达查询需求。

## 使用场景

- 当用户不熟悉PromQL查询语法时
- 当需要快速构建复杂查询时
- 当需要从自然语言描述中提取查询意图时

## 使用限制

- 仅支持生成PromQL查询
- 生成的是查询语句，而非查询结果

## 最佳实践

- 提供清晰简洁的自然语言描述
- 不要在描述中包含项目或时序库名称
- 首次生成的查询可能不完全符合要求，可能需要多次尝试

## 查询示例

- "帮我生成 XXX 的PromQL查询语句"
- "查询每个namespace下的Pod数量"`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text": map[string]interface{}{
					"type":        "string",
					"description": "用于生成PromQL的自然语言文本",
				},
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS项目名称",
				},
				"metricStore": map[string]interface{}{
					"type":        "string",
					"description": "SLS指标库名称",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
				},
			},
			"required": []string{"text", "project", "metricStore", "regionId"},
		},
		Handler: h.handleTextToPromQL,
	}
}

func (h *cmsHandler) handleTextToPromQL(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	text := paramString(params, "text", "")
	project := paramString(params, "project", "")
	metricStore := paramString(params, "metricStore", "")
	regionID := paramString(params, "regionId", "")

	if text == "" || project == "" || metricStore == "" || regionID == "" {
		return buildResponse(nil, true, "text, project, metricStore and regionId are required"), nil
	}

	slog.InfoContext(ctx, "cms_text_to_promql",
		"project", project, "metricStore", metricStore, "region", regionID)

	// Use the SLS TextToSQL method which handles PromQL generation
	promql, err := h.slsClient.TextToSQL(ctx, regionID, project, metricStore, text)
	if err != nil {
		slog.ErrorContext(ctx, "cms_text_to_promql failed", "error", err)
		return buildResponse(nil, true, fmt.Sprintf("Text to PromQL failed: %s", err)), nil
	}

	return buildResponse(map[string]interface{}{
		"query": promql,
	}, false, ""), nil
}
