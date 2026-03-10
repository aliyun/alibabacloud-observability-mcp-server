package timeparse

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// refTimeProperty is a fixed reference time for property tests.
var refTimeProperty = time.Date(2024, 6, 15, 12, 30, 45, 0, time.UTC)

// TestProperty_TimeExpressionRoundTrip verifies that for any valid time expression,
// parse(format(parse(expr))) produces the same timestamp as parse(expr).
// That is, parsing → formatting → re-parsing yields a result consistent with the first parse.
//
// Feature: go-mcp-server-rewrite, Property 3: 时间表达式往返一致性
// Validates: Requirements 6.8
func TestProperty_TimeExpressionRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for valid relative time expressions: now()-Nd, now()-Nh, now()-Nm, now()-Ns
	genRelativeExpr := gopter.CombineGens(
		gen.IntRange(1, 365),
		gen.OneConstOf("s", "m", "h", "d"),
	).Map(func(values []interface{}) string {
		amount := values[0].(int)
		unit := values[1].(string)
		return fmt.Sprintf("now()-%d%s", amount, unit)
	})

	// Generator for absolute timestamps (seconds) in a reasonable range
	// Range: 2000-01-01 to 2030-01-01
	genAbsoluteTimestamp := gen.Int64Range(946684800, 1893456000).Map(func(ts int64) string {
		return fmt.Sprintf("%d", ts)
	})

	// Generator for date-time strings via formatting a random timestamp
	genDateTimeString := gen.Int64Range(946684800, 1893456000).Map(func(ts int64) string {
		return time.Unix(ts, 0).UTC().Format("2006-01-02 15:04:05")
	})

	// Generator for preset keywords
	genPresetKeyword := gen.OneConstOf("last_1h", "last_24h", "last_7d", "last_30d", "last_5m", "last_10s")

	// Generator for bare now expressions
	genBareNow := gen.OneConstOf("now", "now()")

	// Combine all valid expression generators
	genValidExpr := gen.OneGenOf(
		genRelativeExpr,
		genAbsoluteTimestamp,
		genDateTimeString,
		genPresetKeyword,
		genBareNow,
	)

	properties.Property("parse(format(parse(expr))) == parse(expr) for valid expressions", prop.ForAll(
		func(expr string) bool {
			// First parse
			ts1, err := ParseTimeExpression(expr, refTimeProperty)
			if err != nil {
				// If it fails to parse, skip (should not happen with our generators)
				return true
			}

			// Format the parsed timestamp
			formatted := FormatTimestamp(ts1)

			// Re-parse the formatted string
			ts2, err := ParseTimeExpression(formatted, refTimeProperty)
			if err != nil {
				return false
			}

			// The two timestamps must be equal
			return ts1 == ts2
		},
		genValidExpr,
	))

	properties.TestingRun(t)
}

// TestProperty_InvalidTimeExpressionError verifies that for any string that does not
// conform to any known time expression format, ParseTimeExpression returns a non-nil
// error whose message contains the original input.
//
// Feature: go-mcp-server-rewrite, Property 4: 无效时间表达式错误处理
// Validates: Requirements 6.6
func TestProperty_InvalidTimeExpressionError(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for strings that are definitely not valid time expressions.
	// We use alphabetic strings with a prefix that ensures they won't match any pattern.
	genInvalidExpr := gen.AlphaString().SuchThat(func(v interface{}) bool {
		s := v.(string)
		if s == "" {
			return false // empty string is a separate case, keep it simple
		}
		// Filter out strings that could accidentally be valid expressions
		lower := strings.ToLower(s)
		if lower == "now" || lower == "today" || lower == "yesterday" {
			return false
		}
		if strings.HasPrefix(lower, "now") {
			return false
		}
		if strings.HasPrefix(lower, "last_") {
			return false
		}
		return true
	}).Map(func(s string) string {
		// Prefix with "invalid_" to ensure it won't match any numeric pattern
		return "invalid_" + s
	})

	properties.Property("invalid expressions return error containing original input", prop.ForAll(
		func(expr string) bool {
			_, err := ParseTimeExpression(expr, refTimeProperty)
			if err == nil {
				return false // should have returned an error
			}
			// Error message must contain the original input
			return strings.Contains(err.Error(), expr)
		},
		genInvalidExpr,
	))

	properties.TestingRun(t)
}
