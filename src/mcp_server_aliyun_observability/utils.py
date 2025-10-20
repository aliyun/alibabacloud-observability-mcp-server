import hashlib
import json
import logging
import os.path
from datetime import datetime
from functools import wraps
from pathlib import Path
from typing import Any, Callable, Optional, TypeVar, cast

from alibabacloud_arms20190808.client import Client as ArmsClient
from alibabacloud_credentials.client import Client as CredClient
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
from mcp_server_aliyun_observability.settings import (
    get_settings,
    normalize_host,
)
from mcp_server_aliyun_observability.logger import log_info

logger = logging.getLogger(__name__)


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
        try:
            # 将路径转换为绝对路径，支持用户目录（~）和环境变量（如 $HOME）
            expanded_path = os.path.expandvars(file_path)
            self.file_path = Path(expanded_path).expanduser().resolve()
            with open(self.file_path, 'r', encoding='utf-8') as file:
                self.config = json.load(file)
                logger.warning(f"已加载外部知识库配置文件 {self.file_path}")
        except FileNotFoundError:
            logger.warning(f"外部知识库配置文件 {self.file_path} 不存在")
        except json.JSONDecodeError as e:
            logger.warning(f"外部知识库配置 JSON 格式错误: {e}")

        # 全局默认 endpoint
        self.global_default = self.config.get("default_endpoint", None)

        # 项目配置
        self.projects = self.config.get("projects", {})

    def get_config(self, project:str, logstore:str) -> str:
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

    def __init__(self, access_key_id: str, access_key_secret: str, knowledge_config: str):
        self.access_key_id = access_key_id
        self.access_key_secret = access_key_secret
        self.knowledge_config = KnowledgeEndpoint(knowledge_config) if knowledge_config else None
    
    
class SLSClientWrapper:
    """
    A wrapper for aliyun client
    """

    def __init__(self, credential: Optional[CredentialWrapper] = None):
        self.credential = credential

    def with_region(
        self, region: str = None, endpoint: Optional[str] = None
    ) -> SLSClient:
        if self.credential:
            config = open_api_models.Config(
                access_key_id=self.credential.access_key_id,
                access_key_secret=self.credential.access_key_secret,
            )
        else:
            credentialsClient = CredClient()
            config = open_api_models.Config(credential=credentialsClient)
        # Prefer explicit endpoint > global settings resolver (mapping > template)
        settings = get_settings().sls
        if endpoint:
            host = normalize_host(endpoint)
            source = "explicit"
        else:
            # detect mapping vs template for logging clarity
            if region in settings.endpoints:
                host = settings.endpoints[region]
                source = "mapping"
            else:
                host = settings.resolve(region)
                source = "template"
        # Log which region/endpoint is used
        try:
            log_info(f"SLS endpoint resolved: region={region}, endpoint={host}, source={source}")
        except Exception:
            pass
        config.endpoint = host
        return SLSClient(config)
    
    def get_knowledge_config(self, project: str, logstore: str) -> str:
        if self.credential and self.credential.knowledge_config:
            res = self.credential.knowledge_config.get_config(project, logstore)
            if "uri" in res and "key" in res:
                return res
        return None


class ArmsClientWrapper:
    """
    A wrapper for aliyun arms client
    """

    def __init__(self, credential: Optional[CredentialWrapper] = None):
        self.credential = credential

    def with_region(self, region: str, endpoint: Optional[str] = None) -> ArmsClient:
        if self.credential:
            config = open_api_models.Config(
                access_key_id=self.credential.access_key_id,
                access_key_secret=self.credential.access_key_secret,
            )
        else:
            credentialsClient = CredClient()
            config = open_api_models.Config(credential=credentialsClient)
        # Prefer explicit endpoint > global settings resolver
        arms_settings = get_settings().arms
        if endpoint:
            host = normalize_host(endpoint)
            source = "explicit"
        else:
            if region in arms_settings.endpoints:
                host = arms_settings.endpoints[region]
                source = "mapping"
            else:
                host = arms_settings.resolve(region)
                source = "template"
        try:
            log_info(f"ARMS endpoint resolved: region={region}, endpoint={host}, source={source}")
        except Exception:
            pass
        config.endpoint = host
        return ArmsClient(config)


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
            for error in TEQ_EXCEPTION_ERROR:
                if e.code == error["errorCode"]:
                    return cast(
                        T,
                        {
                            "solution": error["solution"],
                            "message": error["errorMessage"],
                        },
                    )
            message=e.message
            if "Max retries exceeded with url" in message:
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
            raise e

    return wrapper


def text_to_sql(
    ctx: Context, text: str, project: str, log_store: str, region_id: str
) -> dict[str, Any]:
    try:
        sls_client_wrapper = ctx.request_context.lifespan_context["sls_client"]
        sls_client: Client = sls_client_wrapper.with_region("cn-shanghai")
        knowledge_config = sls_client_wrapper.get_knowledge_config(project, log_store)
        request: CallAiToolsRequest = CallAiToolsRequest()
        request.tool_name = "text_to_sql"
        request.region_id = region_id
        params: dict[str, Any] = {
            "project": project,
            "logstore": log_store,
            "sys.query": append_current_time(text),
            "external_knowledge_uri": knowledge_config["uri"] if knowledge_config else "",
            "external_knowledge_key": knowledge_config["key"] if knowledge_config else "",
        }
        request.params = params
        runtime: util_models.RuntimeOptions = util_models.RuntimeOptions()
        runtime.read_timeout = 60000
        runtime.connect_timeout = 60000
        tool_response: CallAiToolsResponse = sls_client.call_ai_tools_with_options(
            request=request, headers={}, runtime=runtime
        )
        data = tool_response.body
        if "------answer------\n" in data:
            data = data.split("------answer------\n")[1]
        return {
            "data": data,
            "requestId": tool_response.headers.get("x-log-requestid", ""),
        }
    except Exception as e:
        logger.error(f"调用SLS AI工具失败: {str(e)}")
        raise

def append_current_time(text: str) -> str:
    """
    添加当前时间
    """
    return f"当前时间: {get_current_time()},问题:{text}"
