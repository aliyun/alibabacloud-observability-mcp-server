package server

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alibabacloud-observability-mcp-server-go/pkg/config"
)

const shutdownTimeout = 30 * time.Second

// Run creates the transport, starts it, and blocks until a SIGINT/SIGTERM
// signal is received. On signal it performs a graceful shutdown within 30
// seconds: the transport stops accepting new requests, in-flight requests
// are drained, and then the server is shut down.
func Run(ctx context.Context, cfg *config.Config, srv *Server) error {
	transport, err := NewTransport(cfg, srv)
	if err != nil {
		return err
	}

	// Start transport in a goroutine so we can listen for signals.
	errCh := make(chan error, 1)
	go func() {
		errCh <- transport.Start(ctx)
	}()

	// Listen for OS signals.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		slog.Info("lifecycle: received signal, starting graceful shutdown", "signal", sig)
	case err := <-errCh:
		// Transport exited on its own (e.g. stdio closed).
		if err != nil {
			slog.Error("lifecycle: transport exited with error", "error", err)
			return err
		}
		slog.Info("lifecycle: transport exited cleanly")
		return nil
	case <-ctx.Done():
		slog.Info("lifecycle: context cancelled, starting graceful shutdown")
	}

	// Graceful shutdown with 30-second deadline.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	slog.Info("lifecycle: shutting down transport")
	if err := transport.Shutdown(shutdownCtx); err != nil {
		slog.Error("lifecycle: transport shutdown error", "error", err)
	}

	slog.Info("lifecycle: shutting down server")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("lifecycle: server shutdown error", "error", err)
	}

	slog.Info("lifecycle: shutdown complete")
	return nil
}
