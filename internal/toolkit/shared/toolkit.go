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
		Description: `List available CMS workspaces.

## Overview
Retrieve the list of available Cloud Monitor Service (CMS) workspaces in a specified region.
A workspace is a logical container in CMS for organizing and managing monitoring data.

## Parameters
- regionId: Alibaba Cloud region identifier, e.g. "cn-hongkong", "cn-beijing"

## Response
Returns a dictionary containing workspace information:
- workspaces: List of workspaces, each including name, ID, description, etc.
- total_count: Total number of workspaces
- region: The queried region ID

## Use Cases
- Retrieve available workspaces before using PaaS-layer APIs
- Provide workspace selection for DoAI-layer queries
- Manage and monitor resource usage across multiple workspaces

## Notes
- Workspaces in different regions are independent
- Workspace visibility depends on the current user's permissions
- This is a foundational tool that provides workspace selection for other PaaS and DoAI tools`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID",
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
		Description: `List all available entity domains.

## Overview
Retrieve all available entity domains in the system. An entity domain is the top-level
classification of entities, such as APM, Kubernetes, cloud products, etc.
This is the first step to discover supported entity types.

## Use Cases
- Discover all supported entity domains
- Select the correct domain parameter for subsequent queries
- Build dynamic domain selection interfaces

## Response
Each domain contains:
- __domain__: Domain name (e.g. apm, k8s, cloud)
- cnt: Total number of entities in the domain`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "CMS workspace name, obtainable via list_workspace",
				},
				"regionId": map[string]interface{}{
					"type":        "string",
					"description": "Alibaba Cloud region ID",
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
		Description: `Get the introduction and usage guide for Alibaba Cloud Observability MCP Server.

## Overview
Returns the service overview, core capabilities, and usage constraints of the
Alibaba Cloud Observability MCP Server. Helps users quickly understand what the
service can do and the prerequisites for each tool layer.

## Use Cases
- Learn about service capabilities and constraints on first use
- Understand prerequisites for different tool layers

## Notes
- This tool requires no parameters and can be called directly
- The response includes prerequisites for each tool layer`,
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
		"description": "Alibaba Cloud Observability MCP Server - AI-driven observability data access",
		"capabilities": map[string]interface{}{
			"data_access": []string{
				"Query log data (SLS Log Store)",
				"Query metric data (time-series metrics)",
				"Query trace data (distributed tracing)",
				"Query event data (anomaly events)",
				"Query entity information (applications, containers, cloud products, etc.)",
				"Profiling data queries",
			},
			"ai_features": []string{
				"Natural language to SQL queries",
				"Natural language to PromQL queries",
				"Intelligent entity discovery and relationship analysis",
			},
		},
		"tool_layers": map[string]interface{}{
			"paas": map[string]interface{}{
				"description": "PaaS layer toolkit (recommended) - modern observability powered by Cloud Monitor 2.0",
				"capabilities": []string{
					"Entity discovery and management",
					"Unified queries for metrics, logs, events, traces, and profiling",
					"Dataset and metadata management",
				},
				"prerequisites": "⚠️ Requires Alibaba Cloud Monitor 2.0 service activation",
				"note":          "Suitable for scenarios requiring unified data models and entity relationship analysis",
			},
			"iaas": map[string]interface{}{
				"description": "IaaS layer toolkit - direct access to underlying storage services",
				"capabilities": []string{
					"Direct SLS Log Store queries",
					"Direct SLS Metric Store queries",
					"Execute native SQL/PromQL queries",
					"Log store and project management",
				},
				"prerequisites": "✓ No Cloud Monitor 2.0 required, only SLS service permissions needed",
				"note":          "Suitable for direct SLS data access or scenarios not dependent on Cloud Monitor 2.0",
			},
			"shared": map[string]interface{}{
				"description": "Shared toolkit - basic service discovery and management",
				"capabilities": []string{
					"Workspace management",
					"Entity domain discovery",
					"Service introduction",
				},
				"prerequisites": "✓ Available for all scenarios",
			},
		},
		"important_notes": []string{
			"PaaS tools (umodel_* series) depend on Cloud Monitor 2.0 and require service activation",
			"IaaS tools (sls_* series) access SLS directly without Cloud Monitor 2.0",
			"PaaS tools are recommended for better entity relationships and unified data model experience",
			"If Cloud Monitor 2.0 is not activated, use IaaS tools to query SLS data directly",
		},
		"references": map[string]interface{}{
			"cloudmonitor_2_0": "https://help.aliyun.com/zh/cms/cloudmonitor-2-0/product-overview/what-is-cloud-monitor-2-0",
			"sls_overview":     "https://help.aliyun.com/zh/sls/",
			"github":           "https://github.com/aliyun/alibabacloud-observability-mcp-server",
		},
	}, nil
}
