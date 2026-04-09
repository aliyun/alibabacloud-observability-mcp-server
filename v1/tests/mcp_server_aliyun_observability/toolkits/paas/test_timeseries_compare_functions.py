"""时序数据对比模块 - 新增函数测试

测试 timeseries_compare.py 中新增的对比功能函数：
- calculate_diff_score: 计算差异评分
- parse_time_series_data: 解析时序数据
- compare_time_series: 对比时序数据
- sort_by_diff_score: 按评分排序
- format_time_range: 格式化时间范围
- build_compare_output: 构建对比输出
- compare_output_to_dict: 转换为字典格式
"""

import pytest
from typing import List, Dict, Any

from mcp_server_aliyun_observability.toolkits.paas.timeseries_compare import (
    TimeSeriesKey,
    TimeSeriesStats,
    TimeSeriesData,
    TimeSeriesCompareResult,
    TimeRangeInfo,
    CompareOutput,
    DiffDetails,
    Trend,
    KeyType,
    calculate_diff_score,
    parse_time_series_data,
    compare_time_series,
    sort_by_diff_score,
    format_time_range,
    build_compare_output,
    compare_output_to_dict,
)


class TestCalculateDiffScore:
    """测试 calculate_diff_score 函数"""

    def test_new_series_returns_max_score(self):
        """新增时序（当前有数据，对比无数据）应返回最高评分 1.0"""
        current = TimeSeriesStats(max=100, min=50, avg=75, count=10)
        compare = None
        
        score = calculate_diff_score(current, compare)
        
        assert score == 1.0

    def test_disappeared_series_returns_max_score(self):
        """消失时序（当前无数据，对比有数据）应返回最高评分 1.0"""
        current = TimeSeriesStats(count=0)
        compare = TimeSeriesStats(max=100, min=50, avg=75, count=10)
        
        score = calculate_diff_score(current, compare)
        
        assert score == 1.0

    def test_both_empty_returns_zero(self):
        """两者都无数据应返回 0"""
        current = TimeSeriesStats(count=0)
        compare = TimeSeriesStats(count=0)
        
        score = calculate_diff_score(current, compare)
        
        assert score == 0.0

    def test_no_change_returns_zero(self):
        """无变化应返回接近 0 的评分"""
        current = TimeSeriesStats(max=100, min=50, avg=75, count=10)
        compare = TimeSeriesStats(max=100, min=50, avg=75, count=10)
        
        score = calculate_diff_score(current, compare)
        
        assert score == 0.0

    def test_significant_change_returns_high_score(self):
        """显著变化应返回较高评分"""
        current = TimeSeriesStats(max=200, min=100, avg=150, count=10)
        compare = TimeSeriesStats(max=100, min=50, avg=75, count=10)
        
        score = calculate_diff_score(current, compare)
        
        # 100% 变化应该有较高评分
        assert score > 0.3

    def test_score_is_normalized(self):
        """评分应在 0-1 范围内"""
        current = TimeSeriesStats(max=1000, min=500, avg=750, count=10)
        compare = TimeSeriesStats(max=100, min=50, avg=75, count=10)
        
        score = calculate_diff_score(current, compare)
        
        assert 0 <= score <= 1


class TestParseTimeSeriesData:
    """测试 parse_time_series_data 函数"""

    def test_empty_data_returns_empty_list(self):
        """空数据应返回空列表"""
        result = parse_time_series_data([])
        
        assert result == []

    def test_parses_metrics_data(self):
        """应正确解析 metrics 类型数据"""
        data = [
            {
                "__entity_id__": "entity1",
                "__labels__": '{"service": "svc1"}',
                "__value__": [100.0, 110.0, 105.0],
                "__ts__": [1704067200000000000, 1704067260000000000, 1704067320000000000]
            }
        ]
        
        result = parse_time_series_data(data, KeyType.METRICS)
        
        assert len(result) == 1
        assert result[0].key.entity_id == "entity1"
        assert result[0].key.labels == '{"service": "svc1"}'
        assert result[0].key.metric == ""  # metrics 类型不包含 metric
        assert result[0].stats.count == 3
        assert result[0].stats.max == 110.0
        assert result[0].stats.min == 100.0

    def test_parses_golden_metrics_data(self):
        """应正确解析 golden_metrics 类型数据"""
        data = [
            {
                "__entity_id__": "entity1",
                "__labels__": '{"service": "svc1"}',
                "metric": "request_count",
                "metric_set_id": "metric_set_1",
                "__value__": [100.0, 110.0, 105.0],
                "__ts__": [1704067200000000000, 1704067260000000000, 1704067320000000000]
            }
        ]
        
        result = parse_time_series_data(data, KeyType.GOLDEN_METRICS)
        
        assert len(result) == 1
        assert result[0].key.entity_id == "entity1"
        assert result[0].key.metric == "request_count"
        assert result[0].key.metric_set_id == "metric_set_1"

    def test_handles_dict_labels(self):
        """应正确处理字典格式的 labels"""
        data = [
            {
                "__entity_id__": "entity1",
                "__labels__": {"service": "svc1", "env": "prod"},
                "__value__": [100.0],
                "__ts__": [1704067200000000000]
            }
        ]
        
        result = parse_time_series_data(data, KeyType.METRICS)
        
        assert len(result) == 1
        # labels 应该被转换为 JSON 字符串
        assert "service" in result[0].key.labels
        assert "svc1" in result[0].key.labels

    def test_handles_missing_fields(self):
        """应正确处理缺失字段"""
        data = [
            {
                "__entity_id__": "entity1"
                # 缺少 __labels__, __value__, __ts__
            }
        ]
        
        result = parse_time_series_data(data, KeyType.METRICS)
        
        assert len(result) == 1
        assert result[0].key.entity_id == "entity1"
        assert result[0].key.labels == "{}"
        assert result[0].stats.count == 0


class TestCompareTimeSeries:
    """测试 compare_time_series 函数"""

    def test_empty_data_returns_empty_results(self):
        """空数据应返回空结果"""
        result = compare_time_series([], [])
        
        assert result == []

    def test_matches_by_key_hash(self):
        """应通过 key hash 匹配时序"""
        key = TimeSeriesKey(entity_id="e1", labels="{}")
        current = [TimeSeriesData(
            key=key,
            stats=TimeSeriesStats(max=100, min=50, avg=75, count=10),
            values=[100, 50, 75],
            timestamps=[1, 2, 3]
        )]
        compare = [TimeSeriesData(
            key=key,
            stats=TimeSeriesStats(max=80, min=40, avg=60, count=10),
            values=[80, 40, 60],
            timestamps=[1, 2, 3]
        )]
        
        result = compare_time_series(current, compare)
        
        assert len(result) == 1
        assert result[0].current_stats.avg == 75
        assert result[0].compare_stats.avg == 60

    def test_identifies_new_series(self):
        """应识别新增时序"""
        key = TimeSeriesKey(entity_id="e1", labels="{}")
        current = [TimeSeriesData(
            key=key,
            stats=TimeSeriesStats(max=100, min=50, avg=75, count=10),
            values=[100, 50, 75],
            timestamps=[1, 2, 3]
        )]
        compare = []  # 对比时段无数据
        
        result = compare_time_series(current, compare)
        
        assert len(result) == 1
        assert result[0].diff_details.trend == Trend.NEW
        assert result[0].compare_stats is None

    def test_identifies_disappeared_series(self):
        """应识别消失时序"""
        key = TimeSeriesKey(entity_id="e1", labels="{}")
        current = []  # 当前时段无数据
        compare = [TimeSeriesData(
            key=key,
            stats=TimeSeriesStats(max=80, min=40, avg=60, count=10),
            values=[80, 40, 60],
            timestamps=[1, 2, 3]
        )]
        
        result = compare_time_series(current, compare)
        
        assert len(result) == 1
        assert result[0].diff_details.trend == Trend.DISAPPEARED


class TestSortByDiffScore:
    """测试 sort_by_diff_score 函数"""

    def test_sorts_descending(self):
        """应按评分降序排序"""
        results = [
            TimeSeriesCompareResult(
                key=TimeSeriesKey(entity_id="e1"),
                current_stats=TimeSeriesStats(),
                diff_score=0.3
            ),
            TimeSeriesCompareResult(
                key=TimeSeriesKey(entity_id="e2"),
                current_stats=TimeSeriesStats(),
                diff_score=0.8
            ),
            TimeSeriesCompareResult(
                key=TimeSeriesKey(entity_id="e3"),
                current_stats=TimeSeriesStats(),
                diff_score=0.5
            ),
        ]
        
        sorted_results = sort_by_diff_score(results)
        
        assert sorted_results[0].diff_score == 0.8
        assert sorted_results[1].diff_score == 0.5
        assert sorted_results[2].diff_score == 0.3

    def test_empty_list(self):
        """空列表应返回空列表"""
        result = sort_by_diff_score([])
        
        assert result == []


class TestFormatTimeRange:
    """测试 format_time_range 函数"""

    def test_formats_correctly(self):
        """应正确格式化时间范围"""
        from_ts = 1704067200  # 2024-01-01 00:00:00 UTC
        to_ts = 1704153600    # 2024-01-02 00:00:00 UTC
        
        result = format_time_range(from_ts, to_ts)
        
        assert result.from_unix == from_ts
        assert result.to_unix == to_ts
        assert "2024-01-01" in result.from_time
        assert "2024-01-02" in result.to_time


class TestBuildCompareOutput:
    """测试 build_compare_output 函数"""

    def test_builds_complete_output(self):
        """应构建完整的对比输出"""
        key = TimeSeriesKey(entity_id="e1", labels="{}")
        current_data = [TimeSeriesData(
            key=key,
            stats=TimeSeriesStats(max=100, min=50, avg=75, count=10),
            values=[100, 50, 75],
            timestamps=[1, 2, 3]
        )]
        compare_data = [TimeSeriesData(
            key=key,
            stats=TimeSeriesStats(max=80, min=40, avg=60, count=10),
            values=[80, 40, 60],
            timestamps=[1, 2, 3]
        )]
        
        result = build_compare_output(
            current_data=current_data,
            compare_data=compare_data,
            current_from=1704067200,
            current_to=1704070800,
            compare_from=1704063600,
            compare_to=1704067200,
            offset_seconds=3600
        )
        
        assert result.compare_enabled is True
        assert result.offset == "3600s"
        assert result.total_series == 1
        assert len(result.results) == 1

    def test_empty_data(self):
        """空数据应返回空结果"""
        result = build_compare_output(
            current_data=[],
            compare_data=[],
            current_from=1704067200,
            current_to=1704070800,
            compare_from=1704063600,
            compare_to=1704067200,
            offset_seconds=3600
        )
        
        assert result.compare_enabled is True
        assert result.total_series == 0
        assert result.results == []


class TestCompareOutputToDict:
    """测试 compare_output_to_dict 函数"""

    def test_converts_to_dict(self):
        """应正确转换为字典格式"""
        output = CompareOutput(
            compare_enabled=True,
            current_time_range=TimeRangeInfo(
                from_time="2024-01-01 00:00:00",
                to_time="2024-01-01 01:00:00",
                from_unix=1704067200,
                to_unix=1704070800
            ),
            compare_time_range=TimeRangeInfo(
                from_time="2023-12-31 23:00:00",
                to_time="2024-01-01 00:00:00",
                from_unix=1704063600,
                to_unix=1704067200
            ),
            offset="3600s",
            total_series=1,
            results=[
                TimeSeriesCompareResult(
                    key=TimeSeriesKey(entity_id="e1", labels="{}"),
                    current_stats=TimeSeriesStats(max=100, min=50, avg=75, count=10),
                    compare_stats=TimeSeriesStats(max=80, min=40, avg=60, count=10),
                    diff_score=0.25,
                    diff_details=DiffDetails(trend=Trend.UP, avg_change=15, avg_change_percent=25)
                )
            ]
        )
        
        result = compare_output_to_dict(output)
        
        assert result["compare_enabled"] is True
        assert result["offset"] == "3600s"
        assert result["total_series"] == 1
        assert len(result["results"]) == 1
        assert result["results"][0]["key"]["entity_id"] == "e1"
        assert result["results"][0]["current"]["avg"] == 75
        assert result["results"][0]["compare"]["avg"] == 60
        assert result["results"][0]["diff"]["trend"] == "up"

    def test_handles_none_compare_stats(self):
        """应正确处理 compare_stats 为 None 的情况"""
        output = CompareOutput(
            compare_enabled=True,
            current_time_range=TimeRangeInfo(),
            compare_time_range=TimeRangeInfo(),
            offset="3600s",
            total_series=1,
            results=[
                TimeSeriesCompareResult(
                    key=TimeSeriesKey(entity_id="e1"),
                    current_stats=TimeSeriesStats(max=100, min=50, avg=75, count=10),
                    compare_stats=None,  # 新增时序
                    diff_score=1.0,
                    diff_details=DiffDetails(trend=Trend.NEW)
                )
            ]
        )
        
        result = compare_output_to_dict(output)
        
        assert "compare" not in result["results"][0]
        assert result["results"][0]["diff"]["trend"] == "new"
