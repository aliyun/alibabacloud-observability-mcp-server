package iaas

import (
	"strings"
	"testing"

	"github.com/alibabacloud-observability-mcp-server-go/pkg/toolkit"
)

func TestIaaSToolkit_Name(t *testing.T) {
	tk := NewIaaSToolkit(&mockSLSClient{}, &mockCMSClient{})
	if got := tk.Name(); got != "iaas" {
		t.Errorf("Name() = %q, want %q", got, "iaas")
	}
}

func TestIaaSToolkit_ToolCount(t *testing.T) {
	tk := NewIaaSToolkit(&mockSLSClient{}, &mockCMSClient{})
	expectedSLS := len(expectedSLSToolNames())
	expectedCMS := len(expectedCMSToolNames())
	wantTotal := expectedSLS + expectedCMS
	if got := len(tk.Tools()); got != wantTotal {
		t.Errorf("Tools() returned %d tools, want %d (SLS=%d + CMS=%d)", got, wantTotal, expectedSLS, expectedCMS)
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
	wantSLS := len(expectedSLSToolNames())
	if slsCount != wantSLS {
		t.Errorf("expected %d sls_ tools, got %d", wantSLS, slsCount)
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
	wantCMS := len(expectedCMSToolNames())
	if cmsCount != wantCMS {
		t.Errorf("expected %d cms_ tools, got %d", wantCMS, cmsCount)
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
