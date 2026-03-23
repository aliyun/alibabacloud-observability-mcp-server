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
	// 12 SLS (11 main + 1 deprecated alias) + 2 CMS = 14
	if got := len(tk.Tools()); got != 14 {
		t.Errorf("Tools() returned %d tools, want 14", got)
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
	// 12 from SLSTools (11 main + 1 deprecated alias)
	if slsCount != 12 {
		t.Errorf("expected 12 sls_ tools, got %d", slsCount)
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
	// 2 CMS tools: cms_execute_promql + cms_text_to_promql
	if cmsCount != 2 {
		t.Errorf("expected 2 cms_ tools, got %d", cmsCount)
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
