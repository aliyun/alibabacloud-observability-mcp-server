// Package iaas implements the IaaS Toolkit, providing tools for direct access
// to SLS and CMS underlying APIs. SLS tool names use the `sls_` prefix.
package iaas

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/alibabacloud-observability-mcp-server-go/internal/client"
	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/timeparse"
)

// SLSTools returns all SLS tools backed by the given SLS and CMS clients.
// The CMS client is needed for sls_execute_spl when using workspace parameter.
func SLSTools(slsClient client.SLSClient, cmsClient client.CMSClient) []toolkit.Tool {
	h := &slsHandler{slsClient: slsClient, cmsClient: cmsClient}
	return []toolkit.Tool{
		h.queryLogstoreTool(),
		h.queryMetricstoreTool(),
		h.listProjectsTool(),
		h.listLogstoresTool(),
		h.listMetricstoresTool(),
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
func parseTimeParam(params map[string]interface{}, key, defaultVal string) (int64, error) {
	v, ok := params[key]
	if !ok || v == nil {
		return timeparse.ParseTimeExpression(defaultVal, time.Now())
	}
	switch val := v.(type) {
	case string:
		if val == "" {
			return timeparse.ParseTimeExpression(defaultVal, time.Now())
		}
		return timeparse.ParseTimeExpression(val, time.Now())
	case float64:
		ts := int64(val)
		// 13-digit timestamps are milliseconds
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
		return timeparse.ParseTimeExpression(defaultVal, time.Now())
	}
}

// ===========================================================================
// Tool 1: sls_query_logstore
// ===========================================================================

func (h *slsHandler) queryLogstoreTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "sls_query_logstore",
		Description: `执行SLS日志查询。

## 功能概述

该工具用于在指定的SLS项目和日志库上执行查询语句，并返回查询结果。

## 使用场景

- 当需要根据特定条件查询日志数据时
- 当需要分析特定时间范围内的日志信息时
- 当需要检索日志中的特定事件或错误时
- 当需要统计日志数据的聚合信息时

## 查询语法

查询必须使用SLS有效的查询语法，而非自然语言。如果不了解日志库的结构，可以先使用sls_list_logstores工具获取索引信息。

## 时间范围

- from_time: 开始时间，支持Unix时间戳（秒/毫秒）或相对时间表达式（如 'now-1h'）
- to_time: 结束时间，支持Unix时间戳（秒/毫秒）或相对时间表达式（如 'now'）`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS项目名称",
				},
				"logStore": map[string]interface{}{
					"type":        "string",
					"description": "SLS日志库名称",
				},
				"query": map[string]interface{}{
					"type":        "string",
					"description": "SLS查询语句",
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
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "返回结果的最大数量，范围1-100，默认10",
					"default":     10,
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
				},
			},
			"required": []string{"project", "logStore", "query", "regionId"},
		},
		Handler: h.handleQueryLogstore,
	}
}

func (h *slsHandler) handleQueryLogstore(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	project := paramString(params, "project", "")
	logStore := paramString(params, "logStore", "")
	query := paramString(params, "query", "")
	regionID := paramString(params, "regionId", "")

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

	slog.InfoContext(ctx, "sls_query_logstore",
		"project", project, "logStore", logStore, "region", regionID)

	results, err := h.slsClient.Query(ctx, regionID, project, logStore, query, fromTS, toTS, 100, 0, false)
	if err != nil {
		slog.ErrorContext(ctx, "sls_query_logstore failed", "error", err)

		if isSPLIncompatibleError(err) {
			slog.WarnContext(ctx, "sls_query_logstore: logstore may not support SPL format",
				"project", project, "logStore", logStore, "query", query)
			return buildResponse(nil, true, fmt.Sprintf(
				"Query failed: %s\n\nThis logstore may not support SPL query format. "+
					"Please try using SQL format instead, e.g.: '* | select count(1) as cnt' or '* | select * limit 10'. "+
					"You can also use the sls_execute_sql tool for SQL queries, or sls_execute_spl for SPL queries on supported logstores.",
				err)), nil
		}

		return buildResponse(nil, true, fmt.Sprintf("Query failed: %s", err)), nil
	}

	return buildResponse(results, false, ""), nil
}

// isSPLIncompatibleError checks whether the error indicates the logstore does not support SPL format.
func isSPLIncompatibleError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "InvalidSPLFormat") ||
		strings.Contains(msg, "InvalidSpls") ||
		strings.Contains(msg, "not support SPL")
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
// Tool 2: sls_query_metricstore
// ===========================================================================

func (h *slsHandler) queryMetricstoreTool() toolkit.Tool {
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
		Handler: h.handleQueryMetricstore,
	}
}

func (h *slsHandler) handleQueryMetricstore(ctx context.Context, params map[string]interface{}) (interface{}, error) {
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

	slog.InfoContext(ctx, "sls_query_metricstore",
		"project", project, "metricStore", metricStore, "region", regionID)

	// Use the SLS Query method against the metric store
	results, err := h.slsClient.Query(ctx, regionID, project, metricStore, query, fromTS, toTS, 100, 0, false)
	if err != nil {
		slog.ErrorContext(ctx, "sls_query_metricstore failed", "error", err)
		if isMetricStoreNotFoundError(err) {
			msg := fmt.Sprintf(
				"Metric store '%s' does not exist in project '%s'. "+
					"Please use sls_list_metricstores to check available metric stores. "+
					"Alternatively, you can use cms_query_metric to query CMS metrics directly.",
				metricStore, project,
			)
			return buildResponse(nil, true, msg), nil
		}
		return buildResponse(nil, true, fmt.Sprintf("Query failed: %s", err)), nil
	}

	return buildResponse(results, false, ""), nil
}

// ===========================================================================
// Tool 3: sls_list_projects
// ===========================================================================

func (h *slsHandler) listProjectsTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "sls_list_projects",
		Description: `列出阿里云日志服务中的所有项目。

## 功能概述

该工具可以列出指定区域中的所有SLS项目，支持通过项目名进行模糊搜索。
如果不提供项目名称，则返回该区域的所有项目。

## 使用场景

- 当需要查找特定项目是否存在时
- 当需要获取某个区域下所有可用的SLS项目列表时
- 当需要根据项目名称的部分内容查找相关项目时

## 返回数据结构

返回的项目信息包含：
- project_name: 项目名称
- description: 项目描述
- region_id: 项目所在区域

## 查询示例

- "有没有叫 XXX 的 project"
- "列出所有SLS项目"`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"projectName": map[string]interface{}{
					"type":        "string",
					"description": "项目名称查询字符串，支持模糊搜索",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "返回结果的最大数量，范围1-100，默认50",
					"default":     50,
					"minimum":     1,
					"maximum":     100,
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
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
		"message":  fmt.Sprintf("当前最多支持查询%d个项目，为防止返回数据过长，如果需要查询更多项目，您可以提供 project 的关键词来模糊查询", limit),
	}, false, ""), nil
}

// ===========================================================================
// Tool 4: sls_list_logstores
// ===========================================================================

func (h *slsHandler) listLogstoresTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "sls_list_logstores",
		Description: `列出SLS项目中的日志库。

## 功能概述

该工具可以列出指定SLS项目中的所有日志库，如果不选，则默认为日志库类型。
支持通过日志库名称进行模糊搜索。如果不提供日志库名称，则返回项目中的所有日志库。

## 使用场景

- 当需要查找特定项目下是否存在某个日志库时
- 当需要获取项目中所有可用的日志库列表时
- 当需要根据日志库名称的部分内容查找相关日志库时
- 如果从上下文未指定 project参数，除非用户说了遍历，则可使用 sls_list_projects 工具获取项目列表

## 是否指标库

如果需要查找指标或者时序相关的库，请将isMetricStore参数设置为True

## 查询示例

- "我想查询有没有 XXX 的日志库"
- "某个 project 有哪些 logstore"`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS项目名称，必须精确匹配，不能包含中文字符",
				},
				"logStore": map[string]interface{}{
					"type":        "string",
					"description": "日志库名称，支持模糊搜索",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "返回结果的最大数量，范围1-100，默认10",
					"default":     10,
					"minimum":     1,
					"maximum":     100,
				},
				"isMetricStore": map[string]interface{}{
					"type":        "boolean",
					"description": "是否查询指标库，默认为false。仅当需要查找指标库时设置为true",
					"default":     false,
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
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
	message := fmt.Sprintf("当前最多支持查询%d个日志库，为防止返回数据过长，如果需要查询更多日志库，您可以提供 logstore 的关键词来模糊查询", limit)
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
// Tool 5: sls_list_metricstores
// ===========================================================================

func (h *slsHandler) listMetricstoresTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "sls_list_metricstores",
		Description: `列出SLS项目中的指标库。

## 功能概述

该工具可以列出指定SLS项目中的所有指标库（Metric Store）。

## 使用场景

- 当需要查找特定项目下的指标库时
- 当需要获取项目中所有可用的指标库列表时`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS项目名称，必须精确匹配",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
				},
			},
			"required": []string{"project", "regionId"},
		},
		Handler: h.handleListMetricstores,
	}
}

func (h *slsHandler) handleListMetricstores(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	project := paramString(params, "project", "")
	regionID := paramString(params, "regionId", "")

	if project == "" || regionID == "" {
		return buildResponse(nil, true, "project and regionId are required"), nil
	}

	slog.InfoContext(ctx, "sls_list_metricstores", "project", project, "region", regionID)

	metricstores, err := h.slsClient.ListMetricStores(ctx, regionID, project)
	if err != nil {
		slog.ErrorContext(ctx, "sls_list_metricstores failed", "error", err)
		return buildResponse(nil, true, fmt.Sprintf("Failed to list metric stores: %s", err)), nil
	}

	return buildResponse(map[string]interface{}{
		"metricstores": metricstores,
	}, false, ""), nil
}

// ===========================================================================
// Tool 6: sls_text_to_sql
// ===========================================================================

func (h *slsHandler) textToSQLTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "sls_text_to_sql",
		Description: `将自然语言转换为SLS查询语句。当用户有明确的logstore查询需求，必须优先使用该工具来生成查询语句。

## 功能概述

该工具可以将自然语言描述转换为有效的SLS查询语句，便于用户使用自然语言表达查询需求。

## 使用场景

- 当用户不熟悉SLS查询语法时
- 当需要快速构建复杂查询时
- 当需要从自然语言描述中提取查询意图时

## 使用限制

- 仅支持生成SLS查询，不支持其他数据库的SQL
- 生成的是查询语句，而非查询结果，需要配合sls_query_logstore工具使用

## 最佳实践

- 提供清晰简洁的自然语言描述
- 不要在描述中包含项目或日志库名称

## 查询示例

- "帮我生成下 XXX 的日志查询语句"
- "查找最近一小时内的错误日志"`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text": map[string]interface{}{
					"type":        "string",
					"description": "用于生成查询的自然语言文本",
				},
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS项目名称",
				},
				"logStore": map[string]interface{}{
					"type":        "string",
					"description": "SLS日志库名称",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
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
		Description: `⚠️ [已废弃] 将自然语言转换为SLS查询语句（旧版API）。

## 废弃说明

此工具已废弃，请优先使用 sls_text_to_sql 工具。

新版 sls_text_to_sql 工具使用 CMS Chat API，提供更智能的 SQL 生成能力，
包括更好的上下文理解、更准确的查询生成和更详细的解释说明。

## 功能概述

该工具使用旧版SLS CallAiTools API将自然语言描述转换为有效的SLS查询语句。

## 使用场景

- 仅当新版 sls_text_to_sql 工具不可用时作为备选方案

## 使用限制

- 仅支持生成SLS查询，不支持其他数据库的SQL
- 生成的是查询语句，而非查询结果，需要配合sls_query_logstore工具使用
- 需要对应的 logstore 已经设定了索引信息`,
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
		Description: `将自然语言转换为PromQL查询语句。

## 功能概述

该工具可以将自然语言描述转换为有效的PromQL查询语句，便于用户使用自然语言表达查询需求。

## 使用场景

- 当用户不熟悉PromQL查询语法时
- 当需要快速构建复杂查询时

## 使用限制

- 仅支持生成PromQL查询
- 生成的是查询语句，而非查询结果

## 最佳实践

- 提供清晰简洁的自然语言描述
- 不要在描述中包含项目或时序库名称

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
		Description: `SLS SOP (Standard Operating Procedure) 智能运维助手。

## 功能概述

该工具是一个智能助手，用于回答关于SLS使用方法、功能介绍、操作步骤等方面的问题。

## 使用场景

- 当用户不知道如何使用某个SLS功能时
- 当用户需要了解SLS的概念或术语时
- 当用户遇到操作问题需要指引时

## 示例用法

- "如何创建新的数据加工"
- "什么是 Project"
- "怎么配置告警"`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text": map[string]interface{}{
					"type":        "string",
					"description": "用户关于SLS使用或SOP的问题",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID",
				},
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS项目名称（可选）",
				},
				"logStore": map[string]interface{}{
					"type":        "string",
					"description": "SLS日志库名称（可选）",
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

	// SOP uses TextToSQL as the underlying AI mechanism with the question as input
	answer, err := h.slsClient.TextToSQL(ctx, regionID, project, logStore, text)
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
		Description: `执行SLS日志查询。

## 功能概述

该工具用于在指定的SLS项目和日志库上执行查询语句，并返回查询结果。

## 使用场景

- 当需要根据特定条件查询日志数据时
- 当需要分析特定时间范围内的日志信息时
- 当需要检索日志中的特定事件或错误时
- 当需要统计日志数据的聚合信息时

## 查询语法

查询必须使用SLS有效的查询语法，而非自然语言。如果不了解日志库的结构，可以先使用sls_list_logstores工具获取索引信息。

## 时间范围

- from_time: 开始时间，支持Unix时间戳（秒/毫秒）或相对时间表达式（如 'now-1h'）
- to_time: 结束时间，支持Unix时间戳（秒/毫秒）或相对时间表达式（如 'now'）`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS项目名称",
				},
				"logStore": map[string]interface{}{
					"type":        "string",
					"description": "SLS日志库名称",
				},
				"query": map[string]interface{}{
					"type":        "string",
					"description": "SLS查询语句",
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
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "返回结果的最大数量，范围1-100，默认10",
					"default":     10,
				},
				"offset": map[string]interface{}{
					"type":        "integer",
					"description": "查询开始偏移量，用于分页，默认0",
					"default":     0,
				},
				"reverse": map[string]interface{}{
					"type":        "boolean",
					"description": "是否按时间戳降序返回，默认false",
					"default":     false,
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
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

	results, err := h.slsClient.Query(ctx, regionID, project, logStore, query, fromTS, toTS, limit, offset, reverse)
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
		Description: `执行原生SPL查询语句。

## 功能概述

该工具允许直接执行原生SPL（Search Processing Language）查询语句，
为高级用户提供最大的灵活性和功能。支持复杂的数据操作和分析。

## 两种模式

1. **CMS模式**（推荐）：提供 workspace 参数，使用 CMS 工作空间执行 SPL
2. **SLS模式**：提供 project 和 logStore 参数，使用 SLS 日志库执行查询

两种模式互斥，优先使用 workspace 模式。

## 使用场景

- 复杂的数据分析和统计计算，超出标准API的覆盖范围
- 自定义的数据聚合和转换操作
- 跨多个实体集合的联合查询和关联分析
- 高级的数据挖掘和机器学习分析

## 注意事项

- 需要对SPL语法有一定的了解
- 请确保查询语句的正确性，错误的查询可能导致无结果或错误
- 复杂查询可能消耗较多的计算资源和时间`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "原生SPL查询语句",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS工作空间名称，可通过list_workspace获取（与 project/logStore 互斥，优先使用）",
				},
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS项目名称（与 workspace 互斥）",
				},
				"logStore": map[string]interface{}{
					"type":        "string",
					"description": "SLS日志库名称（与 workspace 互斥）",
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
			"required": []string{"query", "regionId"},
		},
		Handler: h.handleExecuteSPL,
	}
}

func (h *slsHandler) handleExecuteSPL(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	query := paramString(params, "query", "")
	workspace := paramString(params, "workspace", "")
	project := paramString(params, "project", "")
	logStore := paramString(params, "logStore", "")
	regionID := paramString(params, "regionId", "")

	if query == "" || regionID == "" {
		return buildResponse(nil, true, "query and regionId are required"), nil
	}

	fromTS, err := parseTimeParam(params, "from_time", "now-5m")
	if err != nil {
		return buildResponse(nil, true, fmt.Sprintf("invalid from_time: %s", err)), nil
	}
	toTS, err := parseTimeParam(params, "to_time", "now")
	if err != nil {
		return buildResponse(nil, true, fmt.Sprintf("invalid to_time: %s", err)), nil
	}

	// Determine which mode to use: workspace (CMS) or project/logStore (SLS)
	if workspace != "" {
		// CMS mode: use workspace
		return h.executeCMSSPL(ctx, workspace, query, regionID, fromTS, toTS, params)
	}

	if project != "" && logStore != "" {
		// SLS mode: use project/logStore
		return h.executeSLSSPL(ctx, project, logStore, query, regionID, fromTS, toTS, params)
	}

	return buildResponse(nil, true, "workspace or (project + logStore) is required"), nil
}

// executeCMSSPL executes SPL query using CMS client (workspace mode)
func (h *slsHandler) executeCMSSPL(ctx context.Context, workspace, query, regionID string, fromTS, toTS int64, params map[string]interface{}) (interface{}, error) {
	slog.InfoContext(ctx, "sls_execute_spl (CMS mode)",
		"workspace", workspace, "region", regionID,
		"from", fromTS, "to", toTS)

	result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, 1000)
	if err != nil {
		slog.ErrorContext(ctx, "sls_execute_spl (CMS mode) failed", "error", err)
		return buildResponse(nil, true, fmt.Sprintf("SPL query failed: %s", err)), nil
	}

	data := result["data"]

	// Build enhanced response with query and time_range information
	response := map[string]interface{}{
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
	}

	return response, nil
}

// executeSLSSPL executes SPL query using SLS client (project/logStore mode)
func (h *slsHandler) executeSLSSPL(ctx context.Context, project, logStore, query, regionID string, fromTS, toTS int64, params map[string]interface{}) (interface{}, error) {
	slog.InfoContext(ctx, "sls_execute_spl (SLS mode)",
		"project", project, "logStore", logStore, "region", regionID,
		"from", fromTS, "to", toTS)

	results, err := h.slsClient.Query(ctx, regionID, project, logStore, query, fromTS, toTS, 100, 0, false)
	if err != nil {
		slog.ErrorContext(ctx, "sls_execute_spl (SLS mode) failed", "error", err)
		return buildResponse(nil, true, fmt.Sprintf("SPL query failed: %s", err)), nil
	}

	// Build enhanced response with query and time_range information
	response := map[string]interface{}{
		"error":   false,
		"data":    results,
		"message": "SPL query executed successfully",
		"query":   query,
		"time_range": map[string]interface{}{
			"from":          fromTS,
			"to":            toTS,
			"from_readable": time.Unix(fromTS, 0).Format(time.RFC3339),
			"to_readable":   time.Unix(toTS, 0).Format(time.RFC3339),
			"expression":    fmt.Sprintf("%s ~ %s", paramString(params, "from_time", "now-5m"), paramString(params, "to_time", "now")),
		},
	}

	return response, nil
}

// ===========================================================================
// Tool 11: sls_get_context_logs
// ===========================================================================

func (h *slsHandler) getContextLogsTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "sls_get_context_logs",
		Description: `查询指定日志前后的上下文日志。

## 功能概述

该工具用于根据"起始日志"的 pack_id 与 pack_meta，查询该日志前（上文）后（下文）的若干条上下文日志。

说明：上下文查询的时间范围固定为起始日志的前后一天（由SLS服务端限制）。

## 如何获取 pack_id 与 pack_meta

先使用 sls_execute_sql 获取目标日志，并在查询语句末尾追加 |with_pack_meta，
使查询结果携带内部字段：
- __pack_id__：对应本工具的 pack_id
- __pack_meta__：对应本工具的 pack_meta

然后选定你要作为起始点的那条日志，将上述两个字段值传入本工具即可。

## 参数说明

- back_lines / forward_lines：范围 0~100，且两者至少一个大于 0。

## 返回结果

返回结构与SLS OpenAPI一致，其中 logs 内每条日志会包含：
- __index_number__：相对起始日志的位置（负数为上文，0 为起始日志，正数为下文）`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS项目名称",
				},
				"logStore": map[string]interface{}{
					"type":        "string",
					"description": "SLS日志库名称",
				},
				"pack_id": map[string]interface{}{
					"type":        "string",
					"description": "起始日志的 pack_id",
				},
				"pack_meta": map[string]interface{}{
					"type":        "string",
					"description": "起始日志的 pack_meta",
				},
				"back_lines": map[string]interface{}{
					"type":        "integer",
					"description": "向前查询的日志行数，范围0-100，默认10",
					"default":     10,
				},
				"forward_lines": map[string]interface{}{
					"type":        "integer",
					"description": "向后查询的日志行数，范围0-100，默认10",
					"default":     10,
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
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
const packValueHint = "pack_id 和 pack_meta 必须来自之前的日志查询结果。" +
	"请先使用 sls_execute_sql 或 sls_query_logstore 查询日志，" +
	"并在查询语句末尾追加 |with_pack_meta，" +
	"查询结果中将包含 __pack_id__ 和 __pack_meta__ 字段，" +
	"将这两个字段的值传入本工具即可。"

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
			"pack_id 或 pack_meta 的值无效（当前值: pack_id=%q, pack_meta=%q）。%s",
			packID, packMeta, packValueHint)), nil
	}

	if backLines == 0 && forwardLines == 0 {
		return buildResponse(nil, true, "back_lines 与 forward_lines 不能同时为 0，至少一个需要大于 0"), nil
	}

	slog.InfoContext(ctx, "sls_get_context_logs",
		"project", project, "logStore", logStore, "region", regionID,
		"pack_id", packID, "back_lines", backLines, "forward_lines", forwardLines)

	// Use the dedicated GetContextLogs API (matching Python implementation)
	result, err := h.slsClient.GetContextLogs(ctx, regionID, project, logStore, packID, packMeta, backLines, forwardLines)
	if err != nil {
		slog.ErrorContext(ctx, "sls_get_context_logs failed", "error", err)
		return buildResponse(nil, true, fmt.Sprintf(
			"Get context logs failed: %s\n\n提示: %s", err, packValueHint)), nil
	}

	return buildResponse(result, false, ""), nil
}

// ===========================================================================
// Tool 12: sls_text_to_spl
// ===========================================================================

func (h *slsHandler) textToSPLTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "sls_text_to_spl",
		Description: `将自然语言转换为SLS SPL查询语句。

## 功能概述

该工具可以将自然语言描述转换为有效的SLS SPL查询语句。
注意：SPL (Search Processing Language) 是SLS的一种管道式查询语句，主要用于数据加工、过滤、提取等场景，与标准SQL不同。

## 使用场景

- 当用户需要对日志数据进行结构化提取时
- 当需要从非结构化文本中解析字段时

## 注意事项

- 本工具会基于提供的 data_sample 直接执行生成的 SPL 并返回结果
- 返回结果中包含生成的 query 和基于样例数据的执行结果 data`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text": map[string]interface{}{
					"type":        "string",
					"description": "用于生成SPL的自然语言描述",
				},
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS项目名称",
				},
				"logStore": map[string]interface{}{
					"type":        "string",
					"description": "SLS日志库名称",
				},
				"data_sample": map[string]interface{}{
					"type":        "array",
					"description": "样例日志数据，用于SPL生成",
					"items": map[string]interface{}{
						"type": "object",
					},
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
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

	// Use TextToSQL as the underlying mechanism for SPL generation
	spl, err := h.slsClient.TextToSQL(ctx, regionID, project, logStore, text)
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
		Description: `查看阿里云日志服务中某个日志库的日志数据聚合分析结果，提供日志数据概览信息。

## 功能概述

该工具可以给出指定日志库中日志数据的概览信息。给出日志数据中典型的日志模板，以及各个日志模板对应日志数量分布。

## 使用场景

- 当需要查看日志库中日志的概览信息和数据分布时

## 查询实例

- "查询某个 project 的某个 logstore 中的日志数据分布"
- "某个 project 的某个 logstore 中不同风险等级的日志有哪些"`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS项目名称，必须精确匹配，不能包含中文字符",
				},
				"logStore": map[string]interface{}{
					"type":        "string",
					"description": "SLS日志库名称，必须精确匹配，不能包含中文字符",
				},
				"from_time": map[string]interface{}{
					"type":        "string",
					"description": "开始时间: Unix时间戳(秒/毫秒)或相对时间(now-5m)",
					"default":     "now-1h",
				},
				"to_time": map[string]interface{}{
					"type":        "string",
					"description": "结束时间: Unix时间戳(秒/毫秒)或相对时间(now)",
					"default":     "now",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID",
				},
				"logField": map[string]interface{}{
					"type":        "string",
					"description": "包含日志消息的字段名称",
				},
				"filter_query": map[string]interface{}{
					"type":        "string",
					"description": "过滤查询语句，必须是有效的SLS查询语句，用于过滤日志数据",
				},
				"groupField": map[string]interface{}{
					"type":        "string",
					"description": "包含日志消息分组标识的字段名称",
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
	countResult, err := h.slsClient.Query(ctx, regionID, project, logStore, countQuery, fromTS, toTS, 100, 0, false)
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

	modelResult, err := h.slsClient.Query(ctx, regionID, project, logStore, splQuery, fromTS, toTS, 100, 0, false)
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

	matchResult, err := h.slsClient.Query(ctx, regionID, project, logStore, matchQuery, fromTS, toTS, 100, 0, false)
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
	Pattern    string  `json:"pattern"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
	Examples   []string `json:"examples"`
}

// LogExploreResult contains the results of log exploration analysis
type LogExploreResult struct {
	Patterns     []LogPattern           `json:"patterns"`
	TotalLogs    int                    `json:"total_logs"`
	UniquePatterns int                  `json:"unique_patterns"`
	Distribution map[string]int         `json:"distribution"`
}

// analyzeLogPatterns performs pattern recognition and clustering on logs
func (h *slsHandler) analyzeLogPatterns(logs interface{}, maxPatterns int) LogExploreResult {
	result := LogExploreResult{
		Patterns:     make([]LogPattern, 0),
		Distribution: make(map[string]int),
	}

	// Convert logs to slice of maps
	logSlice, ok := logs.([]interface{})
	if !ok {
		return result
	}

	result.TotalLogs = len(logSlice)
	if result.TotalLogs == 0 {
		return result
	}

	// Pattern extraction: group logs by their normalized pattern
	patternMap := make(map[string]*patternInfo)

	for _, log := range logSlice {
		logMap, ok := log.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract the main content field (commonly __content__ or message)
		content := extractLogContent(logMap)
		if content == "" {
			continue
		}

		// Normalize the log to extract pattern
		pattern := normalizeLogToPattern(content)

		if info, exists := patternMap[pattern]; exists {
			info.count++
			if len(info.examples) < 3 {
				info.examples = append(info.examples, content)
			}
		} else {
			patternMap[pattern] = &patternInfo{
				pattern:  pattern,
				count:    1,
				examples: []string{content},
			}
		}
	}

	// Convert to sorted slice and limit to maxPatterns
	patterns := make([]LogPattern, 0, len(patternMap))
	for _, info := range patternMap {
		percentage := float64(info.count) / float64(result.TotalLogs) * 100
		patterns = append(patterns, LogPattern{
			Pattern:    info.pattern,
			Count:      info.count,
			Percentage: percentage,
			Examples:   info.examples,
		})
	}

	// Sort by count descending
	sortPatternsByCount(patterns)

	// Limit to maxPatterns
	if len(patterns) > maxPatterns {
		patterns = patterns[:maxPatterns]
	}

	result.Patterns = patterns
	result.UniquePatterns = len(patternMap)

	// Build distribution map
	for _, p := range patterns {
		result.Distribution[p.Pattern] = p.Count
	}

	return result
}

// patternInfo holds intermediate pattern analysis data
type patternInfo struct {
	pattern  string
	count    int
	examples []string
}

// extractLogContent extracts the main content from a log entry
func extractLogContent(logMap map[string]interface{}) string {
	// Try common content field names
	contentFields := []string{"__content__", "content", "message", "msg", "log", "body"}
	for _, field := range contentFields {
		if v, ok := logMap[field]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}

	// If no content field found, concatenate all string values
	var parts []string
	for _, v := range logMap {
		if s, ok := v.(string); ok && s != "" {
			parts = append(parts, s)
		}
	}
	if len(parts) > 0 {
		return parts[0] // Return first non-empty string
	}
	return ""
}

// normalizeLogToPattern converts a log message to a pattern by replacing variable parts
func normalizeLogToPattern(content string) string {
	pattern := content

	// Replace common variable patterns with placeholders
	// IP addresses
	pattern = replacePattern(pattern, `\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`, "<IP>")
	// UUIDs
	pattern = replacePattern(pattern, `\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`, "<UUID>")
	// Timestamps (various formats)
	pattern = replacePattern(pattern, `\b\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:?\d{2})?\b`, "<TIMESTAMP>")
	// Unix timestamps
	pattern = replacePattern(pattern, `\b1[0-9]{9,12}\b`, "<UNIX_TS>")
	// Numbers (standalone)
	pattern = replacePattern(pattern, `\b\d+\b`, "<NUM>")
	// Hex strings (8+ chars)
	pattern = replacePattern(pattern, `\b[0-9a-fA-F]{8,}\b`, "<HEX>")
	// Email addresses
	pattern = replacePattern(pattern, `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`, "<EMAIL>")
	// URLs
	pattern = replacePattern(pattern, `https?://[^\s]+`, "<URL>")
	// File paths
	pattern = replacePattern(pattern, `/[^\s:]+`, "<PATH>")

	// Truncate very long patterns
	if len(pattern) > 200 {
		pattern = pattern[:200] + "..."
	}

	return pattern
}

// replacePattern replaces regex matches with a placeholder
func replacePattern(s, pattern, replacement string) string {
	re, err := compileRegex(pattern)
	if err != nil {
		return s
	}
	return re.ReplaceAllString(s, replacement)
}

// regexCache caches compiled regular expressions
var regexCache = make(map[string]*regexpWrapper)

type regexpWrapper struct {
	re *regexp.Regexp
}

func compileRegex(pattern string) (*regexp.Regexp, error) {
	if cached, ok := regexCache[pattern]; ok {
		return cached.re, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	regexCache[pattern] = &regexpWrapper{re: re}
	return re, nil
}

// sortPatternsByCount sorts patterns by count in descending order
func sortPatternsByCount(patterns []LogPattern) {
	for i := 0; i < len(patterns)-1; i++ {
		for j := i + 1; j < len(patterns); j++ {
			if patterns[j].Count > patterns[i].Count {
				patterns[i], patterns[j] = patterns[j], patterns[i]
			}
		}
	}
}

// extractCount extracts count from query result
func extractCount(result interface{}) int64 {
	if result == nil {
		return 0
	}
	slice, ok := result.([]interface{})
	if !ok || len(slice) == 0 {
		return 0
	}
	row, ok := slice[0].(map[string]interface{})
	if !ok {
		return 0
	}
	if cnt, ok := row["cnt"]; ok {
		switch v := cnt.(type) {
		case float64:
			return int64(v)
		case int64:
			return v
		case int:
			return int64(v)
		case string:
			var n int64
			fmt.Sscanf(v, "%d", &n)
			return n
		}
	}
	return 0
}

// extractModelID extracts model_id from query result
func extractModelID(result interface{}) string {
	if result == nil {
		return ""
	}
	slice, ok := result.([]interface{})
	if !ok || len(slice) == 0 {
		return ""
	}
	row, ok := slice[0].(map[string]interface{})
	if !ok {
		return ""
	}
	if modelID, ok := row["model_id"].(string); ok {
		return modelID
	}
	return ""
}

// extractErrorMsg extracts error_msg from query result
func extractErrorMsg(result interface{}) string {
	if result == nil {
		return ""
	}
	slice, ok := result.([]interface{})
	if !ok || len(slice) == 0 {
		return ""
	}
	row, ok := slice[0].(map[string]interface{})
	if !ok {
		return ""
	}
	if errMsg, ok := row["error_msg"].(string); ok {
		return errMsg
	}
	return ""
}

// formatLogExploreResults formats the log explore query results
func formatLogExploreResults(result interface{}, timeBucketSize int64) []map[string]interface{} {
	patterns := make([]map[string]interface{}, 0)
	if result == nil {
		return patterns
	}
	slice, ok := result.([]interface{})
	if !ok {
		return patterns
	}

	for _, item := range slice {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

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
					fmt.Sscanf(ts, "%d", &tsInt)
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

// formatLogCompareResults formats the log compare query results
func formatLogCompareResults(result interface{}) []map[string]interface{} {
	if result == nil {
		return []map[string]interface{}{}
	}
	slice, ok := result.([]interface{})
	if !ok {
		return []map[string]interface{}{}
	}

	// Group results by (group, pattern) key
	type pairKey struct {
		group   string
		pattern string
	}
	pairs := make(map[pairKey]map[string]map[string]interface{})

	for _, item := range slice {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

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
		Description: `查看阿里云日志服务中某个日志库的日志数据在两个时间范围内的对比结果。

## 功能概述

该工具可以两个时间范围内的日志数据分布的区别，用于快速分析日志数据的变化情况。

## 使用场景

- 当需要分析两个时间范围内的日志数据分布的区别时

## 查询实例

- "昨天的日志和今天的相比有什么区别"
- "服务发布前后日志有什么变化"`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "SLS项目名称，必须精确匹配，不能包含中文字符",
				},
				"logStore": map[string]interface{}{
					"type":        "string",
					"description": "SLS日志库名称，必须精确匹配，不能包含中文字符",
				},
				"test_from_time": map[string]interface{}{
					"type":        "string",
					"description": "实验组数据的开始时间: Unix时间戳(秒/毫秒)或相对时间(now-5m)",
					"default":     "now-1h",
				},
				"test_to_time": map[string]interface{}{
					"type":        "string",
					"description": "实验组数据的结束时间: Unix时间戳(秒/毫秒)或相对时间(now)",
					"default":     "now",
				},
				"control_from_time": map[string]interface{}{
					"type":        "string",
					"description": "对照组数据的开始时间: Unix时间戳(秒/毫秒)或相对时间(now-5m)",
					"default":     "now-1h",
				},
				"control_to_time": map[string]interface{}{
					"type":        "string",
					"description": "对照组数据的结束时间: Unix时间戳(秒/毫秒)或相对时间(now)",
					"default":     "now",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID",
				},
				"logField": map[string]interface{}{
					"type":        "string",
					"description": "包含日志消息的字段名称",
				},
				"filter_query": map[string]interface{}{
					"type":        "string",
					"description": "过滤查询语句，必须是有效的SLS查询语句，用于过滤日志数据",
				},
				"groupField": map[string]interface{}{
					"type":        "string",
					"description": "包含日志消息分组标识的字段名称",
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

	// Parse test time range
	testFromTS, err := parseTimeParam(params, "test_from_time", "now-1h")
	if err != nil {
		return buildResponse(nil, true, fmt.Sprintf("invalid test_from_time: %s", err)), nil
	}
	testToTS, err := parseTimeParam(params, "test_to_time", "now")
	if err != nil {
		return buildResponse(nil, true, fmt.Sprintf("invalid test_to_time: %s", err)), nil
	}

	// Parse control time range
	controlFromTS, err := parseTimeParam(params, "control_from_time", "now-1h")
	if err != nil {
		return buildResponse(nil, true, fmt.Sprintf("invalid control_from_time: %s", err)), nil
	}
	controlToTS, err := parseTimeParam(params, "control_to_time", "now")
	if err != nil {
		return buildResponse(nil, true, fmt.Sprintf("invalid control_to_time: %s", err)), nil
	}

	// Check for time range overlap
	if !(testToTS < controlFromTS || controlToTS < testFromTS) {
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
	countResult, err := h.slsClient.Query(ctx, regionID, project, logStore, countQuery, testFromTS, testToTS, 100, 0, false)
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

	testModelResult, err := h.slsClient.Query(ctx, regionID, project, logStore, splQuery, testFromTS, testToTS, 100, 0, false)
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
	controlModelResult, err := h.slsClient.Query(ctx, regionID, project, logStore, splQuery, controlFromTS, controlToTS, 100, 0, false)
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

	mergeResult, err := h.slsClient.Query(ctx, regionID, project, logStore, mergeQuery, testFromTS, testToTS, 100, 0, false)
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

	matchResult, err := h.slsClient.Query(ctx, regionID, project, logStore, matchQuery, fromTimestamp, toTimestamp, 100, 0, false)
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


