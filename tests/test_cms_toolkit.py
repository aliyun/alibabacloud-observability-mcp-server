import os
from datetime import datetime

import dotenv
import pytest
from mcp.server.fastmcp import Context, FastMCP
from mcp.shared.context import RequestContext

from mcp_server_aliyun_observability.server import create_lifespan
from mcp_server_aliyun_observability.toolkit.cms_toolkit import CMSToolkit
from mcp_server_aliyun_observability.utils import CredentialWrapper, SLSClientWrapper

dotenv.load_dotenv()

import logging

logger = logging.getLogger(__name__)


@pytest.fixture
def mcp_server():
    """创建模拟的FastMCP服务器实例"""
    mcp_server = FastMCP(
        name="mcp_aliyun_observability_server",
        lifespan=create_lifespan(
            CredentialWrapper(
                os.getenv("ALIYUN_ACCESS_KEY_ID"), os.getenv("ALIYUN_ACCESS_KEY_SECRET")
            )
        ),
    )
    return mcp_server


@pytest.fixture
def mock_request_context():
    """创建模拟的RequestContext实例"""
    context = Context(
        request_context=RequestContext(
            request_id="test_request_id",
            meta=None,
            session=None,
            lifespan_context={
                "cms_client": SLSClientWrapper(
                    CredentialWrapper(
                        os.getenv("ALIYUN_ACCESS_KEY_ID"),
                        os.getenv("ALIYUN_ACCESS_KEY_SECRET"),
                    )
                ),
            },
        )
    )
    return context


@pytest.fixture
def tool_manager(mcp_server):
    """创建ToolManager实例"""
    return CMSToolkit(mcp_server)


@pytest.mark.asyncio
async def test_cms_summarize_alert_events_success(
    tool_manager: CMSToolkit,
    mcp_server: FastMCP,
    mock_request_context: Context,
):
    """测试CMS 告警总结成功的情况"""
    tool = mcp_server._tool_manager.get_tool("cms_summarize_alert_events")
    text = await tool.run(
        {
            "fromTimestampInSeconds": int(datetime.now().timestamp()) - 3600,
            "toTimestampInSeconds": int(datetime.now().timestamp()),
            "regionId": os.getenv("TEST_REGION"),
        },
        context=mock_request_context,
    )
    assert text is not None
    # """
    #  response_body: List[Dict[str, Any]] = response.body
    #         result = {
    #             "data": response_body,
    # """
    # item = text["data"][0]
    # assert item["total"] is not None
    # assert text["message"] == "success"


@pytest.mark.asyncio
async def test_cms_execute_promql_query_success(
    tool_manager: CMSToolkit,
    mcp_server: FastMCP,
    mock_request_context: Context,
):
    """测试PromQL查询执行成功的情况"""
    tool = mcp_server._tool_manager.get_tool("cms_execute_promql_query")
    text = await tool.run(
        {
            "project": os.getenv("TEST_PROJECT"),
            "metricStore": os.getenv("TEST_METRICSTORE"),
            "query": "sum(kube_pod_info) by (namespace)",
            "fromTimestampInSeconds": int(datetime.now().timestamp()) - 3600,
            "toTimestampInSeconds": int(datetime.now().timestamp()),
            "regionId": os.getenv("TEST_REGION"),
        },
        context=mock_request_context,
    )
    assert text is not None
    # """
    #  response_body: List[Dict[str, Any]] = response.body
    #         result = {
    #             "data": response_body,
    # """
    # item = text["data"][0]
    # assert item["total"] is not None
    # assert text["message"] == "success"
