package paas

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/alibabacloud-observability-mcp-server-go/internal/client"
	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit"
)

// DatasetTools returns all dataset management tools backed by the given CMS client.
func DatasetTools(cmsClient client.CMSClient) []toolkit.Tool {
	h := &datasetHandler{cmsClient: cmsClient}
	return []toolkit.Tool{
		h.listDataSetTool(),
		h.searchEntitySetTool(),
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
		Description: `列出指定实体的可用数据集合，为其他PaaS工具提供参数选项。

## 功能概述

该工具是一个元数据查询接口，用于获取指定实体域和类型下可用的数据集合信息。
主要作用是为其他PaaS层工具（如observability、entity等工具）提供可选的参数列表，
包括指标集合、日志集合、事件集合等存储信息。

## 使用场景

- **参数发现**: 为 umodel_get_metrics 提供可用的指标集合（metric_set）列表
- **日志源查询**: 为 umodel_get_logs 提供可用的日志集合（log_set）列表
- **事件源发现**: 为 umodel_get_events 提供可用的事件集合（event_set）列表
- **追踪数据源**: 为 umodel_get_traces 提供可用的追踪集合（trace_set）列表

## 参数说明

- data_set_types: 数据集合类型过滤器，常见类型包括：
  * 'metric_set': 指标集合
  * 'log_set': 日志集合
  * 'event_set': 事件集合
  * 'trace_set': 追踪集合
  * 'profile_set': 性能剖析集合`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS工作空间名称，可通过list_workspace获取",
				},
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "实体域, 不能为 '*'",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "实体类型, 不能为 '*'",
				},
				"data_set_types": map[string]interface{}{
					"type":        "string",
					"description": "逗号分隔的数据集合类型过滤器，如 'metric_set,log_set'",
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": timeRangeDescription,
					"default":     "last_5m",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
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
		Description: `搜索实体集合，支持全文搜索并按相关度排序。

## 功能概述

该工具用于在UModel元数据中搜索实体集合定义，支持按关键词进行全文搜索。
主要用于发现可用的实体集合类型和它们的元数据信息。

## 功能特点

- **全文搜索**: 支持在实体集合的元数据和规格中进行全文搜索
- **相关度排序**: 搜索结果按相关度进行排序
- **可选过滤**: 支持按domain和name进行额外过滤

## 使用场景

- **实体集合发现**: 搜索包含特定关键词的实体集合类型
- **元数据探索**: 了解系统中可用的实体集合及其描述信息`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"search_text": map[string]interface{}{
					"type":        "string",
					"description": "搜索关键词，用于全文搜索",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS工作空间名称，可通过list_workspace获取",
				},
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "可选的实体域过滤",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "可选的实体类型过滤",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "返回多少个实体集合，默认10个",
					"default":     10,
					"minimum":     1,
					"maximum":     100,
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
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
		Description: `列出与指定实体集合相关的其他实体集合。

## 功能概述

该工具用于发现与指定实体集合存在关系定义的其他实体集合类型。
这是一个元数据级别的工具，用于探索UModel拓扑的高级蓝图。

## 功能特点

- **关系发现**: 查找与源实体集合有关系定义的其他实体集合
- **方向控制**: 支持查看入向、出向或双向关系
- **类型过滤**: 可按特定关系类型进行过滤

## 使用场景

- **拓扑探索**: 了解实体集合间可能存在的关系类型
- **依赖分析**: 发现服务可以调用的其他实体类型`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{
					"type":        "string",
					"description": "实体域，如 'apm'",
				},
				"entity_set_name": map[string]interface{}{
					"type":        "string",
					"description": "实体类型，如 'apm.service'",
				},
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS工作空间名称，可通过list_workspace获取",
				},
				"relation_type": map[string]interface{}{
					"type":        "string",
					"description": "关系类型过滤，如 'calls'",
				},
				"direction": map[string]interface{}{
					"type":        "string",
					"description": `关系方向: "in", "out", 或 "both"。默认: "both"`,
					"default":     "both",
					"enum":        []string{"in", "out", "both"},
				},
				"detail": map[string]interface{}{
					"type":        "boolean",
					"description": "是否返回详细信息",
					"default":     false,
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID，如 'cn-hangzhou'",
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
