import logging
import os
import sys

import dotenv
import pytest
from mcp.server.fastmcp import Context, FastMCP
from mcp.shared.context import RequestContext

from mcp_server_aliyun_observability.server import create_lifespan
from mcp_server_aliyun_observability.toolkits.paas.toolkit import register_paas_tools
from mcp_server_aliyun_observability.toolkits.shared.toolkit import (
    register_shared_tools,
)
from mcp_server_aliyun_observability.utils import CMSClientWrapper, CredentialWrapper

logger = logging.getLogger(__name__)
dotenv.load_dotenv()


def check_credentials_and_result(result):
    """检查凭证和结果，如果有凭证问题则跳过测试"""
    assert result is not None
    logger.info(f"测试结果: {result}")
    error = result.get("error")
    if error and "InvalidCredentials" in str(result.get("message", "")):
        pytest.skip("需要有效的阿里云凭证才能运行此测试")
    # 如果有error且不是凭证问题，则测试失败
    assert not error, f"测试结果: {result}"
    return result


@pytest.fixture
def mcp_server():
    """创建模拟的FastMCP服务器实例用于数据集工具测试"""
    mcp_server = FastMCP(
        name="mcp_aliyun_observability_dataset_server",
        lifespan=create_lifespan(),
    )
    # 注册PaaS工具包
    register_paas_tools(mcp_server)
    register_shared_tools(mcp_server)
    return mcp_server


@pytest.fixture
def mock_request_context():
    """创建模拟的RequestContext实例"""
    context = Context(
        request_context=RequestContext(
            request_id="test_dataset_request_id",
            meta=None,
            session=None,
            lifespan_context={
                "cms_client": CMSClientWrapper(),
                "sls_client": None,
                "arms_client": None,
            },
        )
    )
    return context


class TestPaaSDatasetToolkit:
    """PaaS数据集工具的测试类"""

    @pytest.mark.asyncio
    async def test_paas_list_data_set_success(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS数据集列表查询"""
        tool = mcp_server._tool_manager.get_tool("umodel_list_data_set")
        result = await tool.run(
            {
                "domain": "apm",
                "entity_set_name": "apm.service",
                "workspace": os.getenv("TEST_CMS_WORKSPACE", "apm"),
                "regionId": os.getenv("TEST_REGION", "cn-hangzhou"),
            },
            context=mock_request_context,
        )
        result = check_credentials_and_result(result)

    @pytest.mark.asyncio
    async def test_list_workspace_success(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS数据集列表查询"""
        tool = mcp_server._tool_manager.get_tool("list_workspace")
        result = await tool.run(
            {
                "regionId": "cn-qingdao",
            },
            context=mock_request_context,
        )
        result = check_credentials_and_result(result)

    @pytest.mark.asyncio
    async def test_paas_list_data_set_with_types(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS数据集列表查询 - 指定类型"""
        tool = mcp_server._tool_manager.get_tool("umodel_list_data_set")
        result = await tool.run(
            {
                "domain": "apm",
                "entity_set_name": "apm.service",
                "workspace": os.getenv("TEST_CMS_WORKSPACE", "apm"),
                "regionId": os.getenv("TEST_REGION", "cn-hangzhou"),
                "data_set_types": "metric_set",
            },
            context=mock_request_context,
        )
        result = check_credentials_and_result(result)

    @pytest.mark.asyncio
    async def test_paas_search_entity_set_success(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS实体集合搜索"""
        tool = mcp_server._tool_manager.get_tool("umodel_search_entity_set")
        result = await tool.run(
            {
                "search_text": "service",
                "domain": "apm",
                "workspace": os.getenv("TEST_CMS_WORKSPACE", "apm"),
                "regionId": os.getenv("TEST_REGION", "cn-hangzhou"),
            },
            context=mock_request_context,
        )
        result = check_credentials_and_result(result)

    @pytest.mark.asyncio
    async def test_paas_list_related_entity_set_success(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS相关实体集合列表查询"""
        tool = mcp_server._tool_manager.get_tool("umodel_list_related_entity_set")
        result = await tool.run(
            {
                "domain": "apm",
                "entity_set_name": "apm.service",
                "workspace": os.getenv("TEST_CMS_WORKSPACE", "apm"),
                "regionId": os.getenv("TEST_REGION", "cn-hangzhou"),
                "direction": "both",
            },
            context=mock_request_context,
        )
        result = check_credentials_and_result(result)


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
