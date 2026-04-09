"""PaaS Entity Toolkit 测试用例

注意：这些测试需要真实的阿里云凭证，属于集成测试。
"""
import os
import sys

# Add the src directory to Python path to ensure imports work
src_dir = '/apsarapangu/SSDCache1/workspace/code/alibabacloud-observability-mcp-server/src'
if src_dir not in sys.path:
    sys.path.insert(0, src_dir)

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

dotenv.load_dotenv()

# 标记所有测试为集成测试（需要真实阿里云凭证）
pytestmark = [
    pytest.mark.integration,
    pytest.mark.skipif(
        not os.getenv("ALIBABA_CLOUD_ACCESS_KEY_ID") and not os.getenv("ALIYUN_ACCESS_KEY_ID"),
        reason="需要设置 ALIBABA_CLOUD_ACCESS_KEY_ID 或 ALIYUN_ACCESS_KEY_ID 环境变量才能运行"
    ),
]

import logging

logger = logging.getLogger(__name__)


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
    """创建模拟的FastMCP服务器实例用于实体工具测试"""
    mcp_server = FastMCP(
        name="mcp_aliyun_observability_entity_server",
        lifespan=create_lifespan(
            credential=CredentialWrapper(
                access_key_id=os.getenv("ALIYUN_ACCESS_KEY_ID"),
                access_key_secret=os.getenv("ALIYUN_ACCESS_KEY_SECRET"),
                knowledge_config=None,
            ),
        ),
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
            request_id="test_entity_request_id",
            meta=None,
            session=None,
            lifespan_context={
                "cms_client": CMSClientWrapper(
                    credential=CredentialWrapper(
                        access_key_id=os.getenv("ALIYUN_ACCESS_KEY_ID"),
                        access_key_secret=os.getenv("ALIYUN_ACCESS_KEY_SECRET"),
                        knowledge_config=None,
                    ),
                ),
                "sls_client": None,
                "arms_client": None,
            },
        )
    )
    return context


class TestPaaSEntityToolkit:
    """PaaS实体工具的测试类"""

    @pytest.mark.asyncio
    async def test_paas_get_entities_success(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS实体查询 - 使用 entity_filter 参数"""
        tool = mcp_server._tool_manager.get_tool("umodel_get_entities")
        result = await tool.run(
            {
                "domain": "apm",
                "entity_set_name": "apm.service",
                "entity_filter": "name=payment",  # 现在需要提供 entity_ids 或 entity_filter 之一
                "workspace": os.getenv("TEST_CMS_WORKSPACE", "apm"),
                "regionId": os.getenv("TEST_REGION", "cn-hangzhou"),
            },
            context=mock_request_context,
        )
        result = check_credentials_and_result(result)
        # 验证标准响应格式
        assert "error" in result
        assert "data" in result
        assert "message" in result
        assert "query" in result
        assert "time_range" in result

    @pytest.mark.asyncio
    async def test_paas_get_entities_with_entity_ids(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS实体查询 - 使用 entity_ids 参数"""
        tool = mcp_server._tool_manager.get_tool("umodel_get_entities")
        result = await tool.run(
            {
                "domain": "apm",
                "entity_set_name": "apm.service",
                "entity_ids": "service-1,service-2",  # 使用 entity_ids
                "workspace": os.getenv("TEST_CMS_WORKSPACE", "apm"),
                "regionId": os.getenv("TEST_REGION", "cn-hangzhou"),
            },
            context=mock_request_context,
        )
        result = check_credentials_and_result(result)
        # 验证标准响应格式
        assert "error" in result
        assert "data" in result
        assert "message" in result
        assert "query" in result
        assert "time_range" in result

    @pytest.mark.asyncio
    async def test_paas_get_entities_with_time_range(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS实体查询 - 使用 time_range 参数"""
        tool = mcp_server._tool_manager.get_tool("umodel_get_entities")
        result = await tool.run(
            {
                "domain": "apm",
                "entity_set_name": "apm.service",
                "entity_filter": "name=payment",
                "time_range": "last_3h",  # 使用新的 time_range 参数
                "workspace": os.getenv("TEST_CMS_WORKSPACE", "apm"),
                "regionId": os.getenv("TEST_REGION", "cn-hangzhou"),
            },
            context=mock_request_context,
        )
        result = check_credentials_and_result(result)
        # 验证标准响应格式
        assert "error" in result
        assert "time_range" in result
        assert result["time_range"]["expression"] == "last_3h"

    @pytest.mark.asyncio
    async def test_paas_get_neighbor_entities_success(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS邻居实体查询"""
        tool = mcp_server._tool_manager.get_tool("umodel_get_neighbor_entities")
        result = await tool.run(
            {
                "domain": "apm",
                "entity_set_name": "apm.service",
                "entity_id": "5a81706b75fe1295797af01544a31264",
                "workspace": os.getenv("TEST_CMS_WORKSPACE", "apm"),
                "regionId": os.getenv("TEST_REGION", "cn-hangzhou"),
            },
            context=mock_request_context,
        )
        result = check_credentials_and_result(result)

    @pytest.mark.asyncio
    async def test_paas_search_entities_success(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS实体搜索 - 使用新的 time_range 参数和返回 statistics/detail 格式"""
        tool = mcp_server._tool_manager.get_tool("umodel_search_entities")
        result = await tool.run(
            {
                "workspace": os.getenv("TEST_CMS_WORKSPACE", "apm"),
                "search_text": "payment",
                "domain": "apm",
                "entity_set_name": "apm.service",
                "time_range": "last_1h",
                "regionId": os.getenv("TEST_REGION", "cn-hangzhou"),
            },
            context=mock_request_context,
        )
        result = check_credentials_and_result(result)
        # 验证标准响应格式
        assert "error" in result
        assert "data" in result
        assert "message" in result
        assert "query" in result
        assert "time_range" in result
        # 验证 data 包含 statistics 和 detail 两部分
        if result.get("data"):
            assert "statistics" in result["data"]
            assert "detail" in result["data"]

    @pytest.mark.asyncio
    async def test_paas_search_entities_with_wildcard(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS实体搜索 - 使用通配符搜索所有域和类型"""
        tool = mcp_server._tool_manager.get_tool("umodel_search_entities")
        result = await tool.run(
            {
                "workspace": os.getenv("TEST_CMS_WORKSPACE", "apm"),
                "search_text": "test",
                "domain": "*",
                "entity_set_name": "*",
                "time_range": "last_3h",
                "regionId": os.getenv("TEST_REGION", "cn-hangzhou"),
            },
            context=mock_request_context,
        )
        result = check_credentials_and_result(result)
        # 验证标准响应格式
        assert "error" in result
        assert "data" in result
        assert "time_range" in result
        assert result["time_range"]["expression"] == "last_3h"


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
