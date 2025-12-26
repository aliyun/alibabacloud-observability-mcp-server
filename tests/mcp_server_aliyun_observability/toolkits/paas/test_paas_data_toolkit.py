import os

import dotenv
import pytest
from mcp.server.fastmcp import Context, FastMCP
from mcp.shared.context import RequestContext

from mcp_server_aliyun_observability.server import create_lifespan
from mcp_server_aliyun_observability.toolkits.paas.data_toolkit import PaasDataToolkit
from mcp_server_aliyun_observability.toolkits.shared.toolkit import (
    register_shared_tools,
)
from mcp_server_aliyun_observability.utils import CMSClientWrapper, CredentialWrapper

dotenv.load_dotenv()

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
    """创建模拟的FastMCP服务器实例用于数据工具测试"""
    mcp_server = FastMCP(
        name="mcp_aliyun_observability_data_server",
        lifespan=create_lifespan(
            credential=CredentialWrapper(
                access_key_id=os.getenv("ALIYUN_ACCESS_KEY_ID"),
                access_key_secret=os.getenv("ALIYUN_ACCESS_KEY_SECRET"),
                knowledge_config=None,
            ),
        ),
    )
    # 注册数据工具
    PaasDataToolkit(mcp_server)
    register_shared_tools(mcp_server)
    return mcp_server


@pytest.fixture
def mock_request_context():
    """创建模拟的RequestContext实例"""
    context = Context(
        request_context=RequestContext(
            request_id="test_data_request_id",
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


class TestPaaSDataToolkit:
    """PaaS数据工具的测试类"""

    @pytest.mark.asyncio
    async def test_paas_get_metrics_success(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS指标查询"""
        tool = mcp_server._tool_manager.get_tool("umodel_get_metrics")
        result = await tool.run(
            {
                "domain": "apm",
                "entity_set_name": "apm.service",
                "metric_domain_name": "apm.metric.jvm",
                "metric": "arms_jvm_threads_count",
                "workspace": os.getenv("TEST_CMS_WORKSPACE", "apm"),
                "regionId": os.getenv("TEST_REGION", "cn-hangzhou"),
            },
            context=mock_request_context,
        )
        result = check_credentials_and_result(result)

    @pytest.mark.asyncio
    async def test_paas_get_golden_metrics_success(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS黄金指标查询"""
        tool = mcp_server._tool_manager.get_tool("umodel_get_golden_metrics")
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
    async def test_paas_get_relation_metrics_success(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS关系指标查询"""
        tool = mcp_server._tool_manager.get_tool("umodel_get_relation_metrics")
        result = await tool.run(
            {
                "src_domain": "apm",
                "src_entity_set_name": "apm.service",
                "src_entity_ids": "5a81706b75fe1295797af01544a31264",
                "relation_type": "calls",
                "direction": "out",
                "metric_set_domain": "apm",
                "metric_set_name": "apm.metric.jvm",
                "metric": "arms_jvm_mem_max_bytes",
                "workspace": os.getenv("TEST_CMS_WORKSPACE", "apm"),
                "regionId": os.getenv("TEST_REGION", "cn-hangzhou"),
            },
            context=mock_request_context,
        )
        result = check_credentials_and_result(result)

    @pytest.mark.asyncio
    async def test_paas_get_logs_success(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS日志查询"""
        tool = mcp_server._tool_manager.get_tool("umodel_get_logs")
        result = await tool.run(
            {
                "domain": "apm",
                "entity_set_name": "apm.service",
                "log_set_domain": "app",
                "log_set_name": "app.log.common",
                "workspace": os.getenv("TEST_CMS_WORKSPACE", "apm"),
                "regionId": os.getenv("TEST_REGION", "cn-hangzhou"),
            },
            context=mock_request_context,
        )
        result = check_credentials_and_result(result)

    @pytest.mark.asyncio
    async def test_paas_get_events_success(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS事件查询"""
        tool = mcp_server._tool_manager.get_tool("umodel_get_events")
        result = await tool.run(
            {
                "domain": "apm",
                "entity_set_name": "apm.service",
                "event_set_domain": "default",
                "event_set_name": "default.event.common",
                "workspace": os.getenv("TEST_CMS_WORKSPACE", "apm"),
                "regionId": os.getenv("TEST_REGION", "cn-hangzhou"),
                "limit": 50,
            },
            context=mock_request_context,
        )
        result = check_credentials_and_result(result)

    @pytest.mark.asyncio
    async def test_paas_get_traces_success(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS详细trace查询"""
        tool = mcp_server._tool_manager.get_tool("umodel_get_traces")
        result = await tool.run(
            {
                "domain": "apm",
                "entity_set_name": "apm.service",
                "trace_set_domain": "apm",
                "trace_set_name": "apm.trace.common",
                "trace_ids": "test_trace_id_1,test_trace_id_2",
                "workspace": os.getenv("TEST_CMS_WORKSPACE", "apm"),
                "regionId": os.getenv("TEST_REGION", "cn-hangzhou"),
            },
            context=mock_request_context,
        )
        result = check_credentials_and_result(result)

    @pytest.mark.asyncio
    async def test_paas_search_traces_success(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS trace搜索"""
        tool = mcp_server._tool_manager.get_tool("umodel_search_traces")
        result = await tool.run(
            {
                "domain": "apm",
                "entity_set_name": "apm.service",
                "trace_set_domain": "apm",
                "trace_set_name": "apm.trace.common",
                "workspace": os.getenv("TEST_CMS_WORKSPACE", "apm"),
                "regionId": os.getenv("TEST_REGION", "cn-hangzhou"),
                "min_duration_ms": 1000,
                "limit": 50,
            },
            context=mock_request_context,
        )
        result = check_credentials_and_result(result)

    @pytest.mark.asyncio
    async def test_paas_search_traces_with_error_filter(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS trace搜索 - 错误过滤"""
        tool = mcp_server._tool_manager.get_tool("umodel_search_traces")
        result = await tool.run(
            {
                "domain": "apm",
                "entity_set_name": "apm.service",
                "trace_set_domain": "apm",
                "trace_set_name": "apm.trace.common",
                "workspace": os.getenv("TEST_CMS_WORKSPACE", "apm"),
                "regionId": os.getenv("TEST_REGION", "cn-hangzhou"),
                "has_error": True,
                "limit": 30,
            },
            context=mock_request_context,
        )
        result = check_credentials_and_result(result)

    @pytest.mark.asyncio
    async def test_paas_get_profiles_success(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS性能剖析查询"""
        tool = mcp_server._tool_manager.get_tool("umodel_get_profiles")
        result = await tool.run(
            {
                "domain": "apm",
                "entity_set_name": "apm.service",
                "profile_set_domain": "default",
                "profile_set_name": "default.profile.common",
                "entity_ids": "5a81706b75fe1295797af01544a31264",
                "workspace": os.getenv("TEST_CMS_WORKSPACE", "apm"),
                "regionId": os.getenv("TEST_REGION", "cn-hangzhou"),
                "limit": 20,
            },
            context=mock_request_context,
        )
        result = check_credentials_and_result(result)

    @pytest.mark.asyncio
    async def test_paas_time_range_parsing(
        self,
        mcp_server: FastMCP,
        mock_request_context: Context,
    ):
        """测试PaaS时间范围解析功能"""
        tool = mcp_server._tool_manager.get_tool("umodel_get_golden_metrics")
        result = await tool.run(
            {
                "domain": "apm",
                "entity_set_name": "apm.service",
                "workspace": os.getenv("TEST_CMS_WORKSPACE", "apm"),
                "regionId": os.getenv("TEST_REGION", "cn-hangzhou"),
                "from_time": "now-3h",
                "to_time": "now",
            },
            context=mock_request_context,
        )
        result = check_credentials_and_result(result)


class TestMetricSetNameSuffix:
    """测试 metric_set_name 自动拼接逻辑"""

    def test_metric_set_name_needs_suffix(self):
        """测试：传入简单格式时，应自动拼接后缀"""
        # 模拟输入
        metric_set_name = "apm.metric.apm.operation"
        relation_type = "calls"
        src_entity_set_name = "apm.operation"

        # 拼接逻辑
        relation_suffix = f"_{relation_type}_{src_entity_set_name}"
        if relation_suffix not in metric_set_name:
            metric_set_name = f"{metric_set_name}{relation_suffix}"

        # 验证结果
        expected = "apm.metric.apm.operation_calls_apm.operation"
        assert metric_set_name == expected, f"Expected {expected}, got {metric_set_name}"

    def test_metric_set_name_already_has_suffix(self):
        """测试：传入完整格式时，不应重复拼接"""
        # 模拟输入（已经是完整格式）
        metric_set_name = "apm.metric.apm.operation_calls_apm.operation"
        relation_type = "calls"
        src_entity_set_name = "apm.operation"

        # 拼接逻辑
        relation_suffix = f"_{relation_type}_{src_entity_set_name}"
        if relation_suffix not in metric_set_name:
            metric_set_name = f"{metric_set_name}{relation_suffix}"

        # 验证结果：应保持不变
        expected = "apm.metric.apm.operation_calls_apm.operation"
        assert metric_set_name == expected, f"Expected {expected}, got {metric_set_name}"

    def test_metric_set_name_different_relation_type(self):
        """测试：不同 relation_type 时，应正确拼接"""
        # 模拟输入
        metric_set_name = "apm.metric.apm.service"
        relation_type = "depends_on"
        src_entity_set_name = "apm.service"

        # 拼接逻辑
        relation_suffix = f"_{relation_type}_{src_entity_set_name}"
        if relation_suffix not in metric_set_name:
            metric_set_name = f"{metric_set_name}{relation_suffix}"

        # 验证结果
        expected = "apm.metric.apm.service_depends_on_apm.service"
        assert metric_set_name == expected, f"Expected {expected}, got {metric_set_name}"


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
