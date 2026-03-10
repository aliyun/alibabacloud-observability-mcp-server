package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/alibabacloud-observability-mcp-server-go/internal/config"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// newTestMCPServer creates a minimal MCPServer for testing.
func newTestMCPServer() *mcpserver.MCPServer {
	return mcpserver.NewMCPServer("test-server", "0.1.0")
}

// newTestServer wraps a bare MCPServer in our Server struct for transport tests.
func newTestServer(mcpSrv *mcpserver.MCPServer) *Server {
	return &Server{mcpServer: mcpSrv}
}

func TestNewTransport_Stdio(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Transport: "stdio"}}
	srv := newTestServer(newTestMCPServer())

	tr, err := NewTransport(cfg, srv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := tr.(*stdioTransport); !ok {
		t.Fatalf("expected *stdioTransport, got %T", tr)
	}
}

func TestNewTransport_SSE(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Transport: "sse", Host: "127.0.0.1", Port: 9090}}
	srv := newTestServer(newTestMCPServer())

	tr, err := NewTransport(cfg, srv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	st, ok := tr.(*sseTransport)
	if !ok {
		t.Fatalf("expected *sseTransport, got %T", tr)
	}
	if st.addr != "127.0.0.1:9090" {
		t.Fatalf("expected addr 127.0.0.1:9090, got %s", st.addr)
	}
}

func TestNewTransport_StreamableHTTP(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Transport: "streamable-http", Host: "0.0.0.0", Port: 8080}}
	srv := newTestServer(newTestMCPServer())

	tr, err := NewTransport(cfg, srv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	st, ok := tr.(*streamableHTTPTransport)
	if !ok {
		t.Fatalf("expected *streamableHTTPTransport, got %T", tr)
	}
	if st.addr != "0.0.0.0:8080" {
		t.Fatalf("expected addr 0.0.0.0:8080, got %s", st.addr)
	}
}

func TestNewTransport_UnsupportedMode(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Transport: "grpc"}}
	srv := newTestServer(newTestMCPServer())

	_, err := NewTransport(cfg, srv)
	if err == nil {
		t.Fatal("expected error for unsupported transport mode")
	}
}

func TestNewTransport_NilConfig(t *testing.T) {
	srv := newTestServer(newTestMCPServer())
	_, err := NewTransport(nil, srv)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestNewTransport_NilServer(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Transport: "stdio"}}
	_, err := NewTransport(cfg, nil)
	if err == nil {
		t.Fatal("expected error for nil server")
	}
}

func TestListenAddr(t *testing.T) {
	tests := []struct {
		host string
		port int
		want string
	}{
		{"0.0.0.0", 8080, "0.0.0.0:8080"},
		{"127.0.0.1", 3000, "127.0.0.1:3000"},
		{"localhost", 443, "localhost:443"},
		{"::1", 8080, "[::1]:8080"},
	}
	for _, tt := range tests {
		cfg := &config.Config{Server: config.ServerConfig{Host: tt.host, Port: tt.port}}
		got := listenAddr(cfg)
		if got != tt.want {
			t.Errorf("listenAddr(%s, %d) = %q, want %q", tt.host, tt.port, got, tt.want)
		}
	}
}

func TestSSETransport_StartAndShutdown(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Transport: "sse", Host: "127.0.0.1", Port: 0}}
	srv := newTestServer(newTestMCPServer())

	tr, err := NewTransport(cfg, srv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Use a real port by starting on :0 — but SSEServer.Start doesn't support :0 easily,
	// so we pick a free port first.
	port := freePort(t)
	sseTr := tr.(*sseTransport)
	sseTr.addr = fmt.Sprintf("127.0.0.1:%d", port)

	errCh := make(chan error, 1)
	go func() {
		errCh <- sseTr.Start(context.Background())
	}()

	// Wait for the server to be ready.
	waitForServer(t, fmt.Sprintf("http://127.0.0.1:%d/sse", port))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sseTr.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}

	// Start returns http.ErrServerClosed on graceful shutdown.
	if startErr := <-errCh; startErr != nil && startErr != http.ErrServerClosed {
		t.Fatalf("unexpected start error: %v", startErr)
	}
}

func TestStreamableHTTPTransport_StartAndShutdown(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Transport: "streamable-http", Host: "127.0.0.1", Port: 0}}
	srv := newTestServer(newTestMCPServer())

	tr, err := NewTransport(cfg, srv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	port := freePort(t)
	httpTr := tr.(*streamableHTTPTransport)
	httpTr.addr = fmt.Sprintf("127.0.0.1:%d", port)

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpTr.Start(context.Background())
	}()

	waitForServer(t, fmt.Sprintf("http://127.0.0.1:%d/mcp", port))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpTr.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}

	if startErr := <-errCh; startErr != nil && startErr != http.ErrServerClosed {
		t.Fatalf("unexpected start error: %v", startErr)
	}
}

func TestStdioTransport_Shutdown(t *testing.T) {
	tr := &stdioTransport{mcpServer: newTestMCPServer()}
	if err := tr.Shutdown(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- helpers ---

// freePort asks the OS for a free TCP port.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

// waitForServer polls the given URL until it responds or times out.
func waitForServer(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server at %s did not become ready in time", url)
}
