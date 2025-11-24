import hashlib
import json
import logging
import os.path
from datetime import datetime
from functools import wraps
from pathlib import Path
from typing import Any, Callable, Dict, List, Optional, TypeVar, Union, cast

import pandas as pd
from alibabacloud_cms20240330 import models as cms_model
from alibabacloud_cms20240330.client import Client as CmsClient
from alibabacloud_credentials.client import Client as CredClient
from alibabacloud_credentials.models import Config as CredConfig
from alibabacloud_credentials.utils import auth_util
from alibabacloud_sls20201230.client import Client
from alibabacloud_sls20201230.client import Client as SLSClient
from alibabacloud_sls20201230.models import (
    CallAiToolsRequest,
    CallAiToolsResponse,
    IndexJsonKey,
)
from alibabacloud_tea_openapi import models as open_api_models
from alibabacloud_tea_util import models as util_models
from mcp.server.fastmcp import Context
from Tea.exceptions import TeaException
from mcp_server_aliyun_observability.api_error import TEQ_EXCEPTION_ERROR

from mcp_server_aliyun_observability.settings import get_settings, normalize_host
from mcp_server_aliyun_observability.logger import get_logger

logger = get_logger()


def _create_unified_config(
    credential: Optional["CredentialWrapper"] = None,
) -> open_api_models.Config:
    """
    创建统一的阿里云客户端配置

    Args:
        credential: 可选的凭据包装器

    Returns:
        open_api_models.Config: 配置对象
    """
    if credential:
        return open_api_models.Config(
            access_key_id=credential.access_key_id,
            access_key_secret=credential.access_key_secret,
        )
    elif auth_util.environment_role_arn:
        logger.info(f"使用Role-ARN: {auth_util.environment_role_arn} 获取客户端")
        credentialsClient = CredClient(
            CredConfig(
                type="ram_role_arn",
                access_key_id=auth_util.environment_access_key_id,
                access_key_secret=auth_util.environment_access_key_secret,
                role_arn=auth_util.environment_role_arn,
                role_session_name=auth_util.environment_role_session_name,
            )
        )
        return open_api_models.Config(credential=credentialsClient)
    else:
        credentialsClient = CredClient()
        return open_api_models.Config(credential=credentialsClient)


class KnowledgeEndpoint:
    """外部知识库配置
    该类用于加载和管理外部知识库的配置，包括全局/Project/Logstore级别的外部知识库 endpoint 配置。
    其配置优先级：Logstore > Project default > Global default
    配置文件示例如下：
    ```json
    {
    "default_endpoint": {"uri": "https://api.default.com", "key": "Bearer dataset-***"},
    "projects": {
        "project1": {
            "default_endpoint": {"uri": "https://api.project1.com", "key": "Bearer dataset-***"},
            "logstore1": {"uri": "https://api.project1.logstore1.com","key": "Bearer dataset-***"},
            "logstore2": {"uri": "https://api.project1.logstore2.com","key": "Bearer dataset-***"}
        },
        "project2": {
            "logstore3": {"uri": "https://api.project2.logstore3.com","key": "Bearer dataset-***"}
        }
    }
    ```
    }
    """

    def __init__(self, file_path):
        logger.info(f"初始化外部知识库配置，文件路径: {file_path}")

        try:
            # 将路径转换为绝对路径，支持用户目录（~）和环境变量（如 $HOME）
            expanded_path = os.path.expandvars(file_path)
            self.file_path = Path(expanded_path).expanduser().resolve()

            with open(self.file_path, "r", encoding="utf-8") as file:
                self.config = json.load(file)
                logger.info(f"成功加载外部知识库配置文件: {self.file_path}")

        except FileNotFoundError:
            logger.warning(f"外部知识库配置文件不存在: {self.file_path}")
            self.config = {}
        except json.JSONDecodeError as e:
            logger.error(f"外部知识库配置JSON格式错误: {e}", exc_info=True)
            self.config = {}
        except Exception as e:
            logger.error(f"加载外部知识库配置失败，异常详情: {str(e)}", exc_info=True)
            self.config = {}

        # 全局默认 endpoint
        self.global_default = self.config.get("default_endpoint", None)

        # 项目配置
        self.projects = self.config.get("projects", {})

        logger.info(
            f"外部知识库配置初始化完成，全局默认配置: {self.global_default is not None}, 项目配置数量: {len(self.projects)}"
        )

    def get_config(self, project: str, logstore: str) -> str:
        """获取指定项目和日志仓库的外部知识库 endpoint 配置
        优先级：logstore > project default > global default
        :param project: 项目名称
        :param logstore: 日志仓库名称
        :return: 外部知识库 endpoint
        """
        project_config = self.projects.get(project, None)
        if project_config is None:
            return self.global_default

        logstore_config = project_config.get(logstore)
        if logstore_config is None:
            return self.project_config.get("default_endpoint", None)

        return logstore_config


class CredentialWrapper:
    """
    A wrapper for aliyun credentials
    """

    access_key_id: str
    access_key_secret: str
    knowledge_config: KnowledgeEndpoint

    def __init__(
        self,
        access_key_id: str,
        access_key_secret: str,
        knowledge_config: Optional[str] = None,
    ):
        self.access_key_id = access_key_id
        self.access_key_secret = access_key_secret
        self.knowledge_config = (
            KnowledgeEndpoint(knowledge_config) if knowledge_config else None
        )


class CMSClientWrapper:
    """
    A wrapper for aliyun cms client
    """

    def __init__(self, credential: Optional[CredentialWrapper] = None):
        self.credential = credential

    def with_region(self, region: str, endpoint: Optional[str] = None) -> CmsClient:
        config = _create_unified_config(self.credential)
        cms_settings = get_settings().cms
        if endpoint:
            host = normalize_host(endpoint)
            source = "explicit"
        elif region in cms_settings.endpoints:
            host = cms_settings.endpoints[region]
            source = "mapping"
        else:
            host = cms_settings.resolve(region)
            source = "template"
        logger.info(
            f"CMS endpoint resolved: region={region}, endpoint={host}, source={source}"
        )
        config.endpoint = host
        return CmsClient(config)


class SLSClientWrapper:
    """
    A wrapper for aliyun client
    """

    def __init__(self, credential: Optional[CredentialWrapper] = None):
        self.credential = credential

    def with_region(
        self, region: str = None, endpoint: Optional[str] = None
    ) -> SLSClient:
        config = _create_unified_config(self.credential)
        settings = get_settings().sls
        if endpoint:
            host = normalize_host(endpoint)
            source = "explicit"
        elif region in settings.endpoints:
            host = settings.endpoints[region]
            source = "mapping"
        else:
            host = settings.resolve(region)
            source = "template"
        logger.info(
            f"SLS endpoint resolved: region={region}, endpoint={host}, source={source}"
        )
        config.endpoint = host
        return SLSClient(config)

    def get_knowledge_config(self, project: str, logstore: str) -> str:
        if self.credential and self.credential.knowledge_config:
            res = self.credential.knowledge_config.get_config(project, logstore)
            if "uri" in res and "key" in res:
                return res
        return None


def parse_json_keys(json_keys: dict[str, IndexJsonKey]) -> dict[str, dict[str, str]]:
    result: dict[str, dict[str, str]] = {}
    for key, value in json_keys.items():
        result[key] = {
            "alias": value.alias,
            "sensitive": value.case_sensitive,
            "type": value.type,
        }
    return result


def get_arms_user_trace_log_store(user_id: int, region: str) -> dict[str, str]:
    """
    get the log store name of the user's trace
    """
    # project是基于 user_id md5,proj-xtrace-xxx-cn-hangzhou
    if "finance" in region:
        text = str(user_id) + region
        project = f"proj-xtrace-{md5_string(text)}"
    else:
        text = str(user_id)
        project = f"proj-xtrace-{md5_string(text)}-{region}"
    # logstore-xtrace-1277589232893727-cn-hangzhou
    log_store = "logstore-tracing"
    return {"project": project, "log_store": log_store}


def get_current_time() -> str:
    """
    获取当前时间
    """
    return {
        "current_time": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
        "current_timestamp": int(datetime.now().timestamp()),
    }


def md5_string(origin: str) -> str:
    """
    计算字符串的MD5值，与Java实现对应

    Args:
        origin: 要计算MD5的字符串

    Returns:
        MD5值的十六进制字符串
    """
    buf = origin.encode()

    md5 = hashlib.md5()

    md5.update(buf)

    tmp = md5.digest()

    sb = []
    for b in tmp:
        hex_str = format(b & 0xFF, "x")
        sb.append(hex_str)

    return "".join(sb)


T = TypeVar("T")


def handle_tea_exception(func: Callable[..., T]) -> Callable[..., T]:
    """
    装饰器：处理阿里云 SDK 的 TeaException 异常

    Args:
        func: 被装饰的函数

    Returns:
        装饰后的函数，会自动处理 TeaException 异常
    """

    @wraps(func)
    def wrapper(*args, **kwargs) -> T:
        try:
            return func(*args, **kwargs)
        except TeaException as e:
            logger.error(
                f"捕获TeaException异常，错误代码: {e.code}, 错误消息: {e.message}"
            )

            for error in TEQ_EXCEPTION_ERROR:
                if e.code == error["errorCode"]:
                    logger.warning(f"已知异常处理: {error['errorCode']}")
                    return cast(
                        T,
                        {
                            "solution": error["solution"],
                            "message": error["errorMessage"],
                        },
                    )

            message = e.message
            if "Max retries exceeded with url" in message:
                logger.error(f"网络连接异常: {message}")
                return cast(
                    T,
                    {
                        "solution": """
                        可能原因:
                            1.	当前网络不具备访问内网域名的权限（如从公网或不通阿里云 VPC 访问）；
                            2.	指定 region 错误或不可用；
                            3.	工具或网络中存在代理、防火墙限制；
                            如果你需要排查，可以从：
                            •	尝试 ping 下域名是否可联通
                            •	查看是否有 VPC endpoint 配置错误等，如果是非VPC 环境，请配置公网入口端点，一般公网端点不会包含-intranet 等字样
                            """,
                        "message": e.message,
                    },
                )
            logger.error(f"未处理的TeaException异常，重新抛出: {e}")
            raise e

    return wrapper


def text_to_sql(
    ctx: Context, text: str, project: str, log_store: str, region_id: str
) -> dict[str, Any]:
    logger.info(
        f"开始文本转SQL查询，输入参数: text={text}, project={project}, log_store={log_store}, region_id={region_id}"
    )

    try:
        sls_client_wrapper = ctx.request_context.lifespan_context["sls_client"]
        sls_client: Client = sls_client_wrapper.with_region("cn-shanghai")
        knowledge_config = sls_client_wrapper.get_knowledge_config(project, log_store)

        logger.info(f"获取知识库配置: {knowledge_config is not None}")

        request: CallAiToolsRequest = CallAiToolsRequest()
        request.tool_name = "text_to_sql"
        request.region_id = region_id

        params: dict[str, Any] = {
            "project": project,
            "logstore": log_store,
            "sys.query": append_current_time(text),
            "external_knowledge_uri": knowledge_config["uri"]
            if knowledge_config
            else "",
            "external_knowledge_key": knowledge_config["key"]
            if knowledge_config
            else "",
        }
        request.params = params

        logger.info(f"构建SLS AI工具请求，工具名称: {request.tool_name}")

        runtime: util_models.RuntimeOptions = util_models.RuntimeOptions()
        runtime.read_timeout = 60000
        runtime.connect_timeout = 60000

        tool_response: CallAiToolsResponse = sls_client.call_ai_tools_with_options(
            request=request, headers={}, runtime=runtime
        )

        data = tool_response.body
        if "------answer------\n" in data:
            data = data.split("------answer------\n")[1]

        result = {
            "data": data,
            "requestId": tool_response.headers.get("x-log-requestid", ""),
        }

        logger.info(f"文本转SQL查询成功，请求ID: {result['requestId']}")
        return result

    except Exception as e:
        logger.error(f"调用SLS AI工具失败，异常详情: {str(e)}", exc_info=True)
        raise


def append_current_time(text: str) -> str:
    """
    添加当前时间
    """
    return f"当前时间: {get_current_time()},问题:{text}"


def execute_cms_query(
    cms_client: CmsClient,
    workspace_name: str,
    query: str,
    from_timestamp: Optional[int] = None,
    to_timestamp: Optional[int] = None,
) -> pd.DataFrame:
    """
    执行cms查询
    """

    try:
        if not from_timestamp:
            ###过去五分钟
            from_timestamp = int(datetime.now().timestamp()) - 300
        if not to_timestamp:
            to_timestamp = int(datetime.now().timestamp())
        logger.info(
            f"开始执行CMS查询，输入参数: workspace_name={workspace_name}, query={query}, from_timestamp={from_timestamp}, to_timestamp={to_timestamp}"
        )

        request = cms_model.GetEntityStoreDataRequest(
            query=query, from_=from_timestamp, to=to_timestamp
        )

        logger.info(f"发送CMS请求到workspace: {workspace_name}")
        response: cms_model.GetEntityStoreDataResponse = (
            cms_client.get_entity_store_data(workspace_name, request).body
        )
        logger.info(f"CMS查询响应: {response}")
        data, header = response.data, response.header
        df_result = pd.DataFrame(data, columns=header)
        logger.info(f"CMS查询成功，返回{len(df_result)}条结果")
        return df_result

    except Exception as e:
        logger.error(f"执行CMS查询失败，异常详情: {str(e)}", exc_info=True)
        raise


def execute_cms_query_with_context(
    ctx: Context,
    query: str,
    workspace: str,
    region_id: str,
    from_time: Union[str, int],
    to_time: Union[str, int],
    limit: int = 1000,
) -> Dict[str, Any]:
    """Execute SPL query against CMS entity store with full context handling

    Enhanced version of execute_cms_query that provides:
    - Time expression parsing (supports both timestamps and relative expressions)
    - Context management and error handling
    - Standardized response format
    - Result limiting

    Args:
        ctx: MCP context for accessing CMS client
        query: SPL query statement
        workspace: CMS workspace name
        region_id: Aliyun region ID
        from_time: Start time - Unix timestamp (seconds/milliseconds) or relative expression (now-5m)
        to_time: End time - Unix timestamp (seconds/milliseconds) or relative expression (now)
        limit: Maximum number of results to return

    Returns:
        Dict with standardized response format containing data, metadata, and status
    """
    try:
        # Import TimeRangeParser locally to avoid circular imports
        from mcp_server_aliyun_observability.toolkits.paas.time_utils import (
            TimeRangeParser,
        )

        # Parse time expressions to timestamps
        from_timestamp, to_timestamp = TimeRangeParser.parse_time_range(
            from_time, to_time
        )

        cms_client: CmsClient = ctx.request_context.lifespan_context[
            "cms_client"
        ].with_region(region_id)

        # Execute CMS query using the standard execute_cms_query function
        data = execute_cms_query(
            cms_client, workspace, query, from_timestamp, to_timestamp
        )

        # Convert DataFrame to dict format
        if hasattr(data, "to_dict"):
            result_data = data.to_dict("records")
        else:
            result_data = data if isinstance(data, list) else []

        # Apply limit if needed (CMS might return more than requested)
        if len(result_data) > limit:
            result_data = result_data[:limit]

        return {
            "error": False,
            "data": result_data,
            "query": query,
            "workspace": workspace,
            "time_range": {"from_time": from_timestamp, "to_time": to_timestamp},
            "message": f"success, returned {len(result_data)} records"
            if result_data
            else "No data found",
        }
    except Exception as e:
        logger.error(f"CMS查询执行失败: {str(e)}", exc_info=True)
        return {
            "error": True,
            "data": [],
            "query": query,
            "workspace": workspace,
            "message": f"Query execution failed: {str(e)}",
        }
