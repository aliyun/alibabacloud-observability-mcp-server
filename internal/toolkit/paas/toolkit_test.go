package paas

import (
	"strings"
	"testing"

	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit"
)

func TestPaaSToolkit_Name(t *testing.T) {
	tk := NewPaaSToolkit(&mockCMSClient{})
	if got := tk.Name(); got != "paas" {
		t.Errorf("Name() = %q, want %q", got, "paas")
	}
}

func TestPaaSToolkit_ToolCount(t *testing.T) {
	tk := NewPaaSToolkit(&mockCMSClient{})
	// 3 entity + 9 data (8 main + 1 alias) + 1 timeseries + 3 dataset + 2 data_agent (1 main + 1 alias) = 18
	if got := len(tk.Tools()); got != 18 {
		t.Errorf("Tools() returned %d tools, want 18", got)
	}
}

func TestPaaSToolkit_AllToolsHaveUmodelPrefix(t *testing.T) {
	tk := NewPaaSToolkit(&mockCMSClient{})
	for _, tool := range tk.Tools() {
		// Allow both umodel_ prefix and cms_natural_language_query alias
		if tool.Name != "cms_natural_language_query" && !strings.HasPrefix(tool.Name, "umodel_") {
			t.Errorf("tool %q does not have umodel_ prefix", tool.Name)
		}
	}
}

func TestPaaSToolkit_ImplementsToolkit(t *testing.T) {
	var _ toolkit.Toolkit = NewPaaSToolkit(&mockCMSClient{})
}
