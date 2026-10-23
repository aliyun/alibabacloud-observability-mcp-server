package timeparse

import (
	"testing"
	"time"
)

func TestParseExplicitUTC(t *testing.T) {
	original := GetLocation()
	SetLocation(time.FixedZone("UTC+8", 8*60*60))
	t.Cleanup(func() { SetLocation(original) })
	now := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		expr string
		want int64
	}{
		{"2024-01-01T00:00:00Z", 1704067200},
		{"2024-01-01T08:00:00+08:00", 1704067200},
		{"2024-01-01T00:00:00", 1704038400},
		{"2024-01-01 00:00:00", 1704038400},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			got, err := ParseTimeExpression(tc.expr, now)
			if err != nil || got != tc.want {
				t.Fatalf("ParseTimeExpression(%q) = %d, %v; want %d", tc.expr, got, err, tc.want)
			}
		})
	}
}
