package iaas

import (
	"github.com/alibabacloud-observability-mcp-server-go/pkg/client"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/toolkit"
)

// IaaSToolkit groups all IaaS-layer tools: SLS direct access tools (sls_* prefix)
// and CMS direct access tools (cms_* prefix).
type IaaSToolkit struct {
	tools []toolkit.Tool
}

// NewIaaSToolkit creates an IaaSToolkit that aggregates all IaaS sub-tools
// backed by the given SLS and CMS clients.
func NewIaaSToolkit(slsClient client.SLSClient, cmsClient client.CMSClient) *IaaSToolkit {
	var tools []toolkit.Tool
	tools = append(tools, SLSTools(slsClient, cmsClient)...)
	tools = append(tools, CMSTools(cmsClient, slsClient)...)
	return &IaaSToolkit{tools: tools}
}

// Name returns "iaas".
func (t *IaaSToolkit) Name() string { return "iaas" }

// Tools returns all IaaS tools.
func (t *IaaSToolkit) Tools() []toolkit.Tool { return t.tools }
