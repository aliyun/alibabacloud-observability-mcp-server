package paas

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// KeyType identifies how to build the unique key for a time series.
const (
	KeyTypeMetrics       = "metrics"
	KeyTypeGoldenMetrics = "golden_metrics"
)

// Trend represents the direction of change between two time periods.
type Trend string

const (
	TrendUp          Trend = "up"
	TrendDown        Trend = "down"
	TrendStable      Trend = "stable"
	TrendNew         Trend = "new"
	TrendDisappeared Trend = "disappeared"
)

// TrendThresholdPercent is the threshold for trend determination.
const TrendThresholdPercent = 5.0

// ---------------------------------------------------------------------------
// Data models
// ---------------------------------------------------------------------------

// TimeSeriesKey uniquely identifies a time series for matching across periods.
type TimeSeriesKey struct {
	EntityID    string `json:"entity_id"`
	Labels      string `json:"labels"`
	Metric      string `json:"metric,omitempty"`
	MetricSetID string `json:"metric_set_id,omitempty"`
}

// Hash returns a unique string for matching time series across periods.
func (k TimeSeriesKey) Hash() string {
	return k.Metric + "|" + k.MetricSetID + "|" + k.EntityID + "|" + k.Labels
}

// TimeSeriesStats holds statistical summary of a time series.
type TimeSeriesStats struct {
	Max     float64 `json:"max"`
	Min     float64 `json:"min"`
	Avg     float64 `json:"avg"`
	Count   int     `json:"count"`
	MaxTime string  `json:"max_time"`
	MinTime string  `json:"min_time"`
}

// DiffDetails holds the difference analysis between two periods.
type DiffDetails struct {
	Trend            Trend   `json:"trend"`
	AvgChange        float64 `json:"avg_change"`
	AvgChangePercent float64 `json:"avg_change_percent"`
	MaxChange        float64 `json:"max_change"`
	MinChange        float64 `json:"min_change"`
}

// TimeSeriesData holds a single time series with its values and stats.
type TimeSeriesData struct {
	Key        TimeSeriesKey
	Stats      TimeSeriesStats
	Values     []float64
	Timestamps []int64
}

// TimeSeriesCompareResult holds the comparison result for a single time series.
type TimeSeriesCompareResult struct {
	Key          TimeSeriesKey    `json:"key"`
	CurrentStats TimeSeriesStats  `json:"current"`
	CompareStats *TimeSeriesStats `json:"compare,omitempty"`
	DiffScore    float64          `json:"diff_score"`
	DiffDetails  DiffDetails      `json:"diff"`
}

// TimeRangeInfo holds human-readable and unix time range information.
type TimeRangeInfo struct {
	FromTime string `json:"from"`
	ToTime   string `json:"to"`
	FromUnix int64  `json:"from_unix"`
	ToUnix   int64  `json:"to_unix"`
}

// CompareOutput holds the complete comparison output.
type CompareOutput struct {
	CompareEnabled   bool                      `json:"compare_enabled"`
	CurrentTimeRange TimeRangeInfo             `json:"current_time_range"`
	CompareTimeRange *TimeRangeInfo            `json:"compare_time_range,omitempty"`
	Offset           string                    `json:"offset"`
	TotalSeries      int                       `json:"total_series"`
	Results          []TimeSeriesCompareResult `json:"results"`
}

// ---------------------------------------------------------------------------
// Core functions (exported for testing)
// ---------------------------------------------------------------------------

// FormatTimestampNs formats a nanosecond timestamp to "YYYY-MM-DD HH:MM:SS" (UTC).
func FormatTimestampNs(timestampNs int64) string {
	if timestampNs <= 0 {
		return ""
	}
	ts := timestampNs / 1_000_000_000
	t := time.Unix(ts, 0).UTC()
	return t.Format("2006-01-02 15:04:05")
}

// ParseDurationToSeconds parses a duration string like "30m", "1h", "1d", "1w"
// into seconds. Returns 0 for invalid input.
func ParseDurationToSeconds(duration string) int64 {
	duration = strings.TrimSpace(strings.ToLower(duration))
	if duration == "" {
		return 0
	}

	re := regexp.MustCompile(`^(\d+)([smhdw])$`)
	matches := re.FindStringSubmatch(duration)
	if matches == nil {
		return 0
	}

	value, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil || value == 0 {
		return 0
	}

	unitToSeconds := map[string]int64{
		"s": 1,
		"m": 60,
		"h": 3600,
		"d": 86400,
		"w": 604800,
	}

	multiplier, ok := unitToSeconds[matches[2]]
	if !ok {
		return 0
	}
	return value * multiplier
}

// ComputeStats calculates statistical summary for a time series.
func ComputeStats(values []float64, timestamps []int64) TimeSeriesStats {
	if len(values) == 0 {
		return TimeSeriesStats{}
	}

	count := len(values)
	maxVal := values[0]
	minVal := values[0]
	sum := 0.0
	maxIdx := 0
	minIdx := 0

	for i, v := range values {
		sum += v
		if v > maxVal {
			maxVal = v
			maxIdx = i
		}
		if v < minVal {
			minVal = v
			minIdx = i
		}
	}

	avg := sum / float64(count)

	maxTime := ""
	minTime := ""
	if len(timestamps) > maxIdx {
		maxTime = FormatTimestampNs(timestamps[maxIdx])
	}
	if len(timestamps) > minIdx {
		minTime = FormatTimestampNs(timestamps[minIdx])
	}

	return TimeSeriesStats{
		Max:     maxVal,
		Min:     minVal,
		Avg:     avg,
		Count:   count,
		MaxTime: maxTime,
		MinTime: minTime,
	}
}

// AnalyzeTrend analyzes the change trend between current and compare stats.
func AnalyzeTrend(currentStats TimeSeriesStats, compareStats *TimeSeriesStats) DiffDetails {
	currentHasData := currentStats.Count > 0
	compareHasData := compareStats != nil && compareStats.Count > 0

	// Scenario 1: current has data, compare doesn't -> new
	if currentHasData && !compareHasData {
		return DiffDetails{
			Trend:            TrendNew,
			AvgChange:        currentStats.Avg,
			AvgChangePercent: 100.0,
			MaxChange:        currentStats.Max,
			MinChange:        currentStats.Min,
		}
	}

	// Scenario 2: current has no data, compare does -> disappeared
	if !currentHasData && compareHasData {
		return DiffDetails{
			Trend:            TrendDisappeared,
			AvgChange:        -compareStats.Avg,
			AvgChangePercent: -100.0,
			MaxChange:        -compareStats.Max,
			MinChange:        -compareStats.Min,
		}
	}

	// Scenario 3: both have no data -> stable
	if !currentHasData && !compareHasData {
		return DiffDetails{Trend: TrendStable}
	}

	// Scenario 4: both have data -> compute changes
	avgChange := currentStats.Avg - compareStats.Avg
	maxChange := currentStats.Max - compareStats.Max
	minChange := currentStats.Min - compareStats.Min

	var avgChangePercent float64
	if compareStats.Avg == 0 {
		if currentStats.Avg == 0 {
			avgChangePercent = 0.0
		} else if currentStats.Avg > 0 {
			avgChangePercent = 100.0
		} else {
			avgChangePercent = -100.0
		}
	} else {
		avgChangePercent = (avgChange / math.Abs(compareStats.Avg)) * 100.0
	}

	var trend Trend
	if avgChangePercent > TrendThresholdPercent {
		trend = TrendUp
	} else if avgChangePercent < -TrendThresholdPercent {
		trend = TrendDown
	} else {
		trend = TrendStable
	}

	return DiffDetails{
		Trend:            trend,
		AvgChange:        avgChange,
		AvgChangePercent: avgChangePercent,
		MaxChange:        maxChange,
		MinChange:        minChange,
	}
}

// CalculateDiffScore computes a 0-1 score quantifying the difference between periods.
func CalculateDiffScore(currentStats TimeSeriesStats, compareStats *TimeSeriesStats) float64 {
	currentHasData := currentStats.Count > 0
	compareHasData := compareStats != nil && compareStats.Count > 0

	// New or disappeared -> max score
	if currentHasData != compareHasData {
		return 1.0
	}
	// Both empty
	if !currentHasData && !compareHasData {
		return 0.0
	}

	safeChangeRate := func(current, compare float64) float64 {
		if compare == 0 {
			if current != 0 {
				return 1.0
			}
			return 0.0
		}
		return (current - compare) / math.Abs(compare)
	}

	avgRate := safeChangeRate(currentStats.Avg, compareStats.Avg)
	maxRate := safeChangeRate(currentStats.Max, compareStats.Max)
	minRate := safeChangeRate(currentStats.Min, compareStats.Min)

	const (
		avgWeight = 0.5
		maxWeight = 0.3
		minWeight = 0.2
	)

	score := math.Abs(avgRate)*avgWeight + math.Abs(maxRate)*maxWeight + math.Abs(minRate)*minWeight

	// Normalize to 0-1 (cap at 200% change)
	if score > 2.0 {
		score = 2.0
	}
	return score / 2.0
}

// ---------------------------------------------------------------------------
// Time series parsing and comparison
// ---------------------------------------------------------------------------

// ParseTimeSeriesData parses API response data into TimeSeriesData objects.
func ParseTimeSeriesData(data []interface{}, keyType string) []TimeSeriesData {
	var results []TimeSeriesData

	for _, item := range data {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		entityID := fmt.Sprintf("%v", row["__entity_id__"])
		if entityID == "<nil>" {
			entityID = ""
		}

		labelsRaw := row["__labels__"]
		var labels string
		switch v := labelsRaw.(type) {
		case map[string]interface{}:
			b, _ := json.Marshal(v)
			labels = string(b)
		case string:
			labels = v
		default:
			labels = "{}"
		}

		key := TimeSeriesKey{
			EntityID: entityID,
			Labels:   labels,
		}
		if keyType == KeyTypeGoldenMetrics {
			if m, ok := row["metric"]; ok {
				key.Metric = fmt.Sprintf("%v", m)
			}
			if m, ok := row["metric_set_id"]; ok {
				key.MetricSetID = fmt.Sprintf("%v", m)
			}
		}

		values := parseFloatArray(row["__value__"])
		timestamps := parseInt64Array(row["__ts__"])
		stats := ComputeStats(values, timestamps)

		results = append(results, TimeSeriesData{
			Key:        key,
			Stats:      stats,
			Values:     values,
			Timestamps: timestamps,
		})
	}

	return results
}

// parseFloatArray extracts a float64 slice from an interface value.
func parseFloatArray(value interface{}) []float64 {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case []interface{}:
		var result []float64
		for _, item := range v {
			f, err := toFloat64(item)
			if err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
				result = append(result, f)
			}
		}
		return result
	case []float64:
		return v
	case string:
		var parsed []interface{}
		if err := json.Unmarshal([]byte(v), &parsed); err == nil {
			return parseFloatArray(parsed)
		}
	}
	return nil
}

// parseInt64Array extracts an int64 slice from an interface value.
func parseInt64Array(value interface{}) []int64 {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case []interface{}:
		var result []int64
		for _, item := range v {
			n, err := toInt64(item)
			if err == nil {
				result = append(result, n)
			}
		}
		return result
	case []int64:
		return v
	case string:
		var parsed []interface{}
		if err := json.Unmarshal([]byte(v), &parsed); err == nil {
			return parseInt64Array(parsed)
		}
	}
	return nil
}

func toFloat64(v interface{}) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case json.Number:
		return n.Float64()
	case string:
		return strconv.ParseFloat(n, 64)
	}
	return 0, fmt.Errorf("cannot convert %T to float64", v)
}

func toInt64(v interface{}) (int64, error) {
	switch n := v.(type) {
	case float64:
		return int64(n), nil
	case int:
		return int64(n), nil
	case int64:
		return n, nil
	case json.Number:
		return n.Int64()
	case string:
		return strconv.ParseInt(n, 10, 64)
	}
	return 0, fmt.Errorf("cannot convert %T to int64", v)
}

// CompareTimeSeries compares current and previous period time series data.
func CompareTimeSeries(currentData, compareData []TimeSeriesData) []TimeSeriesCompareResult {
	// Build hash map for compare data
	compareMap := make(map[string]TimeSeriesData, len(compareData))
	for _, ts := range compareData {
		compareMap[ts.Key.Hash()] = ts
	}

	matched := make(map[string]bool)
	var results []TimeSeriesCompareResult

	// Process current period data
	for _, current := range currentData {
		hashKey := current.Key.Hash()

		var compareStats *TimeSeriesStats
		if compareTSD, ok := compareMap[hashKey]; ok {
			matched[hashKey] = true
			s := compareTSD.Stats
			compareStats = &s
		}

		diffDetails := AnalyzeTrend(current.Stats, compareStats)
		diffScore := CalculateDiffScore(current.Stats, compareStats)

		results = append(results, TimeSeriesCompareResult{
			Key:          current.Key,
			CurrentStats: current.Stats,
			CompareStats: compareStats,
			DiffScore:    diffScore,
			DiffDetails:  diffDetails,
		})
	}

	// Add disappeared time series (only in compare period)
	for _, compareTSD := range compareData {
		hashKey := compareTSD.Key.Hash()
		if matched[hashKey] {
			continue
		}

		emptyStats := TimeSeriesStats{}
		s := compareTSD.Stats
		diffDetails := AnalyzeTrend(emptyStats, &s)
		diffScore := CalculateDiffScore(emptyStats, &s)

		results = append(results, TimeSeriesCompareResult{
			Key:          compareTSD.Key,
			CurrentStats: emptyStats,
			CompareStats: &s,
			DiffScore:    diffScore,
			DiffDetails:  diffDetails,
		})
	}

	return results
}

// SortByDiffScore sorts results by diff score descending (highest first).
func SortByDiffScore(results []TimeSeriesCompareResult) {
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].DiffScore > results[j].DiffScore
	})
}

// FormatTimeRange creates a TimeRangeInfo from unix timestamps (seconds).
func FormatTimeRange(fromTS, toTS int64) TimeRangeInfo {
	fromDT := time.Unix(fromTS, 0).UTC()
	toDT := time.Unix(toTS, 0).UTC()
	return TimeRangeInfo{
		FromTime: fromDT.Format("2006-01-02 15:04:05"),
		ToTime:   toDT.Format("2006-01-02 15:04:05"),
		FromUnix: fromTS,
		ToUnix:   toTS,
	}
}

// BuildCompareOutput constructs the full comparison output.
func BuildCompareOutput(
	currentData, compareData []TimeSeriesData,
	currentFrom, currentTo, compareFrom, compareTo int64,
	offsetSeconds int64,
) CompareOutput {
	results := CompareTimeSeries(currentData, compareData)
	SortByDiffScore(results)

	compareRange := FormatTimeRange(compareFrom, compareTo)
	return CompareOutput{
		CompareEnabled:   true,
		CurrentTimeRange: FormatTimeRange(currentFrom, currentTo),
		CompareTimeRange: &compareRange,
		Offset:           fmt.Sprintf("%ds", offsetSeconds),
		TotalSeries:      len(results),
		Results:          results,
	}
}

// toInterfaceSlice attempts to convert an interface{} to []interface{}.
func toInterfaceSlice(v interface{}) ([]interface{}, bool) {
	if v == nil {
		return nil, false
	}
	s, ok := v.([]interface{})
	return s, ok
}
