"""PaaS UModel MCP 工具优化 - 时间范围解析属性测试

Feature: umodel-paas-optimization, Property 1: 时间范围解析正确性

**Validates: Requirements 1.1, 1.3**

Property 1: 时间范围解析正确性
- *For any* PaaS 工具和任意有效的时间范围表达式（包括空值/None），工具应使用 `compute_time_range()` 正确解析时间参数，空值时默认使用 `last_1h`。

测试框架: pytest + hypothesis
最小迭代次数: 100 per property test
"""
import time
from typing import Optional, Tuple
from unittest.mock import MagicMock, patch

import pytest
from hypothesis import given, settings, assume
from hypothesis import strategies as st
from mcp.server.fastmcp import FastMCP

from mcp_server_aliyun_observability.toolkits.paas.data_toolkit import PaasDataToolkit
from mcp_server_aliyun_observability.toolkits.paas.time_utils import compute_time_range


# ============================================================================
# Helper Functions
# ============================================================================

def create_toolkit():
    """创建 PaasDataToolkit 实例用于测试"""
    server = MagicMock(spec=FastMCP)
    server.tool = MagicMock(return_value=lambda f: f)
    with patch.object(PaasDataToolkit, '_register_tools'):
        toolkit = PaasDataToolkit(server)
    return toolkit


# ============================================================================
# Test Data Generation Strategies (from design doc)
# ============================================================================

# 有效的时间范围表达式
valid_time_ranges = st.one_of(
    st.just(None),
    st.just(''),
    st.sampled_from(['last_5m', 'last_1h', 'last_3d', 'today', 'yesterday']),
    st.builds(
        lambda n, u: f'last_{n}{u}',
        st.integers(min_value=1, max_value=100),
        st.sampled_from(['m', 'h', 'd'])
    )
)

# 相对预设格式: last_Xm, last_Xh, last_Xd, last_Xw
relative_preset_time_ranges = st.builds(
    lambda n, u: f'last_{n}{u}',
    st.integers(min_value=1, max_value=100),
    st.sampled_from(['m', 'h', 'd', 'w'])
)

# 简单时长格式: Xm, Xh, Xd
simple_duration_time_ranges = st.builds(
    lambda n, u: f'{n}{u}',
    st.integers(min_value=1, max_value=100),
    st.sampled_from(['m', 'h', 'd'])
)

# 关键字格式
keyword_time_ranges = st.sampled_from(['today', 'yesterday', 'now'])

# Grafana 风格格式: now-Xm~now, now-Xh~now-Ym
grafana_style_time_ranges = st.one_of(
    # now-Xm~now
    st.builds(
        lambda n, u: f'now-{n}{u}~now',
        st.integers(min_value=1, max_value=60),
        st.sampled_from(['m', 'h'])
    ),
    # now-Xh~now-Ym (确保 X > Y)
    st.builds(
        lambda x, y, u: f'now-{x}{u}~now-{y}{u}',
        st.integers(min_value=10, max_value=60),
        st.integers(min_value=1, max_value=9),
        st.sampled_from(['m', 'h'])
    )
)

# 空值/None 输入
empty_inputs = st.one_of(
    st.just(None),
    st.just(''),
    st.just('   '),  # 仅空白字符
    st.just('\t'),
    st.just('\n')
)


# ============================================================================
# Property Tests - Property 1: 时间范围解析正确性
# ============================================================================

class TestTimeRangeParsingCorrectness:
    """Property 1: 时间范围解析正确性属性测试
    
    Feature: umodel-paas-optimization, Property 1: 时间范围解析正确性
    **Validates: Requirements 1.1, 1.3**
    """

    @settings(max_examples=100)
    @given(time_range=empty_inputs)
    def test_property_1_1_empty_input_defaults_to_last_1h(
        self, time_range: Optional[str]
    ):
        """Property 1.1: 空值/None 输入应默认使用 last_1h
        
        Feature: umodel-paas-optimization, Property 1: 时间范围解析正确性
        **Validates: Requirements 1.1, 1.3**
        
        验证：当 time_range 为 None、空字符串或仅空白字符时，
        _parse_time_range 应返回约 1 小时的时间范围。
        """
        toolkit = create_toolkit()
        
        from_ts, to_ts = toolkit._parse_time_range(time_range)
        
        # 验证返回的是有效的时间戳
        assert isinstance(from_ts, int), f"from_ts should be int, got {type(from_ts)}"
        assert isinstance(to_ts, int), f"to_ts should be int, got {type(to_ts)}"
        assert from_ts < to_ts, f"from_ts ({from_ts}) should be less than to_ts ({to_ts})"
        
        # 验证时间范围约为 1 小时（3600 秒，允许 10 秒误差）
        duration = to_ts - from_ts
        assert 3590 <= duration <= 3610, \
            f"Duration should be ~3600s (1h), got {duration}s for input {time_range!r}"

    @settings(max_examples=100)
    @given(time_range=relative_preset_time_ranges)
    def test_property_1_2_relative_preset_parsed_correctly(
        self, time_range: str
    ):
        """Property 1.2: 相对预设格式（last_Xm/h/d/w）应正确解析
        
        Feature: umodel-paas-optimization, Property 1: 时间范围解析正确性
        **Validates: Requirements 1.1, 1.3**
        
        验证：_parse_time_range 的结果应与 compute_time_range 一致。
        """
        toolkit = create_toolkit()
        
        # 使用 _parse_time_range 解析
        from_ts, to_ts = toolkit._parse_time_range(time_range)
        
        # 使用 compute_time_range 直接解析作为参考
        expected_from, expected_to = compute_time_range(time_range)
        
        # 验证结果一致（允许 2 秒误差，因为两次调用之间可能有时间差）
        assert abs(from_ts - expected_from) <= 2, \
            f"from_ts mismatch: got {from_ts}, expected {expected_from}"
        assert abs(to_ts - expected_to) <= 2, \
            f"to_ts mismatch: got {to_ts}, expected {expected_to}"
        
        # 验证时间范围有效
        assert from_ts < to_ts, f"from_ts ({from_ts}) should be less than to_ts ({to_ts})"

    @settings(max_examples=100)
    @given(time_range=simple_duration_time_ranges)
    def test_property_1_3_simple_duration_parsed_correctly(
        self, time_range: str
    ):
        """Property 1.3: 简单时长格式（Xm/h/d）应正确解析
        
        Feature: umodel-paas-optimization, Property 1: 时间范围解析正确性
        **Validates: Requirements 1.1, 1.3**
        
        验证：_parse_time_range 的结果应与 compute_time_range 一致。
        """
        toolkit = create_toolkit()
        
        # 使用 _parse_time_range 解析
        from_ts, to_ts = toolkit._parse_time_range(time_range)
        
        # 使用 compute_time_range 直接解析作为参考
        expected_from, expected_to = compute_time_range(time_range)
        
        # 验证结果一致（允许 2 秒误差）
        assert abs(from_ts - expected_from) <= 2, \
            f"from_ts mismatch: got {from_ts}, expected {expected_from}"
        assert abs(to_ts - expected_to) <= 2, \
            f"to_ts mismatch: got {to_ts}, expected {expected_to}"
        
        # 验证时间范围有效
        assert from_ts < to_ts, f"from_ts ({from_ts}) should be less than to_ts ({to_ts})"

    @settings(max_examples=100)
    @given(time_range=keyword_time_ranges)
    def test_property_1_4_keyword_parsed_correctly(
        self, time_range: str
    ):
        """Property 1.4: 关键字格式（today/yesterday/now）应正确解析
        
        Feature: umodel-paas-optimization, Property 1: 时间范围解析正确性
        **Validates: Requirements 1.1, 1.3**
        
        验证：_parse_time_range 的结果应与 compute_time_range 一致。
        """
        toolkit = create_toolkit()
        
        # 使用 _parse_time_range 解析
        from_ts, to_ts = toolkit._parse_time_range(time_range)
        
        # 使用 compute_time_range 直接解析作为参考
        expected_from, expected_to = compute_time_range(time_range)
        
        # 验证结果一致（允许 2 秒误差）
        assert abs(from_ts - expected_from) <= 2, \
            f"from_ts mismatch: got {from_ts}, expected {expected_from}"
        assert abs(to_ts - expected_to) <= 2, \
            f"to_ts mismatch: got {to_ts}, expected {expected_to}"
        
        # 验证时间范围有效
        assert from_ts < to_ts, f"from_ts ({from_ts}) should be less than to_ts ({to_ts})"

    @settings(max_examples=100)
    @given(time_range=grafana_style_time_ranges)
    def test_property_1_5_grafana_style_parsed_correctly(
        self, time_range: str
    ):
        """Property 1.5: Grafana 风格格式（now-Xm~now）应正确解析
        
        Feature: umodel-paas-optimization, Property 1: 时间范围解析正确性
        **Validates: Requirements 1.1, 1.3**
        
        验证：_parse_time_range 的结果应与 compute_time_range 一致。
        """
        toolkit = create_toolkit()
        
        # 使用 _parse_time_range 解析
        from_ts, to_ts = toolkit._parse_time_range(time_range)
        
        # 使用 compute_time_range 直接解析作为参考
        expected_from, expected_to = compute_time_range(time_range)
        
        # 验证结果一致（允许 2 秒误差）
        assert abs(from_ts - expected_from) <= 2, \
            f"from_ts mismatch: got {from_ts}, expected {expected_from}"
        assert abs(to_ts - expected_to) <= 2, \
            f"to_ts mismatch: got {to_ts}, expected {expected_to}"
        
        # 验证时间范围有效
        assert from_ts < to_ts, f"from_ts ({from_ts}) should be less than to_ts ({to_ts})"

    @settings(max_examples=100)
    @given(time_range=valid_time_ranges)
    def test_property_1_6_all_valid_formats_return_valid_timestamps(
        self, time_range: Optional[str]
    ):
        """Property 1.6: 所有有效格式应返回有效的时间戳元组
        
        Feature: umodel-paas-optimization, Property 1: 时间范围解析正确性
        **Validates: Requirements 1.1, 1.3**
        
        验证：对于任意有效的时间范围表达式，_parse_time_range 应返回：
        1. 两个整数类型的时间戳
        2. from_ts < to_ts
        3. 时间戳在合理范围内（不是负数，不超过当前时间太多）
        """
        toolkit = create_toolkit()
        
        from_ts, to_ts = toolkit._parse_time_range(time_range)
        
        # 验证返回类型
        assert isinstance(from_ts, int), f"from_ts should be int, got {type(from_ts)}"
        assert isinstance(to_ts, int), f"to_ts should be int, got {type(to_ts)}"
        
        # 验证时间顺序
        assert from_ts < to_ts, \
            f"from_ts ({from_ts}) should be less than to_ts ({to_ts}) for input {time_range!r}"
        
        # 验证时间戳在合理范围内
        now = int(time.time())
        # from_ts 应该在过去 1 年内
        one_year_ago = now - 365 * 24 * 3600
        assert from_ts >= one_year_ago, \
            f"from_ts ({from_ts}) should be within last year for input {time_range!r}"
        
        # to_ts 应该不超过当前时间太多（允许 10 秒误差）
        assert to_ts <= now + 10, \
            f"to_ts ({to_ts}) should not exceed current time ({now}) for input {time_range!r}"

    @settings(max_examples=100)
    @given(time_range=valid_time_ranges)
    def test_property_1_7_uses_compute_time_range_internally(
        self, time_range: Optional[str]
    ):
        """Property 1.7: _parse_time_range 应内部使用 compute_time_range
        
        Feature: umodel-paas-optimization, Property 1: 时间范围解析正确性
        **Validates: Requirements 1.1, 1.3**
        
        验证：_parse_time_range 的结果应与直接调用 compute_time_range 一致。
        """
        toolkit = create_toolkit()
        
        # 使用 _parse_time_range 解析
        from_ts, to_ts = toolkit._parse_time_range(time_range)
        
        # 确定实际使用的表达式（空值默认为 last_1h）
        actual_expr = time_range if time_range and time_range.strip() else 'last_1h'
        
        # 使用 compute_time_range 直接解析作为参考
        expected_from, expected_to = compute_time_range(actual_expr)
        
        # 验证结果一致（允许 2 秒误差，因为两次调用之间可能有时间差）
        assert abs(from_ts - expected_from) <= 2, \
            f"from_ts mismatch for {time_range!r}: got {from_ts}, expected {expected_from}"
        assert abs(to_ts - expected_to) <= 2, \
            f"to_ts mismatch for {time_range!r}: got {to_ts}, expected {expected_to}"

    @settings(max_examples=100)
    @given(
        n=st.integers(min_value=1, max_value=100),
        unit=st.sampled_from(['m', 'h', 'd'])
    )
    def test_property_1_8_duration_calculation_correct(
        self, n: int, unit: str
    ):
        """Property 1.8: 时间范围的持续时间应与表达式一致
        
        Feature: umodel-paas-optimization, Property 1: 时间范围解析正确性
        **Validates: Requirements 1.1, 1.3**
        
        验证：对于 last_Nm/h/d 格式，返回的时间范围持续时间应正确。
        """
        toolkit = create_toolkit()
        
        time_range = f'last_{n}{unit}'
        from_ts, to_ts = toolkit._parse_time_range(time_range)
        
        # 计算期望的持续时间（秒）
        unit_seconds = {'m': 60, 'h': 3600, 'd': 86400}
        expected_duration = n * unit_seconds[unit]
        
        # 验证实际持续时间（允许 2 秒误差）
        actual_duration = to_ts - from_ts
        assert abs(actual_duration - expected_duration) <= 2, \
            f"Duration mismatch for {time_range}: got {actual_duration}s, expected {expected_duration}s"

    @settings(max_examples=100)
    @given(time_range=valid_time_ranges)
    def test_property_1_9_to_timestamp_close_to_now(
        self, time_range: Optional[str]
    ):
        """Property 1.9: 对于相对时间表达式，to_ts 应接近当前时间
        
        Feature: umodel-paas-optimization, Property 1: 时间范围解析正确性
        **Validates: Requirements 1.1, 1.3**
        
        验证：对于 last_X、today、now 等相对表达式，to_ts 应接近当前时间。
        """
        toolkit = create_toolkit()
        
        from_ts, to_ts = toolkit._parse_time_range(time_range)
        
        now = int(time.time())
        
        # 确定实际使用的表达式
        actual_expr = time_range if time_range and time_range.strip() else 'last_1h'
        
        # 对于 yesterday，to_ts 是今天的开始，不是当前时间
        if actual_expr.lower() == 'yesterday':
            # yesterday 的 to_ts 是今天的开始（午夜）
            # 允许 24 小时 + 10 秒的误差
            assert to_ts <= now + 10, \
                f"to_ts ({to_ts}) should not exceed current time ({now}) for 'yesterday'"
        else:
            # 其他相对表达式的 to_ts 应接近当前时间（允许 10 秒误差）
            assert abs(to_ts - now) <= 10, \
                f"to_ts ({to_ts}) should be close to now ({now}) for {time_range!r}"


# ============================================================================
# 边界情况测试
# ============================================================================

class TestTimeRangeParsingEdgeCases:
    """时间范围解析边界情况测试
    
    Feature: umodel-paas-optimization, Property 1: 时间范围解析正确性
    **Validates: Requirements 1.1, 1.3**
    """

    def test_absolute_timestamp_range(self):
        """测试绝对时间戳范围格式
        
        Feature: umodel-paas-optimization, Property 1: 时间范围解析正确性
        **Validates: Requirements 1.1, 1.3**
        """
        toolkit = create_toolkit()
        
        # 使用固定的绝对时间戳
        time_range = "1706864400~1706868000"
        from_ts, to_ts = toolkit._parse_time_range(time_range)
        
        assert from_ts == 1706864400, f"from_ts should be 1706864400, got {from_ts}"
        assert to_ts == 1706868000, f"to_ts should be 1706868000, got {to_ts}"

    def test_human_readable_range(self):
        """测试人类可读日期时间范围格式
        
        Feature: umodel-paas-optimization, Property 1: 时间范围解析正确性
        **Validates: Requirements 1.1, 1.3**
        """
        toolkit = create_toolkit()
        
        time_range = "2024-02-02 10:10:10~2024-02-02 10:20:10"
        from_ts, to_ts = toolkit._parse_time_range(time_range)
        
        # 验证时间范围为 10 分钟
        duration = to_ts - from_ts
        assert duration == 600, f"Duration should be 600s (10min), got {duration}s"

    def test_invalid_expression_raises_valueerror(self):
        """测试无效表达式抛出 ValueError
        
        Feature: umodel-paas-optimization, Property 1: 时间范围解析正确性
        **Validates: Requirements 1.1, 1.3**
        """
        toolkit = create_toolkit()
        
        with pytest.raises(ValueError) as exc_info:
            toolkit._parse_time_range("invalid_time_expression")
        
        error_message = str(exc_info.value)
        assert "无效的时间表达式" in error_message, \
            f"Error message should contain '无效的时间表达式': {error_message}"

    def test_error_message_contains_suggestions(self):
        """测试错误消息包含建议的格式
        
        Feature: umodel-paas-optimization, Property 1: 时间范围解析正确性
        **Validates: Requirements 1.1, 1.3**
        """
        toolkit = create_toolkit()
        
        with pytest.raises(ValueError) as exc_info:
            toolkit._parse_time_range("not_a_valid_time")
        
        error_message = str(exc_info.value)
        # 验证错误消息包含建议的格式
        assert any(fmt in error_message for fmt in ['last_5m', 'last_1h', 'today', 'yesterday']), \
            f"Error message should contain format suggestions: {error_message}"


# ============================================================================
# 运行测试
# ============================================================================

if __name__ == "__main__":
    pytest.main([__file__, "-v"])
