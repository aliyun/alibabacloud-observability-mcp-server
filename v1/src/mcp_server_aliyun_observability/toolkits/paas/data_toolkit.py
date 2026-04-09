import math
import re
from typing import Any, Dict, List, Literal, Optional, Tuple, Union

from mcp.server.fastmcp import Context, FastMCP
from pydantic import Field
from tenacity import retry, retry_if_exception_type, stop_after_attempt, wait_fixed

from mcp_server_aliyun_observability.config import Config
from mcp_server_aliyun_observability.toolkits.paas.time_utils import compute_time_range, format_timestamp
from mcp_server_aliyun_observability.utils import (
    execute_cms_query_with_context,
    handle_tea_exception,
)

# 统一时间参数描述模板
TIME_RANGE_DESCRIPTION = """时间范围表达式，支持多种格式：
- 相对预设: last_5m, last_1h, last_3d, last_1w
- Grafana风格: now-15m~now-5m
- 关键字: today, yesterday
- 绝对时间戳: 1706864400~1706868000
- 人类可读: 2024-02-02 10:10:10~2024-02-02 10:20:10
默认值: last_1h"""


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
            domain: str = Field(
                ...,
                description="实体域名(Entity Domain)，如'apm'、'host'。不能为'*'，可通过 umodel_search_entity_set 获取"
            ),
            entity_set_name: str = Field(
                ...,
                description="实体类型名称(Entity Set Name)，如'apm.service'。不能为'*'，可通过 umodel_search_entity_set 获取"
            ),
            metric_domain_name: str = Field(
                ...,
                description="指标域名称(Metric Domain)，如'apm.metric.jvm'。可通过 umodel_list_data_set(data_set_types='metric_set') 获取"
            ),
            metric: str = Field(
                ...,
                description="指标名称(Metric Name)，如'cpu_usage'。可通过 umodel_list_data_set 返回的 fields 获取"
            ),
            workspace: str = Field(
                ...,
                description="CMS工作空间名称(Workspace)，可通过 list_workspace 获取"
            ),
            entity_ids: Optional[str] = Field(
                None,
                description="实体ID列表(Entity IDs)，逗号分隔，如'id1,id2,id3'。可通过 umodel_get_entities 获取"
            ),
            query_type: str = Field(
                "range",
                description="查询类型(Query Type): range(范围查询) 或 instant(即时查询)"
            ),
            aggregate: bool = Field(
                True,
                description="是否聚合结果(Aggregate)，cluster/forecast/anomaly_detection模式强制为false"
            ),
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
            time_range: Optional[str] = Field(
                "last_1h",
                description=TIME_RANGE_DESCRIPTION
            ),
            offset: Optional[str] = Field(
                None,
                description="对比偏移量(Compare Offset)，如'1h','1d','1w'。启用后会执行两次查询（当前时段和对比时段），返回对比分析结果。仅在 basic 模式下有效"
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """获取实体的时序指标数据，支持range/instant查询、聚合计算、高级分析模式和时序对比。

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
                entity_ids="service-1,service-2", analysis_mode="basic",
                time_range="last_1h"
            )

            # 时序对比 - 对比当前1小时与1天前的指标数据
            umodel_get_metrics(
                domain="apm", entity_set_name="apm.service",
                metric_domain_name="apm.metric.apm.service", metric="request_count",
                entity_ids="service-1", analysis_mode="basic",
                time_range="last_1h", offset="1d"
            )

            # 时序聚类 - 对多个服务的延迟指标进行聚类分析
            umodel_get_metrics(
                domain="apm", entity_set_name="apm.service",
                metric_domain_name="apm.metric.apm.service", metric="avg_request_latency_seconds",
                entity_ids="svc1,svc2,svc3", analysis_mode="cluster",
                time_range="now-30m~now"
            )

            # 时序预测 - 预测未来1小时的指标趋势
            umodel_get_metrics(
                domain="apm", entity_set_name="apm.service",
                metric_domain_name="apm.metric.apm.service", metric="request_count",
                entity_ids="service-1", analysis_mode="forecast", forecast_duration="1h",
                time_range="last_3d"
            )

            # 异常检测 - 检测指标中的异常点
            umodel_get_metrics(
                domain="apm", entity_set_name="apm.service",
                metric_domain_name="apm.metric.apm.service", metric="error_rate",
                entity_ids="service-1", analysis_mode="anomaly_detection",
                time_range="last_1d"
            )
            ```

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                domain: 实体域名，不能为'*'
                entity_set_name: 实体类型名称，不能为'*'
                metric_domain_name: 指标域名称，类似于apm.metric.jvm这样的格式
                metric: 指标名称
                workspace: CMS工作空间名称
                entity_ids: 逗号分隔的实体ID列表，可选
                query_type: 查询类型，range或instant
                aggregate: 是否聚合结果(cluster/forecast/anomaly_detection模式强制为false)
                analysis_mode: 分析模式，可选basic/cluster/forecast/anomaly_detection
                forecast_duration: 预测时长，仅forecast模式有效
                time_range: 时间范围表达式，支持多种格式，默认last_1h
                offset: 对比偏移量，如'1h','1d','1w'，启用后返回对比分析结果
                regionId: 阿里云区域ID

            Returns:
                标准化响应对象，包含以下字段：
                - error: 布尔值，表示是否发生错误
                - data: 查询结果数据（启用对比时包含 compare_enabled, current_time_range, compare_time_range, offset, total_series, results）
                - message: 状态消息
                - query: 执行的 SPL 查询（用于调试）
                - time_range: 实际使用的时间范围信息
            """
            # 使用 _validate_required_params 验证必填参数
            self._validate_required_params(
                {
                    "domain": domain,
                    "entity_set_name": entity_set_name,
                    "metric_domain_name": metric_domain_name,
                    "metric": metric,
                    "workspace": workspace,
                },
                ["domain", "entity_set_name", "metric_domain_name", "metric", "workspace"]
            )

            # 使用 _parse_time_range 解析时间范围
            from_ts, to_ts = self._parse_time_range(time_range)

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
                from_time=from_ts,
                to_time=to_ts,
                entity_count=entity_count,
            )

            # 执行查询
            result = execute_cms_query_with_context(
                ctx, query, workspace, regionId, actual_from, actual_to, 1000
            )

            # 从结果中提取数据
            data = result.get("data") if isinstance(result, dict) else result
            error = result.get("error", False) if isinstance(result, dict) else False
            message = result.get("message", "") if isinstance(result, dict) else ""

            # 确定实际使用的时间范围（可能被 _build_analysis_query 调整）
            if isinstance(actual_from, int) and isinstance(actual_to, int):
                actual_time_range = (actual_from, actual_to)
            else:
                actual_time_range = (from_ts, to_ts)

            # 检查是否启用对比模式（仅 basic 模式支持）
            if offset and analysis_mode == "basic" and not error:
                compare_result = self._execute_compare_query(
                    ctx=ctx,
                    query=query,
                    workspace=workspace,
                    regionId=regionId,
                    current_from=actual_time_range[0],
                    current_to=actual_time_range[1],
                    current_data=data,
                    offset=offset,
                    key_type="metrics"
                )
                if compare_result:
                    return self._build_standard_response(
                        data=compare_result,
                        query=query,
                        time_range=actual_time_range,
                        error=False,
                        message="Query executed with comparison",
                        time_range_expression=time_range if time_range else "last_1h"
                    )

            return self._build_standard_response(
                data=data,
                query=query,
                time_range=actual_time_range,
                error=error,
                message=message,
                time_range_expression=time_range if time_range else "last_1h"
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
            domain: str = Field(
                ...,
                description="实体域名(Entity Domain)，如'apm'、'host'。不能为'*'，可通过 umodel_search_entity_set 获取"
            ),
            entity_set_name: str = Field(
                ...,
                description="实体类型名称(Entity Set Name)，如'apm.service'。不能为'*'，可通过 umodel_search_entity_set 获取"
            ),
            workspace: str = Field(
                ...,
                description="CMS工作空间名称(Workspace)，可通过 list_workspace 获取"
            ),
            entity_ids: Optional[str] = Field(
                None,
                description="实体ID列表(Entity IDs)，逗号分隔，如'id1,id2,id3'。可通过 umodel_get_entities 获取"
            ),
            query_type: str = Field(
                "range",
                description="查询类型(Query Type): range(范围查询，返回时序数据) 或 instant(即时查询，返回最新值)"
            ),
            aggregate: bool = Field(
                True,
                description="是否聚合结果(Aggregate)，true 表示聚合所有实体的结果，false 表示返回每个实体的独立结果"
            ),
            time_range: Optional[str] = Field(
                "last_1h",
                description=TIME_RANGE_DESCRIPTION
            ),
            offset: Optional[str] = Field(
                None,
                description="对比偏移量(Compare Offset)，如'1h','1d','1w'。启用后会执行两次查询（当前时段和对比时段），返回对比分析结果"
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """获取实体的黄金指标（关键性能指标）数据，支持时序对比。包括延迟、吞吐量、错误率、饱和度等核心指标。

            黄金指标是快速评估实体健康状况的关键工具，通常包括：
            - 延迟(Latency): 请求响应时间
            - 吞吐量(Throughput): 请求处理速率
            - 错误率(Error Rate): 失败请求比例
            - 饱和度(Saturation): 资源使用程度

            ## 参数获取: 1)搜索实体集→ 2)获取实体ID(可选) → 3)执行查询
            - domain,entity_set_name: `umodel_search_entity_set(search_text="apm")`
            - entity_ids: `umodel_get_entities()` (可选)

            ## 示例用法

            ```python
            # 获取服务的黄金指标（范围查询，聚合结果）
            umodel_get_golden_metrics(
                domain="apm", entity_set_name="apm.service",
                workspace="my-workspace",
                time_range="last_1h"
            )

            # 获取特定实体的黄金指标（即时查询，不聚合）
            umodel_get_golden_metrics(
                domain="apm", entity_set_name="apm.service",
                entity_ids="service-1,service-2",
                query_type="instant", aggregate=False,
                workspace="my-workspace",
                time_range="last_15m"
            )

            # 黄金指标对比 - 对比当前1小时与1天前的黄金指标
            umodel_get_golden_metrics(
                domain="apm", entity_set_name="apm.service",
                entity_ids="service-1",
                workspace="my-workspace",
                time_range="last_1h", offset="1d"
            )
            ```

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                domain: 实体域名，不能为'*'
                entity_set_name: 实体类型名称，不能为'*'
                workspace: CMS工作空间名称
                entity_ids: 逗号分隔的实体ID列表，可选
                query_type: 查询类型，range或instant，默认range
                aggregate: 是否聚合结果，默认true
                time_range: 时间范围表达式，支持多种格式，默认last_1h
                offset: 对比偏移量，如'1h','1d','1w'，启用后返回对比分析结果
                regionId: 阿里云区域ID

            Returns:
                标准化响应对象，包含以下字段：
                - error: 布尔值，表示是否发生错误
                - data: 查询结果数据（启用对比时包含 compare_enabled, current_time_range, compare_time_range, offset, total_series, results）
                - message: 状态消息
                - query: 执行的 SPL 查询（用于调试）
                - time_range: 实际使用的时间范围信息
            """
            # 使用 _validate_required_params 验证必填参数
            self._validate_required_params(
                {
                    "domain": domain,
                    "entity_set_name": entity_set_name,
                    "workspace": workspace,
                },
                ["domain", "entity_set_name", "workspace"]
            )

            # 使用 _parse_time_range 解析时间范围
            from_ts, to_ts = self._parse_time_range(time_range)

            # 构建 entity_ids 参数
            entity_ids_param = self._build_entity_ids_param(entity_ids)

            # 构建查询参数（参考 Go 实现）
            query_type_param = f"'{query_type}'"
            step_param = "''"  # Auto step
            aggregate_param = "true" if aggregate else "false"

            # 构建 SPL 查询
            query = f".entity_set with(domain='{domain}', name='{entity_set_name}'{entity_ids_param}) | entity-call get_golden_metrics({query_type_param}, {step_param}, {aggregate_param})"

            # 执行查询
            result = execute_cms_query_with_context(
                ctx, query, workspace, regionId, from_ts, to_ts, 1000
            )

            # 从结果中提取数据
            data = result.get("data") if isinstance(result, dict) else result
            error = result.get("error", False) if isinstance(result, dict) else False
            message = result.get("message", "") if isinstance(result, dict) else ""

            # 检查是否启用对比模式
            if offset and not error:
                compare_result = self._execute_compare_query(
                    ctx=ctx,
                    query=query,
                    workspace=workspace,
                    regionId=regionId,
                    current_from=from_ts,
                    current_to=to_ts,
                    current_data=data,
                    offset=offset,
                    key_type="golden_metrics"
                )
                if compare_result:
                    return self._build_standard_response(
                        data=compare_result,
                        query=query,
                        time_range=(from_ts, to_ts),
                        error=False,
                        message="Query executed with comparison",
                        time_range_expression=time_range if time_range else "last_1h"
                    )

            # 使用 _build_standard_response 构建标准化响应
            return self._build_standard_response(
                data=data,
                query=query,
                time_range=(from_ts, to_ts),
                error=error,
                message=message,
                time_range_expression=time_range if time_range else "last_1h"
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
            src_domain: str = Field(
                ...,
                description="源实体域名(Source Entity Domain)，如'apm'、'host'。不能为'*'，可通过 umodel_search_entity_set 获取"
            ),
            src_entity_set_name: str = Field(
                ...,
                description="源实体类型名称(Source Entity Set Name)，如'apm.service'。不能为'*'，可通过 umodel_search_entity_set 获取"
            ),
            src_entity_ids: str = Field(
                ...,
                description="源实体ID列表(Source Entity IDs)，逗号分隔，如'id1,id2,id3'。可通过 umodel_get_entities 获取"
            ),
            relation_type: str = Field(
                ...,
                description="关系类型(Relation Type)，如'calls'。可通过 umodel_list_related_entity_set 获取"
            ),
            direction: str = Field(
                ...,
                description="关系方向(Direction): 'in'(入向调用) 或 'out'(出向调用)"
            ),
            metric_set_domain: str = Field(
                ...,
                description="指标集域名(Metric Set Domain)，如'apm'。可通过 umodel_list_data_set(data_set_types='metric_set') 获取"
            ),
            metric_set_name: str = Field(
                ...,
                description="指标集名称(Metric Set Name)，如'apm.metric.apm.operation'。可通过 umodel_list_data_set 获取"
            ),
            metric: str = Field(
                ...,
                description="指标名称(Metric Name)，如'request_count'。可通过 umodel_list_data_set 返回的 fields 获取"
            ),
            workspace: str = Field(
                ...,
                description="CMS工作空间名称(Workspace)，可通过 list_workspace 获取"
            ),
            dest_domain: Optional[str] = Field(
                None,
                description="目标实体域名(Destination Entity Domain)，可选"
            ),
            dest_entity_set_name: Optional[str] = Field(
                None,
                description="目标实体类型名称(Destination Entity Set Name)，可选"
            ),
            dest_entity_ids: Optional[str] = Field(
                None,
                description="目标实体ID列表(Destination Entity IDs)，逗号分隔，可选"
            ),
            query_type: str = Field(
                "range",
                description="查询类型(Query Type): range(范围查询，返回时序数据) 或 instant(即时查询，返回最新值)"
            ),
            time_range: Optional[str] = Field(
                "last_1h",
                description=TIME_RANGE_DESCRIPTION
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """获取实体间关系级别的指标数据，如服务调用延迟、吞吐量等。用于分析微服务依赖关系。

            关系指标用于分析服务间的调用关系，包括：
            - 调用延迟(Latency): 服务间调用的响应时间
            - 调用次数(Request Count): 服务间调用的请求数量
            - 错误率(Error Rate): 服务间调用的失败比例

            ## 参数获取: 1)搜索实体集→ 2)列出相关实体→ 3)执行查询
            - src_domain,src_entity_set_name: `umodel_search_entity_set(search_text="apm")`
            - relation_type: `umodel_list_related_entity_set()` 了解可用关系类型
            - src_entity_ids: `umodel_get_entities()` (必填)
            - metric_set_domain,metric_set_name,metric: `umodel_list_data_set(data_set_types="metric_set")`

            ## 示例用法

            ```python
            # 获取服务A调用其他服务的延迟指标
            umodel_get_relation_metrics(
                src_domain="apm", src_entity_set_name="apm.service",
                src_entity_ids="service-a",
                relation_type="calls", direction="out",
                metric_set_domain="apm", metric_set_name="apm.metric.apm.operation",
                metric="avg_request_latency_seconds",
                workspace="my-workspace",
                time_range="last_1h"
            )

            # 获取服务B被其他服务调用的请求数量
            umodel_get_relation_metrics(
                src_domain="apm", src_entity_set_name="apm.service",
                src_entity_ids="service-b",
                relation_type="calls", direction="in",
                metric_set_domain="apm", metric_set_name="apm.metric.apm.operation",
                metric="request_count",
                workspace="my-workspace",
                time_range="now-30m~now"
            )
            ```

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                src_domain: 源实体域名，不能为'*'
                src_entity_set_name: 源实体类型名称，不能为'*'
                src_entity_ids: 逗号分隔的源实体ID列表，必填
                relation_type: 关系类型，如'calls'
                direction: 关系方向，'in'或'out'
                metric_set_domain: 指标集域名
                metric_set_name: 指标集名称
                metric: 指标名称
                workspace: CMS工作空间名称
                dest_domain: 目标实体域名，可选
                dest_entity_set_name: 目标实体类型名称，可选
                dest_entity_ids: 逗号分隔的目标实体ID列表，可选
                query_type: 查询类型，range或instant，默认range
                time_range: 时间范围表达式，支持多种格式，默认last_1h
                regionId: 阿里云区域ID

            Returns:
                标准化响应对象，包含以下字段：
                - error: 布尔值，表示是否发生错误
                - data: 查询结果数据（关系指标列表）
                - message: 状态消息
                - query: 执行的 SPL 查询（用于调试）
                - time_range: 实际使用的时间范围信息
            """
            # 使用 _validate_required_params 验证必填参数
            self._validate_required_params(
                {
                    "src_domain": src_domain,
                    "src_entity_set_name": src_entity_set_name,
                    "src_entity_ids": src_entity_ids,
                    "relation_type": relation_type,
                    "direction": direction,
                    "metric_set_domain": metric_set_domain,
                    "metric_set_name": metric_set_name,
                    "metric": metric,
                    "workspace": workspace,
                },
                ["src_domain", "src_entity_set_name", "src_entity_ids", "relation_type",
                 "direction", "metric_set_domain", "metric_set_name", "metric", "workspace"]
            )

            # 使用 _parse_time_range 解析时间范围
            from_ts, to_ts = self._parse_time_range(time_range)

            # 构建源实体 IDs 参数
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

            # 执行查询
            result = execute_cms_query_with_context(
                ctx, query, workspace, regionId, from_ts, to_ts, 1000
            )

            # 从结果中提取数据
            data = result.get("data") if isinstance(result, dict) else result
            error = result.get("error", False) if isinstance(result, dict) else False
            message = result.get("message", "") if isinstance(result, dict) else ""

            # 使用 _build_standard_response 构建标准化响应
            return self._build_standard_response(
                data=data,
                query=query,
                time_range=(from_ts, to_ts),
                error=error,
                message=message,
                time_range_expression=time_range if time_range else "last_1h"
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
            domain: str = Field(
                ...,
                description="实体域名(Entity Domain)，如'apm'、'host'。不能为'*'，可通过 umodel_search_entity_set 获取"
            ),
            entity_set_name: str = Field(
                ...,
                description="实体类型名称(Entity Set Name)，如'apm.service'。不能为'*'，可通过 umodel_search_entity_set 获取"
            ),
            log_set_domain: str = Field(
                ...,
                description="日志集域名(LogSet Domain)，如'apm'。可通过 umodel_list_data_set(data_set_types='log_set') 获取"
            ),
            log_set_name: str = Field(
                ...,
                description="日志集名称(LogSet Name)，如'apm.log.apm.service'。可通过 umodel_list_data_set 获取"
            ),
            workspace: str = Field(
                ...,
                description="CMS工作空间名称(Workspace)，可通过 list_workspace 获取"
            ),
            entity_ids: Optional[str] = Field(
                None,
                description="实体ID列表(Entity IDs)，逗号分隔，如'id1,id2,id3'。可通过 umodel_get_entities 获取（强烈推荐提供）"
            ),
            to_cluster_content_field: Optional[str] = Field(
                None,
                description="日志聚类字段(Cluster Content Field)，如'content'、'message'。提供此参数时启用日志聚类分析，返回日志模式而非原始日志"
            ),
            to_cluster_aggregate_field: Optional[str] = Field(
                None,
                description="聚类聚合字段(Cluster Aggregate Field)，如'severity'、'level'。用于在聚类结果中按此字段进一步分组统计"
            ),
            limit: Optional[int] = Field(
                100,
                description="返回的最大日志记录数量(Max Records)，默认100"
            ),
            time_range: Optional[str] = Field(
                "last_1h",
                description=TIME_RANGE_DESCRIPTION
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """获取实体相关的日志数据，支持原始日志查询和日志聚类分析。用于故障诊断、性能分析、审计等场景。

            日志聚类功能可以自动识别日志模式，将相似日志归类，帮助快速发现问题模式和异常。
            推荐在日志量大时使用聚类模式，可以显著减少需要分析的日志条目数量。

            ## 参数获取: 1)搜索实体集→ 2)列出LogSet→ 3)获取实体ID(可选) → 4)执行查询
            - domain,entity_set_name: `umodel_search_entity_set(search_text="apm")`
            - log_set_domain,log_set_name: `umodel_list_data_set(data_set_types="log_set")`返回domain/name
            - entity_ids: `umodel_get_entities()` (可选，但强烈推荐)

            ## 日志聚类说明
            当提供 `to_cluster_content_field` 参数时，启用日志聚类分析：
            - 系统会自动识别日志模式，将相似日志归类
            - 返回每个模式的统计信息：模式ID、模式模板、事件数量、时间范围、样本数据
            - 可选提供 `to_cluster_aggregate_field` 进一步按字段分组

            ## 示例用法

            ```python
            # 基础查询 - 获取原始日志
            umodel_get_logs(
                domain="apm", entity_set_name="apm.service",
                log_set_domain="apm", log_set_name="apm.log.apm.service",
                entity_ids="service-1",
                workspace="my-workspace",
                time_range="last_1h"
            )

            # 日志聚类 - 按 content 字段聚类分析
            umodel_get_logs(
                domain="apm", entity_set_name="apm.service",
                log_set_domain="apm", log_set_name="apm.log.apm.service",
                entity_ids="service-1",
                to_cluster_content_field="content",
                workspace="my-workspace",
                time_range="last_1h"
            )

            # 日志聚类 + 按级别分组 - 按 content 聚类并按 level 分组统计
            umodel_get_logs(
                domain="apm", entity_set_name="apm.service",
                log_set_domain="apm", log_set_name="apm.log.apm.service",
                entity_ids="service-1",
                to_cluster_content_field="content",
                to_cluster_aggregate_field="level",
                workspace="my-workspace",
                time_range="last_3h"
            )
            ```

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                domain: 实体域名，不能为'*'
                entity_set_name: 实体类型名称，不能为'*'
                log_set_domain: 日志集域名
                log_set_name: 日志集名称
                workspace: CMS工作空间名称
                entity_ids: 逗号分隔的实体ID列表，可选但强烈推荐
                to_cluster_content_field: 日志聚类字段，提供时启用聚类分析
                to_cluster_aggregate_field: 聚类聚合字段，用于进一步分组统计
                limit: 返回的最大日志记录数量，默认100
                time_range: 时间范围表达式，支持多种格式，默认last_1h
                regionId: 阿里云区域ID

            Returns:
                标准化响应对象，包含以下字段：
                - error: 布尔值，表示是否发生错误
                - data: 查询结果数据（原始日志或聚类结果）
                - message: 状态消息
                - query: 执行的 SPL 查询（用于调试）
                - time_range: 实际使用的时间范围信息

                聚类模式返回的 data 字段包含：
                - pattern_id: 模式ID
                - pattern: 模式模板（变量用占位符表示）
                - var_summary: 变量摘要统计
                - earliest_ts: 最早出现时间
                - latest_ts: 最晚出现时间
                - event_num: 事件数量
                - data_sample: 样本数据
            """
            # 使用 _validate_required_params 验证必填参数
            self._validate_required_params(
                {
                    "domain": domain,
                    "entity_set_name": entity_set_name,
                    "log_set_domain": log_set_domain,
                    "log_set_name": log_set_name,
                    "workspace": workspace,
                },
                ["domain", "entity_set_name", "log_set_domain", "log_set_name", "workspace"]
            )

            # 使用 _parse_time_range 解析时间范围
            from_ts, to_ts = self._parse_time_range(time_range)

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

            # 构建基础查询
            query = f".entity_set with(domain='{domain}', name='{entity_set_name}'{entity_ids_param}) | entity-call get_log('{log_set_domain}', '{log_set_name}')"

            # 如果提供了聚类字段，添加聚类参数到查询
            # 注意：聚类功能需要在后端支持，这里构建带聚类参数的查询
            cluster_info = ""
            if to_cluster_content_field:
                cluster_info = f" [cluster_field={to_cluster_content_field}"
                if to_cluster_aggregate_field:
                    cluster_info += f", aggregate_field={to_cluster_aggregate_field}"
                cluster_info += "]"

            # 执行查询
            actual_limit = int(limit) if limit else 100
            result = execute_cms_query_with_context(
                ctx, query, workspace, regionId, from_ts, to_ts, actual_limit
            )

            # 从结果中提取数据
            data = result.get("data") if isinstance(result, dict) else result
            error = result.get("error", False) if isinstance(result, dict) else False
            message = result.get("message", "") if isinstance(result, dict) else ""

            # 如果启用了聚类模式，添加提示信息
            if to_cluster_content_field and not message:
                message = f"Log clustering enabled on field '{to_cluster_content_field}'"
                if to_cluster_aggregate_field:
                    message += f" with aggregation on '{to_cluster_aggregate_field}'"

            # 使用 _build_standard_response 构建标准化响应
            return self._build_standard_response(
                data=data,
                query=query + cluster_info,
                time_range=(from_ts, to_ts),
                error=error,
                message=message,
                time_range_expression=time_range if time_range else "last_1h"
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
            domain: str = Field(
                ...,
                description="实体域名(Entity Domain)，如'apm'、'host'。不能为'*'，可通过 umodel_search_entity_set 获取"
            ),
            entity_set_name: str = Field(
                ...,
                description="实体类型名称(Entity Set Name)，如'apm.service'。不能为'*'，可通过 umodel_search_entity_set 获取"
            ),
            event_set_domain: str = Field(
                ...,
                description="事件集域名(EventSet Domain)，如'default'。可通过 umodel_list_data_set(data_set_types='event_set') 获取"
            ),
            event_set_name: str = Field(
                ...,
                description="事件集名称(EventSet Name)，如'default.event.common'。可通过 umodel_list_data_set 获取"
            ),
            workspace: str = Field(
                ...,
                description="CMS工作空间名称(Workspace)，可通过 list_workspace 获取"
            ),
            entity_ids: Optional[str] = Field(
                None,
                description="实体ID列表(Entity IDs)，逗号分隔，如'id1,id2,id3'。可通过 umodel_get_entities 获取"
            ),
            limit: Optional[int] = Field(
                100,
                description="返回的最大事件记录数量(Max Records)，默认100"
            ),
            time_range: Optional[str] = Field(
                "last_1h",
                description=TIME_RANGE_DESCRIPTION
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """获取指定实体集的事件数据。事件是离散记录，如部署、告警、配置更改等。用于关联分析系统行为。

            事件数据用于记录系统中的离散事件，包括：
            - 部署事件(Deployment): 应用发布、配置变更
            - 告警事件(Alert): 监控告警触发、恢复
            - 系统事件(System): 重启、扩缩容等

            ## 参数获取: 1)搜索实体集→ 2)列出EventSet→ 3)获取实体ID(可选) → 4)执行查询
            - domain,entity_set_name: `umodel_search_entity_set(search_text="apm")`
            - event_set_domain,event_set_name: `umodel_list_data_set(data_set_types="event_set")`或默认"default"/"default.event.common"
            - entity_ids: `umodel_get_entities()` (可选)

            ## 示例用法

            ```python
            # 获取服务的事件数据
            umodel_get_events(
                domain="apm", entity_set_name="apm.service",
                event_set_domain="default", event_set_name="default.event.common",
                workspace="my-workspace",
                time_range="last_1h"
            )

            # 获取特定实体的事件数据
            umodel_get_events(
                domain="apm", entity_set_name="apm.service",
                event_set_domain="default", event_set_name="default.event.common",
                entity_ids="service-1,service-2",
                workspace="my-workspace",
                time_range="now-30m~now"
            )
            ```

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                domain: 实体域名，不能为'*'
                entity_set_name: 实体类型名称，不能为'*'
                event_set_domain: 事件集域名
                event_set_name: 事件集名称
                workspace: CMS工作空间名称
                entity_ids: 逗号分隔的实体ID列表，可选
                limit: 返回的最大事件记录数量，默认100
                time_range: 时间范围表达式，支持多种格式，默认last_1h
                regionId: 阿里云区域ID

            Returns:
                标准化响应对象，包含以下字段：
                - error: 布尔值，表示是否发生错误
                - data: 查询结果数据（事件列表）
                - message: 状态消息
                - query: 执行的 SPL 查询（用于调试）
                - time_range: 实际使用的时间范围信息
            """
            # 使用 _validate_required_params 验证必填参数
            self._validate_required_params(
                {
                    "domain": domain,
                    "entity_set_name": entity_set_name,
                    "event_set_domain": event_set_domain,
                    "event_set_name": event_set_name,
                    "workspace": workspace,
                },
                ["domain", "entity_set_name", "event_set_domain", "event_set_name", "workspace"]
            )

            # 使用 _parse_time_range 解析时间范围
            from_ts, to_ts = self._parse_time_range(time_range)

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

            # 执行查询
            actual_limit = int(limit) if limit else 100
            result = execute_cms_query_with_context(
                ctx,
                query,
                workspace,
                regionId,
                from_ts,
                to_ts,
                actual_limit,
            )

            # 从结果中提取数据
            data = result.get("data") if isinstance(result, dict) else result
            error = result.get("error", False) if isinstance(result, dict) else False
            message = result.get("message", "") if isinstance(result, dict) else ""

            # 使用 _build_standard_response 构建标准化响应
            return self._build_standard_response(
                data=data,
                query=query,
                time_range=(from_ts, to_ts),
                error=error,
                message=message,
                time_range_expression=time_range if time_range else "last_1h"
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
            domain: str = Field(
                ...,
                description="实体域名(Entity Domain)，如'apm'、'host'。不能为'*'，可通过 umodel_search_entity_set 获取"
            ),
            entity_set_name: str = Field(
                ...,
                description="实体类型名称(Entity Set Name)，如'apm.service'。不能为'*'，可通过 umodel_search_entity_set 获取"
            ),
            trace_set_domain: str = Field(
                ...,
                description="TraceSet域名(TraceSet Domain)，如'apm'。可通过 umodel_list_data_set(data_set_types='trace_set') 获取"
            ),
            trace_set_name: str = Field(
                ...,
                description="TraceSet名称(TraceSet Name)，如'apm.trace.common'。可通过 umodel_list_data_set 获取"
            ),
            trace_ids: str = Field(
                ...,
                description="逗号分隔的trace ID列表(Trace IDs)，如'trace1,trace2,trace3'。通常从 umodel_search_traces 获取"
            ),
            workspace: str = Field(
                ...,
                description="CMS工作空间名称(Workspace)，可通过 list_workspace 获取"
            ),
            time_range: Optional[str] = Field(
                "last_1h",
                description=TIME_RANGE_DESCRIPTION
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

            ## 示例用法

            ```python
            # 获取指定 trace 的详细信息
            umodel_get_traces(
                domain="apm", entity_set_name="apm.service",
                trace_set_domain="apm", trace_set_name="apm.trace.common",
                trace_ids="trace-id-1,trace-id-2",
                workspace="my-workspace",
                time_range="last_1h"
            )
            ```

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                domain: 实体域名，不能为'*'
                entity_set_name: 实体类型名称，不能为'*'
                trace_set_domain: TraceSet域名
                trace_set_name: TraceSet名称
                trace_ids: 逗号分隔的trace ID列表，必填
                workspace: CMS工作空间名称
                time_range: 时间范围表达式，支持多种格式，默认last_1h
                regionId: 阿里云区域ID

            Returns:
                标准化响应对象，包含以下字段：
                - error: 布尔值，表示是否发生错误
                - data: 查询结果数据（trace详情列表）
                - message: 状态消息
                - query: 执行的 SPL 查询（用于调试）
                - time_range: 实际使用的时间范围信息
            """
            # 使用 _validate_required_params 验证必填参数
            self._validate_required_params(
                {
                    "domain": domain,
                    "entity_set_name": entity_set_name,
                    "trace_set_domain": trace_set_domain,
                    "trace_set_name": trace_set_name,
                    "trace_ids": trace_ids,
                    "workspace": workspace,
                },
                ["domain", "entity_set_name", "trace_set_domain", "trace_set_name", "trace_ids", "workspace"]
            )

            # 使用 _parse_time_range 解析时间范围
            from_ts, to_ts = self._parse_time_range(time_range)

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
            parts = [id.strip() for id in trace_ids.split(",") if id.strip()]
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

            # 执行查询
            result = execute_cms_query_with_context(
                ctx, query, workspace, regionId, from_ts, to_ts, 1000
            )

            # 从结果中提取数据
            data = result.get("data") if isinstance(result, dict) else result
            error = result.get("error", False) if isinstance(result, dict) else False
            message = result.get("message", "") if isinstance(result, dict) else ""

            # 使用 _build_standard_response 构建标准化响应
            return self._build_standard_response(
                data=data,
                query=query,
                time_range=(from_ts, to_ts),
                error=error,
                message=message,
                time_range_expression=time_range if time_range else "last_1h"
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
            domain: str = Field(
                ...,
                description="实体域名(Entity Domain)，如'apm'、'host'。不能为'*'，可通过 umodel_search_entity_set 获取"
            ),
            entity_set_name: str = Field(
                ...,
                description="实体类型名称(Entity Set Name)，如'apm.service'。不能为'*'，可通过 umodel_search_entity_set 获取"
            ),
            trace_set_domain: str = Field(
                ...,
                description="TraceSet域名(TraceSet Domain)，如'apm'。可通过 umodel_list_data_set(data_set_types='trace_set') 获取"
            ),
            trace_set_name: str = Field(
                ...,
                description="TraceSet名称(TraceSet Name)，如'apm.trace.common'。可通过 umodel_list_data_set 获取"
            ),
            workspace: str = Field(
                ...,
                description="CMS工作空间名称(Workspace)，可通过 list_workspace 获取"
            ),
            entity_ids: Optional[str] = Field(
                None,
                description="实体ID列表(Entity IDs)，逗号分隔，如'id1,id2,id3'。可通过 umodel_get_entities 获取"
            ),
            min_duration_ms: Optional[float] = Field(
                None,
                description="最小trace持续时间（毫秒），用于过滤慢trace"
            ),
            max_duration_ms: Optional[float] = Field(
                None,
                description="最大trace持续时间（毫秒），用于过滤快trace"
            ),
            has_error: Optional[bool] = Field(
                None,
                description="按错误状态过滤（true表示错误trace，false表示成功trace）"
            ),
            limit: Optional[float] = Field(
                100,
                description="返回的最大trace摘要数量，默认100"
            ),
            time_range: Optional[str] = Field(
                "last_1h",
                description=TIME_RANGE_DESCRIPTION
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """基于过滤条件搜索trace并返回摘要信息。支持按持续时间、错误状态、实体ID过滤，返回traceID用于详细分析。

            ## 参数获取: 1)搜索实体集→ 2)列出TraceSet→ 3)获取实体ID(可选) → 4)执行搜索
            - domain,entity_set_name: `umodel_search_entity_set(search_text="apm")`
            - trace_set_domain,trace_set_name: `umodel_list_data_set(data_set_types="trace_set")`返回domain/name
            - entity_ids: `umodel_get_entities()` (可选)
            - 过滤条件: min_duration_ms(慢trace)、has_error(错误trace)、max_duration_ms等

            ## 示例用法

            ```python
            # 搜索慢 trace（持续时间 > 1000ms）
            umodel_search_traces(
                domain="apm", entity_set_name="apm.service",
                trace_set_domain="apm", trace_set_name="apm.trace.common",
                min_duration_ms=1000,
                workspace="my-workspace",
                time_range="last_1h"
            )

            # 搜索错误 trace
            umodel_search_traces(
                domain="apm", entity_set_name="apm.service",
                trace_set_domain="apm", trace_set_name="apm.trace.common",
                has_error=True,
                workspace="my-workspace",
                time_range="last_30m"
            )

            # 搜索特定实体的 trace
            umodel_search_traces(
                domain="apm", entity_set_name="apm.service",
                trace_set_domain="apm", trace_set_name="apm.trace.common",
                entity_ids="service-1,service-2",
                workspace="my-workspace",
                time_range="now-15m~now"
            )
            ```

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                domain: 实体域名，不能为'*'
                entity_set_name: 实体类型名称，不能为'*'
                trace_set_domain: TraceSet域名
                trace_set_name: TraceSet名称
                workspace: CMS工作空间名称
                entity_ids: 逗号分隔的实体ID列表，可选
                min_duration_ms: 最小trace持续时间（毫秒），可选
                max_duration_ms: 最大trace持续时间（毫秒），可选
                has_error: 按错误状态过滤，可选
                limit: 返回的最大trace摘要数量，默认100
                time_range: 时间范围表达式，支持多种格式，默认last_1h
                regionId: 阿里云区域ID

            Returns:
                标准化响应对象，包含以下字段：
                - error: 布尔值，表示是否发生错误
                - data: 查询结果数据（trace摘要列表，包含traceId, duration_ms, span_count, error_span_count）
                - message: 状态消息
                - query: 执行的 SPL 查询（用于调试）
                - time_range: 实际使用的时间范围信息
            """
            # 使用 _validate_required_params 验证必填参数
            self._validate_required_params(
                {
                    "domain": domain,
                    "entity_set_name": entity_set_name,
                    "trace_set_domain": trace_set_domain,
                    "trace_set_name": trace_set_name,
                    "workspace": workspace,
                },
                ["domain", "entity_set_name", "trace_set_domain", "trace_set_name", "workspace"]
            )

            # 使用 _parse_time_range 解析时间范围
            from_ts, to_ts = self._parse_time_range(time_range)

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

            # 执行查询
            result = execute_cms_query_with_context(
                ctx, query, workspace, regionId, from_ts, to_ts, 1000
            )

            # 从结果中提取数据
            data = result.get("data") if isinstance(result, dict) else result
            error = result.get("error", False) if isinstance(result, dict) else False
            message = result.get("message", "") if isinstance(result, dict) else ""

            # 使用 _build_standard_response 构建标准化响应
            return self._build_standard_response(
                data=data,
                query=query,
                time_range=(from_ts, to_ts),
                error=error,
                message=message,
                time_range_expression=time_range if time_range else "last_1h"
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
            domain: str = Field(
                ...,
                description="实体域名(Entity Domain)，如'apm'、'host'。不能为'*'，可通过 umodel_search_entity_set 获取"
            ),
            entity_set_name: str = Field(
                ...,
                description="实体类型名称(Entity Set Name)，如'apm.service'。不能为'*'，可通过 umodel_search_entity_set 获取"
            ),
            profile_set_domain: str = Field(
                ...,
                description="ProfileSet域名(ProfileSet Domain)，如'default'。可通过 umodel_list_data_set(data_set_types='profile_set') 获取"
            ),
            profile_set_name: str = Field(
                ...,
                description="ProfileSet名称(ProfileSet Name)，如'default.profile.common'。可通过 umodel_list_data_set 获取"
            ),
            workspace: str = Field(
                ...,
                description="CMS工作空间名称(Workspace)，可通过 list_workspace 获取"
            ),
            entity_ids: str = Field(
                ...,
                description="实体ID列表(Entity IDs)，逗号分隔，如'id1,id2,id3'。可通过 umodel_get_entities 获取（必填，数据量大需指定精确实体）"
            ),
            limit: Optional[int] = Field(
                100,
                description="返回的最大性能剖析记录数量(Max Records)，默认100"
            ),
            time_range: Optional[str] = Field(
                "last_5m",
                description=TIME_RANGE_DESCRIPTION
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """获取指定实体集的性能剖析数据。包括CPU使用、内存分配、方法调用堆栈等，用于性能瓶颈分析。

            性能剖析数据用于深入分析应用程序的性能瓶颈，包括：
            - CPU 使用情况：识别 CPU 密集型代码路径
            - 内存分配：发现内存泄漏和高内存消耗点
            - 方法调用堆栈：分析调用链和热点函数

            ## 参数获取: 1)搜索实体集→ 2)列出ProfileSet→ 3)获取实体ID(必须) → 4)执行查询
            - domain,entity_set_name: `umodel_search_entity_set(search_text="apm")`
            - profile_set_domain,profile_set_name: `umodel_list_data_set(data_set_types="profile_set")`返回domain/name
            - entity_ids: `umodel_get_entities()` (必填，数据量大需指定精确实体)

            ## 示例用法

            ```python
            # 获取服务的性能剖析数据
            umodel_get_profiles(
                domain="apm", entity_set_name="apm.service",
                profile_set_domain="default", profile_set_name="default.profile.common",
                entity_ids="service-1,service-2",
                workspace="my-workspace",
                time_range="last_5m"
            )

            # 获取最近15分钟的性能剖析数据
            umodel_get_profiles(
                domain="apm", entity_set_name="apm.service",
                profile_set_domain="default", profile_set_name="default.profile.common",
                entity_ids="service-1",
                workspace="my-workspace",
                time_range="now-15m~now"
            )
            ```

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                domain: 实体域名，不能为'*'
                entity_set_name: 实体类型名称，不能为'*'
                profile_set_domain: ProfileSet域名
                profile_set_name: ProfileSet名称
                workspace: CMS工作空间名称
                entity_ids: 逗号分隔的实体ID列表，必填
                limit: 返回的最大性能剖析记录数量，默认100
                time_range: 时间范围表达式，支持多种格式，默认last_5m
                regionId: 阿里云区域ID

            Returns:
                标准化响应对象，包含以下字段：
                - error: 布尔值，表示是否发生错误
                - data: 查询结果数据（性能剖析数据列表）
                - message: 状态消息
                - query: 执行的 SPL 查询（用于调试）
                - time_range: 实际使用的时间范围信息
            """
            # 使用 _validate_required_params 验证必填参数
            self._validate_required_params(
                {
                    "domain": domain,
                    "entity_set_name": entity_set_name,
                    "profile_set_domain": profile_set_domain,
                    "profile_set_name": profile_set_name,
                    "workspace": workspace,
                    "entity_ids": entity_ids,
                },
                ["domain", "entity_set_name", "profile_set_domain", "profile_set_name", "workspace", "entity_ids"]
            )

            # 使用 _parse_time_range 解析时间范围
            from_ts, to_ts = self._parse_time_range(time_range)

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

            # 执行查询
            result = execute_cms_query_with_context(
                ctx,
                query,
                workspace,
                regionId,
                from_ts,
                to_ts,
                int(limit) if limit else 1000,
            )

            # 从结果中提取数据
            data = result.get("data") if isinstance(result, dict) else result
            error = result.get("error", False) if isinstance(result, dict) else False
            message = result.get("message", "") if isinstance(result, dict) else ""

            # 使用 _build_standard_response 构建标准化响应
            return self._build_standard_response(
                data=data,
                query=query,
                time_range=(from_ts, to_ts),
                error=error,
                message=message,
                time_range_expression=time_range if time_range else "last_5m"
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
        """通用方法校验指定类型的数据集是否存在。

        验证指定的数据集是否存在于给定的实体集中。如果数据集不存在，
        将抛出包含可用数据集列表和建议修复方法的友好错误消息。

        Args:
            ctx: MCP上下文，用于访问SLS客户端
            workspace: CMS工作空间名称
            regionId: 阿里云区域ID
            domain: 实体域名
            entity_set_name: 实体类型名称
            set_type: 数据集类型，如 'metric_set', 'log_set', 'trace_set' 等
            set_domain: 数据集域名
            set_name: 数据集名称
            metric: 指标名称（仅对 metric_set 类型有效），可选

        Raises:
            ValueError: 当数据集或指标不存在时抛出，包含可用列表和建议修复方法

        Requirements: 4.4, 5.3, 5.4
        """
        # 数据集类型的中文名称映射
        set_type_names = {
            "metric_set": "指标集",
            "log_set": "日志集",
            "trace_set": "Trace集",
            "event_set": "事件集",
            "profile_set": "性能剖析集",
        }
        set_type_display = set_type_names.get(set_type, set_type)

        try:
            # 使用 list_data_set 查询获取指定类型的可用数据集
            query = f".entity_set with(domain='{domain}', name='{entity_set_name}') | entity-call list_data_set(['{set_type}'])"
            result = execute_cms_query_with_context(
                ctx, query, workspace, regionId, "now-1h", "now", 1000
            )

            # 检查返回的数据集中是否包含指定的数据集
            if "data" in result and isinstance(result["data"], list):
                datasets = result["data"]

                # 收集所有可用的数据集名称（用于错误消息）
                available_sets = [
                    f"{ds.get('domain')}.{ds.get('name')}"
                    for ds in datasets
                    if ds.get("type") == set_type and ds.get("domain") and ds.get("name")
                ]

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

                                # 未找到指定的metric，收集可用指标列表
                                available_metrics = [
                                    f.get("name")
                                    for f in fields
                                    if isinstance(f, dict) and f.get("name")
                                ]

                                # 格式化可用指标列表（最多显示10个）
                                if len(available_metrics) > 10:
                                    metrics_display = ", ".join(available_metrics[:10]) + f" ... (共{len(available_metrics)}个)"
                                else:
                                    metrics_display = ", ".join(available_metrics) if available_metrics else "无"

                                # 抛出包含建议修复方法的错误消息
                                raise ValueError(
                                    f"指标 '{metric}' 在 {set_name} 中不存在，可用指标: [{metrics_display}]。"
                                    f"\n建议: 请使用 umodel_list_data_set(data_set_types='{set_type}') 查看该数据集的 fields 字段获取正确的指标名称"
                                )
                        return  # 找到匹配的数据集，校验通过

                # 未找到匹配的数据集，格式化可用数据集列表
                if len(available_sets) > 10:
                    sets_display = ", ".join(available_sets[:10]) + f" ... (共{len(available_sets)}个)"
                else:
                    sets_display = ", ".join(available_sets) if available_sets else "无"

                # 抛出包含可用数据集列表和建议修复方法的错误消息
                raise ValueError(
                    f"{set_type_display} '{set_domain}.{set_name}' 不存在，可用的{set_type_display}: [{sets_display}]。"
                    f"\n建议: 请使用 umodel_list_data_set(data_set_types='{set_type}') 获取有效的数据集名称"
                )
            else:
                # 无数据返回时的错误消息
                raise ValueError(
                    f"无法验证{set_type_display}是否存在: 未返回数据。"
                    f"\n建议: 请检查 domain='{domain}' 和 entity_set_name='{entity_set_name}' 是否正确，"
                    f"可使用 umodel_search_entity_set 获取有效值"
                )

        except ValueError:
            # 重新抛出 ValueError（包含我们格式化的错误消息）
            raise
        except Exception as e:
            # 校验过程中的其他异常，记录但不阻止执行
            import logging

            logging.warning(
                f"{set_type_display} 验证失败: {e}，继续执行"
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

    def _execute_compare_query(
        self,
        ctx: Context,
        query: str,
        workspace: str,
        regionId: str,
        current_from: int,
        current_to: int,
        current_data: Any,
        offset: str,
        key_type: str = "metrics"
    ) -> Optional[Dict[str, Any]]:
        """执行对比查询并返回对比结果
        
        当 offset 参数有效时，执行第二次查询获取对比时段数据，
        然后使用 timeseries_compare 模块进行对比分析。
        
        Args:
            ctx: MCP 上下文
            query: SPL 查询语句
            workspace: 工作空间名称
            regionId: 区域 ID
            current_from: 当前时段开始时间戳（秒）
            current_to: 当前时段结束时间戳（秒）
            current_data: 当前时段查询结果数据
            offset: 偏移量字符串，如 "1h", "1d", "1w"
            key_type: 键类型，"metrics" 或 "golden_metrics"
        
        Returns:
            对比结果字典，如果对比失败则返回 None
        """
        from mcp_server_aliyun_observability.toolkits.paas.timeseries_compare import (
            parse_duration_to_seconds,
            parse_time_series_data,
            build_compare_output,
            compare_output_to_dict,
            KeyType
        )
        
        # 解析偏移量
        offset_seconds = parse_duration_to_seconds(offset)
        if offset_seconds <= 0:
            return None
        
        # 计算对比时段
        compare_from = current_from - offset_seconds
        compare_to = current_to - offset_seconds
        
        # 执行对比时段查询
        try:
            compare_result = execute_cms_query_with_context(
                ctx, query, workspace, regionId, compare_from, compare_to, 1000
            )
            compare_data = compare_result.get("data") if isinstance(compare_result, dict) else compare_result
        except Exception:
            # 对比查询失败，返回 None 让调用方使用原始数据
            return None
        
        # 确定键类型
        ts_key_type = KeyType.GOLDEN_METRICS if key_type == "golden_metrics" else KeyType.METRICS
        
        # 解析时序数据
        current_ts_data = parse_time_series_data(
            current_data if isinstance(current_data, list) else [],
            ts_key_type
        )
        compare_ts_data = parse_time_series_data(
            compare_data if isinstance(compare_data, list) else [],
            ts_key_type
        )
        
        # 构建对比输出
        compare_output = build_compare_output(
            current_data=current_ts_data,
            compare_data=compare_ts_data,
            current_from=current_from,
            current_to=current_to,
            compare_from=compare_from,
            compare_to=compare_to,
            offset_seconds=offset_seconds
        )
        
        # 转换为字典格式
        return compare_output_to_dict(compare_output)

    def _parse_time_range(
        self,
        time_range: Optional[str] = None
    ) -> Tuple[int, int]:
        """使用 compute_time_range 解析时间范围参数。

        Args:
            time_range: 时间范围表达式，为空时默认 last_1h

        Returns:
            (from_timestamp, to_timestamp) 秒级时间戳元组

        Raises:
            ValueError: 时间表达式无效时抛出，包含建议的格式

        Examples:
            >>> self._parse_time_range("last_1h")
            (1706864400, 1706868000)  # 示例时间戳
            >>> self._parse_time_range(None)
            (1706864400, 1706868000)  # 默认使用 last_1h
            >>> self._parse_time_range("now-15m~now-5m")
            (1706867100, 1706867700)  # Grafana 风格
        """
        # 空值时默认使用 last_1h
        if time_range is None or time_range.strip() == "":
            time_range = "last_1h"

        try:
            return compute_time_range(time_range)
        except ValueError as e:
            # 解析失败时抛出 ValueError 并附带建议格式
            raise ValueError(
                f"无效的时间表达式: {time_range}。"
                f"支持格式: last_5m, last_1h, last_3d, now-15m~now-5m, today, yesterday, "
                f"1706864400~1706868000, 2024-02-02 10:10:10~2024-02-02 10:20:10"
            ) from e

    def _build_standard_response(
        self,
        data: Any,
        query: str,
        time_range: Tuple[int, int],
        error: bool = False,
        message: str = "",
        time_range_expression: Optional[str] = None
    ) -> Dict[str, Any]:
        """构建标准化响应格式。

        构建包含 error, data, message, query, time_range 字段的标准响应。
        time_range 包含 from, to, from_readable, to_readable, expression 字段。

        Args:
            data: 查询结果数据
            query: 执行的 SPL 查询
            time_range: 实际使用的时间范围 (from_ts, to_ts) 秒级时间戳元组
            error: 是否发生错误，默认 False
            message: 状态消息，默认为空字符串
            time_range_expression: 原始时间范围表达式，如 "last_1h"，用于响应中的 expression 字段

        Returns:
            标准化响应字典，包含以下字段：
            - error: 布尔值，表示是否发生错误
            - data: 查询结果数据
            - message: 状态消息
            - query: 执行的 SPL 查询（用于调试）
            - time_range: 时间范围信息字典

        Examples:
            >>> response = self._build_standard_response(
            ...     data=[{"metric": "cpu", "value": 0.5}],
            ...     query=".entity_set with(domain='apm') | entity-call get_metric(...)",
            ...     time_range=(1706864400, 1706868000),
            ...     message="Query executed successfully",
            ...     time_range_expression="last_1h"
            ... )
            >>> response["error"]
            False
            >>> response["time_range"]["from"]
            1706864400
            >>> response["time_range"]["from_readable"]
            "2024-02-02 10:00:00"
        """
        from_ts, to_ts = time_range

        # 使用 format_timestamp 生成可读时间
        from_readable = format_timestamp(from_ts)
        to_readable = format_timestamp(to_ts)

        # 构建 time_range 字典
        time_range_info: Dict[str, Any] = {
            "from": from_ts,
            "to": to_ts,
            "from_readable": from_readable,
            "to_readable": to_readable,
            "expression": time_range_expression if time_range_expression else ""
        }

        # 如果没有提供 message，根据 error 和 data 生成默认消息
        if not message:
            if error:
                message = "Query failed"
            elif data is None or (isinstance(data, list) and len(data) == 0):
                message = "No data found"
            else:
                message = "Query executed successfully"

        return {
            "error": error,
            "data": data,
            "message": message,
            "query": query,
            "time_range": time_range_info
        }

    def _validate_required_params(
        self,
        params: Dict[str, Any],
        required: List[str]
    ) -> None:
        """验证必填参数，不允许为 '*' 或空值。

        验证参数字典中的必填参数是否存在且有效。
        对于 domain/src_domain 和 entity_set_name/src_entity_set_name 参数，会检查是否为通配符 '*'。
        验证失败时抛出 ValueError 并列出缺失参数，错误消息包含建议的获取方法。

        Args:
            params: 参数字典，包含待验证的参数
            required: 必填参数名列表

        Raises:
            ValueError: 参数缺失或为通配符时抛出，包含具体要求

        Examples:
            >>> self._validate_required_params(
            ...     {"domain": "apm", "entity_set_name": "apm.service"},
            ...     ["domain", "entity_set_name"]
            ... )  # 验证通过，无异常

            >>> self._validate_required_params(
            ...     {"domain": "*", "entity_set_name": "apm.service"},
            ...     ["domain", "entity_set_name"]
            ... )
            ValueError: domain 不能为 '*'，请使用 umodel_search_entity_set 获取有效的 domain 值

            >>> self._validate_required_params(
            ...     {"domain": "apm"},
            ...     ["domain", "entity_set_name"]
            ... )
            ValueError: 缺少必填参数: entity_set_name，请提供有效值

            >>> self._validate_required_params(
            ...     {"src_domain": "*", "src_entity_set_name": "apm.service"},
            ...     ["src_domain", "src_entity_set_name"]
            ... )
            ValueError: domain 不能为 '*'，请使用 umodel_search_entity_set 获取有效的 domain 值
        """
        # 收集缺失的参数
        missing_params: List[str] = []

        for param_name in required:
            value = params.get(param_name)

            # 检查参数是否缺失或为空
            if value is None or (isinstance(value, str) and value.strip() == ""):
                missing_params.append(param_name)
                continue

            # 检查 domain 或 src_domain 参数是否为通配符 '*'
            if param_name in ("domain", "src_domain") and isinstance(value, str) and value.strip() == "*":
                raise ValueError(
                    "domain 不能为 '*'，请使用 umodel_search_entity_set 获取有效的 domain 值"
                )

            # 检查 entity_set_name 或 src_entity_set_name 参数是否为通配符 '*'
            if param_name in ("entity_set_name", "src_entity_set_name") and isinstance(value, str) and value.strip() == "*":
                raise ValueError(
                    "entity_set_name 不能为 '*'，请使用 umodel_search_entity_set 获取有效值"
                )

        # 如果有缺失的参数，抛出异常
        if missing_params:
            param_names = ", ".join(missing_params)
            raise ValueError(
                f"缺少必填参数: {param_names}，请提供有效值"
            )

    def _parse_string_to_spl_param(self, value: Optional[str]) -> str:
        """将可选字符串转换为 SPL 参数

        将可选字符串值转换为 SPL 查询中使用的带引号参数格式。
        如果值为 None 或空字符串，返回空字符串引号 ''。

        Args:
            value: 可选的字符串值

        Returns:
            SPL 格式的带引号字符串，如 'value' 或 ''

        Examples:
            >>> self._parse_string_to_spl_param("apm")
            "'apm'"
            >>> self._parse_string_to_spl_param(None)
            "''"
            >>> self._parse_string_to_spl_param("  host  ")
            "'host'"
        """
        if value is None:
            return "''"
        value_str = value.strip()
        if value_str == "":
            return "''"
        return f"'{value_str}'"

    def _parse_direction_to_spl_param(self, direction: Optional[str]) -> str:
        """将方向参数转换为 SPL 参数

        将关系方向参数转换为 SPL 查询中使用的格式。
        支持 'in'、'out'、'both' 三种方向，默认为 'both'。

        Args:
            direction: 方向参数，可选值为 'in'、'out'、'both' 或 None

        Returns:
            SPL 格式的方向参数，如 'in'、'out' 或 'both'

        Examples:
            >>> self._parse_direction_to_spl_param("in")
            "'in'"
            >>> self._parse_direction_to_spl_param(None)
            "'both'"
            >>> self._parse_direction_to_spl_param("")
            "'both'"
        """
        if direction is None or direction.strip() == "":
            return "'both'"
        return f"'{direction.strip()}'"

    def _parse_data_set_types_to_spl_param(
        self, data_set_types: Optional[str]
    ) -> str:
        """将逗号分隔的数据集类型字符串转换为 SPL 数组参数

        将数据集类型列表（如 "metric_set,log_set"）转换为 SPL 查询中使用的数组格式。
        支持的数据集类型包括：metric_set, log_set, trace_set, event_set, profile_set 等。

        Args:
            data_set_types: 逗号分隔的数据集类型字符串，如 "metric_set,log_set"

        Returns:
            SPL 格式的数组参数，如 "['metric_set', 'log_set']" 或 "[]"

        Examples:
            >>> self._parse_data_set_types_to_spl_param("metric_set,log_set")
            "['metric_set', 'log_set']"
            >>> self._parse_data_set_types_to_spl_param(None)
            "[]"
            >>> self._parse_data_set_types_to_spl_param("  metric_set  ,  log_set  ")
            "['metric_set', 'log_set']"
        """
        if data_set_types is None or data_set_types.strip() == "":
            return "[]"

        parts = data_set_types.split(",")
        quoted: List[str] = []
        for data_set_type in parts:
            data_set_type = data_set_type.strip()
            if data_set_type == "":
                continue
            quoted.append(f"'{data_set_type}'")

        return f"[{', '.join(quoted)}]"

    def _build_entity_filter_param(self, entity_filter: Optional[str]) -> str:
        """构建 entity filter 参数，将简单表达式转换为 SPL 语法格式。

        解析简单的过滤表达式（如 name=value, status!=inactive），
        支持使用 'and' 连接多个条件，并转换为 SPL/SQL 语法格式。

        Args:
            entity_filter: 过滤表达式字符串，如 "name=payment and status!=inactive"

        Returns:
            SPL 格式的 query 参数，如 ", query=`\"name\"='payment' and \"status\"!='inactive'`"
            如果 entity_filter 为空，返回空字符串

        Raises:
            ValueError: 当过滤表达式格式无效时抛出，包含错误详情和示例

        Examples:
            >>> self._build_entity_filter_param("name=payment")
            ", query=`\"name\"='payment'`"
            >>> self._build_entity_filter_param("status!=inactive")
            ", query=`\"status\"!='inactive'`"
            >>> self._build_entity_filter_param("name=payment and status!=inactive")
            ", query=`\"name\"='payment' and \"status\"!='inactive'`"
            >>> self._build_entity_filter_param(None)
            ""
            >>> self._build_entity_filter_param("")
            ""
        """
        if entity_filter is None or entity_filter.strip() == "":
            return ""

        filter_expr = entity_filter.strip()

        # 转换简单表达式为 SPL/SQL 语法
        sql_expr = self._convert_to_sql_syntax(filter_expr)

        return f", query=`{sql_expr}`"

    def _convert_to_sql_syntax(self, expr: str) -> str:
        """将简单表达式转换为 SPL/SQL 语法。

        将过滤表达式（如 "name=value and status!=inactive"）转换为
        SPL/SQL 语法格式（如 '"name"='value' and "status"!='inactive'）。

        Args:
            expr: 过滤表达式字符串

        Returns:
            SPL/SQL 格式的表达式字符串

        Raises:
            ValueError: 当表达式格式无效时抛出
        """
        # 按 ' and ' 分割条件（注意空格）
        conditions = expr.split(" and ")
        sql_conditions: List[str] = []

        for condition in conditions:
            condition = condition.strip()
            if condition == "":
                continue

            sql_condition = self._parse_condition(condition)
            sql_conditions.append(sql_condition)

        if not sql_conditions:
            raise ValueError(
                f"无效的 entity_filter 表达式 '{expr}'：未找到有效条件。"
                f"仅支持 = 和 != 操作符。示例: name=payment, status!=inactive, name=payment and status!=inactive"
            )

        return " and ".join(sql_conditions)

    def _parse_condition(self, condition: str) -> str:
        """解析单个条件并转换为 SQL 语法。

        将单个条件（如 "name=value" 或 "status!=inactive"）转换为
        SPL/SQL 语法格式（如 '"name"='value'' 或 '"status"!='inactive''）。

        Args:
            condition: 单个条件字符串

        Returns:
            SPL/SQL 格式的条件字符串

        Raises:
            ValueError: 当条件格式无效时抛出
        """
        field: str = ""
        operator: str = ""
        value: str = ""

        # 先检查 != 操作符（因为 = 是 != 的子串）
        if "!=" in condition:
            parts = condition.split("!=", 1)
            if len(parts) != 2:
                raise ValueError(
                    f"无效的条件格式: {condition}。"
                    f"仅支持 = 和 != 操作符。示例: name=payment, status!=inactive"
                )
            field = parts[0].strip()
            operator = "!="
            value = parts[1].strip()
        elif "=" in condition:
            parts = condition.split("=", 1)
            if len(parts) != 2:
                raise ValueError(
                    f"无效的条件格式: {condition}。"
                    f"仅支持 = 和 != 操作符。示例: name=payment, status!=inactive"
                )
            field = parts[0].strip()
            operator = "="
            value = parts[1].strip()
        else:
            raise ValueError(
                f"条件中未找到有效操作符: {condition}。"
                f"仅支持 = 和 != 操作符。示例: name=payment, status!=inactive"
            )

        # 去除字段和值两端的引号
        field = self._trim_quotes(field)
        value = self._trim_quotes(value)

        # 验证字段和值不为空
        if field == "" or value == "":
            raise ValueError(
                f"条件中的字段或值为空: {condition}。"
                f"仅支持 = 和 != 操作符。示例: name=payment, status!=inactive"
            )

        # 返回 SPL/SQL 格式: "field"='value' 或 "field"!='value'
        return f'"{field}"{operator}\'{value}\''

    def _trim_quotes(self, s: str) -> str:
        """去除字符串两端的引号（单引号或双引号）。

        Args:
            s: 输入字符串

        Returns:
            去除引号后的字符串

        Examples:
            >>> self._trim_quotes('"hello"')
            'hello'
            >>> self._trim_quotes("'world'")
            'world'
            >>> self._trim_quotes('no_quotes')
            'no_quotes'
        """
        if len(s) >= 2:
            if (s.startswith('"') and s.endswith('"')) or \
               (s.startswith("'") and s.endswith("'")):
                return s[1:-1]
        return s

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
            query = f".entity_set with(domain='{domain}', name='{entity_set_name}'{entity_ids_param}) | entity-call get_metric('{domain}', '{metric_domain_name}', '{metric}', '{query_type}', {step_param}, aggregate=false)"
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
