package paas

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/alibabacloud-observability-mcp-server-go/pkg/client"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/toolkit"
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
		"apm.metric.exception":   true,
		"apm.metric.service":     true,
		"apm.metric.apm.service": true,
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
	"apm.metric.jvm":         "apm.instance",
	"apm.metric.exception":   "apm.instance", // also works with apm.service
	"apm.metric.service":     "apm.service",
	"apm.metric.apm.service": "apm.service",
	"k8s.metric.pod":         "k8s.pod",
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

// calculateTimeRange adjusts a time range to fit within [minDays, maxDays].
// If the current duration is shorter than minDays, it extends from toTS backwards.
// If longer than maxDays, it truncates from toTS backwards.
func calculateTimeRange(fromTS, toTS int64, minDays, maxDays int) (int64, int64) {
	currentDuration := toTS - fromTS
	minDuration := int64(minDays) * 86400
	maxDuration := int64(maxDays) * 86400

	finalDuration := currentDuration
	if finalDuration < minDuration {
		finalDuration = minDuration
	}
	if finalDuration > maxDuration {
		finalDuration = maxDuration
	}
	return toTS - finalDuration, toTS
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
		Description: `Retrieve time-series metric data for entities, supporting multiple analysis modes.

## Overview
Query time-series metric data for a specified entity set, supporting multiple analysis modes:
- basic: Returns raw time-series data (default), supports time-series comparison
- cluster: K-Means clustering analysis
- forecast: Time-series forecasting
- anomaly_detection: Anomaly detection

## Parameter Discovery Flow
1. Search entity sets: umodel_search_entity_set(search_text="apm")
2. List metric sets: umodel_list_data_set(data_set_types="metric_set") to get metric_domain_name and metric
3. Get entity IDs (optional): umodel_get_entities()
4. Execute query

## Analysis Modes
- basic: Raw time-series data, supports offset parameter for time-series comparison
- cluster: K-Means clustering analysis on time-series data
- forecast: Time-series forecasting based on historical data, requires forecast_duration
- anomaly_detection: Detect anomalies in time-series data

## Use Cases
- Query service CPU usage, request latency, and other time-series metrics
- Compare metric data across different time periods (basic + offset)
- Discover clustering patterns in metric data (cluster)
- Predict future metric trends (forecast)
- Detect anomalous metric values (anomaly_detection)`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "Entity domain, e.g. 'apm', 'host'. Cannot be '*', obtainable via umodel_search_entity_set",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "Entity set name, e.g. 'apm.service'. Cannot be '*', obtainable via umodel_search_entity_set",
				},
				"metric_domain_name": map[string]interface{}{
					"type":        "string",
					"description": "Metric domain name, e.g. 'apm.metric.jvm'. Obtainable via umodel_list_data_set(data_set_types='metric_set')",
				},
				"metric": map[string]interface{}{
					"type":        "string",
					"description": "Metric name, e.g. 'cpu_usage'. Obtainable from the fields returned by umodel_list_data_set",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS workspace name, obtainable via list_workspace",
				},
				"entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "Comma-separated entity IDs, e.g. 'id1,id2,id3'. Obtainable via umodel_get_entities",
				},
				"query_type": map[string]interface{}{
					"type":        "string",
					"description": "Query type: range (range query) or instant (instant query)",
					"default":     "range",
					"enum":        []string{"range", "instant"},
				},
				"aggregate": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to aggregate results, default true",
					"default":     true,
				},
				"analysis_mode": map[string]interface{}{
					"type":        "string",
					"description": "Analysis mode: basic (raw data), cluster (K-Means clustering), forecast (time-series forecasting), anomaly_detection (anomaly detection). Default: basic",
					"default":     "basic",
					"enum":        []string{"basic", "cluster", "forecast", "anomaly_detection"},
				},
				"forecast_duration": map[string]interface{}{
					"type":        "string",
					"description": "Forecast duration, only valid when analysis_mode='forecast'. E.g. '1h' (forecast next 1 hour), '30m' (forecast next 30 minutes). Supports s/m/h/d units",
				},
				"offset": map[string]interface{}{
					"type":        "string",
					"description": "Comparison time offset, only valid when analysis_mode='basic'. E.g. '1d' (compare with 1 day ago), '1w' (compare with 1 week ago). Supports s/m/h/d/w units",
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": timeRangeDescription,
					"default":     "last_1h",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID, e.g. 'cn-hongkong'",
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

	// Calculate entity count (used for cluster mode n_clusters calculation)
	entityCount := 0
	if entityIDs != "" {
		for _, id := range strings.Split(entityIDs, ",") {
			if strings.TrimSpace(id) != "" {
				entityCount++
			}
		}
	}

	// Build base query for getting metrics
	// SPL format: get_metric(domain, metric_domain_name, metric, query_type, step)
	baseQuery := fmt.Sprintf(
		".entity_set with(domain='%s', name='%s'%s) | entity-call get_metric('%s', '%s', '%s', '%s', %s)",
		domain, entitySetName, entityIDsParam,
		domain, metricDomainName, metric, queryType, stepParam,
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
		// Cluster mode: K-Means clustering analysis (matching Python implementation)
		// Calculate n_clusters: ceil(entityCount / 2), clamped to [2, 7]
		nClusters := 2
		if entityCount > 0 {
			nClusters = int(math.Ceil(float64(entityCount) / 2))
			if nClusters < 2 {
				nClusters = 2
			}
			if nClusters > 7 {
				nClusters = 7
			}
		}

		// Cluster mode needs aggregate=false in the base query
		clusterBaseQuery := fmt.Sprintf(
			".entity_set with(domain='%s', name='%s'%s) | entity-call get_metric('%s', '%s', '%s', '%s', %s, aggregate=false)",
			domain, entitySetName, entityIDsParam,
			domain, metricDomainName, metric, queryType, stepParam,
		)

		query = fmt.Sprintf(`%s
| stats __entity_id_array__ = array_agg(__entity_id__), __labels_array__ = array_agg(__labels__), ts_array = array_agg(__ts__), ds_array = array_agg(__value__)
| extend ret = cluster(ds_array, 'kmeans', '{"n_clusters":"%d"}')
| extend __cluster_index__ = ret.assignments, error_msg = ret.error_msg, __entity_id__ = __entity_id_array__, __labels__ = __labels_array__, __value__ = ds_array, __ts__ = ts_array
| project __entity_id__, __labels__, __ts__, __value__, __cluster_index__
| unnest
| stats cnt = count(1), __entities__ = array_agg(__entity_id__), __labels_agg__ = array_agg(__ts__), __value_agg__ = array_agg(__value__) by __cluster_index__
| extend __sample_value__ = __value_agg__[1], __sample_ts__ = __labels_agg__[1]
| extend __sample_value_min__ = array_min(__sample_value__), __sample_value_max__ = array_max(__sample_value__), __sample_value_avg__ = reduce(__sample_value__, 0.0, (s, x) -> s + x, s -> s) / cast(cardinality(__sample_value__) as double)
| project __cluster_index__, __entities__, __sample_ts__, __sample_value__, __sample_value_max__, __sample_value_min__, __sample_value_avg__`, clusterBaseQuery, nClusters)

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
		// Forecast mode: time series prediction (matching Python implementation)
		// Default forecast_duration to 30 minutes if not provided
		if forecastDuration == "" {
			forecastDuration = "30m"
		}
		forecastSeconds := ParseDurationToSeconds(forecastDuration)
		if forecastSeconds <= 0 {
			forecastSeconds = 1800 // Default 30 minutes
		}

		// Adjust time range: 1-5 days
		adjustedFrom, adjustedTo := calculateTimeRange(fromTS, toTS, 1, 5)
		learningDuration := adjustedTo - adjustedFrom

		// Calculate forecast points
		forecastPoints := int(forecastSeconds * 200 / learningDuration)
		if forecastPoints < 3 {
			forecastPoints = 3
		}

		// Forecast mode needs aggregate=false in the base query
		forecastBaseQuery := fmt.Sprintf(
			".entity_set with(domain='%s', name='%s'%s) | entity-call get_metric('%s', '%s', '%s', '%s', %s, aggregate=false)",
			domain, entitySetName, entityIDsParam,
			domain, metricDomainName, metric, queryType, stepParam,
		)

		query = fmt.Sprintf(`%s
| extend r = series_forecast(__value__, %d)
| extend __forecast_rst_m__ = zip(r.time_series, r.forecast_metric_series, r.forecast_metric_lower_series, r.forecast_metric_upper_series), __forecast_msg__ = r.error_msg
| extend __forecast_rst__ = slice(__forecast_rst_m__, cardinality(__forecast_rst_m__) - %d + 1, %d)
| project __labels__, __name__, __ts__, __value__, __forecast_rst__, __forecast_msg__, __entity_id__
| extend __forecast_ts__ = transform(__forecast_rst__, x->x.field0), __forecast_value__ = transform(__forecast_rst__, x->x.field1), __forecast_lower_value__ = transform(__forecast_rst__, x->x.field2), __forecast_upper_value__ = transform(__forecast_rst__, x->x.field3)
| project __labels__, __name__, __entity_id__, __forecast_ts__, __forecast_value__, __forecast_lower_value__, __forecast_upper_value__`,
			forecastBaseQuery, forecastPoints, forecastPoints, forecastPoints)

		slog.InfoContext(ctx, "umodel_get_metrics",
			"workspace", workspace, "domain", domain, "metric", metric,
			"analysis_mode", analysisMode, "forecast_duration", forecastDuration, "region", regionID)

		result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, adjustedFrom, adjustedTo, 1000)
		if err != nil {
			slog.ErrorContext(ctx, "umodel_get_metrics forecast failed", "error", err)
			return buildStandardResponse(nil, query, adjustedFrom, adjustedTo, true,
				fmt.Sprintf("Failed to perform forecast analysis: %s", err), timeRange), nil
		}

		data := result["data"]
		return buildStandardResponse(data, query, adjustedFrom, adjustedTo, false, "", timeRange), nil

	case "anomaly_detection":
		// Anomaly detection mode: detect anomalies in time series (matching Python implementation)
		// Adjust time range: 1-3 days
		adjustedFrom, adjustedTo := calculateTimeRange(fromTS, toTS, 1, 3)
		// Convert to nanosecond timestamp
		startTimeNs := adjustedFrom * 1_000_000_000

		// Anomaly detection mode needs aggregate=false in the base query
		anomalyBaseQuery := fmt.Sprintf(
			".entity_set with(domain='%s', name='%s'%s) | entity-call get_metric('%s', '%s', '%s', '%s', %s, aggregate=false)",
			domain, entitySetName, entityIDsParam,
			domain, metricDomainName, metric, queryType, stepParam,
		)

		query = fmt.Sprintf(`%s
| extend slice_index = find_first_index(__ts__, x -> x > %d)
| extend len = cardinality(__ts__)
| extend r = series_decompose_anomalies(__value__)
| extend anomaly_b = r.anomalies_score_series, anomaly_type = r.anomalies_type_series, __anomaly_msg__ = r.error_msg
| extend x = zip(anomaly_b, __ts__, anomaly_type, __value__)
| extend __anomaly_rst__ = filter(x, x-> x.field0 > 0 and x.field1 >= %d)
| extend __anomaly_list_ = transform(__anomaly_rst__, x-> map(ARRAY['anomary_time', 'anomary_type', 'value'], ARRAY[cast(x.field1 as varchar), x.field2, cast(x.field3 as varchar)]))
| extend __detection_value__ = slice(__value__, slice_index, len - slice_index)
| extend __value_min__ = array_min(__detection_value__), __value_max__ = array_max(__detection_value__), __value_avg__ = reduce(__detection_value__, 0.0, (s, x) -> s + x, s -> s) / cast(len as double)
| project __entity_id__, __anomaly_list_, __anomaly_msg__, __value_min__, __value_max__, __value_avg__`,
			anomalyBaseQuery, startTimeNs, startTimeNs)

		slog.InfoContext(ctx, "umodel_get_metrics",
			"workspace", workspace, "domain", domain, "metric", metric,
			"analysis_mode", analysisMode, "region", regionID)

		result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, adjustedFrom, adjustedTo, 1000)
		if err != nil {
			slog.ErrorContext(ctx, "umodel_get_metrics anomaly_detection failed", "error", err)
			return buildStandardResponse(nil, query, adjustedFrom, adjustedTo, true,
				fmt.Sprintf("Failed to perform anomaly detection: %s", err), timeRange), nil
		}

		data := result["data"]
		return buildStandardResponse(data, query, adjustedFrom, adjustedTo, false, "", timeRange), nil

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
		Description: `Retrieve golden metrics (key performance indicators) for entities, with time-shift comparison support.

## Overview
Query golden metrics for a specified entity set. Golden metrics measure service health:
- Latency: Request response time
- Throughput: Request processing rate
- Error Rate: Failed request ratio
- Saturation: Resource utilization level

## Parameter Discovery
1. Search entity sets: umodel_search_entity_set(search_text="apm")
2. Get entity IDs (optional): umodel_get_entities()
3. Execute query

## Comparison
Supports offset parameter for time-shift comparison, e.g. '1d' compares with data from 1 day ago.

## Use Cases
- Quickly assess overall service health
- Monitor key performance indicators
- Compare performance across different time periods`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "Entity domain, e.g. 'apm', 'host'. Must not be '*'. Obtain via umodel_search_entity_set",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "Entity set name, e.g. 'apm.service'. Must not be '*'. Obtain via umodel_search_entity_set",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS workspace name. Obtain via list_workspace",
				},
				"entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "Comma-separated entity IDs, e.g. 'id1,id2,id3'. Obtain via umodel_get_entities",
				},
				"query_type": map[string]interface{}{
					"type":        "string",
					"description": "Query type: range (range query, returns time-series data) or instant (instant query, returns latest value)",
					"default":     "range",
					"enum":        []string{"range", "instant"},
				},
				"aggregate": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to aggregate results. true aggregates all entities, false returns per-entity results",
					"default":     true,
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": timeRangeDescription,
					"default":     "last_1h",
				},
				"offset": map[string]interface{}{
					"type":        "string",
					"description": "Comparison offset, e.g. '1h', '1d', '1w'. When set, two queries are executed (current and comparison periods) and a comparison analysis is returned",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID, e.g. 'cn-hongkong'",
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
		Description: `Retrieve relation-level metric data between entities.

## Overview
Query metric data for relationships between entity sets, such as inter-service call latency and throughput.
Supports upstream, downstream, and bidirectional relationship queries.

## Parameter Discovery
1. Search entity sets: umodel_search_entity_set(search_text="apm")
2. List metric sets: umodel_list_data_set(data_set_types="metric_set") to get metric_set_domain and metric
3. Get entity IDs (optional): umodel_get_entities()
4. Execute query

## Direction
- "upstream": Upstream relationship (caller)
- "downstream": Downstream relationship (callee)
- "both": Bidirectional (default)

## Use Cases
- Query inter-service call latency metrics
- Analyze upstream/downstream dependency performance
- Monitor inter-service communication health`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"src_domain": map[string]interface{}{
					"type":        "string",
					"description": "Source entity domain, e.g. 'apm', 'host'. Must not be '*'. Obtain via umodel_search_entity_set",
				},
				"src_entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "Source entity set name, e.g. 'apm.service'. Must not be '*'. Obtain via umodel_search_entity_set",
				},
				"src_entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "Comma-separated source entity IDs, e.g. 'id1,id2,id3'. Obtain via umodel_get_entities",
				},
				"relation_type": map[string]interface{}{
					"type":        "string",
					"description": "Relation type, e.g. 'calls'. Used to auto-generate metric_set_name",
				},
				"direction": map[string]interface{}{
					"type":        "string",
					"description": "Relation direction: 'out' (src->dest) or 'in' (dest->src). Default: out",
					"default":     "out",
					"enum":        []string{"in", "out"},
				},
				"metric_set_domain": map[string]interface{}{
					"type":        "string",
					"description": "Metric set domain, e.g. 'apm'. Obtain via umodel_list_data_set(data_set_types='metric_set')",
				},
				"metric_set_name": map[string]interface{}{
					"type":        "string",
					"description": "Metric set name. If not provided, auto-generated as relation_type + '.' + src_entity_set_name",
				},
				"metric": map[string]interface{}{
					"type":        "string",
					"description": "Metric name, e.g. 'latency', 'throughput'. Obtain from the fields returned by umodel_list_data_set",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS workspace name. Obtain via list_workspace",
				},
				"dest_domain": map[string]interface{}{
					"type":        "string",
					"description": "Destination entity domain (optional), used to filter by target domain",
				},
				"dest_entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "Destination entity set name (optional), used to filter by target type",
				},
				"dest_entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "Comma-separated destination entity IDs (optional)",
				},
				"query_type": map[string]interface{}{
					"type":        "string",
					"description": "Query type: range (range query) or instant (instant query)",
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
					"description": "Alibaba Cloud region ID, e.g. 'cn-hongkong'",
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
	direction := paramString(params, "direction", "out")
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
	validDirections := map[string]bool{"in": true, "out": true}
	if !validDirections[direction] {
		return buildStandardResponse(nil, "", 0, 0, true,
			fmt.Sprintf("invalid direction '%s', must be one of: in, out", direction), timeRange), nil
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

	// Auto-generate metric_set_name if not provided.
	// Format: {metric_set_domain}.metric.{src_entity_set_name}_{relation_type}_{dest_entity_set_name}
	// Example: apm.metric.apm.service_calls_apm.external.nosql
	if metricSetName == "" {
		if destEntitySetName == "" {
			return buildStandardResponse(nil, "", 0, 0, true,
				"metric_set_name is required when dest_entity_set_name is not provided. "+
					"Either provide metric_set_name directly, or provide dest_entity_set_name to auto-generate it. "+
					"Use umodel_list_related_entity_set to find valid relation_data_sets names", timeRange), nil
		}
		metricSetName = metricSetDomain + ".metric." + srcEntitySetName + "_" + relationType + "_" + destEntitySetName
	}

	// Auto-infer dest_domain from dest_entity_set_name prefix if not provided.
	// e.g. "apm.external.nosql" → "apm", "k8s.deployment" → "k8s"
	if destDomain == "" && destEntitySetName != "" {
		if idx := strings.Index(destEntitySetName, "."); idx > 0 {
			destDomain = destEntitySetName[:idx]
		}
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
	// Parameters (per CMS API): dest_domain, dest_name, dest_ids, filter, relation_type, direction,
	//   metric_set_domain, metric_set_name, metric, query_type, step, aggregate_labels
	stepParam := "''"            // Auto step
	aggregateLabelsParam := "[]" // No aggregation by default
	relationTypeParam := parseStringToSPLParam(relationType)
	directionParam := parseStringToSPLParam(direction)
	query := fmt.Sprintf(
		".entity_set with(domain='%s', name='%s'%s) | entity-call get_relation_metric(%s, %s, %s, '', %s, %s, '%s', '%s', '%s', '%s', %s, %s)",
		srcDomain, srcEntitySetName, srcEntityIDsParam,
		destDomainParam, destEntitySetNameParam, destEntityIDsParam, relationTypeParam, directionParam,
		metricSetDomain, metricSetName, metric, queryType, stepParam, aggregateLabelsParam,
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
		Description: `Retrieve log data for entities, supporting raw log queries and log clustering analysis.

## Overview
Query log data for a specified entity set, supporting raw log queries and field-based log clustering.
Log clustering automatically identifies log patterns, groups similar logs, and helps quickly discover issues.

## Parameter Discovery
1. Search entity sets: umodel_search_entity_set(search_text="apm")
2. List log sets: umodel_list_data_set(data_set_types="log_set") to get log_set_domain and log_set_name
3. Get entity IDs (optional but recommended): umodel_get_entities()
4. Execute query

## Log Clustering
When to_cluster_content_field is provided, log clustering analysis is enabled:
- Automatically identifies log patterns and groups similar logs
- Returns statistics per pattern: pattern ID, pattern template, event count, time range, sample data
- Optionally provide to_cluster_aggregate_field for further grouping by field

## Use Cases
- Query service error logs
- Log clustering analysis to discover anomalous patterns
- Fault diagnosis and performance analysis`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "Entity domain, e.g. 'apm', 'host'. Cannot be '*'",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "Entity set name, e.g. 'apm.service'. Cannot be '*'",
				},
				"log_set_domain": map[string]interface{}{
					"type":        "string",
					"description": "Log set domain, e.g. 'apm'. Obtainable via umodel_list_data_set(data_set_types='log_set')",
				},
				"log_set_name": map[string]interface{}{
					"type":        "string",
					"description": "Log set name, e.g. 'apm.log.apm.service'. Obtainable via umodel_list_data_set",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS workspace name, obtainable via list_workspace",
				},
				"entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "Comma-separated entity IDs, obtainable via umodel_get_entities (strongly recommended)",
				},
				"to_cluster_content_field": map[string]interface{}{
					"type":        "string",
					"description": "Log clustering field, e.g. 'content', 'message'. When provided, enables log clustering analysis and returns log patterns instead of raw logs",
				},
				"to_cluster_aggregate_field": map[string]interface{}{
					"type":        "string",
					"description": "Clustering aggregation field, e.g. 'severity', 'level'. Used for further grouping in clustering results",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of log records to return, default 100",
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
					"description": "Alibaba Cloud region ID",
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
		Description: `Retrieve event data for a specified entity set.

## Overview
Query event data for a specified entity set. Events are discrete records such as deployments, alerts, and configuration changes.
Used for correlating system behavior changes.

## Parameter Discovery
1. Search entity sets: umodel_search_entity_set(search_text="apm")
2. List event sets: umodel_list_data_set(data_set_types="event_set") or use defaults "default"/"default.event.common"
3. Get entity IDs (optional): umodel_get_entities()
4. Execute query

## Use Cases
- View service deployment events
- Correlate alert events with metric anomalies
- Analyze the impact of configuration changes on the system`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "Entity domain, e.g. 'apm', 'host'. Cannot be '*'",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "Entity set name, e.g. 'apm.service'. Cannot be '*'",
				},
				"event_set_domain": map[string]interface{}{
					"type":        "string",
					"description": "Event set domain, e.g. 'default'. Obtainable via umodel_list_data_set(data_set_types='event_set')",
				},
				"event_set_name": map[string]interface{}{
					"type":        "string",
					"description": "Event set name, e.g. 'default.event.common'. Obtainable via umodel_list_data_set",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS workspace name, obtainable via list_workspace",
				},
				"entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "Comma-separated entity IDs, obtainable via umodel_get_entities",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of event records to return, default 100",
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
					"description": "Alibaba Cloud region ID",
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

	// Two query modes based on event set type (matching umodel-mcp implementation):
	// 1. Default event set (default/default.event.common): table-style query via .event_set with SQL conditions
	// 2. Custom event set: entity-call via .entity_set
	var query string
	if eventSetDomain == "default" && eventSetName == "default.event.common" {
		// Build SQL conditions for default event set
		conditions := fmt.Sprintf(`"resource.entity.domain" = '%s' and "resource.entity.entity_type" = '%s'`,
			domain, entitySetName)

		// Add entity ID filter if specified
		if entityIDs != "" {
			parts := strings.Split(entityIDs, ",")
			quoted := make([]string, 0, len(parts))
			for _, id := range parts {
				id = strings.TrimSpace(id)
				if id != "" {
					quoted = append(quoted, "'"+id+"'")
				}
			}
			if len(quoted) > 0 {
				conditions += fmt.Sprintf(` and "resource.entity.entity_id" in (%s)`, strings.Join(quoted, ","))
			}
		}

		query = fmt.Sprintf(".event_set with(domain='%s', name='%s', query=`%s`) | limit 0, %d",
			eventSetDomain, eventSetName, conditions, limit)
	} else {
		// Custom event set: use entity-call get_event
		entityIDsParam := buildEntityIDsParam(entityIDs)
		query = fmt.Sprintf(
			".entity_set with(domain='%s', name='%s'%s) | entity-call get_event('%s', '%s') | limit 0, %d",
			domain, entitySetName, entityIDsParam,
			eventSetDomain, eventSetName, limit,
		)
	}

	slog.InfoContext(ctx, "umodel_get_events",
		"workspace", workspace, "domain", domain, "event_set_name", eventSetName, "region", regionID)

	result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, limit)
	if err != nil {
		// Auto-retry on MultipleStorageFound: parse storage ID components and retry
		// using Table mode (.event_set with storage_domain/storage_kind/storage_name)
		if storage := parseMultipleStorageID(err.Error()); storage != nil {
			slog.InfoContext(ctx, "umodel_get_events retrying with storage (table mode)",
				"storage_domain", storage.Domain, "storage_kind", storage.Kind, "storage_name", storage.Name)
			retryQuery := buildEventQueryWithStorage(eventSetDomain, eventSetName, storage, entityIDs, limit)
			result, err = h.cmsClient.ExecuteSPL(ctx, regionID, workspace, retryQuery, fromTS, toTS, limit)
			if err != nil {
				slog.ErrorContext(ctx, "umodel_get_events retry failed", "error", err)
				return buildStandardResponse(nil, retryQuery, fromTS, toTS, true,
					fmt.Sprintf("Failed to get events (retry with storage): %s", err), timeRange), nil
			}
			data := result["data"]
			return buildStandardResponse(data, retryQuery, fromTS, toTS, false, "", timeRange), nil
		}

		slog.ErrorContext(ctx, "umodel_get_events failed", "error", err)
		return buildStandardResponse(nil, query, fromTS, toTS, true,
			fmt.Sprintf("Failed to get events: %s", err), timeRange), nil
	}

	data := result["data"]
	return buildStandardResponse(data, query, fromTS, toTS, false, "", timeRange), nil
}

// storageInfo holds parsed storage components from a storage ID.
// Storage ID format: "domain@kind@name", e.g. "k8s@sls_logstore@k8s-log-xxx/k8s-event".
type storageInfo struct {
	Domain string
	Kind   string
	Name   string
}

// parseMultipleStorageID extracts the first storage ID from a MultipleStorageFound error message
// and parses it into domain/kind/name components.
func parseMultipleStorageID(errMsg string) *storageInfo {
	if !strings.Contains(errMsg, "MultipleStorageFound") {
		return nil
	}
	marker := "available storage IDs: ["
	idx := strings.Index(errMsg, marker)
	if idx < 0 {
		return nil
	}
	rest := errMsg[idx+len(marker):]
	end := strings.Index(rest, "]")
	if end < 0 {
		return nil
	}
	ids := rest[:end]
	parts := strings.SplitN(ids, ",", 2)
	if len(parts) == 0 {
		return nil
	}
	raw := strings.TrimSpace(parts[0])
	return parseStorageID(raw)
}

// parseStorageID parses a storage ID string "domain@kind@name" into components.
func parseStorageID(id string) *storageInfo {
	// Format: domain@kind@name (e.g. "k8s@sls_logstore@k8s-log-xxx/k8s-event")
	parts := strings.SplitN(id, "@", 3)
	if len(parts) != 3 {
		return nil
	}
	return &storageInfo{Domain: parts[0], Kind: parts[1], Name: parts[2]}
}

// buildEventQueryWithStorage builds an event query using Table mode (.event_set with)
// with explicit storage_domain/storage_kind/storage_name parameters.
// This is used as a fallback when Object mode entity-call fails with MultipleStorageFound.
func buildEventQueryWithStorage(eventSetDomain, eventSetName string, storage *storageInfo, entityIDs string, limit int) string {
	var queryParam string
	if entityIDs != "" {
		// 将 entity_ids 转为过滤条件
		parts := strings.Split(entityIDs, ",")
		quoted := make([]string, 0, len(parts))
		for _, id := range parts {
			id = strings.TrimSpace(id)
			if id != "" {
				quoted = append(quoted, "'"+id+"'")
			}
		}
		if len(quoted) > 0 {
			queryParam = fmt.Sprintf(", query=`\"resource.entity.entity_id\" in (%s)`", strings.Join(quoted, ","))
		}
	}
	return fmt.Sprintf(
		".event_set with(domain='%s', name='%s', storage_domain='%s', storage_kind='%s', storage_name='%s'%s) | limit 0, %d",
		eventSetDomain, eventSetName,
		storage.Domain, storage.Kind, storage.Name,
		queryParam, limit,
	)
}

// ===========================================================================
// Tool 5: umodel_get_traces
// ===========================================================================

func (h *dataHandler) getTracesTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "umodel_get_traces",
		Description: `Retrieve detailed trace data for specified trace IDs, including all spans and metadata.

## Overview
Retrieve detailed trace data by trace ID, including all spans, durations, and metadata.
Used for in-depth analysis of slow and error traces.

## Parameter Discovery Flow
1. Search entity sets: umodel_search_entity_set(search_text="apm")
2. List TraceSets: umodel_list_data_set(data_set_types="trace_set") to get trace_set_domain and trace_set_name
3. Get trace IDs: Typically obtained from umodel_search_traces output
4. Execute query

## Output Fields
- duration_ms: Total span duration (milliseconds)
- exclusive_duration_ms: Exclusive span duration (milliseconds)

## Use Cases
- Analyze call chains of slow requests
- Locate root causes of error traces
- View span-level details`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "Entity domain, e.g. 'apm', 'host'. Cannot be '*'",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "Entity set name, e.g. 'apm.service'. Cannot be '*'",
				},
				"trace_set_domain": map[string]interface{}{
					"type":        "string",
					"description": "TraceSet domain, e.g. 'apm'. Obtainable via umodel_list_data_set(data_set_types='trace_set')",
				},
				"trace_set_name": map[string]interface{}{
					"type":        "string",
					"description": "TraceSet name, e.g. 'apm.trace.common'. Obtainable via umodel_list_data_set",
				},
				"trace_ids": map[string]interface{}{
					"type":        "string",
					"description": "Comma-separated trace IDs, e.g. 'trace1,trace2'. Typically obtained from umodel_search_traces",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS workspace name, obtainable via list_workspace",
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": timeRangeDescription,
					"default":     "last_1h",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID",
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
// Tool 6: umodel_get_profiles
// ===========================================================================

func (h *dataHandler) getProfilingTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "umodel_get_profiles",
		Description: `Retrieve profiling data for a specified entity set.

## Overview
Query profiling data for a specified entity set, including CPU usage, memory allocation, method call stacks, etc.
Used for performance bottleneck analysis and code-level optimization.

## Parameter Discovery Flow
1. Search entity sets: umodel_search_entity_set(search_text="apm")
2. List ProfileSets: umodel_list_data_set(data_set_types="profile_set") to get profile_set_domain and profile_set_name
3. Get entity IDs (required): umodel_get_entities()
4. Execute query

## Use Cases
- Analyze CPU-intensive code paths
- Discover memory leaks and high memory consumption points
- Analyze call chains and hotspot functions`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "Entity domain, e.g. 'apm', 'host'. Cannot be '*'",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "Entity set name, e.g. 'apm.service'. Cannot be '*'",
				},
				"profile_set_domain": map[string]interface{}{
					"type":        "string",
					"description": "ProfileSet domain, e.g. 'default'. Obtainable via umodel_list_data_set(data_set_types='profile_set')",
				},
				"profile_set_name": map[string]interface{}{
					"type":        "string",
					"description": "ProfileSet name, e.g. 'default.profile.common'. Obtainable via umodel_list_data_set",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS workspace name, obtainable via list_workspace",
				},
				"entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "Comma-separated entity IDs (required, specify exact entities for large datasets). Obtainable via umodel_get_entities",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of profiling records to return, default 100",
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
					"description": "Alibaba Cloud region ID",
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

	slog.InfoContext(ctx, "umodel_get_profiles",
		"workspace", workspace, "domain", domain, "profile_set_name", profileSetName, "region", regionID)

	result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, limit)
	if err != nil {
		slog.ErrorContext(ctx, "umodel_get_profiles failed", "error", err)
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
		Description: `Search traces based on filter conditions and return summary information.

## Overview
Supports filtering by duration, error status, and entity IDs. Returns trace IDs for detailed analysis.
Search results contain trace summary: traceId, duration_ms, span_count, error_span_count.

## Parameter Discovery Flow
1. Search entity sets: umodel_search_entity_set(search_text="apm")
2. List TraceSets: umodel_list_data_set(data_set_types="trace_set") to get trace_set_domain and trace_set_name
3. Get entity IDs (optional): umodel_get_entities()
4. Execute search

## Filter Conditions
- min_duration_ms: Minimum trace duration (ms), for filtering slow traces
- max_duration_ms: Maximum trace duration (ms), for filtering fast traces
- has_error: Filter by error status (true for error traces)

## Use Cases
- Search for slow traces (duration exceeding threshold)
- Search for error traces
- Search for traces of specific entities`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "Entity domain, e.g. 'apm', 'host'. Cannot be '*'",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "Entity set name, e.g. 'apm.service'. Cannot be '*'",
				},
				"trace_set_domain": map[string]interface{}{
					"type":        "string",
					"description": "TraceSet domain, e.g. 'apm'. Obtainable via umodel_list_data_set(data_set_types='trace_set')",
				},
				"trace_set_name": map[string]interface{}{
					"type":        "string",
					"description": "TraceSet name, e.g. 'apm.trace.common'. Obtainable via umodel_list_data_set",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS workspace name, obtainable via list_workspace",
				},
				"entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "Comma-separated entity IDs, e.g. 'id1,id2,id3'. Obtainable via umodel_get_entities",
				},
				"min_duration_ms": map[string]interface{}{
					"type":        "number",
					"description": "Minimum trace duration in milliseconds, for filtering slow traces",
				},
				"max_duration_ms": map[string]interface{}{
					"type":        "number",
					"description": "Maximum trace duration in milliseconds, for filtering fast traces",
				},
				"has_error": map[string]interface{}{
					"type":        "boolean",
					"description": "Filter by error status (true for error traces, false for successful traces)",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of trace summaries to return, default 100",
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
					"description": "Alibaba Cloud region ID",
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
