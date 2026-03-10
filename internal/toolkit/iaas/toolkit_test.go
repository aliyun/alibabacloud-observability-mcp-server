package iaas

import (
	"strings"
	"testing"

	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit"
)

func TestIaaSToolkit_Name(t *testing.T) {
	tk := NewIaaSToolkit(&mockSLSClient{}, &mockCMSClient{})
	if got := tk.Name(); got != "iaas" {
		t.Errorf("Name() = %q, want %q", got, "iaas")
	}
}

func TestIaaSToolkit_ToolCount(t *testing.T) {
	tk := NewIaaSToolkit(&mockSLSClient{}, &mockCMSClient{})
	// 15 SLS (14 main + 1 deprecated alias) + 6 CMS (4 main + 2 aliases) = 21
	if got := len(tk.Tools()); got != 21 {
		t.Errorf("Tools() returned %d tools, want 21", got)
	}
}

func TestIaaSToolkit_SLSToolsHavePrefix(t *testing.T) {
	tk := NewIaaSToolkit(&mockSLSClient{}, &mockCMSClient{})
	slsCount := 0
	for _, tool := range tk.Tools() {
		if strings.HasPrefix(tool.Name, "sls_") {
			slsCount++
		}
	}
	// 15 from SLSTools (14 main + 1 deprecated alias) + 1 from CMSTools (sls_query_metricstore for PromQL) = 16
	if slsCount != 16 {
		t.Errorf("expected 16 sls_ tools, got %d", slsCount)
	}
}

func TestIaaSToolkit_CMSToolsHavePrefix(t *testing.T) {
	tk := NewIaaSToolkit(&mockSLSClient{}, &mockCMSClient{})
	cmsCount := 0
	for _, tool := range tk.Tools() {
		if strings.HasPrefix(tool.Name, "cms_") {
			cmsCount++
		}
	}
	// 3 main cms_ tools + 2 aliases (cms_execute_promql, cms_text_to_promql) = 5
	if cmsCount != 5 {
		t.Errorf("expected 5 cms_ tools, got %d", cmsCount)
	}
}

func TestIaaSToolkit_AllToolsHaveValidPrefix(t *testing.T) {
	tk := NewIaaSToolkit(&mockSLSClient{}, &mockCMSClient{})
	for _, tool := range tk.Tools() {
		if !strings.HasPrefix(tool.Name, "sls_") && !strings.HasPrefix(tool.Name, "cms_") {
			t.Errorf("tool %q does not have sls_ or cms_ prefix", tool.Name)
		}
	}
}

func TestIaaSToolkit_ImplementsToolkit(t *testing.T) {
	var _ toolkit.Toolkit = NewIaaSToolkit(&mockSLSClient{}, &mockCMSClient{})
}
