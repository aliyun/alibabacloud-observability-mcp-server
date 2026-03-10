// Package paas implements the PaaS Toolkit, providing tools for entity management,
// data querying, dataset management, and natural language queries using the
// Cloud Monitor 2.0 unified data model. All tool names use the `umodel_` prefix.
package paas

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/alibabacloud-observability-mcp-server-go/internal/client"
	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/timeparse"
)

// timeRangeDescription is the shared description for time_range parameters.
const timeRangeDescription = `Flexible time range formats:
- **Relative Presets:** "last_5m", "last_1h", "last_3d", "last_1w", "last_1M", "last_1y".
- **Grafana-style:** "now-15m~now-5m".
- **Keywords:** "today", "yesterday".
- **Absolute Timestamps:** "1706864400~1706868000".
- **Human-readable:** "2024-02-02 10:10:10~2024-02-02 10:20:10".
Default: last_1h`

// EntityTools returns all entity management tools backed by the given CMS client.
func EntityTools(cmsClient client.CMSClient) []toolkit.Tool {
	h := &entityHandler{cmsClient: cmsClient}
	return []toolkit.Tool{
		h.getEntitiesTool(),
		h.getNeighborEntitiesTool(),
		h.searchEntitiesTool(),
	}
}

// entityHandler holds the CMS client and provides tool constructors and handlers.
type entityHandler struct {
	cmsClient client.CMSClient
}

// ---------------------------------------------------------------------------
// Helper: parse time range
// ---------------------------------------------------------------------------

// parseTimeRange parses a time_range expression (e.g. "last_1h", "now-15m~now-5m")
// into (fromTS, toTS) in seconds. Supports the tilde-separated range syntax used
// by the Python implementation.
func parseTimeRange(raw string) (int64, int64, error) {
	if raw == "" {
		raw = "last_1h"
	}
	now := time.Now()

	// Handle tilde-separated range: "expr1~expr2"
	if parts := strings.SplitN(raw, "~", 2); len(parts) == 2 {
		fromTS, err := timeparse.ParseTimeExpression(strings.TrimSpace(parts[0]), now)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid time range from expression: %w", err)
		}
		toTS, err := timeparse.ParseTimeExpression(strings.TrimSpace(parts[1]), now)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid time range to expression: %w", err)
		}
		return fromTS, toTS, nil
	}

	// Single expression: treat as "from" with "to" = now
	fromTS, err := timeparse.ParseTimeExpression(raw, now)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid time expression: %w", err)
	}
	return fromTS, now.Unix(), nil
}

// ---------------------------------------------------------------------------
// Helper: build standard response
// ---------------------------------------------------------------------------

func buildStandardResponse(data interface{}, query string, fromTS, toTS int64, isError bool, message, expression string) map[string]interface{} {
	if expression == "" {
		expression = "last_1h"
	}
	if message == "" {
		if isError {
			message = "Query failed"
		} else if data == nil {
			message = "No data found"
		} else {
			message = "Query executed successfully"
		}
	}
	return map[string]interface{}{
		"error":   isError,
		"data":    data,
		"message": message,
		"query":   query,
		"time_range": map[string]interface{}{
			"from":          fromTS,
			"to":            toTS,
			"from_readable": timeparse.FormatTimestamp(fromTS),
			"to_readable":   timeparse.FormatTimestamp(toTS),
			"expression":    expression,
		},
	}
}

// ---------------------------------------------------------------------------
// Helper: parameter extraction
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

// ---------------------------------------------------------------------------
// Helper: SPL parameter builders (ported from Python)
// ---------------------------------------------------------------------------

// buildEntityIDsParam builds the ", ids=[...]" SPL fragment from a comma-separated string.
func buildEntityIDsParam(entityIDs string) string {
	entityIDs = strings.TrimSpace(entityIDs)
	if entityIDs == "" {
		return ""
	}
	parts := splitAndTrim(entityIDs)
	quoted := make([]string, 0, len(parts))
	for _, id := range parts {
		quoted = append(quoted, "'"+id+"'")
	}
	return ", ids=[" + strings.Join(quoted, ",") + "]"
}

// buildEntityFilterParam builds the ", query=`...`" SPL fragment from a filter expression.
func buildEntityFilterParam(filter string) string {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return ""
	}
	sqlExpr := convertToSQLSyntax(filter)
	return ", query=`" + sqlExpr + "`"
}

// convertToSQLSyntax converts simple filter expressions (e.g. "name=payment and status!=inactive")
// to SQL-like syntax (e.g. `"name"='payment' and "status"!='inactive'`).
func convertToSQLSyntax(expr string) string {
	conditions := strings.Split(expr, " and ")
	sqlParts := make([]string, 0, len(conditions))
	for _, cond := range conditions {
		cond = strings.TrimSpace(cond)
		if cond == "" {
			continue
		}
		if idx := strings.Index(cond, "!="); idx >= 0 {
			field := strings.Trim(strings.TrimSpace(cond[:idx]), "'\"")
			value := strings.Trim(strings.TrimSpace(cond[idx+2:]), "'\"")
			sqlParts = append(sqlParts, fmt.Sprintf(`"%s"!='%s'`, field, value))
		} else if idx := strings.Index(cond, "="); idx >= 0 {
			field := strings.Trim(strings.TrimSpace(cond[:idx]), "'\"")
			value := strings.Trim(strings.TrimSpace(cond[idx+1:]), "'\"")
			sqlParts = append(sqlParts, fmt.Sprintf(`"%s"='%s'`, field, value))
		}
	}
	return strings.Join(sqlParts, " and ")
}

// parseEntityIDsToSPLParam converts "id1,id2" to "['id1','id2']".
func parseEntityIDsToSPLParam(entityIDs string) string {
	parts := splitAndTrim(entityIDs)
	quoted := make([]string, 0, len(parts))
	for _, id := range parts {
		quoted = append(quoted, "'"+id+"'")
	}
	return "[" + strings.Join(quoted, ",") + "]"
}

// parseStringToSPLParam wraps a value in single quotes, or returns '' if empty.
func parseStringToSPLParam(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "''"
	}
	return "'" + value + "'"
}

// parseDirectionToSPLParam wraps a direction value, defaulting to 'both'.
func parseDirectionToSPLParam(direction string) string {
	direction = strings.TrimSpace(direction)
	if direction == "" {
		return "'both'"
	}
	return "'" + direction + "'"
}

func splitAndTrim(s string) []string {
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// ===========================================================================
// Tool 1: umodel_get_entities
// ===========================================================================

func (h *entityHandler) getEntitiesTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "umodel_get_entities",
		Description: `获取实体信息的PaaS API工具。

## 功能概述
该工具用于检索实体信息，支持分页查询、精确ID查询和过滤条件查询。
如需要模糊搜索请使用 umodel_search_entities 工具。

## 功能特点
- **数量控制**: 默认返回20个实体，支持通过limit参数控制返回数量
- **精确查询**: 支持根据实体ID列表进行精确查询
- **过滤查询**: 支持使用 entity_filter 表达式进行条件过滤
- **职责清晰**: 专注于基础实体信息获取，不包含复杂搜索逻辑

## 使用场景
- **分页浏览**: 分页获取实体列表
- **精确查询**: 根据已知的实体ID列表批量获取实体详细信息
- **条件过滤**: 使用 entity_filter 按属性过滤实体

## 参数说明
- domain: 实体集合的域，如 'apm'、'infrastructure' 等，不能为 '*'
- entity_set_name: 实体集合名称，如 'apm.service'、'host.instance' 等，不能为 '*'
- entity_ids: 可选的逗号分隔实体ID字符串，用于精确查询指定实体
- entity_filter: 可选的过滤表达式，如 'name=payment and status!=inactive'
- time_range: 时间范围表达式，支持多种格式，默认 last_1h
- limit: 返回多少个实体，默认20个，最大1000个

**注意**: entity_ids 和 entity_filter 至少需要提供一个`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "实体域, 不能为 '*'",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "实体类型, 不能为 '*'",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS工作空间名称，可通过list_workspace获取",
				},
				"entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "可选的逗号分隔实体ID列表，用于精确查询指定实体。entity_ids 和 entity_filter 至少需要提供一个",
				},
				"entity_filter": map[string]interface{}{
					"type":        "string",
					"description": "实体过滤表达式，如 'name=payment' 或 'status!=inactive'，支持 'and' 连接多个条件。entity_ids 和 entity_filter 至少需要提供一个",
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": timeRangeDescription,
					"default":     "last_1h",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "返回多少个实体，默认20个",
					"default":     20,
					"minimum":     1,
					"maximum":     1000,
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
				},
			},
			"required": []string{"domain", "entity_set_name", "workspace", "regionId"},
		},
		Handler: h.handleGetEntities,
	}
}

func (h *entityHandler) handleGetEntities(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	domain := paramString(params, "domain", "")
	entitySetName := paramString(params, "entity_set_name", "")
	workspace := paramString(params, "workspace", "")
	regionID := paramString(params, "regionId", "")
	entityIDs := paramString(params, "entity_ids", "")
	entityFilter := paramString(params, "entity_filter", "")
	timeRange := paramString(params, "time_range", "last_1h")
	limit := paramInt(params, "limit", 20)

	// Validate required params - domain and entity_set_name cannot be '*'
	if domain == "" || domain == "*" {
		return buildStandardResponse(nil, "", 0, 0, true,
			"domain 不能为空或 '*'，请使用 umodel_list_entity_types 获取有效的 domain 值", timeRange), nil
	}
	if entitySetName == "" || entitySetName == "*" {
		return buildStandardResponse(nil, "", 0, 0, true,
			"entity_set_name 不能为空或 '*'，请使用 umodel_list_entity_types 获取有效值", timeRange), nil
	}
	if workspace == "" || regionID == "" {
		return buildStandardResponse(nil, "", 0, 0, true, "workspace and regionId are required", timeRange), nil
	}

	// At least one of entity_ids or entity_filter must be provided
	if entityIDs == "" && entityFilter == "" {
		return buildStandardResponse(nil, "", 0, 0, true,
			"必须至少提供 entity_ids 或 entity_filter 之一。entity_ids: 逗号分隔的实体ID列表；entity_filter: 过滤表达式，如 'name=payment and status!=inactive'", timeRange), nil
	}

	fromTS, toTS, err := parseTimeRange(timeRange)
	if err != nil {
		return buildStandardResponse(nil, "", 0, 0, true, err.Error(), timeRange), nil
	}

	idsParam := buildEntityIDsParam(entityIDs)
	filterParam := buildEntityFilterParam(entityFilter)

	query := fmt.Sprintf(
		".entity_set with(domain='%s', name='%s'%s%s) | entity-call get_entities() | limit %d",
		domain, entitySetName, idsParam, filterParam, limit,
	)

	slog.InfoContext(ctx, "umodel_get_entities", "workspace", workspace, "domain", domain, "entity_set_name", entitySetName, "region", regionID)

	result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, limit)
	if err != nil {
		slog.ErrorContext(ctx, "umodel_get_entities failed", "error", err)
		return buildStandardResponse(nil, query, fromTS, toTS, true, fmt.Sprintf("Failed to get entities: %s", err), timeRange), nil
	}

	data := result["data"]
	return buildStandardResponse(data, query, fromTS, toTS, false, "", timeRange), nil
}

// ===========================================================================
// Tool 2: umodel_get_neighbor_entities
// ===========================================================================

func (h *entityHandler) getNeighborEntitiesTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "umodel_get_neighbor_entities",
		Description: `获取实体的邻居关系（拓扑关系）

## 功能概述
检索与源实体直接相关的实体，支持上游、下游和双向查询。
单层遍历，用于探索实体的直接连接关系。

## 方向说明
- "out": 下游关系（被调用方）
- "in": 上游关系（调用方）
- "both": 双向关系（默认）

## 使用场景
- 查看服务的上下游依赖关系
- 分析微服务调用链路
- 探索基础设施拓扑关系

## 参数说明
- src_entity_domain: 源实体域
- src_name: 源实体类型名称
- src_entity_ids: 逗号分隔的源实体ID列表
- dest_entity_domain: 可选的目标实体域过滤
- dest_name: 可选的目标实体类型过滤
- relation_type: 可选的关系类型过滤
- direction: 方向 "out"/"in"/"both"，默认 "both"`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS工作空间名称",
				},
				"src_entity_domain": map[string]interface{}{
					"type":        "string",
					"description": "源实体域，如 'apm'、'k8s' 等",
				},
				"src_name": map[string]interface{}{
					"type":        "string",
					"description": "源实体类型名称，如 'apm.service'、'k8s.pod' 等",
				},
				"src_entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "逗号分隔的源实体ID列表",
				},
				"dest_entity_domain": map[string]interface{}{
					"type":        "string",
					"description": "可选的目标实体域过滤",
				},
				"dest_name": map[string]interface{}{
					"type":        "string",
					"description": "可选的目标实体类型过滤",
				},
				"relation_type": map[string]interface{}{
					"type":        "string",
					"description": "可选的关系类型过滤",
				},
				"direction": map[string]interface{}{
					"type":        "string",
					"description": `方向: "out" (下游), "in" (上游), "both" (双向)。默认: "both"`,
					"default":     "both",
					"enum":        []string{"in", "out", "both"},
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": timeRangeDescription,
					"default":     "last_1h",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "最大返回数量，默认10",
					"default":     10,
					"minimum":     1,
					"maximum":     1000,
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID",
				},
			},
			"required": []string{"workspace", "src_entity_domain", "src_name", "src_entity_ids", "regionId"},
		},
		Handler: h.handleGetNeighborEntities,
	}
}

func (h *entityHandler) handleGetNeighborEntities(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	workspace := paramString(params, "workspace", "")
	srcDomain := paramString(params, "src_entity_domain", "")
	srcName := paramString(params, "src_name", "")
	srcEntityIDs := paramString(params, "src_entity_ids", "")
	destDomain := paramString(params, "dest_entity_domain", "")
	destName := paramString(params, "dest_name", "")
	relationType := paramString(params, "relation_type", "")
	direction := paramString(params, "direction", "both")
	timeRange := paramString(params, "time_range", "last_1h")
	limit := paramInt(params, "limit", 10)
	regionID := paramString(params, "regionId", "")

	if workspace == "" || srcDomain == "" || srcName == "" || srcEntityIDs == "" || regionID == "" {
		return buildStandardResponse(nil, "", 0, 0, true,
			"workspace, src_entity_domain, src_name, src_entity_ids and regionId are required", timeRange), nil
	}

	if direction != "in" && direction != "out" && direction != "both" {
		return buildStandardResponse(nil, "", 0, 0, true,
			fmt.Sprintf("Invalid direction: %s. Must be 'in', 'out', or 'both'", direction), timeRange), nil
	}

	fromTS, toTS, err := parseTimeRange(timeRange)
	if err != nil {
		return buildStandardResponse(nil, "", 0, 0, true, err.Error(), timeRange), nil
	}

	entityIDsSPL := parseEntityIDsToSPLParam(srcEntityIDs)
	destDomainParam := parseStringToSPLParam(destDomain)
	destNameParam := parseStringToSPLParam(destName)
	relationTypeParam := parseStringToSPLParam(relationType)
	directionParam := parseDirectionToSPLParam(direction)

	query := fmt.Sprintf(
		".entity_set with(domain='%s', name='%s', ids=%s) | entity-call get_neighbor_entities(%s, %s, [], '', %s, %s) | limit %d",
		srcDomain, srcName, entityIDsSPL,
		destDomainParam, destNameParam, relationTypeParam, directionParam,
		limit,
	)

	slog.InfoContext(ctx, "umodel_get_neighbor_entities", "workspace", workspace, "src_domain", srcDomain, "src_name", srcName, "direction", direction, "region", regionID)

	result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, limit)
	if err != nil {
		slog.ErrorContext(ctx, "umodel_get_neighbor_entities failed", "error", err)
		return buildStandardResponse(nil, query, fromTS, toTS, true, fmt.Sprintf("Failed to get neighbor entities: %s", err), timeRange), nil
	}

	data := result["data"]
	return buildStandardResponse(data, query, fromTS, toTS, false, "", timeRange), nil
}

// ===========================================================================
// Tool 3: umodel_search_entities
// ===========================================================================

func (h *entityHandler) searchEntitiesTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "umodel_search_entities",
		Description: `基于关键词全文搜索实体信息

## 功能概述
在指定的实体集合中根据关键词进行全文搜索，查找名称或属性包含搜索关键词的实体。
返回 'statistics'（按类型统计匹配数量）和 'detail'（匹配的实体详情）两部分数据。

## 功能特点
- **全文检索**: 支持对实体名称和属性进行模糊搜索
- **统计信息**: 返回按实体类型分组的匹配数量统计
- **详细结果**: 返回匹配的实体详细信息
- **灵活过滤**: 支持通配符 '*' 搜索所有域或类型

## 使用场景
- **服务搜索**: 根据服务名称片段搜索相关的微服务实体
- **基础设施搜索**: 根据主机名或IP地址搜索基础设施实体
- **快速定位**: 在大量实体中搜索包含特定关键词的实体
- **实体发现**: 作为主要的实体发现工具

## 返回数据说明
- **statistics**: 按实体类型分组的匹配数量统计
- **detail**: 匹配的实体详细信息列表`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS工作空间名称，可通过list_workspace获取",
				},
				"search_text": map[string]interface{}{
					"type":        "string",
					"description": "搜索关键词（全文搜索），支持关键词、IP、服务名等",
				},
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "实体域，可以为 '*' 表示搜索所有域",
					"default":     "*",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "实体类型，可以为 '*' 表示搜索所有类型",
					"default":     "*",
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": timeRangeDescription,
					"default":     "last_1h",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "返回多少个详细搜索结果，默认10个",
					"default":     10,
					"minimum":     1,
					"maximum":     1000,
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID",
				},
			},
			"required": []string{"workspace", "search_text", "regionId"},
		},
		Handler: h.handleSearchEntities,
	}
}

func (h *entityHandler) handleSearchEntities(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	workspace := paramString(params, "workspace", "")
	searchText := paramString(params, "search_text", "")
	domain := paramString(params, "domain", "*")
	entitySetName := paramString(params, "entity_set_name", "*")
	timeRange := paramString(params, "time_range", "last_1h")
	limit := paramInt(params, "limit", 10)
	regionID := paramString(params, "regionId", "")

	if workspace == "" || regionID == "" {
		return buildStandardResponse(nil, "", 0, 0, true, "workspace and regionId are required", timeRange), nil
	}

	fromTS, toTS, err := parseTimeRange(timeRange)
	if err != nil {
		return buildStandardResponse(nil, "", 0, 0, true, err.Error(), timeRange), nil
	}

	// Build base query
	queryBasic := fmt.Sprintf(".entity with(domain='%s', type='%s'", domain, entitySetName)
	if searchText != "" {
		queryBasic += fmt.Sprintf(", query='%s'", searchText)
	}
	queryBasic += ")"

	// Statistics query - group by domain and entity type
	queryStats := queryBasic +
		" | stats __arbitrary_entity_id__ = arbitrary(__entity_id__), match_count = count(1) by __domain__, __entity_type__" +
		" | project __arbitrary_entity_id__ = __arbitrary_entity_id__, __domain__ = __domain__, __name__ = __entity_type__, match_count" +
		" | sort match_count desc | limit 100"

	// Detail query - return matching entity details
	queryDetail := fmt.Sprintf("%s | limit %d", queryBasic, limit)

	slog.InfoContext(ctx, "umodel_search_entities", "workspace", workspace, "search_text", searchText, "domain", domain, "region", regionID)

	// Execute statistics query
	statsResult, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, queryStats, fromTS, toTS, 100)
	if err != nil {
		slog.ErrorContext(ctx, "umodel_search_entities stats query failed", "error", err)
		return buildStandardResponse(nil, queryStats, fromTS, toTS, true,
			fmt.Sprintf("Failed to search entities: %s", err), timeRange), nil
	}

	// Execute detail query
	detailResult, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, queryDetail, fromTS, toTS, limit)
	if err != nil {
		slog.ErrorContext(ctx, "umodel_search_entities detail query failed", "error", err)
		return buildStandardResponse(nil, queryDetail, fromTS, toTS, true,
			fmt.Sprintf("Failed to search entities: %s", err), timeRange), nil
	}

	statsData := statsResult["data"]
	detailData := detailResult["data"]

	combinedData := map[string]interface{}{
		"statistics": statsData,
		"detail":     detailData,
	}

	combinedQuery := fmt.Sprintf("statistics: %s\ndetail: %s", queryStats, queryDetail)
	return buildStandardResponse(combinedData, combinedQuery, fromTS, toTS, false, "", timeRange), nil
}
