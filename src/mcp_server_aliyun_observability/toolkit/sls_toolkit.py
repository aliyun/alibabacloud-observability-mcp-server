from typing import Any, Dict, List

from alibabacloud_sls20201230.client import Client
from alibabacloud_sls20201230.models import (
    CallAiToolsRequest,
    CallAiToolsResponse,
    GetIndexResponse,
    GetIndexResponseBody,
    GetLogsRequest,
    GetLogsResponse,
    IndexJsonKey,
    IndexKey,
    ListLogStoresRequest,
    ListLogStoresResponse,
    ListProjectRequest,
    ListProjectResponse,
)
from alibabacloud_tea_util import models as util_models
from mcp.server.fastmcp import Context, FastMCP
from mcp.server.fastmcp.prompts import base
from pydantic import Field
from tenacity import retry, retry_if_exception_type, stop_after_attempt, wait_fixed

from mcp_server_aliyun_observability.logger import log_error
from mcp_server_aliyun_observability.utils import (
    append_current_time,
    get_current_time,
    handle_tea_exception,
    parse_json_keys,
    text_to_sql,
)


class SLSToolkit:
    """aliyun observability tools manager"""

    def __init__(self, server: FastMCP):
        """
        initialize the tools manager

        Args:
            server: FastMCP server instance
        """
        self.server = server
        self._register_sls_tools()
        self._register_prompts()

    def _register_prompts(self):
        """register sls related prompts functions"""

        @self.server.prompt(
            name="sls 日志查询 prompt",
            description="当用户需要查询 sls 日志时，可以调用该 prompt来获取过程",
        )
        def query_sls_logs(question: str) -> str:
            """当用户需要查询 sls 日志时，可以调用该 prompt来获取过程"""
            return [
                base.UserMessage("基于以下问题查询下对应的 sls日志:"),
                base.UserMessage(f"问题: {question}"),
                base.UserMessage("过程如下:"),
                base.UserMessage(
                    content="1.首先尝试从上下文提取有效的 project 和 logstore 信息,如果上下文没有提供，请使用 sls_list_projects 和 sls_list_logstores 工具获取"
                ),
                base.UserMessage(
                    content="2.如果问题里面已经明确包含了查询语句，则直接使用，如果问题里面没有明确包含查询语句，则需要使用 sls_translate_natural_language_to_log_query 工具生成查询语句"
                ),
                base.UserMessage(
                    "3. 最后使用 sls_execute_query 工具执行查询语句，获取查询结果"
                ),
                base.UserMessage("3. 返回查询到的日志"),
            ]

    def _register_sls_tools(self):
        """register sls related tools functions"""

        @self.server.tool()
        def sls_list_projects(
            ctx: Context,
            projectName: str = Field(None, description="project name,fuzzy search"),
            limit: int = Field(
                default=50, description="limit,max is 100", ge=1, le=100
            ),
            regionId: str = Field(default=..., description="aliyun region id"),
        ):
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
                limit: 返回结果的最大数量，范围1-100，默认10
                regionId: 阿里云区域ID,region id format like "xx-xxx",like "cn-hangzhou"

            Returns:
                包含项目信息的字典列表，每个字典包含project_name、description和region_id
            """
            sls_client: Client = ctx.request_context.lifespan_context[
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
            stop=stop_after_attempt(2),
            wait=wait_fixed(1),
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
            logStore: str = Field(None, description="log store name,fuzzy search"),
            limit: int = Field(10, description="limit,max is 100", ge=1, le=100),
            isMetricStore: bool = Field(
                False,
                description="is metric store,default is False,only use want to find metric store",
            ),
            logStoreType: str = Field(
                None,
                description="log store type,default is logs,should be logs,metrics",
            ),
            regionId: str = Field(
                default=...,
                description="aliyun region id,region id format like 'xx-xxx',like 'cn-hangzhou'",
            ),
        ) -> Any:
            """列出SLS项目中的日志库。

            ## 功能概述

            该工具可以列出指定SLS项目中的所有日志库，如果不选，则默认为日志库类型
            支持通过日志库名称进行模糊搜索。如果不提供日志库名称，则返回项目中的所有日志库。

            ## 使用场景

            - 当需要查找特定项目下是否存在某个日志库时
            - 当需要获取项目中所有可用的日志库列表时
            - 当需要根据日志库名称的部分内容查找相关日志库时
            - 如果从上下文未指定 project参数，除非用户说了遍历，则可使用 sls_list_projects 工具获取项目列表

            ## 是否指标库

            如果需要查找指标或者时序相关的库,请将is_metric_store参数设置为True

            ## 查询示例

            - "我想查询有没有 XXX 的日志库"
            - "某个 project 有哪些 log store"

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                project: SLS项目名称，必须精确匹配
                log_store: 日志库名称，支持模糊搜索
                limit: 返回结果的最大数量，范围1-100，默认10
                is_metric_store: 是否指标库，可选值为True或False，默认为False
                region_id: 阿里云区域ID

            Returns:
                日志库名称的字符串列表
            """
            if isMetricStore:
                logStoreType = "Metrics"

            if project == "":
                return {
                    "total": 0,
                    "logstores": [],
                    "messager": "Please specify the project name,if you want to list all projects,please use sls_list_projects tool",
                }
            sls_client: Client = ctx.request_context.lifespan_context[
                "sls_client"
            ].with_region(regionId)
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
                    "Sorry not found logstore,please make sure your project and region or logstore name is correct, if you want to find metric store,please check is_metric_store parameter"
                    if log_store_count == 0
                    else f"当前最多支持查询{limit}个日志库，未防止返回数据过长，如果需要查询更多日志库，您可以提供 logstore 的关键词来模糊查询"
                ),
            }

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(2),
            wait=wait_fixed(1),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def sls_describe_logstore(
            ctx: Context,
            project: str = Field(
                ..., description="sls project name,must exact match,not fuzzy search"
            ),
            logStore: str = Field(
                ..., description="sls log store name,must exact match,not fuzzy search"
            ),
            regionId: str = Field(
                default=...,
                description="aliyun region id,region id format like 'xx-xxx',like 'cn-hangzhou'",
            ),
        ) -> Any:
            """获取SLS日志库的结构信息。

            ## 功能概述

            该工具用于获取指定SLS项目中日志库的索引信息和结构定义，包括字段类型、别名、是否大小写敏感等信息。

            ## 使用场景

            - 当需要了解日志库的字段结构时
            - 当需要获取日志库的索引配置信息时
            - 当构建查询语句前需要了解可用字段时
            - 当需要分析日志数据结构时

            ## 返回数据结构

            返回一个字典，键为字段名，值包含以下信息：
            - alias: 字段别名
            - sensitive: 是否大小写敏感
            - type: 字段类型
            - json_keys: JSON字段的子字段信息

            ## 查询示例

            - "我想查询 XXX 的日志库的 schema"
            - "我想查询 XXX 的日志库的 index"
            - "我想查询 XXX 的日志库的结构信息"

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                project: SLS项目名称，必须精确匹配
                log_store: SLS日志库名称，必须精确匹配
                region_id: 阿里云区域ID

            Returns:
                包含日志库结构信息的字典
            """
            sls_client: Client = ctx.request_context.lifespan_context[
                "sls_client"
            ].with_region(regionId)
            response: GetIndexResponse = sls_client.get_index(project, logStore)
            response_body: GetIndexResponseBody = response.body
            keys: dict[str, IndexKey] = response_body.keys
            index_dict: dict[str, dict[str, str]] = {}
            for key, value in keys.items():
                index_dict[key] = {
                    "alias": value.alias,
                    "sensitive": value.case_sensitive,
                    "type": value.type,
                    "json_keys": parse_json_keys(value.json_keys),
                }
            return index_dict

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(2),
            wait=wait_fixed(1),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def sls_execute_sql_query(
            ctx: Context,
            project: str = Field(..., description="sls project name"),
            logStore: str = Field(..., description="sls log store name"),
            query: str = Field(..., description="query"),
            fromTimestampInSeconds: int = Field(
                ...,
                description="from timestamp,unit is second,should be unix timestamp, only number,no other characters",
            ),
            toTimestampInSeconds: int = Field(
                ...,
                description="to timestamp,unit is second,should be unix timestamp, only number,no other characters",
            ),
            limit: int = Field(10, description="limit,max is 100", ge=1, le=100),
            regionId: str = Field(
                default=...,
                description="aliyun region id,region id format like 'xx-xxx',like 'cn-hangzhou'",
            ),
        ) -> Any:
            """执行SLS日志查询。

            ## 功能概述

            该工具用于在指定的SLS项目和日志库上执行查询语句，并返回查询结果。查询将在指定的时间范围内执行。 如果上下文没有提到具体的 SQL 语句，必须优先使用 sls_translate_text_to_sql_query 工具生成查询语句,无论问题有多简单

            ## 使用场景

            - 当需要根据特定条件查询日志数据时
            - 当需要分析特定时间范围内的日志信息时
            - 当需要检索日志中的特定事件或错误时
            - 当需要统计日志数据的聚合信息时


            ## 查询语法

            查询必须使用SLS有效的查询语法，而非自然语言。如果不了解日志库的结构，可以先使用sls_describe_logstore工具获取索引信息。

            ## 时间范围

            查询必须指定时间范围：  if the query is generated by sls_translate_text_to_sql_query tool, should use the fromTimestampInSeconds and toTimestampInSeconds in the sls_translate_text_to_sql_query response
            - fromTimestampInSeconds: 开始时间戳（秒）
            - toTimestampInSeconds: 结束时间戳（秒）

            ## 查询示例

            - "帮我查询下 XXX 的日志信息"
            - "查找最近一小时内的错误日志"

            ## 错误处理
            - Column xxx can not be resolved 如果是 sls_translate_text_to_sql_query 工具生成的查询语句 可能存在查询列未开启统计，可以提示用户增加相对应的信息，或者调用 sls_describe_logstore 工具获取索引信息之后，要用户选择正确的字段或者提示用户对列开启统计。当确定列开启统计之后，可以再次调用sls_translate_text_to_sql_query 工具生成查询语句

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                project: SLS项目名称
                logStore: SLS日志库名称
                query: SLS查询语句
                fromTimestamp: 查询开始时间戳（秒）
                toTimestamp: 查询结束时间戳（秒）
                limit: 返回结果的最大数量，范围1-100，默认10
                regionId: 阿里云区域ID

            Returns:
                查询结果列表，每个元素为一条日志记录
            """
            sls_client: Client = ctx.request_context.lifespan_context[
                "sls_client"
            ].with_region(regionId)
            request: GetLogsRequest = GetLogsRequest(
                query=query,
                from_=fromTimestampInSeconds,
                to=toTimestampInSeconds,
                line=limit,
            )
            runtime: util_models.RuntimeOptions = util_models.RuntimeOptions()
            runtime.read_timeout = 60000
            runtime.connect_timeout = 60000
            response: GetLogsResponse = sls_client.get_logs_with_options(
                project, logStore, request, headers={}, runtime=runtime
            )
            response_body: List[Dict[str, Any]] = response.body
            result = {
                "data": response_body,
                "message": "success"
                if response_body
                else "Not found data by query,you can try to change the query or time range",
            }
            return result

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(2),
            wait=wait_fixed(1),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def sls_translate_text_to_sql_query(
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
        ) -> Any:
            """将自然语言转换为SLS查询语句。当用户有明确的 logstore 查询需求，必须优先使用该工具来生成查询语句

            ## 功能概述

            该工具可以将自然语言描述转换为有效的SLS查询语句，便于用户使用自然语言表达查询需求。用户有任何 SLS 日志查询需求时，都需要优先使用该工具。

            ## 使用场景

            - 当用户不熟悉SLS查询语法时
            - 当需要快速构建复杂查询时
            - 当需要从自然语言描述中提取查询意图时

            ## 使用限制

            - 仅支持生成SLS查询，不支持其他数据库的SQL如MySQL、PostgreSQL等
            - 生成的是查询语句，而非查询结果，需要配合sls_execute_query工具使用
            - 如果查询涉及ARMS应用，应优先使用arms_generate_trace_query工具
            - 需要对应的 log_sotre 已经设定了索引信息，如果生成的结果里面有字段没有索引或者开启统计，可能会导致查询失败，需要友好的提示用户增加相对应的索引信息

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
                log_store: SLS日志库名称
                region_id: 阿里云区域ID

            Returns:
                生成的SLS查询语句
            """

            return text_to_sql(ctx, text, project, logStore, regionId)

        @self.server.tool()
        def sls_diagnose_query(
            ctx: Context,
            query: str = Field(..., description="sls query"),
            errorMessage: str = Field(..., description="error message"),
            project: str = Field(..., description="sls project name"),
            logStore: str = Field(..., description="sls log store name"),
            regionId: str = Field(
                default=...,
                description="aliyun region id,region id format like 'xx-xxx',like 'cn-hangzhou'",
            ),
        ) -> Any:
            """诊断SLS查询语句。

            ## 功能概述

            当 SLS 查询语句执行失败时，可以调用该工具，根据错误信息，生成诊断结果。诊断结果会包含查询语句的正确性、性能分析、优化建议等信息。

            ## 使用场景

            - 当需要诊断SLS查询语句的正确性时
            - 当 SQL 执行错误需要查找原因时

            ## 查询示例

            - "帮我诊断下 XXX 的日志查询语句"
            - "帮我分析下 XXX 的日志查询语句"

            Args:
                ctx: MCP上下文，用于访问SLS客户端
                query: SLS查询语句
                error_message: 错误信息
                project: SLS项目名称
                log_store: SLS日志库名称
                region_id: 阿里云区域ID
            """
            try:
                sls_client_wrapper = ctx.request_context.lifespan_context["sls_client"]
                sls_client: Client = sls_client_wrapper.with_region("cn-shanghai")
                knowledge_config = sls_client_wrapper.get_knowledge_config(
                    project, logStore
                )
                request: CallAiToolsRequest = CallAiToolsRequest()
                request.tool_name = "diagnosis_sql"
                request.region_id = regionId
                params: dict[str, Any] = {
                    "project": project,
                    "logstore": logStore,
                    "sys.query": append_current_time(
                        f"帮我诊断下 {query} 的日志查询语句,错误信息为 {errorMessage}"
                    ),
                    "external_knowledge_uri": knowledge_config["uri"]
                    if knowledge_config
                    else "",
                    "external_knowledge_key": knowledge_config["key"]
                    if knowledge_config
                    else "",
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
                log_error(f"调用SLS AI工具失败: {str(e)}")
                raise
