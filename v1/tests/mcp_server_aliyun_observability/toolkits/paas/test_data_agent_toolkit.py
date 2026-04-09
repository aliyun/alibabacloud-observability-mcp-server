"""Data Agent Toolkit 测试用例

注意：这些测试需要真实的阿里云凭证，属于集成测试。
"""
import os

import dotenv
import pytest
from mcp.server.fastmcp import Context, FastMCP
from mcp.shared.context import RequestContext

from mcp_server_aliyun_observability.server import create_lifespan
from mcp_server_aliyun_observability.toolkits.paas.data_agent_toolkit import DataAgentToolkit
from mcp_server_aliyun_observability.utils import CMSClientWrapper, CredentialWrapper

dotenv.load_dotenv()

# 标记所有测试为集成测试（需要真实阿里云凭证）
pytestmark = [
    pytest.mark.integration,
    pytest.mark.skipif(
        not os.getenv("ALIBABA_CLOUD_ACCESS_KEY_ID") and not os.getenv("ALIYUN_ACCESS_KEY_ID"),
        reason="需要设置 ALIBABA_CLOUD_ACCESS_KEY_ID 或 ALIYUN_ACCESS_KEY_ID 环境变量才能运行"
    ),
    pytest.mark.skipif(
        not os.getenv("ALIBABA_CLOUD_WORKSPACE"),
        reason="需要设置 ALIBABA_CLOUD_WORKSPACE 环境变量才能运行"
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
    """创建模拟的FastMCP服务器实例用于数据代理工具测试"""
    mcp_server = FastMCP(
        name="mcp_aliyun_observability_data_agent_server",
        lifespan=create_lifespan(
            credential=CredentialWrapper(
                access_key_id=os.getenv("ALIYUN_ACCESS_KEY_ID"),
                access_key_secret=os.getenv("ALIYUN_ACCESS_KEY_SECRET"),
                knowledge_config=None,
            ),
        ),
    )
    # 注册数据代理工具
    DataAgentToolkit(mcp_server)
    return mcp_server


@pytest.fixture
def mock_request_context():
    """创建模拟的RequestContext实例"""
    context = Context(
        request_context=RequestContext(
            request_id="test_data_agent_request_id",
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


class TestDataAgentToolkit:
    """Data Agent工具的测试类"""

    @pytest.mark.asyncio
    async def test_cms_natural_language_query_tool_registered(
        self,
        mcp_server: FastMCP,
    ):
        """测试工具是否正确注册"""
        tool = mcp_server._tool_manager.get_tool("cms_natural_language_query")
        assert tool is not None, "cms_natural_language_query 工具应该被注册"

    @pytest.mark.asyncio
    async def test_cms_natural_language_query_basic(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试基本的自然语言数据查询"""
        tool = mcp_server._tool_manager.get_tool("cms_natural_language_query")
        assert tool is not None

        result = await tool.run(
            arguments={
                "query": "查询服务列表",
            },
            context=mock_request_context,
        )

        # 检查结果结构
        assert result is not None
        assert "data" in result
        assert "message" in result
        assert "request_id" in result
        assert "error" in result
        assert "time_range" in result

        # 如果有凭证问题，跳过进一步验证
        if result.get("error") and "InvalidCredentials" in str(result.get("message", "")):
            pytest.skip("需要有效的阿里云凭证才能运行此测试")

        logger.info(f"CMS Natural Language Query 结果: {result}")

    @pytest.mark.asyncio
    async def test_cms_natural_language_query_top_services(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试查询请求量最高的服务"""
        tool = mcp_server._tool_manager.get_tool("cms_natural_language_query")
        assert tool is not None

        result = await tool.run(
            arguments={
                "query": "查询请求量最高的5个服务",
            },
            context=mock_request_context,
        )

        # 检查结果结构
        assert result is not None
        assert "data" in result
        assert "time_range" in result

        # 如果有凭证问题，跳过进一步验证
        if result.get("error") and "InvalidCredentials" in str(result.get("message", "")):
            pytest.skip("需要有效的阿里云凭证才能运行此测试")

        logger.info(f"CMS Natural Language Query 查询结果: {result}")
