package server

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	sls "github.com/alibabacloud-go/sls-20201230/v6/client"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/client"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/config"
	"github.com/alibabacloud-observability-mcp-server-go/pkg/toolkit"
	"github.com/mark3labs/mcp-go/mcp"
)

// stubSLSClient implements client.SLSClient for testing.
type stubSLSClient struct{}

func (s *stubSLSClient) Query(_ context.Context, _, _, _ string, _ *sls.GetLogsRequest) ([]map[string]interface{}, error) {
	return nil, nil
}
func (s *stubSLSClient) GetContextLogs(_ context.Context, _, _, _, _, _ string, _, _ int) (map[string]interface{}, error) {
	return nil, nil
}
func (s *stubSLSClient) ListProjects(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (s *stubSLSClient) ListProjectsWithFilter(_ context.Context, _, _ string, _ int) ([]map[string]interface{}, error) {
	return nil, nil
}
func (s *stubSLSClient) ListLogStores(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}
func (s *stubSLSClient) ListLogStoresWithFilter(_ context.Context, _, _, _ string, _ int, _ bool) ([]string, error) {
	return nil, nil
}
func (s *stubSLSClient) ListMetricStores(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}
func (s *stubSLSClient) TextToSQL(_ context.Context, _, _, _, _ string) (string, error) {
	return "", nil
}

// stubCMSClient implements client.CMSClient for testing.
type stubCMSClient struct{}

func (s *stubCMSClient) ExecuteSPL(_ context.Context, _, _, _ string, _, _ int64, _ int) (map[string]interface{}, error) {
	return nil, nil
}
func (s *stubCMSClient) ListWorkspaces(_ context.Context, _ string) ([]map[string]interface{}, error) {
	return nil, nil
}
func (s *stubCMSClient) QueryMetric(_ context.Context, _, _, _ string, _ map[string]string, _, _ int64) ([]map[string]interface{}, error) {
	return nil, nil
}
func (s *stubCMSClient) TextToSQL(_ context.Context, _, _, _, _ string) (string, error) {
	return "", nil
}

func (s *stubCMSClient) ChatWithSkill(_ context.Context, _, _, _, _, _ string) (string, error) {
	return "", nil
}

func (s *stubCMSClient) DataAgentQuery(_ context.Context, _, _, _ string, _, _ int64) (*client.DataAgentResult, error) {
	return &client.DataAgentResult{}, nil
}

// Compile-time interface checks.
var _ client.SLSClient = (*stubSLSClient)(nil)
var _ client.CMSClient = (*stubCMSClient)(nil)

func newTestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Transport: "stdio",
			Host:      "0.0.0.0",
			Port:      8080,
		},
		Logging: config.LoggingConfig{
			Level:     "info",
			DebugMode: false,
		},
		Toolkit: config.ToolkitConfig{
			Scope: "all",
		},
		Network: config.NetworkConfig{
			MaxRetry: 1,
		},
	}
}

// fakeToolkit is a minimal Toolkit implementation for testing.
type fakeToolkit struct {
	name  string
	tools []toolkit.Tool
}

func (f *fakeToolkit) Name() string          { return f.name }
func (f *fakeToolkit) Tools() []toolkit.Tool { return f.tools }

func TestNewServer_NilConfig(t *testing.T) {
	reg := toolkit.NewRegistry()
	_, err := NewServer(nil, reg, &stubSLSClient{}, &stubCMSClient{})
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestNewServer_NilRegistry(t *testing.T) {
	cfg := newTestConfig()
	_, err := NewServer(cfg, nil, &stubSLSClient{}, &stubCMSClient{})
	if err == nil {
		t.Fatal("expected error for nil registry")
	}
}

func TestNewServer_EmptyRegistry(t *testing.T) {
	cfg := newTestConfig()
	reg := toolkit.NewRegistry()
	srv, err := NewServer(cfg, reg, &stubSLSClient{}, &stubCMSClient{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if srv.MCPServer() == nil {
		t.Fatal("expected non-nil MCPServer")
	}
}

func TestNewServer_ToolRegistration(t *testing.T) {
	cfg := newTestConfig()
	reg := toolkit.NewRegistry()

	tk := &fakeToolkit{
		name: "test",
		tools: []toolkit.Tool{
			{
				Name:        "test_tool_alpha",
				Description: "Alpha tool",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "A name",
						},
					},
					"required": []string{"name"},
				},
				Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
					return "hello", nil
				},
			},
			{
				Name:        "test_tool_beta",
				Description: "Beta tool",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
				Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
					return map[string]interface{}{"ok": true}, nil
				},
			},
		},
	}
	reg.Register(tk)

	srv, err := NewServer(cfg, reg, &stubSLSClient{}, &stubCMSClient{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if srv == nil {
		t.Fatal("expected non-nil server")
	}

	// Verify the registry tools match what we registered.
	registeredTools := reg.List()
	if len(registeredTools) != 2 {
		t.Fatalf("expected 2 tools in registry, got %d", len(registeredTools))
	}

	names := make(map[string]bool)
	for _, tool := range registeredTools {
		names[tool.Name] = true
	}
	if !names["test_tool_alpha"] {
		t.Error("expected test_tool_alpha in registry")
	}
	if !names["test_tool_beta"] {
		t.Error("expected test_tool_beta in registry")
	}
}

func TestConvertTool(t *testing.T) {
	tests := []struct {
		name    string
		tool    toolkit.Tool
		wantErr bool
	}{
		{
			name: "tool with full schema",
			tool: toolkit.Tool{
				Name:        "my_tool",
				Description: "A test tool",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"region": map[string]interface{}{
							"type":        "string",
							"description": "Region ID",
						},
					},
					"required": []string{"region"},
				},
			},
			wantErr: false,
		},
		{
			name: "tool with nil schema",
			tool: toolkit.Tool{
				Name:        "no_schema_tool",
				Description: "No schema",
				InputSchema: nil,
			},
			wantErr: false,
		},
		{
			name: "tool with empty schema",
			tool: toolkit.Tool{
				Name:        "empty_schema_tool",
				Description: "Empty schema",
				InputSchema: map[string]interface{}{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mcpTool, err := convertTool(tt.tool)
			if (err != nil) != tt.wantErr {
				t.Fatalf("convertTool() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if mcpTool.Name != tt.tool.Name {
				t.Errorf("name = %q, want %q", mcpTool.Name, tt.tool.Name)
			}
			if mcpTool.Description != tt.tool.Description {
				t.Errorf("description = %q, want %q", mcpTool.Description, tt.tool.Description)
			}
		})
	}
}

func TestConvertTool_SchemaPreserved(t *testing.T) {
	tool := toolkit.Tool{
		Name:        "schema_test",
		Description: "Schema preservation test",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workspace": map[string]interface{}{
					"type":        "string",
					"description": "Workspace name",
				},
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "Max results",
				},
			},
			"required": []string{"workspace"},
		},
	}

	mcpTool, err := convertTool(tool)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The raw schema should be set (NewToolWithRawSchema stores it in RawInputSchema).
	// Marshal the tool to JSON and verify the schema is preserved.
	toolJSON, err := json.Marshal(mcpTool)
	if err != nil {
		t.Fatalf("failed to marshal mcp tool: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(toolJSON, &parsed); err != nil {
		t.Fatalf("failed to unmarshal tool JSON: %v", err)
	}

	schema, ok := parsed["inputSchema"].(map[string]interface{})
	if !ok {
		t.Fatal("inputSchema not found or not an object")
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties not found or not an object")
	}

	if _, ok := props["workspace"]; !ok {
		t.Error("workspace property not preserved in schema")
	}
	if _, ok := props["limit"]; !ok {
		t.Error("limit property not preserved in schema")
	}
}

func TestFormatResult(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		wantText string
		wantErr  bool
	}{
		{
			name:     "nil result",
			input:    nil,
			wantText: "",
		},
		{
			name:     "string result",
			input:    "hello world",
			wantText: "hello world",
		},
		{
			name:     "map result",
			input:    map[string]interface{}{"key": "value"},
			wantText: `{"key":"value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatResult(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("formatResult() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if len(result.Content) == 0 {
				t.Fatal("expected at least one content item")
			}
			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatalf("expected TextContent, got %T", result.Content[0])
			}
			if textContent.Text != tt.wantText {
				t.Errorf("text = %q, want %q", textContent.Text, tt.wantText)
			}
		})
	}
}

func TestWrapHandler_Success(t *testing.T) {
	cfg := newTestConfig()
	reg := toolkit.NewRegistry()

	called := false
	tk := &fakeToolkit{
		name: "test",
		tools: []toolkit.Tool{
			{
				Name:        "success_tool",
				Description: "Returns success",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
				Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
					called = true
					return "ok", nil
				},
			},
		},
	}
	reg.Register(tk)

	srv, err := NewServer(cfg, reg, &stubSLSClient{}, &stubCMSClient{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get the wrapped handler and call it directly.
	tool := tk.Tools()[0]
	wrappedHandler := srv.wrapHandler(tool)

	result, err := wrappedHandler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("handler was not called")
	}
	if result.IsError {
		t.Error("expected no error in result")
	}
}

func TestWrapHandler_Error(t *testing.T) {
	cfg := newTestConfig()
	reg := toolkit.NewRegistry()

	tk := &fakeToolkit{
		name: "test",
		tools: []toolkit.Tool{
			{
				Name:        "error_tool",
				Description: "Returns error",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
				Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
					return nil, fmt.Errorf("something went wrong")
				},
			},
		},
	}
	reg.Register(tk)

	srv, err := NewServer(cfg, reg, &stubSLSClient{}, &stubCMSClient{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tool := tk.Tools()[0]
	wrappedHandler := srv.wrapHandler(tool)

	result, err := wrappedHandler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error from wrapper: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true in result")
	}
}

func TestWrapHandler_PanicRecovery(t *testing.T) {
	cfg := newTestConfig()
	reg := toolkit.NewRegistry()

	tk := &fakeToolkit{
		name: "test",
		tools: []toolkit.Tool{
			{
				Name:        "panic_tool",
				Description: "Panics",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
				Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
					panic("unexpected panic")
				},
			},
		},
	}
	reg.Register(tk)

	srv, err := NewServer(cfg, reg, &stubSLSClient{}, &stubCMSClient{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tool := tk.Tools()[0]
	wrappedHandler := srv.wrapHandler(tool)

	// The panic should be recovered; the function should not crash.
	// Note: after panic recovery, the deferred recover runs but the handler
	// returns zero values since the panic interrupts normal flow.
	func() {
		defer func() {
			// If the test itself panics, that means recovery didn't work.
			if r := recover(); r != nil {
				t.Errorf("panic was not recovered: %v", r)
			}
		}()
		_, _ = wrappedHandler(context.Background(), mcp.CallToolRequest{})
	}()
}

func TestServer_Shutdown(t *testing.T) {
	cfg := newTestConfig()
	reg := toolkit.NewRegistry()
	srv, err := NewServer(cfg, reg, &stubSLSClient{}, &stubCMSClient{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = srv.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}
}

func TestApplyRuntimeDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		region    string
		workspace string
		params    map[string]interface{}
		wantR     string
		wantW     string
	}{
		{
			name:      "fills missing regionId and workspace",
			region:    "cn-hongkong",
			workspace: "default-ws",
			params:    map[string]interface{}{},
			wantR:     "cn-hongkong",
			wantW:     "default-ws",
		},
		{
			name:      "fills empty string regionId and workspace",
			region:    "cn-beijing",
			workspace: "ws-2",
			params:    map[string]interface{}{"regionId": "", "workspace": ""},
			wantR:     "cn-beijing",
			wantW:     "ws-2",
		},
		{
			name:      "does not overwrite user-provided values",
			region:    "cn-hongkong",
			workspace: "default-ws",
			params:    map[string]interface{}{"regionId": "cn-shanghai", "workspace": "my-ws"},
			wantR:     "cn-shanghai",
			wantW:     "my-ws",
		},
		{
			name:      "no-op when runtime config is empty",
			region:    "",
			workspace: "",
			params:    map[string]interface{}{},
			wantR:     "",
			wantW:     "",
		},
		{
			name:      "fills only regionId when workspace config is empty",
			region:    "cn-hongkong",
			workspace: "",
			params:    map[string]interface{}{},
			wantR:     "cn-hongkong",
			wantW:     "",
		},
		{
			name:      "does not touch non-string param values",
			region:    "cn-hongkong",
			workspace: "default-ws",
			params:    map[string]interface{}{"regionId": 123, "workspace": true},
			wantR:     "",
			wantW:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := newTestConfig()
			cfg.Runtime = config.RuntimeConfig{
				Region:    tt.region,
				Workspace: tt.workspace,
			}
			srv := &Server{cfg: cfg}

			srv.applyRuntimeDefaults(tt.params)

			gotR, _ := tt.params["regionId"].(string)
			gotW, _ := tt.params["workspace"].(string)

			if gotR != tt.wantR {
				t.Errorf("regionId = %q, want %q", gotR, tt.wantR)
			}
			if gotW != tt.wantW {
				t.Errorf("workspace = %q, want %q", gotW, tt.wantW)
			}
		})
	}
}
