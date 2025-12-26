import math
import re
from typing import Any, Dict, Literal, Optional, Union

from mcp.server.fastmcp import Context, FastMCP
from pydantic import Field
from tenacity import retry, retry_if_exception_type, stop_after_attempt, wait_fixed

from mcp_server_aliyun_observability.config import Config
from mcp_server_aliyun_observability.utils import (
    execute_cms_query_with_context,
    handle_tea_exception,
)


class PaasDataToolkit:
    """PaaS Data Toolkit - 可观测数据查询工具包

    ## 工具链流程: 1)发现数据源 → 2)执行数据查询

    **发现阶段**: `umodel_search_entity_set()` → `umodel_list_data_set()` → `umodel_get_entities()`
    **查询阶段**: metrics, logs, events, traces, profiles等8种数据类型查询工具

    ## 统一参数获取模式
    - EntitySet: domain,entity_set_name ← `umodel_search_entity_set(search_text="关键词")`
    - DataSet: {type}_set_domain,{type}_set_name ← `umodel_list_data_set(data_set_types="类型")`
    - 实体ID: entity_ids ← `umodel_get_entities()` (可选)
    - 特定字段: metric/trace_ids等 ← 对应工具返回的fields/结果
    """

    def __init__(self, server: FastMCP):
        self.server = server
        self._register_tools()

    def _register_tools(self):
        """Register data-related PaaS tools"""

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(Config.get_retry_attempts()),
            wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def umodel_get_metrics(
            ctx: Context,
            domain: str = Field(..., description="实体域, cannot be '*'"),
            entity_set_name: str = Field(..., description="实体类型, cannot be '*'"),
            metric_domain_name: str = Field(..., description="指标域, cannot be '*'"),
            metric: str = Field(default=..., description="指标名称"),
            workspace: str = Field(
                ..., description="CMS工作空间名称，可通过list_workspace获取"
            ),
            entity_ids: Optional[str] = Field(
                None, description="Comma-separated entity IDs"
            ),
            query_type: str = Field(
                "range", description="Query type: range or instant"
            ),
            aggregate: bool = Field(True, description="Aggregate results"),
            analysis_mode: Literal[
                "basic", "cluster", "forecast", "anomaly_detection"
            ] = Field(
                "basic",
                description="""分析模式:
- basic: (默认)返回原始时序数据
- cluster: 使用K-Means对指标进行聚类分析，输出聚类索引、实体列表、采样数据及统计值
- forecast: 基于1-5天历史数据预测未来趋势，输出预测值及置信区间
- anomaly_detection: 使用时序分解识别异常点，输出异常列表及统计值""",
            ),
            forecast_duration: Optional[str] = Field(
                None,
                description="预测时长(仅forecast模式有效)，如'30m','1h','2d'。默认30分钟",
            ),
            from_time: Union[str, int] = Field(
                "now-5m", description="开始时间: Unix时间戳(秒/毫秒)或相对时间(now-5m)"
            ),
            to_time: Union[str, int] = Field(
                "now", description="结束时间: Unix时间戳(秒/毫秒)或相对时间(now)"
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """获取实体的时序指标数据，支持range/instant查询、聚合计算和高级分析模式。

            ## 参数获取: 1)搜索实体集→ 2)列出MetricSet→ 3)获取实体ID(可选) → 4)执行查询
            - domain,entity_set_name: `umodel_search_entity_set(search_text="apm")`
            - metric_domain_name,metric: `umodel_list_data_set(data_set_types="metric_set")`返回name/fields
            - entity_ids: `umodel_get_entities()` (可选)

            ## 分析模式说明
            - **basic**: 返回原始时序数据
            - **cluster**: 时序聚类，输出字段: __cluster_index__, __entities__, __sample_ts__, __sample_value__, __sample_value_max/min/avg__
            - **forecast**: 时序预测，输出字段: __forecast_ts__, __forecast_value__, __forecast_lower/upper_value__, __labels__, __name__, __entity_id__
            - **anomaly_detection**: 异常检测，输出字段: __entity_id__, __anomaly_list_, __anomaly_msg__, __value_min/max/avg__

            ## 示例用法

            ```python
            # 基础查询 - 获取服务的CPU使用率时序数据
            umodel_get_metrics(
                domain="apm", entity_set_name="apm.service",
                metric_domain_name="apm.metric.apm.operation", metric="cpu_usage",
                entity_ids="service-1,service-2", analysis_mode="basic"
            )

            # 时序聚类 - 对多个服务的延迟指标进行聚类分析
            umodel_get_metrics(
                domain="apm", entity_set_name="apm.service",
                metric_domain_name="apm.metric.apm.service", metric="avg_request_latency_seconds",
                entity_ids="svc1,svc2,svc3", analysis_mode="cluster"
            )

            # 时序预测 - 预测未来1小时的指标趋势
            umodel_get_metrics(
                domain="apm", entity_set_name="apm.service",
                metric_domain_name="apm.metric.apm.service", metric="request_count",
                entity_ids="service-1", analysis_mode="forecast", forecast_duration="1h"
            )

            # 异常检测 - 检测指标中的异常点
            umodel_get_metrics(
                domain="apm", entity_set_name="apm.service",
                metric_domain_name="apm.metric.apm.service", metric="error_rate",
                entity_ids="service-1", analysis_mode="anomaly_detection"
            )
            ```

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                domain: 实体域名
                entity_set_name: 实体类型名称
                metric_domain_name: 指标域名称，类似于apm.metric.jvm这样的格式
                metric: 指标名称
                entity_ids: 逗号分隔的实体ID列表，可选
                query_type: 查询类型，range或instant
                aggregate: 是否聚合结果(cluster/forecast/anomaly_detection模式强制为false)
                analysis_mode: 分析模式，可选basic/cluster/forecast/anomaly_detection
                forecast_duration: 预测时长，仅forecast模式有效
                from_time: 数据查询开始时间
                to_time: 数据查询结束时间
                regionId: 阿里云区域ID

            Returns:
                包含指标时序数据或分析结果的响应对象
            """
            # 校验 metric_domain_name 是否存在
            metric_parts = metric_domain_name.split(".")
            if len(metric_parts) >= 2:
                metric_set_domain = metric_parts[0]
                metric_set_name = metric_domain_name
            else:
                metric_set_domain = metric_domain_name
                metric_set_name = metric_domain_name

            self._validate_data_set_exists(
                ctx,
                workspace,
                regionId,
                domain,
                entity_set_name,
                "metric_set",
                metric_set_domain,
                metric_set_name,
                metric,
            )

            entity_ids_param = self._build_entity_ids_param(entity_ids)
            step_param = "''"  # Auto step

            # 计算实体数量（用于 cluster 模式）
            entity_count = 0
            if entity_ids and entity_ids.strip():
                entity_count = len(
                    [id.strip() for id in entity_ids.split(",") if id.strip()]
                )

            # 根据分析模式构建查询和调整时间范围
            query, actual_from, actual_to = self._build_analysis_query(
                domain=domain,
                entity_set_name=entity_set_name,
                metric_domain_name=metric_domain_name,
                metric=metric,
                entity_ids_param=entity_ids_param,
                query_type=query_type,
                step_param=step_param,
                aggregate=aggregate,
                analysis_mode=analysis_mode,
                forecast_duration=forecast_duration,
                from_time=from_time,
                to_time=to_time,
                entity_count=entity_count,
            )

            return execute_cms_query_with_context(
                ctx, query, workspace, regionId, actual_from, actual_to, 1000
            )

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(Config.get_retry_attempts()),
            wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def umodel_get_golden_metrics(
            ctx: Context,
            domain: str = Field(..., description="实体域, cannot be '*'"),
            entity_set_name: str = Field(..., description="实体类型, cannot be '*'"),
            workspace: str = Field(
                ..., description="CMS工作空间名称，可通过list_workspace获取"
            ),
            entity_ids: Optional[str] = Field(
                None, description="Comma-separated entity IDs"
            ),
            from_time: Union[str, int] = Field(
                "now-5m", description="开始时间: Unix时间戳(秒/毫秒)或相对时间(now-5m)"
            ),
            to_time: Union[str, int] = Field(
                "now", description="结束时间: Unix时间戳(秒/毫秒)或相对时间(now)"
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """获取实体的黄金指标（关键性能指标）数据。包括延迟、吞吐量、错误率、饱和度等核心指标。
            ## 参数获取: 1)搜索实体集→ 2)获取实体ID(可选) → 3)执行查询
            - domain,entity_set_name: `umodel_search_entity_set(search_text="apm")`
            - entity_ids: `umodel_get_entities()` (可选)
            """
            entity_ids_param = self._build_entity_ids_param(entity_ids)

            query = f".entity_set with(domain='{domain}', name='{entity_set_name}'{entity_ids_param}) | entity-call get_golden_metrics()"
            return execute_cms_query_with_context(
                ctx, query, workspace, regionId, from_time, to_time, 1000
            )

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(Config.get_retry_attempts()),
            wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def umodel_get_relation_metrics(
            ctx: Context,
            src_domain: str = Field(..., description="源实体域, cannot be '*'"),
            src_entity_set_name: str = Field(
                ..., description="源实体类型, cannot be '*'"
            ),
            src_entity_ids: str = Field(..., description="逗号分隔的源实体ID列表"),
            relation_type: str = Field(..., description="关系类型，如'calls'"),
            direction: str = Field(..., description="关系方向: 'in'或'out'"),
            metric_set_domain: str = Field(..., description="指标集域名"),
            metric_set_name: str = Field(..., description="指标集名称"),
            metric: str = Field(..., description="具体指标名称"),
            workspace: str = Field(
                ..., description="CMS工作空间名称，可通过list_workspace获取"
            ),
            dest_domain: Optional[str] = Field(None, description="目标实体域"),
            dest_entity_set_name: Optional[str] = Field(
                None, description="目标实体类型"
            ),
            dest_entity_ids: Optional[str] = Field(
                None, description="逗号分隔的目标实体ID列表"
            ),
            query_type: str = Field("range", description="查询类型: range或instant"),
            from_time: Union[str, int] = Field(
                "now-5m", description="开始时间: Unix时间戳(秒/毫秒)或相对时间(now-5m)"
            ),
            to_time: Union[str, int] = Field(
                "now", description="结束时间: Unix时间戳(秒/毫秒)或相对时间(now)"
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """获取实体间关系级别的指标数据，如服务调用延迟、吞吐量等。用于分析微服务依赖关系。
            ## 参数获取: 1)搜索实体集→ 2)列出相关实体→ 3)执行查询
            - src_domain,src_entity_set_name: `umodel_search_entity_set(search_text="apm")`
            - relation_type: `umodel_list_related_entity_set()`了解可用关系类型
            - src_entity_ids: `umodel_get_entities()` (必填)
            - metric_set_domain,metric_set_name,metric: `umodel_list_data_set(data_set_types="metric_set")`
            """
            # 构建源实体 IDs 参数
            if not src_entity_ids or not src_entity_ids.strip():
                raise ValueError("src_entity_ids is required and cannot be empty")
            src_parts = [id.strip() for id in src_entity_ids.split(",") if id.strip()]
            src_quoted = [f"'{id}'" for id in src_parts]
            src_entity_ids_param = f"[{','.join(src_quoted)}]"

            # 构建目标实体参数
            dest_domain_param = f"'{dest_domain}'" if dest_domain else "''"
            dest_name_param = (
                f"'{dest_entity_set_name}'" if dest_entity_set_name else "''"
            )

            if dest_entity_ids and dest_entity_ids.strip():
                dest_parts = [
                    id.strip() for id in dest_entity_ids.split(",") if id.strip()
                ]
                dest_quoted = [f"'{id}'" for id in dest_parts]
                dest_entity_ids_param = f"[{','.join(dest_quoted)}]"
            else:
                dest_entity_ids_param = "[]"

            # 先校验用户传入的原始 metric_set_name 是否存在（在拼接之前）
            self._validate_data_set_exists(
                ctx,
                workspace,
                regionId,
                src_domain,
                src_entity_set_name,
                "metric_set",
                metric_set_domain,
                metric_set_name,
                metric,
            )

            # 自动拼接 metric_set_name：如果未包含 relation_type，则拼接为 {name}_{relation}_{src_entity}
            relation_suffix = f"_{relation_type}_{src_entity_set_name}"
            if relation_suffix not in metric_set_name:
                metric_set_name = f"{metric_set_name}{relation_suffix}"

            # 根据Go实现构建正确的查询
            # get_relation_metric 前两个参数是 src_domain 和 src_entity_set_name
            query = f".entity_set with(domain='{src_domain}', name='{src_entity_set_name}', ids={src_entity_ids_param}) | entity-call get_relation_metric('{src_domain}', '{src_entity_set_name}', {dest_entity_ids_param}, {dest_domain_param}, '{relation_type}', '{direction}', '{metric_set_domain}', '{metric_set_name}', '{metric}', '{query_type}', {dest_name_param}, [])"

            return execute_cms_query_with_context(
                ctx, query, workspace, regionId, from_time, to_time, 1000
            )

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(Config.get_retry_attempts()),
            wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def umodel_get_logs(
            ctx: Context,
            domain: str = Field(..., description="实体域, cannot be '*'"),
            entity_set_name: str = Field(..., description="实体类型, cannot be '*'"),
            log_set_name: str = Field(..., description="LogSet name"),
            log_set_domain: str = Field(..., description="LogSet domain"),
            workspace: str = Field(
                ..., description="CMS工作空间名称，可通过list_workspace获取"
            ),
            entity_ids: Optional[str] = Field(
                None, description="Comma-separated entity IDs"
            ),
            from_time: Union[str, int] = Field(
                "now-5m", description="开始时间: Unix时间戳(秒/毫秒)或相对时间(now-5m)"
            ),
            to_time: Union[str, int] = Field(
                "now", description="结束时间: Unix时间戳(秒/毫秒)或相对时间(now)"
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """获取实体相关的日志数据，用于故障诊断、性能分析、审计等场景。
            ## 参数获取: 1)搜索实体集→ 2)列出LogSet→ 3)获取实体ID(可选) → 4)执行查询
            - domain,entity_set_name: `umodel_search_entity_set(search_text="apm")`
            - log_set_domain,log_set_name: `umodel_list_data_set(data_set_types="log_set")`返回domain/name
            - entity_ids: `umodel_get_entities()` (可选)
            """
            # 校验 log_set_domain 和 log_set_name 是否存在
            self._validate_data_set_exists(
                ctx,
                workspace,
                regionId,
                domain,
                entity_set_name,
                "log_set",
                log_set_domain,
                log_set_name,
            )

            entity_ids_param = self._build_entity_ids_param(entity_ids)

            query = f".entity_set with(domain='{domain}', name='{entity_set_name}'{entity_ids_param}) | entity-call get_log('{log_set_domain}', '{log_set_name}')"
            return execute_cms_query_with_context(
                ctx, query, workspace, regionId, from_time, to_time, 1000
            )

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(Config.get_retry_attempts()),
            wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def umodel_get_events(
            ctx: Context,
            domain: str = Field(..., description="EntitySet域名，如'apm'"),
            entity_set_name: str = Field(
                ..., description="EntitySet名称，如'apm.service'"
            ),
            event_set_domain: str = Field(..., description="EventSet域名，如'default'"),
            event_set_name: str = Field(
                ..., description="EventSet名称，如'default.event.common'"
            ),
            workspace: str = Field(
                ..., description="CMS工作空间名称，可通过list_workspace获取"
            ),
            entity_ids: Optional[str] = Field(
                None, description="逗号分隔的实体ID列表，如id1,id2,id3"
            ),
            limit: Optional[float] = Field(100, description="返回的最大事件记录数量"),
            from_time: Union[str, int] = Field(
                "now-5m", description="开始时间: Unix时间戳(秒/毫秒)或相对时间(now-5m)"
            ),
            to_time: Union[str, int] = Field(
                "now", description="结束时间: Unix时间戳(秒/毫秒)或相对时间(now)"
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """获取指定实体集的事件数据。事件是离散记录，如部署、告警、配置更改等。用于关联分析系统行为。
            ## 参数获取: 1)搜索实体集→ 2)列出EventSet→ 3)获取实体ID(可选) → 4)执行查询
            - domain,entity_set_name: `umodel_search_entity_set(search_text="apm")`
            - event_set_domain,event_set_name: `umodel_list_data_set(data_set_types="event_set")`或默认"default"/"default.event.common"
            - entity_ids: `umodel_get_entities()` (可选)
            """
            # 校验 event_set_domain 和 event_set_name 是否存在
            self._validate_data_set_exists(
                ctx,
                workspace,
                regionId,
                domain,
                entity_set_name,
                "event_set",
                event_set_domain,
                event_set_name,
            )

            entity_ids_param = self._build_entity_ids_param(entity_ids)

            # 根据Go代码，get_event应该与get_log类似，通过entity-call调用
            query = f".entity_set with(domain='{domain}', name='{entity_set_name}'{entity_ids_param}) | entity-call get_event('{event_set_domain}', '{event_set_name}')"
            return execute_cms_query_with_context(
                ctx,
                query,
                workspace,
                regionId,
                from_time,
                to_time,
                int(limit) if limit else 1000,
            )

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(Config.get_retry_attempts()),
            wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def umodel_get_traces(
            ctx: Context,
            domain: str = Field(..., description="EntitySet域名，如'apm'"),
            entity_set_name: str = Field(
                ..., description="EntitySet名称，如'apm.service'"
            ),
            trace_set_domain: str = Field(..., description="TraceSet域名，如'apm'"),
            trace_set_name: str = Field(
                ..., description="TraceSet名称，如'apm.trace.common'"
            ),
            trace_ids: str = Field(
                ..., description="逗号分隔的trace ID列表，如trace1,trace2,trace3"
            ),
            workspace: str = Field(
                ..., description="CMS工作空间名称，可通过list_workspace获取"
            ),
            from_time: Union[str, int] = Field(
                "now-5m", description="开始时间: Unix时间戳(秒/毫秒)或相对时间(now-5m)"
            ),
            to_time: Union[str, int] = Field(
                "now", description="结束时间: Unix时间戳(秒/毫秒)或相对时间(now)"
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """获取指定trace ID的详细trace数据，包括所有span、独占耗时和元数据。用于深入分析慢trace和错误trace。

            ## 参数获取: 1)搜索trace → 2)获取详细信息
            - trace_ids: 通常从`umodel_search_traces()`工具输出中获得
            - domain,entity_set_name: `umodel_search_entity_set(search_text="apm")`
            - trace_set_domain,trace_set_name: `umodel_list_data_set(data_set_types="trace_set")`返回domain/name

            ## 输出字段说明
            - duration_ms: span总耗时（毫秒）
            - exclusive_duration_ms: span独占耗时（毫秒），即排除子span后的实际执行时间
            """
            # 校验 trace_set_domain 和 trace_set_name 是否存在
            self._validate_data_set_exists(
                ctx,
                workspace,
                regionId,
                domain,
                entity_set_name,
                "trace_set",
                trace_set_domain,
                trace_set_name,
            )

            # 构建 trace_ids 参数
            if not trace_ids or not trace_ids.strip():
                raise ValueError("trace_ids is required and cannot be empty")

            parts = [id.strip() for id in trace_ids.split(",") if id.strip()]
            if not parts:
                raise ValueError("trace_ids is required and cannot be empty")

            quoted_filters = [f"traceId='{id}'" for id in parts]
            trace_ids_param = " or ".join(quoted_filters)

            # 使用 .let 语法构建多步骤查询，计算 exclusive_duration（独占耗时）
            query = f""".let trace_data = .entity_set with(domain='{domain}', name='{entity_set_name}') | entity-call get_trace('{trace_set_domain}', '{trace_set_name}') | where {trace_ids_param} | extend duration_ms = cast(duration as double) / 1000000;

.let trace_data_with_time = $trace_data
| extend startTime=cast(startTime as bigint), duration=cast(duration as bigint)
| extend endTime = startTime + duration
| make-trace
    traceId=traceId,
    spanId=spanId,
    parentSpanId=parentSpanId,
    statusCode=statusCode,
    startTime=startTime,
    endTime=endTime | extend span_list_with_exclusive = trace_exclusive_duration(traceRow.traceId, traceRow.spanList)
| extend span_id = span_list_with_exclusive.span_id, span_index = span_list_with_exclusive.span_index, exclusive_duration = span_list_with_exclusive.exclusive_duration
| extend __trace_id__ = traceRow.traceId | project __trace_id__, span_id, exclusive_duration | unnest | extend exclusive_duration_ms = cast(exclusive_duration as double) / 1000000;

$trace_data | join $trace_data_with_time on $trace_data_with_time.__trace_id__ = traceId and $trace_data_with_time.span_id = spanId | project-away duration, exclusive_duration | sort traceId desc, exclusive_duration_ms desc, duration_ms desc | limit 1000"""
            return execute_cms_query_with_context(
                ctx, query, workspace, regionId, from_time, to_time, 1000
            )

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(Config.get_retry_attempts()),
            wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def umodel_search_traces(
            ctx: Context,
            domain: str = Field(..., description="EntitySet域名，如'apm'"),
            entity_set_name: str = Field(
                ..., description="EntitySet名称，如'apm.service'"
            ),
            trace_set_domain: str = Field(..., description="TraceSet域名，如'apm'"),
            trace_set_name: str = Field(
                ..., description="TraceSet名称，如'apm.trace.common'"
            ),
            workspace: str = Field(
                ..., description="CMS工作空间名称，可通过list_workspace获取"
            ),
            entity_ids: Optional[str] = Field(
                None, description="逗号分隔的实体ID列表，如id1,id2,id3"
            ),
            min_duration_ms: Optional[float] = Field(
                None, description="最小trace持续时间（毫秒）"
            ),
            max_duration_ms: Optional[float] = Field(
                None, description="最大trace持续时间（毫秒）"
            ),
            has_error: Optional[bool] = Field(
                None,
                description="按错误状态过滤（true表示错误trace，false表示成功trace）",
            ),
            limit: Optional[float] = Field(100, description="返回的最大trace摘要数量"),
            from_time: Union[str, int] = Field(
                "now-5m", description="开始时间: Unix时间戳(秒/毫秒)或相对时间(now-5m)"
            ),
            to_time: Union[str, int] = Field(
                "now", description="结束时间: Unix时间戳(秒/毫秒)或相对时间(now)"
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """基于过滤条件搜索trace并返回摘要信息。支持按持续时间、错误状态、实体ID过滤，返回traceID用于详细分析。
            ## 参数获取: 1)搜索实体集→ 2)列出TraceSet→ 3)获取实体ID(可选) → 4)执行搜索
            - domain,entity_set_name: `umodel_search_entity_set(search_text="apm")`
            - trace_set_domain,trace_set_name: `umodel_list_data_set(data_set_types="trace_set")`返回domain/name
            - entity_ids: `umodel_get_entities()` (可选)
            - 过滤条件: min_duration_ms(慢trace)、has_error(错误trace)、max_duration_ms等
            """
            # 校验 trace_set_domain 和 trace_set_name 是否存在
            self._validate_data_set_exists(
                ctx,
                workspace,
                regionId,
                domain,
                entity_set_name,
                "trace_set",
                trace_set_domain,
                trace_set_name,
            )

            # 构建带有可选 entity_ids 的查询
            entity_ids_param = self._build_entity_ids_param(entity_ids)

            # 构建过滤条件
            filter_params = []

            if min_duration_ms is not None:
                filter_params.append(
                    f"cast(duration as bigint) > {int(min_duration_ms * 1000000)}"
                )

            if max_duration_ms is not None:
                filter_params.append(
                    f"cast(duration as bigint) < {int(max_duration_ms * 1000000)}"
                )

            if has_error is not None:
                filter_params.append("cast(statusCode as varchar) = '2'")

            limit_value = 100
            if limit is not None and limit > 0:
                limit_value = int(limit)

            filter_param_str = ""
            if filter_params:
                filter_param_str = "| where " + " and ".join(filter_params)

            stats_str = "| extend duration_ms = cast(duration as double) / 1000000, is_error = case when cast(statusCode as varchar) = '2' then 1 else 0 end |  stats span_count = count(1), error_span_count = sum(is_error), duration_ms = max(duration_ms) by traceId | sort duration_ms desc, error_span_count desc | project traceId, duration_ms, span_count, error_span_count"

            # 实现 search_trace 调用逻辑
            query = f".entity_set with(domain='{domain}', name='{entity_set_name}'{entity_ids_param}) | entity-call get_trace('{trace_set_domain}', '{trace_set_name}') {filter_param_str} {stats_str} | limit {limit_value}"
            return execute_cms_query_with_context(
                ctx, query, workspace, regionId, from_time, to_time, 1000
            )

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(Config.get_retry_attempts()),
            wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def umodel_get_profiles(
            ctx: Context,
            domain: str = Field(..., description="EntitySet域名，如'apm'"),
            entity_set_name: str = Field(
                ..., description="EntitySet名称，如'apm.service'"
            ),
            profile_set_domain: str = Field(
                ..., description="ProfileSet域名，如'default'"
            ),
            profile_set_name: str = Field(
                ..., description="ProfileSet名称，如'default.profile.common'"
            ),
            workspace: str = Field(
                ..., description="CMS工作空间名称，可通过list_workspace获取"
            ),
            entity_ids: str = Field(
                ..., description="逗号分隔的实体ID列表，必填，如id1,id2,id3"
            ),
            limit: Optional[float] = Field(
                100, description="返回的最大性能剖析记录数量"
            ),
            from_time: Union[str, int] = Field(
                "now-5m", description="开始时间: Unix时间戳(秒/毫秒)或相对时间(now-5m)"
            ),
            to_time: Union[str, int] = Field(
                "now", description="结束时间: Unix时间戳(秒/毫秒)或相对时间(now)"
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """获取指定实体集的性能剖析数据。包括CPU使用、内存分配、方法调用堆栈等，用于性能瓶颈分析。
            ## 参数获取: 1)搜索实体集→ 2)列出ProfileSet→ 3)获取实体ID(必须) → 4)执行查询
            - domain,entity_set_name: `umodel_search_entity_set(search_text="apm")`
            - profile_set_domain,profile_set_name: `umodel_list_data_set(data_set_types="profile_set")`返回domain/name
            - entity_ids: `umodel_get_entities()` (必填,数据量大需指定精确实体)
            """
            # 根据Go代码，entity_ids是必需的
            if not entity_ids or not entity_ids.strip():
                raise ValueError("entity_ids is required and cannot be empty")

            # 校验 profile_set_domain 和 profile_set_name 是否存在
            self._validate_profile_set_exists(
                ctx,
                workspace,
                regionId,
                domain,
                entity_set_name,
                profile_set_domain,
                profile_set_name,
            )

            entity_ids_param = self._build_entity_ids_param(entity_ids)

            # 按照Go代码，使用get_profile而不是get_profiles
            query = f".entity_set with(domain='{domain}', name='{entity_set_name}'{entity_ids_param}) | entity-call get_profile('{profile_set_domain}', '{profile_set_name}')"
            return execute_cms_query_with_context(
                ctx,
                query,
                workspace,
                regionId,
                from_time,
                to_time,
                int(limit) if limit else 1000,
            )

    def _validate_data_set_exists(
        self,
        ctx: Context,
        workspace: str,
        regionId: str,
        domain: str,
        entity_set_name: str,
        set_type: str,
        set_domain: str,
        set_name: str,
        metric: Optional[str] = None,
    ) -> None:
        """通用方法校验指定类型的数据集是否存在"""
        try:
            # 使用 list_data_set 查询获取指定类型的可用数据集
            query = f".entity_set with(domain='{domain}', name='{entity_set_name}') | entity-call list_data_set(['{set_type}'])"
            result = execute_cms_query_with_context(
                ctx, query, workspace, regionId, "now-1h", "now", 1000
            )

            # 检查返回的数据集中是否包含指定的数据集
            if "data" in result and isinstance(result["data"], list):
                datasets = result["data"]
                for dataset in datasets:
                    if (
                        dataset.get("domain") == set_domain
                        and dataset.get("name") == set_name
                        and dataset.get("type") == set_type
                    ):
                        # 继续校验metric是否存在
                        if metric and set_type == "metric_set":
                            # 从dataset中获取fields数组
                            fields = dataset.get("fields", [])

                            # 如果fields是字符串，尝试反序列化为list
                            if isinstance(fields, str):
                                try:
                                    import json

                                    fields = json.loads(fields)
                                except (json.JSONDecodeError, ValueError):
                                    # 如果反序列化失败，跳过metric校验
                                    import logging

                                    logging.warning(
                                        f"Failed to parse fields JSON for {set_type} '{set_domain}.{set_name}', skipping metric validation"
                                    )
                                    return

                            if isinstance(fields, list):
                                # 在fields数组中查找指定的metric
                                for field in fields:
                                    if (
                                        isinstance(field, dict)
                                        and field.get("name") == metric
                                    ):
                                        return  # 找到匹配的metric，校验通过

                                # 未找到指定的metric，抛出异常
                                available_metrics = [
                                    f.get("name")
                                    for f in fields
                                    if isinstance(f, dict) and f.get("name")
                                ]
                                raise ValueError(
                                    f"Metric '{metric}' not found in {set_type} '{set_domain}.{set_name}'. "
                                    f"Available metrics: {available_metrics}"
                                )
                        return  # 找到匹配的数据集，校验通过

                # 未找到匹配的数据集，抛出异常
                available_sets = [
                    (ds.get("domain"), ds.get("name"))
                    for ds in datasets
                    if ds.get("type") == set_type
                ]
                raise ValueError(
                    f"{set_type.title()} '{set_domain}.{set_name}' not found. "
                    f"Available {set_type}s: {available_sets}"
                )
            else:
                raise ValueError(
                    f"Failed to validate {set_type} existence: no data returned"
                )

        except Exception as e:
            if "not found" in str(e) or f"Available {set_type}" in str(e):
                raise  # 重新抛出校验失败的异常
            else:
                # 校验过程中的其他异常，记录但不阻止执行
                import logging

                logging.warning(
                    f"{set_type} validation failed with error: {e}, proceeding anyway"
                )

    def _validate_profile_set_exists(
        self,
        ctx: Context,
        workspace: str,
        regionId: str,
        domain: str,
        entity_set_name: str,
        profile_set_domain: str,
        profile_set_name: str,
    ) -> None:
        """校验 profile_set_domain 和 profile_set_name 是否存在"""
        self._validate_data_set_exists(
            ctx,
            workspace,
            regionId,
            domain,
            entity_set_name,
            "profile_set",
            profile_set_domain,
            profile_set_name,
        )

    def _build_entity_ids_param(self, entity_ids: Optional[str]) -> str:
        """Build entity IDs parameter for SPL queries"""
        if not entity_ids or not entity_ids.strip():
            return ""

        parts = [id.strip() for id in entity_ids.split(",") if id.strip()]
        quoted = [f"'{id}'" for id in parts]
        return f", ids=[{','.join(quoted)}]"

    def _parse_duration_to_seconds(self, duration: str) -> int:
        """解析时长字符串为秒数，支持 30m, 1h, 2d, 1w 等格式"""
        if not duration:
            return 0

        duration = duration.strip()
        if len(duration) < 2:
            return 0

        unit = duration[-1].lower()
        try:
            value = int(duration[:-1])
        except ValueError:
            return 0

        multipliers = {
            "m": 60,
            "h": 3600,
            "d": 86400,
            "w": 604800,
        }
        return value * multipliers.get(unit, 1)

    def _calculate_time_range(
        self,
        from_time: Union[str, int],
        to_time: Union[str, int],
        min_duration_days: int,
        max_duration_days: int,
    ) -> tuple:
        """根据分析模式计算调整后的时间范围

        Args:
            from_time: 原始开始时间
            to_time: 原始结束时间
            min_duration_days: 最小时长（天）
            max_duration_days: 最大时长（天）

        Returns:
            (adjusted_from, adjusted_to) 调整后的时间范围
        """
        import time

        # 解析时间为秒级时间戳
        now = int(time.time())

        def parse_time(t: Union[str, int]) -> int:
            if isinstance(t, int):
                # 如果是毫秒级时间戳，转换为秒
                return t // 1000 if t > 10000000000 else t
            if isinstance(t, str):
                if t == "now":
                    return now
                # 处理 now-5m 格式
                match = re.match(r"now-(\d+)([mhd])", t)
                if match:
                    value = int(match.group(1))
                    unit = match.group(2)
                    multipliers = {"m": 60, "h": 3600, "d": 86400}
                    return now - value * multipliers.get(unit, 1)
            return now

        from_ts = parse_time(from_time)
        to_ts = parse_time(to_time)

        current_duration = to_ts - from_ts
        min_duration = min_duration_days * 86400
        max_duration = max_duration_days * 86400

        # 将时间范围限制在 [min_duration, max_duration] 区间内
        final_duration = current_duration
        if final_duration < min_duration:
            final_duration = min_duration
        if final_duration > max_duration:
            final_duration = max_duration

        return to_ts - final_duration, to_ts

    def _build_analysis_query(
        self,
        domain: str,
        entity_set_name: str,
        metric_domain_name: str,
        metric: str,
        entity_ids_param: str,
        query_type: str,
        step_param: str,
        aggregate: bool,
        analysis_mode: str,
        forecast_duration: Optional[str],
        from_time: Union[str, int],
        to_time: Union[str, int],
        entity_count: int,
    ) -> tuple:
        """根据分析模式构建查询语句和时间范围

        Returns:
            (query, from_time, to_time) 查询语句和调整后的时间范围
        """
        import time

        now = int(time.time())

        # 解析原始时间范围
        def parse_time(t: Union[str, int]) -> int:
            if isinstance(t, int):
                return t // 1000 if t > 10000000000 else t
            if isinstance(t, str):
                if t == "now":
                    return now
                match = re.match(r"now-(\d+)([mhd])", t)
                if match:
                    value = int(match.group(1))
                    unit = match.group(2)
                    multipliers = {"m": 60, "h": 3600, "d": 86400}
                    return now - value * multipliers.get(unit, 1)
            return now

        from_ts = parse_time(from_time)
        to_ts = parse_time(to_time)

        # basic 模式：返回原始查询
        if analysis_mode == "basic":
            query = f".entity_set with(domain='{domain}', name='{entity_set_name}'{entity_ids_param}) | entity-call get_metric('{domain}', '{metric_domain_name}', '{metric}', '{query_type}', {step_param})"
            return query, from_time, to_time

        # cluster 模式：时序聚类
        if analysis_mode == "cluster":
            # 计算聚类数：nClusters = ceil(entityCount / 2)，最少 2，最多 7
            n_clusters = max(2, min(7, math.ceil(entity_count / 2))) if entity_count > 0 else 2

            base_query = f".entity_set with(domain='{domain}', name='{entity_set_name}'{entity_ids_param}) | entity-call get_metric('{domain}', '{metric_domain_name}', '{metric}', '{query_type}', {step_param}, aggregate=false)"
            query = f"""{base_query}
| stats __entity_id_array__ = array_agg(__entity_id__), __labels_array__ = array_agg(__labels__), ts_array = array_agg(__ts__), ds_array = array_agg(__value__)
| extend ret = cluster(ds_array, 'kmeans', '{{"n_clusters":"{n_clusters}"}}')
| extend __cluster_index__ = ret.assignments, error_msg = ret.error_msg, __entity_id__ = __entity_id_array__, __labels__ = __labels_array__, __value__ = ds_array, __ts__ = ts_array
| project __entity_id__, __labels__, __ts__, __value__, __cluster_index__
| unnest
| stats cnt = count(1), __entities__ = array_agg(__entity_id__), __labels_agg__ = array_agg(__ts__), __value_agg__ = array_agg(__value__) by __cluster_index__
| extend __sample_value__ = __value_agg__[1], __sample_ts__ = __labels_agg__[1]
| extend __sample_value_min__ = array_min(__sample_value__), __sample_value_max__ = array_max(__sample_value__), __sample_value_avg__ = reduce(__sample_value__, 0.0, (s, x) -> s + x, s -> s) / cast(cardinality(__sample_value__) as double)
| project __cluster_index__, __entities__, __sample_ts__, __sample_value__, __sample_value_max__, __sample_value_min__, __sample_value_avg__"""
            return query, from_time, to_time

        # forecast 模式：时序预测
        if analysis_mode == "forecast":
            # 调整时间范围：1-5 天
            adjusted_from, adjusted_to = self._calculate_time_range(
                from_time, to_time, min_duration_days=1, max_duration_days=5
            )
            learning_duration = adjusted_to - adjusted_from

            # 解析预测时长，默认 30 分钟
            forecast_dur = (
                self._parse_duration_to_seconds(forecast_duration)
                if forecast_duration
                else 1800
            )
            if forecast_dur <= 0:
                forecast_dur = 1800

            # 计算预测点数
            forecast_points = max(3, int(forecast_dur * 200 / learning_duration))

            base_query = f".entity_set with(domain='{domain}', name='{entity_set_name}'{entity_ids_param}) | entity-call get_metric('{domain}', '{metric_domain_name}', '{metric}', '{query_type}', {step_param}, aggregate=false)"
            query = f"""{base_query}
| extend r = series_forecast(__value__, {forecast_points})
| extend __forecast_rst_m__ = zip(r.time_series, r.forecast_metric_series, r.forecast_metric_lower_series, r.forecast_metric_upper_series), __forecast_msg__ = r.error_msg
| extend __forecast_rst__ = slice(__forecast_rst_m__, cardinality(__forecast_rst_m__) - {forecast_points} + 1, {forecast_points})
| project __labels__, __name__, __ts__, __value__, __forecast_rst__, __forecast_msg__, __entity_id__
| extend __forecast_ts__ = transform(__forecast_rst__, x->x.field0), __forecast_value__ = transform(__forecast_rst__, x->x.field1), __forecast_lower_value__ = transform(__forecast_rst__, x->x.field2), __forecast_upper_value__ = transform(__forecast_rst__, x->x.field3)
| project __labels__, __name__, __entity_id__, __forecast_ts__, __forecast_value__, __forecast_lower_value__, __forecast_upper_value__"""
            return query, adjusted_from, adjusted_to

        # anomaly_detection 模式：异常检测
        if analysis_mode == "anomaly_detection":
            # 调整时间范围：1-3 天
            adjusted_from, adjusted_to = self._calculate_time_range(
                from_time, to_time, min_duration_days=1, max_duration_days=3
            )
            # 转换为纳秒级时间戳
            start_time_ns = adjusted_from * 1_000_000_000

            base_query = f".entity_set with(domain='{domain}', name='{entity_set_name}'{entity_ids_param}) | entity-call get_metric('{domain}', '{metric_domain_name}', '{metric}', '{query_type}', {step_param}, aggregate=false)"
            query = f"""{base_query}
| extend slice_index = find_first_index(__ts__, x -> x > {start_time_ns})
| extend len = cardinality(__ts__)
| extend r = series_decompose_anomalies(__value__)
| extend anomaly_b = r.anomalies_score_series, anomaly_type = r.anomalies_type_series, __anomaly_msg__ = r.error_msg
| extend x = zip(anomaly_b, __ts__, anomaly_type, __value__)
| extend __anomaly_rst__ = filter(x, x-> x.field0 > 0 and x.field1 >= {start_time_ns})
| extend __anomaly_list_ = transform(__anomaly_rst__, x-> map(ARRAY['anomary_time', 'anomary_type', 'value'], ARRAY[cast(x.field1 as varchar), x.field2, cast(x.field3 as varchar)]))
| extend __detection_value__ = slice(__value__, slice_index, len - slice_index)
| extend __value_min__ = array_min(__detection_value__), __value_max__ = array_max(__detection_value__), __value_avg__ = reduce(__detection_value__, 0.0, (s, x) -> s + x, s -> s) / cast(len as double)
| project __entity_id__, __anomaly_list_, __anomaly_msg__, __value_min__, __value_max__, __value_avg__"""
            return query, adjusted_from, adjusted_to

        # 未知模式，回退到 basic
        query = f".entity_set with(domain='{domain}', name='{entity_set_name}'{entity_ids_param}) | entity-call get_metric('{domain}', '{metric_domain_name}', '{metric}', '{query_type}', {step_param})"
        return query, from_time, to_time
