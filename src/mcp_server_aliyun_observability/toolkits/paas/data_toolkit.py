from typing import Any, Dict, Optional, Union

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
            from_time: Union[str, int] = Field(
                "now-5m", description="开始时间: Unix时间戳(秒/毫秒)或相对时间(now-5m)"
            ),
            to_time: Union[str, int] = Field(
                "now", description="结束时间: Unix时间戳(秒/毫秒)或相对时间(now)"
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """获取实体的时序指标数据，支持range/instant查询和聚合计算。

            ## 参数获取: 1)搜索实体集→ 2)列出MetricSet→ 3)获取实体ID(可选) → 4)执行查询
            - domain,entity_set_name: `umodel_search_entity_set(search_text="apm")`
            - metric_domain_name,metric: `umodel_list_data_set(data_set_types="metric_set")`返回name/fields
            - entity_ids: `umodel_get_entities()` (可选)

            ## 示例用法

            ```
            # 获取服务的CPU使用率时序数据
            umodel_get_metrics(
                domain="apm",
                entity_set_name="apm.service",
                metric_domain_name="apm.metric.apm.operation",
                metric="cpu_usage",
                entity_ids="service-1,service-2",
                query_type="range",
                aggregate=True
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
                aggregate: 是否聚合结果
                from_time: 数据查询开始时间
                to_time: 数据查询结束时间
                regionId: 阿里云区域ID

            Returns:
                包含指标时序数据的响应对象
            """
            # 校验 metric_domain_name 是否存在
            # metric_domain_name 格式如 "apm.metric.apm.operation"，需要从中提取 domain 和 name
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

            query = f".entity_set with(domain='{domain}', name='{entity_set_name}'{entity_ids_param}) | entity-call get_metric('{domain}', '{metric_domain_name}', '{metric}', '{query_type}', {step_param})"
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

            # 根据Go实现构建正确的查询
            query = f".entity_set with(domain='{src_domain}', name='{src_entity_set_name}', ids={src_entity_ids_param}) | entity-call get_relation_metric({dest_domain_param}, {dest_name_param}, {dest_entity_ids_param}, '', '{relation_type}', '{direction}', '{metric_set_domain}', '{metric_set_name}', '{metric}', '{query_type}', '', [])"

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
            """获取指定trace ID的详细trace数据，包括所有span、时序数据和元数据。用于深入分析慢trace和错误trace。
            ## 参数获取: 1)搜索trace → 2)获取详细信息
            - trace_ids: 通常从`umodel_search_traces()`工具输出中获得
            - domain,entity_set_name: `umodel_search_entity_set(search_text="apm")`
            - trace_set_domain,trace_set_name: `umodel_list_data_set(data_set_types="trace_set")`返回domain/name
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

            # 实现基于 trace_ids 的查询逻辑
            # 这里需要使用不同的查询方式，直接通过 trace_ids 获取详细信息
            query = f".entity_set with(domain='{domain}', name='{entity_set_name}') | entity-call get_trace('{trace_set_domain}', '{trace_set_name}') | where {trace_ids_param} | extend duration_ms = cast(duration as double) / 1000000 | project-away duration | sort traceId desc, duration_ms desc | limit 1000"
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
