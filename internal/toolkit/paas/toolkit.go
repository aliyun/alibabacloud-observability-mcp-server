// Package paas implements the PaaS Toolkit, providing tools backed by
// Cloud Monitor Service 2.0 unified data model (umodel_* prefix).
package paas

import (
	"github.com/alibabacloud-observability-mcp-server-go/internal/client"
	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit"
)

// PaaSToolkit groups all PaaS-layer tools: entity management, data queries,
// timeseries comparison, dataset management, and natural language queries.
type PaaSToolkit struct {
	tools []toolkit.Tool
}

// NewPaaSToolkit creates a PaaSToolkit that aggregates all PaaS sub-tools
// backed by the given CMS client.
func NewPaaSToolkit(cmsClient client.CMSClient) *PaaSToolkit {
	var tools []toolkit.Tool
	tools = append(tools, EntityTools(cmsClient)...)
	tools = append(tools, DataTools(cmsClient)...)
	tools = append(tools, DatasetTools(cmsClient)...)
	tools = append(tools, DataAgentTools(cmsClient)...)
	return &PaaSToolkit{tools: tools}
}

// Name returns "paas".
func (p *PaaSToolkit) Name() string { return "paas" }

// Tools returns all PaaS tools.
func (p *PaaSToolkit) Tools() []toolkit.Tool { return p.tools }
