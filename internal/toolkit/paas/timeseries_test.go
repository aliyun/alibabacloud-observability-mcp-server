package paas

import (
	"context"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TimeseriesTools: tool count, names, prefix
// ---------------------------------------------------------------------------

func TestTimeseriesTools_Count(t *testing.T) {
	tools := TimeseriesTools(&mockCMSClient{})
	if got := len(tools); got != 1 {
		t.Fatalf("TimeseriesTools() returned %d tools, want 1", got)
	}
}

func TestTimeseriesTools_Names(t *testing.T) {
	tools := TimeseriesTools(&mockCMSClient{})
	want := "umodel_compare_metrics"
	if tools[0].Name != want {
		t.Fatalf("tool name = %q, want %q", tools[0].Name, want)
	}
}

func TestTimeseriesTools_AllHaveUmodelPrefix(t *testing.T) {
	tools := TimeseriesTools(&mockCMSClient{})
	for _, tool := range tools {
		if !strings.HasPrefix(tool.Name, "umodel_") {
			t.Errorf("tool %q does not have umodel_ prefix", tool.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// Missing params validation
// ---------------------------------------------------------------------------

func TestHandleCompareMetrics_MissingParams(t *testing.T) {
	tools := TimeseriesTools(&mockCMSClient{})
	handler := tools[0].Handler

	resp, err := handler(context.Background(), map[string]interface{}{
		"domain": "apm",
		// missing other required params
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := resp.(map[string]interface{})
	if !ok {
		t.Fatalf("response is not map[string]interface{}")
	}
	if m["error"] != true {
		t.Errorf("expected error=true for missing params")
	}
}

func TestHandleCompareMetrics_InvalidOffset(t *testing.T) {
	tools := TimeseriesTools(&mockCMSClient{})
	handler := tools[0].Handler

	resp, err := handler(context.Background(), map[string]interface{}{
		"domain":             "apm",
		"entity_set_name":    "apm.service",
		"metric_domain_name": "apm.metric.service",
		"metric":             "cpu_usage",
		"workspace":          "ws",
		"regionId":           "cn-hangzhou",
		"offset":             "invalid",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := resp.(map[string]interface{})
	if m["error"] != true {
		t.Errorf("expected error=true for invalid offset")
	}
}

func TestHandleCompareMetrics_IncompatibleMetric(t *testing.T) {
	tools := TimeseriesTools(&mockCMSClient{})
	handler := tools[0].Handler

	resp, err := handler(context.Background(), map[string]interface{}{
		"domain":             "apm",
		"entity_set_name":    "apm.service",
		"metric_domain_name": "apm.metric.jvm",
		"metric":             "arms_jvm_mem_used_bytes",
		"workspace":          "ws",
		"regionId":           "cn-hangzhou",
		"offset":             "1d",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := resp.(map[string]interface{})
	if m["error"] != true {
		t.Errorf("expected error=true for incompatible metric/entity combination")
	}
	msg, _ := m["message"].(string)
	if msg == "" {
		t.Errorf("expected non-empty error message with compatibility hint")
	}
}

// ---------------------------------------------------------------------------
// ParseDurationToSeconds
// ---------------------------------------------------------------------------

func TestParseDurationToSeconds(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"30s", 30},
		{"30m", 1800},
		{"1h", 3600},
		{"1d", 86400},
		{"1w", 604800},
		{"", 0},
		{"invalid", 0},
		{"0d", 0},
		{"  1h  ", 3600},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseDurationToSeconds(tt.input)
			if got != tt.want {
				t.Errorf("ParseDurationToSeconds(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ComputeStats
// ---------------------------------------------------------------------------

func TestComputeStats_Empty(t *testing.T) {
	stats := ComputeStats(nil, nil)
	if stats.Count != 0 {
		t.Errorf("Count = %d, want 0", stats.Count)
	}
	if stats.Max != 0 || stats.Min != 0 || stats.Avg != 0 {
		t.Errorf("expected zero stats for empty input")
	}
}

func TestComputeStats_SingleValue(t *testing.T) {
	stats := ComputeStats([]float64{42.0}, []int64{1000000000000})
	if stats.Count != 1 {
		t.Errorf("Count = %d, want 1", stats.Count)
	}
	if stats.Max != 42.0 || stats.Min != 42.0 || stats.Avg != 42.0 {
		t.Errorf("stats = {Max:%v, Min:%v, Avg:%v}, want all 42.0", stats.Max, stats.Min, stats.Avg)
	}
}

func TestComputeStats_MultipleValues(t *testing.T) {
	values := []float64{10.0, 20.0, 30.0, 40.0, 50.0}
	timestamps := []int64{
		1000000000000, 2000000000000, 3000000000000,
		4000000000000, 5000000000000,
	}
	stats := ComputeStats(values, timestamps)

	if stats.Count != 5 {
		t.Errorf("Count = %d, want 5", stats.Count)
	}
	if stats.Min != 10.0 {
		t.Errorf("Min = %v, want 10.0", stats.Min)
	}
	if stats.Max != 50.0 {
		t.Errorf("Max = %v, want 50.0", stats.Max)
	}
	if stats.Avg != 30.0 {
		t.Errorf("Avg = %v, want 30.0", stats.Avg)
	}
}

func TestComputeStats_MathInvariant(t *testing.T) {
	values := []float64{5.0, 1.0, 9.0, 3.0, 7.0}
	stats := ComputeStats(values, nil)

	if stats.Min > stats.Avg || stats.Avg > stats.Max {
		t.Errorf("invariant violated: Min(%v) <= Avg(%v) <= Max(%v)", stats.Min, stats.Avg, stats.Max)
	}
	if stats.Count != len(values) {
		t.Errorf("Count = %d, want %d", stats.Count, len(values))
	}
}

// ---------------------------------------------------------------------------
// AnalyzeTrend
// ---------------------------------------------------------------------------

func TestAnalyzeTrend_New(t *testing.T) {
	current := TimeSeriesStats{Avg: 10.0, Max: 20.0, Min: 5.0, Count: 3}
	diff := AnalyzeTrend(current, nil)
	if diff.Trend != TrendNew {
		t.Errorf("Trend = %q, want %q", diff.Trend, TrendNew)
	}
}

func TestAnalyzeTrend_Disappeared(t *testing.T) {
	compare := TimeSeriesStats{Avg: 10.0, Max: 20.0, Min: 5.0, Count: 3}
	diff := AnalyzeTrend(TimeSeriesStats{}, &compare)
	if diff.Trend != TrendDisappeared {
		t.Errorf("Trend = %q, want %q", diff.Trend, TrendDisappeared)
	}
}

func TestAnalyzeTrend_Stable(t *testing.T) {
	stats := TimeSeriesStats{Avg: 100.0, Max: 110.0, Min: 90.0, Count: 10}
	diff := AnalyzeTrend(stats, &stats)
	if diff.Trend != TrendStable {
		t.Errorf("Trend = %q, want %q", diff.Trend, TrendStable)
	}
	if diff.AvgChangePercent != 0.0 {
		t.Errorf("AvgChangePercent = %v, want 0.0", diff.AvgChangePercent)
	}
}

func TestAnalyzeTrend_Up(t *testing.T) {
	current := TimeSeriesStats{Avg: 120.0, Max: 130.0, Min: 110.0, Count: 10}
	compare := TimeSeriesStats{Avg: 100.0, Max: 110.0, Min: 90.0, Count: 10}
	diff := AnalyzeTrend(current, &compare)
	if diff.Trend != TrendUp {
		t.Errorf("Trend = %q, want %q", diff.Trend, TrendUp)
	}
}

func TestAnalyzeTrend_Down(t *testing.T) {
	current := TimeSeriesStats{Avg: 80.0, Max: 90.0, Min: 70.0, Count: 10}
	compare := TimeSeriesStats{Avg: 100.0, Max: 110.0, Min: 90.0, Count: 10}
	diff := AnalyzeTrend(current, &compare)
	if diff.Trend != TrendDown {
		t.Errorf("Trend = %q, want %q", diff.Trend, TrendDown)
	}
}

func TestAnalyzeTrend_BothEmpty(t *testing.T) {
	diff := AnalyzeTrend(TimeSeriesStats{}, nil)
	if diff.Trend != TrendStable {
		t.Errorf("Trend = %q, want %q for both empty", diff.Trend, TrendStable)
	}
}

func TestAnalyzeTrend_ExactThresholdIsStable(t *testing.T) {
	// Exactly 5% change should be stable (not up)
	current := TimeSeriesStats{Avg: 105.0, Count: 1}
	compare := TimeSeriesStats{Avg: 100.0, Count: 1}
	diff := AnalyzeTrend(current, &compare)
	if diff.Trend != TrendStable {
		t.Errorf("Trend = %q, want %q for exactly 5%% change", diff.Trend, TrendStable)
	}
}

// ---------------------------------------------------------------------------
// CalculateDiffScore
// ---------------------------------------------------------------------------

func TestCalculateDiffScore_IdenticalData(t *testing.T) {
	stats := TimeSeriesStats{Avg: 100.0, Max: 110.0, Min: 90.0, Count: 10}
	score := CalculateDiffScore(stats, &stats)
	if score != 0.0 {
		t.Errorf("DiffScore = %v, want 0.0 for identical data", score)
	}
}

func TestCalculateDiffScore_DifferentData(t *testing.T) {
	current := TimeSeriesStats{Avg: 200.0, Max: 220.0, Min: 180.0, Count: 10}
	compare := TimeSeriesStats{Avg: 100.0, Max: 110.0, Min: 90.0, Count: 10}
	score := CalculateDiffScore(current, &compare)
	if score <= 0.0 {
		t.Errorf("DiffScore = %v, want > 0 for different data", score)
	}
	if score > 1.0 {
		t.Errorf("DiffScore = %v, want <= 1.0", score)
	}
}

func TestCalculateDiffScore_NewTimeSeries(t *testing.T) {
	current := TimeSeriesStats{Avg: 100.0, Count: 5}
	score := CalculateDiffScore(current, nil)
	if score != 1.0 {
		t.Errorf("DiffScore = %v, want 1.0 for new time series", score)
	}
}

func TestCalculateDiffScore_DisappearedTimeSeries(t *testing.T) {
	compare := TimeSeriesStats{Avg: 100.0, Count: 5}
	score := CalculateDiffScore(TimeSeriesStats{}, &compare)
	if score != 1.0 {
		t.Errorf("DiffScore = %v, want 1.0 for disappeared time series", score)
	}
}

func TestCalculateDiffScore_BothEmpty(t *testing.T) {
	score := CalculateDiffScore(TimeSeriesStats{}, nil)
	if score != 0.0 {
		t.Errorf("DiffScore = %v, want 0.0 for both empty", score)
	}
}

func TestCalculateDiffScore_Range(t *testing.T) {
	current := TimeSeriesStats{Avg: 150.0, Max: 160.0, Min: 140.0, Count: 10}
	compare := TimeSeriesStats{Avg: 100.0, Max: 110.0, Min: 90.0, Count: 10}
	score := CalculateDiffScore(current, &compare)
	if score < 0.0 || score > 1.0 {
		t.Errorf("DiffScore = %v, want in [0, 1]", score)
	}
}

// ---------------------------------------------------------------------------
// SortByDiffScore
// ---------------------------------------------------------------------------

func TestSortByDiffScore_DescendingOrder(t *testing.T) {
	results := []TimeSeriesCompareResult{
		{DiffScore: 0.1},
		{DiffScore: 0.9},
		{DiffScore: 0.5},
		{DiffScore: 0.3},
		{DiffScore: 0.7},
	}
	SortByDiffScore(results)

	for i := 0; i < len(results)-1; i++ {
		if results[i].DiffScore < results[i+1].DiffScore {
			t.Errorf("results[%d].DiffScore (%v) < results[%d].DiffScore (%v), want descending",
				i, results[i].DiffScore, i+1, results[i+1].DiffScore)
		}
	}
}

func TestSortByDiffScore_Empty(t *testing.T) {
	var results []TimeSeriesCompareResult
	SortByDiffScore(results) // should not panic
}

// ---------------------------------------------------------------------------
// CompareTimeSeries
// ---------------------------------------------------------------------------

func TestCompareTimeSeries_MatchingKeys(t *testing.T) {
	key := TimeSeriesKey{EntityID: "e1", Labels: "{}"}
	current := []TimeSeriesData{
		{Key: key, Stats: TimeSeriesStats{Avg: 100.0, Count: 5}},
	}
	compare := []TimeSeriesData{
		{Key: key, Stats: TimeSeriesStats{Avg: 100.0, Count: 5}},
	}

	results := CompareTimeSeries(current, compare)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].CompareStats == nil {
		t.Error("CompareStats should not be nil for matched key")
	}
	if results[0].DiffScore != 0.0 {
		t.Errorf("DiffScore = %v, want 0.0 for identical data", results[0].DiffScore)
	}
}

func TestCompareTimeSeries_NewAndDisappeared(t *testing.T) {
	keyNew := TimeSeriesKey{EntityID: "new1", Labels: "{}"}
	keyGone := TimeSeriesKey{EntityID: "gone1", Labels: "{}"}

	current := []TimeSeriesData{
		{Key: keyNew, Stats: TimeSeriesStats{Avg: 50.0, Count: 3}},
	}
	compare := []TimeSeriesData{
		{Key: keyGone, Stats: TimeSeriesStats{Avg: 50.0, Count: 3}},
	}

	results := CompareTimeSeries(current, compare)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (1 new + 1 disappeared)", len(results))
	}

	// Both should have max diff score
	for _, r := range results {
		if r.DiffScore != 1.0 {
			t.Errorf("DiffScore = %v, want 1.0 for new/disappeared", r.DiffScore)
		}
	}
}

// ---------------------------------------------------------------------------
// TimeSeriesKey.Hash
// ---------------------------------------------------------------------------

func TestTimeSeriesKey_Hash(t *testing.T) {
	k1 := TimeSeriesKey{EntityID: "e1", Labels: "{}", Metric: "cpu", MetricSetID: "ms1"}
	k2 := TimeSeriesKey{EntityID: "e1", Labels: "{}", Metric: "cpu", MetricSetID: "ms1"}
	k3 := TimeSeriesKey{EntityID: "e2", Labels: "{}", Metric: "cpu", MetricSetID: "ms1"}

	if k1.Hash() != k2.Hash() {
		t.Error("identical keys should have same hash")
	}
	if k1.Hash() == k3.Hash() {
		t.Error("different keys should have different hash")
	}
}

// ---------------------------------------------------------------------------
// FormatTimestampNs
// ---------------------------------------------------------------------------

func TestFormatTimestampNs(t *testing.T) {
	// 2024-01-01 00:00:00 UTC = 1704067200 seconds = 1704067200000000000 ns
	got := FormatTimestampNs(1704067200000000000)
	want := "2024-01-01 00:00:00"
	if got != want {
		t.Errorf("FormatTimestampNs = %q, want %q", got, want)
	}
}

func TestFormatTimestampNs_Zero(t *testing.T) {
	got := FormatTimestampNs(0)
	if got != "" {
		t.Errorf("FormatTimestampNs(0) = %q, want empty", got)
	}
}

func TestFormatTimestampNs_Negative(t *testing.T) {
	got := FormatTimestampNs(-1)
	if got != "" {
		t.Errorf("FormatTimestampNs(-1) = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// ParseTimeSeriesData
// ---------------------------------------------------------------------------

func TestParseTimeSeriesData_Basic(t *testing.T) {
	data := []interface{}{
		map[string]interface{}{
			"__entity_id__": "e1",
			"__labels__":    "{}",
			"__value__":     []interface{}{10.0, 20.0, 30.0},
			"__ts__":        []interface{}{1000000000000.0, 2000000000000.0, 3000000000000.0},
		},
	}

	results := ParseTimeSeriesData(data, KeyTypeMetrics)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Stats.Count != 3 {
		t.Errorf("Count = %d, want 3", results[0].Stats.Count)
	}
	if results[0].Key.EntityID != "e1" {
		t.Errorf("EntityID = %q, want %q", results[0].Key.EntityID, "e1")
	}
}

func TestParseTimeSeriesData_GoldenMetrics(t *testing.T) {
	data := []interface{}{
		map[string]interface{}{
			"__entity_id__": "e1",
			"__labels__":    "{}",
			"__value__":     []interface{}{10.0},
			"__ts__":        []interface{}{1000000000000.0},
			"metric":        "cpu_usage",
			"metric_set_id": "ms1",
		},
	}

	results := ParseTimeSeriesData(data, KeyTypeGoldenMetrics)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Key.Metric != "cpu_usage" {
		t.Errorf("Metric = %q, want %q", results[0].Key.Metric, "cpu_usage")
	}
	if results[0].Key.MetricSetID != "ms1" {
		t.Errorf("MetricSetID = %q, want %q", results[0].Key.MetricSetID, "ms1")
	}
}

// ---------------------------------------------------------------------------
// BuildCompareOutput
// ---------------------------------------------------------------------------

func TestBuildCompareOutput(t *testing.T) {
	key := TimeSeriesKey{EntityID: "e1", Labels: "{}"}
	current := []TimeSeriesData{
		{Key: key, Stats: TimeSeriesStats{Avg: 200.0, Max: 220.0, Min: 180.0, Count: 5},
			Values: []float64{200.0}},
	}
	compare := []TimeSeriesData{
		{Key: key, Stats: TimeSeriesStats{Avg: 100.0, Max: 110.0, Min: 90.0, Count: 5},
			Values: []float64{100.0}},
	}

	output := BuildCompareOutput(current, compare, 1000, 2000, 500, 1500, 500)

	if !output.CompareEnabled {
		t.Error("CompareEnabled should be true")
	}
	if output.TotalSeries != 1 {
		t.Errorf("TotalSeries = %d, want 1", output.TotalSeries)
	}
	if output.Offset != "500s" {
		t.Errorf("Offset = %q, want %q", output.Offset, "500s")
	}
	if len(output.Results) != 1 {
		t.Fatalf("Results count = %d, want 1", len(output.Results))
	}
	if output.Results[0].DiffScore <= 0 {
		t.Errorf("DiffScore = %v, want > 0", output.Results[0].DiffScore)
	}
}

// ---------------------------------------------------------------------------
// handleCompareMetrics success path
// ---------------------------------------------------------------------------

func TestHandleCompareMetrics_Success(t *testing.T) {
	mock := &mockCMSClient{
		executeSPLFn: func(ctx context.Context, region, workspace, query string, from, to int64, limit int) (map[string]interface{}, error) {
			return map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"__entity_id__": "e1",
						"__labels__":    "{}",
						"__value__":     []interface{}{10.0, 20.0, 30.0},
						"__ts__":        []interface{}{1000000000000.0, 2000000000000.0, 3000000000000.0},
					},
				},
			}, nil
		},
	}

	tools := TimeseriesTools(mock)
	handler := tools[0].Handler

	resp, err := handler(context.Background(), map[string]interface{}{
		"domain":             "apm",
		"entity_set_name":    "apm.service",
		"metric_domain_name": "apm.metric.service",
		"metric":             "cpu_usage",
		"workspace":          "ws",
		"regionId":           "cn-hangzhou",
		"offset":             "1d",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m := resp.(map[string]interface{})
	if m["error"] != false {
		t.Errorf("expected error=false, got %v", m["error"])
	}
	if m["data"] == nil {
		t.Error("expected non-nil data")
	}
}


