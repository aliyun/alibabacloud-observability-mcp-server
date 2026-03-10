// Package server provides transport layer support for the MCP server.
// It bridges the Server with mcp-go's stdio, SSE, and streamable-http transports.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/alibabacloud-observability-mcp-server-go/internal/config"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// Transport abstracts the different MCP transport modes so the caller can
// start and stop them uniformly.
type Transport interface {
	// Start begins serving. It blocks until the transport is stopped or
	// encounters a fatal error.
	Start(ctx context.Context) error
	// Shutdown gracefully stops the transport.
	Shutdown(ctx context.Context) error
}

// NewTransport creates the appropriate Transport based on cfg.Transport.
// Supported values: "stdio", "sse", "streamable-http".
func NewTransport(cfg *config.Config, srv *Server) (Transport, error) {
	if cfg == nil {
		return nil, fmt.Errorf("transport: config must not be nil")
	}
	if srv == nil {
		return nil, fmt.Errorf("transport: server must not be nil")
	}

	switch cfg.Server.Transport {
	case "stdio":
		return &stdioTransport{mcpServer: srv.MCPServer()}, nil
	case "sse":
		return newSSETransport(cfg, srv.MCPServer()), nil
	case "streamable-http":
		return newStreamableHTTPTransport(cfg, srv.MCPServer()), nil
	default:
		return nil, fmt.Errorf("transport: unsupported transport mode %q (valid: stdio, sse, streamable-http)", cfg.Server.Transport)
	}
}

// --- stdio ---

type stdioTransport struct {
	mcpServer *mcpserver.MCPServer
}

func (t *stdioTransport) Start(_ context.Context) error {
	slog.Info("transport: starting stdio")
	return mcpserver.ServeStdio(t.mcpServer)
}

func (t *stdioTransport) Shutdown(_ context.Context) error {
	// stdio transport is terminated by closing stdin; no explicit shutdown needed.
	return nil
}

// --- SSE ---

type sseTransport struct {
	sse  *mcpserver.SSEServer
	addr string
}

func newSSETransport(cfg *config.Config, mcpSrv *mcpserver.MCPServer) *sseTransport {
	addr := listenAddr(cfg)
	baseURL := fmt.Sprintf("http://%s", baseURLHost(cfg))

	// Create a healthMux that will delegate to the SSEServer once set.
	// Addr is left empty so that Start() can set it (avoids conflicts when
	// tests override the addr field).
	hm := &healthMux{}
	httpSrv := &http.Server{
		Handler: hm,
	}

	sse := mcpserver.NewSSEServer(mcpSrv,
		mcpserver.WithBaseURL(baseURL),
		mcpserver.WithHTTPServer(httpSrv),
	)

	// Now wire the SSEServer (which implements http.Handler) as the inner handler.
	hm.inner = sse

	return &sseTransport{sse: sse, addr: addr}
}

func (t *sseTransport) Start(_ context.Context) error {
	slog.Info("transport: starting SSE", "addr", t.addr)
	return t.sse.Start(t.addr)
}

func (t *sseTransport) Shutdown(ctx context.Context) error {
	return t.sse.Shutdown(ctx)
}

// --- streamable-http ---

type streamableHTTPTransport struct {
	srv  *mcpserver.StreamableHTTPServer
	addr string
}

func newStreamableHTTPTransport(cfg *config.Config, mcpSrv *mcpserver.MCPServer) *streamableHTTPTransport {
	addr := listenAddr(cfg)

	// Create a healthMux whose inner handler will be set after the
	// StreamableHTTPServer is constructed (breaking the circular dependency).
	// Addr is left empty so that Start() can set it.
	hm := &healthMux{}
	httpSrv := &http.Server{
		Handler: hm,
	}

	srv := mcpserver.NewStreamableHTTPServer(mcpSrv,
		mcpserver.WithEndpointPath("/streamhttp"),
		mcpserver.WithStreamableHTTPServer(httpSrv),
	)

	// Wire the StreamableHTTPServer as the inner handler for non-/health routes.
	hm.inner = srv

	return &streamableHTTPTransport{srv: srv, addr: addr}
}

func (t *streamableHTTPTransport) Start(_ context.Context) error {
	slog.Info("transport: starting streamable-http", "addr", t.addr)
	return t.srv.Start(t.addr)
}

func (t *streamableHTTPTransport) Shutdown(ctx context.Context) error {
	return t.srv.Shutdown(ctx)
}

// listenAddr builds the "host:port" string from config.
func listenAddr(cfg *config.Config) string {
	return net.JoinHostPort(cfg.Server.Host, fmt.Sprintf("%d", cfg.Server.Port))
}

// baseURLHost returns a host suitable for constructing client-facing URLs.
// Wildcard addresses (0.0.0.0, ::, "") are replaced with 127.0.0.1 because
// clients cannot connect to a wildcard address.
func baseURLHost(cfg *config.Config) string {
	host := cfg.Server.Host
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, fmt.Sprintf("%d", cfg.Server.Port))
}
