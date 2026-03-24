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

	"github.com/alibabacloud-observability-mcp-server-go/pkg/client"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/timeparse"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/toolkit"
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
// to SPL query syntax (e.g. `name="payment" and status!="inactive"`).
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
			sqlParts = append(sqlParts, fmt.Sprintf(`%s!="%s"`, field, value))
		} else if idx := strings.Index(cond, "="); idx >= 0 {
			field := strings.Trim(strings.TrimSpace(cond[:idx]), "'\"")
			value := strings.Trim(strings.TrimSpace(cond[idx+1:]), "'\"")
			sqlParts = append(sqlParts, fmt.Sprintf(`%s="%s"`, field, value))
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

// parseStringToSPLParam wraps a value in single quotes, or returns ” if empty.
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
		Description: `PaaS API tool for retrieving entity information.

## Overview
Retrieves entity information with support for paginated queries, exact ID lookups, and filter-based queries.
For fuzzy search, use the umodel_search_entities tool instead.

## Features
- **Pagination**: Returns 20 entities by default, configurable via the limit parameter
- **Exact Lookup**: Supports querying by a list of entity IDs
- **Filter Query**: Supports entity_filter expressions for conditional filtering
- **Focused**: Retrieves basic entity information only, no complex search logic

## Use Cases
- **Paginated Browsing**: Retrieve entity lists page by page
- **Exact Lookup**: Batch-retrieve entity details by known entity IDs
- **Conditional Filtering**: Filter entities by attributes using entity_filter

## Parameters
- domain: Entity domain, e.g. 'apm', 'infrastructure', cannot be '*'
- entity_set_name: Entity set name, e.g. 'apm.service', 'host.instance', cannot be '*'
- entity_ids: Optional comma-separated entity ID string for exact lookups
- entity_filter: Optional filter expression, e.g. 'service=qwen38b and status!=inactive'
- time_range: Time range expression, supports multiple formats, default last_1h
- limit: Number of entities to return, default 20, max 1000

**Note**: When neither entity_ids nor entity_filter is provided, all entities in the entity set are returned (up to the limit)`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "Entity domain, cannot be '*'",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "Entity set name, cannot be '*'",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS workspace name, obtainable via list_workspace",
				},
				"entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "Optional comma-separated entity IDs for exact lookup. At least one of entity_ids or entity_filter must be provided",
				},
				"entity_filter": map[string]interface{}{
					"type":        "string",
					"description": "Entity filter expression, e.g. 'name=payment' or 'status!=inactive', supports 'and' to join conditions. At least one of entity_ids or entity_filter must be provided",
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": timeRangeDescription,
					"default":     "last_1h",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Number of entities to return, default 20",
					"default":     20,
					"minimum":     1,
					"maximum":     1000,
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID, e.g. 'cn-hongkong'",
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
			"domain cannot be empty or '*'. Use umodel_list_entity_types to get valid domain values", timeRange), nil
	}
	if entitySetName == "" || entitySetName == "*" {
		return buildStandardResponse(nil, "", 0, 0, true,
			"entity_set_name cannot be empty or '*'. Use umodel_list_entity_types to get valid values", timeRange), nil
	}
	if workspace == "" || regionID == "" {
		return buildStandardResponse(nil, "", 0, 0, true, "workspace and regionId are required", timeRange), nil
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
		Description: `Retrieve neighbor entities (topology relationships) of an entity.

## Overview
Retrieves entities directly related to the source entity, supporting upstream, downstream, and bidirectional queries.
Single-layer traversal for exploring direct connections of an entity.

## Direction
- "out": Downstream relationships (callees)
- "in": Upstream relationships (callers)
- "both": Bidirectional relationships (default)

## Use Cases
- View upstream/downstream dependencies of a service
- Analyze microservice call chains
- Explore infrastructure topology relationships

## Parameters
- src_entity_domain: Source entity domain
- src_name: Source entity set name
- src_entity_ids: Comma-separated source entity IDs
- dest_entity_domain: Optional destination entity domain filter
- dest_name: Optional destination entity set name filter
- relation_type: Optional relationship type filter
- direction: Direction "out"/"in"/"both", default "both"`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS workspace name",
				},
				"src_entity_domain": map[string]interface{}{
					"type":        "string",
					"description": "Source entity domain, e.g. 'apm', 'k8s'",
				},
				"src_name": map[string]interface{}{
					"type":        "string",
					"description": "Source entity set name, e.g. 'apm.service', 'k8s.pod'",
				},
				"src_entity_ids": map[string]interface{}{
					"type":        "string",
					"description": "Comma-separated source entity IDs",
				},
				"dest_entity_domain": map[string]interface{}{
					"type":        "string",
					"description": "Optional destination entity domain filter",
				},
				"dest_name": map[string]interface{}{
					"type":        "string",
					"description": "Optional destination entity set name filter",
				},
				"relation_type": map[string]interface{}{
					"type":        "string",
					"description": "Optional relationship type filter",
				},
				"direction": map[string]interface{}{
					"type":        "string",
					"description": `Direction: "out" (downstream), "in" (upstream), "both" (bidirectional). Default: "both"`,
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
					"description": "Maximum number of results to return, default 10",
					"default":     10,
					"minimum":     1,
					"maximum":     1000,
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID",
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
		Description: `Full-text search for entities by keyword.

## Overview
Searches for entities matching a keyword in the specified entity set, looking for matches in entity names and attributes.
Returns 'statistics' (match count by type) and 'detail' (matched entity details).

## Features
- **Full-text Search**: Supports fuzzy search across entity names and attributes
- **Statistics**: Returns match count grouped by entity type
- **Detailed Results**: Returns matched entity details
- **Flexible Filtering**: Supports wildcard '*' to search all domains or types

## Use Cases
- **Service Search**: Search for microservice entities by service name fragments
- **Infrastructure Search**: Search for infrastructure entities by hostname or IP
- **Quick Lookup**: Find entities containing specific keywords among large datasets
- **Entity Discovery**: Primary tool for entity discovery

## Response
- **statistics**: Match count grouped by entity type
- **detail**: List of matched entity details`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS workspace name, obtainable via list_workspace",
				},
				"search_text": map[string]interface{}{
					"type":        "string",
					"description": "Search keyword (full-text search), supports keywords, IPs, service names, etc.",
				},
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "Entity domain, use '*' to search all domains",
					"default":     "*",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "Entity set name, use '*' to search all types",
					"default":     "*",
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": timeRangeDescription,
					"default":     "last_1h",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Number of detailed search results to return, default 10",
					"default":     10,
					"minimum":     1,
					"maximum":     1000,
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID",
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
