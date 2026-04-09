"""时序统计计算函数 - 单元测试

测试 compute_stats() 函数的正确性，包括：
- 计算 max, min, avg, count
- 计算 max_time, min_time（格式化为 YYYY-MM-DD HH:MM:SS）
- 处理空数据集（返回零值）

**Validates: Requirements 2.1, 2.2, 2.3**
"""

import pytest

from mcp_server_aliyun_observability.toolkits.paas.timeseries_compare import (
    compute_stats,
    TimeSeriesStats,
)


class TestComputeStats:
    """compute_stats() 函数单元测试
    
    **Validates: Requirements 2.1, 2.2, 2.3**
    """

    def test_empty_data_returns_zero_values(self):
        """测试空数据集返回零值
        
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
        
        **Validates: Requirements 2.1**
        """
        # 2024-01-08 10:00:00 UTC = 1704708000 seconds
        ts_ns = 1704708000 * 1_000_000_000
        
        stats = compute_stats([100.0], [ts_ns])
        
        assert stats.max == 100.0
        assert stats.min == 100.0
        assert stats.avg == 100.0
        assert stats.count == 1
        assert stats.max_time == "2024-01-08 10:00:00"
        assert stats.min_time == "2024-01-08 10:00:00"

    def test_multiple_values_basic_stats(self):
        """测试多个数据点的基本统计值
        
        **Validates: Requirements 2.1**
        """
        values = [291.0, 282.0, 298.0]
        timestamps = [
            1767883273 * 1_000_000_000,
            1767883303 * 1_000_000_000,
            1767883333 * 1_000_000_000
        ]
        
        stats = compute_stats(values, timestamps)
        
        assert stats.max == 298.0
        assert stats.min == 282.0
        assert abs(stats.avg - 290.333333) < 0.001
        assert stats.count == 3

    def test_max_min_time_format(self):
        """测试时间格式为 YYYY-MM-DD HH:MM:SS
        
        **Validates: Requirements 2.3**
        """
        # 2024-01-08 10:30:45 UTC = 1704709845 seconds
        ts_ns = 1704709845 * 1_000_000_000
        
        stats = compute_stats([50.0], [ts_ns])
        
        # 验证时间格式
        assert len(stats.max_time) == 19  # YYYY-MM-DD HH:MM:SS = 19 characters
        assert stats.max_time[4] == "-"
        assert stats.max_time[7] == "-"
        assert stats.max_time[10] == " "
        assert stats.max_time[13] == ":"
        assert stats.max_time[16] == ":"

    def test_max_time_corresponds_to_max_value(self):
        """测试 max_time 对应最大值出现的时间
        
        **Validates: Requirements 2.1**
        """
        values = [100.0, 300.0, 200.0]  # max is 300.0 at index 1
        timestamps = [
            1704708000 * 1_000_000_000,  # 2024-01-08 10:00:00
            1704711600 * 1_000_000_000,  # 2024-01-08 11:00:00 (max)
            1704715200 * 1_000_000_000,  # 2024-01-08 12:00:00
        ]
        
        stats = compute_stats(values, timestamps)
        
        assert stats.max == 300.0
        assert stats.max_time == "2024-01-08 11:00:00"

    def test_min_time_corresponds_to_min_value(self):
        """测试 min_time 对应最小值出现的时间
        
        **Validates: Requirements 2.1**
        """
        values = [200.0, 100.0, 300.0]  # min is 100.0 at index 1
        timestamps = [
            1704708000 * 1_000_000_000,  # 2024-01-08 10:00:00
            1704711600 * 1_000_000_000,  # 2024-01-08 11:00:00 (min)
            1704715200 * 1_000_000_000,  # 2024-01-08 12:00:00
        ]
        
        stats = compute_stats(values, timestamps)
        
        assert stats.min == 100.0
        assert stats.min_time == "2024-01-08 11:00:00"

    def test_negative_values(self):
        """测试负数值
        
        **Validates: Requirements 2.1**
        """
        values = [-10.0, -5.0, -20.0]
        timestamps = [
            1704708000 * 1_000_000_000,
            1704711600 * 1_000_000_000,
            1704715200 * 1_000_000_000,
        ]
        
        stats = compute_stats(values, timestamps)
        
        assert stats.max == -5.0
        assert stats.min == -20.0
        assert abs(stats.avg - (-11.666667)) < 0.001
        assert stats.count == 3

    def test_mixed_positive_negative_values(self):
        """测试正负混合值
        
        **Validates: Requirements 2.1**
        """
        values = [-10.0, 0.0, 10.0]
        timestamps = [
            1704708000 * 1_000_000_000,
            1704711600 * 1_000_000_000,
            1704715200 * 1_000_000_000,
        ]
        
        stats = compute_stats(values, timestamps)
        
        assert stats.max == 10.0
        assert stats.min == -10.0
        assert stats.avg == 0.0
        assert stats.count == 3

    def test_all_same_values(self):
        """测试所有值相同的情况
        
        **Validates: Requirements 2.1**
        """
        values = [50.0, 50.0, 50.0]
        timestamps = [
            1704708000 * 1_000_000_000,
            1704711600 * 1_000_000_000,
            1704715200 * 1_000_000_000,
        ]
        
        stats = compute_stats(values, timestamps)
        
        assert stats.max == 50.0
        assert stats.min == 50.0
        assert stats.avg == 50.0
        assert stats.count == 3
        # When all values are the same, max_time should be the first occurrence
        assert stats.max_time == "2024-01-08 10:00:00"
        assert stats.min_time == "2024-01-08 10:00:00"

    def test_values_without_timestamps(self):
        """测试有值但无时间戳的情况
        
        **Validates: Requirements 2.1, 2.2**
        """
        stats = compute_stats([100.0, 200.0], [])
        
        assert stats.max == 200.0
        assert stats.min == 100.0
        assert stats.avg == 150.0
        assert stats.count == 2
        assert stats.max_time == ""
        assert stats.min_time == ""

    def test_large_dataset(self):
        """测试大数据集
        
        **Validates: Requirements 2.1**
        """
        # Generate 1000 values
        values = [float(i) for i in range(1000)]
        base_ts = 1704708000 * 1_000_000_000
        timestamps = [base_ts + i * 60 * 1_000_000_000 for i in range(1000)]
        
        stats = compute_stats(values, timestamps)
        
        assert stats.max == 999.0
        assert stats.min == 0.0
        assert stats.avg == 499.5
        assert stats.count == 1000

    def test_floating_point_precision(self):
        """测试浮点数精度
        
        **Validates: Requirements 2.1**
        """
        values = [0.1, 0.2, 0.3]
        timestamps = [
            1704708000 * 1_000_000_000,
            1704711600 * 1_000_000_000,
            1704715200 * 1_000_000_000,
        ]
        
        stats = compute_stats(values, timestamps)
        
        assert stats.max == 0.3
        assert stats.min == 0.1
        # Average of 0.1, 0.2, 0.3 = 0.2
        assert abs(stats.avg - 0.2) < 0.0001
        assert stats.count == 3


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
