import re
import time
from datetime import datetime, timezone, timedelta
from typing import Tuple, Union, Optional

# ============================================================================
# 常量定义
# ============================================================================

# 时间单位到秒的映射
UNIT_SECONDS: dict[str, int] = {
    's': 1,              # 秒
    'm': 60,             # 分钟
    'h': 3600,           # 小时
    'd': 86400,          # 天
    'w': 604800,         # 周 (7 * 86400)
    'M': 2592000,        # 月 (30 * 86400)
    'y': 31536000,       # 年 (365 * 86400)
}

# 正则表达式模式
# 相对预设：last_5m, last_1h, last_3d
RE_RELATIVE_PRESET = re.compile(r'^last_(\d+)([smhdwMy])$', re.IGNORECASE)

# 简单时长：1h, 30m, 7d
RE_SIMPLE_DURATION = re.compile(r'^(\d+)([smhdwMy])$', re.IGNORECASE)

# Grafana 风格：now-15m, now+1h
RE_GRAFANA_STYLE = re.compile(r'^now([+-])(\d+)([smhdwMy])$', re.IGNORECASE)

# Unix 时间戳：10/13/16/19 位数字
RE_TIMESTAMP = re.compile(r'^\d{10,19}$')

# 日期时间格式（按优先级排序）
DATETIME_FORMATS = [
    '%Y-%m-%d %H:%M:%S',  # 2024-02-02 10:10:10
    '%Y-%m-%d %H:%M',     # 2024-02-02 10:10
    '%Y-%m-%d',           # 2024-02-02
]

# 时间戳精度映射（位数 -> 除数）
TIMESTAMP_DIVISORS: dict[int, int] = {
    10: 1,           # 秒
    13: 1000,        # 毫秒
    16: 1000000,     # 微秒
    19: 1000000000,  # 纳秒
}


# ============================================================================
# 辅助函数
# ============================================================================

def _unit_to_seconds(amount: int, unit: str) -> int:
    """将时间单位转换为秒数。
    
    Args:
        amount: 数量
        unit: 单位（s/m/h/d/w/M/y）
        
    Returns:
        总秒数
        
    Raises:
        ValueError: 不支持的单位
    """
    # 处理大小写：M 表示月，m 表示分钟
    if unit == 'M':
        normalized_unit = 'M'
    else:
        normalized_unit = unit.lower()
    
    if normalized_unit not in UNIT_SECONDS:
        raise ValueError(f"不支持的时间单位: {unit}，支持: s, m, h, d, w, M, y")
    
    return amount * UNIT_SECONDS[normalized_unit]


def _start_of_day(dt: datetime) -> datetime:
    """获取给定日期的午夜时间。
    
    Args:
        dt: 日期时间对象
        
    Returns:
        当天 00:00:00 的 datetime 对象（保留时区信息）
    """
    return dt.replace(hour=0, minute=0, second=0, microsecond=0)


def _normalize_timestamp_enhanced(timestamp: int) -> int:
    """标准化时间戳为秒级（增强版，支持微秒和纳秒）。
    
    根据位数自动判断精度：
    - 10位：秒
    - 13位：毫秒
    - 16位：微秒
    - 19位：纳秒
    
    Args:
        timestamp: 时间戳（秒/毫秒/微秒/纳秒）
        
    Returns:
        秒级时间戳
    """
    ts_str = str(abs(timestamp))
    length = len(ts_str)
    
    # 根据位数确定除数
    if length <= 10:
        return timestamp
    elif length <= 13:
        return timestamp // 1000
    elif length <= 16:
        return timestamp // 1000000
    else:  # 19位或更长
        return timestamp // 1000000000


# ============================================================================
# 单点时间解析器
# ============================================================================

def _parse_grafana_style(expr: str, now: datetime) -> int:
    """解析 Grafana 风格表达式（now±Xu）。
    
    Args:
        expr: 如 "now-15m", "now+1h", "now-1w", "now-1M"
        now: 参考时间
        
    Returns:
        秒级 Unix 时间戳
        
    Raises:
        ValueError: 格式无效
    """
    # 处理单独的 "now"
    if expr.strip().lower() == 'now':
        return int(now.timestamp())
    
    match = RE_GRAFANA_STYLE.match(expr.strip())
    if not match:
        raise ValueError(f"无效的 Grafana 风格表达式: {expr}")
    
    operator, amount_str, unit = match.groups()
    amount = int(amount_str)
    
    offset_seconds = _unit_to_seconds(amount, unit)
    now_ts = int(now.timestamp())
    
    if operator == '-':
        return now_ts - offset_seconds
    else:  # operator == '+'
        return now_ts + offset_seconds


def _parse_timestamp(expr: str) -> int:
    """解析 Unix 时间戳字符串。
    
    根据位数自动判断精度：
    - 10位：秒
    - 13位：毫秒
    - 16位：微秒
    - 19位：纳秒
    
    Args:
        expr: 纯数字字符串
        
    Returns:
        秒级 Unix 时间戳
        
    Raises:
        ValueError: 不是有效的时间戳格式
    """
    expr = expr.strip()
    if not RE_TIMESTAMP.match(expr):
        raise ValueError(f"无效的时间戳格式: {expr}")
    
    timestamp = int(expr)
    return _normalize_timestamp_enhanced(timestamp)


def _parse_datetime_string(expr: str, now: datetime) -> int:
    """解析人类可读日期时间字符串。
    
    支持格式：
    - YYYY-MM-DD HH:MM:SS
    - YYYY-MM-DD HH:MM
    - YYYY-MM-DD
    
    Args:
        expr: 日期时间字符串
        now: 参考时间（用于获取时区）
        
    Returns:
        秒级 Unix 时间戳
        
    Raises:
        ValueError: 无法解析的格式
    """
    expr = expr.strip()
    tz = now.tzinfo
    
    for fmt in DATETIME_FORMATS:
        try:
            dt = datetime.strptime(expr, fmt)
            # 应用参考时间的时区
            if tz is not None:
                dt = dt.replace(tzinfo=tz)
            return int(dt.timestamp())
        except ValueError:
            continue
    
    raise ValueError(f"无法解析日期时间字符串: {expr}，支持格式: YYYY-MM-DD, YYYY-MM-DD HH:MM, YYYY-MM-DD HH:MM:SS")


def _parse_time_point(expr: str, now: datetime) -> int:
    """解析单个时间点表达式为 Unix 时间戳。
    
    支持格式（按优先级）：
    1. now, now-1h, now+30m（Grafana 风格）
    2. 1706864400（Unix 时间戳）
    3. 2024-02-02 10:10:10（日期时间字符串）
    
    Args:
        expr: 时间点表达式
        now: 参考时间
        
    Returns:
        秒级 Unix 时间戳
        
    Raises:
        ValueError: 无法解析的表达式
    """
    expr = expr.strip()
    
    # 1. 尝试 Grafana 风格（now, now-1h, now+30m）
    if expr.lower().startswith('now'):
        return _parse_grafana_style(expr, now)
    
    # 2. 尝试 Unix 时间戳
    if RE_TIMESTAMP.match(expr):
        return _parse_timestamp(expr)
    
    # 3. 尝试日期时间字符串
    try:
        return _parse_datetime_string(expr, now)
    except ValueError:
        pass
    
    raise ValueError(f"无法解析时间点表达式: {expr}")


# ============================================================================
# 完整表达式解析器
# ============================================================================

def _parse_relative_preset(expr: str, now: datetime) -> Tuple[int, int]:
    """解析相对预设表达式（last_Xu 格式）。
    
    Args:
        expr: 如 "last_5m", "last_1h", "last_3d"
        now: 参考时间
        
    Returns:
        (from_timestamp, to_timestamp) 元组
        
    Raises:
        ValueError: 格式无效或数量为零/负数
    """
    match = RE_RELATIVE_PRESET.match(expr.strip())
    if not match:
        raise ValueError(f"无效的相对预设表达式: {expr}")
    
    amount_str, unit = match.groups()
    amount = int(amount_str)
    
    if amount <= 0:
        raise ValueError(f"时长必须为正数: {amount}")
    
    offset_seconds = _unit_to_seconds(amount, unit)
    now_ts = int(now.timestamp())
    
    return now_ts - offset_seconds, now_ts


def _parse_simple_duration(expr: str, now: datetime) -> Tuple[int, int]:
    """解析简单时长表达式（Nu 格式）。
    
    Args:
        expr: 如 "1h", "30m", "7d"
        now: 参考时间
        
    Returns:
        (from_timestamp, to_timestamp) 元组
        
    Raises:
        ValueError: 格式无效或数量为零/负数
    """
    match = RE_SIMPLE_DURATION.match(expr.strip())
    if not match:
        raise ValueError(f"无效的简单时长表达式: {expr}")
    
    amount_str, unit = match.groups()
    amount = int(amount_str)
    
    if amount <= 0:
        raise ValueError(f"时长必须为正数: {amount}")
    
    offset_seconds = _unit_to_seconds(amount, unit)
    now_ts = int(now.timestamp())
    
    return now_ts - offset_seconds, now_ts


def _parse_keyword(expr: str, now: datetime) -> Tuple[int, int]:
    """解析关键字表达式。
    
    Args:
        expr: "today", "yesterday", "now"
        now: 参考时间
        
    Returns:
        (from_timestamp, to_timestamp) 元组
    """
    keyword = expr.strip().lower()
    now_ts = int(now.timestamp())
    
    if keyword == 'today':
        start = _start_of_day(now)
        return int(start.timestamp()), now_ts
    
    elif keyword == 'yesterday':
        start_today = _start_of_day(now)
        start_yesterday = start_today - timedelta(days=1)
        return int(start_yesterday.timestamp()), int(start_today.timestamp())
    
    elif keyword == 'now':
        # 单独的 "now" 默认返回最近 1 小时
        return now_ts - 3600, now_ts
    
    raise ValueError(f"无法识别的关键字: {expr}")


# ============================================================================
# 统一 API
# ============================================================================

def compute_time_range(
    expr: Optional[str],
    now: Optional[datetime] = None
) -> Tuple[int, int]:
    """解析时间范围表达式并返回 Unix 时间戳元组。
    
    支持多种格式：
    - 相对预设：last_5m, last_1h, last_3d, last_1w, last_1M, last_1y
    - 简单时长：1h, 30m, 7d
    - Grafana 风格范围：now-15m~now-5m
    - 关键字：today, yesterday, now
    - 绝对时间戳范围：1706864400~1706868000
    - 人类可读范围：2024-02-02 10:10:10~2024-02-02 10:20:10
    
    Args:
        expr: 时间范围表达式，支持多种格式
        now: 参考时间，默认为当前系统时间
        
    Returns:
        (from_timestamp, to_timestamp) 秒级 Unix 时间戳元组
        
    Raises:
        ValueError: 表达式无效或时间范围顺序错误
    """
    # 设置默认参考时间
    if now is None:
        now = datetime.now()
    
    # 处理空值/None，默认为 last_1h
    if expr is None or expr.strip() == '':
        expr = 'last_1h'
    
    expr = expr.strip()
    
    # 检查是否为范围表达式（包含 ~）
    if '~' in expr:
        parts = expr.split('~', 1)
        if len(parts) != 2:
            raise ValueError(f"无效的范围表达式: {expr}")
        
        left = parts[0].strip()
        right = parts[1].strip()
        
        # 解析左侧
        try:
            from_ts = _parse_time_point(left, now)
        except ValueError as e:
            raise ValueError(f"范围表达式左侧无效: {left}") from e
        
        # 解析右侧
        try:
            to_ts = _parse_time_point(right, now)
        except ValueError as e:
            raise ValueError(f"范围表达式右侧无效: {right}") from e
        
        # 验证时间顺序
        if from_ts > to_ts:
            raise ValueError(f"开始时间({from_ts})必须小于结束时间({to_ts})")
        
        return from_ts, to_ts
    
    # 尝试相对预设（last_Xu）
    if RE_RELATIVE_PRESET.match(expr):
        return _parse_relative_preset(expr, now)
    
    # 尝试简单时长（Nu）
    if RE_SIMPLE_DURATION.match(expr):
        return _parse_simple_duration(expr, now)
    
    # 尝试关键字（today, yesterday, now）
    keyword = expr.lower()
    if keyword in ('today', 'yesterday', 'now'):
        return _parse_keyword(expr, now)
    
    # 尝试单个时间点（返回 [point, now]）
    try:
        point_ts = _parse_time_point(expr, now)
        now_ts = int(now.timestamp())
        return point_ts, now_ts
    except ValueError:
        pass
    
    raise ValueError(f"无法解析时间表达式: {expr}")


def format_timestamp(
    timestamp: int,
    tz: Optional[timezone] = None
) -> str:
    """将 Unix 时间戳格式化为人类可读字符串。
    
    Args:
        timestamp: 秒级 Unix 时间戳
        tz: 目标时区，默认为本地时区
        
    Returns:
        格式为 "YYYY-MM-DD HH:MM:SS" 的字符串
    """
    dt = datetime.fromtimestamp(timestamp, tz=tz)
    return dt.strftime('%Y-%m-%d %H:%M:%S')


# ============================================================================
# 向后兼容的 TimeRangeParser 类
# ============================================================================

class TimeRangeParser:
    """时间范围解析工具类（向后兼容）
    
    支持多种时间格式的解析：
    1. Unix时间戳（整数，支持秒/毫秒/微秒/纳秒）
    2. 相对时间表达式（如 "now-1h", "now-30m", "now-1d"）
    3. 相对预设（如 "last_5m", "last_1h"）
    4. 关键字（如 "today", "yesterday"）
    5. 日期时间字符串（如 "2024-02-02 10:10:10"）
    """

    @staticmethod
    def parse_time_expression(time_expr: Union[str, int]) -> int:
        """解析时间表达式为Unix时间戳（秒）
        
        Args:
            time_expr: 时间表达式，支持：
                - Unix时间戳（整数，秒/毫秒/微秒/纳秒）
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
        
        # 使用新的解析器
        now = datetime.now()
        return _parse_time_point(str(time_expr), now)

    @staticmethod
    def _normalize_timestamp(timestamp: int) -> int:
        """标准化时间戳为秒级（增强版，支持微秒和纳秒）
        
        根据位数自动判断精度：
        - 10位：秒
        - 13位：毫秒
        - 16位：微秒
        - 19位：纳秒
        
        Args:
            timestamp: 时间戳（秒/毫秒/微秒/纳秒）
            
        Returns:
            秒级时间戳
        """
        return _normalize_timestamp_enhanced(timestamp)

    @staticmethod
    def _parse_relative_time(time_expr: str) -> int:
        """解析相对时间表达式（向后兼容）
        
        Args:
            time_expr: 相对时间表达式，如 "now-1h", "now-30m"
            
        Returns:
            Unix时间戳（秒）
        """
        now = datetime.now()
        return _parse_grafana_style(time_expr, now)

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
