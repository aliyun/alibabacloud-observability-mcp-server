package timeparse

import (
	"strings"
	"testing"
	"time"
)

// reference time for all tests: 2024-06-15 12:30:45 UTC
var refTime = time.Date(2024, 6, 15, 12, 30, 45, 0, time.UTC)

func TestParseTimeExpression_RelativeTime(t *testing.T) {
	nowUnix := refTime.Unix()

	tests := []struct {
		name string
		expr string
		want int64
	}{
		{"now()-1h", "now()-1h", nowUnix - 3600},
		{"now()-30m", "now()-30m", nowUnix - 1800},
		{"now-5m", "now-5m", nowUnix - 300},
		{"now()-1d", "now()-1d", nowUnix - 86400},
		{"now()+2h", "now()+2h", nowUnix + 7200},
		{"now+1h", "now+1h", nowUnix + 3600},
		{"now()-7d", "now()-7d", nowUnix - 604800},
		{"now()-1w", "now()-1w", nowUnix - 604800},
		{"now()-10s", "now()-10s", nowUnix - 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimeExpression(tt.expr, refTime)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseTimeExpression(%q) = %d, want %d", tt.expr, got, tt.want)
			}
		})
	}
}

func TestParseTimeExpression_AbsoluteTimestamp(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want int64
	}{
		{"10-digit seconds", "1718451045", 1718451045},
		{"13-digit milliseconds", "1718451045000", 1718451045},
		{"another 10-digit", "1640995200", 1640995200},
		{"another 13-digit", "1640995200000", 1640995200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimeExpression(tt.expr, refTime)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseTimeExpression(%q) = %d, want %d", tt.expr, got, tt.want)
			}
		})
	}
}

func TestParseTimeExpression_GrafanaStyle(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want int64
	}{
		{
			"now/d - start of day",
			"now/d",
			time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC).Unix(),
		},
		{
			"now-1d/d - start of yesterday",
			"now-1d/d",
			time.Date(2024, 6, 14, 0, 0, 0, 0, time.UTC).Unix(),
		},
		{
			"now/h - start of current hour",
			"now/h",
			time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC).Unix(),
		},
		{
			"now/m - start of current minute",
			"now/m",
			time.Date(2024, 6, 15, 12, 30, 0, 0, time.UTC).Unix(),
		},
		{
			"now()-2h/h - truncate after offset",
			"now()-2h/h",
			time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC).Unix(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimeExpression(tt.expr, refTime)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseTimeExpression(%q) = %d, want %d", tt.expr, got, tt.want)
			}
		})
	}
}

func TestParseTimeExpression_DateTimeString(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want int64
	}{
		{
			"YYYY-MM-DD HH:MM:SS",
			"2024-01-01 00:00:00",
			time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
		},
		{
			"ISO 8601 with Z",
			"2024-01-01T00:00:00Z",
			time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
		},
		{
			"ISO 8601 without Z",
			"2024-01-01T00:00:00",
			time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
		},
		{
			"YYYY-MM-DD HH:MM",
			"2024-06-15 14:30",
			time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC).Unix(),
		},
		{
			"YYYY-MM-DD only",
			"2024-06-15",
			time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC).Unix(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimeExpression(tt.expr, refTime)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseTimeExpression(%q) = %d, want %d", tt.expr, got, tt.want)
			}
		})
	}
}

func TestParseTimeExpression_PresetKeywords(t *testing.T) {
	nowUnix := refTime.Unix()

	tests := []struct {
		name string
		expr string
		want int64
	}{
		{"last_1h", "last_1h", nowUnix - 3600},
		{"last_24h", "last_24h", nowUnix - 86400},
		{"last_7d", "last_7d", nowUnix - 604800},
		{"last_30d", "last_30d", nowUnix - 2592000},
		{"last_5m", "last_5m", nowUnix - 300},
		{"today", "today", time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC).Unix()},
		{"yesterday", "yesterday", time.Date(2024, 6, 14, 0, 0, 0, 0, time.UTC).Unix()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimeExpression(tt.expr, refTime)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseTimeExpression(%q) = %d, want %d", tt.expr, got, tt.want)
			}
		})
	}
}

func TestParseTimeExpression_BareNow(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{"now", "now"},
		{"now()", "now()"},
		{"NOW", "NOW"},
		{"Now", "Now"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimeExpression(tt.expr, refTime)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != refTime.Unix() {
				t.Errorf("ParseTimeExpression(%q) = %d, want %d", tt.expr, got, refTime.Unix())
			}
		})
	}
}

func TestParseTimeExpression_InvalidExpressions(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{"empty string", ""},
		{"random text", "foobar"},
		{"partial now", "now-"},
		{"invalid unit", "now()-1x"},
		{"just a dash", "-"},
		{"too short number", "12345"},
		{"letters and numbers", "abc123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTimeExpression(tt.expr, refTime)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", tt.expr)
			}
			// Error should contain the original input
			if !strings.Contains(err.Error(), tt.expr) {
				t.Errorf("error %q should contain original input %q", err.Error(), tt.expr)
			}
		})
	}
}

func TestParseTimeExpression_WhitespaceHandling(t *testing.T) {
	nowUnix := refTime.Unix()

	tests := []struct {
		name string
		expr string
		want int64
	}{
		{"leading space", "  now()-1h", nowUnix - 3600},
		{"trailing space", "now()-1h  ", nowUnix - 3600},
		{"both spaces", "  now()-1h  ", nowUnix - 3600},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimeExpression(tt.expr, refTime)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseTimeExpression(%q) = %d, want %d", tt.expr, got, tt.want)
			}
		})
	}
}

// TestParseTimeExpression_TimestampBoundary tests boundary values for absolute timestamps:
// timestamp 0, very large values, and millisecond/second auto-identification.
// Validates: Requirements 6.2
func TestParseTimeExpression_TimestampBoundary(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		want    int64
		wantErr bool
	}{
		// Exactly 10 digits → seconds
		{"min 10-digit (epoch 0 padded)", "1000000000", 1000000000, false},
		{"max 10-digit", "9999999999", 9999999999, false},
		// Exactly 13 digits → milliseconds, converted to seconds
		{"min 13-digit", "1000000000000", 1000000000, false},
		{"max 13-digit", "9999999999999", 9999999999, false},
		// 11 and 12 digit values are also treated as milliseconds (len > 10)
		{"11-digit ms", "10000000000", 10000000, false},
		{"12-digit ms", "100000000000", 100000000, false},
		// Too short (< 10 digits) → error
		{"9-digit too short", "999999999", 0, true},
		// Too long (> 13 digits) → error
		{"14-digit too long", "10000000000000", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimeExpression(tt.expr, refTime)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tt.expr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseTimeExpression(%q) = %d, want %d", tt.expr, got, tt.want)
			}
		})
	}
}

// TestParseTimeExpression_PresetKeywordValues verifies that preset keywords
// produce the exact expected timestamp values relative to the reference time.
// Validates: Requirements 6.5
func TestParseTimeExpression_PresetKeywordValues(t *testing.T) {
	nowUnix := refTime.Unix()

	tests := []struct {
		name string
		expr string
		want int64
	}{
		{"last_1h = now - 3600s", "last_1h", nowUnix - 3600},
		{"last_24h = now - 86400s", "last_24h", nowUnix - 86400},
		{"last_7d = now - 604800s", "last_7d", nowUnix - 7*86400},
		{"last_30d = now - 2592000s", "last_30d", nowUnix - 30*86400},
		{"last_10s = now - 10s", "last_10s", nowUnix - 10},
		{"last_1M = now - 2592000s (30 days)", "last_1M", nowUnix - 2592000},
		{"last_1y = now - 31536000s (365 days)", "last_1y", nowUnix - 31536000},
		{"today = start of day", "today", time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC).Unix()},
		{"yesterday = start of previous day", "yesterday", time.Date(2024, 6, 14, 0, 0, 0, 0, time.UTC).Unix()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimeExpression(tt.expr, refTime)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseTimeExpression(%q) = %d, want %d (diff=%d)", tt.expr, got, tt.want, got-tt.want)
			}
		})
	}
}

