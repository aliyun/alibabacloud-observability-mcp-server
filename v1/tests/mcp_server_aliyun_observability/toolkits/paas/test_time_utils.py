"""TimeRangeParser 单元测试"""
from datetime import datetime

import pytest

from mcp_server_aliyun_observability.toolkits.paas.time_utils import TimeRangeParser


class TestParseTimeExpression:
    """parse_time_expression 方法测试"""

    def test_unix_timestamp_seconds(self):
        """测试秒级 Unix 时间戳"""
        ts = 1704067200
        assert TimeRangeParser.parse_time_expression(ts) == ts

    def test_unix_timestamp_milliseconds(self):
        """测试毫秒级 Unix 时间戳自动转换为秒"""
        ts_ms = 1704067200000
        ts_s = 1704067200
        assert TimeRangeParser.parse_time_expression(ts_ms) == ts_s

    def test_unix_timestamp_string(self):
        """测试纯数字字符串时间戳"""
        assert TimeRangeParser.parse_time_expression("1704067200") == 1704067200

    def test_relative_now(self):
        """测试 'now' 关键字"""
        result = TimeRangeParser.parse_time_expression("now")
        import time
        assert abs(result - int(time.time())) <= 2

    def test_relative_now_minus_hours(self):
        """测试 now-Nh 相对时间"""
        import time
        result = TimeRangeParser.parse_time_expression("now-1h")
        expected = int(time.time()) - 3600
        assert abs(result - expected) <= 2

    def test_relative_now_minus_minutes(self):
        """测试 now-Nm 相对时间"""
        import time
        result = TimeRangeParser.parse_time_expression("now-30m")
        expected = int(time.time()) - 1800
        assert abs(result - expected) <= 2

    def test_datetime_string_space_format(self):
        """测试 'YYYY-MM-DD HH:MM:SS' 格式（问题中报告的格式）"""
        result = TimeRangeParser.parse_time_expression("2026-03-06 17:30:00")
        expected = int(datetime.strptime("2026-03-06 17:30:00", "%Y-%m-%d %H:%M:%S").timestamp())
        assert result == expected

    def test_datetime_string_iso8601_format(self):
        """测试 ISO 8601 格式 'YYYY-MM-DDTHH:MM:SS'"""
        result = TimeRangeParser.parse_time_expression("2026-03-06T17:30:00")
        expected = int(datetime.strptime("2026-03-06T17:30:00", "%Y-%m-%dT%H:%M:%S").timestamp())
        assert result == expected

    def test_datetime_string_iso8601_utc_format(self):
        """测试 ISO 8601 UTC 格式 'YYYY-MM-DDTHH:MM:SSZ'"""
        result = TimeRangeParser.parse_time_expression("2026-03-06T17:30:00Z")
        expected = int(datetime.strptime("2026-03-06T17:30:00Z", "%Y-%m-%dT%H:%M:%SZ").timestamp())
        assert result == expected

    def test_datetime_string_slash_format(self):
        """测试 'YYYY/MM/DD HH:MM:SS' 格式"""
        result = TimeRangeParser.parse_time_expression("2026/03/06 17:30:00")
        expected = int(datetime.strptime("2026/03/06 17:30:00", "%Y/%m/%d %H:%M:%S").timestamp())
        assert result == expected

    def test_datetime_date_only_format(self):
        """测试 'YYYY-MM-DD' 纯日期格式"""
        result = TimeRangeParser.parse_time_expression("2026-03-06")
        expected = int(datetime.strptime("2026-03-06", "%Y-%m-%d").timestamp())
        assert result == expected

    def test_unsupported_format_raises_error(self):
        """测试不支持的格式抛出 ValueError"""
        with pytest.raises(ValueError, match="不支持的时间格式"):
            TimeRangeParser.parse_time_expression("invalid-date")

    def test_parse_time_range_with_datetime_strings(self):
        """测试 parse_time_range 使用日期时间字符串"""
        from_ts, to_ts = TimeRangeParser.parse_time_range(
            "2026-03-06 17:30:00",
            "2026-03-06 17:36:00",
        )
        expected_from = int(datetime.strptime("2026-03-06 17:30:00", "%Y-%m-%d %H:%M:%S").timestamp())
        expected_to = int(datetime.strptime("2026-03-06 17:36:00", "%Y-%m-%d %H:%M:%S").timestamp())
        assert from_ts == expected_from
        assert to_ts == expected_to
        assert from_ts < to_ts
