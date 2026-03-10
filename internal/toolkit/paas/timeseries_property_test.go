package paas

import (
	"math"
	"sort"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// ---------------------------------------------------------------------------
// Generators
// ---------------------------------------------------------------------------

// genNonEmptyFloat64Slice generates a non-empty slice of finite float64 values.
func genNonEmptyFloat64Slice() gopter.Gen {
	return gen.SliceOfN(
		50,
		gen.Float64Range(-1e12, 1e12),
	).SuchThat(func(v []float64) bool {
		if len(v) == 0 {
			return false
		}
		for _, f := range v {
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return false
			}
		}
		return true
	})
}

// genTimeSeriesStats generates a valid TimeSeriesStats with Count > 0.
func genTimeSeriesStats() gopter.Gen {
	return genNonEmptyFloat64Slice().Map(func(values []float64) TimeSeriesStats {
		return ComputeStats(values, nil)
	})
}

// ---------------------------------------------------------------------------
// Property 15: 时序统计量数学不变量
// ---------------------------------------------------------------------------

// TestProperty_TimeSeriesStatsMathInvariant verifies that for any non-empty
// float64 array, the computed statistics satisfy:
//   - Min <= Avg <= Max
//   - Count == len(array)
//
// Feature: go-mcp-server-rewrite, Property 15: 时序统计量数学不变量
// **Validates: Requirements 14.2**
func TestProperty_TimeSeriesStatsMathInvariant(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("Min <= Avg <= Max and Count == len(array)", prop.ForAll(
		func(values []float64) bool {
			stats := ComputeStats(values, nil)

			if stats.Count != len(values) {
				t.Logf("Count mismatch: got %d, want %d", stats.Count, len(values))
				return false
			}

			if stats.Min > stats.Avg {
				t.Logf("Min (%f) > Avg (%f)", stats.Min, stats.Avg)
				return false
			}

			if stats.Avg > stats.Max {
				t.Logf("Avg (%f) > Max (%f)", stats.Avg, stats.Max)
				return false
			}

			return true
		},
		genNonEmptyFloat64Slice(),
	))

	properties.TestingRun(t)
}

// ---------------------------------------------------------------------------
// Property 16: 趋势分析正确性
// ---------------------------------------------------------------------------

// TestProperty_TrendAnalysisCorrectness verifies that:
//   - For any data where current avg is significantly higher than compare avg,
//     the trend is "up".
//   - For any data where current avg is significantly lower than compare avg,
//     the trend is "down".
//   - For any data where current and compare stats are identical,
//     the trend is "stable".
//
// Feature: go-mcp-server-rewrite, Property 16: 趋势分析正确性
// **Validates: Requirements 14.3**
func TestProperty_TrendAnalysisCorrectness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Sub-property: identical stats → stable trend
	properties.Property("identical stats produce stable trend", prop.ForAll(
		func(values []float64) bool {
			stats := ComputeStats(values, nil)
			compareCopy := stats
			diff := AnalyzeTrend(stats, &compareCopy)
			return diff.Trend == TrendStable
		},
		genNonEmptyFloat64Slice(),
	))

	// Sub-property: strictly increasing current avg → up trend
	// We generate a base stats and then create a current with significantly higher avg.
	properties.Property("significantly higher current avg produces up trend", prop.ForAll(
		func(baseAvg float64, multiplier float64) bool {
			// Compare stats with baseAvg
			compareStats := TimeSeriesStats{
				Avg:   baseAvg,
				Max:   baseAvg + 10,
				Min:   baseAvg - 10,
				Count: 10,
			}
			// Current stats with significantly higher avg
			currentAvg := baseAvg + math.Abs(baseAvg)*multiplier
			currentStats := TimeSeriesStats{
				Avg:   currentAvg,
				Max:   currentAvg + 10,
				Min:   currentAvg - 10,
				Count: 10,
			}

			diff := AnalyzeTrend(currentStats, &compareStats)
			if diff.Trend != TrendUp {
				t.Logf("Expected up trend for baseAvg=%f, currentAvg=%f, got %s (changePercent=%f)",
					baseAvg, currentAvg, diff.Trend, diff.AvgChangePercent)
				return false
			}
			return true
		},
		gen.Float64Range(1, 1000).WithLabel("baseAvg"),
		gen.Float64Range(0.1, 10.0).WithLabel("multiplier"),
	))

	// Sub-property: significantly lower current avg → down trend
	properties.Property("significantly lower current avg produces down trend", prop.ForAll(
		func(baseAvg float64, multiplier float64) bool {
			compareStats := TimeSeriesStats{
				Avg:   baseAvg,
				Max:   baseAvg + 10,
				Min:   baseAvg - 10,
				Count: 10,
			}
			currentAvg := baseAvg - math.Abs(baseAvg)*multiplier
			currentStats := TimeSeriesStats{
				Avg:   currentAvg,
				Max:   currentAvg + 10,
				Min:   currentAvg - 10,
				Count: 10,
			}

			diff := AnalyzeTrend(currentStats, &compareStats)
			if diff.Trend != TrendDown {
				t.Logf("Expected down trend for baseAvg=%f, currentAvg=%f, got %s (changePercent=%f)",
					baseAvg, currentAvg, diff.Trend, diff.AvgChangePercent)
				return false
			}
			return true
		},
		gen.Float64Range(1, 1000).WithLabel("baseAvg"),
		gen.Float64Range(0.1, 10.0).WithLabel("multiplier"),
	))

	properties.TestingRun(t)
}

// ---------------------------------------------------------------------------
// Property 17: 差异评分属性
// ---------------------------------------------------------------------------

// TestProperty_DiffScoreProperties verifies that:
//   - For any two identical time series stats, the diff score is 0.
//   - For any two different time series stats, the diff score is > 0.
//
// Feature: go-mcp-server-rewrite, Property 17: 差异评分属性
// **Validates: Requirements 14.4**
func TestProperty_DiffScoreProperties(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Sub-property: identical stats → diff score == 0
	properties.Property("identical stats produce zero diff score", prop.ForAll(
		func(values []float64) bool {
			stats := ComputeStats(values, nil)
			compareCopy := stats
			score := CalculateDiffScore(stats, &compareCopy)
			if score != 0.0 {
				t.Logf("Expected 0 diff score for identical stats, got %f", score)
				return false
			}
			return true
		},
		genNonEmptyFloat64Slice(),
	))

	// Sub-property: different stats → diff score > 0
	properties.Property("different stats produce positive diff score", prop.ForAll(
		func(values1 []float64, values2 []float64) bool {
			stats1 := ComputeStats(values1, nil)
			stats2 := ComputeStats(values2, nil)

			// Skip if stats happen to be identical
			if stats1.Avg == stats2.Avg && stats1.Max == stats2.Max && stats1.Min == stats2.Min {
				return true // vacuously true, skip this case
			}

			score := CalculateDiffScore(stats1, &stats2)
			if score <= 0.0 {
				t.Logf("Expected positive diff score for different stats, got %f (stats1=%+v, stats2=%+v)",
					score, stats1, stats2)
				return false
			}
			return true
		},
		genNonEmptyFloat64Slice(),
		genNonEmptyFloat64Slice(),
	))

	// Sub-property: diff score is in [0, 1] range
	properties.Property("diff score is bounded in [0, 1]", prop.ForAll(
		func(values1 []float64, values2 []float64) bool {
			stats1 := ComputeStats(values1, nil)
			stats2 := ComputeStats(values2, nil)
			score := CalculateDiffScore(stats1, &stats2)
			if score < 0.0 || score > 1.0 {
				t.Logf("Diff score %f out of [0, 1] range", score)
				return false
			}
			return true
		},
		genNonEmptyFloat64Slice(),
		genNonEmptyFloat64Slice(),
	))

	properties.TestingRun(t)
}

// ---------------------------------------------------------------------------
// Property 18: 对比结果排序不变量
// ---------------------------------------------------------------------------

// TestProperty_CompareResultSortInvariant verifies that after sorting by diff
// score, for all adjacent elements results[i] and results[i+1], we have
// results[i].DiffScore >= results[i+1].DiffScore (descending order).
//
// Feature: go-mcp-server-rewrite, Property 18: 对比结果排序不变量
// **Validates: Requirements 14.5**
func TestProperty_CompareResultSortInvariant(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator: a slice of diff scores in [0, 1]
	genDiffScores := gen.SliceOfN(20, gen.Float64Range(0, 1)).SuchThat(func(v []float64) bool {
		return len(v) >= 2
	})

	properties.Property("sorted results are in descending order of DiffScore", prop.ForAll(
		func(scores []float64) bool {
			// Build a slice of TimeSeriesCompareResult with the generated scores
			results := make([]TimeSeriesCompareResult, len(scores))
			for i, s := range scores {
				results[i] = TimeSeriesCompareResult{
					DiffScore: s,
					Key:       TimeSeriesKey{EntityID: "e", Labels: "{}"},
				}
			}

			SortByDiffScore(results)

			for i := 0; i < len(results)-1; i++ {
				if results[i].DiffScore < results[i+1].DiffScore {
					t.Logf("Sort invariant violated at index %d: %f < %f",
						i, results[i].DiffScore, results[i+1].DiffScore)
					return false
				}
			}
			return true
		},
		genDiffScores,
	))

	// Sub-property: sorting is idempotent (sorting twice gives same result)
	properties.Property("sorting is idempotent", prop.ForAll(
		func(scores []float64) bool {
			build := func() []TimeSeriesCompareResult {
				results := make([]TimeSeriesCompareResult, len(scores))
				for i, s := range scores {
					results[i] = TimeSeriesCompareResult{DiffScore: s}
				}
				return results
			}

			results1 := build()
			SortByDiffScore(results1)

			results2 := make([]TimeSeriesCompareResult, len(results1))
			copy(results2, results1)
			SortByDiffScore(results2)

			for i := range results1 {
				if results1[i].DiffScore != results2[i].DiffScore {
					return false
				}
			}
			return true
		},
		genDiffScores,
	))

	// Sub-property: sorted result is a permutation of the original (same elements)
	properties.Property("sorting preserves all elements", prop.ForAll(
		func(scores []float64) bool {
			results := make([]TimeSeriesCompareResult, len(scores))
			for i, s := range scores {
				results[i] = TimeSeriesCompareResult{DiffScore: s}
			}

			originalScores := make([]float64, len(scores))
			copy(originalScores, scores)

			SortByDiffScore(results)

			sortedScores := make([]float64, len(results))
			for i, r := range results {
				sortedScores[i] = r.DiffScore
			}

			sort.Float64s(originalScores)
			sort.Float64s(sortedScores)

			if len(originalScores) != len(sortedScores) {
				return false
			}
			for i := range originalScores {
				if originalScores[i] != sortedScores[i] {
					return false
				}
			}
			return true
		},
		genDiffScores,
	))

	properties.TestingRun(t)
}
