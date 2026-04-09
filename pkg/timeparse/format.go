package timeparse

import (
	"fmt"
	"sync"
	"time"
)

// location is the timezone used for formatting timestamps and parsing date-time strings.
// Defaults to Asia/Shanghai. Set via SetLocation at startup.
var (
	location     *time.Location
	locationOnce sync.Once
)

func init() {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.UTC
	}
	location = loc
}

// SetLocation sets the timezone used by FormatTimestamp and parseDateTimeString.
// Should be called once at startup from config.
func SetLocation(loc *time.Location) {
	if loc != nil {
		location = loc
	}
}

// GetLocation returns the current timezone location.
func GetLocation() *time.Location {
	return location
}

// FormatTimestamp formats a Unix timestamp (seconds) into a human-readable
// date-time string using the configured timezone. The output format
// "2006-01-02 15:04:05" is compatible with ParseTimeExpression.
func FormatTimestamp(ts int64) string {
	return time.Unix(ts, 0).In(location).Format("2006-01-02 15:04:05")
}

// ParseTimeRange parses a time range specified by from/to expressions.
// Both expressions are parsed using ParseTimeExpression with the given
// reference time. Returns (fromTs, toTs, error).
func ParseTimeRange(from, to string, now time.Time) (int64, int64, error) {
	fromTs, err := ParseTimeExpression(from, now)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid 'from' time: %w", err)
	}

	toTs, err := ParseTimeExpression(to, now)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid 'to' time: %w", err)
	}

	if fromTs > toTs {
		return 0, 0, fmt.Errorf("'from' time (%d) is after 'to' time (%d)", fromTs, toTs)
	}

	return fromTs, toTs, nil
}
