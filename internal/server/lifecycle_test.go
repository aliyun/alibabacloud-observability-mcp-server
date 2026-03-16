package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/alibabacloud-observability-mcp-server-go/internal/config"
	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit"
)

func TestRun_SignalTriggersGracefulShutdown(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Transport: "sse",
			Host:      "127.0.0.1",
			Port:      0,
		},
	}
	reg := toolkit.NewRegistry()
	srv, err := NewServer(cfg, reg, &stubSLSClient{}, &stubCMSClient{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Pick a free port and override config so Run uses it.
	port := freePort(t)
	cfg.Server.Port = port

	ctx := context.Background()
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, cfg, srv)
	}()

	// Wait for the server to be ready.
	addr := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	waitForServer(t, addr)

	// Send SIGINT to ourselves to trigger graceful shutdown.
	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("FindProcess: %v", err)
	}
	if err := proc.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("Signal: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return within 10 seconds after signal")
	}
}

func TestRun_ContextCancelTriggersShutdown(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Transport: "sse",
			Host:      "127.0.0.1",
			Port:      0,
		},
	}
	reg := toolkit.NewRegistry()
	srv, err := NewServer(cfg, reg, &stubSLSClient{}, &stubCMSClient{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	port := freePort(t)
	cfg.Server.Port = port

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, cfg, srv)
	}()

	addr := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	waitForServer(t, addr)

	// Cancel the context to trigger shutdown.
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return within 10 seconds after context cancel")
	}
}

func TestHealthEndpoint_SSE(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Transport: "sse", Host: "127.0.0.1", Port: 0}}
	mcpSrv := newTestMCPServer()

	tr := newSSETransport(cfg, mcpSrv)
	port := freePort(t)
	tr.addr = fmt.Sprintf("127.0.0.1:%d", port)

	errCh := make(chan error, 1)
	go func() {
		errCh <- tr.Start(context.Background())
	}()

	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	waitForServer(t, healthURL)

	resp, err := http.Get(healthURL)
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}

	var body healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("expected status ok, got %s", body.Status)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = tr.Shutdown(ctx)
}

func TestHealthEndpoint_StreamableHTTP(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Transport: "streamable-http", Host: "127.0.0.1", Port: 0}}
	mcpSrv := newTestMCPServer()

	tr := newStreamableHTTPTransport(cfg, mcpSrv)
	port := freePort(t)
	tr.addr = fmt.Sprintf("127.0.0.1:%d", port)

	errCh := make(chan error, 1)
	go func() {
		errCh <- tr.Start(context.Background())
	}()

	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	waitForServer(t, healthURL)

	resp, err := http.Get(healthURL)
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("expected status ok, got %s", body.Status)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = tr.Shutdown(ctx)
}

func TestHealthHandler_Direct(t *testing.T) {
	handler := healthHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("expected ok, got %s", body.Status)
	}
}

func TestHealthMux_RoutesToHealth(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	hm := &healthMux{inner: inner}

	// /health should return 200 with JSON.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	hm.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /health, got %d", rec.Code)
	}

	// Other paths should delegate to inner.
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/other", nil)
	hm.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTeapot {
		t.Fatalf("expected 418 for /other, got %d", rec2.Code)
	}
}

func TestHealthMux_NilInner(t *testing.T) {
	hm := &healthMux{}

	// /health should still work.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	hm.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /health, got %d", rec.Code)
	}

	// Other paths should return 404.
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/other", nil)
	hm.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for /other with nil inner, got %d", rec2.Code)
	}
}
