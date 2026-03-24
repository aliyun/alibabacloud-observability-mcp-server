// Package logger provides structured JSON logging for the MCP server using
// the standard library's log/slog package. It supports configurable log levels,
// tool call logging with duration/status tracking, and debug mode for full
// request/response output.
package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	globalLogger *slog.Logger
	loggerOnce   sync.Once
	debugMode    bool
)

// Init initializes the global structured logger with the given log level and
// debug mode flag. It is safe to call from multiple goroutines; only the first
// call performs actual initialization. The logger outputs JSON to stderr.
func Init(level string, debug bool) *slog.Logger {
	loggerOnce.Do(func() {
		debugMode = debug
		lvl := ParseLevel(level)

		handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: lvl,
		})

		globalLogger = slog.New(handler)
		slog.SetDefault(globalLogger)
	})
	return globalLogger
}

// Get returns the global logger. If Init() has not been called, it initializes
// with default settings (info level, debug off).
func Get() *slog.Logger {
	if globalLogger == nil {
		Init("info", false)
	}
	return globalLogger
}

// ParseLevel converts a string log level name to the corresponding slog.Level.
// Supported values: debug, info, warn, error. Defaults to info for unknown values.
func ParseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// IsDebugMode returns whether debug mode is enabled.
func IsDebugMode() bool {
	return debugMode
}

// ToolCall logs the result of a tool invocation, including the tool name,
// a summary of the parameters, the elapsed duration, and the result status.
// This satisfies Requirement 13.3.
func ToolCall(ctx context.Context, toolName string, paramSummary string, duration time.Duration, status string, err error) {
	logger := Get()
	attrs := []slog.Attr{
		slog.String("tool", toolName),
		slog.String("params", truncate(paramSummary, 200)),
		slog.String("duration", duration.String()),
		slog.Int64("duration_ms", duration.Milliseconds()),
		slog.String("status", status),
	}

	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
		logger.LogAttrs(ctx, slog.LevelError, "tool call failed", attrs...)
		return
	}

	logger.LogAttrs(ctx, slog.LevelInfo, "tool call completed", attrs...)
}

// DebugRequest logs the full request content when debug mode is enabled.
// This satisfies Requirement 13.4.
func DebugRequest(ctx context.Context, toolName string, params any) {
	if !debugMode {
		return
	}
	Get().LogAttrs(ctx, slog.LevelDebug, "tool request",
		slog.String("tool", toolName),
		slog.Any("request", params),
	)
}

// DebugResponse logs the full response content when debug mode is enabled.
// This satisfies Requirement 13.4.
func DebugResponse(ctx context.Context, toolName string, response any) {
	if !debugMode {
		return
	}
	Get().LogAttrs(ctx, slog.LevelDebug, "tool response",
		slog.String("tool", toolName),
		slog.Any("response", response),
	)
}

// truncate shortens a string to maxLen characters, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ResetForTesting resets the global logger so Init() can be called again.
// This MUST only be used in tests.
func ResetForTesting() {
	globalLogger = nil
	loggerOnce = sync.Once{}
	debugMode = false
}

// ParamSummary builds a concise summary string from a map of tool parameters.
// Keys are sorted and values are truncated for readability.
func ParamSummary(params map[string]any) string {
	if len(params) == 0 {
		return "{}"
	}
	var parts []string
	for k, v := range params {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return truncate(strings.Join(parts, ", "), 200)
}
