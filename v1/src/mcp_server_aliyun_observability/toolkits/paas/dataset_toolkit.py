from typing import Any, Dict, Optional, Union

from mcp.server.fastmcp import Context, FastMCP
from pydantic import Field
from tenacity import retry, retry_if_exception_type, stop_after_attempt, wait_fixed

from mcp_server_aliyun_observability.config import Config
from mcp_server_aliyun_observability.utils import (
    execute_cms_query_with_context,
    handle_tea_exception,
)


class PaaSDatasetToolkit:
    """PaaS Dataset Management Toolkit

    Provides structured dataset query tools ported from umodel metadata handlers.
    """

    def __init__(self, server: FastMCP):
        self.server = server
        self._register_tools()

    def _register_tools(self):
        """Register metadata-related PaaS tools"""

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(Config.get_retry_attempts()),
            wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def umodel_list_data_set(
            ctx: Context,
            workspace: str = Field(
                ..., description="CMS工作空间名称，可通过list_workspace获取"
            ),
            domain: str = Field(..., description="实体域, cannot be '*'"),
            entity_set_name: str = Field(..., description="实体类型, cannot be '*'"),
            data_set_types: Optional[str] = Field(
                None, description="Comma-separated data set types"
            ),
            from_time: Union[str, int] = Field(
                "now-5m", description="开始时间: Unix时间戳(秒/毫秒)或相对时间(now-5m)"
            ),
            to_time: Union[str, int] = Field(
                "now", description="结束时间: Unix时间戳(秒/毫秒)或相对时间(now)"
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """列出指定实体的可用数据集合，为其他PaaS工具提供参数选项。

            ## 功能概述

            该工具是一个元数据查询接口，用于获取指定实体域和类型下可用的数据集合信息。
            主要作用是为其他PaaS层工具（如observability、entity等工具）提供可选的参数列表，
            包括指标集合、日志集合、事件集合等存储信息。

            ## 使用场景

            - **参数发现**: 为 umodel_get_metrics 提供可用的指标集合（metric_set）列表
            - **日志源查询**: 为 umodel_get_logs 提供可用的日志集合（log_set）列表
            - **事件源发现**: 为 umodel_get_events 提供可用的事件集合（event_set）列表
            - **追踪数据源**: 为 umodel_get_traces 提供可用的追踪集合（trace_set）列表
            - **实体关联**: 为实体查询工具提供关联的数据集合信息

            ## 参数说明

            - data_set_types: 数据集合类型过滤器，常见类型包括：
              * 'metric_set': 指标集合，用于获取可查询的指标名称列表
              * 'log_set': 日志集合，用于获取可查询的日志数据源
              * 'event_set': 事件集合，用于获取可查询的事件数据源
              * 'trace_set': 追踪集合，用于获取可查询的追踪数据源
              * 'entity_set_name': 实体集合，用于获取实体关联的数据集合

            ## 工具依赖关系

            这个工具的输出为其他PaaS工具提供参数选项：
            - umodel_get_metrics → 使用返回的 metric_set 信息选择指标
            - umodel_get_logs → 使用返回的 log_set 信息选择日志源
            - umodel_get_events → 使用返回的 event_set 信息选择事件源
            - umodel_get_traces → 使用返回的 trace_set 信息选择追踪源

            ## 示例用法

            ```
            # 获取服务实体的所有数据集合类型
            umodel_list_data_set(domain="apm", entity_set_name="apm.service")

            # 仅获取指标集合，用于后续指标查询
            umodel_list_data_set(
                domain="apm",
                entity_set_name="apm.service",
                data_set_types="metric_set"
            )

            # 获取日志和事件集合，用于故障分析
            umodel_list_data_set(
                domain="apm",
                entity_set_name="apm.service",
                data_set_types="log_set,event_set"
            )
            ```

            Args:
                ctx: MCP上下文，用于访问CMS客户端
                workspace: CMS工作空间名称
                domain: 实体域，不能为通配符 '*'
                entity_set_name: 实体类型，不能为通配符 '*'
                data_set_types: 数据集合类型过滤器，逗号分隔的类型列表
                from_time: 查询开始时间
                to_time: 查询结束时间
                regionId: 阿里云区域ID

            Returns:
                包含指定实体的可用数据集合列表，每个数据集合包含名称、类型、存储信息等元数据
            """
            types_param = ""
            if data_set_types:
                types_list = [f"'{t.strip()}'" for t in data_set_types.split(",")]
                types_param = f"[{','.join(types_list)}]"
            else:
                types_param = "[]"
            query = f".entity_set with(domain='{domain}', name='{entity_set_name}') | entity-call list_data_set({types_param})"
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
        def umodel_search_entity_set(
            ctx: Context,
            search_text: str = Field(..., description="搜索关键词，用于全文搜索"),
            workspace: str = Field(
                ..., description="CMS工作空间名称，可通过list_workspace获取"
            ),
            domain: Optional[str] = Field(None, description="可选的实体域过滤"),
            entity_set_name: Optional[str] = Field(
                None, description="可选的实体类型过滤"
            ),
            limit: int = Field(
                10, description="返回多少个实体集合，默认10个", ge=1, le=100
            ),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """搜索实体集合，支持全文搜索并按相关度排序。

            ## 功能概述

            该工具用于在UModel元数据中搜索实体集合定义，支持按关键词进行全文搜索。
            主要用于发现可用的实体集合类型和它们的元数据信息。

            ## 功能特点

            - **全文搜索**: 支持在实体集合的元数据和规格中进行全文搜索
            - **相关度排序**: 搜索结果按相关度进行排序
            - **元数据查询**: 返回实体集合的domain、name、display_name等元数据信息
            - **可选过滤**: 支持按domain和name进行额外过滤

            ## 使用场景

            - **实体集合发现**: 搜索包含特定关键词的实体集合类型
            - **元数据探索**: 了解系统中可用的实体集合及其描述信息
            - **工具链集成**: 为其他工具提供实体集合的发现能力

            Args:
                ctx: MCP上下文，用于访问CMS客户端
                search_text: 搜索关键词
                workspace: CMS工作空间名称
                domain: 可选的域过滤
                entity_set_name: 可选的实体类型过滤
                limit: 返回结果数量限制
                regionId: 阿里云区域ID

            Returns:
                包含搜索到的实体集合信息的响应对象
            """
            # 基于Go实现构建SPL查询
            query = ".umodel | where kind = 'entity_set' and __type__ = 'node'"

            if domain:
                query += (
                    f" | where json_extract_scalar(metadata, '$.domain') = '{domain}'"
                )
            if entity_set_name:
                query += f" | where json_extract_scalar(metadata, '$.name') = '{entity_set_name}'"
            # 添加全文搜索过滤
            query += f" | where strpos(metadata, '{search_text}') > 0 or strpos(spec, '{search_text}') > 0"

            # 只返回name列表，简化输出
            query += " | extend name = json_extract_scalar(metadata, '$.name') | project name | limit 100"

            result = execute_cms_query_with_context(
                ctx, query, workspace, regionId, "now-1h", "now", limit
            )
            return result

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(Config.get_retry_attempts()),
            wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def umodel_list_related_entity_set(
            ctx: Context,
            domain: str = Field(..., description="实体域，如'apm'"),
            entity_set_name: str = Field(..., description="实体类型，如'apm.service'"),
            workspace: str = Field(
                ..., description="CMS工作空间名称，可通过list_workspace获取"
            ),
            relation_type: Optional[str] = Field(
                None, description="关系类型过滤，如'calls'"
            ),
            direction: str = Field(
                "both", description="关系方向: 'in', 'out', 或 'both'"
            ),
            detail: bool = Field(False, description="是否返回详细信息"),
            regionId: str = Field(..., description="Region ID"),
        ) -> Dict[str, Any]:
            """列出与指定实体集合相关的其他实体集合。

            ## 功能概述

            该工具用于发现与指定实体集合存在关系定义的其他实体集合类型。
            这是一个元数据级别的工具，用于探索UModel拓扑的高级蓝图。

            ## 功能特点

            - **关系发现**: 查找与源实体集合有关系定义的其他实体集合
            - **方向控制**: 支持查看入向、出向或双向关系
            - **类型过滤**: 可按特定关系类型进行过滤
            - **元数据级别**: 显示可能的关系，不保证实际实体间存在连接

            ## 使用场景

            - **拓扑探索**: 了解实体集合间可能存在的关系类型
            - **依赖分析**: 发现服务可以调用的其他实体类型
            - **关系建模**: 为关系查询工具提供参数信息

            Args:
                ctx: MCP上下文，用于访问CMS客户端
                domain: 源实体集合的域
                entity_set_name: 源实体集合的名称
                workspace: CMS工作空间名称
                relation_type: 可选的关系类型过滤
                direction: 关系方向
                detail: 是否返回详细信息
                regionId: 阿里云区域ID

            Returns:
                包含相关实体集合信息的响应对象
            """
            # 构建参数
            relation_type_param = f"'{relation_type}'" if relation_type else "''"
            direction_param = f"'{direction}'"
            detail_param = "true" if detail else "false"

            # 基于Go实现构建查询
            query = f".entity_set with(domain='{domain}', name='{entity_set_name}') | entity-call list_related_entity_set({relation_type_param}, {direction_param}, {detail_param})"

            return execute_cms_query_with_context(
                ctx, query, workspace, regionId, "now-1h", "now", 1000
            )
