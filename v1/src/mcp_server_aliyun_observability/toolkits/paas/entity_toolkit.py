from typing import Any, Dict, List, Optional, Tuple, Union

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
TIME_RANGE_DESCRIPTION = """Flexible time range formats:
- **Relative Presets:** "last_5m", "last_1h", "last_3d", "last_1w", "last_1M", "last_1y".
- **Grafana-style:** "now-15m~now-5m".
- **Keywords:** "today", "yesterday".
- **Absolute Timestamps:** "1706864400~1706868000".
- **Human-readable:** "2024-02-02 10:10:10~2024-02-02 10:20:10".
Default: last_1h"""


class PaaSEntityToolkit:
    """PaaS Entity Management Toolkit

    Provides structured entity query tools ported from umodel entity handlers.
    """

    def __init__(self, server: FastMCP):
        self.server = server
        self._register_tools()

    def _register_tools(self):
        """Register entity-related PaaS tools"""

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(Config.get_retry_attempts()),
            wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def umodel_get_entities(
            ctx: Context,
            domain: str = Field(..., description="实体域, cannot be '*'"),
            entity_set_name: str = Field(..., description="实体类型, cannot be '*'"),
            workspace: str = Field(
                ..., description="CMS工作空间名称，可通过list_workspace获取"
            ),
            entity_ids: Optional[str] = Field(
                None, description="可选的逗号分隔实体ID列表，用于精确查询指定实体。entity_ids 和 entity_filter 至少需要提供一个"
            ),
            entity_filter: Optional[str] = Field(
                None, description="实体过滤表达式，如 'name=payment' 或 'status!=inactive'，支持 'and' 连接多个条件。entity_ids 和 entity_filter 至少需要提供一个"
            ),
            time_range: Optional[str] = Field(
                "last_1h",
                description=TIME_RANGE_DESCRIPTION
            ),
            limit: int = Field(
                20, description="返回多少个实体，默认20个", ge=1, le=1000
            ),
            regionId: str = Field(..., description="Region ID like 'cn-hangzhou'"),
        ) -> Dict[str, Any]:
            """获取实体信息的PaaS API工具。

            ## 功能概述

            该工具用于检索实体信息，支持分页查询、精确ID查询和过滤条件查询。
            如需要模糊搜索请使用 `umodel_search_entities` 工具。

            ## 功能特点

            - **数量控制**: 默认返回20个实体，支持通过limit参数控制返回数量
            - **精确查询**: 支持根据实体ID列表进行精确查询
            - **过滤查询**: 支持使用 entity_filter 表达式进行条件过滤
            - **职责清晰**: 专注于基础实体信息获取，不包含复杂搜索逻辑

            ## 使用场景

            - **分页浏览**: 分页获取实体列表，适用于大量实体的展示场景
            - **精确查询**: 根据已知的实体ID列表批量获取实体详细信息
            - **条件过滤**: 使用 entity_filter 按属性过滤实体
            - **基础数据**: 为其他分析工具提供基础实体数据

            ## 参数说明

            - domain: 实体集合的域，如 'apm'、'infrastructure' 等
            - entity_set_name: 实体集合名称，如 'apm.service'、'host.instance' 等
            - entity_ids: 可选的逗号分隔实体ID字符串，用于精确查询指定实体
            - entity_filter: 可选的过滤表达式，如 'name=payment and status!=inactive'
            - time_range: 时间范围表达式，支持多种格式，默认 last_1h
            - limit: 返回多少个实体，默认20个，最大1000个

            **注意**: entity_ids 和 entity_filter 至少需要提供一个

            ## 工具分工

            - `umodel_get_entities`: 基础实体信息获取（本工具）
            - `umodel_search_entities`: 基于关键词的模糊搜索
            - `umodel_get_neighbor_entities`: 获取实体的邻居关系

            ## 数量控制说明

            使用 `|limit {count}` 格式控制返回数量：
            - 返回前10个实体：limit=10 → `|limit 10`
            - 返回前50个实体：limit=50 → `|limit 50`
            - 返回前100个实体：limit=100 → `|limit 100`

            ## 示例用法

            ```
            # 根据实体ID批量查询
            umodel_get_entities(
                domain="apm",
                entity_set_name="apm.service",
                entity_ids="service-1,service-2,service-3"
            )

            # 使用过滤条件查询
            umodel_get_entities(
                domain="apm",
                entity_set_name="apm.service",
                entity_filter="name=payment"
            )

            # 组合使用 entity_ids 和 entity_filter
            umodel_get_entities(
                domain="apm",
                entity_set_name="apm.service",
                entity_ids="service-1,service-2",
                entity_filter="status!=inactive"
            )

            # 使用时间范围参数
            umodel_get_entities(
                domain="apm",
                entity_set_name="apm.service",
                entity_ids="service-1",
                time_range="last_3h"
            )
            ```

            Args:
                ctx: MCP上下文，用于访问CMS客户端
                domain: 实体集合域名
                entity_set_name: 实体集合名称
                entity_ids: 可选的逗号分隔实体ID列表
                entity_filter: 可选的过滤表达式
                time_range: 时间范围表达式，默认 last_1h
                limit: 返回实体数量
                regionId: 阿里云区域ID

            Returns:
                标准化响应对象，包含 error, data, message, query, time_range 字段
            """
            # 验证 domain 和 entity_set_name 不能为通配符
            self._validate_required_params(
                {"domain": domain, "entity_set_name": entity_set_name},
                ["domain", "entity_set_name"]
            )

            # 验证至少提供 entity_ids 或 entity_filter 之一
            has_entity_ids = entity_ids is not None and entity_ids.strip() != ""
            has_entity_filter = entity_filter is not None and entity_filter.strip() != ""
            
            if not has_entity_ids and not has_entity_filter:
                raise ValueError(
                    "必须至少提供 entity_ids 或 entity_filter 之一。"
                    "entity_ids: 逗号分隔的实体ID列表，如 'id1,id2,id3'。"
                    "entity_filter: 过滤表达式，如 'name=payment and status!=inactive'"
                )

            # 解析时间范围
            from_ts, to_ts = self._parse_time_range(time_range)

            # 构建 entity IDs 参数
            entity_ids_param = self._build_entity_ids_param(entity_ids)

            # 构建 entity filter 参数
            entity_filter_param = self._build_entity_filter_param(entity_filter)

            # 构建查询
            query = f".entity_set with(domain='{domain}', name='{entity_set_name}'{entity_ids_param}{entity_filter_param}) | entity-call get_entities() | limit {limit}"

            # 执行查询
            result = execute_cms_query_with_context(
                ctx, query, workspace, regionId, from_ts, to_ts, limit
            )

            # 构建标准响应
            # 从结果中提取数据
            data = result.get("data") if isinstance(result, dict) else result
            
            return self._build_standard_response(
                data=data,
                query=query,
                time_range=(from_ts, to_ts),
                error=False,
                message="",
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
        def umodel_get_neighbor_entities(
            ctx: Context,
            workspace: str = Field(
                ..., description="CMS workspace identifier"
            ),
            src_entity_domain: str = Field(..., description="Source Domain"),
            src_name: str = Field(..., description="Source EntitySet name"),
            src_entity_ids: str = Field(..., description="Comma-separated source entity IDs"),
            dest_entity_domain: Optional[str] = Field(
                None, description="Optional target Domain filter"
            ),
            dest_name: Optional[str] = Field(
                None, description="Optional target EntitySet filter"
            ),
            relation_type: Optional[str] = Field(
                None, description="Optional relation type filter"
            ),
            direction: str = Field(
                "both", description='Direction: "out" (downstream), "in" (upstream), "both". Default: "both"'
            ),
            time_range: Optional[str] = Field(
                "last_1h",
                description=TIME_RANGE_DESCRIPTION
            ),
            limit: int = Field(
                10, description="Max results. Default: 10", ge=1, le=1000
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """Retrieves entities directly related to source entities in topology.

            Single-level traversal. Use for exploring immediate connections.
            "out": downstream, "in": upstream, "both": bidirectional.

            ## Parameters

            - workspace: CMS workspace identifier
            - src_entity_domain: Source Domain (e.g., "apm", "k8s")
            - src_name: Source EntitySet name (e.g., "apm.service", "k8s.pod")
            - src_entity_ids: Comma-separated source entity IDs
            - dest_entity_domain: Optional target Domain filter
            - dest_name: Optional target EntitySet filter
            - relation_type: Optional relation type filter
            - direction: "out" (downstream), "in" (upstream), "both" (default)
            - time_range: Time range expression, default last_1h
            - limit: Max results, default 10

            ## Example Usage

            ```python
            # Get downstream neighbors
            umodel_get_neighbor_entities(
                workspace="apm",
                src_entity_domain="apm",
                src_name="apm.service",
                src_entity_ids="payment-service-001",
                direction="out"
            )

            # Get upstream neighbors filtered by target type
            umodel_get_neighbor_entities(
                workspace="apm",
                src_entity_domain="apm",
                src_name="apm.service",
                src_entity_ids="payment-service-001",
                dest_entity_domain="k8s",
                dest_name="k8s.pod",
                direction="in"
            )

            # Get neighbors with specific relation type
            umodel_get_neighbor_entities(
                workspace="apm",
                src_entity_domain="apm",
                src_name="apm.service",
                src_entity_ids="svc-1,svc-2",
                relation_type="calls",
                direction="both"
            )
            ```

            Returns:
                Standard response with error, data, message, query, time_range fields
            """
            # Validate required params
            self._validate_required_params(
                {
                    "src_entity_domain": src_entity_domain,
                    "src_name": src_name,
                    "src_entity_ids": src_entity_ids
                },
                ["src_entity_domain", "src_name", "src_entity_ids"]
            )

            # Validate direction
            if direction not in ("in", "out", "both"):
                raise ValueError(
                    f"Invalid direction: {direction}. Must be 'in', 'out', or 'both'"
                )

            # Parse time range
            from_ts, to_ts = self._parse_time_range(time_range)

            # Build entity IDs param for SPL
            entity_ids_spl = self._parse_entity_ids_to_spl_param(src_entity_ids)

            # Build optional params for get_neighbor_entities()
            dest_domain_param = self._parse_string_to_spl_param(dest_entity_domain)
            dest_name_param = self._parse_string_to_spl_param(dest_name)
            relation_type_param = self._parse_string_to_spl_param(relation_type)
            direction_param = self._parse_direction_to_spl_param(direction)

            # Build query matching Go implementation
            query = (
                f".entity_set with(domain='{src_entity_domain}', name='{src_name}', ids={entity_ids_spl}) "
                f"| entity-call get_neighbor_entities({dest_domain_param}, {dest_name_param}, [], '', {relation_type_param}, {direction_param}) "
                f"| limit {limit}"
            )

            # Execute query
            result = execute_cms_query_with_context(
                ctx, query, workspace, regionId, from_ts, to_ts, limit
            )

            # Extract data from result
            data = result.get("data") if isinstance(result, dict) else result

            # Build standard response
            return self._build_standard_response(
                data=data,
                query=query,
                time_range=(from_ts, to_ts),
                error=False,
                message="",
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
        def umodel_search_entities(
            ctx: Context,
            workspace: str = Field(
                ..., description="CMS工作空间名称，可通过list_workspace获取"
            ),
            search_text: str = Field(
                None, description="搜索关键词（全文搜索），支持关键词、IP、服务名等"
            ),
            domain: str = Field(
                "*", description="实体域，可以为 '*' 表示搜索所有域，如 'apm'、'infrastructure' 等"
            ),
            entity_set_name: str = Field(
                "*", description="实体类型，可以为 '*' 表示搜索所有类型，如 'apm.service'、'host.instance' 等"
            ),
            time_range: Optional[str] = Field(
                "last_1h",
                description=TIME_RANGE_DESCRIPTION
            ),
            limit: int = Field(
                10, description="返回多少个详细搜索结果，默认10个", ge=1, le=1000
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """基于关键词全文搜索实体信息。

            ## 功能概述

            该工具用于在指定的实体集合中根据关键词进行全文搜索，查找名称或属性
            包含搜索关键词的实体。返回 'statistics'（按类型统计匹配数量）和 
            'detail'（匹配的实体详情）两部分数据。

            ## 功能特点

            - **全文检索**: 支持对实体名称和属性进行模糊搜索
            - **统计信息**: 返回按实体类型分组的匹配数量统计
            - **详细结果**: 返回匹配的实体详细信息
            - **灵活过滤**: 支持通配符 '*' 搜索所有域或类型

            ## 使用场景

            - **服务搜索**: 根据服务名称片段搜索相关的微服务实体
            - **基础设施搜索**: 根据主机名或IP地址搜索基础设施实体
            - **快速定位**: 在大量实体中搜索包含特定关键词的实体
            - **实体发现**: 作为主要的实体发现工具

            ## 参数说明

            - workspace: CMS工作空间名称（必需）
            - search_text: 搜索关键词，支持部分匹配和模糊搜索
            - domain: 实体集合的域，可以为 '*' 表示搜索所有域
            - entity_set_name: 实体集合名称，可以为 '*' 表示搜索所有类型
            - time_range: 时间范围表达式，默认 last_1h
            - limit: 返回多少个详细搜索结果，默认10个

            ## 返回数据说明

            返回包含两部分数据：
            - **statistics**: 按实体类型分组的匹配数量统计
              - __domain__: 实体域
              - __name__: 实体类型名称
              - match_count: 匹配数量
              - __arbitrary_entity_id__: 示例实体ID
            - **detail**: 匹配的实体详细信息列表

            ## 示例用法

            ```
            # 搜索包含"payment"关键词的服务
            umodel_search_entities(
                workspace="apm",
                search_text="payment",
                domain="apm",
                entity_set_name="apm.service"
            )

            # 搜索所有域中包含特定IP的实体
            umodel_search_entities(
                workspace="apm",
                search_text="192.168.1",
                domain="*",
                entity_set_name="*",
                limit=20
            )

            # 使用时间范围参数
            umodel_search_entities(
                workspace="apm",
                search_text="order",
                time_range="last_3h"
            )
            ```

            Args:
                ctx: MCP上下文，用于访问CMS客户端
                workspace: CMS工作空间名称
                search_text: 搜索关键词
                domain: 实体集合域名，可以为 '*'
                entity_set_name: 实体集合名称，可以为 '*'
                time_range: 时间范围表达式，默认 last_1h
                limit: 返回详细搜索结果数量
                regionId: 阿里云区域ID

            Returns:
                标准化响应对象，包含 error, data, message, query, time_range 字段
                data 包含 statistics 和 detail 两部分
            """
            # 解析时间范围
            from_ts, to_ts = self._parse_time_range(time_range)

            # 构建基础查询
            query_basic = f".entity with(domain='{domain}', type='{entity_set_name}'"
            if search_text and search_text.strip():
                query_basic += f", query='{search_text}'"
            query_basic += ")"

            # 统计查询 - 按域和实体类型分组统计匹配数量
            query_stats = (
                query_basic + 
                " | stats __arbitrary_entity_id__ = arbitrary(__entity_id__), match_count = count(1) by __domain__, __entity_type__"
                " | project __arbitrary_entity_id__ = __arbitrary_entity_id__, __domain__ = __domain__, __name__ = __entity_type__, match_count"
                " | sort match_count desc | limit 100"
            )

            # 详情查询 - 返回匹配的实体详情
            query_detail = query_basic + f" | limit {limit}"

            # 执行统计查询
            stats_result = execute_cms_query_with_context(
                ctx, query_stats, workspace, regionId, from_ts, to_ts, 100
            )

            # 执行详情查询
            detail_result = execute_cms_query_with_context(
                ctx, query_detail, workspace, regionId, from_ts, to_ts, limit
            )

            # 提取数据
            statistics_data = stats_result.get("data") if isinstance(stats_result, dict) else stats_result
            detail_data = detail_result.get("data") if isinstance(detail_result, dict) else detail_result

            # 构建组合数据
            combined_data = {
                "statistics": statistics_data,
                "detail": detail_data
            }

            # 构建标准响应
            return self._build_standard_response(
                data=combined_data,
                query=f"statistics: {query_stats}\ndetail: {query_detail}",
                time_range=(from_ts, to_ts),
                error=False,
                message="",
                time_range_expression=time_range if time_range else "last_1h"
            )

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
        对于 domain 和 entity_set_name 参数，会检查是否为通配符 '*'。
        验证失败时抛出 ValueError 并列出缺失参数，错误消息包含建议的获取方法。

        Args:
            params: 参数字典，包含待验证的参数
            required: 必填参数名列表

        Raises:
            ValueError: 参数缺失或为通配符时抛出，包含具体要求
        """
        # 收集缺失的参数
        missing_params: List[str] = []

        for param_name in required:
            value = params.get(param_name)

            # 检查参数是否缺失或为空
            if value is None or (isinstance(value, str) and value.strip() == ""):
                missing_params.append(param_name)
                continue

            # 检查 domain 参数是否为通配符 '*'
            if param_name == "domain" and isinstance(value, str) and value.strip() == "*":
                raise ValueError(
                    "domain 不能为 '*'，请使用 umodel_search_entity_set 获取有效的 domain 值"
                )

            # 检查 entity_set_name 参数是否为通配符 '*'
            if param_name == "entity_set_name" and isinstance(value, str) and value.strip() == "*":
                raise ValueError(
                    "entity_set_name 不能为 '*'，请使用 umodel_search_entity_set 获取有效值"
                )

        # 如果有缺失的参数，抛出异常
        if missing_params:
            param_names = ", ".join(missing_params)
            raise ValueError(
                f"缺少必填参数: {param_names}，请提供有效值"
            )

    def _build_entity_filter_param(self, entity_filter: Optional[str]) -> str:
        """Build entity filter parameter for SPL queries"""
        if not entity_filter or not entity_filter.strip():
            return ""

        # Convert simple expressions to SQL syntax
        sql_expr = self._convert_to_sql_syntax(entity_filter.strip())
        return f", query=`{sql_expr}`"

    def _build_entity_ids_param(self, entity_ids: Optional[str]) -> str:
        """Build entity IDs parameter for SPL queries"""
        if not entity_ids or not entity_ids.strip():
            return ""

        parts = [id.strip() for id in entity_ids.split(",") if id.strip()]
        quoted = [f"'{id}'" for id in parts]
        return f", ids=[{','.join(quoted)}]"

    def _convert_to_sql_syntax(self, expr: str) -> str:
        """Convert simple filter expressions to SQL syntax"""
        # Handle 'and' operations
        conditions = [c.strip() for c in expr.split(" and ") if c.strip()]
        sql_conditions = []

        for condition in conditions:
            # Parse condition: field operator value
            if "!=" in condition:
                parts = condition.split("!=", 1)
                field = parts[0].strip().strip("'\"")
                value = parts[1].strip().strip("'\"")
                sql_conditions.append(f"\"{field}\"!='{value}'")
            elif "=" in condition:
                parts = condition.split("=", 1)
                field = parts[0].strip().strip("'\"")
                value = parts[1].strip().strip("'\"")
                sql_conditions.append(f"\"{field}\"='{value}'")
            else:
                raise ValueError(f"Invalid condition format: {condition}")

        return " and ".join(sql_conditions)

    def _parse_entity_ids_to_spl_param(self, entity_ids: str) -> str:
        """Parse comma-separated entity IDs to SPL array format.
        
        Args:
            entity_ids: Comma-separated entity IDs string
            
        Returns:
            SPL array format string, e.g., "['id1','id2']"
        """
        parts = [id.strip() for id in entity_ids.split(",") if id.strip()]
        quoted = [f"'{id}'" for id in parts]
        return f"[{','.join(quoted)}]"

    def _parse_string_to_spl_param(self, value: Optional[str]) -> str:
        """Parse optional string to SPL parameter format.
        
        Args:
            value: Optional string value
            
        Returns:
            SPL string format "'value'" or "''" if None/empty
        """
        if value is None or value.strip() == "":
            return "''"
        return f"'{value.strip()}'"

    def _parse_direction_to_spl_param(self, direction: Optional[str]) -> str:
        """Parse direction to SPL parameter format.
        
        Args:
            direction: Direction string ("in", "out", "both")
            
        Returns:
            SPL string format, defaults to "'both'"
        """
        if direction is None or direction.strip() == "":
            return "'both'"
        return f"'{direction.strip()}'"
