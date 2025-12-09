
import json

from typing import Any, Dict, List, Optional, Union, Tuple

from alibabacloud_sls20201230.client import Client as SLSClient
from alibabacloud_sls20201230.models import (
    CallAiToolsRequest,
    CallAiToolsResponse,
    GetHistogramsRequest,
    GetHistogramsResponse,
    GetLogsRequest,
    GetLogsResponse,
    ListLogStoresRequest,
    ListLogStoresResponse,
    ListProjectRequest,
    ListProjectResponse,
)
from alibabacloud_tea_util import models as util_models
from mcp.server.fastmcp import Context, FastMCP
from pydantic import Field
from tenacity import retry, retry_if_exception_type, stop_after_attempt, wait_fixed

from mcp_server_aliyun_observability.config import Config
from mcp_server_aliyun_observability.logger import log_error
from mcp_server_aliyun_observability.utils import (
    execute_cms_query_with_context,
    handle_tea_exception,
)
from mcp_server_aliyun_observability.utils import text_to_sql as utils_text_to_sql


class IaaSToolkit:
    """Infrastructure as a Service Layer Toolkit

    Provides basic infrastructure tools for SLS database query operations:
    - sls_text_to_sql: Convert natural language to SQL queries
    - sls_execute_sql: Execute SQL queries against SLS
    - cms_execute_promql: Execute PromQL queries against CMS
    - sls_execute_spl: Execute raw SPL queries
    - sls_list_projects: List SLS projects in a region
    - sls_list_logstores: List log stores in an SLS project
    """

    def __init__(self, server: FastMCP):
        """Initialize the IaaS toolkit

        Args:
            server: FastMCP server instance
        """
        self.server = server
        self._register_tools()

    def _register_tools(self):
        """Register IaaS layer tools"""

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(Config.get_retry_attempts()),
            wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def cms_text_to_promql(
            ctx: Context,
            text: str = Field(
                ..., description="the natural language text to generate promql"
            ),
            project: str = Field(..., description="sls project name"),
            metricStore: str = Field(..., description="sls metric store name"),
            regionId: str = Field(default=..., description="aliyun region id"),
        ) -> str:
            """将自然语言转换为PromQL查询语句。

            ## 功能概述

            该工具可以将自然语言描述转换为有效的PromQL查询语句，便于用户使用自然语言表达查询需求。

            ## 使用场景

            - 当用户不熟悉PromQL查询语法时
            - 当需要快速构建复杂查询时
            - 当需要从自然语言描述中提取查询意图时

            ## 使用限制

            - 仅支持生成PromQL查询
            - 生成的是查询语句，而非查询结果


            ## 最佳实践

            - 提供清晰简洁的自然语言描述
            - 不要在描述中包含项目或时序库名称
            - 首次生成的查询可能不完全符合要求，可能需要多次尝试

            ## 查询示例

            - "帮我生成 XXX 的PromQL查询语句"
            - "查询每个namespace下的Pod数量"

            Args:
                ctx: MCP上下文，用于访问CMS客户端
                text: 用于生成查询的自然语言文本
                project: SLS项目名称
                metricStore: SLS时序库名称
                regionId: 阿里云区域ID

            Returns:
                生成的PromQL查询语句
            """
            try:
                sls_client: SLSClient = ctx.request_context.lifespan_context[
                    "sls_client"
                ].with_region("cn-shanghai")
                request: CallAiToolsRequest = CallAiToolsRequest()
                request.tool_name = "text_to_promql"
                request.region_id = regionId
                params: dict[str, Any] = {
                    "project": project,
                    "metricstore": metricStore,
                    "sys.query": text,
                }
                request.params = params
                runtime: util_models.RuntimeOptions = util_models.RuntimeOptions()
                runtime.read_timeout = 60000
                runtime.connect_timeout = 60000
                tool_response: CallAiToolsResponse = (
                    sls_client.call_ai_tools_with_options(
                        request=request, headers={}, runtime=runtime
                    )
                )
                data = tool_response.body
                if "------answer------\n" in data:
                    data = data.split("------answer------\n")[1]
                return data
            except Exception as e:
                log_error(f"调用CMS AI工具失败: {str(e)}")
                raise

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(Config.get_retry_attempts()),
            wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def sls_text_to_sql(
            ctx: Context,
            text: str = Field(
                ...,
                description="the natural language text to generate sls log store query",
            ),
            project: str = Field(..., description="sls project name"),
            logStore: str = Field(..., description="sls log store name"),
            regionId: str = Field(
                default=...,
                description="aliyun region id,region id format like 'xx-xxx',like 'cn-hangzhou'",
            ),
        ) -> Dict[str, Any]:
            """将自然语言转换为SLS查询语句。当用户有明确的 logstore 查询需求，必须优先使用该工具来生成查询语句

            ## 功能概述

            该工具可以将自然语言描述转换为有效的SLS查询语句，便于用户使用自然语言表达查询需求。用户有任何 SLS 日志查询需求时，都需要优先使用该工具。

            ## 使用场景

            - 当用户不熟悉SLS查询语法时
            - 当需要快速构建复杂查询时
            - 当需要从自然语言描述中提取查询意图时

            ## 使用限制

            - 仅支持生成SLS查询，不支持其他数据库的SQL如MySQL、PostgreSQL等
            - 生成的是查询语句，而非查询结果，需要配合execute_sql工具使用
            - 需要对应的 log_store 已经设定了索引信息，如果生成的结果里面有字段没有索引或者开启统计，可能会导致查询失败，需要友好的提示用户增加相对应的索引信息

            ## 最佳实践

            - 提供清晰简洁的自然语言描述
            - 不要在描述中包含项目或日志库名称
            - 如有需要，指定查询的时间范围
            - 首次生成的查询可能不完全符合要求，可能需要多次尝试

            ## 查询示例

            - "帮我生成下 XXX 的日志查询语句"
            - "查找最近一小时内的错误日志"

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                text: 用于生成查询的自然语言文本
                project: SLS项目名称
                logStore: SLS日志库名称
                regionId: 阿里云区域ID

            Returns:
                生成的SLS查询语句
            """
            return utils_text_to_sql(ctx, text, project, logStore, regionId)

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(Config.get_retry_attempts()),
            wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def sls_execute_sql(
            ctx: Context,
            project: str = Field(..., description="sls project name"),
            logStore: str = Field(..., description="sls log store name"),
            query: str = Field(..., description="query"),
            from_time: Union[str, int] = Field(
                "now-5m",
                description="from time,support unix timestamp or relative time like 'now-5m'",
            ),
            to_time: Union[str, int] = Field(
                "now",
                description="to time,support unix timestamp or relative time like 'now'",
            ),
            limit: int = Field(10, description="limit,max is 100", ge=1, le=100),
            regionId: str = Field(
                default=...,
                description="aliyun region id,region id format like 'xx-xxx',like 'cn-hangzhou'",
            ),
        ) -> Dict[str, Any]:
            """执行SLS日志查询。

            ## 功能概述

            该工具用于在指定的SLS项目和日志库上执行查询语句，并返回查询结果。查询将在指定的时间范围内执行。如果上下文没有提到具体的 SQL 语句，必须优先使用 iaas_sls_text_to_sql 工具生成查询语句，无论问题有多简单

            ## 使用场景

            - 当需要根据特定条件查询日志数据时
            - 当需要分析特定时间范围内的日志信息时
            - 当需要检索日志中的特定事件或错误时
            - 当需要统计日志数据的聚合信息时

            ## 查询语法

            查询必须使用SLS有效的查询语法，而非自然语言。如果不了解日志库的结构，可以先使用iaas_sls_list_logstores工具获取索引信息。

            ## 时间范围

            查询必须指定时间范围：如果查询是由 iaas_sls_text_to_sql 工具生成的，应使用 iaas_sls_text_to_sql 响应中的时间戳
            - from_time: 开始时间，支持Unix时间戳（秒/毫秒）或相对时间表达式（如 'now-1h'）
            - to_time: 结束时间，支持Unix时间戳（秒/毫秒）或相对时间表达式（如 'now'）

            ## 查询示例

            - "帮我查询下 XXX 的日志信息"
            - "查找最近一小时内的错误日志"

            ## 错误处理
            - Column xxx can not be resolved 如果是 iaas_sls_text_to_sql 工具生成的查询语句，可能存在查询列未开启统计，可以提示用户增加相对应的信息，或者调用 iaas_sls_list_logstores 工具获取索引信息之后，要用户选择正确的字段或者提示用户对列开启统计。当确定列开启统计之后，可以再次调用 iaas_sls_text_to_sql 工具生成查询语句

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                project: SLS项目名称
                logStore: SLS日志库名称
                query: SLS查询语句
                from_time: 查询开始时间，支持时间戳或相对时间
                to_time: 查询结束时间，支持时间戳或相对时间
                limit: 返回结果的最大数量，范围1-100，默认10
                regionId: 阿里云区域ID

            Returns:
                查询结果列表，每个元素为一条日志记录
            """
            sls_client: SLSClient = ctx.request_context.lifespan_context[
                "sls_client"
            ].with_region(regionId)

            # Parse time parameters
            from mcp_server_aliyun_observability.toolkits.paas.time_utils import (
                TimeRangeParser,
            )

            from_timestamp = TimeRangeParser.parse_time_expression(from_time)
            to_timestamp = TimeRangeParser.parse_time_expression(to_time)

            request: GetLogsRequest = GetLogsRequest(
                query=query,
                from_=from_timestamp,
                to=to_timestamp,
                line=limit,
            )

            runtime: util_models.RuntimeOptions = util_models.RuntimeOptions()
            runtime.read_timeout = 60000
            runtime.connect_timeout = 60000

            response: GetLogsResponse = sls_client.get_logs_with_options(
                project, logStore, request, headers={}, runtime=runtime
            )

            response_body: List[Dict[str, Any]] = response.body
            return {
                "data": response_body,
                "message": "success" if response_body else "No data found",
            }

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(Config.get_retry_attempts()),
            wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def cms_execute_promql(
            ctx: Context,
            project: str = Field(..., description="sls project name"),
            metricStore: str = Field(..., description="sls metric store name"),
            query: str = Field(..., description="promql query statement"),
            from_time: Union[str, int] = Field(
                "now-5m",
                description="from time,support unix timestamp or relative time like 'now-5m'",
            ),
            to_time: Union[str, int] = Field(
                "now",
                description="to time,support unix timestamp or relative time like 'now'",
            ),
            regionId: str = Field(
                default=...,
                description="aliyun region id,region id format like 'xx-xxx',like 'cn-hangzhou'",
            ),
        ) -> Dict[str, Any]:
            """执行PromQL指标查询。

            ## 功能概述

            该工具用于在指定的SLS项目和指标库上执行PromQL查询语句，并返回时序指标数据。主要用于查询和分析时序指标数据。

            ## 使用场景

            - 当需要查询时序指标数据时
            - 当需要分析系统性能指标时
            - 当需要监控业务指标趋势时
            - 当需要进行指标聚合计算时

            ## 查询语法

            查询必须使用有效的PromQL语法。PromQL是专门用于时序数据查询的表达式语言。

            ## 时间范围

            查询必须指定时间范围：
            - from_time: 开始时间，支持Unix时间戳（秒/毫秒）或相对时间表达式（如 'now-1h'）
            - to_time: 结束时间，支持Unix时间戳（秒/毫秒）或相对时间表达式（如 'now'）

            ## 查询示例

            - "查询CPU使用率指标"
            - "获取内存使用率的时序数据"

            ## 注意事项

            - 确保指定的metric store存在且包含所需的指标数据
            - PromQL语法与SQL语法不同，请使用正确的PromQL表达式
            - 时间范围不宜过大，以免查询超时

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                project: SLS项目名称
                metricStore: SLS指标库名称
                query: PromQL查询语句
                from_time: 查询开始时间，支持时间戳或相对时间
                to_time: 查询结束时间，支持时间戳或相对时间
                regionId: 阿里云区域ID

            Returns:
                PromQL查询执行结果，包含时序指标数据
            """
            sls_client: SLSClient = ctx.request_context.lifespan_context[
                "sls_client"
            ].with_region(regionId)

            # Parse time parameters
            from mcp_server_aliyun_observability.toolkits.paas.time_utils import (
                TimeRangeParser,
            )

            from_timestamp = TimeRangeParser.parse_time_expression(from_time)
            to_timestamp = TimeRangeParser.parse_time_expression(to_time)

            # Wrap PromQL in SLS template
            spl_query = f"""
.set "sql.session.velox_support_row_constructor_enabled" = 'true';
.set "sql.session.presto_velox_mix_run_not_check_linked_agg_enabled" = 'true';
.set "sql.session.presto_velox_mix_run_support_complex_type_enabled" = 'true';
.set "sql.session.velox_sanity_limit_enabled" = 'false';
.metricstore with(promql_query='{query}',range='1m')
| extend latest_ts = element_at(__ts__,cardinality(__ts__)), 
         latest_val = element_at(__value__,cardinality(__value__))
| stats arr_ts = array_agg(__ts__), 
        arr_val = array_agg(__value__), 
        title_agg = array_agg(json_format(cast(__labels__ as json))), 
        cnt = count(*), 
        latest_ts = array_agg(latest_ts), 
        latest_val = array_agg(latest_val)
| project title_agg, cnt, latest_ts, latest_val
"""

            request: GetLogsRequest = GetLogsRequest(
                query=spl_query,
                from_=from_timestamp,
                to=to_timestamp,
            )

            runtime: util_models.RuntimeOptions = util_models.RuntimeOptions()
            runtime.read_timeout = 60000
            runtime.connect_timeout = 60000

            response: GetLogsResponse = sls_client.get_logs_with_options(
                project, metricStore, request, headers={}, runtime=runtime
            )

            response_body: List[Dict[str, Any]] = response.body
            return {
                "data": response_body,
                "message": "success" if response_body else "No data found",
            }

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(Config.get_retry_attempts()),
            wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def sls_list_projects(
            ctx: Context,
            projectName: Optional[str] = Field(
                None, description="project name,fuzzy search"
            ),
            limit: int = Field(
                default=50, description="limit,max is 100", ge=1, le=100
            ),
            regionId: str = Field(default=..., description="aliyun region id"),
        ) -> Dict[str, Any]:
            """列出阿里云日志服务中的所有项目。

            ## 功能概述

            该工具可以列出指定区域中的所有SLS项目，支持通过项目名进行模糊搜索。如果不提供项目名称，则返回该区域的所有项目。

            ## 使用场景

            - 当需要查找特定项目是否存在时
            - 当需要获取某个区域下所有可用的SLS项目列表时
            - 当需要根据项目名称的部分内容查找相关项目时

            ## 返回数据结构

            返回的项目信息包含：
            - project_name: 项目名称
            - description: 项目描述
            - region_id: 项目所在区域

            ## 查询示例

            - "有没有叫 XXX 的 project"
            - "列出所有SLS项目"

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                projectName: 项目名称查询字符串，支持模糊搜索
                limit: 返回结果的最大数量，范围1-100，默认50
                regionId: 阿里云区域ID,region id format like "xx-xxx",like "cn-hangzhou"

            Returns:
                包含项目信息的字典列表，每个字典包含project_name、description和region_id
            """
            sls_client: SLSClient = ctx.request_context.lifespan_context[
                "sls_client"
            ].with_region(regionId)

            request: ListProjectRequest = ListProjectRequest(
                project_name=projectName,
                size=limit,
            )
            response: ListProjectResponse = sls_client.list_project(request)

            return {
                "projects": [
                    {
                        "project_name": project.project_name,
                        "description": project.description,
                        "region_id": project.region,
                    }
                    for project in response.body.projects
                ],
                "message": f"当前最多支持查询{limit}个项目，未防止返回数据过长，如果需要查询更多项目，您可以提供 project 的关键词来模糊查询",
            }

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(Config.get_retry_attempts()),
            wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def sls_execute_spl(
            ctx: Context,
            query: str = Field(..., description="Raw SPL query statement"),
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
            """执行原SPL查询语句。

            ## 功能概述

            该工具允许直接执行原生SPL（Search Processing Language）查询语句，
            为高级用户提供最大的灵活性和功能。支持复杂的数据操作和分析。

            ## 使用场景

            - 复杂的数据分析和统计计算，超出标准API的覆盖范围
            - 自定义的数据聚合和转换操作
            - 跨多个实体集合的联合查询和关联分析
            - 高级的数据挖掘和机器学习分析
            - 业务方法验证和原型开发

            ## 注意事项

            - 需要对SPL语法有一定的了解
            - 请确保查询语句的正确性，错误的查询可能导致无结果或错误
            - 复杂查询可能消耗较多的计算资源和时间

            ## 示例用法

            ```
            # 执行简单的SPL查询
            sls_execute_spl(
                query=".entity_set with(domain='apm', name='apm.service') | entity-call get_entities() | head 10"
            )

            # 执行复杂的聚合查询
            sls_execute_spl(
                query=".entity_set with(domain='apm', name='apm.service') | entity-call get_metrics('system', 'cpu', 'usage') | stats avg(value) by entity_id"
            )
            ```

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                query: 原生SPL查询语句
                workspace: CMS工作空间名称
                from_time: 查询开始时间
                to_time: 查询结束时间
                regionId: 阿里云区域ID

            Returns:
                SPL查询执行结果的响应对象
            """
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
        def sls_list_logstores(
            ctx: Context,
            project: str = Field(
                ...,
                description="sls project name,must exact match,should not contain chinese characters",
            ),
            logStore: Optional[str] = Field(
                None, description="log store name,fuzzy search"
            ),
            limit: int = Field(10, description="limit,max is 100", ge=1, le=100),
            isMetricStore: bool = Field(
                False,
                description="is metric store,default is False,only use want to find metric store",
            ),
            regionId: str = Field(
                default=...,
                description="aliyun region id,region id format like 'xx-xxx',like 'cn-hangzhou'",
            ),
        ) -> Dict[str, Any]:
            """列出SLS项目中的日志库。

            ## 功能概述

            该工具可以列出指定SLS项目中的所有日志库，如果不选，则默认为日志库类型
            支持通过日志库名称进行模糊搜索。如果不提供日志库名称，则返回项目中的所有日志库。

            ## 使用场景

            - 当需要查找特定项目下是否存在某个日志库时
            - 当需要获取项目中所有可用的日志库列表时
            - 当需要根据日志库名称的部分内容查找相关日志库时
            - 如果从上下文未指定 project参数，除非用户说了遍历，则可使用 iaas_sls_list_projects 工具获取项目列表

            ## 是否指标库

            如果需要查找指标或者时序相关的库，请将isMetricStore参数设置为True

            ## 查询示例

            - "我想查询有没有 XXX 的日志库"
            - "某个 project 有哪些 log store"

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                project: SLS项目名称，必须精确匹配
                logStore: 日志库名称，支持模糊搜索
                limit: 返回结果的最大数量，范围1-100，默认10
                isMetricStore: 是否指标库，可选值为True或False，默认为False
                regionId: 阿里云区域ID

            Returns:
                日志库名称的字符串列表
            """
            if project == "":
                return {
                    "total": 0,
                    "logstores": [],
                    "message": "Please specify the project name,if you want to list all projects,please use iaas_sls_list_projects tool",
                }

            sls_client: SLSClient = ctx.request_context.lifespan_context[
                "sls_client"
            ].with_region(regionId)

            logStoreType = "Metrics" if isMetricStore else None
            request: ListLogStoresRequest = ListLogStoresRequest(
                logstore_name=logStore,
                size=limit,
                telemetry_type=logStoreType,
            )
            response: ListLogStoresResponse = sls_client.list_log_stores(
                project, request
            )

            log_store_count = response.body.total
            log_store_list = response.body.logstores

            return {
                "total": log_store_count,
                "logstores": log_store_list,
                "message": (
                    "Sorry not found logstore,please make sure your project and region or logstore name is correct, if you want to find metric store,please check isMetricStore parameter"
                    if log_store_count == 0
                    else f"当前最多支持查询{limit}个日志库，未防止返回数据过长，如果需要查询更多日志库，您可以提供 logstore 的关键词来模糊查询"
                ),
            }

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(Config.get_retry_attempts()),
            wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def sls_log_explore(
            ctx: Context,
            project: str = Field(
                ...,
                description="sls project name, must exact match, should not contain chinese characters",
            ),
            logStore: str = Field(
                ..., 
                description="sls log store name, must exact match, should not contain chinese characters"
            ),
            from_time: Union[str, int] = Field(
                "now-1h", description="开始时间: Unix时间戳(秒/毫秒)或相对时间(now-5m)"
            ),
            to_time: Union[str, int] = Field(
                "now", description="结束时间: Unix时间戳(秒/毫秒)或相对时间(now)"
            ),
            regionId: str = Field(
                ..., 
                description="Region ID"
            ),
            logField: str = Field(
                ...,
                description="name of field containing log message"
            ),
            filter_query: Optional[str] = Field(
                None,
                description="filter query must be a valid sls query statement, which is used to filter log data"
            ),
            groupField: Optional[str] = Field(
                None,
                description="name of field containing group identity of log messages"
            )
        ) -> Dict[str, Any]:
            """查看阿里云日志服务中某个日志库的日志数据聚合分析结果，提供日志数据概览信息。

            ## 功能概述

            该工具可以给出指定日志库中日志数据的概览信息。给出日志数据中典型的日志模板，以及各个日志模板对应日志数量分布。

            ## 使用场景

            - 当需要查看日志库中日志的概览信息和数据分布时

            ## 查询实例

            - "查询某个 project 的某个 logstore 中的日志数据分布"
            - "某个 project 的某个 logstore 中不同风险等级的日志有哪些"

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                project: SLS项目名称，必须精确匹配
                logStore: SLS日志库名称，必须精确匹配
                from_time: 查询开始时间
                to_time: 查询结束时间
                regionId: 阿里云区域ID
                logField: 日志字段名称
                filter_query: 过滤查询语句
                groupField: 分组字段名称

            Returns:
                日志数据概览信息
            """
            
            sls_client: SLSClient = ctx.request_context.lifespan_context[
                "sls_client"
            ].with_region(regionId)

            # Parse time parameters
            from mcp_server_aliyun_observability.toolkits.paas.time_utils import (
                TimeRangeParser,
            )

            from_timestamp = TimeRangeParser.parse_time_expression(from_time)
            to_timestamp = TimeRangeParser.parse_time_expression(to_time)
            
            # 1) get total number of log records in the specified time range
            request: GetHistogramsRequest = GetHistogramsRequest(
                from_=from_timestamp,
                to=to_timestamp,
                query=None if not filter_query else filter_query
            )
            response: GetHistogramsResponse = sls_client.get_histograms(
                project=project, 
                logstore=logStore, 
                request=request
            )
            histograms = response.body
            total_count = sum([histogram.count for histogram in histograms])
            if total_count == 0:
                return {
                    "patterns": [],
                    "message": f"Failed to do log explore because no log data found in the specified time range ({from_time} ~ {to_time})"
                }
            sampling_rate = int(50000 / total_count * 100)
            sampling_rate = min(max(sampling_rate, 1), 100)

            # 2) create model to generate log patterns
            filter_query = "*" if not filter_query else filter_query
            if not groupField:
                spl_query = f"""
{filter_query} and "{logField}": *
    | sample -method='bernoulli' {sampling_rate}
    | where "{logField}" != '' and "{logField}" is not null
    | stats content_arr = array_agg("{logField}")
    | extend ret = get_log_patterns(content_arr, ARRAY[',', ' ', '''', '"', ';', '!', '=', '(', ')', '[', ']', '{{', '}}', '?', ':', '', '\t', '\n'], cast(null as array(varchar)), cast(null as array(varchar)), '{{"threshold": 3, "tolerance": 0.1, "maxDigitRatio": 0.1}}')
    | extend model_id = ret.model_id, error_msg = ret.error_msg
    | project model_id, error_msg
    | limit 50000
"""
            else:
                spl_query = f"""
{filter_query} and "{logField}": *
    | sample -method='bernoulli' {sampling_rate}
    | where "{logField}" != '' and "{logField}" is not null
    | extend label_concat = coalesce(cast("{groupField}" as varchar), '')
    | stats content_arr = array_agg("{logField}"), label_arr = array_agg(label_concat)
    | extend ret = get_log_patterns(content_arr, ARRAY[',', ' ', '''', '"', ';', '!', '=', '(', ')', '[', ']', '{{', '}}', '?', ':', '', '\t', '\n'], cast(null as array(varchar)), label_arr, '{{"threshold": 3, "tolerance": 0.1, "maxDigitRatio": 0.1}}')
    | extend model_id = ret.model_id, error_msg = ret.error_msg
    | project model_id, error_msg
    | limit 50000
"""
            runtime: util_models.RuntimeOptions = util_models.RuntimeOptions()
            runtime.read_timeout = 60000
            runtime.connect_timeout = 60000

            request: GetLogsRequest = GetLogsRequest(
                query=spl_query,
                from_=from_timestamp,
                to=to_timestamp
            )
            response: GetLogsResponse = sls_client.get_logs_with_options(
                project=project,
                logstore=logStore,
                request=request,
                headers={},
                runtime=runtime
            )
            if not response.body:
                return {
                    "patterns": [],
                    "message": "Failed to do log explore because pattern model creation failed"
                }
            model_id = response.body[0].get("model_id")
            error_msg = response.body[0].get("error_msg")
            if not model_id:
                return {
                    "patterns": [],
                    "message": f"Failed to do log explore because pattern model creation failed: {error_msg}"
                }
            
            # 3) use model to match log data
            sampling_rate = int(200000 / total_count * 100)
            sampling_rate = min(max(sampling_rate, 1), 100)

            time_bucket_size = max((to_timestamp - from_timestamp) // 10, 1)

            if not groupField:
                spl_query = f"""
{filter_query} and "{logField}": *
| sample -method='bernoulli' {sampling_rate}
| where "{logField}" != '' and "{logField}" is not null
| extend ret = match_log_patterns('{model_id}', "{logField}")
| extend is_matched = ret.is_matched, pattern_id = ret.pattern_id, pattern = ret.pattern, pattern_regexp = ret.regexp, variables = ret.variables, time_bucket_id = __time__ - __time__ % {time_bucket_size}
| stats pattern = arbitrary(pattern), pattern_regexp = arbitrary(pattern_regexp), variables_arr = array_agg(variables), earliest_ts = min(__time__), latest_ts = max(__time__), event_num = count(1), hist = histogram(time_bucket_id), data_sample = arbitrary("{logField}") by pattern_id
| extend var_summary = summary_log_variables(variables_arr, '{{"topk": 10}}')
| project pattern_id, pattern, pattern_regexp, var_summary, earliest_ts, latest_ts, event_num, hist, data_sample
| sort event_num desc
| limit 200000
"""
            else:
                spl_query = f"""
{filter_query} and "{logField}": *
| sample -method='bernoulli' {sampling_rate}
| where "{logField}" != '' and "{logField}" is not null
| extend label_concat = coalesce(cast("{groupField}" as varchar), '')
| extend ret = match_log_patterns('{model_id}', "{logField}", label_concat)
| extend is_matched = ret.is_matched, pattern_id = ret.pattern_id, pattern = ret.pattern, pattern_regexp = ret.regexp, variables = ret.variables, time_bucket_id = __time__ - __time__ % {time_bucket_size}
| stats pattern = arbitrary(pattern), pattern_regexp = arbitrary(pattern_regexp), variables_arr = array_agg(variables), earliest_ts = min(__time__), latest_ts = max(__time__), event_num = count(1), hist = histogram(time_bucket_id), data_sample = arbitrary("{logField}") by pattern_id, label_concat
| extend var_summary = summary_log_variables(variables_arr, '{{"topk": 10}}')
| project pattern_id, pattern, label_concat, pattern_regexp, var_summary, earliest_ts, latest_ts, event_num, hist, data_sample
| sort event_num desc
| limit 200000
"""
            request: GetLogsRequest = GetLogsRequest(
                query=spl_query,
                from_=from_timestamp,
                to=to_timestamp
            )
            response: GetLogsResponse = sls_client.get_logs_with_options(
                project=project,
                logstore=logStore,
                request=request,
                headers={},
                runtime=runtime
            )
            if not response.body:
                return {
                    "patterns": [],
                    "message": "Failed to do log explore because match log data failed"
                }

            # 4) format result
            def format_result(item: Dict[str, Any]) -> Dict[str, Any]:
                # basic info
                formatted_item = {
                    "pattern": item.get("pattern"),
                    "pattern_regexp": item.get("pattern_regexp"),
                    "event_num": item.get("event_num"),
                    "group": item.get("label_concat")
                }
                if not formatted_item["pattern"]:
                    formatted_item["pattern"] = "<unknown-pattern>"
                    formatted_item["pattern_regexp"] = "<unknown-pattern-regexp>"
                # histogram info
                hist = json.loads(item.get("hist"))
                pattern_hist: List[Dict] = []
                for ts, count in hist.items():
                    pattern_hist.append({
                        "from_timestamp": int(ts),
                        "to_timestamp": int(ts) + time_bucket_size,
                        "count": count
                    })
                pattern_hist.sort(key=lambda x: x["from_timestamp"])
                formatted_item["histogram"] = pattern_hist
                # variables info
                var_summary = json.loads(item.get("var_summary"))
                var_candidates = var_summary[0]
                var_candidates_count = var_summary[1]
                var_candidates_type = var_summary[2]
                var_candidates_format = var_summary[3]
                variables: List[Dict] = []
                for i, (var_type, var_format) in enumerate(zip(var_candidates_type, var_candidates_format)):
                    var_info = {
                        "index": i,
                        "type": var_type,
                        "format": var_format,
                        "candidates": {}
                    }
                    for candidate, count in zip(var_candidates[i], var_candidates_count[i]):
                        var_info["candidates"][candidate] = count
                    variables.append(var_info)
                formatted_item["variables"] = variables
                return formatted_item
            
            results = [format_result(item) for item in response.body]
            return {
                "patterns": results,
                "message": "success"
            }

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(Config.get_retry_attempts()),
            wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def sls_log_compare(
            ctx: Context,
            project: str = Field(
                ...,
                description="sls project name, must exact match, should not contain chinese characters",
            ),
            logStore: str = Field(
                ..., 
                description="sls log store name, must exact match, should not contain chinese characters"
            ),
            test_from_time: Union[str, int] = Field(
                "now-1h", description="实验组数据的开始时间: Unix时间戳(秒/毫秒)或相对时间(now-5m)"
            ),
            test_to_time: Union[str, int] = Field(
                "now", description="实验组数据的结束时间: Unix时间戳(秒/毫秒)或相对时间(now)"
            ),
            control_from_time: Union[str, int] = Field(
                "now-1h", description="对照组数据的开始时间: Unix时间戳(秒/毫秒)或相对时间(now-5m)"
            ),
            control_to_time: Union[str, int] = Field(
                "now", description="对照组数据的结束时间: Unix时间戳(秒/毫秒)或相对时间(now)"
            ),
            regionId: str = Field(
                ..., 
                description="Region ID"
            ),
            logField: str = Field(
                ...,
                description="name of field containing log message"
            ),
            filter_query: Optional[str] = Field(
                None,
                description="filter query must be a valid sls query statement, which is used to filter log data"
            ),
            groupField: Optional[str] = Field(
                None,
                description="name of field containing group identity of log messages"
            )
        ) -> Dict[str, Any]:
            """查看阿里云日志服务中某个日志库的日志数据在两个时间范围内的对比结果。

            ## 功能概述

            该工具可以两个时间范围内的日志数据分布的区别，用于快速分析日志数据的变化情况。

            ## 使用场景

            - 当需要分析两个时间范围内的日志数据分布的区别时

            ## 查询实例

            - "昨天的日志和今天的相比有什么区别"
            - "服务发布前后日志有什么变化"

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                project: SLS项目名称，必须精确匹配
                logStore: SLS日志库名称，必须精确匹配
                test_from_time: 实验组数据的开始时间
                test_to_time: 实验组数据的结束时间
                control_from_time: 对照组数据的开始时间
                control_to_time: 对照组数据的结束时间
                regionId: 阿里云区域ID
                logField: 日志字段名称
                filter_query: 过滤查询语句
                groupField: 分组字段名称

            Returns:
                日志数据对比结果
            """
            sls_client: SLSClient = ctx.request_context.lifespan_context[
                "sls_client"
            ].with_region(regionId)

            # Parse time parameters
            from mcp_server_aliyun_observability.toolkits.paas.time_utils import (
                TimeRangeParser,
            )

            test_from_timestamp = TimeRangeParser.parse_time_expression(test_from_time)
            test_to_timestamp = TimeRangeParser.parse_time_expression(test_to_time)

            control_from_timestamp = TimeRangeParser.parse_time_expression(control_from_time)
            control_to_timestamp = TimeRangeParser.parse_time_expression(control_to_time)

            if not (test_to_timestamp < control_from_timestamp or control_to_timestamp < test_from_timestamp):
                return {
                    "patterns": [],
                    "message": f"Failed to do log compare because test group ({test_from_time} ~ {test_to_time}) and control group ({control_from_time} ~ {control_to_time}) data time range overlap"
                }

            # 1) get total number of log records in the specified time range
            request: GetHistogramsRequest = GetHistogramsRequest(
                from_=test_from_timestamp,
                to=test_to_timestamp,
                query=None if not filter_query else filter_query
            )
            response: GetHistogramsResponse = sls_client.get_histograms(
                project=project, 
                logstore=logStore, 
                request=request
            )
            histograms = response.body
            total_count = sum([histogram.count for histogram in histograms])
            if total_count == 0:
                return {
                    "patterns": [],
                    "message": f"Failed to do log compare because no log data found in the specified time range ({test_from_time} ~ {test_to_time})"
                }
            sampling_rate = int(50000 / total_count * 100)
            sampling_rate = min(max(sampling_rate, 1), 100)

            # 2) create model to generate log patterns
            filter_query = "*" if not filter_query else filter_query
            if not groupField:
                spl_query = f"""
{filter_query} and "{logField}": *
    | sample -method='bernoulli' {sampling_rate}
    | where "{logField}" != '' and "{logField}" is not null
    | stats content_arr = array_agg("{logField}")
    | extend ret = get_log_patterns(content_arr, ARRAY[',', ' ', '''', '"', ';', '!', '=', '(', ')', '[', ']', '{{', '}}', '?', ':', '', '\t', '\n'], cast(null as array(varchar)), cast(null as array(varchar)), '{{"threshold": 3, "tolerance": 0.1, "maxDigitRatio": 0.1}}')
    | extend model_id = ret.model_id, error_msg = ret.error_msg
    | project model_id, error_msg
    | limit 50000
"""
            else:
                spl_query = f"""
{filter_query} and "{logField}": *
    | sample -method='bernoulli' {sampling_rate}
    | where "{logField}" != '' and "{logField}" is not null
    | extend label_concat = coalesce(cast("{groupField}" as varchar), '')
    | stats content_arr = array_agg("{logField}"), label_arr = array_agg(label_concat)
    | extend ret = get_log_patterns(content_arr, ARRAY[',', ' ', '''', '"', ';', '!', '=', '(', ')', '[', ']', '{{', '}}', '?', ':', '', '\t', '\n'], cast(null as array(varchar)), label_arr, '{{"threshold": 3, "tolerance": 0.1, "maxDigitRatio": 0.1}}')
    | extend model_id = ret.model_id, error_msg = ret.error_msg
    | project model_id, error_msg
    | limit 50000
"""
            runtime: util_models.RuntimeOptions = util_models.RuntimeOptions()
            runtime.read_timeout = 60000
            runtime.connect_timeout = 60000

            request: GetLogsRequest = GetLogsRequest(
                query=spl_query,
                from_=test_from_timestamp,
                to=test_to_timestamp
            )
            response: GetLogsResponse = sls_client.get_logs_with_options(
                project=project,
                logstore=logStore,
                request=request,
                headers={},
                runtime=runtime
            )
            if not response.body:
                return {
                    "patterns": [],
                    "message": "Failed to do log compare because test group pattern model creation failed"
                }
            test_model_id = response.body[0].get("model_id")
            error_msg = response.body[0].get("error_msg")
            if not test_model_id:
                return {
                    "patterns": [],
                    "message": f"Failed to do log compare because test group pattern model creation failed: {error_msg}"
                }

            request: GetLogsRequest = GetLogsRequest(
                query=spl_query,
                from_=control_from_timestamp,
                to=control_to_timestamp
            )
            response: GetLogsResponse = sls_client.get_logs_with_options(
                project=project,
                logstore=logStore,
                request=request,
                headers={},
                runtime=runtime
            )
            if not response.body:
                return {
                    "patterns": [],
                    "message": "Failed to do log compare because control group pattern model creation failed"
                }
            control_model_id = response.body[0].get("model_id")
            error_msg = response.body[0].get("error_msg")
            if not control_model_id:
                return {
                    "patterns": [],
                    "message": f"Failed to do log compare because control group pattern model creation failed: {error_msg}"
                }

            # 3) merge test and control group model
            spl_query = f"""
{filter_query} and "{logField}": *
| where "{logField}" != '' and "{logField}" is not null
| extend ret = merge_log_patterns('{test_model_id}', '{control_model_id}')
| limit 1
| extend model_id = ret.model_id, error_msg = ret.error_msg
| project model_id, error_msg
"""
            request: GetLogsRequest = GetLogsRequest(
                query=spl_query,
                from_=test_from_timestamp,
                to=test_to_timestamp
            )
            response: GetLogsResponse = sls_client.get_logs_with_options(
                project=project,
                logstore=logStore,
                request=request,
                headers={},
                runtime=runtime
            )
            if not response.body:
                return {
                    "patterns": [],
                    "message": "Failed to do log compare because merge test and control group model failed"
                }
            model_id = response.body[0].get("model_id")
            error_msg = response.body[0].get("error_msg")
            if not model_id:
                return {
                    "patterns": [],
                    "message": f"Failed to do log compare because merge test and control group model failed: {error_msg}"
                }

            # 4) use model to match log data
            sampling_rate = int(200000 / total_count * 100)
            sampling_rate = min(max(sampling_rate, 1), 100)

            from_timestamp = min(test_from_timestamp, control_from_timestamp)
            to_timestamp = max(test_to_timestamp, control_to_timestamp)
            if not groupField:
                spl_query = f"""
{filter_query} and "{logField}": * 
| extend group_id = if(__time__ >= {test_from_timestamp} and __time__ < {test_to_timestamp}, 'test', if(__time__ >= {control_from_timestamp} and __time__ < {control_to_timestamp}, 'control', 'null'))
| where group_id != 'null'
| sample -method='bernoulli' {sampling_rate}
| where "{logField}" != '' and "{logField}" is not null
| extend ret = match_log_patterns('{model_id}', "{logField}")
| extend is_matched = ret.is_matched, pattern_id = ret.pattern_id, pattern = ret.pattern, pattern_regexp = ret.regexp, variables = ret.variables
| stats pattern = arbitrary(pattern), pattern_regexp = arbitrary(pattern_regexp), variables_arr = array_agg(variables), earliest_ts = min(__time__), latest_ts = max(__time__), event_num = count(1), data_sample = arbitrary("{logField}") by pattern_id, group_id
| extend var_summary = summary_log_variables(variables_arr, '{{"topk": 10}}')
| project pattern_id, pattern, pattern_regexp, var_summary, earliest_ts, latest_ts, event_num, data_sample, group_id
| sort event_num desc
| limit 200000
"""
            else:
                spl_query = f"""
{filter_query} and "{logField}": * and "{groupField}": *
| extend group_id = if(__time__ >= {test_from_timestamp} and __time__ < {test_to_timestamp}, 'test', if(__time__ >= {control_from_timestamp} and __time__ < {control_to_timestamp}, 'control', 'null'))
| where group_id != 'null'
| sample -method='bernoulli' {sampling_rate}
| where "{logField}" != '' and "{logField}" is not null
| extend label_concat = coalesce(cast("{groupField}" as varchar), '')
| extend ret = match_log_patterns('{model_id}', "{logField}", label_concat)
| extend is_matched = ret.is_matched, pattern_id = ret.pattern_id, pattern = ret.pattern, pattern_regexp = ret.regexp, variables = ret.variables
| stats pattern = arbitrary(pattern), pattern_regexp = arbitrary(pattern_regexp), variables_arr = array_agg(variables), earliest_ts = min(__time__), latest_ts = max(__time__), event_num = count(1), data_sample = arbitrary("{logField}") by pattern_id, label_concat, group_id
| extend var_summary = summary_log_variables(variables_arr, '{{"topk": 10}}')
| project pattern_id, pattern, label_concat, pattern_regexp, var_summary, earliest_ts, latest_ts, event_num, data_sample, group_id
| sort event_num desc
| limit 200000
"""
            request: GetLogsRequest = GetLogsRequest(
                query=spl_query,
                from_=from_timestamp,
                to=to_timestamp
            )
            response: GetLogsResponse = sls_client.get_logs_with_options(
                project=project,
                logstore=logStore,
                request=request,
                headers={},
                runtime=runtime
            )
            if not response.body:
                return {
                    "patterns": [],
                    "message": "Failed to do log compare because match log data failed"
                }

            # 5) format result
            def format_result(item: Dict[str, Any]) -> Dict[str, Any]:
                # basic info
                formatted_item = {
                    "pattern": item.get("pattern"),
                    "pattern_regexp": item.get("pattern_regexp"),
                    "event_num": item.get("event_num"),
                    "group": item.get("label_concat"),
                    "test_or_control": item.get("group_id")
                }
                # variables info
                var_summary = json.loads(item.get("var_summary"))
                var_candidates = var_summary[0]
                var_candidates_count = var_summary[1]
                var_candidates_type = var_summary[2]
                var_candidates_format = var_summary[3]
                variables: List[Dict] = []
                for i, (var_type, var_format) in enumerate(zip(var_candidates_type, var_candidates_format)):
                    var_info = {
                        "index": i,
                        "type": var_type,
                        "format": var_format,
                        "candidates": {}
                    }
                    for candidate, count in zip(var_candidates[i], var_candidates_count[i]):
                        var_info["candidates"][candidate] = count
                    variables.append(var_info)
                formatted_item["variables"] = variables
                return formatted_item
            
            pairs: Dict[Tuple[str, str], Dict] = {}
            for item in response.body:
                formatted_item = format_result(item)
                key = (formatted_item["group"], formatted_item["pattern"])
                if key not in pairs:
                    pairs[key] = {}
                pairs[key][formatted_item["test_or_control"]] = formatted_item
            
            def merge_items(test: Optional[Dict] = None, control: Optional[Dict] = None) -> Optional[Dict]:
                if not test and not control:
                    return None

                return {
                    "pattern": test["pattern"] if not control else control["pattern"],
                    "pattern_regexp": test["pattern_regexp"] if not control else control["pattern_regexp"],
                    "test_event_num": 0 if not test else test["event_num"],
                    "control_event_num": 0 if not control else control["event_num"],
                    "group": test["group"] if not control else control["group"],
                    "test_variables": [] if not test else test["variables"],
                    "control_variables": [] if not control else control["variables"]
                }
            
            results: List[Dict] = []
            for pair in pairs.values():
                merged_item = merge_items(pair.get("test"), pair.get("control"))
                if merged_item:
                    results.append(merged_item)
            return {
                "patterns": results,
                "message": "success"
            }

def register_iaas_tools(server: FastMCP):
    """Register IaaS toolkit tools with the FastMCP server

    Args:
        server: FastMCP server instance
    """
    IaaSToolkit(server)
