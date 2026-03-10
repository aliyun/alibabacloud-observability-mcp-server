import re
import time
from typing import Tuple, Union


class TimeRangeParser:
    """时间范围解析工具类
    
    支持两种时间格式的解析：
    1. Unix时间戳（整数）
    2. 相对时间表达式（如 "now-1h", "now-30m", "now-1d"）
    """

    @staticmethod
    def parse_time_expression(time_expr: Union[str, int]) -> int:
        """解析时间表达式为Unix时间戳（秒）
        
        Args:
            time_expr: 时间表达式，支持：
                - Unix时间戳（整数，秒或毫秒）
                - 相对时间表达式：now-1h, now-30m, now-1d, now-7d
                
        Returns:
            Unix时间戳（秒）
            
        Examples:
            parse_time_expression(1640995200) -> 1640995200 (秒时间戳)
            parse_time_expression(1640995200000) -> 1640995200 (毫秒转秒)
            parse_time_expression("now-1h") -> 当前时间-1小时的时间戳
            parse_time_expression("now-30m") -> 当前时间-30分钟的时间戳
        """
        # 如果是整数，需要判断是秒还是毫秒时间戳
        if isinstance(time_expr, int):
            return TimeRangeParser._normalize_timestamp(time_expr)
            
        if isinstance(time_expr, str) and time_expr.isdigit():
            return TimeRangeParser._normalize_timestamp(int(time_expr))
        
        # 解析相对时间表达式
        if isinstance(time_expr, str) and time_expr.startswith("now"):
            return TimeRangeParser._parse_relative_time(time_expr)
        
        # 如果都不匹配，尝试直接转换为整数
        try:
            timestamp = int(time_expr)
            return TimeRangeParser._normalize_timestamp(timestamp)
        except (ValueError, TypeError):
            raise ValueError(f"不支持的时间格式: {time_expr}")

    @staticmethod
    def _normalize_timestamp(timestamp: int) -> int:
        """标准化时间戳为秒级
        
        自动判断输入的时间戳是秒还是毫秒，并转换为秒级时间戳
        
        Args:
            timestamp: 时间戳（秒或毫秒）
            
        Returns:
            秒级时间戳
        """
        # 判断是否为毫秒时间戳（通常毫秒时间戳长度为13位，秒级为10位）
        # 2000年1月1日的时间戳约为946684800（10位）
        # 如果时间戳大于这个值的1000倍，则认为是毫秒时间戳
        if timestamp > 946684800000:  # 大于2000年的毫秒时间戳
            return timestamp // 1000
        else:
            return timestamp

    @staticmethod
    def _parse_relative_time(time_expr: str) -> int:
        """解析相对时间表达式
        
        Args:
            time_expr: 相对时间表达式，如 "now-1h", "now-30m"
            
        Returns:
            Unix时间戳（秒）
        """
        now = int(time.time())
        
        # 如果只是 "now"
        if time_expr.strip().lower() == "now":
            return now
            
        # 匹配模式: now-{数字}{单位}
        pattern = r'^now([+-])(\d+)([smhd])$'
        match = re.match(pattern, time_expr.strip().lower())
        
        if not match:
            raise ValueError(f"无效的相对时间格式: {time_expr}. 支持格式: now, now-1h, now-30m, now-1d")
        
        operator, amount_str, unit = match.groups()
        amount = int(amount_str)
        
        # 计算时间偏移（秒）
        unit_multipliers = {
            's': 1,          # 秒
            'm': 60,         # 分钟
            'h': 3600,       # 小时
            'd': 86400,      # 天
        }
        
        if unit not in unit_multipliers:
            raise ValueError(f"不支持的时间单位: {unit}. 支持单位: s, m, h, d")
        
        offset_seconds = amount * unit_multipliers[unit]
        
        # 根据操作符计算最终时间
        if operator == '-':
            return now - offset_seconds
        else:  # operator == '+'
            return now + offset_seconds

    @staticmethod
    def parse_time_range(from_time: Union[str, int], to_time: Union[str, int]) -> Tuple[int, int]:
        """解析时间范围
        
        Args:
            from_time: 开始时间表达式
            to_time: 结束时间表达式
            
        Returns:
            (开始时间戳, 结束时间戳) 的元组
            
        Examples:
            parse_time_range("now-1h", "now") -> (当前时间-1小时, 当前时间)
            parse_time_range(1640995200, 1640998800) -> (1640995200, 1640998800)
        """
        from_timestamp = TimeRangeParser.parse_time_expression(from_time)
        to_timestamp = TimeRangeParser.parse_time_expression(to_time)
        
        # 确保时间范围有效
        if from_timestamp >= to_timestamp:
            raise ValueError(f"开始时间({from_timestamp})必须小于结束时间({to_timestamp})")
        
        return from_timestamp, to_timestamp

    @staticmethod
    def get_default_time_range(duration_minutes: int = 15) -> Tuple[int, int]:
        """获取默认时间范围
        
        Args:
            duration_minutes: 时间范围长度（分钟），默认15分钟
            
        Returns:
            (开始时间戳, 结束时间戳) 的元组
        """
        now = int(time.time())
        from_time = now - (duration_minutes * 60)
        return from_time, now