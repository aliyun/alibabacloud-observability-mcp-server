// Package shared implements the Shared Toolkit, providing common tools
// used across both PaaS and IaaS layers: workspace management, entity
// domain discovery, and service introduction.
package shared

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/alibabacloud-observability-mcp-server-go/internal/client"
	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/timeparse"
)

// SharedToolkit groups the shared tools: list_workspace, list_domains, introduction.
type SharedToolkit struct {
	cmsClient client.CMSClient
}

// New creates a SharedToolkit backed by the given CMS client.
func New(cmsClient client.CMSClient) *SharedToolkit {
	return &SharedToolkit{cmsClient: cmsClient}
}

// Name returns "shared".
func (s *SharedToolkit) Name() string { return "shared" }

// Tools returns all shared tools.
func (s *SharedToolkit) Tools() []toolkit.Tool {
	return []toolkit.Tool{
		s.listWorkspaceTool(),
		s.listDomainsTool(),
		s.introductionTool(),
	}
}

// listWorkspaceTool returns the list_workspace tool definition.
func (s *SharedToolkit) listWorkspaceTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "list_workspace",
		Description: `列出可用的CMS工作空间

## 功能概述
获取指定区域内可用的Cloud Monitor Service (CMS)工作空间列表。
工作空间是CMS中用于组织和管理监控数据的逻辑容器。

## 参数说明
- regionId: 阿里云区域标识符，如 "cn-hangzhou", "cn-beijing" 等

## 返回结果
返回包含工作空间信息的字典，包括：
- workspaces: 工作空间列表，每个工作空间包含名称、ID、描述等信息
- total_count: 工作空间总数
- region: 查询的区域ID

## 使用场景
- 在使用PaaS层API之前，需要先获取可用的工作空间
- 为DoAI层查询提供工作空间选择
- 管理和监控多个工作空间的资源使用情况

## 注意事项
- 不同区域的工作空间是独立的
- 工作空间的可见性取决于当前用户的权限
- 这是一个基础工具，为其他PaaS和DoAI工具提供工作空间选择`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID",
				},
			},
			"required": []string{"regionId"},
		},
		Handler: s.handleListWorkspace,
	}
}

// handleListWorkspace calls the CMS API to list workspaces in the given region.
func (s *SharedToolkit) handleListWorkspace(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	regionID, _ := params["regionId"].(string)
	if regionID == "" {
		return map[string]interface{}{
			"error":       true,
			"workspaces":  []interface{}{},
			"total_count": 0,
			"region":      regionID,
			"message":     "regionId is required",
		}, nil
	}

	slog.InfoContext(ctx, "list_workspace", "region", regionID)

	workspaces, err := s.cmsClient.ListWorkspaces(ctx, regionID)
	if err != nil {
		slog.ErrorContext(ctx, "list_workspace failed", "region", regionID, "error", err)
		return map[string]interface{}{
			"error":       true,
			"workspaces":  []interface{}{},
			"total_count": 0,
			"region":      regionID,
			"message":     fmt.Sprintf("Failed to retrieve workspaces: %s", err),
		}, nil
	}

	return map[string]interface{}{
		"error":       false,
		"workspaces":  workspaces,
		"total_count": len(workspaces),
		"region":      regionID,
		"message":     fmt.Sprintf("Successfully retrieved %d workspaces from region %s", len(workspaces), regionID),
	}, nil
}

// listDomainsTool returns the list_domains tool definition.
func (s *SharedToolkit) listDomainsTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "list_domains",
		Description: `列出所有可用的实体域

## 功能概述
获取系统中所有可用的实体域（domain）列表。实体域是实体的最高级分类，
如 APM、容器、云产品等。这是发现系统支持实体类型的第一步。

## 使用场景
- 了解系统支持的所有实体域
- 为后续查询选择正确的domain参数
- 构建动态的域选择界面

## 返回数据
每个域包含：
- __domain__: 域名称（如 apm, k8s, cloud）
- cnt: 该域下的实体总数量`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS工作空间名称，可通过list_workspace获取",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "阿里云区域ID",
				},
			},
			"required": []string{"workspace", "regionId"},
		},
		Handler: s.handleListDomains,
	}
}

// listDomainsQuery is the SPL query used to discover all entity domains.
const listDomainsQuery = ".entity with(domain='*', type='*', topk=1000) | stats cnt=count(1) by __domain__ | project __domain__, cnt | sort cnt desc"

// handleListDomains executes an SPL query to retrieve all entity domains.
func (s *SharedToolkit) handleListDomains(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	workspace, _ := params["workspace"].(string)
	regionID, _ := params["regionId"].(string)

	if workspace == "" || regionID == "" {
		return map[string]interface{}{
			"error":   true,
			"data":    []interface{}{},
			"message": "workspace and regionId are required",
		}, nil
	}

	slog.InfoContext(ctx, "list_domains", "workspace", workspace, "region", regionID)

	now := time.Now()
	fromTS, _ := timeparse.ParseTimeExpression("now-24h", now)
	toTS, _ := timeparse.ParseTimeExpression("now", now)

	result, err := s.cmsClient.ExecuteSPL(ctx, regionID, workspace, listDomainsQuery, fromTS, toTS, 1000)
	if err != nil {
		slog.ErrorContext(ctx, "list_domains failed", "workspace", workspace, "region", regionID, "error", err)
		return map[string]interface{}{
			"error":   true,
			"data":    []interface{}{},
			"query":   listDomainsQuery,
			"message": fmt.Sprintf("Failed to list domains: %s", err),
		}, nil
	}

	return map[string]interface{}{
		"error":     false,
		"data":      result["data"],
		"query":     listDomainsQuery,
		"workspace": workspace,
		"message":   "success",
	}, nil
}

// introductionTool returns the introduction tool definition.
func (s *SharedToolkit) introductionTool() toolkit.Tool {
	return toolkit.Tool{
		Name: "introduction",
		Description: `获取阿里云可观测性MCP Server的介绍和使用说明

## 功能概述
返回阿里云可观测性 MCP Server 的服务概述、核心能力和使用限制说明。
帮助用户快速了解服务能做什么，以及使用各层工具的前提条件。

## 使用场景
- 首次接入时了解服务能力和限制
- 了解不同工具层的使用前提

## 注意事项
- 此工具不需要任何参数，可直接调用
- 返回信息包含各层工具的使用前提条件`,
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: handleIntroduction,
	}
}

// handleIntroduction returns a static description of the service capabilities.
func handleIntroduction(_ context.Context, _ map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{
		"name":        "Alibaba Cloud Observability MCP Server",
		"version":     "1.0.0",
		"description": "阿里云可观测性 MCP 服务 - 提供 AI 驱动的可观测数据访问能力",
		"capabilities": map[string]interface{}{
			"data_access": []string{
				"查询日志数据（SLS 日志库）",
				"查询指标数据（时序指标）",
				"查询链路数据（分布式追踪）",
				"查询事件数据（异常事件）",
				"查询实体信息（应用、容器、云产品等）",
				"性能剖析数据查询",
			},
			"ai_features": []string{
				"自然语言转 SQL 查询",
				"自然语言转 PromQL 查询",
				"智能实体发现和关系分析",
			},
		},
		"tool_layers": map[string]interface{}{
			"paas": map[string]interface{}{
				"description": "PaaS 层工具集（推荐）- 基于云监控 2.0 的现代化可观测能力",
				"capabilities": []string{
					"实体发现和管理",
					"指标、日志、事件、链路、性能剖析的统一查询",
					"数据集和元数据管理",
				},
				"prerequisites": "⚠️ 需要开通阿里云监控 2.0 服务",
				"note":          "适用于需要统一数据模型和实体关系分析的场景",
			},
			"iaas": map[string]interface{}{
				"description": "IaaS 层工具集 - 直接访问底层存储服务",
				"capabilities": []string{
					"直接查询 SLS 日志库（Log Store）",
					"直接查询 SLS 指标库（Metric Store）",
					"执行原生 SQL/PromQL 查询",
					"日志库和项目管理",
				},
				"prerequisites": "✓ 无需云监控 2.0，仅需 SLS 服务权限",
				"note":          "适用于直接访问 SLS 数据或不依赖云监控 2.0 的场景",
			},
			"shared": map[string]interface{}{
				"description": "共享工具集 - 基础服务发现和管理",
				"capabilities": []string{
					"工作空间管理",
					"实体域发现",
					"服务介绍",
				},
				"prerequisites": "✓ 所有场景可用",
			},
		},
		"important_notes": []string{
			"PaaS 层工具（umodel_* 系列）依赖云监控 2.0，需要先开通服务",
			"IaaS 层工具（sls_* 系列）直接访问 SLS，无需云监控 2.0",
			"建议优先使用 PaaS 层工具以获得更好的实体关系和统一数据模型体验",
			"如果未开通云监控 2.0，可使用 IaaS 层工具直接查询 SLS 数据",
		},
		"references": map[string]interface{}{
			"cloudmonitor_2_0": "https://help.aliyun.com/zh/cms/cloudmonitor-2-0/product-overview/what-is-cloud-monitor-2-0",
			"sls_overview":     "https://help.aliyun.com/zh/sls/",
			"github":           "https://github.com/aliyun/alibabacloud-observability-mcp-server",
		},
	}, nil
}
