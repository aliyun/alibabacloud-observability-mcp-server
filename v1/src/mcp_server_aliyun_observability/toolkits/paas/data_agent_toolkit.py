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
            # 从环境变量获取 workspace 和 region_id
            workspace = os.getenv("ALIBABA_CLOUD_WORKSPACE", "")
            region_id = os.getenv("ALIBABA_CLOUD_REGION", "")
            
            if not workspace:
                return {
                    "data": None,
                    "message": "环境变量 ALIBABA_CLOUD_WORKSPACE 未设置，请在启动服务时配置",
                    "request_id": "",
                    "error": True
                }
            if not region_id:
                return {
                    "data": None,
                    "message": "环境变量 ALIBABA_CLOUD_REGION 未设置，请在启动服务时配置",
                    "request_id": "",
                    "error": True
                }
            
            return _data_agent_query(
                ctx=ctx,
                query=query,
                workspace=workspace,
                region_id=region_id,
            )


def _data_agent_query(
    ctx: Context,
    query: str,
    workspace: str,
    region_id: str,
) -> Dict[str, Any]:
    """调用 CMS SDK 的对话接口进行自然语言数据查询
    
    该函数使用 CMS SDK 的 CreateThread + CreateChatWithSSE API，
    通过 data-agent skill 实现自然语言数据查询功能。
    
    Args:
        ctx: MCP上下文
        query: 自然语言查询文本
        workspace: CMS工作空间名称
        region_id: 阿里云区域ID

    Returns:
        Dict包含查询结果
    """
    digital_employee_name = "apsara-ops"
    skill = "data-agent"
    
    # 设置默认时间范围：最近15分钟
    current_time = int(time.time())
    from_time = current_time - 900
    to_time = current_time
    
    language = os.getenv("LANGUAGE", "zh")
    time_zone = os.getenv("TIMEZONE", "Asia/Shanghai")

    logger.info(
        f"开始自然语言数据查询，输入参数: query={query}, "
        f"workspace={workspace}, region_id={region_id}, "
        f"from_time={from_time}, to_time={to_time}"
    )

    try:
        # 获取 CMS 客户端
        cms_client_wrapper = ctx.request_context.lifespan_context["cms_client"]
        cms_client: CmsClient = cms_client_wrapper.with_region(region_id)
        
        # Step 1: 创建会话线程
        thread_request = cms_model.CreateThreadRequest()
        thread_request.title = f"data-query-{int(time.time())}"
        
        thread_variables = cms_model.CreateThreadRequestVariables()
        thread_variables.workspace = workspace
        thread_request.variables = thread_variables
        
        logger.info(f"创建会话线程，digital_employee: {digital_employee_name}")
        thread_response = cms_client.create_thread(digital_employee_name, thread_request)
        
        if not thread_response.body or not thread_response.body.thread_id:
            raise Exception("Failed to create thread: missing thread_id")
        
        thread_id = thread_response.body.thread_id
        logger.info(f"会话线程创建成功，thread_id: {thread_id}")
        
        # Step 2: 创建 Chat 请求
        chat_request = cms_model.CreateChatRequest()
        chat_request.digital_employee_name = digital_employee_name
        chat_request.action = "create"
        chat_request.thread_id = thread_id
        
        # 构建消息内容
        message = cms_model.CreateChatRequestMessages()
        message.role = "user"
        
        content = cms_model.CreateChatRequestMessagesContents()
        content.type = "text"
        content.value = query
        message.contents = [content]
        
        chat_request.messages = [message]
        
        # 构建变量
        user_context = json.dumps([{
            "type": "metadata",
            "data": {
                "from_time": from_time,
                "to_time": to_time,
                "fromTime": from_time,
                "toTime": to_time
            }
        }])
        
        # 构建 variables
        variables = {
            "region": region_id,
            "workspace": workspace,
            "language": language,
            "timeZone": time_zone,
            "timeStamp": str(int(time.time())),
            "startTime": from_time,
            "endTime": to_time,
            "skill_name": skill,
            "userContext": user_context,
            "config": json.dumps({"disableThreadData": False}),
            "skill": skill
        }
        
        chat_request.variables = variables
        
        logger.info(f"发送Chat请求，thread_id: {thread_id}")
        
        # Step 3: 调用 SSE API 并收集响应
        runtime = util_models.RuntimeOptions()
        runtime.read_timeout = 180000  # 3分钟超时
        runtime.connect_timeout = 30000
        
        collected_data = []
        collected_text = []
        collected_tool_results = []
        collected_sql = None
        trace_id = None  # 使用 trace_id 而不是 request_id
        
        # 使用 SSE 流式获取响应
        for response in cms_client.create_chat_with_sse(chat_request, {}, runtime):
            # 获取 trace_id（整个流式请求的唯一标识）
            if response.body:
                if hasattr(response.body, 'trace_id') and response.body.trace_id:
                    trace_id = response.body.trace_id
            
            if response.body and response.body.messages:
                for msg in response.body.messages:
                    # 获取 msg 的所有关键字段
                    msg_tools = getattr(msg, 'tools', None)
                    msg_contents = getattr(msg, 'contents', None)
                    msg_artifacts = getattr(msg, 'artifacts', None)
                    
                    # 处理工具调用结果
                    if msg_tools:
                        for tool in msg_tools:
                            # 支持 dict 和对象两种类型
                            if isinstance(tool, dict):
                                tool_name = tool.get("name") or tool.get("id")
                                tool_result = tool.get("result")
                                tool_args = tool.get("arguments")
                                tool_status = tool.get("status")
                                tool_contents = tool.get("contents", [])
                            else:
                                # 对象类型
                                tool_name = getattr(tool, 'name', None) or getattr(tool, 'id', None)
                                tool_result = getattr(tool, 'result', None)
                                tool_args = getattr(tool, 'arguments', None)
                                tool_status = getattr(tool, 'status', None)
                                tool_contents = getattr(tool, 'contents', []) or []
                                
                            logger.info(f"[data_agent] 工具调用: name={tool_name}, status={tool_status}")
                            
                            # 提取 SQL（如果是 QuerySLSLogs 工具）
                            if tool_name == "QuerySLSLogs" and tool_args:
                                if isinstance(tool_args, dict) and "query" in tool_args:
                                    collected_sql = tool_args["query"]
                                    logger.info(f"[data_agent] 提取到SQL: {collected_sql}")
                            
                            # 处理工具内容（data-agent 返回的数据）
                            if tool_contents and tool_status == "success":
                                for tc in tool_contents:
                                    # 支持 dict 和对象两种类型
                                    if isinstance(tc, dict):
                                        tc_value = tc.get("value")
                                        tc_type = tc.get("type", "unknown")
                                    else:
                                        tc_value = getattr(tc, 'value', None)
                                        tc_type = getattr(tc, 'type', "unknown")
                                    
                                    if tc_value:
                                        logger.info(f"[data_agent] 工具内容: type={tc_type}, value_type={type(tc_value).__name__}, "
                                                   f"value_preview={str(tc_value)[:200] if tc_value else None}")
                                        
                                        if isinstance(tc_value, str):
                                            # 尝试解析为 JSON
                                            try:
                                                tc_data = json.loads(tc_value)
                                                logger.info(f"[data_agent] JSON解析成功: keys={list(tc_data.keys()) if isinstance(tc_data, dict) else 'list'}")
                                            except json.JSONDecodeError:
                                                # 如果 JSON 解析失败，尝试 YAML 解析
                                                if tc_value.startswith("type:"):
                                                    try:
                                                        tc_data = yaml.safe_load(tc_value)
                                                        logger.info(f"[data_agent] YAML解析成功: type={tc_data.get('type') if isinstance(tc_data, dict) else 'unknown'}")
                                                    except yaml.YAMLError:
                                                        tc_data = None
                                                        logger.warning(f"[data_agent] YAML解析失败")
                                                else:
                                                    tc_data = None
                                        else:
                                            tc_data = tc_value
                                        
                                        # 收集 entity_list, metric_set_query 等数据
                                        if isinstance(tc_data, dict) and tc_data.get("type"):
                                            collected_data.append(tc_data)
                                            logger.info(f"[data_agent] 收集到数据: type={tc_data.get('type')}, "
                                                       f"data_count={len(tc_data.get('data', []))}")
                                        
                                        # 收集文本内容（如 generate_diagnosis_report）
                                        if tool_name == "generate_diagnosis_report" and tc_type == "text":
                                            collected_text.append(tc_value)
                            
                            # 收集工具结果
                            if tool_result:
                                collected_tool_results.append({
                                    "tool": tool_name,
                                    "result": tool_result,
                                    "arguments": tool_args,
                                    "status": tool_status
                                })
                                logger.info(f"[data_agent] 收集工具结果: tool={tool_name}")
                    
                    # 处理文本内容
                    if msg_contents:
                        for content_item in msg_contents:
                            # 支持 dict 和对象两种类型
                            if isinstance(content_item, dict):
                                content_type = content_item.get("type")
                                content_value = content_item.get("value", "")
                            else:
                                content_type = getattr(content_item, 'type', None)
                                content_value = getattr(content_item, 'value', "")
                            
                            if content_type == "text" and content_value:
                                # 只收集非空的文本内容
                                if content_value.strip():
                                    collected_text.append(content_value)
                            elif content_type == "data" and content_value:
                                # 处理数据类型的内容
                                try:
                                    if isinstance(content_value, str):
                                        data_value = json.loads(content_value)
                                    else:
                                        data_value = content_value
                                    collected_data.append(data_value)
                                    logger.info(f"[data_agent] 收集到content数据: type={data_value.get('type') if isinstance(data_value, dict) else 'unknown'}")
                                except json.JSONDecodeError:
                                    collected_data.append(content_value)
                    
                    # 处理 artifacts（可能包含查询结果数据）
                    if msg_artifacts:
                        for artifact in msg_artifacts:
                            # 支持 dict 和对象两种类型
                            if isinstance(artifact, dict):
                                artifact_type = artifact.get("type")
                                artifact_value = artifact.get("value")
                            else:
                                artifact_type = getattr(artifact, 'type', None)
                                artifact_value = getattr(artifact, 'value', None)
                            
                            if artifact_value:
                                try:
                                    if isinstance(artifact_value, str):
                                        artifact_data = json.loads(artifact_value)
                                    else:
                                        artifact_data = artifact_value
                                    
                                    # 如果是列表，逐个添加
                                    if isinstance(artifact_data, list):
                                        for item in artifact_data:
                                            if isinstance(item, dict) and item.get("type"):
                                                collected_data.append(item)
                                                logger.info(f"[data_agent] 从artifact收集到数据: type={item.get('type')}")
                                    elif isinstance(artifact_data, dict) and artifact_data.get("type"):
                                        collected_data.append(artifact_data)
                                        logger.info(f"[data_agent] 从artifact收集到数据: type={artifact_data.get('type')}")
                                except (json.JSONDecodeError, TypeError) as e:
                                    logger.debug(f"[data_agent] 解析artifact失败: {e}")
        
        # 输出收集结果汇总
        logger.info(f"[data_agent] SSE流结束，收集结果: data_count={len(collected_data)}, "
                   f"text_count={len(collected_text)}, tool_results_count={len(collected_tool_results)}, "
                   f"has_sql={bool(collected_sql)}, trace_id={trace_id}")
        
        # 构建响应
        explanation = "".join(collected_text)
        
        # 整合所有数据
        result_data = {
            "query_results": collected_data if collected_data else None,
            "tool_results": collected_tool_results if collected_tool_results else None,
        }
        
        # 如果有 SQL，添加到结果中
        if collected_sql:
            result_data["generated_sql"] = collected_sql
        
        # 如果有工具结果，尝试提取主要数据
        if collected_tool_results:
            for tool_result in collected_tool_results:
                if tool_result.get("result"):
                    try:
                        parsed_result = tool_result["result"]
                        if isinstance(parsed_result, str):
                            parsed_result = json.loads(parsed_result)
                        if isinstance(parsed_result, dict) and "data" in parsed_result:
                            result_data["data"] = parsed_result["data"]
                            logger.info(f"[data_agent] 从工具结果提取到data字段")
                            break
                        elif isinstance(parsed_result, list):
                            result_data["data"] = parsed_result
                            logger.info(f"[data_agent] 从工具结果提取到列表数据")
                            break
                    except (json.JSONDecodeError, TypeError) as e:
                        logger.debug(f"[data_agent] 解析工具结果失败: {e}")
        
        logger.info(f"[data_agent] 查询完成，trace_id: {trace_id}, "
                   f"has_query_results={bool(result_data.get('query_results'))}, "
                   f"has_explanation={bool(explanation)}")
        
        return {
            "data": result_data,
            "message": explanation if explanation else "查询完成",
            "trace_id": trace_id or "",
            "error": False,
            "time_range": {
                "from_time": from_time,
                "to_time": to_time
            }
        }
            
    except Exception as e:
        logger.error(f"调用数据查询失败，异常详情: {str(e)}", exc_info=True)
        return {
            "data": None,
            "message": f"查询失败: {str(e)}",
            "trace_id": "",
            "error": True,
            "time_range": {
                "from_time": from_time,
                "to_time": to_time
            }
        }
