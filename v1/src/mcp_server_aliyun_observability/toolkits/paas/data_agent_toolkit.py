"""Data Agent Pro Toolkit - 自然语言数据查询工具包

该工具包通过调用 CMS SDK 的对话接口，提供自然语言数据查询功能。
使用 data-agent skill 实现智能数据查询。

参考 text_to_sql 的实现方式，通过 CreateThread + CreateChatWithSSE API 实现。
"""

import json
import os
import time
from typing import Any, Dict

import yaml
from alibabacloud_cms20240330 import models as cms_model
from alibabacloud_cms20240330.client import Client as CmsClient
from alibabacloud_tea_util import models as util_models
from mcp.server.fastmcp import Context, FastMCP
from pydantic import Field
from tenacity import retry, retry_if_exception_type, stop_after_attempt, wait_fixed

from mcp_server_aliyun_observability.config import Config
from mcp_server_aliyun_observability.logger import get_logger
from mcp_server_aliyun_observability.utils import handle_tea_exception

logger = get_logger()


class DataAgentToolkit:
    """Data Agent Pro Toolkit - 自然语言数据查询工具包
    
    通过调用 CMS SDK 的对话接口，提供自然语言数据查询功能。
    支持对可观测数据进行自然语言查询，包括指标、日志、链路等数据类型。
    """

    def __init__(self, server: FastMCP):
        self.server = server
        self._register_tools()

    def _register_tools(self):
        """Register data agent tools"""

        @self.server.tool()
        @retry(
            stop=stop_after_attempt(Config.get_retry_attempts()),
            wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def cms_natural_language_query(
            ctx: Context,
            query: str = Field(
                ...,
                description="自然语言查询文本，描述你想要查询的可观测数据。示例：'查询请求量最高的10个服务'、'哪些服务的错误率超过1%'、'查询响应时间超过1秒的服务'"
            ),
            workspace: str = Field(
                ...,
                description="CMS workspace name, obtainable via list_workspace"
            ),
            regionId: str = Field(
                ...,
                description="Alibaba Cloud region ID, e.g. 'cn-hongkong'"
            ),
            time_range: str = Field(
                default="last_15m",
                description="Time range expression, e.g. 'last_15m', 'last_1h', 'last_1d'"
            ),
        ) -> Dict[str, Any]:
            """使用自然语言查询可观测数据。

            ## 功能概述
            
            用户可以使用自然语言描述想要查询的数据，系统会自动理解意图并返回相应的数据结果。

            ## 使用场景
            
            - 当需要快速查询可观测数据但不熟悉具体 API 时
            - 当需要进行复杂的数据分析但不想编写复杂查询语句时
            - 当需要获取服务性能概览、错误分析、慢请求统计等信息时

            ## 支持的查询类型
            
            - 指标查询：如"查询服务A的CPU使用率"、"获取内存使用最高的Pod"
            - 日志查询：如"统计错误日志数量"、"查找包含Exception的日志"
            - 链路查询：如"查询延迟超过1秒的请求"、"获取服务调用拓扑"
            - 聚合统计：如"按服务分组统计请求数量"、"计算平均响应时间"

            ## 返回数据结构
            
            返回的数据包含：
            - data: 查询结果数据，包含 query_results（实体列表、指标数据等）
            - message: AI 生成的解释说明文本
            - trace_id: 追踪ID，用于问题排查
            - error: 是否发生错误（true/false）
            - time_range: 查询的时间范围

            ## 查询示例
            
            - "查询请求量最高的10个服务"
            - "哪些服务的错误率超过1%"
            - "查询响应时间超过1秒的服务"
            - "统计各服务的请求数量"

            Args:
                ctx: MCP上下文，用于访问CMS客户端
                query: 自然语言查询文本，描述想要查询的数据

            Returns:
                包含查询结果的字典：
                - data: 查询结果数据
                - message: 解释说明
                - trace_id: 追踪ID
                - error: 是否发生错误
                - time_range: 查询时间范围
            """
            # Use tool parameters, fall back to env vars
            ws = workspace or os.getenv("ALIBABA_CLOUD_WORKSPACE", "")
            region = regionId or os.getenv("ALIBABA_CLOUD_REGION", "")
            
            if not ws:
                return {
                    "data": None,
                    "message": "workspace is required (pass as parameter or set ALIBABA_CLOUD_WORKSPACE)",
                    "request_id": "",
                    "error": True
                }
            if not region:
                return {
                    "data": None,
                    "message": "regionId is required (pass as parameter or set ALIBABA_CLOUD_REGION)",
                    "request_id": "",
                    "error": True
                }
            
            return _data_agent_query(
                ctx=ctx,
                query=query,
                workspace=ws,
                region_id=region,
            )


def _data_agent_query(
    ctx: Context,
    query: str,
    workspace: str,
    region_id: str,
) -> Dict[str, Any]:
    """调用 CMS SDK 的对话接口进行自然语言数据查询。

    使用 CMS SDK 的 CreateThread + CreateChatWithSSE API，
    通过 data-agent-pro skill 实现自然语言数据查询。
    实现逻辑与 Go 版本 DataAgentQuery 保持一致。
    """
    digital_employee_name = "apsara-ops"
    skill = "data-agent-pro"

    current_time = int(time.time())
    from_time = current_time - 900
    to_time = current_time

    language = os.getenv("LANGUAGE", "zh")
    time_zone = os.getenv("TIMEZONE", "Asia/Shanghai")

    logger.info(
        f"开始自然语言数据查询: query={query}, workspace={workspace}, "
        f"region_id={region_id}, from_time={from_time}, to_time={to_time}"
    )

    try:
        cms_client_wrapper = ctx.request_context.lifespan_context["cms_client"]
        cms_client: CmsClient = cms_client_wrapper.with_region(region_id)

        # Step 1: Create a thread
        thread_request = cms_model.CreateThreadRequest()
        thread_request.title = f"data-query-{int(time.time())}"
        thread_variables = cms_model.CreateThreadRequestVariables()
        thread_variables.workspace = workspace
        thread_request.variables = thread_variables

        thread_response = cms_client.create_thread(digital_employee_name, thread_request)
        if not thread_response.body or not thread_response.body.thread_id:
            raise Exception("Failed to create thread: missing thread_id")

        thread_id = thread_response.body.thread_id
        logger.info(f"Thread created, thread_id: {thread_id}")

        # Step 2: Create chat request with SSE
        chat_request = cms_model.CreateChatRequest()
        chat_request.digital_employee_name = digital_employee_name
        chat_request.action = "create"
        chat_request.thread_id = thread_id

        message = cms_model.CreateChatRequestMessages()
        message.role = "user"
        content = cms_model.CreateChatRequestMessagesContents()
        content.type = "text"
        content.value = query
        message.contents = [content]
        chat_request.messages = [message]

        user_context = json.dumps([{
            "type": "metadata",
            "data": {
                "from_time": from_time,
                "to_time": to_time,
                "fromTime": from_time,
                "toTime": to_time,
            }
        }])

        chat_request.variables = {
            "region": region_id,
            "workspace": workspace,
            "language": language,
            "timeZone": time_zone,
            "timeStamp": str(int(time.time())),
            "startTime": from_time,
            "endTime": to_time,
            "skill_name": skill,
            "userContext": user_context,
            "config": json.dumps({"disableThreadData": True}),
            "skill": skill,
        }

        # Step 3: Call SSE API and collect responses
        runtime = util_models.RuntimeOptions()
        runtime.read_timeout = 180000
        runtime.connect_timeout = 30000

        collected_text = []
        trace_id = None

        for response in cms_client.create_chat_with_sse(chat_request, {}, runtime):
            if response.body and hasattr(response.body, 'trace_id') and response.body.trace_id:
                trace_id = response.body.trace_id

            if response.body and response.body.messages:
                for msg in response.body.messages:
                    # Extract text from artifacts[].parts[kind=text].text (matching Go)
                    msg_artifacts = getattr(msg, 'artifacts', None)
                    if msg_artifacts:
                        for artifact in msg_artifacts:
                            if not isinstance(artifact, dict):
                                continue
                            parts = artifact.get("parts", [])
                            if not isinstance(parts, list):
                                continue
                            for part in parts:
                                if not isinstance(part, dict):
                                    continue
                                if part.get("kind") == "text" and "text" in part:
                                    collected_text.append(part["text"])

        explanation = "".join(collected_text)
        logger.info(f"Data agent query complete, trace_id: {trace_id}, text_length: {len(explanation)}")

        return {
            "message": explanation if explanation else "查询完成",
            "trace_id": trace_id or "",
            "error": False,
            "timestamp": int(time.time()),
        }

    except Exception as e:
        logger.error(f"调用数据查询失败: {str(e)}", exc_info=True)
        return {
            "data": None,
            "message": f"查询失败: {str(e)}",
            "trace_id": "",
            "error": True,
            "timestamp": int(time.time()),
        }

