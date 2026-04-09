package timeparse

import (
	"strings"
	"testing"
	"time"
)

func TestFormatTimestamp(t *testing.T) {
	loc := GetLocation() // Asia/Shanghai by default
	tests := []struct {
		name string
		ts   int64
		want string
	}{
		{"epoch zero", 0, time.Unix(0, 0).In(loc).Format("2006-01-02 15:04:05")},
		{"known date", 1704067200, time.Unix(1704067200, 0).In(loc).Format("2006-01-02 15:04:05")},
		{"mid-day", 1718451045, time.Unix(1718451045, 0).In(loc).Format("2006-01-02 15:04:05")},
		{"end of year", 1735689599, time.Unix(1735689599, 0).In(loc).Format("2006-01-02 15:04:05")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTimestamp(tt.ts)
			if got != tt.want {
				t.Errorf("FormatTimestamp(%d) = %q, want %q", tt.ts, got, tt.want)
			}
		})
	}
}

func TestFormatTimestamp_RoundTrip(t *testing.T) {
	// Verify that FormatTimestamp output can be parsed back by ParseTimeExpression
	// to the same timestamp value (Requirement 6.8).
	timestamps := []int64{
		0,
		1704067200,
		1718451045,
		1735689599,
		refTime.Unix(),
	}

	for _, ts := range timestamps {
		formatted := FormatTimestamp(ts)
		parsed, err := ParseTimeExpression(formatted, refTime)
		if err != nil {
			t.Fatalf("ParseTimeExpression(%q) failed: %v", formatted, err)
		}
		if parsed != ts {
			t.Errorf("round-trip failed: FormatTimestamp(%d) = %q, ParseTimeExpression => %d", ts, formatted, parsed)
		}
	}
}

func TestParseTimeRange_Valid(t *testing.T) {
	tests := []struct {
		name     string
		from     string
		to       string
		wantFrom int64
		wantTo   int64
	}{
		{
			"relative expressions",
			"now()-1h", "now()",
			refTime.Unix() - 3600, refTime.Unix(),
		},
		{
			"absolute timestamps",
			"1718400000", "1718451045",
			1718400000, 1718451045,
		},
		{
			"date-time strings",
			"2024-01-01 00:00:00", "2024-06-15 12:30:45",
			time.Date(2024, 1, 1, 0, 0, 0, 0, GetLocation()).Unix(),
			time.Date(2024, 6, 15, 12, 30, 45, 0, GetLocation()).Unix(),
		},
		{
			"preset keyword and now",
			"last_1h", "now()",
			refTime.Unix() - 3600, refTime.Unix(),
		},
		{
			"same from and to",
			"now()", "now()",
			refTime.Unix(), refTime.Unix(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFrom, gotTo, err := ParseTimeRange(tt.from, tt.to, refTime)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotFrom != tt.wantFrom {
				t.Errorf("from = %d, want %d", gotFrom, tt.wantFrom)
			}
			if gotTo != tt.wantTo {
				t.Errorf("to = %d, want %d", gotTo, tt.wantTo)
			}
		})
	}
}

func TestParseTimeRange_InvalidFrom(t *testing.T) {
	_, _, err := ParseTimeRange("invalid_expr", "now()", refTime)
	if err == nil {
		t.Fatal("expected error for invalid 'from', got nil")
	}
	if !strings.Contains(err.Error(), "invalid 'from' time") {
		t.Errorf("error %q should mention invalid 'from' time", err.Error())
	}
}

func TestParseTimeRange_InvalidTo(t *testing.T) {
	_, _, err := ParseTimeRange("now()", "invalid_expr", refTime)
	if err == nil {
		t.Fatal("expected error for invalid 'to', got nil")
	}
	if !strings.Contains(err.Error(), "invalid 'to' time") {
		t.Errorf("error %q should mention invalid 'to' time", err.Error())
	}
}

func TestParseTimeRange_FromAfterTo(t *testing.T) {
	_, _, err := ParseTimeRange("now()", "now()-1h", refTime)
	if err == nil {
		t.Fatal("expected error when 'from' is after 'to', got nil")
	}
	if !strings.Contains(err.Error(), "after") {
		t.Errorf("error %q should mention 'from' is after 'to'", err.Error())
	}
}
