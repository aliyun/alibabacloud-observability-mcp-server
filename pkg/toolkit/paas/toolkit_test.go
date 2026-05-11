package paas

import (
	"strings"
	"testing"

	"github.com/alibabacloud-observability-mcp-server-go/pkg/toolkit"
)

func TestPaaSToolkit_Name(t *testing.T) {
	tk := NewPaaSToolkit(&mockCMSClient{})
	if got := tk.Name(); got != "paas" {
		t.Errorf("Name() = %q, want %q", got, "paas")
	}
}

func TestPaaSToolkit_ToolCount(t *testing.T) {
	tk := NewPaaSToolkit(&mockCMSClient{})
	wantTotal := len(expectedEntityToolNames()) + len(expectedDataToolNames()) + len(expectedDatasetToolNames()) + len(expectedDataAgentToolNames())
	if got := len(tk.Tools()); got != wantTotal {
		t.Errorf("Tools() returned %d tools, want %d", got, wantTotal)
	}
}

func TestPaaSToolkit_AllToolsHaveUmodelPrefix(t *testing.T) {
	tk := NewPaaSToolkit(&mockCMSClient{})
	for _, tool := range tk.Tools() {
		if !strings.HasPrefix(tool.Name, "umodel_") && !strings.HasPrefix(tool.Name, "cms_") {
			t.Errorf("tool %q does not have umodel_ prefix", tool.Name)
		}
	}
}

func TestPaaSToolkit_ImplementsToolkit(t *testing.T) {
	var _ toolkit.Toolkit = NewPaaSToolkit(&mockCMSClient{})
}
