// Package server provides the MCP Server core that bridges the toolkit registry
// with the mcp-go library. It handles tool registration, lifecycle management,
// and client integration.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/alibabacloud-observability-mcp-server-go/internal/client"
	"github.com/alibabacloud-observability-mcp-server-go/internal/config"
	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit"
)

const (
	serverName    = "alibabacloud-observability-mcp-server"
	serverVersion = "0.1.0"
)

// Server wraps the mcp-go MCPServer and integrates it with the toolkit
// registry and Alibaba Cloud clients.
type Server struct {
	mcpServer *mcpserver.MCPServer
	registry  *toolkit.Registry
	slsClient client.SLSClient
	cmsClient client.CMSClient
	cfg       *config.Config
}

// NewServer creates a new Server, registers all tools from the toolkit registry
// scoped by cfg.Toolkit.Scope, and wires up tool handlers.
func NewServer(cfg *config.Config, registry *toolkit.Registry, slsClient client.SLSClient, cmsClient client.CMSClient) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("server: config must not be nil")
	}
	if registry == nil {
		return nil, fmt.Errorf("server: registry must not be nil")
	}

	mcpSrv := mcpserver.NewMCPServer(
		serverName,
		serverVersion,
		mcpserver.WithToolCapabilities(false),
	)

	s := &Server{
		mcpServer: mcpSrv,
		registry:  registry,
		slsClient: slsClient,
		cmsClient: cmsClient,
		cfg:       cfg,
	}

	if err := s.registerTools(); err != nil {
		return nil, fmt.Errorf("server: register tools: %w", err)
	}

	return s, nil
}

// MCPServer returns the underlying mcp-go MCPServer instance.
// This is used by the transport layer (Task 15.2) to create stdio/SSE/HTTP servers.
func (s *Server) MCPServer() *mcpserver.MCPServer {
	return s.mcpServer
}

// registerTools reads all tools from the registry and registers them with the
// mcp-go server. Each toolkit Tool is converted to an mcp.Tool with the
// appropriate input schema, and its handler is wrapped to bridge the toolkit
// handler signature with the mcp-go ToolHandlerFunc signature.
func (s *Server) registerTools() error {
	tools := s.registry.List()
	if len(tools) == 0 {
		slog.Warn("server: no tools registered in registry")
		return nil
	}

	for _, t := range tools {
		mcpTool, err := convertTool(t)
		if err != nil {
			return fmt.Errorf("convert tool %q: %w", t.Name, err)
		}
		handler := s.wrapHandler(t)
		s.mcpServer.AddTool(mcpTool, handler)
		slog.Debug("server: registered tool", "name", t.Name)
	}

	slog.Info("server: tools registered", "count", len(tools))
	return nil
}

// convertTool converts a toolkit.Tool to an mcp.Tool. It serializes the
// toolkit's InputSchema map to JSON and uses NewToolWithRawSchema so that
// arbitrary JSON Schema definitions are preserved exactly.
func convertTool(t toolkit.Tool) (mcp.Tool, error) {
	schema := t.InputSchema
	if schema == nil {
		schema = map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}

	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return mcp.Tool{}, fmt.Errorf("marshal input schema: %w", err)
	}

	return mcp.NewToolWithRawSchema(t.Name, t.Description, schemaBytes), nil
}

// wrapHandler creates an mcp-go ToolHandlerFunc that delegates to the toolkit
// tool's Handler. It extracts arguments from the MCP request, calls the toolkit
// handler, and converts the result to an MCP CallToolResult.
func (s *Server) wrapHandler(t toolkit.Tool) mcpserver.ToolHandlerFunc {
	handler := t.Handler
	toolName := t.Name

	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Recover from panics in tool handlers to prevent server crash.
		defer func() {
			if r := recover(); r != nil {
				slog.Error("server: panic in tool handler",
					"tool", toolName,
					"panic", fmt.Sprintf("%v", r),
				)
			}
		}()

		params := request.GetArguments()
		if params == nil {
			params = make(map[string]interface{})
		}

		// Inject default regionId and workspace from runtime config
		// when the caller does not provide them explicitly.
		s.applyRuntimeDefaults(params)

		result, err := handler(ctx, params)
		if err != nil {
			slog.Error("server: tool handler error",
				"tool", toolName,
				"error", err,
			)
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf("Error: %s", err.Error())),
				},
				IsError: true,
			}, nil
		}

		return formatResult(result)
	}
}

// applyRuntimeDefaults fills in regionId and workspace from the runtime config
// when the caller has not provided them (empty string or absent).
func (s *Server) applyRuntimeDefaults(params map[string]interface{}) {
	defaults := map[string]string{
		"regionId":  s.cfg.Runtime.Region,
		"workspace": s.cfg.Runtime.Workspace,
	}
	for key, fallback := range defaults {
		if fallback == "" {
			continue
		}
		val, exists := params[key]
		if !exists {
			params[key] = fallback
			slog.Debug("server: applied runtime default", "param", key, "value", fallback)
			continue
		}
		str, ok := val.(string)
		if ok && str == "" {
			params[key] = fallback
			slog.Debug("server: applied runtime default", "param", key, "value", fallback)
		}
	}
}

// formatResult converts a toolkit handler result into an MCP CallToolResult.
// It handles string results directly and marshals other types to JSON.
func formatResult(result interface{}) (*mcp.CallToolResult, error) {
	if result == nil {
		return mcp.NewToolResultText(""), nil
	}

	switch v := result.(type) {
	case string:
		return mcp.NewToolResultText(v), nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshal tool result: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// Shutdown performs a graceful shutdown of the server.
func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("server: shutting down")
	// The mcp-go library handles cleanup internally through the transport layer.
	// This method is a hook for any additional cleanup needed at the server level.
	return nil
}
