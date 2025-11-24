"""通用数据模型定义"""
from typing import Any, List, Optional

from pydantic import BaseModel, Field


class BaseToolParams(BaseModel):
    """所有工具的基础参数"""
    workspace: str = Field(
        ...,
        description="工作空间名称，数据隔离的基本单位，不同工作空间对应不同的数据域或项目。可使用 workspaces_list 工具获取可用的工作空间列表"
    )
    region_id: str = Field(
        default="cn-shanghai",
        description="阿里云区域ID，如 cn-shanghai, cn-beijing, cn-hangzhou 等"
    )


# EntitySelector 已废弃，直接使用参数
# 保留类定义用于兼容性，后续可以完全删除
class EntitySelector(BaseModel):
    """[已废弃] 实体选择器，请直接使用domain, type, filters参数"""
    domain: str = Field(
        ...,
        description="必选实体域，如 apm, k8s, cloud_product。可使用 entities_list_domains 工具获取所有可用的实体域"
    )
    type: str = Field(
        ...,
        description="必选实体类型，如 apm.service, k8s.pod, k8s.cluster。可使用 entities_list_types 工具获取指定域下的所有实体类型"
    )
    filters: Optional[str] = Field(
        None,
        description="（可选）过滤条件，用于筛选特定的实体。支持自然语言描述，如 '名称为frontend'、'状态为健康'、'CPU使用率大于80%' 等"
    )

# EntitySelectorWithQuery 已废弃，直接使用参数
# 保留类定义用于兼容性，后续可以完全删除
class EntitySelectorWithQuery(BaseModel):
    """[已废弃] 带查询功能的实体选择器，请直接使用domain, type, filters, query参数"""
    domain: Optional[str] = Field(
        None,
        description="（可选）实体域，如 apm, k8s, cloud_product。可使用 entities_list_domains 工具获取所有可用的实体域"
    )
    type: Optional[str] = Field(
        None,
        description="（可选）实体类型，如 apm.service, k8s.pod, k8s.cluster。可使用 entities_list_types 工具获取指定域下的所有实体类型"
    )
    filters: Optional[str] = Field(
        None,
        description="（可选）过滤条件，用于筛选特定的实体。支持自然语言描述，如 '名称为frontend'、'状态为健康'、'CPU使用率大于80%' 等"
    )
    query: Optional[str] = Field(
        None,
        description="自然语言过滤条件，用于复杂查询和智能筛选。如果提供此字段，将启用深度搜索(deepSearch)；否则使用快速搜索(fastSearch)。示例：'service包含web的服务' 或 'CPU使用率大于80%的ECS实例' 或 'region等于cn-hangzhou的健康服务'"
    )
    output_mode: str = Field(
        default="list",
        description="输出模式：'list' 返回实体详细列表（默认），'count' 只返回实体数量统计。当用户询问数量或需要避免大量数据时使用count模式"
    )


class EntityFuzzySelector(BaseModel):
    """实体模糊搜索选择器，专用于entities_fuzzy_search工具"""
    query: str = Field(
        ...,
        description="搜索关键词，支持实体名称、ID等关键词模糊匹配。示例：'payment'、'order-service'、'web-001'"
    )
    domain: Optional[str] = Field(
        None,
        description="（可选）实体域过滤，如 apm, k8s, cloud_product。可使用 entities_list_domains 工具获取所有可用的实体域"
    )
    type: Optional[str] = Field(
        None,
        description="（可选）实体类型过滤，如 apm.service, k8s.pod, k8s.cluster。可使用 entities_list_types 工具获取指定域下的所有实体类型"
    )
    limit: int = Field(
        default=100,
        description="返回结果数量限制，默认100"
    )


class TimeRange(BaseModel):
    """时间范围定义"""
    start_time: str = Field(
        default="now()-1h",
        description="开始时间，支持相对时间表达式如 now()-1h 或绝对时间戳"
    )
    end_time: str = Field(
        default="now()",
        description="结束时间，支持相对时间表达式如 now() 或绝对时间戳"
    )


class MetricQuery(BaseModel):
    """指标查询参数"""
    metric_name: str = Field(
        ...,
        description="指标名称。可使用 metrics_list 工具获取实体支持的所有指标列表"
    )
    aggregation: Optional[str] = Field(
        None,
        description="聚合方式（如 avg, sum, max, min, count）"
    )
    interval: Optional[str] = Field(
        "1m",
        description="数据点间隔，默认1分钟"
    )


class TraceFilter(BaseModel):
    """链路过滤条件"""
    error: Optional[bool] = Field(
        None,
        description="是否只查询错误链路"
    )
    min_duration: Optional[int] = Field(
        None,
        description="最小耗时（毫秒）"
    )
    max_duration: Optional[int] = Field(
        None,
        description="最大耗时（毫秒）"
    )
    status_codes: Optional[List[str]] = Field(
        None,
        description="HTTP状态码列表"
    )


class EventFilter(BaseModel):
    """事件过滤条件"""
    event_type: Optional[str] = Field(
        None,
        description="事件类型（如 change, alert）"
    )
    severity: Optional[str] = Field(
        None,
        description="严重程度（如 critical, warning, info）"
    )
    source: Optional[str] = Field(
        None,
        description="事件来源"
    )