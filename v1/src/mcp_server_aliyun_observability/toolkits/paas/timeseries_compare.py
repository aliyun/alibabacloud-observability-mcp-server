"""时序数据对比模块

本模块提供时序数据对比功能，包括：
- 时序数据的统计计算（max, min, avg, count, max_time, min_time）
- 变化趋势分析（up, down, stable, new, disappeared）
- 差异评分计算（diff_score）
- 标准化对比输出格式

参考实现：
- Go 实现：umodel-mcp/umodel-mcp-server/internal/utils/timeseries_compare_test.go
"""

from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional

# ============================================================================
# 常量定义
# ============================================================================


class KeyType:
    """时序数据类型"""

    METRICS = "metrics"
    GOLDEN_METRICS = "golden_metrics"


class Trend:
    """变化趋势常量"""

    UP = "up"
    DOWN = "down"
    STABLE = "stable"
    NEW = "new"
    DISAPPEARED = "disappeared"


# 趋势判断阈值（百分比）
TREND_THRESHOLD_PERCENT = 5.0


# ============================================================================
# 数据模型
# ============================================================================


@dataclass
class TimeSeriesKey:
    """时序数据的唯一标识

    用于匹配当前时段和对比时段的时序数据。

    Attributes:
        entity_id: 实体 ID
        labels: 标签（JSON 字符串格式）
        metric: 指标名称
        metric_set_id: 指标集 ID
    """

    entity_id: str = ""
    labels: str = "{}"
    metric: str = ""
    metric_set_id: str = ""

    def hash(self) -> str:
        """生成唯一哈希值用于时序匹配

        哈希值基于 metric、metric_set_id、entity_id 和 labels 组合生成，
        用于在当前时段和对比时段之间匹配相同的时序数据。

        Returns:
            唯一标识字符串，格式为 "metric|metric_set_id|entity_id|labels"
        """
        return f"{self.metric}|{self.metric_set_id}|{self.entity_id}|{self.labels}"


@dataclass
class TimeSeriesStats:
    """时序统计值

    包含时序数据的统计摘要信息。

    Attributes:
        max: 最大值
        min: 最小值
        avg: 平均值
        count: 数据点数量
        max_time: 最大值出现的时间（格式: YYYY-MM-DD HH:MM:SS）
        min_time: 最小值出现的时间（格式: YYYY-MM-DD HH:MM:SS）
    """

    max: float = 0.0
    min: float = 0.0
    avg: float = 0.0
    count: int = 0
    max_time: str = ""
    min_time: str = ""


@dataclass
class DiffDetails:
    """差异详情

    包含两个时段数据的差异分析结果。

    Attributes:
        trend: 变化趋势（up/down/stable/new/disappeared）
        avg_change: 平均值变化量
        avg_change_percent: 平均值变化百分比
        max_change: 最大值变化量
        min_change: 最小值变化量
    """

    trend: str = Trend.STABLE
    avg_change: float = 0.0
    avg_change_percent: float = 0.0
    max_change: float = 0.0
    min_change: float = 0.0


@dataclass
class TimeSeriesData:
    """单个时序数据

    包含时序数据的完整信息，包括标识、统计值和原始数据点。

    Attributes:
        key: 时序标识
        stats: 统计值
        values: 数值列表
        timestamps: 时间戳列表（纳秒）
    """

    key: TimeSeriesKey
    stats: TimeSeriesStats
    values: List[float] = field(default_factory=list)
    timestamps: List[int] = field(default_factory=list)


@dataclass
class TimeSeriesCompareResult:
    """单个时序的对比结果

    包含单个时序在两个时段的对比分析结果。

    Attributes:
        key: 时序标识
        current_stats: 当前时段统计值
        compare_stats: 对比时段统计值（可为 None，表示新增时序）
        diff_score: 差异评分（0-1）
        diff_details: 详细差异信息
    """

    key: TimeSeriesKey
    current_stats: TimeSeriesStats
    compare_stats: Optional[TimeSeriesStats] = None
    diff_score: float = 0.0
    diff_details: DiffDetails = field(default_factory=DiffDetails)


@dataclass
class TimeRangeInfo:
    """时间范围信息

    包含时间范围的人类可读格式和 Unix 时间戳。

    Attributes:
        from_time: 人类可读的开始时间（格式: YYYY-MM-DD HH:MM:SS）
        to_time: 人类可读的结束时间（格式: YYYY-MM-DD HH:MM:SS）
        from_unix: 开始时间的 Unix 时间戳（秒）
        to_unix: 结束时间的 Unix 时间戳（秒）
    """

    from_time: str = ""
    to_time: str = ""
    from_unix: int = 0
    to_unix: int = 0


@dataclass
class CompareOutput:
    """对比输出结构

    包含完整的对比分析结果，用于标准化输出。

    Attributes:
        compare_enabled: 是否启用了对比
        current_time_range: 当前时段的时间范围信息
        compare_time_range: 对比时段的时间范围信息（如果启用）
        offset: 偏移量字符串（如 "3600s"）
        total_series: 时序数据总数
        results: 对比结果数组
    """

    compare_enabled: bool = False
    current_time_range: TimeRangeInfo = field(default_factory=TimeRangeInfo)
    compare_time_range: Optional[TimeRangeInfo] = None
    offset: str = ""
    total_series: int = 0
    results: List[TimeSeriesCompareResult] = field(default_factory=list)


# ============================================================================
# 核心函数
# ============================================================================


def _format_timestamp_ns(timestamp_ns: int) -> str:
    """将纳秒时间戳格式化为人类可读格式

    Args:
        timestamp_ns: 纳秒时间戳

    Returns:
        格式化的时间字符串（YYYY-MM-DD HH:MM:SS）
    """
    from datetime import datetime, timezone

    if timestamp_ns <= 0:
        return ""

    # 将纳秒转换为秒
    timestamp_s = timestamp_ns // 1_000_000_000

    # 转换为 datetime 对象（UTC）
    dt = datetime.fromtimestamp(timestamp_s, tz=timezone.utc)

    # 格式化为 YYYY-MM-DD HH:MM:SS
    return dt.strftime("%Y-%m-%d %H:%M:%S")


def parse_duration_to_seconds(duration: str) -> int:
    """解析时间偏移量字符串为秒数

    支持的格式：
    - s: 秒（如 "30s" -> 30）
    - m: 分钟（如 "30m" -> 1800）
    - h: 小时（如 "1h" -> 3600）
    - d: 天（如 "1d" -> 86400）
    - w: 周（如 "1w" -> 604800）

    Args:
        duration: 时间偏移量字符串，格式为 "数字+单位"，如 "30m", "1h", "1d"

    Returns:
        对应的秒数。如果格式无效，返回 0。

    Examples:
        >>> parse_duration_to_seconds("30m")
        1800
        >>> parse_duration_to_seconds("1h")
        3600
        >>> parse_duration_to_seconds("1d")
        86400
        >>> parse_duration_to_seconds("1w")
        604800
        >>> parse_duration_to_seconds("invalid")
        0

    Note:
        - 数字部分必须为正整数
        - 单位不区分大小写
        - 空字符串或 None 返回 0
    """
    import re

    # 处理空值或非字符串
    if not duration or not isinstance(duration, str):
        return 0

    # 去除空白并转为小写
    duration = duration.strip().lower()

    # 空字符串返回 0
    if not duration:
        return 0

    # 定义单位到秒的映射
    unit_to_seconds = {
        "s": 1,  # 秒
        "m": 60,  # 分钟
        "h": 3600,  # 小时
        "d": 86400,  # 天
        "w": 604800,  # 周
    }

    # 使用正则表达式匹配格式：数字 + 单位
    pattern = r"^(\d+)([smhdw])$"
    match = re.match(pattern, duration)

    if not match:
        return 0

    # 提取数字和单位
    value = int(match.group(1))
    unit = match.group(2)

    # 数字为 0 时返回 0
    if value == 0:
        return 0

    # 计算秒数
    return value * unit_to_seconds[unit]


def compute_stats(values: List[float], timestamps: List[int]) -> TimeSeriesStats:
    """计算时序统计值

    计算给定时序数据的统计摘要，包括最大值、最小值、平均值、
    数据点数量以及最大/最小值出现的时间。

    Args:
        values: 数值列表
        timestamps: 时间戳列表（纳秒）

    Returns:
        统计值对象，包含 max, min, avg, count, max_time, min_time

    Note:
        - 当数据集为空时，返回零值（max=0, min=0, avg=0, count=0）
        - 时间格式为 YYYY-MM-DD HH:MM:SS
        - timestamps 应与 values 长度相同，否则时间信息可能不准确
    """
    # 处理空数据集
    if not values:
        return TimeSeriesStats(
            max=0.0, min=0.0, avg=0.0, count=0, max_time="", min_time=""
        )

    count = len(values)

    # 计算 max, min, avg
    max_val = max(values)
    min_val = min(values)
    avg_val = sum(values) / count

    # 找到 max 和 min 对应的索引
    max_idx = values.index(max_val)
    min_idx = values.index(min_val)

    # 获取对应的时间戳并格式化
    max_time = ""
    min_time = ""

    if timestamps and len(timestamps) > max_idx:
        max_time = _format_timestamp_ns(timestamps[max_idx])

    if timestamps and len(timestamps) > min_idx:
        min_time = _format_timestamp_ns(timestamps[min_idx])

    return TimeSeriesStats(
        max=max_val,
        min=min_val,
        avg=avg_val,
        count=count,
        max_time=max_time,
        min_time=min_time,
    )


def analyze_trend(
    current_stats: TimeSeriesStats, compare_stats: Optional[TimeSeriesStats]
) -> DiffDetails:
    """分析变化趋势

    分析当前时段和对比时段的统计数据，计算变化量和变化百分比，
    并判断变化趋势。

    Args:
        current_stats: 当前时段统计
        compare_stats: 对比时段统计（可为 None）

    Returns:
        差异详情（包含趋势）

    趋势判断规则：
        - new: 当前时段有数据但对比时段无数据
        - disappeared: 当前时段无数据但对比时段有数据
        - stable: avg_change_percent 绝对值 < 5%
        - up: avg_change_percent > 5%
        - down: avg_change_percent < -5%

    Note:
        - 变化量 = 当前值 - 对比值
        - 变化百分比 = (当前值 - 对比值) / 对比值 * 100
        - 当对比值为 0 时，如果当前值也为 0，则变化百分比为 0；
          否则变化百分比为 100%（表示从无到有）
    """
    # 判断数据是否存在（count > 0 表示有数据）
    current_has_data = current_stats.count > 0
    compare_has_data = compare_stats is not None and compare_stats.count > 0

    # 场景 1: 当前有数据，对比无数据 -> new
    if current_has_data and not compare_has_data:
        return DiffDetails(
            trend=Trend.NEW,
            avg_change=current_stats.avg,
            avg_change_percent=100.0,  # 从无到有，视为 100% 增长
            max_change=current_stats.max,
            min_change=current_stats.min,
        )

    # 场景 2: 当前无数据，对比有数据 -> disappeared
    if not current_has_data and compare_has_data:
        return DiffDetails(
            trend=Trend.DISAPPEARED,
            avg_change=-compare_stats.avg,
            avg_change_percent=-100.0,  # 从有到无，视为 -100% 变化
            max_change=-compare_stats.max,
            min_change=-compare_stats.min,
        )

    # 场景 3: 两者都无数据 -> stable（无变化）
    if not current_has_data and not compare_has_data:
        return DiffDetails(
            trend=Trend.STABLE,
            avg_change=0.0,
            avg_change_percent=0.0,
            max_change=0.0,
            min_change=0.0,
        )

    # 场景 4: 两者都有数据 -> 计算变化量和趋势
    # 计算变化量
    avg_change = current_stats.avg - compare_stats.avg
    max_change = current_stats.max - compare_stats.max
    min_change = current_stats.min - compare_stats.min

    # 计算变化百分比
    if compare_stats.avg == 0:
        # 对比值为 0 时的特殊处理
        if current_stats.avg == 0:
            avg_change_percent = 0.0
        else:
            # 从 0 变为非 0，视为 100% 增长（如果是正数）或 -100%（如果是负数）
            avg_change_percent = 100.0 if current_stats.avg > 0 else -100.0
    else:
        avg_change_percent = (avg_change / abs(compare_stats.avg)) * 100.0

    # 判断趋势
    # Requirements:
    # - stable: |avg_change_percent| < 5% (strictly less than)
    # - up: avg_change_percent > 5% (strictly greater than)
    # - down: avg_change_percent < -5% (strictly less than)
    # Note: exactly 5% or -5% should be stable
    if avg_change_percent > TREND_THRESHOLD_PERCENT:
        trend = Trend.UP
    elif avg_change_percent < -TREND_THRESHOLD_PERCENT:
        trend = Trend.DOWN
    else:
        # This covers: -5% <= avg_change_percent <= 5%
        trend = Trend.STABLE

    return DiffDetails(
        trend=trend,
        avg_change=avg_change,
        avg_change_percent=avg_change_percent,
        max_change=max_change,
        min_change=min_change,
    )


# ============================================================================
# 时序对比核心函数
# ============================================================================


def calculate_diff_score(
    current_stats: TimeSeriesStats, compare_stats: Optional[TimeSeriesStats]
) -> float:
    """计算差异评分

    计算当前时段和对比时段统计数据的差异评分，用于排序和识别显著变化。
    评分范围为 0-1，值越大表示变化越显著。

    Args:
        current_stats: 当前时段统计
        compare_stats: 对比时段统计（可为 None）

    Returns:
        差异评分（0-1），值越大表示变化越显著

    Note:
        - 新增或消失的时序评分为 1.0（最高）
        - 评分基于 avg、max、min 的变化率加权计算
        - 权重：avg=0.5, max=0.3, min=0.2
    """
    # 新增或消失的时序，评分最高
    current_has_data = current_stats.count > 0
    compare_has_data = compare_stats is not None and compare_stats.count > 0

    if current_has_data != compare_has_data:
        return 1.0

    # 两者都无数据
    if not current_has_data and not compare_has_data:
        return 0.0

    # 计算各指标的变化率
    def safe_change_rate(current: float, compare: float) -> float:
        if compare == 0:
            return 1.0 if current != 0 else 0.0
        return (current - compare) / abs(compare)

    avg_change_rate = safe_change_rate(current_stats.avg, compare_stats.avg)
    max_change_rate = safe_change_rate(current_stats.max, compare_stats.max)
    min_change_rate = safe_change_rate(current_stats.min, compare_stats.min)

    # 加权计算评分
    AVG_WEIGHT = 0.5
    MAX_WEIGHT = 0.3
    MIN_WEIGHT = 0.2

    score = (
        abs(avg_change_rate) * AVG_WEIGHT
        + abs(max_change_rate) * MAX_WEIGHT
        + abs(min_change_rate) * MIN_WEIGHT
    )

    # 归一化到 0-1 范围（上限为 200% 变化）
    if score > 2.0:
        score = 2.0
    return score / 2.0


def parse_time_series_data(
    data: List[Dict[str, Any]], key_type: str = KeyType.METRICS
) -> List[TimeSeriesData]:
    """解析查询结果为时序数据列表

    将 API 返回的原始数据解析为 TimeSeriesData 对象列表。

    Args:
        data: API 返回的数据列表，每个元素是一个字典
        key_type: 键类型，决定如何构建唯一标识
            - KeyType.METRICS: 使用 entity_id + labels
            - KeyType.GOLDEN_METRICS: 使用 metric + metric_set_id + entity_id + labels

    Returns:
        TimeSeriesData 对象列表

    Note:
        - __value__ 字段应为数值数组
        - __ts__ 字段应为纳秒时间戳数组
        - __entity_id__ 字段为实体 ID
        - __labels__ 字段为标签（JSON 字符串或字典）
        - metric 和 metric_set_id 字段仅在 golden_metrics 模式下使用
    """
    import json

    results = []

    for row in data:
        # 构建 key
        entity_id = str(row.get("__entity_id__", ""))

        # 处理 labels
        labels_raw = row.get("__labels__", "{}")
        if isinstance(labels_raw, dict):
            labels = json.dumps(labels_raw, sort_keys=True)
        else:
            labels = str(labels_raw) if labels_raw else "{}"

        key = TimeSeriesKey(
            entity_id=entity_id,
            labels=labels,
            metric=(
                str(row.get("metric", "")) if key_type == KeyType.GOLDEN_METRICS else ""
            ),
            metric_set_id=(
                str(row.get("metric_set_id", ""))
                if key_type == KeyType.GOLDEN_METRICS
                else ""
            ),
        )

        # 解析 values 和 timestamps
        values = _parse_float_array(row.get("__value__"))
        timestamps = _parse_int64_array(row.get("__ts__"))

        # 计算统计值
        stats = compute_stats(values, timestamps)

        ts_data = TimeSeriesData(
            key=key, stats=stats, values=values, timestamps=timestamps
        )
        results.append(ts_data)

    return results


def _parse_float_array(value: Any) -> List[float]:
    """解析值为浮点数数组

    Args:
        value: 原始值，可能是列表、字符串或其他类型

    Returns:
        浮点数列表
    """
    import json
    import math

    if value is None:
        return []

    if isinstance(value, list):
        result = []
        for v in value:
            try:
                f = float(v)
                if not math.isnan(f) and not math.isinf(f):
                    result.append(f)
            except (ValueError, TypeError):
                pass
        return result

    if isinstance(value, str):
        try:
            parsed = json.loads(value)
            if isinstance(parsed, list):
                return _parse_float_array(parsed)
        except json.JSONDecodeError:
            pass

    return []


def _parse_int64_array(value: Any) -> List[int]:
    """解析值为整数数组（纳秒时间戳）

    Args:
        value: 原始值，可能是列表、字符串或其他类型

    Returns:
        整数列表
    """
    import json

    if value is None:
        return []

    if isinstance(value, list):
        result = []
        for v in value:
            try:
                result.append(int(v))
            except (ValueError, TypeError):
                pass
        return result

    if isinstance(value, str):
        try:
            parsed = json.loads(value)
            if isinstance(parsed, list):
                return _parse_int64_array(parsed)
        except json.JSONDecodeError:
            pass

    return []


def compare_time_series(
    current_data: List[TimeSeriesData], compare_data: List[TimeSeriesData]
) -> List[TimeSeriesCompareResult]:
    """对比当前时段和历史时段的时序数据

    将当前时段和对比时段的时序数据进行匹配和对比分析。

    Args:
        current_data: 当前时段的时序数据列表
        compare_data: 对比时段的时序数据列表

    Returns:
        对比结果列表，包含每个时序的对比分析

    Note:
        - 使用 key.hash() 进行时序匹配
        - 新增时序（仅在当前时段存在）标记为 "new"
        - 消失时序（仅在对比时段存在）标记为 "disappeared"
    """
    # 构建对比数据的哈希映射
    compare_map: Dict[str, TimeSeriesData] = {}
    for ts in compare_data:
        compare_map[ts.key.hash()] = ts

    # 跟踪已匹配的对比数据
    matched_compare: Dict[str, bool] = {}

    results: List[TimeSeriesCompareResult] = []

    # 处理当前时段数据
    for current in current_data:
        hash_key = current.key.hash()

        compare_ts = compare_map.get(hash_key)
        if compare_ts:
            matched_compare[hash_key] = True
            compare_stats = compare_ts.stats
        else:
            compare_stats = None

        # 计算差异详情和评分
        diff_details = analyze_trend(current.stats, compare_stats)
        diff_score = calculate_diff_score(current.stats, compare_stats)

        result = TimeSeriesCompareResult(
            key=current.key,
            current_stats=current.stats,
            compare_stats=compare_stats,
            diff_score=diff_score,
            diff_details=diff_details,
        )
        results.append(result)

    # 添加消失的时序（仅在对比时段存在）
    for compare_ts in compare_data:
        hash_key = compare_ts.key.hash()
        if hash_key not in matched_compare:
            # 创建空的当前统计
            empty_stats = TimeSeriesStats()
            diff_details = analyze_trend(empty_stats, compare_ts.stats)
            diff_score = calculate_diff_score(empty_stats, compare_ts.stats)

            result = TimeSeriesCompareResult(
                key=compare_ts.key,
                current_stats=empty_stats,
                compare_stats=compare_ts.stats,
                diff_score=diff_score,
                diff_details=diff_details,
            )
            results.append(result)

    return results


def sort_by_diff_score(
    results: List[TimeSeriesCompareResult],
) -> List[TimeSeriesCompareResult]:
    """按差异评分降序排序

    Args:
        results: 对比结果列表

    Returns:
        排序后的结果列表（评分高的在前）
    """
    return sorted(results, key=lambda x: x.diff_score, reverse=True)


def format_time_range(from_ts: int, to_ts: int) -> TimeRangeInfo:
    """格式化时间范围

    Args:
        from_ts: 开始时间戳（秒）
        to_ts: 结束时间戳（秒）

    Returns:
        TimeRangeInfo 对象
    """
    from datetime import datetime, timezone

    from_dt = datetime.fromtimestamp(from_ts, tz=timezone.utc)
    to_dt = datetime.fromtimestamp(to_ts, tz=timezone.utc)

    return TimeRangeInfo(
        from_time=from_dt.strftime("%Y-%m-%d %H:%M:%S"),
        to_time=to_dt.strftime("%Y-%m-%d %H:%M:%S"),
        from_unix=from_ts,
        to_unix=to_ts,
    )


def build_compare_output(
    current_data: List[TimeSeriesData],
    compare_data: List[TimeSeriesData],
    current_from: int,
    current_to: int,
    compare_from: int,
    compare_to: int,
    offset_seconds: int,
) -> CompareOutput:
    """构建完整的对比输出

    Args:
        current_data: 当前时段的时序数据
        compare_data: 对比时段的时序数据
        current_from: 当前时段开始时间戳（秒）
        current_to: 当前时段结束时间戳（秒）
        compare_from: 对比时段开始时间戳（秒）
        compare_to: 对比时段结束时间戳（秒）
        offset_seconds: 偏移量（秒）

    Returns:
        CompareOutput 对象
    """
    # 执行对比
    results = compare_time_series(current_data, compare_data)

    # 按评分排序
    results = sort_by_diff_score(results)

    return CompareOutput(
        compare_enabled=True,
        current_time_range=format_time_range(current_from, current_to),
        compare_time_range=format_time_range(compare_from, compare_to),
        offset=f"{offset_seconds}s",
        total_series=len(results),
        results=results,
    )


def compare_output_to_dict(output: CompareOutput) -> Dict[str, Any]:
    """将 CompareOutput 转换为字典格式

    Args:
        output: CompareOutput 对象

    Returns:
        字典格式的输出，适合 JSON 序列化
    """

    def time_range_to_dict(tr: TimeRangeInfo) -> Dict[str, Any]:
        return {
            "from": tr.from_time,
            "to": tr.to_time,
            "from_unix": tr.from_unix,
            "to_unix": tr.to_unix,
        }

    def stats_to_dict(stats: TimeSeriesStats) -> Dict[str, Any]:
        return {
            "max": stats.max,
            "min": stats.min,
            "avg": stats.avg,
            "count": stats.count,
            "max_time": stats.max_time,
            "min_time": stats.min_time,
        }

    def key_to_dict(key: TimeSeriesKey) -> Dict[str, Any]:
        result = {"entity_id": key.entity_id, "labels": key.labels}
        if key.metric:
            result["metric"] = key.metric
        if key.metric_set_id:
            result["metric_set_id"] = key.metric_set_id
        return result

    def diff_to_dict(diff: DiffDetails) -> Dict[str, Any]:
        return {
            "trend": diff.trend,
            "avg_change": diff.avg_change,
            "avg_change_percent": diff.avg_change_percent,
            "max_change": diff.max_change,
            "min_change": diff.min_change,
        }

    def result_to_dict(r: TimeSeriesCompareResult) -> Dict[str, Any]:
        result = {
            "key": key_to_dict(r.key),
            "current": stats_to_dict(r.current_stats),
            "diff_score": r.diff_score,
            "diff": diff_to_dict(r.diff_details),
        }
        if r.compare_stats:
            result["compare"] = stats_to_dict(r.compare_stats)
        return result

    return {
        "compare_enabled": output.compare_enabled,
        "current_time_range": time_range_to_dict(output.current_time_range),
        "compare_time_range": (
            time_range_to_dict(output.compare_time_range)
            if output.compare_time_range
            else None
        ),
        "offset": output.offset,
        "total_series": output.total_series,
        "results": [result_to_dict(r) for r in output.results],
    }
