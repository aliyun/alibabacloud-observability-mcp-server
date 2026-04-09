"""时序数据对比模块 - 属性测试

Feature: metric-compare, Property 6: 输出结构完整性

**Validates: Requirements 6.1, 6.2, 7.1**

Property 6: 输出结构完整性
- *For any* 对比输出，结构应包含所有必要字段：
  - 顶层：`compare_enabled`, `current_time_range`, `compare_time_range`, `offset`, `total_series`, `results`
  - 每个结果项：`key`, `current_stats`, `compare_stats`, `diff_score`, `diff_details`
  - 时间范围：`from`, `to`, `from_unix`, `to_unix`

测试框架: pytest + hypothesis
最小迭代次数: 100 per property test
"""

from dataclasses import asdict, fields
from typing import List, Optional

import pytest
from hypothesis import given, settings, assume
from hypothesis import strategies as st

from mcp_server_aliyun_observability.toolkits.paas.timeseries_compare import (
    CompareOutput,
    DiffDetails,
    TimeRangeInfo,
    TimeSeriesCompareResult,
    TimeSeriesData,
    TimeSeriesKey,
    TimeSeriesStats,
    Trend,
    compute_stats,
    _format_timestamp_ns,
    parse_duration_to_seconds,
)


# ============================================================================
# Test Data Generation Strategies
# ============================================================================

# 生成有效的 entity_id
entity_id_strategy = st.text(
    alphabet=st.characters(whitelist_categories=('Ll', 'Lu', 'Nd')),
    min_size=1,
    max_size=32
)

# 生成有效的 metric 名称
metric_name_strategy = st.text(
    alphabet=st.characters(whitelist_categories=('Ll', 'Lu', 'Nd'), whitelist_characters='_'),
    min_size=1,
    max_size=50
)

# 生成有效的 metric_set_id
metric_set_id_strategy = st.text(
    alphabet=st.characters(whitelist_categories=('Ll', 'Lu', 'Nd'), whitelist_characters='@._'),
    min_size=1,
    max_size=100
)

# 生成有效的 labels JSON 字符串
labels_strategy = st.one_of(
    st.just("{}"),
    st.builds(
        lambda k, v: f'{{"{k}": "{v}"}}',
        st.text(alphabet='abcdefghijklmnopqrstuvwxyz', min_size=1, max_size=10),
        st.text(alphabet='abcdefghijklmnopqrstuvwxyz0123456789', min_size=1, max_size=20)
    )
)

# 生成 TimeSeriesKey
time_series_key_strategy = st.builds(
    TimeSeriesKey,
    entity_id=entity_id_strategy,
    labels=labels_strategy,
    metric=metric_name_strategy,
    metric_set_id=metric_set_id_strategy
)

# 生成有效的时间字符串 (YYYY-MM-DD HH:MM:SS 格式)
time_string_strategy = st.builds(
    lambda y, mo, d, h, mi, s: f"{y:04d}-{mo:02d}-{d:02d} {h:02d}:{mi:02d}:{s:02d}",
    st.integers(min_value=2020, max_value=2030),
    st.integers(min_value=1, max_value=12),
    st.integers(min_value=1, max_value=28),  # 使用 28 避免月份天数问题
    st.integers(min_value=0, max_value=23),
    st.integers(min_value=0, max_value=59),
    st.integers(min_value=0, max_value=59)
)

# 生成 TimeSeriesStats
time_series_stats_strategy = st.builds(
    TimeSeriesStats,
    max=st.floats(min_value=0, max_value=1e6, allow_nan=False, allow_infinity=False),
    min=st.floats(min_value=0, max_value=1e6, allow_nan=False, allow_infinity=False),
    avg=st.floats(min_value=0, max_value=1e6, allow_nan=False, allow_infinity=False),
    count=st.integers(min_value=0, max_value=10000),
    max_time=time_string_strategy,
    min_time=time_string_strategy
)

# 生成 DiffDetails
diff_details_strategy = st.builds(
    DiffDetails,
    trend=st.sampled_from([Trend.UP, Trend.DOWN, Trend.STABLE, Trend.NEW, Trend.DISAPPEARED]),
    avg_change=st.floats(min_value=-1e6, max_value=1e6, allow_nan=False, allow_infinity=False),
    avg_change_percent=st.floats(min_value=-100, max_value=1000, allow_nan=False, allow_infinity=False),
    max_change=st.floats(min_value=-1e6, max_value=1e6, allow_nan=False, allow_infinity=False),
    min_change=st.floats(min_value=-1e6, max_value=1e6, allow_nan=False, allow_infinity=False)
)

# 生成 TimeRangeInfo
time_range_info_strategy = st.builds(
    TimeRangeInfo,
    from_time=time_string_strategy,
    to_time=time_string_strategy,
    from_unix=st.integers(min_value=1577836800, max_value=1893456000),  # 2020-01-01 to 2030-01-01
    to_unix=st.integers(min_value=1577836800, max_value=1893456000)
)

# 生成 TimeSeriesCompareResult
time_series_compare_result_strategy = st.builds(
    TimeSeriesCompareResult,
    key=time_series_key_strategy,
    current_stats=time_series_stats_strategy,
    compare_stats=st.one_of(st.none(), time_series_stats_strategy),
    diff_score=st.floats(min_value=0, max_value=1, allow_nan=False, allow_infinity=False),
    diff_details=diff_details_strategy
)

# 生成 offset 字符串
offset_strategy = st.one_of(
    st.just(""),
    st.just("0s"),
    st.builds(lambda n: f"{n}s", st.integers(min_value=1, max_value=604800))  # 最多 1 周
)

# 生成 CompareOutput
compare_output_strategy = st.builds(
    CompareOutput,
    compare_enabled=st.booleans(),
    current_time_range=time_range_info_strategy,
    compare_time_range=st.one_of(st.none(), time_range_info_strategy),
    offset=offset_strategy,
    total_series=st.integers(min_value=0, max_value=1000),
    results=st.lists(time_series_compare_result_strategy, min_size=0, max_size=10)
)


# ============================================================================
# Strategies for Property 1: 时序统计计算正确性
# ============================================================================

# 生成有效的浮点数值（用于时序数据）
float_value_strategy = st.floats(
    min_value=-1e9, 
    max_value=1e9, 
    allow_nan=False, 
    allow_infinity=False
)

# 生成有效的纳秒时间戳（2020-01-01 到 2030-01-01）
# 纳秒时间戳 = 秒时间戳 * 1_000_000_000
timestamp_ns_strategy = st.integers(
    min_value=1577836800 * 1_000_000_000,  # 2020-01-01 00:00:00 UTC
    max_value=1893456000 * 1_000_000_000   # 2030-01-01 00:00:00 UTC
)

# 生成非空时序数据（values 和 timestamps 列表）
# 确保 values 和 timestamps 长度相同
@st.composite
def non_empty_time_series_strategy(draw):
    """生成非空的时序数据（values 和 timestamps 列表）
    
    确保：
    - values 列表非空
    - timestamps 列表与 values 长度相同
    - timestamps 按升序排列（模拟真实时序数据）
    """
    # 生成 1 到 100 个数据点
    size = draw(st.integers(min_value=1, max_value=100))
    
    # 生成 values 列表
    values = draw(st.lists(float_value_strategy, min_size=size, max_size=size))
    
    # 生成 timestamps 列表（按升序排列）
    base_ts = draw(timestamp_ns_strategy)
    # 每个时间点间隔 30 秒（30_000_000_000 纳秒）
    timestamps = [base_ts + i * 30_000_000_000 for i in range(size)]
    
    return values, timestamps


# ============================================================================
# Property Tests - Property 1: 时序统计计算正确性
# ============================================================================

class TestComputeStatsProperty:
    """Property 1: 时序统计计算正确性属性测试
    
    Feature: metric-compare, Property 1: 时序统计计算正确性
    **Validates: Requirements 2.1**
    
    *For any* 非空时序数据（包含 values 和 timestamps 列表），`compute_stats` 函数计算的统计值应满足：
    - `max` 等于 values 中的最大值
    - `min` 等于 values 中的最小值
    - `avg` 等于 values 的算术平均值
    - `count` 等于 values 的长度
    - `max_time` 对应 max 值出现的时间戳
    - `min_time` 对应 min 值出现的时间戳
    """

    @settings(max_examples=100)
    @given(data=non_empty_time_series_strategy())
    def test_property_1_1_max_equals_maximum_value(self, data):
        """Property 1.1: max 等于 values 中的最大值
        
        Feature: metric-compare, Property 1: 时序统计计算正确性
        **Validates: Requirements 2.1**
        """
        values, timestamps = data
        
        stats = compute_stats(values, timestamps)
        
        expected_max = max(values)
        assert stats.max == expected_max, \
            f"Expected max={expected_max}, got {stats.max}"

    @settings(max_examples=100)
    @given(data=non_empty_time_series_strategy())
    def test_property_1_2_min_equals_minimum_value(self, data):
        """Property 1.2: min 等于 values 中的最小值
        
        Feature: metric-compare, Property 1: 时序统计计算正确性
        **Validates: Requirements 2.1**
        """
        values, timestamps = data
        
        stats = compute_stats(values, timestamps)
        
        expected_min = min(values)
        assert stats.min == expected_min, \
            f"Expected min={expected_min}, got {stats.min}"

    @settings(max_examples=100)
    @given(data=non_empty_time_series_strategy())
    def test_property_1_3_avg_equals_arithmetic_mean(self, data):
        """Property 1.3: avg 等于 values 的算术平均值
        
        Feature: metric-compare, Property 1: 时序统计计算正确性
        **Validates: Requirements 2.1**
        """
        values, timestamps = data
        
        stats = compute_stats(values, timestamps)
        
        expected_avg = sum(values) / len(values)
        # 使用 pytest.approx 处理浮点数精度问题
        assert stats.avg == pytest.approx(expected_avg, rel=1e-9), \
            f"Expected avg={expected_avg}, got {stats.avg}"

    @settings(max_examples=100)
    @given(data=non_empty_time_series_strategy())
    def test_property_1_4_count_equals_values_length(self, data):
        """Property 1.4: count 等于 values 的长度
        
        Feature: metric-compare, Property 1: 时序统计计算正确性
        **Validates: Requirements 2.1**
        """
        values, timestamps = data
        
        stats = compute_stats(values, timestamps)
        
        expected_count = len(values)
        assert stats.count == expected_count, \
            f"Expected count={expected_count}, got {stats.count}"

    @settings(max_examples=100)
    @given(data=non_empty_time_series_strategy())
    def test_property_1_5_max_time_corresponds_to_max_value_timestamp(self, data):
        """Property 1.5: max_time 对应 max 值出现的时间戳
        
        Feature: metric-compare, Property 1: 时序统计计算正确性
        **Validates: Requirements 2.1**
        """
        values, timestamps = data
        
        stats = compute_stats(values, timestamps)
        
        # 找到 max 值的索引
        max_val = max(values)
        max_idx = values.index(max_val)
        
        # 获取对应的时间戳并格式化
        expected_max_time = _format_timestamp_ns(timestamps[max_idx])
        
        assert stats.max_time == expected_max_time, \
            f"Expected max_time={expected_max_time}, got {stats.max_time}"

    @settings(max_examples=100)
    @given(data=non_empty_time_series_strategy())
    def test_property_1_6_min_time_corresponds_to_min_value_timestamp(self, data):
        """Property 1.6: min_time 对应 min 值出现的时间戳
        
        Feature: metric-compare, Property 1: 时序统计计算正确性
        **Validates: Requirements 2.1**
        """
        values, timestamps = data
        
        stats = compute_stats(values, timestamps)
        
        # 找到 min 值的索引
        min_val = min(values)
        min_idx = values.index(min_val)
        
        # 获取对应的时间戳并格式化
        expected_min_time = _format_timestamp_ns(timestamps[min_idx])
        
        assert stats.min_time == expected_min_time, \
            f"Expected min_time={expected_min_time}, got {stats.min_time}"

    @settings(max_examples=100)
    @given(data=non_empty_time_series_strategy())
    def test_property_1_all_stats_correct(self, data):
        """Property 1 综合测试: 所有统计值应同时满足正确性要求
        
        Feature: metric-compare, Property 1: 时序统计计算正确性
        **Validates: Requirements 2.1**
        
        验证 compute_stats 函数对于任意非空时序数据，计算的所有统计值都正确。
        """
        values, timestamps = data
        
        stats = compute_stats(values, timestamps)
        
        # 计算期望值
        expected_max = max(values)
        expected_min = min(values)
        expected_avg = sum(values) / len(values)
        expected_count = len(values)
        
        max_idx = values.index(expected_max)
        min_idx = values.index(expected_min)
        expected_max_time = _format_timestamp_ns(timestamps[max_idx])
        expected_min_time = _format_timestamp_ns(timestamps[min_idx])
        
        # 验证所有统计值
        assert stats.max == expected_max, \
            f"max: expected {expected_max}, got {stats.max}"
        assert stats.min == expected_min, \
            f"min: expected {expected_min}, got {stats.min}"
        assert stats.avg == pytest.approx(expected_avg, rel=1e-9), \
            f"avg: expected {expected_avg}, got {stats.avg}"
        assert stats.count == expected_count, \
            f"count: expected {expected_count}, got {stats.count}"
        assert stats.max_time == expected_max_time, \
            f"max_time: expected {expected_max_time}, got {stats.max_time}"
        assert stats.min_time == expected_min_time, \
            f"min_time: expected {expected_min_time}, got {stats.min_time}"


class TestComputeStatsEdgeCases:
    """compute_stats 边界情况测试
    
    Feature: metric-compare, Property 1: 时序统计计算正确性
    **Validates: Requirements 2.1, 2.2**
    """

    def test_empty_values_returns_zero_stats(self):
        """测试空数据集返回零值
        
        Feature: metric-compare, Property 1: 时序统计计算正确性
        **Validates: Requirements 2.2**
        """
        stats = compute_stats([], [])
        
        assert stats.max == 0.0
        assert stats.min == 0.0
        assert stats.avg == 0.0
        assert stats.count == 0
        assert stats.max_time == ""
        assert stats.min_time == ""

    def test_single_value(self):
        """测试单个数据点
        
        Feature: metric-compare, Property 1: 时序统计计算正确性
        **Validates: Requirements 2.1**
        """
        values = [42.5]
        timestamps = [1704700800_000_000_000]  # 2024-01-08 10:00:00 UTC
        
        stats = compute_stats(values, timestamps)
        
        assert stats.max == 42.5
        assert stats.min == 42.5
        assert stats.avg == 42.5
        assert stats.count == 1
        # max 和 min 应该是同一个时间点
        assert stats.max_time == stats.min_time

    def test_all_same_values(self):
        """测试所有值相同的情况
        
        Feature: metric-compare, Property 1: 时序统计计算正确性
        **Validates: Requirements 2.1**
        """
        values = [100.0, 100.0, 100.0, 100.0]
        timestamps = [
            1704700800_000_000_000,
            1704700830_000_000_000,
            1704700860_000_000_000,
            1704700890_000_000_000
        ]
        
        stats = compute_stats(values, timestamps)
        
        assert stats.max == 100.0
        assert stats.min == 100.0
        assert stats.avg == 100.0
        assert stats.count == 4
        # 当所有值相同时，max_time 和 min_time 应该是第一个出现的时间
        expected_time = _format_timestamp_ns(timestamps[0])
        assert stats.max_time == expected_time
        assert stats.min_time == expected_time

    def test_negative_values(self):
        """测试包含负值的情况
        
        Feature: metric-compare, Property 1: 时序统计计算正确性
        **Validates: Requirements 2.1**
        """
        values = [-10.0, -5.0, 0.0, 5.0, 10.0]
        timestamps = [
            1704700800_000_000_000,
            1704700830_000_000_000,
            1704700860_000_000_000,
            1704700890_000_000_000,
            1704700920_000_000_000
        ]
        
        stats = compute_stats(values, timestamps)
        
        assert stats.max == 10.0
        assert stats.min == -10.0
        assert stats.avg == pytest.approx(0.0)
        assert stats.count == 5
        # max 在最后一个位置
        assert stats.max_time == _format_timestamp_ns(timestamps[4])
        # min 在第一个位置
        assert stats.min_time == _format_timestamp_ns(timestamps[0])

    def test_values_without_timestamps(self):
        """测试有值但无时间戳的情况
        
        Feature: metric-compare, Property 1: 时序统计计算正确性
        **Validates: Requirements 2.1**
        """
        values = [1.0, 2.0, 3.0]
        timestamps = []  # 空时间戳列表
        
        stats = compute_stats(values, timestamps)
        
        # 统计值应该正确计算
        assert stats.max == 3.0
        assert stats.min == 1.0
        assert stats.avg == pytest.approx(2.0)
        assert stats.count == 3
        # 时间应该为空
        assert stats.max_time == ""
        assert stats.min_time == ""

    def test_large_dataset(self):
        """测试大数据集
        
        Feature: metric-compare, Property 1: 时序统计计算正确性
        **Validates: Requirements 2.1**
        """
        # 生成 1000 个数据点
        import random
        random.seed(42)  # 固定种子以保证可重复性
        
        values = [random.uniform(-1000, 1000) for _ in range(1000)]
        base_ts = 1704700800_000_000_000
        timestamps = [base_ts + i * 30_000_000_000 for i in range(1000)]
        
        stats = compute_stats(values, timestamps)
        
        # 验证统计值
        assert stats.max == max(values)
        assert stats.min == min(values)
        assert stats.avg == pytest.approx(sum(values) / len(values))
        assert stats.count == 1000
        
        # 验证时间
        max_idx = values.index(max(values))
        min_idx = values.index(min(values))
        assert stats.max_time == _format_timestamp_ns(timestamps[max_idx])
        assert stats.min_time == _format_timestamp_ns(timestamps[min_idx])


# ============================================================================
# Strategies for Property 2: 偏移量解析正确性
# ============================================================================

# 定义单位到秒的映射（与实现保持一致）
UNIT_TO_SECONDS = {
    's': 1,           # 秒
    'm': 60,          # 分钟
    'h': 3600,        # 小时
    'd': 86400,       # 天
    'w': 604800,      # 周
}

# 生成有效的偏移量字符串（格式为 Xu，其中 X 为正整数，u 为 s/m/h/d/w）
@st.composite
def valid_duration_strategy(draw):
    """生成有效的偏移量字符串
    
    生成格式为 "数字+单位" 的有效偏移量字符串，如 "30m", "1h", "1d", "1w"
    
    Returns:
        tuple: (duration_string, expected_seconds)
    """
    # 生成正整数（1 到 1000）
    value = draw(st.integers(min_value=1, max_value=1000))
    
    # 随机选择单位
    unit = draw(st.sampled_from(['s', 'm', 'h', 'd', 'w']))
    
    # 构建偏移量字符串
    duration_string = f"{value}{unit}"
    
    # 计算期望的秒数
    expected_seconds = value * UNIT_TO_SECONDS[unit]
    
    return duration_string, expected_seconds


# 生成带有大写单位的有效偏移量字符串（测试大小写不敏感）
@st.composite
def valid_duration_with_case_strategy(draw):
    """生成带有大写或小写单位的有效偏移量字符串
    
    测试单位大小写不敏感的特性
    
    Returns:
        tuple: (duration_string, expected_seconds)
    """
    # 生成正整数（1 到 1000）
    value = draw(st.integers(min_value=1, max_value=1000))
    
    # 随机选择单位（大写或小写）
    unit_lower = draw(st.sampled_from(['s', 'm', 'h', 'd', 'w']))
    use_upper = draw(st.booleans())
    unit = unit_lower.upper() if use_upper else unit_lower
    
    # 构建偏移量字符串
    duration_string = f"{value}{unit}"
    
    # 计算期望的秒数
    expected_seconds = value * UNIT_TO_SECONDS[unit_lower]
    
    return duration_string, expected_seconds


# 生成带有空白的有效偏移量字符串（测试空白处理）
@st.composite
def valid_duration_with_whitespace_strategy(draw):
    """生成带有前后空白的有效偏移量字符串
    
    测试空白字符的处理
    
    Returns:
        tuple: (duration_string, expected_seconds)
    """
    # 生成正整数（1 到 1000）
    value = draw(st.integers(min_value=1, max_value=1000))
    
    # 随机选择单位
    unit = draw(st.sampled_from(['s', 'm', 'h', 'd', 'w']))
    
    # 生成前后空白
    leading_spaces = draw(st.text(alphabet=' \t', min_size=0, max_size=3))
    trailing_spaces = draw(st.text(alphabet=' \t', min_size=0, max_size=3))
    
    # 构建偏移量字符串
    duration_string = f"{leading_spaces}{value}{unit}{trailing_spaces}"
    
    # 计算期望的秒数
    expected_seconds = value * UNIT_TO_SECONDS[unit]
    
    return duration_string, expected_seconds


# 生成无效的偏移量字符串
invalid_duration_strategy = st.one_of(
    # 空字符串
    st.just(""),
    # 只有空白
    st.text(alphabet=' \t', min_size=1, max_size=5),
    # 只有数字，没有单位
    st.builds(lambda n: str(n), st.integers(min_value=1, max_value=1000)),
    # 只有单位，没有数字
    st.sampled_from(['s', 'm', 'h', 'd', 'w', 'S', 'M', 'H', 'D', 'W']),
    # 无效单位
    st.builds(lambda n, u: f"{n}{u}", 
              st.integers(min_value=1, max_value=100),
              st.sampled_from(['x', 'y', 'z', 'a', 'b', 'ms', 'ns', 'sec', 'min', 'hour', 'day', 'week'])),
    # 负数
    st.builds(lambda n, u: f"-{n}{u}",
              st.integers(min_value=1, max_value=100),
              st.sampled_from(['s', 'm', 'h', 'd', 'w'])),
    # 浮点数
    st.builds(lambda n, u: f"{n:.1f}{u}",
              st.floats(min_value=0.1, max_value=100, allow_nan=False, allow_infinity=False),
              st.sampled_from(['s', 'm', 'h', 'd', 'w'])),
    # 随机无效字符串
    st.text(alphabet='abcdefghijklmnopqrstuvwxyz!@#$%^&*()', min_size=1, max_size=10),
)


# ============================================================================
# Property Tests - Property 2: 偏移量解析正确性
# ============================================================================

class TestParseDurationToSecondsProperty:
    """Property 2: 偏移量解析正确性属性测试
    
    Feature: metric-compare, Property 2: 偏移量解析正确性
    **Validates: Requirements 1.3**
    
    *For any* 有效的偏移量字符串（格式为 `Xu`，其中 X 为正整数，u 为 s/m/h/d/w），
    `parse_duration_to_seconds` 函数应返回正确的秒数：
    - `30m` → 1800
    - `1h` → 3600
    - `1d` → 86400
    - `1w` → 604800
    """

    @settings(max_examples=100)
    @given(data=valid_duration_strategy())
    def test_property_2_1_valid_duration_returns_correct_seconds(self, data):
        """Property 2.1: 有效的偏移量字符串应返回正确的秒数
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        """
        duration_string, expected_seconds = data
        
        result = parse_duration_to_seconds(duration_string)
        
        assert result == expected_seconds, \
            f"Expected {expected_seconds} for '{duration_string}', got {result}"

    @settings(max_examples=100)
    @given(data=valid_duration_strategy())
    def test_property_2_2_result_is_positive_for_valid_input(self, data):
        """Property 2.2: 有效输入应返回正整数
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        """
        duration_string, _ = data
        
        result = parse_duration_to_seconds(duration_string)
        
        assert result > 0, \
            f"Expected positive result for '{duration_string}', got {result}"
        assert isinstance(result, int), \
            f"Expected int result for '{duration_string}', got {type(result)}"

    @settings(max_examples=100)
    @given(data=valid_duration_with_case_strategy())
    def test_property_2_3_case_insensitive_unit(self, data):
        """Property 2.3: 单位应不区分大小写
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        """
        duration_string, expected_seconds = data
        
        result = parse_duration_to_seconds(duration_string)
        
        assert result == expected_seconds, \
            f"Expected {expected_seconds} for '{duration_string}' (case insensitive), got {result}"

    @settings(max_examples=100)
    @given(data=valid_duration_with_whitespace_strategy())
    def test_property_2_4_handles_whitespace(self, data):
        """Property 2.4: 应正确处理前后空白
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        """
        duration_string, expected_seconds = data
        
        result = parse_duration_to_seconds(duration_string)
        
        assert result == expected_seconds, \
            f"Expected {expected_seconds} for '{duration_string}' (with whitespace), got {result}"

    @settings(max_examples=100)
    @given(invalid_duration=invalid_duration_strategy)
    def test_property_2_5_invalid_duration_returns_zero(self, invalid_duration):
        """Property 2.5: 无效的偏移量字符串应返回 0
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        """
        result = parse_duration_to_seconds(invalid_duration)
        
        assert result == 0, \
            f"Expected 0 for invalid duration '{invalid_duration}', got {result}"

    @settings(max_examples=100)
    @given(value=st.integers(min_value=1, max_value=1000))
    def test_property_2_6_seconds_unit_returns_same_value(self, value):
        """Property 2.6: 秒单位应返回相同的数值
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        
        验证 Xs 格式返回 X 秒
        """
        duration_string = f"{value}s"
        
        result = parse_duration_to_seconds(duration_string)
        
        assert result == value, \
            f"Expected {value} for '{duration_string}', got {result}"

    @settings(max_examples=100)
    @given(value=st.integers(min_value=1, max_value=1000))
    def test_property_2_7_minutes_unit_returns_value_times_60(self, value):
        """Property 2.7: 分钟单位应返回数值乘以 60
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        
        验证 Xm 格式返回 X * 60 秒
        """
        duration_string = f"{value}m"
        expected = value * 60
        
        result = parse_duration_to_seconds(duration_string)
        
        assert result == expected, \
            f"Expected {expected} for '{duration_string}', got {result}"

    @settings(max_examples=100)
    @given(value=st.integers(min_value=1, max_value=1000))
    def test_property_2_8_hours_unit_returns_value_times_3600(self, value):
        """Property 2.8: 小时单位应返回数值乘以 3600
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        
        验证 Xh 格式返回 X * 3600 秒
        """
        duration_string = f"{value}h"
        expected = value * 3600
        
        result = parse_duration_to_seconds(duration_string)
        
        assert result == expected, \
            f"Expected {expected} for '{duration_string}', got {result}"

    @settings(max_examples=100)
    @given(value=st.integers(min_value=1, max_value=1000))
    def test_property_2_9_days_unit_returns_value_times_86400(self, value):
        """Property 2.9: 天单位应返回数值乘以 86400
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        
        验证 Xd 格式返回 X * 86400 秒
        """
        duration_string = f"{value}d"
        expected = value * 86400
        
        result = parse_duration_to_seconds(duration_string)
        
        assert result == expected, \
            f"Expected {expected} for '{duration_string}', got {result}"

    @settings(max_examples=100)
    @given(value=st.integers(min_value=1, max_value=1000))
    def test_property_2_10_weeks_unit_returns_value_times_604800(self, value):
        """Property 2.10: 周单位应返回数值乘以 604800
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        
        验证 Xw 格式返回 X * 604800 秒
        """
        duration_string = f"{value}w"
        expected = value * 604800
        
        result = parse_duration_to_seconds(duration_string)
        
        assert result == expected, \
            f"Expected {expected} for '{duration_string}', got {result}"


class TestParseDurationToSecondsEdgeCases:
    """parse_duration_to_seconds 边界情况测试
    
    Feature: metric-compare, Property 2: 偏移量解析正确性
    **Validates: Requirements 1.3**
    """

    def test_specific_examples_from_design(self):
        """测试设计文档中的具体示例
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        
        验证设计文档中列出的具体示例：
        - 30m → 1800
        - 1h → 3600
        - 1d → 86400
        - 1w → 604800
        """
        assert parse_duration_to_seconds("30m") == 1800, \
            "30m should equal 1800 seconds"
        assert parse_duration_to_seconds("1h") == 3600, \
            "1h should equal 3600 seconds"
        assert parse_duration_to_seconds("1d") == 86400, \
            "1d should equal 86400 seconds"
        assert parse_duration_to_seconds("1w") == 604800, \
            "1w should equal 604800 seconds"

    def test_empty_string_returns_zero(self):
        """测试空字符串返回 0
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        """
        assert parse_duration_to_seconds("") == 0

    def test_none_returns_zero(self):
        """测试 None 返回 0
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        """
        assert parse_duration_to_seconds(None) == 0

    def test_zero_value_returns_zero(self):
        """测试数值为 0 时返回 0
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        """
        assert parse_duration_to_seconds("0s") == 0
        assert parse_duration_to_seconds("0m") == 0
        assert parse_duration_to_seconds("0h") == 0
        assert parse_duration_to_seconds("0d") == 0
        assert parse_duration_to_seconds("0w") == 0

    def test_uppercase_units(self):
        """测试大写单位
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        """
        assert parse_duration_to_seconds("30S") == 30
        assert parse_duration_to_seconds("30M") == 1800
        assert parse_duration_to_seconds("1H") == 3600
        assert parse_duration_to_seconds("1D") == 86400
        assert parse_duration_to_seconds("1W") == 604800

    def test_whitespace_handling(self):
        """测试空白字符处理
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        """
        assert parse_duration_to_seconds("  30m  ") == 1800
        assert parse_duration_to_seconds("\t1h\t") == 3600
        assert parse_duration_to_seconds(" 1d ") == 86400

    def test_invalid_unit_returns_zero(self):
        """测试无效单位返回 0
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        """
        assert parse_duration_to_seconds("30x") == 0
        assert parse_duration_to_seconds("1y") == 0
        assert parse_duration_to_seconds("1ms") == 0
        assert parse_duration_to_seconds("1sec") == 0

    def test_invalid_format_returns_zero(self):
        """测试无效格式返回 0
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        """
        assert parse_duration_to_seconds("invalid") == 0
        assert parse_duration_to_seconds("abc") == 0
        assert parse_duration_to_seconds("m30") == 0  # 单位在前
        assert parse_duration_to_seconds("30") == 0   # 没有单位
        assert parse_duration_to_seconds("m") == 0    # 没有数字

    def test_negative_value_returns_zero(self):
        """测试负数返回 0
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        """
        assert parse_duration_to_seconds("-30m") == 0
        assert parse_duration_to_seconds("-1h") == 0

    def test_float_value_returns_zero(self):
        """测试浮点数返回 0
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        """
        assert parse_duration_to_seconds("1.5h") == 0
        assert parse_duration_to_seconds("30.5m") == 0

    def test_large_values(self):
        """测试大数值
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        """
        # 1000 小时
        assert parse_duration_to_seconds("1000h") == 1000 * 3600
        # 365 天
        assert parse_duration_to_seconds("365d") == 365 * 86400
        # 52 周
        assert parse_duration_to_seconds("52w") == 52 * 604800

    def test_common_use_cases(self):
        """测试常见使用场景
        
        Feature: metric-compare, Property 2: 偏移量解析正确性
        **Validates: Requirements 1.3**
        """
        # 常见的对比偏移量
        assert parse_duration_to_seconds("30m") == 1800      # 30 分钟
        assert parse_duration_to_seconds("1h") == 3600       # 1 小时
        assert parse_duration_to_seconds("2h") == 7200       # 2 小时
        assert parse_duration_to_seconds("6h") == 21600      # 6 小时
        assert parse_duration_to_seconds("12h") == 43200     # 12 小时
        assert parse_duration_to_seconds("1d") == 86400      # 1 天
        assert parse_duration_to_seconds("7d") == 604800     # 7 天
        assert parse_duration_to_seconds("1w") == 604800     # 1 周


# ============================================================================
# Property Tests - Property 6: 输出结构完整性
# ============================================================================

class TestOutputStructureCompleteness:
    """Property 6: 输出结构完整性属性测试
    
    Feature: metric-compare, Property 6: 输出结构完整性
    **Validates: Requirements 6.1, 6.2, 7.1**
    """

    @settings(max_examples=100)
    @given(output=compare_output_strategy)
    def test_property_6_1_compare_output_has_all_top_level_fields(
        self, output: CompareOutput
    ):
        """Property 6.1: CompareOutput 应包含所有顶层必要字段
        
        Feature: metric-compare, Property 6: 输出结构完整性
        **Validates: Requirements 6.1, 6.2, 7.1**
        
        验证：CompareOutput 数据类应包含以下顶层字段：
        - compare_enabled
        - current_time_range
        - compare_time_range
        - offset
        - total_series
        - results
        """
        # 获取所有字段名
        field_names = {f.name for f in fields(output)}
        
        # 验证必要的顶层字段存在
        required_top_level_fields = {
            'compare_enabled',
            'current_time_range',
            'compare_time_range',
            'offset',
            'total_series',
            'results'
        }
        
        assert required_top_level_fields.issubset(field_names), \
            f"Missing top-level fields: {required_top_level_fields - field_names}"
        
        # 验证字段类型
        assert isinstance(output.compare_enabled, bool), \
            f"compare_enabled should be bool, got {type(output.compare_enabled)}"
        assert isinstance(output.current_time_range, TimeRangeInfo), \
            f"current_time_range should be TimeRangeInfo, got {type(output.current_time_range)}"
        assert output.compare_time_range is None or isinstance(output.compare_time_range, TimeRangeInfo), \
            f"compare_time_range should be None or TimeRangeInfo, got {type(output.compare_time_range)}"
        assert isinstance(output.offset, str), \
            f"offset should be str, got {type(output.offset)}"
        assert isinstance(output.total_series, int), \
            f"total_series should be int, got {type(output.total_series)}"
        assert isinstance(output.results, list), \
            f"results should be list, got {type(output.results)}"

    @settings(max_examples=100)
    @given(result=time_series_compare_result_strategy)
    def test_property_6_2_compare_result_has_all_required_fields(
        self, result: TimeSeriesCompareResult
    ):
        """Property 6.2: TimeSeriesCompareResult 应包含所有必要字段
        
        Feature: metric-compare, Property 6: 输出结构完整性
        **Validates: Requirements 6.1, 6.2, 7.1**
        
        验证：每个结果项应包含以下字段：
        - key
        - current_stats
        - compare_stats
        - diff_score
        - diff_details
        """
        # 获取所有字段名
        field_names = {f.name for f in fields(result)}
        
        # 验证必要的字段存在
        required_result_fields = {
            'key',
            'current_stats',
            'compare_stats',
            'diff_score',
            'diff_details'
        }
        
        assert required_result_fields.issubset(field_names), \
            f"Missing result fields: {required_result_fields - field_names}"
        
        # 验证字段类型
        assert isinstance(result.key, TimeSeriesKey), \
            f"key should be TimeSeriesKey, got {type(result.key)}"
        assert isinstance(result.current_stats, TimeSeriesStats), \
            f"current_stats should be TimeSeriesStats, got {type(result.current_stats)}"
        assert result.compare_stats is None or isinstance(result.compare_stats, TimeSeriesStats), \
            f"compare_stats should be None or TimeSeriesStats, got {type(result.compare_stats)}"
        assert isinstance(result.diff_score, float), \
            f"diff_score should be float, got {type(result.diff_score)}"
        assert isinstance(result.diff_details, DiffDetails), \
            f"diff_details should be DiffDetails, got {type(result.diff_details)}"

    @settings(max_examples=100)
    @given(time_range=time_range_info_strategy)
    def test_property_6_3_time_range_info_has_all_required_fields(
        self, time_range: TimeRangeInfo
    ):
        """Property 6.3: TimeRangeInfo 应包含所有必要字段
        
        Feature: metric-compare, Property 6: 输出结构完整性
        **Validates: Requirements 6.1, 6.2, 7.1**
        
        验证：时间范围信息应包含以下字段：
        - from (人类可读格式)
        - to (人类可读格式)
        - from_unix (Unix 时间戳)
        - to_unix (Unix 时间戳)
        """
        # 获取所有字段名
        field_names = {f.name for f in fields(time_range)}
        
        # 验证必要的字段存在（注意：设计文档中使用 from/to，但 Python 中使用 from_time/to_time 避免关键字冲突）
        required_time_range_fields = {
            'from_time',  # 对应设计文档中的 from
            'to_time',    # 对应设计文档中的 to
            'from_unix',
            'to_unix'
        }
        
        assert required_time_range_fields.issubset(field_names), \
            f"Missing time range fields: {required_time_range_fields - field_names}"
        
        # 验证字段类型
        assert isinstance(time_range.from_time, str), \
            f"from_time should be str, got {type(time_range.from_time)}"
        assert isinstance(time_range.to_time, str), \
            f"to_time should be str, got {type(time_range.to_time)}"
        assert isinstance(time_range.from_unix, int), \
            f"from_unix should be int, got {type(time_range.from_unix)}"
        assert isinstance(time_range.to_unix, int), \
            f"to_unix should be int, got {type(time_range.to_unix)}"

    @settings(max_examples=100)
    @given(key=time_series_key_strategy)
    def test_property_6_4_time_series_key_has_all_required_fields(
        self, key: TimeSeriesKey
    ):
        """Property 6.4: TimeSeriesKey 应包含所有必要字段
        
        Feature: metric-compare, Property 6: 输出结构完整性
        **Validates: Requirements 6.1, 6.2, 7.1**
        
        验证：时序标识应包含以下字段：
        - entity_id
        - labels
        - metric
        - metric_set_id
        """
        # 获取所有字段名
        field_names = {f.name for f in fields(key)}
        
        # 验证必要的字段存在
        required_key_fields = {
            'entity_id',
            'labels',
            'metric',
            'metric_set_id'
        }
        
        assert required_key_fields.issubset(field_names), \
            f"Missing key fields: {required_key_fields - field_names}"
        
        # 验证字段类型
        assert isinstance(key.entity_id, str), \
            f"entity_id should be str, got {type(key.entity_id)}"
        assert isinstance(key.labels, str), \
            f"labels should be str, got {type(key.labels)}"
        assert isinstance(key.metric, str), \
            f"metric should be str, got {type(key.metric)}"
        assert isinstance(key.metric_set_id, str), \
            f"metric_set_id should be str, got {type(key.metric_set_id)}"

    @settings(max_examples=100)
    @given(stats=time_series_stats_strategy)
    def test_property_6_5_time_series_stats_has_all_required_fields(
        self, stats: TimeSeriesStats
    ):
        """Property 6.5: TimeSeriesStats 应包含所有必要字段
        
        Feature: metric-compare, Property 6: 输出结构完整性
        **Validates: Requirements 6.1, 6.2, 7.1**
        
        验证：统计值应包含以下字段：
        - max
        - min
        - avg
        - count
        - max_time
        - min_time
        """
        # 获取所有字段名
        field_names = {f.name for f in fields(stats)}
        
        # 验证必要的字段存在
        required_stats_fields = {
            'max',
            'min',
            'avg',
            'count',
            'max_time',
            'min_time'
        }
        
        assert required_stats_fields.issubset(field_names), \
            f"Missing stats fields: {required_stats_fields - field_names}"
        
        # 验证字段类型
        assert isinstance(stats.max, float), \
            f"max should be float, got {type(stats.max)}"
        assert isinstance(stats.min, float), \
            f"min should be float, got {type(stats.min)}"
        assert isinstance(stats.avg, float), \
            f"avg should be float, got {type(stats.avg)}"
        assert isinstance(stats.count, int), \
            f"count should be int, got {type(stats.count)}"
        assert isinstance(stats.max_time, str), \
            f"max_time should be str, got {type(stats.max_time)}"
        assert isinstance(stats.min_time, str), \
            f"min_time should be str, got {type(stats.min_time)}"

    @settings(max_examples=100)
    @given(diff_details=diff_details_strategy)
    def test_property_6_6_diff_details_has_all_required_fields(
        self, diff_details: DiffDetails
    ):
        """Property 6.6: DiffDetails 应包含所有必要字段
        
        Feature: metric-compare, Property 6: 输出结构完整性
        **Validates: Requirements 6.1, 6.2, 7.1**
        
        验证：差异详情应包含以下字段：
        - trend
        - avg_change
        - avg_change_percent
        - max_change
        - min_change
        """
        # 获取所有字段名
        field_names = {f.name for f in fields(diff_details)}
        
        # 验证必要的字段存在
        required_diff_fields = {
            'trend',
            'avg_change',
            'avg_change_percent',
            'max_change',
            'min_change'
        }
        
        assert required_diff_fields.issubset(field_names), \
            f"Missing diff_details fields: {required_diff_fields - field_names}"
        
        # 验证字段类型
        assert isinstance(diff_details.trend, str), \
            f"trend should be str, got {type(diff_details.trend)}"
        assert isinstance(diff_details.avg_change, float), \
            f"avg_change should be float, got {type(diff_details.avg_change)}"
        assert isinstance(diff_details.avg_change_percent, float), \
            f"avg_change_percent should be float, got {type(diff_details.avg_change_percent)}"
        assert isinstance(diff_details.max_change, float), \
            f"max_change should be float, got {type(diff_details.max_change)}"
        assert isinstance(diff_details.min_change, float), \
            f"min_change should be float, got {type(diff_details.min_change)}"

    @settings(max_examples=100)
    @given(output=compare_output_strategy)
    def test_property_6_7_compare_output_serializable_to_dict(
        self, output: CompareOutput
    ):
        """Property 6.7: CompareOutput 应能序列化为字典
        
        Feature: metric-compare, Property 6: 输出结构完整性
        **Validates: Requirements 6.1, 6.2, 7.1**
        
        验证：CompareOutput 应能通过 asdict 转换为字典，
        且字典包含所有必要的顶层键。
        """
        # 转换为字典
        output_dict = asdict(output)
        
        # 验证顶层键存在
        required_keys = {
            'compare_enabled',
            'current_time_range',
            'compare_time_range',
            'offset',
            'total_series',
            'results'
        }
        
        assert required_keys.issubset(output_dict.keys()), \
            f"Missing keys in dict: {required_keys - output_dict.keys()}"
        
        # 验证 current_time_range 字典结构
        current_tr = output_dict['current_time_range']
        assert 'from_time' in current_tr, "current_time_range missing 'from_time'"
        assert 'to_time' in current_tr, "current_time_range missing 'to_time'"
        assert 'from_unix' in current_tr, "current_time_range missing 'from_unix'"
        assert 'to_unix' in current_tr, "current_time_range missing 'to_unix'"
        
        # 验证 results 中每个项的结构
        for i, result in enumerate(output_dict['results']):
            assert 'key' in result, f"results[{i}] missing 'key'"
            assert 'current_stats' in result, f"results[{i}] missing 'current_stats'"
            assert 'compare_stats' in result, f"results[{i}] missing 'compare_stats'"
            assert 'diff_score' in result, f"results[{i}] missing 'diff_score'"
            assert 'diff_details' in result, f"results[{i}] missing 'diff_details'"

    @settings(max_examples=100)
    @given(key=time_series_key_strategy)
    def test_property_6_8_time_series_key_hash_returns_string(
        self, key: TimeSeriesKey
    ):
        """Property 6.8: TimeSeriesKey.hash() 应返回字符串
        
        Feature: metric-compare, Property 6: 输出结构完整性
        **Validates: Requirements 6.1, 6.2, 7.1**
        
        验证：TimeSeriesKey 的 hash() 方法应返回非空字符串，
        用于时序数据匹配。
        """
        hash_value = key.hash()
        
        assert isinstance(hash_value, str), \
            f"hash() should return str, got {type(hash_value)}"
        assert len(hash_value) > 0, \
            "hash() should return non-empty string"
        
        # 验证 hash 包含所有关键字段
        assert key.metric in hash_value, \
            f"hash should contain metric '{key.metric}'"
        assert key.metric_set_id in hash_value, \
            f"hash should contain metric_set_id '{key.metric_set_id}'"
        assert key.entity_id in hash_value, \
            f"hash should contain entity_id '{key.entity_id}'"
        assert key.labels in hash_value, \
            f"hash should contain labels '{key.labels}'"


# ============================================================================
# 边界情况测试
# ============================================================================

class TestOutputStructureEdgeCases:
    """输出结构边界情况测试
    
    Feature: metric-compare, Property 6: 输出结构完整性
    **Validates: Requirements 6.1, 6.2, 7.1**
    """

    def test_empty_compare_output(self):
        """测试空的 CompareOutput 结构
        
        Feature: metric-compare, Property 6: 输出结构完整性
        **Validates: Requirements 6.1, 6.2, 7.1**
        """
        output = CompareOutput()
        
        # 验证默认值
        assert output.compare_enabled is False
        assert isinstance(output.current_time_range, TimeRangeInfo)
        assert output.compare_time_range is None
        assert output.offset == ""
        assert output.total_series == 0
        assert output.results == []

    def test_compare_output_with_empty_results(self):
        """测试结果为空的 CompareOutput
        
        Feature: metric-compare, Property 6: 输出结构完整性
        **Validates: Requirements 6.1, 6.2, 7.1**
        """
        output = CompareOutput(
            compare_enabled=True,
            current_time_range=TimeRangeInfo(
                from_time="2024-01-08 10:00:00",
                to_time="2024-01-08 11:00:00",
                from_unix=1704700800,
                to_unix=1704704400
            ),
            compare_time_range=TimeRangeInfo(
                from_time="2024-01-08 09:00:00",
                to_time="2024-01-08 10:00:00",
                from_unix=1704697200,
                to_unix=1704700800
            ),
            offset="3600s",
            total_series=0,
            results=[]
        )
        
        # 验证结构完整性
        output_dict = asdict(output)
        assert 'compare_enabled' in output_dict
        assert 'current_time_range' in output_dict
        assert 'compare_time_range' in output_dict
        assert 'offset' in output_dict
        assert 'total_series' in output_dict
        assert 'results' in output_dict
        assert output_dict['results'] == []

    def test_compare_result_with_null_compare_stats(self):
        """测试 compare_stats 为 None 的结果项（新增时序）
        
        Feature: metric-compare, Property 6: 输出结构完整性
        **Validates: Requirements 6.1, 6.2, 7.1**
        """
        result = TimeSeriesCompareResult(
            key=TimeSeriesKey(
                entity_id="test_entity",
                labels="{}",
                metric="request_count",
                metric_set_id="apm@metric_set"
            ),
            current_stats=TimeSeriesStats(
                max=100.0,
                min=50.0,
                avg=75.0,
                count=10,
                max_time="2024-01-08 10:30:00",
                min_time="2024-01-08 10:15:00"
            ),
            compare_stats=None,  # 新增时序，无对比数据
            diff_score=1.0,
            diff_details=DiffDetails(trend=Trend.NEW)
        )
        
        # 验证结构
        result_dict = asdict(result)
        assert result_dict['compare_stats'] is None
        assert result_dict['diff_details']['trend'] == Trend.NEW

    def test_time_series_key_hash_uniqueness(self):
        """测试 TimeSeriesKey.hash() 的唯一性
        
        Feature: metric-compare, Property 6: 输出结构完整性
        **Validates: Requirements 6.1, 6.2, 7.1**
        """
        key1 = TimeSeriesKey(
            entity_id="entity1",
            labels='{"service": "svc1"}',
            metric="request_count",
            metric_set_id="apm@metric_set"
        )
        
        key2 = TimeSeriesKey(
            entity_id="entity2",  # 不同的 entity_id
            labels='{"service": "svc1"}',
            metric="request_count",
            metric_set_id="apm@metric_set"
        )
        
        key3 = TimeSeriesKey(
            entity_id="entity1",
            labels='{"service": "svc1"}',
            metric="request_count",
            metric_set_id="apm@metric_set"
        )
        
        # 不同的 key 应有不同的 hash
        assert key1.hash() != key2.hash(), \
            "Different keys should have different hashes"
        
        # 相同的 key 应有相同的 hash
        assert key1.hash() == key3.hash(), \
            "Same keys should have same hashes"

    def test_default_diff_details_values(self):
        """测试 DiffDetails 的默认值
        
        Feature: metric-compare, Property 6: 输出结构完整性
        **Validates: Requirements 6.1, 6.2, 7.1**
        """
        diff = DiffDetails()
        
        assert diff.trend == Trend.STABLE
        assert diff.avg_change == 0.0
        assert diff.avg_change_percent == 0.0
        assert diff.max_change == 0.0
        assert diff.min_change == 0.0

    def test_default_time_series_stats_values(self):
        """测试 TimeSeriesStats 的默认值
        
        Feature: metric-compare, Property 6: 输出结构完整性
        **Validates: Requirements 6.1, 6.2, 7.1**
        """
        stats = TimeSeriesStats()
        
        assert stats.max == 0.0
        assert stats.min == 0.0
        assert stats.avg == 0.0
        assert stats.count == 0
        assert stats.max_time == ""
        assert stats.min_time == ""


# ============================================================================
# Property Tests - Property 7: 时间格式正确性
# ============================================================================

class TestTimeFormatCorrectnessProperty:
    """Property 7: 时间格式正确性属性测试
    
    Feature: metric-compare, Property 7: 时间格式正确性
    **Validates: Requirements 2.3**
    
    *For any* 时间戳，格式化后的字符串应符合 `YYYY-MM-DD HH:MM:SS` 格式（19 个字符）。
    """
    
    @settings(max_examples=100)
    @given(timestamp_ns=timestamp_ns_strategy)
    def test_property_7_1_formatted_time_has_correct_length(self, timestamp_ns: int):
        """Property 7.1: 格式化后的时间字符串长度应为 19 个字符
        
        Feature: metric-compare, Property 7: 时间格式正确性
        **Validates: Requirements 2.3**
        """
        formatted_time = _format_timestamp_ns(timestamp_ns)
        
        assert len(formatted_time) == 19, \
            f"Expected length 19, got {len(formatted_time)} for timestamp {timestamp_ns}: '{formatted_time}'"
    
    @settings(max_examples=100)
    @given(timestamp_ns=timestamp_ns_strategy)
    def test_property_7_2_formatted_time_matches_pattern(self, timestamp_ns: int):
        """Property 7.2: 格式化后的时间字符串应符合 YYYY-MM-DD HH:MM:SS 格式
        
        Feature: metric-compare, Property 7: 时间格式正确性
        **Validates: Requirements 2.3**
        """
        import re
        
        formatted_time = _format_timestamp_ns(timestamp_ns)
        
        # 正则表达式匹配 YYYY-MM-DD HH:MM:SS 格式
        pattern = r'^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$'
        
        assert re.match(pattern, formatted_time), \
            f"Time '{formatted_time}' does not match pattern YYYY-MM-DD HH:MM:SS"
    
    @settings(max_examples=100)
    @given(timestamp_ns=timestamp_ns_strategy)
    def test_property_7_3_formatted_time_has_valid_date_components(self, timestamp_ns: int):
        """Property 7.3: 格式化后的时间字符串应包含有效的日期组件
        
        Feature: metric-compare, Property 7: 时间格式正确性
        **Validates: Requirements 2.3**
        """
        formatted_time = _format_timestamp_ns(timestamp_ns)
        
        # 解析日期组件
        year = int(formatted_time[0:4])
        month = int(formatted_time[5:7])
        day = int(formatted_time[8:10])
        
        # 验证年份范围（2020-2030，与 timestamp_ns_strategy 一致）
        assert 2020 <= year <= 2030, \
            f"Year {year} out of expected range [2020, 2030]"
        
        # 验证月份范围
        assert 1 <= month <= 12, \
            f"Month {month} out of valid range [1, 12]"
        
        # 验证日期范围
        assert 1 <= day <= 31, \
            f"Day {day} out of valid range [1, 31]"
    
    @settings(max_examples=100)
    @given(timestamp_ns=timestamp_ns_strategy)
    def test_property_7_4_formatted_time_has_valid_time_components(self, timestamp_ns: int):
        """Property 7.4: 格式化后的时间字符串应包含有效的时间组件
        
        Feature: metric-compare, Property 7: 时间格式正确性
        **Validates: Requirements 2.3**
        """
        formatted_time = _format_timestamp_ns(timestamp_ns)
        
        # 解析时间组件
        hour = int(formatted_time[11:13])
        minute = int(formatted_time[14:16])
        second = int(formatted_time[17:19])
        
        # 验证小时范围
        assert 0 <= hour <= 23, \
            f"Hour {hour} out of valid range [0, 23]"
        
        # 验证分钟范围
        assert 0 <= minute <= 59, \
            f"Minute {minute} out of valid range [0, 59]"
        
        # 验证秒范围
        assert 0 <= second <= 59, \
            f"Second {second} out of valid range [0, 59]"
    
    @settings(max_examples=100)
    @given(timestamp_ns=timestamp_ns_strategy)
    def test_property_7_5_formatted_time_has_correct_separators(self, timestamp_ns: int):
        """Property 7.5: 格式化后的时间字符串应包含正确的分隔符
        
        Feature: metric-compare, Property 7: 时间格式正确性
        **Validates: Requirements 2.3**
        """
        formatted_time = _format_timestamp_ns(timestamp_ns)
        
        # 验证日期分隔符（位置 4 和 7 应为 '-'）
        assert formatted_time[4] == '-', \
            f"Expected '-' at position 4, got '{formatted_time[4]}'"
        assert formatted_time[7] == '-', \
            f"Expected '-' at position 7, got '{formatted_time[7]}'"
        
        # 验证日期和时间之间的空格（位置 10）
        assert formatted_time[10] == ' ', \
            f"Expected ' ' at position 10, got '{formatted_time[10]}'"
        
        # 验证时间分隔符（位置 13 和 16 应为 ':'）
        assert formatted_time[13] == ':', \
            f"Expected ':' at position 13, got '{formatted_time[13]}'"
        assert formatted_time[16] == ':', \
            f"Expected ':' at position 16, got '{formatted_time[16]}'"
    
    @settings(max_examples=100)
    @given(data=non_empty_time_series_strategy())
    def test_property_7_6_compute_stats_returns_correctly_formatted_times(self, data):
        """Property 7.6: compute_stats 返回的 max_time 和 min_time 应符合正确格式
        
        Feature: metric-compare, Property 7: 时间格式正确性
        **Validates: Requirements 2.3**
        """
        import re
        
        values, timestamps = data
        
        stats = compute_stats(values, timestamps)
        
        # 正则表达式匹配 YYYY-MM-DD HH:MM:SS 格式
        pattern = r'^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$'
        
        # 验证 max_time 格式
        assert re.match(pattern, stats.max_time), \
            f"max_time '{stats.max_time}' does not match pattern YYYY-MM-DD HH:MM:SS"
        assert len(stats.max_time) == 19, \
            f"max_time length should be 19, got {len(stats.max_time)}"
        
        # 验证 min_time 格式
        assert re.match(pattern, stats.min_time), \
            f"min_time '{stats.min_time}' does not match pattern YYYY-MM-DD HH:MM:SS"
        assert len(stats.min_time) == 19, \
            f"min_time length should be 19, got {len(stats.min_time)}"


class TestTimeFormatEdgeCases:
    """时间格式边界情况测试
    
    Feature: metric-compare, Property 7: 时间格式正确性
    **Validates: Requirements 2.3**
    """
    
    def test_zero_timestamp_returns_empty_string(self):
        """测试零时间戳返回空字符串
        
        Feature: metric-compare, Property 7: 时间格式正确性
        **Validates: Requirements 2.3**
        """
        formatted_time = _format_timestamp_ns(0)
        
        assert formatted_time == "", \
            f"Expected empty string for zero timestamp, got '{formatted_time}'"
    
    def test_negative_timestamp_returns_empty_string(self):
        """测试负时间戳返回空字符串
        
        Feature: metric-compare, Property 7: 时间格式正确性
        **Validates: Requirements 2.3**
        """
        formatted_time = _format_timestamp_ns(-1)
        
        assert formatted_time == "", \
            f"Expected empty string for negative timestamp, got '{formatted_time}'"
    
    def test_specific_timestamp_format(self):
        """测试特定时间戳的格式化结果
        
        Feature: metric-compare, Property 7: 时间格式正确性
        **Validates: Requirements 2.3**
        """
        # 2024-01-08 10:00:00 UTC 的纳秒时间戳
        timestamp_ns = 1704708000_000_000_000
        
        formatted_time = _format_timestamp_ns(timestamp_ns)
        
        # 验证格式
        assert len(formatted_time) == 19
        assert formatted_time == "2024-01-08 10:00:00"
    
    def test_leap_year_date_format(self):
        """测试闰年日期的格式化
        
        Feature: metric-compare, Property 7: 时间格式正确性
        **Validates: Requirements 2.3**
        """
        import re
        
        # 2024-02-29 12:00:00 UTC（2024 是闰年）
        timestamp_ns = 1709208000_000_000_000
        
        formatted_time = _format_timestamp_ns(timestamp_ns)
        
        # 验证格式
        pattern = r'^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$'
        assert re.match(pattern, formatted_time), \
            f"Time '{formatted_time}' does not match expected pattern"
        assert len(formatted_time) == 19
    
    def test_year_boundary_timestamps(self):
        """测试年份边界时间戳的格式化
        
        Feature: metric-compare, Property 7: 时间格式正确性
        **Validates: Requirements 2.3**
        """
        import re
        
        pattern = r'^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$'
        
        # 2020-01-01 00:00:00 UTC
        ts_2020 = 1577836800_000_000_000
        formatted_2020 = _format_timestamp_ns(ts_2020)
        assert re.match(pattern, formatted_2020)
        assert len(formatted_2020) == 19
        assert formatted_2020.startswith("2020-01-01")
        
        # 2029-12-31 23:59:59 UTC
        ts_2029 = 1893455999_000_000_000
        formatted_2029 = _format_timestamp_ns(ts_2029)
        assert re.match(pattern, formatted_2029)
        assert len(formatted_2029) == 19
        assert formatted_2029.startswith("2029-12-31")
    
    def test_compute_stats_empty_timestamps_returns_empty_times(self):
        """测试 compute_stats 在时间戳为空时返回空时间字符串
        
        Feature: metric-compare, Property 7: 时间格式正确性
        **Validates: Requirements 2.3**
        """
        values = [1.0, 2.0, 3.0]
        timestamps = []
        
        stats = compute_stats(values, timestamps)
        
        assert stats.max_time == "", \
            f"Expected empty max_time, got '{stats.max_time}'"
        assert stats.min_time == "", \
            f"Expected empty min_time, got '{stats.min_time}'"
    
    def test_compute_stats_empty_values_returns_empty_times(self):
        """测试 compute_stats 在值为空时返回空时间字符串
        
        Feature: metric-compare, Property 7: 时间格式正确性
        **Validates: Requirements 2.3**
        """
        values = []
        timestamps = []
        
        stats = compute_stats(values, timestamps)
        
        assert stats.max_time == "", \
            f"Expected empty max_time, got '{stats.max_time}'"
        assert stats.min_time == "", \
            f"Expected empty min_time, got '{stats.min_time}'"


# ============================================================================
# 运行测试
# ============================================================================

if __name__ == "__main__":
    pytest.main([__file__, "-v"])
