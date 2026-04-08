// Package timeparse provides time expression parsing utilities for the MCP server.
// It supports relative time expressions, absolute timestamps, Grafana-style expressions,
// date-time strings, and preset keywords.
package timeparse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Unit-to-seconds mapping
var unitSeconds = map[byte]int64{
	's': 1,
	'm': 60,
	'h': 3600,
	'd': 86400,
	'w': 604800,
	'M': 2592000,  // 30 days
	'y': 31536000, // 365 days
}

// Regex patterns
var (
	// Relative time: now()-1h, now()-30m, now-5m, now()+2h, now+1d
	reRelativeTime = regexp.MustCompile(`(?i)^now(?:\(\))?\s*([+-])\s*(\d+)([smhdwMy])$`)

	// Grafana truncation: now/d, now-1d/d, now()/h, now()-1h/h
	reGrafanaTrunc = regexp.MustCompile(`(?i)^now(?:\(\))?\s*(?:([+-])\s*(\d+)([smhdwMy]))?\s*/([dhm])$`)

	// Absolute Unix timestamp: 10 digits (seconds) or 13 digits (milliseconds)
	reTimestamp = regexp.MustCompile(`^\d{10,13}$`)

	// Preset keywords: last_1h, last_24h, last_7d, last_30d
	rePresetKeyword = regexp.MustCompile(`(?i)^last_(\d+)([smhdwMy])$`)
)

// Date-time formats to try in order
var dateTimeFormats = []string{
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
}

// supportedFormats is the list of supported format descriptions for error messages.
var supportedFormats = []string{
	"relative time (now()-1h, now-5m, now()+2h)",
	"absolute Unix timestamp (seconds: 10 digits, milliseconds: 13 digits)",
	"Grafana-style (now/d, now-1d/d, now/h)",
	"date-time string (2024-01-01 00:00:00, 2024-01-01T00:00:00Z)",
	"preset keywords (last_1h, last_24h, today, yesterday, last_7d, last_30d)",
}

// ParseTimeExpression parses a time expression string into a Unix timestamp (seconds).
// It supports:
//   - Relative time: now()-1h, now()-30m, now-5m, now()+2h
//   - Absolute Unix timestamps: seconds (10 digits) and milliseconds (13 digits)
//   - Grafana-style: now/d, now-1d/d, now/h
//   - Date-time strings: 2024-01-01 00:00:00, 2024-01-01T00:00:00Z
//   - Preset keywords: last_1h, last_24h, today, yesterday, last_7d, last_30d
//   - Bare "now" or "now()": returns current timestamp
func ParseTimeExpression(expr string, now time.Time) (int64, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return 0, newParseError(expr)
	}

	// 1. Bare "now" or "now()"
	lower := strings.ToLower(expr)
	if lower == "now" || lower == "now()" {
		return now.Unix(), nil
	}

	// 2. Preset keywords: today, yesterday
	if lower == "today" {
		return startOfDay(now).Unix(), nil
	}
	if lower == "yesterday" {
		return startOfDay(now).Add(-24 * time.Hour).Unix(), nil
	}

	// 3. Preset keyword pattern: last_1h, last_24h, last_7d, last_30d
	if ts, err := parsePresetKeyword(expr, now); err == nil {
		return ts, nil
	}

	// 4. Grafana truncation: now/d, now-1d/d
	if ts, err := parseGrafanaTrunc(expr, now); err == nil {
		return ts, nil
	}

	// 5. Relative time: now()-1h, now-5m
	if ts, err := parseRelativeTime(expr, now); err == nil {
		return ts, nil
	}

	// 6. Absolute Unix timestamp
	if ts, err := parseTimestamp(expr); err == nil {
		return ts, nil
	}

	// 7. Date-time string
	if ts, err := parseDateTimeString(expr); err == nil {
		return ts, nil
	}

	return 0, newParseError(expr)
}

// parseRelativeTime handles expressions like now()-1h, now-5m, now()+2h.
func parseRelativeTime(expr string, now time.Time) (int64, error) {
	m := reRelativeTime.FindStringSubmatch(expr)
	if m == nil {
		return 0, fmt.Errorf("not a relative time expression")
	}

	op := m[1]
	amount, _ := strconv.ParseInt(m[2], 10, 64)
	unit := normalizeUnit(m[3])

	secs, ok := unitSeconds[unit]
	if !ok {
		return 0, fmt.Errorf("unsupported time unit: %s", m[3])
	}

	offset := amount * secs
	ts := now.Unix()
	if op == "-" {
		ts -= offset
	} else {
		ts += offset
	}
	return ts, nil
}

// parseGrafanaTrunc handles expressions like now/d, now-1d/d, now/h.
func parseGrafanaTrunc(expr string, now time.Time) (int64, error) {
	m := reGrafanaTrunc.FindStringSubmatch(expr)
	if m == nil {
		return 0, fmt.Errorf("not a Grafana truncation expression")
	}

	// Apply offset if present
	base := now
	if m[1] != "" {
		op := m[1]
		amount, _ := strconv.ParseInt(m[2], 10, 64)
		unit := normalizeUnit(m[3])
		secs, ok := unitSeconds[unit]
		if !ok {
			return 0, fmt.Errorf("unsupported time unit: %s", m[3])
		}
		offset := time.Duration(amount*secs) * time.Second
		if op == "-" {
			base = base.Add(-offset)
		} else {
			base = base.Add(offset)
		}
	}

	// Truncate based on the truncation unit
	truncUnit := strings.ToLower(m[4])
	switch truncUnit {
	case "d":
		return startOfDay(base).Unix(), nil
	case "h":
		return base.Truncate(time.Hour).Unix(), nil
	case "m":
		return base.Truncate(time.Minute).Unix(), nil
	default:
		return 0, fmt.Errorf("unsupported truncation unit: %s", truncUnit)
	}
}

// parseTimestamp handles absolute Unix timestamps (10 or 13 digits).
func parseTimestamp(expr string) (int64, error) {
	if !reTimestamp.MatchString(expr) {
		return 0, fmt.Errorf("not a timestamp")
	}

	ts, err := strconv.ParseInt(expr, 10, 64)
	if err != nil {
		return 0, err
	}

	// 13-digit timestamps are milliseconds, convert to seconds
	if len(expr) > 10 {
		ts = ts / 1000
	}

	return ts, nil
}

// parseDateTimeString handles date-time strings in various formats.
// Uses the configured timezone (default Asia/Shanghai) for formats without
// explicit timezone info, matching the Python implementation behavior.
func parseDateTimeString(expr string) (int64, error) {
	for _, layout := range dateTimeFormats {
		t, err := time.ParseInLocation(layout, expr, location)
		if err == nil {
			return t.Unix(), nil
		}
	}
	return 0, fmt.Errorf("not a date-time string")
}

// parsePresetKeyword handles preset keywords like last_1h, last_24h, last_7d.
func parsePresetKeyword(expr string, now time.Time) (int64, error) {
	m := rePresetKeyword.FindStringSubmatch(expr)
	if m == nil {
		return 0, fmt.Errorf("not a preset keyword")
	}

	amount, _ := strconv.ParseInt(m[1], 10, 64)
	unit := normalizeUnit(m[2])

	secs, ok := unitSeconds[unit]
	if !ok {
		return 0, fmt.Errorf("unsupported time unit: %s", m[2])
	}

	return now.Unix() - amount*secs, nil
}

// normalizeUnit normalizes a time unit character.
// 'M' stays as 'M' (month), everything else is lowercased.
func normalizeUnit(u string) byte {
	if u == "M" {
		return 'M'
	}
	return strings.ToLower(u)[0]
}

// startOfDay returns the start of the day (00:00:00) for the given time,
// preserving the location.
func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// newParseError creates a descriptive error for an unrecognized time expression.
func newParseError(expr string) error {
	return fmt.Errorf(
		"unrecognized time expression: %q. Supported formats: %s",
		expr,
		strings.Join(supportedFormats, "; "),
	)
}
