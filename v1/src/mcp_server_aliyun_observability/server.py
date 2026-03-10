import os
from contextlib import asynccontextmanager
from typing import AsyncIterator, Optional

from alibabacloud_credentials.client import Client as CredClient
from mcp.server import FastMCP
from mcp_server_aliyun_observability.toolkits.iaas.toolkit import register_iaas_tools
from mcp_server_aliyun_observability.toolkits.paas.toolkit import register_paas_tools
from mcp_server_aliyun_observability.toolkits.shared.toolkit import (
    register_shared_tools,
)
from mcp_server_aliyun_observability.utils import (
    CMSClientWrapper,
    CredentialWrapper,
    SLSClientWrapper,
)


def create_lifespan(credential: Optional[CredentialWrapper] = None):
    @asynccontextmanager
    async def lifespan(fastmcp) -> AsyncIterator[dict]:
        sls_client = SLSClientWrapper(credential)
        cms_client = CMSClientWrapper(credential)
        yield {
            "sls_client": sls_client,
            "cms_client": cms_client,
        }

    return lifespan


def init_server(
    credential: Optional[CredentialWrapper] = None,
    log_level: str = "INFO",
    transport_port: int = 8000,
    host: str = "0.0.0.0",
):
    """initialize the global mcp server instance"""
    mcp_server = FastMCP(
        name="mcp_aliyun_observability_server",
        lifespan=create_lifespan(credential),
        log_level=log_level,
        port=transport_port,
        host=host,
    )

    # 根据 scope 环境变量注册相应的工具包
    scope = os.environ.get("MCP_TOOLKIT_SCOPE", "all").lower()
    registered_scopes = []

    if scope == "all" or scope == "iaas":
        register_iaas_tools(mcp_server)
        registered_scopes.append("IaaS")

    if scope == "all" or scope == "paas":
        register_paas_tools(mcp_server)
        registered_scopes.append("PaaS")

    # 注册共享工具 (所有层级都需要)
    register_shared_tools(mcp_server)
    print("已注册共享工具: list_workspace, list_domains")

    print(f"已注册工具包范围 [{scope}]: {', '.join(registered_scopes)}")

    return mcp_server


def server(
    credential: Optional[CredentialWrapper] = None,
    transport: str = "stdio",
    log_level: str = "INFO",
    transport_port: int = 8000,
    host: str = "0.0.0.0",
):
    server = init_server(credential, log_level, transport_port, host)
    server.run(transport)
    host: str = ("0.0.0.0",)
