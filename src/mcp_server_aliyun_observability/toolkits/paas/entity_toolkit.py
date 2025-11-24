from typing import Any, Dict, Optional, Union

from mcp.server.fastmcp import Context, FastMCP
from pydantic import Field
from tenacity import retry, retry_if_exception_type, stop_after_attempt, wait_fixed

from mcp_server_aliyun_observability.config import Config
from mcp_server_aliyun_observability.utils import (
    execute_cms_query_with_context,
    handle_tea_exception,
)


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
                None, description="可选的逗号分隔实体ID列表，用于精确查询指定实体"
            ),
            from_time: Union[str, int] = Field(
                "now-5m", description="开始时间: Unix时间戳(秒/毫秒)或相对时间(now-5m)"
            ),
            to_time: Union[str, int] = Field(
                "now", description="结束时间: Unix时间戳(秒/毫秒)或相对时间(now)"
            ),
            limit: int = Field(
                20, description="返回多少个实体，默认20个", ge=1, le=1000
            ),
            regionId: str = Field(..., description="Region ID like 'cn-hangzhou'"),
        ) -> Dict[str, Any]:
            """获取实体信息的PaaS API工具。

            ## 功能概述

            该工具用于检索实体信息，支持分页查询和精确ID查询。专注于实体的基础信息获取，
            如需要模糊搜索请使用 `umodel_search_entities` 工具。

            ## 功能特点

            - **数量控制**: 默认返回20个实体，支持通过limit参数控制返回数量
            - **全量查询**: 支持获取指定实体集合下的所有实体（分页返回）
            - **精确查询**: 支持根据实体ID列表进行精确查询
            - **职责清晰**: 专注于基础实体信息获取，不包含复杂过滤逻辑

            ## 使用场景

            - **分页浏览**: 分页获取实体列表，适用于大量实体的展示场景
            - **精确查询**: 根据已知的实体ID列表批量获取实体详细信息
            - **全量获取**: 获取指定实体集合下的所有实体信息
            - **基础数据**: 为其他分析工具提供基础实体数据

            ## 参数说明

            - domain: 实体集合的域，如 'apm'、'infrastructure' 等
            - entity_set_name: 实体集合名称，如 'apm.service'、'host.instance' 等
            - entity_ids: 可选的逗号分隔实体ID字符串，用于精确查询指定实体
            - from_time/to_time: 查询时间范围，支持时间戳和相对时间表达式
            - limit: 返回多少个实体，默认20个，最大1000个

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
            # 获取前20个服务实体（默认数量）
            umodel_get_entities(
                domain="apm",
                entity_set_name="apm.service"
            )

            # 获取前100个服务实体
            umodel_get_entities(
                domain="apm",
                entity_set_name="apm.service",
                limit=100
            )

            # 根据实体ID批量查询
            umodel_get_entities(
                domain="apm",
                entity_set_name="apm.service",
                entity_ids="service-1,service-2,service-3"
            )
            ```

            Args:
                ctx: MCP上下文，用于访问CMS客户端
                domain: 实体集合域名
                entity_set_name: 实体集合名称
                entity_ids: 可选的逗号分隔实体ID列表
                from_time: 查询开始时间
                to_time: 查询结束时间
                limit: 返回实体数量
                regionId: 阿里云区域ID

            Returns:
                包含实体信息的响应对象，包括实体列表和查询元数据
            """
            # Build entity IDs parameter if provided
            entity_ids_param = self._build_entity_ids_param(entity_ids)

            # 验证domain和entity_set_name不能为通配符
            if domain == "*":
                raise ValueError(
                    "domain parameter cannot be '*', must be a specific domain like 'apm'"
                )
            if entity_set_name == "*":
                raise ValueError(
                    "entity_set_name parameter cannot be '*', must be a specific entity type like 'apm.service'"
                )

            # 有了分页支持后，过滤条件不再是必填的

            # 简化查询，只使用limit参数
            query = f".entity_set with(domain='{domain}', name='{entity_set_name}'{entity_ids_param}) | entity-call get_entities() | limit {limit}"

            return execute_cms_query_with_context(
                ctx, query, workspace, regionId, from_time, to_time, limit
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
            domain: str = Field(..., description="实体域, cannot be '*'"),
            entity_set_name: str = Field(..., description="实体类型, cannot be '*'"),
            entity_id: str = Field(..., description="目标实体ID，用于查找其邻居实体"),
            workspace: str = Field(
                ..., description="CMS工作空间名称，可通过list_workspace获取"
            ),
            from_time: Union[str, int] = Field(
                "now-5m", description="开始时间: Unix时间戳(秒/毫秒)或相对时间(now-5m)"
            ),
            to_time: Union[str, int] = Field(
                "now", description="结束时间: Unix时间戳(秒/毫秒)或相对时间(now)"
            ),
            limit: int = Field(
                20, description="返回多少个邻居实体，默认20个", ge=1, le=1000
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """获取指定实体的邻居实体信息。

            ## 功能概述

            该工具用于查找与指定实体相关联的邻居实体，基于实体间的关联关系
            返回相邻的实体列表。

            ## 功能特点

            - **数量控制**: 默认返回20个邻居实体，支持通过limit参数控制返回数量
            - **关系分析**: 基于实体间的关联关系进行邻居发现
            - **拓扑构建**: 为拓扑分析和依赖关系追踪提供数据支持

            ## 使用场景

            - **依赖分析**: 获取应用服务的上下游依赖关系，查找调用链中的相关服务
            - **关联发现**: 查找与主机实例相关的容器、进程等关联实体
            - **故障影响**: 在故障诊断时，识别可能受影响的相关实体列表
            - **拓扑构建**: 构建实体关系图和拓扑结构

            ## 参数说明

            - domain: 实体集合的域，如 'apm'、'infrastructure' 等
            - entity_set_name: 实体集合名称，如 'apm.service'、'host.instance' 等
            - entity_id: 目标实体的唯一标识符
            - from_time/to_time: 查询时间范围，用于获取时间段内的关联关系
            - limit: 返回多少个邻居实体，默认20个，最大1000个

            ## 数量控制说明

            使用 `|limit {count}` 格式控制返回数量：
            - 返回前10个邻居实体：limit=10 → `|limit 10`
            - 返回前50个邻居实体：limit=50 → `|limit 50`
            - 返回前100个邻居实体：limit=100 → `|limit 100`

            ## 示例用法

            ```
            # 获取前20个邻居实体（默认数量）
            umodel_get_neighbor_entities(
                domain="apm",
                entity_set_name="apm.service",
                entity_id="payment-service-001"
            )

            # 获取前50个邻居实体
            umodel_get_neighbor_entities(
                domain="apm",
                entity_set_name="apm.service",
                entity_id="payment-service-001",
                limit=50
            )

            # 获取特定时间范围内的邻居关系
            umodel_get_neighbor_entities(
                domain="apm",
                entity_set_name="apm.service",
                entity_id="payment-service-001",
                from_time="now-1h",
                to_time="now"
            )
            ```

            Args:
                ctx: MCP上下文，用于访问CMS客户端
                domain: 实体集合域名
                entity_set_name: 实体集合名称
                entity_id: 目标实体ID
                from_time: 查询开始时间
                to_time: 查询结束时间
                limit: 返回邻居实体数量
                regionId: 阿里云区域ID

            Returns:
                包含邻居实体信息的响应对象
            """
            query = f".entity_set with(domain='{domain}', name='{entity_set_name}', ids=['{entity_id}']) | entity-call get_neighbor_entities() | limit {limit}"
            return execute_cms_query_with_context(
                ctx, query, workspace, regionId, from_time, to_time, limit
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
            domain: str = Field(..., description="实体域, can be '*'"),
            entity_set_name: str = Field(..., description="实体类型, can be '*'"),
            search_text: str = Field(..., description="搜索关键词"),
            workspace: str = Field(
                ..., description="CMS工作空间名称，可通过list_workspace获取"
            ),
            from_time: Union[str, int] = Field(
                "now-5m", description="开始时间: Unix时间戳(秒/毫秒)或相对时间(now-5m)"
            ),
            to_time: Union[str, int] = Field(
                "now", description="结束时间: Unix时间戳(秒/毫秒)或相对时间(now)"
            ),
            limit: int = Field(
                20, description="返回多少个搜索结果，默认20个", ge=1, le=1000
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """基于关键词搜索实体信息。

            ## 功能概述

            该工具用于在指定的实体集合中根据关键词进行模糊搜索，查找名称或属性
            包含搜索关键词的实体。支持全文检索，帮助用户快速定位相关实体。

            ## 功能特点

            - **数量控制**: 默认返回20个搜索结果，支持通过limit参数控制返回数量
            - **全文检索**: 支持对实体名称和属性进行模糊搜索
            - **灵活过滤**: 可以在大量实体中快速定位目标实体

            ## 使用场景

            - **服务搜索**: 根据服务名称片段搜索相关的微服务实体
            - **基础设施搜索**: 根据主机名或IP地址搜索基础设施实体
            - **快速定位**: 在大量实体中搜索包含特定关键词的实体
            - **智能筛选**: 为实体选择和筛选提供搜索能力

            ## 参数说明

            - domain: 实体集合的域，如 'apm'、'infrastructure' 等
            - entity_set_name: 实体集合名称，如 'apm.service'、'host.instance' 等
            - search_text: 搜索关键词，支持部分匹配和模糊搜索
            - from_time/to_time: 搜索时间范围，限定搜索的时间窗口
            - limit: 返回多少个搜索结果，默认20个，最大1000个

            ## 数量控制说明

            使用 `|limit {count}` 格式控制返回数量：
            - 返回前10个搜索结果：limit=10 → `|limit 10`
            - 返回前50个搜索结果：limit=50 → `|limit 50`
            - 返回前100个搜索结果：limit=100 → `|limit 100`

            ## 示例用法

            ```
            # 搜索包含"payment"关键词的服务（默认数量）
            umodel_search_entities(
                domain="apm",
                entity_set_name="apm.service",
                search_text="payment"
            )

            # 获取前50个搜索结果
            umodel_search_entities(
                domain="apm",
                entity_set_name="apm.service",
                search_text="payment",
                limit=50
            )

            # 搜索包含特定IP的主机实例
            umodel_search_entities(
                domain="infrastructure",
                entity_set_name="host.instance",
                search_text="192.168.1",
                limit=100
            )
            ```

            Args:
                ctx: MCP上下文，用于访问CMS客户端
                domain: 实体集合域名
                entity_set_name: 实体集合名称
                search_text: 搜索关键词
                from_time: 搜索开始时间
                to_time: 搜索结束时间
                limit: 返回搜索结果数量
                regionId: 阿里云区域ID

            Returns:
                包含搜索结果的响应对象
            """
            query = f".entity with(domain='{domain}', name='{entity_set_name}', query='{search_text}') | limit {limit}"
            return execute_cms_query_with_context(
                ctx, query, workspace, regionId, from_time, to_time, limit
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
