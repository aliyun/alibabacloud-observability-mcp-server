package paas

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/alibabacloud-observability-mcp-server-go/internal/client"
	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit"
)

// DataTools returns all data query tools backed by the given CMS client.
func DataTools(cmsClient client.CMSClient) []toolkit.Tool {
	h := &dataHandler{cmsClient: cmsClient}
	return []toolkit.Tool{
		h.getMetricsTool(),
		h.getGoldenMetricsTool(),
		h.getRelationMetricsTool(),
		h.getLogsTool(),
		h.getEventsTool(),
		h.getTracesTool(),
		h.searchTracesTool(),
		h.getProfilingTool(),
		h.getProfilesAliasTool(), // Alias for Python compatibility
	}
}
// metricCompatibilityMap defines known compatible metric_domain_name values
// for each entity_set_name. This is used for pre-validation to give users
// a helpful hint before the API returns a cryptic "NoRelatedDataSetFound" error.
//
// Key: entity_set_name, Value: set of compatible metric_domain_name prefixes.
// An entity_set_name not present in the map is not validated (pass-through).
var metricCompatibilityMap = map[string]map[string]bool{
	"apm.service": {
		"apm.metric.exception": true,
		"apm.metric.service":   true,
	},
	"apm.instance": {
		"apm.metric.jvm":       true,
		"apm.metric.exception": true,
	},
	"k8s.pod": {
		"k8s.metric.pod": true,
	},
}

// metricSuggestedEntitySet maps a metric_domain_name to the entity_set_name
// that is typically compatible with it. Used to suggest corrections.
var metricSuggestedEntitySet = map[string]string{
	"apm.metric.jvm":       "apm.instance",
	"apm.metric.exception": "apm.instance", // also works with apm.service
	"apm.metric.service":   "apm.service",
	"k8s.metric.pod":       "k8s.pod",
}

// validateMetricCompatibility checks whether the given metric_domain_name is
// compatible with the entity_set_name. Returns an empty string if compatible
// (or unknown), or a suggestion message if incompatible.
func validateMetricCompatibility(entitySetName, metricDomainName string) string {
	compatibleMetrics, known := metricCompatibilityMap[entitySetName]
	if !known {
		// Entity set not in our map — skip validation, let the API decide.
		return ""
	}
	if compatibleMetrics[metricDomainName] {
		return "" // Compatible.
	}

	// Build suggestion.
	suggested, hasSuggestion := metricSuggestedEntitySet[metricDomainName]

	// List what IS compatible with this entity_set_name.
	var compatible []string
	for m := range compatibleMetrics {
		compatible = append(compatible, m)
	}

	msg := fmt.Sprintf(
		"metric_domain_name '%s' is not compatible with entity_set_name '%s'. "+
			"Compatible metric domains for '%s': %v.",
		metricDomainName, entitySetName, entitySetName, compatible,
	)
	if hasSuggestion {
		msg += fmt.Sprintf(
			" Hint: use entity_set_name='%s' for metric_domain_name='%s', "+
				"or call umodel_list_data_set to discover available metric sets.",
			suggested, metricDomainName,
		)
	}
	return msg
}

// getProfilesAliasTool returns an alias tool for umodel_get_profiling
// to maintain compatibility with Python MCP server which uses "umodel_get_profiles"
func (h *dataHandler) getProfilesAliasTool() toolkit.Tool {
	tool := h.getProfilingTool()
	tool.Name = "umodel_get_profiles"
	return tool
}

// dataHandler holds the CMS client and provides tool constructors and handlers.
type dataHandler struct {
	cmsClient client.CMSClient
}

// ===========================================================================
// Tool 1: umodel_get_metrics
// ===========================================================================

func (h *dataHandler) getMetricsTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "umodel_get_metrics",
		Description: `获取实体的时序指标数据，支持多种分析模式。

## 功能概述
查询指定实体集的时序指标数据，支持多种分析模式：
- basic: 返回原始时序数据（默认），支持时序对比
- cluster: K-Means 聚类分析
- forecast: 时序预测
- anomaly_detection: 异常检测

## 参数获取流程
1. 搜索实体集: umodel_search_entity_set(search_text="apm")
2. 列出指标集: umodel_list_data_set(data_set_types="metric_set") 获取 metric_domain_name 和 metric
3. 获取实体ID(可选): umodel_get_entities()
4. 执行查询

## 分析模式说明
- basic: 原始时序数据，支持 offset 参数进行时序对比
- cluster: 使用 K-Means 算法对时序数据进行聚类分析
- forecast: 基于历史数据进行时序预测，需指定 forecast_duration
- anomaly_detection: 检测时序数据中的异常点

## 使用场景
- 查询服务的CPU使用率、请求延迟等时序指标
- 对比不同时间段的指标数据（basic + offset）
- 发现指标数据的聚类模式（cluster）
- 预测未来指标趋势（forecast）
- 检测异常指标值（anomaly_detection）`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "实体域名，如'apm'、'host'。不能为'*'，可通过 umodel_search_entity_set 获取",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "实体类型名称，如'apm.service'。不能为'*'，可通过 umodel_search_entity_set 获取",
				},
				"metric_domain_name": map[string]interface{}{
					"type":        "string",
					"description": "指标域名称，如'apm.metric.jvm'。可通过 umodel_list_data_set(data_set_types='metric_set') 获取",
				},
				"metric": map[string]interface{}{
					"type":        "string",
					"description": "指标名称，如'cpu_usage'。可通过 umodel_list_data_set 返回的 fields 获取",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS工作空间名称，可通过 list_workspace 获取",
				},
				"entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "实体ID列表，逗号分隔，如'id1,id2,id3'。可通过 umodel_get_entities 获取",
				},
				"query_type": map[string]interface{}{
					"type":        "string",
					"description": "查询类型: range(范围查询) 或 instant(即时查询)",
					"default":     "range",
					"enum":        []string{"range", "instant"},
				},
				"aggregate": map[string]interface{}{
					"type":        "boolean",
					"description": "是否聚合结果，默认true",
					"default":     true,
				},
				"analysis_mode": map[string]interface{}{
					"type":        "string",
					"description": "分析模式: basic(原始数据), cluster(K-Means聚类), forecast(时序预测), anomaly_detection(异常检测)。默认: basic",
					"default":     "basic",
					"enum":        []string{"basic", "cluster", "forecast", "anomaly_detection"},
				},
				"forecast_duration": map[string]interface{}{
					"type":        "string",
					"description": "预测时长，仅在 analysis_mode='forecast' 时有效。如 '1h'（预测未来1小时）、'30m'（预测未来30分钟）。支持 s/m/h/d 单位",
				},
				"offset": map[string]interface{}{
					"type":        "string",
					"description": "对比时间偏移量，仅在 analysis_mode='basic' 时有效。如 '1d'（与1天前对比）、'1w'（与1周前对比）。支持 s/m/h/d/w 单位",
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": timeRangeDescription,
					"default":     "last_1h",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
				},
			},
			"required": []string{"domain", "entity_set_name", "metric_domain_name", "metric", "workspace", "regionId"},
		},
		Handler: h.handleGetMetrics,
	}
}

func (h *dataHandler) handleGetMetrics(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	domain := paramString(params, "domain", "")
	entitySetName := paramString(params, "entity_set_name", "")
	metricDomainName := paramString(params, "metric_domain_name", "")
	metric := paramString(params, "metric", "")
	workspace := paramString(params, "workspace", "")
	regionID := paramString(params, "regionId", "")
	entityIDs := paramString(params, "entity_ids", "")
	queryType := paramString(params, "query_type", "range")
	timeRange := paramString(params, "time_range", "last_1h")
	analysisMode := paramString(params, "analysis_mode", "basic")
	forecastDuration := paramString(params, "forecast_duration", "")
	offset := paramString(params, "offset", "")

	// Parse aggregate param (default true)
	aggregate := true
	if v, ok := params["aggregate"]; ok && v != nil {
		if b, ok := v.(bool); ok {
			aggregate = b
		}
	}

	if domain == "" || entitySetName == "" || metricDomainName == "" || metric == "" || workspace == "" || regionID == "" {
		return buildStandardResponse(nil, "", 0, 0, true,
			"domain, entity_set_name, metric_domain_name, metric, workspace and regionId are required", timeRange), nil
	}

	// Pre-validate metric_domain_name / entity_set_name compatibility.
	if hint := validateMetricCompatibility(entitySetName, metricDomainName); hint != "" {
		slog.WarnContext(ctx, "umodel_get_metrics: incompatible metric/entity combination",
			"entity_set_name", entitySetName, "metric_domain_name", metricDomainName)
		return buildStandardResponse(nil, "", 0, 0, true, hint, timeRange), nil
	}

	// Validate analysis_mode parameter
	validModes := map[string]bool{"basic": true, "cluster": true, "forecast": true, "anomaly_detection": true}
	if !validModes[analysisMode] {
		return buildStandardResponse(nil, "", 0, 0, true,
			fmt.Sprintf("invalid analysis_mode '%s', must be one of: basic, cluster, forecast, anomaly_detection", analysisMode), timeRange), nil
	}

	// Validate forecast_duration for forecast mode
	if analysisMode == "forecast" && forecastDuration == "" {
		return buildStandardResponse(nil, "", 0, 0, true,
			"forecast_duration is required when analysis_mode is 'forecast'", timeRange), nil
	}

	// Validate offset is only used with basic mode
	if offset != "" && analysisMode != "basic" {
		return buildStandardResponse(nil, "", 0, 0, true,
			"offset parameter is only valid when analysis_mode is 'basic'", timeRange), nil
	}

	fromTS, toTS, err := parseTimeRange(timeRange)
	if err != nil {
		return buildStandardResponse(nil, "", 0, 0, true, err.Error(), timeRange), nil
	}

	entityIDsParam := buildEntityIDsParam(entityIDs)
	stepParam := "''" // Auto step
	aggregateParam := "true"
	if !aggregate {
		aggregateParam = "false"
	}

	// Build base query for getting metrics
	baseQuery := fmt.Sprintf(
		".entity_set with(domain='%s', name='%s'%s) | entity-call get_metric('%s', '%s', '%s', '%s', %s, %s)",
		domain, entitySetName, entityIDsParam,
		metricDomainName, metric, queryType, stepParam, stepParam, aggregateParam,
	)

	var query string

	switch analysisMode {
	case "basic":
		// Basic mode: return raw time series data, optionally with comparison
		query = baseQuery

		slog.InfoContext(ctx, "umodel_get_metrics",
			"workspace", workspace, "domain", domain, "metric", metric,
			"analysis_mode", analysisMode, "region", regionID)

		// Check if offset is provided for comparison
		if offset != "" {
			offsetSeconds := ParseDurationToSeconds(offset)
			if offsetSeconds <= 0 {
				return buildStandardResponse(nil, "", 0, 0, true,
					fmt.Sprintf("invalid offset '%s', use format like '1d', '1w', '1h'", offset), timeRange), nil
			}

			// Calculate compare period
			compareFrom := fromTS - offsetSeconds
			compareTo := toTS - offsetSeconds

			// Query current period
			currentResult, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, 1000)
			if err != nil {
				slog.ErrorContext(ctx, "umodel_get_metrics current query failed", "error", err)
				return buildStandardResponse(nil, query, fromTS, toTS, true,
					fmt.Sprintf("Failed to query current period metrics: %s", err), timeRange), nil
			}

			// Query compare period
			compareResult, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, compareFrom, compareTo, 1000)
			if err != nil {
				slog.ErrorContext(ctx, "umodel_get_metrics compare query failed", "error", err)
				return buildStandardResponse(nil, query, fromTS, toTS, true,
					fmt.Sprintf("Failed to query compare period metrics: %s", err), timeRange), nil
			}

			// Parse time series data
			currentDataRaw, _ := toInterfaceSlice(currentResult["data"])
			compareDataRaw, _ := toInterfaceSlice(compareResult["data"])

			currentData := ParseTimeSeriesData(currentDataRaw, KeyTypeMetrics)
			compareData := ParseTimeSeriesData(compareDataRaw, KeyTypeMetrics)

			// Build comparison output
			output := BuildCompareOutput(
				currentData, compareData,
				fromTS, toTS, compareFrom, compareTo,
				offsetSeconds,
			)

			return buildStandardResponse(output, query, fromTS, toTS, false, "", timeRange), nil
		}

		// No offset - simple query
		result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, 1000)
		if err != nil {
			slog.ErrorContext(ctx, "umodel_get_metrics failed", "error", err)
			return buildStandardResponse(nil, query, fromTS, toTS, true,
				fmt.Sprintf("Failed to get metrics: %s", err), timeRange), nil
		}

		data := result["data"]
		return buildStandardResponse(data, query, fromTS, toTS, false, "", timeRange), nil

	case "cluster":
		// Cluster mode: K-Means clustering analysis
		query = fmt.Sprintf("%s | ml-call kmeans()", baseQuery)

		slog.InfoContext(ctx, "umodel_get_metrics",
			"workspace", workspace, "domain", domain, "metric", metric,
			"analysis_mode", analysisMode, "region", regionID)

		result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, 1000)
		if err != nil {
			slog.ErrorContext(ctx, "umodel_get_metrics cluster failed", "error", err)
			return buildStandardResponse(nil, query, fromTS, toTS, true,
				fmt.Sprintf("Failed to perform cluster analysis: %s", err), timeRange), nil
		}

		data := result["data"]
		return buildStandardResponse(data, query, fromTS, toTS, false, "", timeRange), nil

	case "forecast":
		// Forecast mode: time series prediction
		// Parse forecast duration to get the prediction period
		forecastSeconds := ParseDurationToSeconds(forecastDuration)
		if forecastSeconds <= 0 {
			return buildStandardResponse(nil, "", 0, 0, true,
				fmt.Sprintf("invalid forecast_duration '%s', use format like '1h', '30m', '1d'", forecastDuration), timeRange), nil
		}

		// Build forecast query with duration parameter
		query = fmt.Sprintf("%s | ml-call forecast('%s')", baseQuery, forecastDuration)

		slog.InfoContext(ctx, "umodel_get_metrics",
			"workspace", workspace, "domain", domain, "metric", metric,
			"analysis_mode", analysisMode, "forecast_duration", forecastDuration, "region", regionID)

		result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, 1000)
		if err != nil {
			slog.ErrorContext(ctx, "umodel_get_metrics forecast failed", "error", err)
			return buildStandardResponse(nil, query, fromTS, toTS, true,
				fmt.Sprintf("Failed to perform forecast analysis: %s", err), timeRange), nil
		}

		data := result["data"]
		return buildStandardResponse(data, query, fromTS, toTS, false, "", timeRange), nil

	case "anomaly_detection":
		// Anomaly detection mode: detect anomalies in time series
		query = fmt.Sprintf("%s | ml-call anomaly_detection()", baseQuery)

		slog.InfoContext(ctx, "umodel_get_metrics",
			"workspace", workspace, "domain", domain, "metric", metric,
			"analysis_mode", analysisMode, "region", regionID)

		result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, 1000)
		if err != nil {
			slog.ErrorContext(ctx, "umodel_get_metrics anomaly_detection failed", "error", err)
			return buildStandardResponse(nil, query, fromTS, toTS, true,
				fmt.Sprintf("Failed to perform anomaly detection: %s", err), timeRange), nil
		}

		data := result["data"]
		return buildStandardResponse(data, query, fromTS, toTS, false, "", timeRange), nil

	default:
		return buildStandardResponse(nil, "", 0, 0, true,
			fmt.Sprintf("unsupported analysis_mode '%s'", analysisMode), timeRange), nil
	}
}

// ===========================================================================
// Tool 2: umodel_get_golden_metrics
// ===========================================================================

func (h *dataHandler) getGoldenMetricsTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "umodel_get_golden_metrics",
		Description: `获取实体的黄金指标（关键性能指标）数据，支持时序对比。

## 功能概述
查询指定实体集的黄金指标数据，黄金指标是衡量服务健康状况的关键指标：
- 延迟(Latency): 请求响应时间
- 吞吐量(Throughput): 请求处理速率
- 错误率(Error Rate): 失败请求比例
- 饱和度(Saturation): 资源使用程度

## 参数获取流程
1. 搜索实体集: umodel_search_entity_set(search_text="apm")
2. 获取实体ID(可选): umodel_get_entities()
3. 执行查询

## 对比功能
支持 offset 参数进行时序对比，如 '1d' 表示与1天前的数据对比。

## 使用场景
- 快速了解服务的整体健康状况
- 监控服务的关键性能指标
- 对比不同时间段的性能变化`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "实体域名，如'apm'、'host'。不能为'*'，可通过 umodel_search_entity_set 获取",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "实体类型名称，如'apm.service'。不能为'*'，可通过 umodel_search_entity_set 获取",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS工作空间名称，可通过 list_workspace 获取",
				},
				"entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "实体ID列表，逗号分隔，如'id1,id2,id3'。可通过 umodel_get_entities 获取",
				},
				"query_type": map[string]interface{}{
					"type":        "string",
					"description": "查询类型: range(范围查询，返回时序数据) 或 instant(即时查询，返回最新值)",
					"default":     "range",
					"enum":        []string{"range", "instant"},
				},
				"aggregate": map[string]interface{}{
					"type":        "boolean",
					"description": "是否聚合结果，true 表示聚合所有实体的结果，false 表示返回每个实体的独立结果",
					"default":     true,
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": timeRangeDescription,
					"default":     "last_1h",
				},
				"offset": map[string]interface{}{
					"type":        "string",
					"description": "对比偏移量，如'1h','1d','1w'。启用后会执行两次查询（当前时段和对比时段），返回对比分析结果",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
				},
			},
			"required": []string{"domain", "entity_set_name", "workspace", "regionId"},
		},
		Handler: h.handleGetGoldenMetrics,
	}
}

func (h *dataHandler) handleGetGoldenMetrics(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	domain := paramString(params, "domain", "")
	entitySetName := paramString(params, "entity_set_name", "")
	workspace := paramString(params, "workspace", "")
	regionID := paramString(params, "regionId", "")
	entityIDs := paramString(params, "entity_ids", "")
	queryType := paramString(params, "query_type", "range")
	timeRange := paramString(params, "time_range", "last_1h")
	offset := paramString(params, "offset", "")

	// Parse aggregate param (default true)
	aggregate := true
	if v, ok := params["aggregate"]; ok && v != nil {
		if b, ok := v.(bool); ok {
			aggregate = b
		}
	}

	if domain == "" || entitySetName == "" || workspace == "" || regionID == "" {
		return buildStandardResponse(nil, "", 0, 0, true,
			"domain, entity_set_name, workspace and regionId are required", timeRange), nil
	}

	// Validate query_type parameter
	if queryType != "range" && queryType != "instant" {
		return buildStandardResponse(nil, "", 0, 0, true,
			fmt.Sprintf("invalid query_type '%s', must be one of: range, instant", queryType), timeRange), nil
	}

	fromTS, toTS, err := parseTimeRange(timeRange)
	if err != nil {
		return buildStandardResponse(nil, "", 0, 0, true, err.Error(), timeRange), nil
	}

	entityIDsParam := buildEntityIDsParam(entityIDs)
	stepParam := "''" // Auto step
	aggregateParam := "true"
	if !aggregate {
		aggregateParam = "false"
	}

	// Build SPL query: .entity_set with(...) | entity-call get_golden_metrics(query_type, step, aggregate)
	query := fmt.Sprintf(
		".entity_set with(domain='%s', name='%s'%s) | entity-call get_golden_metrics('%s', %s, %s)",
		domain, entitySetName, entityIDsParam,
		queryType, stepParam, aggregateParam,
	)

	slog.InfoContext(ctx, "umodel_get_golden_metrics",
		"workspace", workspace, "domain", domain, "entity_set_name", entitySetName,
		"query_type", queryType, "aggregate", aggregate, "region", regionID)

	// Check if offset is provided for comparison
	if offset != "" {
		offsetSeconds := ParseDurationToSeconds(offset)
		if offsetSeconds <= 0 {
			return buildStandardResponse(nil, "", 0, 0, true,
				fmt.Sprintf("invalid offset '%s', use format like '1d', '1w', '1h'", offset), timeRange), nil
		}

		// Calculate compare period
		compareFrom := fromTS - offsetSeconds
		compareTo := toTS - offsetSeconds

		// Query current period
		currentResult, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, 1000)
		if err != nil {
			slog.ErrorContext(ctx, "umodel_get_golden_metrics current query failed", "error", err)
			return buildStandardResponse(nil, query, fromTS, toTS, true,
				fmt.Sprintf("Failed to query current period golden metrics: %s", err), timeRange), nil
		}

		// Query compare period
		compareResult, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, compareFrom, compareTo, 1000)
		if err != nil {
			slog.ErrorContext(ctx, "umodel_get_golden_metrics compare query failed", "error", err)
			return buildStandardResponse(nil, query, fromTS, toTS, true,
				fmt.Sprintf("Failed to query compare period golden metrics: %s", err), timeRange), nil
		}

		// Parse time series data using KeyTypeGoldenMetrics for proper key extraction
		currentDataRaw, _ := toInterfaceSlice(currentResult["data"])
		compareDataRaw, _ := toInterfaceSlice(compareResult["data"])

		currentData := ParseTimeSeriesData(currentDataRaw, KeyTypeGoldenMetrics)
		compareData := ParseTimeSeriesData(compareDataRaw, KeyTypeGoldenMetrics)

		// Build comparison output
		output := BuildCompareOutput(
			currentData, compareData,
			fromTS, toTS, compareFrom, compareTo,
			offsetSeconds,
		)

		return buildStandardResponse(output, query, fromTS, toTS, false, "", timeRange), nil
	}

	// No offset - simple query
	result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, 1000)
	if err != nil {
		slog.ErrorContext(ctx, "umodel_get_golden_metrics failed", "error", err)
		return buildStandardResponse(nil, query, fromTS, toTS, true,
			fmt.Sprintf("Failed to get golden metrics: %s", err), timeRange), nil
	}

	data := result["data"]
	return buildStandardResponse(data, query, fromTS, toTS, false, "", timeRange), nil
}

// ===========================================================================
// Tool 3: umodel_get_relation_metrics
// ===========================================================================

func (h *dataHandler) getRelationMetricsTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "umodel_get_relation_metrics",
		Description: `获取实体间关系级别的指标数据。

## 功能概述
查询指定实体集之间关系的指标数据，如服务间调用的延迟、吞吐量等。
支持上游、下游和双向关系查询。

## 参数获取流程
1. 搜索实体集: umodel_search_entity_set(search_text="apm")
2. 列出指标集: umodel_list_data_set(data_set_types="metric_set") 获取 metric_set_domain 和 metric
3. 获取实体ID(可选): umodel_get_entities()
4. 执行查询

## 方向说明
- "upstream": 上游关系（调用方）
- "downstream": 下游关系（被调用方）
- "both": 双向关系（默认）

## 使用场景
- 查询服务间调用的延迟指标
- 分析上下游依赖的性能数据
- 监控服务间通信的健康状况`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"src_domain": map[string]interface{}{
					"type":        "string",
					"description": "源实体域名，如'apm'、'host'。不能为'*'，可通过 umodel_search_entity_set 获取",
				},
				"src_entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "源实体类型名称，如'apm.service'。不能为'*'，可通过 umodel_search_entity_set 获取",
				},
				"src_entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "源实体ID列表，逗号分隔，如'id1,id2,id3'。可通过 umodel_get_entities 获取",
				},
				"relation_type": map[string]interface{}{
					"type":        "string",
					"description": "关系类型，如'calls'。用于自动拼接 metric_set_name",
				},
				"direction": map[string]interface{}{
					"type":        "string",
					"description": "关系方向: upstream(上游), downstream(下游), both(双向)。默认: both",
					"default":     "both",
					"enum":        []string{"upstream", "downstream", "both"},
				},
				"metric_set_domain": map[string]interface{}{
					"type":        "string",
					"description": "指标集域名，如'apm'。可通过 umodel_list_data_set(data_set_types='metric_set') 获取",
				},
				"metric_set_name": map[string]interface{}{
					"type":        "string",
					"description": "指标集名称。如果不提供，将自动拼接为 relation_type + '.' + src_entity_set_name",
				},
				"metric": map[string]interface{}{
					"type":        "string",
					"description": "指标名称，如'latency'、'throughput'。可通过 umodel_list_data_set 返回的 fields 获取",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS工作空间名称，可通过 list_workspace 获取",
				},
				"dest_domain": map[string]interface{}{
					"type":        "string",
					"description": "目标实体域名（可选），用于过滤特定目标域",
				},
				"dest_entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "目标实体类型名称（可选），用于过滤特定目标类型",
				},
				"dest_entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "目标实体ID列表（可选），逗号分隔",
				},
				"query_type": map[string]interface{}{
					"type":        "string",
					"description": "查询类型: range(范围查询) 或 instant(即时查询)",
					"default":     "range",
					"enum":        []string{"range", "instant"},
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": timeRangeDescription,
					"default":     "last_1h",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
				},
			},
			"required": []string{"src_domain", "src_entity_set_name", "relation_type", "metric_set_domain", "metric", "workspace", "regionId"},
		},
		Handler: h.handleGetRelationMetrics,
	}
}

func (h *dataHandler) handleGetRelationMetrics(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	srcDomain := paramString(params, "src_domain", "")
	srcEntitySetName := paramString(params, "src_entity_set_name", "")
	srcEntityIDs := paramString(params, "src_entity_ids", "")
	relationType := paramString(params, "relation_type", "")
	direction := paramString(params, "direction", "both")
	metricSetDomain := paramString(params, "metric_set_domain", "")
	metricSetName := paramString(params, "metric_set_name", "")
	metric := paramString(params, "metric", "")
	workspace := paramString(params, "workspace", "")
	regionID := paramString(params, "regionId", "")
	destDomain := paramString(params, "dest_domain", "")
	destEntitySetName := paramString(params, "dest_entity_set_name", "")
	destEntityIDs := paramString(params, "dest_entity_ids", "")
	queryType := paramString(params, "query_type", "range")
	timeRange := paramString(params, "time_range", "last_1h")

	// Validate required parameters
	if srcDomain == "" || srcEntitySetName == "" || relationType == "" || metricSetDomain == "" || metric == "" || workspace == "" || regionID == "" {
		return buildStandardResponse(nil, "", 0, 0, true,
			"src_domain, src_entity_set_name, relation_type, metric_set_domain, metric, workspace and regionId are required", timeRange), nil
	}

	// Validate direction parameter
	validDirections := map[string]bool{"upstream": true, "downstream": true, "both": true}
	if !validDirections[direction] {
		return buildStandardResponse(nil, "", 0, 0, true,
			fmt.Sprintf("invalid direction '%s', must be one of: upstream, downstream, both", direction), timeRange), nil
	}

	// Validate query_type parameter
	if queryType != "range" && queryType != "instant" {
		return buildStandardResponse(nil, "", 0, 0, true,
			fmt.Sprintf("invalid query_type '%s', must be one of: range, instant", queryType), timeRange), nil
	}

	fromTS, toTS, err := parseTimeRange(timeRange)
	if err != nil {
		return buildStandardResponse(nil, "", 0, 0, true, err.Error(), timeRange), nil
	}

	// Auto-generate metric_set_name if not provided: relation_type + "." + src_entity_set_name
	if metricSetName == "" {
		metricSetName = relationType + "." + srcEntitySetName
	}

	// Build source entity IDs parameter
	srcEntityIDsParam := buildEntityIDsParam(srcEntityIDs)

	// Build destination parameters
	destDomainParam := parseStringToSPLParam(destDomain)
	destEntitySetNameParam := parseStringToSPLParam(destEntitySetName)
	destEntityIDsParam := "[]"
	if destEntityIDs != "" {
		destEntityIDsParam = parseEntityIDsToSPLParam(destEntityIDs)
	}

	// Build SPL query: .entity_set with(...) | entity-call get_relation_metric(...)
	// Parameters: metric_set_domain, metric_set_name, metric, query_type, step, dest_domain, dest_name, dest_ids, direction
	stepParam := "''" // Auto step
	query := fmt.Sprintf(
		".entity_set with(domain='%s', name='%s'%s) | entity-call get_relation_metric('%s', '%s', '%s', '%s', %s, %s, %s, %s, '%s')",
		srcDomain, srcEntitySetName, srcEntityIDsParam,
		metricSetDomain, metricSetName, metric, queryType, stepParam,
		destDomainParam, destEntitySetNameParam, destEntityIDsParam, direction,
	)

	slog.InfoContext(ctx, "umodel_get_relation_metrics",
		"workspace", workspace, "src_domain", srcDomain, "src_entity_set_name", srcEntitySetName,
		"relation_type", relationType, "metric", metric, "direction", direction, "region", regionID)

	result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, 1000)
	if err != nil {
		slog.ErrorContext(ctx, "umodel_get_relation_metrics failed", "error", err)
		return buildStandardResponse(nil, query, fromTS, toTS, true,
			fmt.Sprintf("Failed to get relation metrics: %s", err), timeRange), nil
	}

	data := result["data"]
	return buildStandardResponse(data, query, fromTS, toTS, false, "", timeRange), nil
}

// ===========================================================================
// Tool 4: umodel_get_logs
// ===========================================================================

func (h *dataHandler) getLogsTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "umodel_get_logs",
		Description: `获取实体相关的日志数据，支持原始日志查询和日志聚类分析。

## 功能概述
查询指定实体集的日志数据，支持原始日志查询和基于字段的日志聚类分析。
日志聚类可自动识别日志模式，将相似日志归类，帮助快速发现问题。

## 参数获取流程
1. 搜索实体集: umodel_search_entity_set(search_text="apm")
2. 列出日志集: umodel_list_data_set(data_set_types="log_set") 获取 log_set_domain 和 log_set_name
3. 获取实体ID(可选但推荐): umodel_get_entities()
4. 执行查询

## 日志聚类说明
当提供 to_cluster_content_field 参数时，启用日志聚类分析：
- 系统会自动识别日志模式，将相似日志归类
- 返回每个模式的统计信息：模式ID、模式模板、事件数量、时间范围、样本数据
- 可选提供 to_cluster_aggregate_field 进一步按字段分组

## 使用场景
- 查询服务的错误日志
- 日志聚类分析，发现异常模式
- 故障诊断和性能分析`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "实体域名，如'apm'、'host'。不能为'*'",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "实体类型名称，如'apm.service'。不能为'*'",
				},
				"log_set_domain": map[string]interface{}{
					"type":        "string",
					"description": "日志集域名，如'apm'。可通过 umodel_list_data_set(data_set_types='log_set') 获取",
				},
				"log_set_name": map[string]interface{}{
					"type":        "string",
					"description": "日志集名称，如'apm.log.apm.service'。可通过 umodel_list_data_set 获取",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS工作空间名称，可通过 list_workspace 获取",
				},
				"entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "实体ID列表，逗号分隔。可通过 umodel_get_entities 获取（强烈推荐提供）",
				},
				"to_cluster_content_field": map[string]interface{}{
					"type":        "string",
					"description": "日志聚类字段，如'content'、'message'。提供此参数时启用日志聚类分析，返回日志模式而非原始日志",
				},
				"to_cluster_aggregate_field": map[string]interface{}{
					"type":        "string",
					"description": "聚类聚合字段，如'severity'、'level'。用于在聚类结果中按此字段进一步分组统计",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "返回的最大日志记录数量，默认100",
					"default":     100,
					"minimum":     1,
					"maximum":     1000,
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": timeRangeDescription,
					"default":     "last_1h",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID",
				},
			},
			"required": []string{"domain", "entity_set_name", "log_set_domain", "log_set_name", "workspace", "regionId"},
		},
		Handler: h.handleGetLogs,
	}
}

func (h *dataHandler) handleGetLogs(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	domain := paramString(params, "domain", "")
	entitySetName := paramString(params, "entity_set_name", "")
	logSetDomain := paramString(params, "log_set_domain", "")
	logSetName := paramString(params, "log_set_name", "")
	workspace := paramString(params, "workspace", "")
	regionID := paramString(params, "regionId", "")
	entityIDs := paramString(params, "entity_ids", "")
	toClusterContentField := paramString(params, "to_cluster_content_field", "")
	toClusterAggregateField := paramString(params, "to_cluster_aggregate_field", "")
	timeRange := paramString(params, "time_range", "last_1h")
	limit := paramInt(params, "limit", 100)

	if domain == "" || entitySetName == "" || logSetDomain == "" || logSetName == "" || workspace == "" || regionID == "" {
		return buildStandardResponse(nil, "", 0, 0, true,
			"domain, entity_set_name, log_set_domain, log_set_name, workspace and regionId are required", timeRange), nil
	}

	fromTS, toTS, err := parseTimeRange(timeRange)
	if err != nil {
		return buildStandardResponse(nil, "", 0, 0, true, err.Error(), timeRange), nil
	}

	entityIDsParam := buildEntityIDsParam(entityIDs)

	var query string
	if toClusterContentField != "" {
		// Log clustering mode
		if toClusterAggregateField != "" {
			// Clustering with aggregate field
			query = fmt.Sprintf(
				".entity_set with(domain='%s', name='%s'%s) | entity-call get_log('%s', '%s') | ml-call log_cluster('%s', '%s')",
				domain, entitySetName, entityIDsParam,
				logSetDomain, logSetName,
				toClusterContentField, toClusterAggregateField,
			)
		} else {
			// Clustering without aggregate field
			query = fmt.Sprintf(
				".entity_set with(domain='%s', name='%s'%s) | entity-call get_log('%s', '%s') | ml-call log_cluster('%s')",
				domain, entitySetName, entityIDsParam,
				logSetDomain, logSetName,
				toClusterContentField,
			)
		}
	} else {
		// Raw log query mode
		query = fmt.Sprintf(
			".entity_set with(domain='%s', name='%s'%s) | entity-call get_log('%s', '%s')",
			domain, entitySetName, entityIDsParam,
			logSetDomain, logSetName,
		)
	}

	slog.InfoContext(ctx, "umodel_get_logs",
		"workspace", workspace, "domain", domain, "log_set_name", logSetName, "region", regionID)

	result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, limit)
	if err != nil {
		slog.ErrorContext(ctx, "umodel_get_logs failed", "error", err)
		return buildStandardResponse(nil, query, fromTS, toTS, true,
			fmt.Sprintf("Failed to get logs: %s", err), timeRange), nil
	}

	data := result["data"]
	return buildStandardResponse(data, query, fromTS, toTS, false, "", timeRange), nil
}

// ===========================================================================
// Tool 4: umodel_get_events
// ===========================================================================

func (h *dataHandler) getEventsTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "umodel_get_events",
		Description: `获取指定实体集的事件数据。

## 功能概述
查询指定实体集的事件数据，事件是离散记录，如部署、告警、配置更改等。
用于关联分析系统行为变化。

## 参数获取流程
1. 搜索实体集: umodel_search_entity_set(search_text="apm")
2. 列出事件集: umodel_list_data_set(data_set_types="event_set") 或使用默认值 "default"/"default.event.common"
3. 获取实体ID(可选): umodel_get_entities()
4. 执行查询

## 使用场景
- 查看服务的部署事件
- 关联告警事件与指标异常
- 分析配置变更对系统的影响`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "实体域名，如'apm'、'host'。不能为'*'",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "实体类型名称，如'apm.service'。不能为'*'",
				},
				"event_set_domain": map[string]interface{}{
					"type":        "string",
					"description": "事件集域名，如'default'。可通过 umodel_list_data_set(data_set_types='event_set') 获取",
				},
				"event_set_name": map[string]interface{}{
					"type":        "string",
					"description": "事件集名称，如'default.event.common'。可通过 umodel_list_data_set 获取",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS工作空间名称，可通过 list_workspace 获取",
				},
				"entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "实体ID列表，逗号分隔。可通过 umodel_get_entities 获取",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "返回的最大事件记录数量，默认100",
					"default":     100,
					"minimum":     1,
					"maximum":     1000,
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": timeRangeDescription,
					"default":     "last_1h",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID",
				},
			},
			"required": []string{"domain", "entity_set_name", "event_set_domain", "event_set_name", "workspace", "regionId"},
		},
		Handler: h.handleGetEvents,
	}
}

func (h *dataHandler) handleGetEvents(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	domain := paramString(params, "domain", "")
	entitySetName := paramString(params, "entity_set_name", "")
	eventSetDomain := paramString(params, "event_set_domain", "")
	eventSetName := paramString(params, "event_set_name", "")
	workspace := paramString(params, "workspace", "")
	regionID := paramString(params, "regionId", "")
	entityIDs := paramString(params, "entity_ids", "")
	timeRange := paramString(params, "time_range", "last_1h")
	limit := paramInt(params, "limit", 100)

	if domain == "" || entitySetName == "" || eventSetDomain == "" || eventSetName == "" || workspace == "" || regionID == "" {
		return buildStandardResponse(nil, "", 0, 0, true,
			"domain, entity_set_name, event_set_domain, event_set_name, workspace and regionId are required", timeRange), nil
	}

	fromTS, toTS, err := parseTimeRange(timeRange)
	if err != nil {
		return buildStandardResponse(nil, "", 0, 0, true, err.Error(), timeRange), nil
	}

	entityIDsParam := buildEntityIDsParam(entityIDs)

	query := fmt.Sprintf(
		".entity_set with(domain='%s', name='%s'%s) | entity-call get_event('%s', '%s')",
		domain, entitySetName, entityIDsParam,
		eventSetDomain, eventSetName,
	)

	slog.InfoContext(ctx, "umodel_get_events",
		"workspace", workspace, "domain", domain, "event_set_name", eventSetName, "region", regionID)

	result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, limit)
	if err != nil {
		slog.ErrorContext(ctx, "umodel_get_events failed", "error", err)
		return buildStandardResponse(nil, query, fromTS, toTS, true,
			fmt.Sprintf("Failed to get events: %s", err), timeRange), nil
	}

	data := result["data"]
	return buildStandardResponse(data, query, fromTS, toTS, false, "", timeRange), nil
}

// ===========================================================================
// Tool 5: umodel_get_traces
// ===========================================================================

func (h *dataHandler) getTracesTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "umodel_get_traces",
		Description: `获取指定trace ID的详细trace数据，包括所有span和元数据。

## 功能概述
根据trace ID获取详细的trace数据，包括所有span、耗时和元数据。
用于深入分析慢trace和错误trace。

## 参数获取流程
1. 搜索实体集: umodel_search_entity_set(search_text="apm")
2. 列出TraceSet: umodel_list_data_set(data_set_types="trace_set") 获取 trace_set_domain 和 trace_set_name
3. 获取trace ID: 通常从 umodel_search_traces 工具输出中获得
4. 执行查询

## 输出字段说明
- duration_ms: span总耗时（毫秒）
- exclusive_duration_ms: span独占耗时（毫秒）

## 使用场景
- 分析慢请求的调用链路
- 定位错误trace的根因
- 查看span级别的详细信息`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "实体域名，如'apm'、'host'。不能为'*'",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "实体类型名称，如'apm.service'。不能为'*'",
				},
				"trace_set_domain": map[string]interface{}{
					"type":        "string",
					"description": "TraceSet域名，如'apm'。可通过 umodel_list_data_set(data_set_types='trace_set') 获取",
				},
				"trace_set_name": map[string]interface{}{
					"type":        "string",
					"description": "TraceSet名称，如'apm.trace.common'。可通过 umodel_list_data_set 获取",
				},
				"trace_ids": map[string]interface{}{
					"type":        "string",
					"description": "逗号分隔的trace ID列表，如'trace1,trace2'。通常从 umodel_search_traces 获取",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS工作空间名称，可通过 list_workspace 获取",
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": timeRangeDescription,
					"default":     "last_1h",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID",
				},
			},
			"required": []string{"domain", "entity_set_name", "trace_set_domain", "trace_set_name", "trace_ids", "workspace", "regionId"},
		},
		Handler: h.handleGetTraces,
	}
}

func (h *dataHandler) handleGetTraces(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	domain := paramString(params, "domain", "")
	entitySetName := paramString(params, "entity_set_name", "")
	traceSetDomain := paramString(params, "trace_set_domain", "")
	traceSetName := paramString(params, "trace_set_name", "")
	traceIDs := paramString(params, "trace_ids", "")
	workspace := paramString(params, "workspace", "")
	regionID := paramString(params, "regionId", "")
	timeRange := paramString(params, "time_range", "last_1h")

	if domain == "" || entitySetName == "" || traceSetDomain == "" || traceSetName == "" || traceIDs == "" || workspace == "" || regionID == "" {
		return buildStandardResponse(nil, "", 0, 0, true,
			"domain, entity_set_name, trace_set_domain, trace_set_name, trace_ids, workspace and regionId are required", timeRange), nil
	}

	fromTS, toTS, err := parseTimeRange(timeRange)
	if err != nil {
		return buildStandardResponse(nil, "", 0, 0, true, err.Error(), timeRange), nil
	}

	// Build trace ID filter: traceId='id1' or traceId='id2'
	parts := splitAndTrim(traceIDs)
	quotedFilters := make([]string, 0, len(parts))
	for _, id := range parts {
		quotedFilters = append(quotedFilters, fmt.Sprintf("traceId='%s'", id))
	}
	traceIDFilter := ""
	if len(quotedFilters) > 0 {
		traceIDFilter = " | where "
		for i, f := range quotedFilters {
			if i > 0 {
				traceIDFilter += " or "
			}
			traceIDFilter += f
		}
	}

	// Build multi-step query with exclusive duration calculation (ported from Python)
	query := fmt.Sprintf(
		`.let trace_data = .entity_set with(domain='%s', name='%s') | entity-call get_trace('%s', '%s')%s | extend duration_ms = cast(duration as double) / 1000000;

.let trace_data_with_time = $trace_data
| extend startTime=cast(startTime as bigint), duration=cast(duration as bigint)
| extend endTime = startTime + duration
| make-trace
    traceId=traceId,
    spanId=spanId,
    parentSpanId=parentSpanId,
    statusCode=statusCode,
    startTime=startTime,
    endTime=endTime | extend span_list_with_exclusive = trace_exclusive_duration(traceRow.traceId, traceRow.spanList)
| extend span_id = span_list_with_exclusive.span_id, span_index = span_list_with_exclusive.span_index, exclusive_duration = span_list_with_exclusive.exclusive_duration
| extend __trace_id__ = traceRow.traceId | project __trace_id__, span_id, exclusive_duration | unnest | extend exclusive_duration_ms = cast(exclusive_duration as double) / 1000000;

$trace_data | join $trace_data_with_time on $trace_data_with_time.__trace_id__ = traceId and $trace_data_with_time.span_id = spanId | project-away duration, exclusive_duration | sort traceId desc, exclusive_duration_ms desc, duration_ms desc | limit 1000`,
		domain, entitySetName, traceSetDomain, traceSetName, traceIDFilter,
	)

	slog.InfoContext(ctx, "umodel_get_traces",
		"workspace", workspace, "domain", domain, "trace_ids", traceIDs, "region", regionID)

	result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, 1000)
	if err != nil {
		slog.ErrorContext(ctx, "umodel_get_traces failed", "error", err)
		return buildStandardResponse(nil, query, fromTS, toTS, true,
			fmt.Sprintf("Failed to get traces: %s", err), timeRange), nil
	}

	data := result["data"]
	return buildStandardResponse(data, query, fromTS, toTS, false, "", timeRange), nil
}

// ===========================================================================
// Tool 6: umodel_get_profiling
// ===========================================================================

func (h *dataHandler) getProfilingTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "umodel_get_profiling",
		Description: `获取指定实体集的性能剖析数据。

## 功能概述
查询指定实体集的性能剖析数据，包括CPU使用、内存分配、方法调用堆栈等。
用于性能瓶颈分析和代码级优化。

## 参数获取流程
1. 搜索实体集: umodel_search_entity_set(search_text="apm")
2. 列出ProfileSet: umodel_list_data_set(data_set_types="profile_set") 获取 profile_set_domain 和 profile_set_name
3. 获取实体ID(必须): umodel_get_entities()
4. 执行查询

## 使用场景
- 分析CPU密集型代码路径
- 发现内存泄漏和高内存消耗点
- 分析调用链和热点函数`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "实体域名，如'apm'、'host'。不能为'*'",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "实体类型名称，如'apm.service'。不能为'*'",
				},
				"profile_set_domain": map[string]interface{}{
					"type":        "string",
					"description": "ProfileSet域名，如'default'。可通过 umodel_list_data_set(data_set_types='profile_set') 获取",
				},
				"profile_set_name": map[string]interface{}{
					"type":        "string",
					"description": "ProfileSet名称，如'default.profile.common'。可通过 umodel_list_data_set 获取",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS工作空间名称，可通过 list_workspace 获取",
				},
				"entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "实体ID列表，逗号分隔（必填，数据量大需指定精确实体）。可通过 umodel_get_entities 获取",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "返回的最大性能剖析记录数量，默认100",
					"default":     100,
					"minimum":     1,
					"maximum":     1000,
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": timeRangeDescription,
					"default":     "last_5m",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID",
				},
			},
			"required": []string{"domain", "entity_set_name", "profile_set_domain", "profile_set_name", "workspace", "entity_ids", "regionId"},
		},
		Handler: h.handleGetProfiling,
	}
}

func (h *dataHandler) handleGetProfiling(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	domain := paramString(params, "domain", "")
	entitySetName := paramString(params, "entity_set_name", "")
	profileSetDomain := paramString(params, "profile_set_domain", "")
	profileSetName := paramString(params, "profile_set_name", "")
	workspace := paramString(params, "workspace", "")
	regionID := paramString(params, "regionId", "")
	entityIDs := paramString(params, "entity_ids", "")
	timeRange := paramString(params, "time_range", "last_5m")
	limit := paramInt(params, "limit", 100)

	if domain == "" || entitySetName == "" || profileSetDomain == "" || profileSetName == "" || workspace == "" || entityIDs == "" || regionID == "" {
		return buildStandardResponse(nil, "", 0, 0, true,
			"domain, entity_set_name, profile_set_domain, profile_set_name, workspace, entity_ids and regionId are required", timeRange), nil
	}

	fromTS, toTS, err := parseTimeRange(timeRange)
	if err != nil {
		return buildStandardResponse(nil, "", 0, 0, true, err.Error(), timeRange), nil
	}

	entityIDsParam := buildEntityIDsParam(entityIDs)

	query := fmt.Sprintf(
		".entity_set with(domain='%s', name='%s'%s) | entity-call get_profile('%s', '%s')",
		domain, entitySetName, entityIDsParam,
		profileSetDomain, profileSetName,
	)

	slog.InfoContext(ctx, "umodel_get_profiling",
		"workspace", workspace, "domain", domain, "profile_set_name", profileSetName, "region", regionID)

	result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, limit)
	if err != nil {
		slog.ErrorContext(ctx, "umodel_get_profiling failed", "error", err)
		return buildStandardResponse(nil, query, fromTS, toTS, true,
			fmt.Sprintf("Failed to get profiling data: %s", err), timeRange), nil
	}

	data := result["data"]
	return buildStandardResponse(data, query, fromTS, toTS, false, "", timeRange), nil
}

// ===========================================================================
// Tool 7: umodel_search_traces
// ===========================================================================

func (h *dataHandler) searchTracesTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "umodel_search_traces",
		Description: `基于过滤条件搜索trace并返回摘要信息。

## 功能概述
支持按持续时间、错误状态、实体ID过滤，返回traceID用于详细分析。
搜索结果包含trace摘要信息：traceId、duration_ms、span_count、error_span_count。

## 参数获取流程
1. 搜索实体集: umodel_search_entity_set(search_text="apm")
2. 列出TraceSet: umodel_list_data_set(data_set_types="trace_set") 获取 trace_set_domain 和 trace_set_name
3. 获取实体ID(可选): umodel_get_entities()
4. 执行搜索

## 过滤条件
- min_duration_ms: 最小trace持续时间（毫秒），用于过滤慢trace
- max_duration_ms: 最大trace持续时间（毫秒），用于过滤快trace
- has_error: 按错误状态过滤（true表示错误trace）

## 使用场景
- 搜索慢trace（持续时间超过阈值）
- 搜索错误trace
- 搜索特定实体的trace`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "实体域名，如'apm'、'host'。不能为'*'",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "实体类型名称，如'apm.service'。不能为'*'",
				},
				"trace_set_domain": map[string]interface{}{
					"type":        "string",
					"description": "TraceSet域名，如'apm'。可通过 umodel_list_data_set(data_set_types='trace_set') 获取",
				},
				"trace_set_name": map[string]interface{}{
					"type":        "string",
					"description": "TraceSet名称，如'apm.trace.common'。可通过 umodel_list_data_set 获取",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS工作空间名称，可通过 list_workspace 获取",
				},
				"entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "实体ID列表，逗号分隔，如'id1,id2,id3'。可通过 umodel_get_entities 获取",
				},
				"min_duration_ms": map[string]interface{}{
					"type":        "number",
					"description": "最小trace持续时间（毫秒），用于过滤慢trace",
				},
				"max_duration_ms": map[string]interface{}{
					"type":        "number",
					"description": "最大trace持续时间（毫秒），用于过滤快trace",
				},
				"has_error": map[string]interface{}{
					"type":        "boolean",
					"description": "按错误状态过滤（true表示错误trace，false表示成功trace）",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "返回的最大trace摘要数量，默认100",
					"default":     100,
					"minimum":     1,
					"maximum":     1000,
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": timeRangeDescription,
					"default":     "last_1h",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID",
				},
			},
			"required": []string{"domain", "entity_set_name", "trace_set_domain", "trace_set_name", "workspace", "regionId"},
		},
		Handler: h.handleSearchTraces,
	}
}

func (h *dataHandler) handleSearchTraces(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	domain := paramString(params, "domain", "")
	entitySetName := paramString(params, "entity_set_name", "")
	traceSetDomain := paramString(params, "trace_set_domain", "")
	traceSetName := paramString(params, "trace_set_name", "")
	workspace := paramString(params, "workspace", "")
	regionID := paramString(params, "regionId", "")
	entityIDs := paramString(params, "entity_ids", "")
	timeRange := paramString(params, "time_range", "last_1h")
	limit := paramInt(params, "limit", 100)

	// Validate required parameters
	if domain == "" || entitySetName == "" || traceSetDomain == "" || traceSetName == "" || workspace == "" || regionID == "" {
		return buildStandardResponse(nil, "", 0, 0, true,
			"domain, entity_set_name, trace_set_domain, trace_set_name, workspace and regionId are required", timeRange), nil
	}

	fromTS, toTS, err := parseTimeRange(timeRange)
	if err != nil {
		return buildStandardResponse(nil, "", 0, 0, true, err.Error(), timeRange), nil
	}

	// Build entity IDs parameter
	entityIDsParam := buildEntityIDsParam(entityIDs)

	// Build filter conditions
	var filterParams []string

	// min_duration_ms filter (duration is in nanoseconds, convert ms to ns)
	if v, ok := params["min_duration_ms"]; ok && v != nil {
		var minDurationMs float64
		switch val := v.(type) {
		case float64:
			minDurationMs = val
		case int:
			minDurationMs = float64(val)
		case int64:
			minDurationMs = float64(val)
		}
		if minDurationMs > 0 {
			minDurationNs := int64(minDurationMs * 1000000)
			filterParams = append(filterParams, fmt.Sprintf("cast(duration as bigint) > %d", minDurationNs))
		}
	}

	// max_duration_ms filter
	if v, ok := params["max_duration_ms"]; ok && v != nil {
		var maxDurationMs float64
		switch val := v.(type) {
		case float64:
			maxDurationMs = val
		case int:
			maxDurationMs = float64(val)
		case int64:
			maxDurationMs = float64(val)
		}
		if maxDurationMs > 0 {
			maxDurationNs := int64(maxDurationMs * 1000000)
			filterParams = append(filterParams, fmt.Sprintf("cast(duration as bigint) < %d", maxDurationNs))
		}
	}

	// has_error filter (statusCode = '2' means error)
	if v, ok := params["has_error"]; ok && v != nil {
		if hasError, ok := v.(bool); ok && hasError {
			filterParams = append(filterParams, "cast(statusCode as varchar) = '2'")
		}
	}

	// Build filter string
	filterParamStr := ""
	if len(filterParams) > 0 {
		filterParamStr = "| where "
		for i, f := range filterParams {
			if i > 0 {
				filterParamStr += " and "
			}
			filterParamStr += f
		}
	}

	// Build stats string for aggregation
	statsStr := "| extend duration_ms = cast(duration as double) / 1000000, is_error = case when cast(statusCode as varchar) = '2' then 1 else 0 end | stats span_count = count(1), error_span_count = sum(is_error), duration_ms = max(duration_ms) by traceId | sort duration_ms desc, error_span_count desc | project traceId, duration_ms, span_count, error_span_count"

	// Build the complete SPL query
	query := fmt.Sprintf(
		".entity_set with(domain='%s', name='%s'%s) | entity-call get_trace('%s', '%s') %s %s | limit %d",
		domain, entitySetName, entityIDsParam,
		traceSetDomain, traceSetName,
		filterParamStr, statsStr, limit,
	)

	slog.InfoContext(ctx, "umodel_search_traces",
		"workspace", workspace, "domain", domain, "entity_set_name", entitySetName,
		"trace_set_domain", traceSetDomain, "trace_set_name", traceSetName,
		"limit", limit, "region", regionID)

	result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, 1000)
	if err != nil {
		slog.ErrorContext(ctx, "umodel_search_traces failed", "error", err)
		return buildStandardResponse(nil, query, fromTS, toTS, true,
			fmt.Sprintf("Failed to search traces: %s", err), timeRange), nil
	}

	data := result["data"]
	return buildStandardResponse(data, query, fromTS, toTS, false, "", timeRange), nil
}

