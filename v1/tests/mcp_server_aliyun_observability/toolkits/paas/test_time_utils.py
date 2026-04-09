"""
时间范围解析器属性测试

Feature: time-range-parser
测试框架: pytest + hypothesis
"""

import pytest
from datetime import datetime, timezone, timedelta
from hypothesis import given, strategies as st, settings, assume

from mcp_server_aliyun_observability.toolkits.paas.time_utils import (
    UNIT_SECONDS,
    _unit_to_seconds,
    _start_of_day,
    _normalize_timestamp_enhanced,
    TimeRangeParser,
)


# ============================================================================
# 测试策略定义
# ============================================================================

# 有效时间单位
valid_units = st.sampled_from(['s', 'm', 'h', 'd', 'w', 'M', 'y'])

# 正整数数量（用于时长）
positive_amount = st.integers(min_value=1, max_value=1000)

# 零或负整数（用于错误测试）
non_positive_amount = st.integers(max_value=0)

# 有效的 10 位时间戳（2000-01-01 到 2100-01-01）
valid_timestamp_10 = st.integers(min_value=946684800, max_value=4102444800)

# 有效的 13 位时间戳（毫秒）
valid_timestamp_13 = st.integers(min_value=946684800000, max_value=4102444800000)

# 有效的 16 位时间戳（微秒）
valid_timestamp_16 = st.integers(min_value=946684800000000, max_value=4102444800000000)

# 有效的 19 位时间戳（纳秒）
valid_timestamp_19 = st.integers(min_value=946684800000000000, max_value=4102444800000000000)

# 有效日期时间
valid_datetime = st.datetimes(
    min_value=datetime(2000, 1, 1),
    max_value=datetime(2100, 12, 31)
)


# ============================================================================
# Property 3: 时间单位支持完整性
# ============================================================================

class TestProperty3_TimeUnitCompleteness:
    """
    Feature: time-range-parser, Property 3: 时间单位支持完整性
    
    For any 支持的时间单位 unit ∈ {s, m, h, d, w, M, y} 和任意正整数 N，
    _unit_to_seconds 应成功返回正确的秒数。
    
    Validates: Requirements 1.2, 2.2
    """
    
    @given(amount=positive_amount, unit=valid_units)
    @settings(max_examples=100)
    def test_all_units_supported(self, amount: int, unit: str):
        """所有支持的单位都应该能正确转换为秒数"""
        result = _unit_to_seconds(amount, unit)
        
        # 验证结果是正整数
        assert isinstance(result, int)
        assert result > 0
        
        # 验证计算正确性
        # 处理大小写：M 表示月，其他小写
        normalized_unit = 'M' if unit == 'M' else unit.lower()
        expected = amount * UNIT_SECONDS[normalized_unit]
        assert result == expected
    
    @given(amount=positive_amount)
    @settings(max_examples=20)
    def test_invalid_unit_raises_error(self, amount: int):
        """无效的单位应该抛出 ValueError"""
        invalid_units = ['x', 'z', 'q', 'p', '1', '@']
        for unit in invalid_units:
            with pytest.raises(ValueError) as exc_info:
                _unit_to_seconds(amount, unit)
            assert "不支持的时间单位" in str(exc_info.value)


# ============================================================================
# Property 12: 时间戳精度归一化
# ============================================================================

class TestProperty12_TimestampNormalization:
    """
    Feature: time-range-parser, Property 12: 时间戳精度归一化
    
    For any Unix 时间戳，根据其位数：
    - 10 位：结果 = 输入
    - 13 位：结果 = 输入 / 1000
    - 16 位：结果 = 输入 / 1,000,000
    - 19 位：结果 = 输入 / 1,000,000,000
    
    Validates: Requirements 5.1, 5.2, 5.3, 5.4
    """
    
    @given(ts=valid_timestamp_10)
    @settings(max_examples=100)
    def test_10_digit_timestamp_unchanged(self, ts: int):
        """10 位时间戳应该保持不变"""
        result = _normalize_timestamp_enhanced(ts)
        assert result == ts
    
    @given(ts=valid_timestamp_13)
    @settings(max_examples=100)
    def test_13_digit_timestamp_divided_by_1000(self, ts: int):
        """13 位时间戳应该除以 1000"""
        result = _normalize_timestamp_enhanced(ts)
        assert result == ts // 1000
    
    @given(ts=valid_timestamp_16)
    @settings(max_examples=100)
    def test_16_digit_timestamp_divided_by_1000000(self, ts: int):
        """16 位时间戳应该除以 1,000,000"""
        result = _normalize_timestamp_enhanced(ts)
        assert result == ts // 1000000
    
    @given(ts=valid_timestamp_19)
    @settings(max_examples=100)
    def test_19_digit_timestamp_divided_by_1000000000(self, ts: int):
        """19 位时间戳应该除以 1,000,000,000"""
        result = _normalize_timestamp_enhanced(ts)
        assert result == ts // 1000000000
    
    def test_timestamp_precision_consistency(self):
        """同一时间点的不同精度时间戳应该归一化为相同的秒级时间戳"""
        # 2024-02-02 10:10:10 UTC
        ts_seconds = 1706868610
        ts_millis = ts_seconds * 1000
        ts_micros = ts_seconds * 1000000
        ts_nanos = ts_seconds * 1000000000
        
        assert _normalize_timestamp_enhanced(ts_seconds) == ts_seconds
        assert _normalize_timestamp_enhanced(ts_millis) == ts_seconds
        assert _normalize_timestamp_enhanced(ts_micros) == ts_seconds
        assert _normalize_timestamp_enhanced(ts_nanos) == ts_seconds


# ============================================================================
# _start_of_day 辅助函数测试
# ============================================================================

class TestStartOfDay:
    """测试 _start_of_day 辅助函数"""
    
    @given(dt=valid_datetime)
    @settings(max_examples=100)
    def test_start_of_day_is_midnight(self, dt: datetime):
        """_start_of_day 应该返回当天午夜"""
        result = _start_of_day(dt)
        
        assert result.hour == 0
        assert result.minute == 0
        assert result.second == 0
        assert result.microsecond == 0
        assert result.year == dt.year
        assert result.month == dt.month
        assert result.day == dt.day
    
    def test_start_of_day_preserves_timezone(self):
        """_start_of_day 应该保留时区信息"""
        tz = timezone(timedelta(hours=8))
        dt = datetime(2024, 2, 2, 15, 30, 45, tzinfo=tz)
        
        result = _start_of_day(dt)
        
        assert result.tzinfo == tz
        assert result.hour == 0


# ============================================================================
# TimeRangeParser._normalize_timestamp 向后兼容测试
# ============================================================================

class TestTimeRangeParserBackwardCompatibility:
    """测试 TimeRangeParser 的向后兼容性"""
    
    def test_normalize_timestamp_10_digit(self):
        """10 位时间戳应该保持不变"""
        ts = 1706868610
        result = TimeRangeParser._normalize_timestamp(ts)
        assert result == ts
    
    def test_normalize_timestamp_13_digit(self):
        """13 位时间戳应该除以 1000"""
        ts = 1706868610000
        result = TimeRangeParser._normalize_timestamp(ts)
        assert result == 1706868610
    
    def test_normalize_timestamp_16_digit(self):
        """16 位时间戳应该除以 1,000,000"""
        ts = 1706868610000000
        result = TimeRangeParser._normalize_timestamp(ts)
        assert result == 1706868610
    
    def test_normalize_timestamp_19_digit(self):
        """19 位时间戳应该除以 1,000,000,000"""
        ts = 1706868610000000000
        result = TimeRangeParser._normalize_timestamp(ts)
        assert result == 1706868610
