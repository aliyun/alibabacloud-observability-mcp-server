"""工具类通用函数"""

import json
from typing import Any, Dict, List, Optional

from alibabacloud_sls20201230.client import Client
from alibabacloud_sls20201230.models import CallAiToolsRequest, CallAiToolsResponse
from alibabacloud_tea_util import models as util_models
from mcp.server.fastmcp import Context
from tenacity import retry, retry_if_exception_type, stop_after_attempt, wait_fixed

from mcp_server_aliyun_observability.config import Config
from mcp_server_aliyun_observability.logger import logger


def build_user_context_from_params(**kwargs) -> List[Dict[str, Any]]:
    """
    通用的用户上下文构建函数，从各种参数中提取实体信息

    支持的参数（统一参数结构）:
    - entity_domain: 实体域（如 apm, k8s, cloud_product）
    - entity_type: 实体类型（如 apm.service, k8s.pod）
    - entity_id: 实体ID（可选，精确指定实体）

    Args:
        **kwargs: 各种MCP工具的参数

    Returns:
        List[Dict[str, Any]]: 符合规范的user_context数组
    """
    user_context = []

    # 处理统一参数结构 entity_domain 和 entity_type
    if "entity_domain" in kwargs or "entity_type" in kwargs:
        entity_domain = kwargs.get("entity_domain", "") or ""
        entity_type = kwargs.get("entity_type", "") or ""

        # 只有在有domain或type时才添加entity上下文
        if entity_domain or entity_type:
            entity_context = {
                "type": "entity",
                "data": {
                    "entity_type": entity_type,
                    "entity_domain": entity_domain,
                },
            }

            # 如果有entity_id，则添加到data中
            if "entity_id" in kwargs and kwargs["entity_id"]:
                entity_context["data"]["entity_id"] = kwargs["entity_id"]

            user_context.append(entity_context)

    # 兼容旧的 domain 参数（保留最小兼容性）
    elif "domain" in kwargs:
        entity_domain = kwargs.get("domain", "") or ""
        entity_type = kwargs.get("entity_type", "") or kwargs.get("type", "") or ""

        if entity_domain or entity_type:
            entity_context = {
                "type": "entity",
                "data": {
                    "entity_type": entity_type,
                    "entity_domain": entity_domain,
                },
            }

            if "entity_id" in kwargs and kwargs["entity_id"]:
                entity_context["data"]["entity_id"] = kwargs["entity_id"]

            user_context.append(entity_context)
    user_context.append({"type": "metadata", "data": {"source": "mcp"}})

    return user_context


@retry(
    stop=stop_after_attempt(Config.get_retry_attempts()),
    wait=wait_fixed(Config.RETRY_WAIT_SECONDS),
    retry=retry_if_exception_type(Exception),
    reraise=True,
)
def call_problem_agent(
    query: str,
    region_id: str,
    workspace: str,
    sls_client: Client,
    user_context: Optional[List[Dict[str, Any]]] = None,
) -> Any:
    """
    调用问题分析agent，使用call_ai_tools接口

    Args:
        query: 查询语句或问题描述
        region_id: 区域ID
        workspace: 工作空间（通常是logstore名称）
        sls_client: SLS客户端实例
        user_context: 用户上下文信息

    Returns:
        Any: AI工具返回的结果（通常是字典或其他结构化数据）

    Raises:
        Exception: 调用失败时抛出异常
    """
    try:
        # 构建请求
        request = CallAiToolsRequest()
        request.tool_name = "_data_query"
        request.region_id = region_id

        # 构建参数
        params: Dict[str, Any] = {
            "query": query,
            "user_context": json.dumps(user_context) if user_context else None,
            "workspace": workspace,
            "regionId": region_id,
        }

        request.params = params

        # 设置运行时配置
        runtime = util_models.RuntimeOptions()
        read_timeout, connect_timeout = Config.get_timeouts()
        runtime.read_timeout = read_timeout
        runtime.connect_timeout = connect_timeout

        # 调用AI工具
        logger.info(f"调用AI工具 data_query，参数: {params}")
        tool_response: CallAiToolsResponse = sls_client.call_ai_tools_with_options(
            request=request, headers={}, runtime=runtime
        )

        # 处理响应
        data = tool_response.body

        # 提取实际答案（如果有分隔符）
        if "------answer------\n" in data:
            data = data.split("------answer------\n")[1]

        logger.debug(f"AI工具 data_query 返回结果长度: {len(data)}")
        return data

    except Exception as e:
        logger.error(f"调用AI工具 data_query 失败: {str(e)}")
        raise


def extract_answer_from_ai_response(response: str) -> str:
    """
    从AI响应中提取答案部分

    Args:
        response: AI工具的原始响应

    Returns:
        str: 提取后的答案
    """
    if "------answer------\n" in response:
        return response.split("------answer------\n")[1]
    return response


def build_ai_tool_params(
    project: str, logstore: str, query: str, region_id: str, **kwargs: Any
) -> Dict[str, Any]:
    """
    构建AI工具调用参数

    Args:
        project: SLS项目名称
        logstore: 日志库名称
        query: 查询语句
        region_id: 区域ID
        **kwargs: 其他额外参数

    Returns:
        Dict[str, Any]: 参数字典
    """
    params = {
        "project": project,
        "logstore": logstore,
        "query": query,
        "region_id": region_id,
    }
    params.update(kwargs)
    return params


def call_data_query(
    ctx: Context,
    query: str,
    region_id: str,
    workspace: str,
    domain: Optional[str] = None,
    entity_type: Optional[str] = None,
    entity_id: Optional[str] = None,
    start_time: Optional[str] = None,
    end_time: Optional[str] = None,
    output_mode: Optional[str] = None,
    user_context: Optional[List[Dict[str, Any]]] = None,
    error_message_prefix: str = "查询失败",
    client_type: str = "sls_client",
    **kwargs: Any,
) -> Dict[str, Any]:
    """
    调用数据查询的通用方法

    Args:
        ctx: MCP上下文
        query: 查询语句
        region_id: 区域ID
        workspace: 工作空间
        domain: 实体域（可选）
        entity_type: 实体类型（可选）
        entity_id: 实体ID（可选）
        start_time: 开始时间（可选）
        end_time: 结束时间（可选）
        output_mode: 输出模式（可选）
        user_context: 用户上下文参数（可选，如果不提供会自动构建）
        error_message_prefix: 错误消息前缀
        client_type: 客户端类型（sls_client, cms_client, arms_client）
        **kwargs: 其他参数

    Returns:
        查询结果或错误信息
    """
    try:
        # 获取客户端
        client_wrapper = ctx.request_context.lifespan_context.get(client_type)
        if not client_wrapper:
            return {
                "error": True,
                "message": f"{client_type} 未初始化",
            }

        # 如果没有提供user_context，则自动构建
        if user_context is None:
            # 构建参数字典
            params = {
                "entity_domain": domain,
                "entity_type": entity_type,
                "entity_id": entity_id,
            }
            # 合并kwargs中的其他参数
            params.update(kwargs)
            user_context = build_user_context_from_params(**params)

        # 调用AI工具
        result = call_problem_agent(
            query=query,
            region_id=region_id,
            workspace=workspace,
            sls_client=client_wrapper.with_region("cn-shanghai"),
            user_context=user_context or [],
        )
        return {
            "error": False,
            "message": result,
        }

    except Exception as e:
        logger.error(f"{error_message_prefix}: {str(e)}")
        return {
            "error": True,
            "message": f"{error_message_prefix}: {str(e)}",
        }
