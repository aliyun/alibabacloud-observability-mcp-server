package timeparse

import (
	"fmt"
	"time"
)

// FormatTimestamp formats a Unix timestamp (seconds) into a human-readable
// date-time string in UTC. The output format "2006-01-02 15:04:05" is
// compatible with ParseTimeExpression, ensuring round-trip consistency.
func FormatTimestamp(ts int64) string {
	return time.Unix(ts, 0).UTC().Format("2006-01-02 15:04:05")
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
