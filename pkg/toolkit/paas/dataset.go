package paas

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/alibabacloud-observability-mcp-server-go/pkg/client"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/toolkit"
)

// DatasetTools returns all dataset management tools backed by the given CMS client.
func DatasetTools(cmsClient client.CMSClient) []toolkit.Tool {
	h := &datasetHandler{cmsClient: cmsClient}
	return []toolkit.Tool{
		h.listDataSetTool(),
		h.searchEntitySetTool(),
		h.getEntitySetTool(),
		h.listRelatedEntitySetTool(),
	}
}

// datasetHandler holds the CMS client and provides tool constructors and handlers.
type datasetHandler struct {
	cmsClient client.CMSClient
}

// ===========================================================================
// Tool 1: umodel_list_data_set
// ===========================================================================

func (h *datasetHandler) listDataSetTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "umodel_list_data_set",
		Description: `List available data sets for a specified entity, providing parameter options for other PaaS tools.

## Overview

This tool is a metadata query interface for retrieving available data set information under a specified entity domain and type.
Its primary purpose is to provide parameter lists for other PaaS-layer tools (e.g. observability, entity tools),
including metric sets, log sets, event sets, and other storage information.

## Use Cases

- **Parameter Discovery**: Provide available metric set (metric_set) lists for umodel_get_metrics
- **Log Source Query**: Provide available log set (log_set) lists for umodel_get_logs
- **Event Source Discovery**: Provide available event set (event_set) lists for umodel_get_events
- **Trace Data Source**: Provide available trace set (trace_set) lists for umodel_get_traces

## Parameters

- data_set_types: Data set type filter, common types include:
  * 'metric_set': Metric sets
  * 'log_set': Log sets
  * 'event_set': Event sets
  * 'trace_set': Trace sets
  * 'profile_set': Profiling sets`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS workspace name, obtainable via list_workspace",
				},
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "Entity domain, cannot be '*'",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "Entity set name, cannot be '*'",
				},
				"data_set_types": map[string]interface{}{
					"type":        "string",
					"description": "Comma-separated data set type filter, e.g. 'metric_set,log_set'",
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": timeRangeDescription,
					"default":     "last_5m",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID, e.g. 'cn-hongkong'",
				},
			},
			"required": []string{"workspace", "domain", "entity_set_name", "regionId"},
		},
		Handler: h.handleListDataSet,
	}
}

func (h *datasetHandler) handleListDataSet(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	workspace := paramString(params, "workspace", "")
	domain := paramString(params, "domain", "")
	entitySetName := paramString(params, "entity_set_name", "")
	dataSetTypes := paramString(params, "data_set_types", "")
	timeRange := paramString(params, "time_range", "last_5m")
	regionID := paramString(params, "regionId", "")

	if workspace == "" || domain == "" || entitySetName == "" || regionID == "" {
		return buildStandardResponse(nil, "", 0, 0, true,
			"workspace, domain, entity_set_name and regionId are required", timeRange), nil
	}

	fromTS, toTS, err := parseTimeRange(timeRange)
	if err != nil {
		return buildStandardResponse(nil, "", 0, 0, true, err.Error(), timeRange), nil
	}

	// Build data_set_types parameter for SPL
	typesParam := "[]"
	if dataSetTypes != "" {
		parts := splitAndTrim(dataSetTypes)
		quoted := make([]string, 0, len(parts))
		for _, t := range parts {
			quoted = append(quoted, "'"+t+"'")
		}
		typesParam = "[" + strings.Join(quoted, ",") + "]"
	}

	query := fmt.Sprintf(
		".entity_set with(domain='%s', name='%s') | entity-call list_data_set(%s)",
		domain, entitySetName, typesParam,
	)

	slog.InfoContext(ctx, "umodel_list_data_set", "workspace", workspace, "domain", domain, "entity_set_name", entitySetName, "region", regionID)

	result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, 1000)
	if err != nil {
		slog.ErrorContext(ctx, "umodel_list_data_set failed", "error", err)
		return buildStandardResponse(nil, query, fromTS, toTS, true,
			fmt.Sprintf("Failed to list data sets: %s", err), timeRange), nil
	}

	data := result["data"]
	return buildStandardResponse(data, query, fromTS, toTS, false, "", timeRange), nil
}

// ===========================================================================
// Tool 2: umodel_search_entity_set
// ===========================================================================

func (h *datasetHandler) searchEntitySetTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "umodel_search_entity_set",
		Description: `Search entity sets with full-text search, sorted by relevance.

## Overview

Searches entity set definitions in UModel metadata by keyword, supporting full-text search.
Primarily used to discover available entity set types and their metadata.

## Features

- **Full-text Search**: Supports full-text search across entity set metadata and specs
- **Relevance Sorting**: Results are sorted by relevance
- **Optional Filtering**: Supports additional filtering by domain and name

## Use Cases

- **Entity Set Discovery**: Search for entity set types containing specific keywords
- **Metadata Exploration**: Discover available entity sets and their descriptions`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"search_text": map[string]interface{}{
					"type":        "string",
					"description": "Search keyword for full-text search",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS workspace name, obtainable via list_workspace",
				},
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "Optional entity domain filter",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "Optional entity set name filter",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Number of entity sets to return, default 10",
					"default":     10,
					"minimum":     1,
					"maximum":     100,
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID, e.g. 'cn-hongkong'",
				},
			},
			"required": []string{"search_text", "workspace", "regionId"},
		},
		Handler: h.handleSearchEntitySet,
	}
}

func (h *datasetHandler) handleSearchEntitySet(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	searchText := paramString(params, "search_text", "")
	workspace := paramString(params, "workspace", "")
	domain := paramString(params, "domain", "")
	entitySetName := paramString(params, "entity_set_name", "")
	limit := paramInt(params, "limit", 10)
	regionID := paramString(params, "regionId", "")

	if searchText == "" || workspace == "" || regionID == "" {
		return buildStandardResponse(nil, "", 0, 0, true,
			"search_text, workspace and regionId are required", "last_1h"), nil
	}

	fromTS, toTS, err := parseTimeRange("last_1h")
	if err != nil {
		return buildStandardResponse(nil, "", 0, 0, true, err.Error(), "last_1h"), nil
	}

	// Build SPL query matching Python implementation
	query := ".umodel | where kind = 'entity_set' and __type__ = 'node'"

	if domain != "" {
		query += fmt.Sprintf(" | where json_extract_scalar(metadata, '$.domain') = '%s'", domain)
	}
	if entitySetName != "" {
		query += fmt.Sprintf(" | where json_extract_scalar(metadata, '$.name') = '%s'", entitySetName)
	}

	// Full-text search filter
	query += fmt.Sprintf(" | where strpos(metadata, '%s') > 0 or strpos(spec, '%s') > 0", searchText, searchText)

	// Project name column and limit
	query += " | extend name = json_extract_scalar(metadata, '$.name') | project name | limit 100"

	slog.InfoContext(ctx, "umodel_search_entity_set", "workspace", workspace, "search_text", searchText, "domain", domain, "region", regionID)

	result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, limit)
	if err != nil {
		slog.ErrorContext(ctx, "umodel_search_entity_set failed", "error", err)
		return buildStandardResponse(nil, query, fromTS, toTS, true,
			fmt.Sprintf("Failed to search entity sets: %s", err), "last_1h"), nil
	}

	data := result["data"]
	return buildStandardResponse(data, query, fromTS, toTS, false, "", "last_1h"), nil
}

// ===========================================================================
// Tool 3: umodel_list_related_entity_set
// ===========================================================================

func (h *datasetHandler) listRelatedEntitySetTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "umodel_list_related_entity_set",
		Description: `List entity sets related to a specified entity set.

## Overview

Discovers other entity set types that have relationship definitions with the specified entity set.
This is a metadata-level tool for exploring the high-level blueprint of the UModel topology.

## Features

- **Relationship Discovery**: Find entity sets with defined relationships to the source entity set
- **Direction Control**: Supports viewing inbound, outbound, or bidirectional relationships
- **Type Filtering**: Filter by specific relationship types

## Use Cases

- **Topology Exploration**: Discover possible relationship types between entity sets
- **Dependency Analysis**: Find other entity types a service can call`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "Entity domain, e.g. 'apm'",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "Entity set name, e.g. 'apm.service'",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS workspace name, obtainable via list_workspace",
				},
				"relation_type": map[string]interface{}{
					"type":        "string",
					"description": "Relationship type filter, e.g. 'calls'",
				},
				"direction": map[string]interface{}{
					"type":        "string",
					"description": `Relationship direction: "in", "out", or "both". Default: "both"`,
					"default":     "both",
					"enum":        []string{"in", "out", "both"},
				},
				"detail": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to return detailed information",
					"default":     false,
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID, e.g. 'cn-hongkong'",
				},
			},
			"required": []string{"domain", "entity_set_name", "workspace", "regionId"},
		},
		Handler: h.handleListRelatedEntitySet,
	}
}

func (h *datasetHandler) handleListRelatedEntitySet(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	domain := paramString(params, "domain", "")
	entitySetName := paramString(params, "entity_set_name", "")
	workspace := paramString(params, "workspace", "")
	relationType := paramString(params, "relation_type", "")
	direction := paramString(params, "direction", "both")
	regionID := paramString(params, "regionId", "")

	// Parse detail as bool
	detail := false
	if v, ok := params["detail"]; ok && v != nil {
		if b, ok := v.(bool); ok {
			detail = b
		}
	}

	if domain == "" || entitySetName == "" || workspace == "" || regionID == "" {
		return buildStandardResponse(nil, "", 0, 0, true,
			"domain, entity_set_name, workspace and regionId are required", "last_1h"), nil
	}

	fromTS, toTS, err := parseTimeRange("last_1h")
	if err != nil {
		return buildStandardResponse(nil, "", 0, 0, true, err.Error(), "last_1h"), nil
	}

	// Build SPL parameters matching Python implementation
	relationTypeParam := "''"
	if relationType != "" {
		relationTypeParam = "'" + relationType + "'"
	}
	directionParam := "'" + direction + "'"
	detailParam := "false"
	if detail {
		detailParam = "true"
	}

	query := fmt.Sprintf(
		".entity_set with(domain='%s', name='%s') | entity-call list_related_entity_set(%s, %s, %s)",
		domain, entitySetName, relationTypeParam, directionParam, detailParam,
	)

	slog.InfoContext(ctx, "umodel_list_related_entity_set", "workspace", workspace, "domain", domain, "entity_set_name", entitySetName, "direction", direction, "region", regionID)

	result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, 1000)
	if err != nil {
		slog.ErrorContext(ctx, "umodel_list_related_entity_set failed", "error", err)
		return buildStandardResponse(nil, query, fromTS, toTS, true,
			fmt.Sprintf("Failed to list related entity sets: %s", err), "last_1h"), nil
	}

	data := result["data"]
	return buildStandardResponse(data, query, fromTS, toTS, false, "", "last_1h"), nil
}

// ===========================================================================
// Tool 4: umodel_get_entity_set
// ===========================================================================

func (h *datasetHandler) getEntitySetTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "umodel_get_entity_set",
		Description: `Get the schema definition of a specified entity set, including field list, field types, and other structural information.

## Overview

Retrieves the metadata definition (schema) of a specific EntitySet to understand its structure and available fields.
Unlike umodel_search_entity_set (full-text search), this tool retrieves the complete definition of a single entity set by exact domain and name.

## Use Cases

- **Structure Understanding**: Discover available fields and field types of an entity set for subsequent filter queries
- **Field Discovery**: Get the list of available entity fields for building umodel_get_entities filter conditions
- **Metadata Viewing**: View the complete schema definition of an entity set

## Parameters

- domain: Entity domain, e.g. 'apm', 'k8s', 'acs'
- entity_set_name: Entity set name, e.g. 'apm.service', 'k8s.pod'
- detail: Whether to return the full detailed JSON Schema, default false returns summary`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "Entity domain, e.g. 'apm', 'k8s', 'acs'",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "Entity set name, e.g. 'apm.service', 'k8s.pod'",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS workspace name, obtainable via list_workspace",
				},
				"detail": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to return the full detailed JSON Schema, default false returns summary",
					"default":     false,
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID, e.g. 'cn-hongkong'",
				},
			},
			"required": []string{"domain", "entity_set_name", "workspace", "regionId"},
		},
		Handler: h.handleGetEntitySet,
	}
}

func (h *datasetHandler) handleGetEntitySet(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	domain := paramString(params, "domain", "")
	entitySetName := paramString(params, "entity_set_name", "")
	workspace := paramString(params, "workspace", "")
	regionID := paramString(params, "regionId", "")

	detail := false
	if v, ok := params["detail"]; ok && v != nil {
		if b, ok := v.(bool); ok {
			detail = b
		}
	}

	if domain == "" || entitySetName == "" || workspace == "" || regionID == "" {
		return buildStandardResponse(nil, "", 0, 0, true,
			"domain, entity_set_name, workspace and regionId are required", "last_1h"), nil
	}

	fromTS, toTS, err := parseTimeRange("last_1h")
	if err != nil {
		return buildStandardResponse(nil, "", 0, 0, true, err.Error(), "last_1h"), nil
	}

	detailParam := "false"
	if detail {
		detailParam = "true"
	}

	query := fmt.Sprintf(
		".entity_set with(domain='%s', name='%s') | entity-call get_entity_set(%s)",
		domain, entitySetName, detailParam,
	)

	slog.InfoContext(ctx, "umodel_get_entity_set", "workspace", workspace, "domain", domain, "entity_set_name", entitySetName, "detail", detail, "region", regionID)

	result, err := h.cmsClient.ExecuteSPL(ctx, regionID, workspace, query, fromTS, toTS, 1000)
	if err != nil {
		slog.ErrorContext(ctx, "umodel_get_entity_set failed", "error", err)
		return buildStandardResponse(nil, query, fromTS, toTS, true,
			fmt.Sprintf("Failed to get entity set schema: %s", err), "last_1h"), nil
	}

	data := result["data"]
	return buildStandardResponse(data, query, fromTS, toTS, false, "", "last_1h"), nil
}
