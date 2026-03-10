package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"  debug  ", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseLevel(tt.input)
			if got != tt.want {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestInitAndGet(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	logger := Init("debug", true)
	if logger == nil {
		t.Fatal("Init returned nil")
	}

	got := Get()
	if got != logger {
		t.Error("Get() returned different logger than Init()")
	}

	if !IsDebugMode() {
		t.Error("expected debug mode to be true")
	}
}

func TestInitDefaultsOnGet(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	logger := Get()
	if logger == nil {
		t.Fatal("Get() returned nil without prior Init()")
	}

	if IsDebugMode() {
		t.Error("expected debug mode to be false by default")
	}
}

func TestInitOnlyOnce(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	first := Init("debug", true)
	second := Init("error", false)

	if first != second {
		t.Error("Init called twice should return the same logger")
	}
	if !IsDebugMode() {
		t.Error("debug mode should remain true from first Init call")
	}
}

// newTestLogger creates a logger that writes JSON to a buffer for inspection.
func newTestLogger(buf *bytes.Buffer, level slog.Level) *slog.Logger {
	handler := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}

func TestToolCall_Success(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	var buf bytes.Buffer
	globalLogger = newTestLogger(&buf, slog.LevelInfo)

	ToolCall(context.Background(), "sls_query_logstore", "project=test, logstore=access", 150*time.Millisecond, "success", nil)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}

	if entry["tool"] != "sls_query_logstore" {
		t.Errorf("tool = %v, want sls_query_logstore", entry["tool"])
	}
	if entry["params"] != "project=test, logstore=access" {
		t.Errorf("params = %v, want project=test, logstore=access", entry["params"])
	}
	if entry["status"] != "success" {
		t.Errorf("status = %v, want success", entry["status"])
	}
	if entry["duration_ms"] != float64(150) {
		t.Errorf("duration_ms = %v, want 150", entry["duration_ms"])
	}
	if entry["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", entry["level"])
	}
}

func TestToolCall_Error(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	var buf bytes.Buffer
	globalLogger = newTestLogger(&buf, slog.LevelInfo)

	ToolCall(context.Background(), "cms_query_metric", "ns=acs_ecs", 500*time.Millisecond, "error", fmt.Errorf("timeout"))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}

	if entry["level"] != "ERROR" {
		t.Errorf("level = %v, want ERROR", entry["level"])
	}
	if entry["error"] != "timeout" {
		t.Errorf("error = %v, want timeout", entry["error"])
	}
}

func TestDebugRequest_DebugOn(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	var buf bytes.Buffer
	globalLogger = newTestLogger(&buf, slog.LevelDebug)
	debugMode = true

	params := map[string]any{"workspace": "default", "region_id": "cn-shanghai"}
	DebugRequest(context.Background(), "umodel_list_entities", params)

	if buf.Len() == 0 {
		t.Fatal("expected debug request log output")
	}

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}

	if entry["tool"] != "umodel_list_entities" {
		t.Errorf("tool = %v, want umodel_list_entities", entry["tool"])
	}
	if entry["request"] == nil {
		t.Error("expected request field in debug log")
	}
}

func TestDebugRequest_DebugOff(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	var buf bytes.Buffer
	globalLogger = newTestLogger(&buf, slog.LevelDebug)
	debugMode = false

	DebugRequest(context.Background(), "umodel_list_entities", map[string]any{"key": "val"})

	if buf.Len() != 0 {
		t.Error("expected no output when debug mode is off")
	}
}

func TestDebugResponse_DebugOn(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	var buf bytes.Buffer
	globalLogger = newTestLogger(&buf, slog.LevelDebug)
	debugMode = true

	DebugResponse(context.Background(), "sls_list_projects", []string{"project1", "project2"})

	if buf.Len() == 0 {
		t.Fatal("expected debug response log output")
	}

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}

	if entry["tool"] != "sls_list_projects" {
		t.Errorf("tool = %v, want sls_list_projects", entry["tool"])
	}
}

func TestDebugResponse_DebugOff(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	var buf bytes.Buffer
	globalLogger = newTestLogger(&buf, slog.LevelDebug)
	debugMode = false

	DebugResponse(context.Background(), "sls_list_projects", []string{"project1"})

	if buf.Len() != 0 {
		t.Error("expected no output when debug mode is off")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is longer than ten", 10, "this is lo..."},
		{"", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestParamSummary(t *testing.T) {
	got := ParamSummary(nil)
	if got != "{}" {
		t.Errorf("ParamSummary(nil) = %q, want {}", got)
	}

	got = ParamSummary(map[string]any{})
	if got != "{}" {
		t.Errorf("ParamSummary(empty) = %q, want {}", got)
	}

	got = ParamSummary(map[string]any{"key": "value"})
	if !strings.Contains(got, "key=value") {
		t.Errorf("ParamSummary = %q, expected to contain key=value", got)
	}
}

func TestToolCallLogContainsAllFields(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	var buf bytes.Buffer
	globalLogger = newTestLogger(&buf, slog.LevelDebug)

	ToolCall(context.Background(), "test_tool", "p=1", 42*time.Millisecond, "success", nil)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}

	// Verify all required fields per Requirement 13.3
	requiredFields := []string{"tool", "params", "duration", "duration_ms", "status", "msg", "level", "time"}
	for _, field := range requiredFields {
		if _, ok := entry[field]; !ok {
			t.Errorf("missing required field %q in tool call log", field)
		}
	}
}

func TestLogLevelFiltering(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	var buf bytes.Buffer
	// Set level to warn — info messages should be filtered out
	globalLogger = newTestLogger(&buf, slog.LevelWarn)

	ToolCall(context.Background(), "test_tool", "p=1", 10*time.Millisecond, "success", nil)

	if buf.Len() != 0 {
		t.Error("expected info-level tool call to be filtered at warn level")
	}
}
