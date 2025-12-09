"""IaaS Toolkit 测试用例

验证IaaS层工具包的功能，包括：
- SQL查询执行
- PromQL查询执行  
- 项目和日志库列表
- 时间参数处理
"""
import os
from unittest.mock import AsyncMock, MagicMock, patch

import dotenv
import pytest
from mcp.server.fastmcp import Context, FastMCP
from mcp.shared.context import RequestContext

from mcp_server_aliyun_observability.server import create_lifespan
from mcp_server_aliyun_observability.toolkits.iaas.toolkit import register_iaas_tools
from mcp_server_aliyun_observability.utils import SLSClientWrapper, CredentialWrapper


dotenv.load_dotenv()

# 测试常量
TEST_PROJECT = "test-project"
TEST_LOGSTORE = "test-logstore"
TEST_REGION = "cn-hangzhou"
TEST_SQL_QUERY = "SELECT * FROM test_table LIMIT 10"
TEST_PROMQL_QUERY = "up{job='prometheus'}"

# log explore and log compare test data
LOG_EXPLORE_PROJECT = "sls-ml-spl-shanghai-cloudspe"
LOG_EXPLORE_LOGSTORE = "massive-hdfs-log"
LOG_EXPLORE_REGION = "cn-shanghai"

@pytest.fixture
def mock_request_context():
    # """模拟请求上下文"""
    # context = MagicMock(spec=Context)
    
    # # 模拟SLS客户端
    # mock_sls_client = MagicMock()
    # mock_sls_client.with_region.return_value = mock_sls_client
    
    # # 模拟查询响应
    # mock_response = MagicMock()
    # mock_response.body = [
    #     {"timestamp": "2024-01-01T00:00:00Z", "level": "info", "message": "test log"},
    #     {"timestamp": "2024-01-01T00:01:00Z", "level": "error", "message": "error log"}
    # ]
    # mock_sls_client.get_logs_with_options.return_value = mock_response
    
    # # 模拟项目列表响应
    # mock_project_response = MagicMock()
    # mock_project_response.body.projects = [
    #     MagicMock(project_name="project1", description="Test Project 1", region="cn-hangzhou"),
    #     MagicMock(project_name="project2", description="Test Project 2", region="cn-hangzhou")
    # ]
    # mock_sls_client.list_project.return_value = mock_project_response
    
    # # 模拟日志库列表响应
    # mock_logstore_response = MagicMock()
    # mock_logstore_response.body.total = 2
    # mock_logstore_response.body.logstores = ["logstore1", "logstore2"]
    # mock_sls_client.list_log_stores.return_value = mock_logstore_response
    
    # context.request_context.lifespan_context = {
    #     "sls_client": mock_sls_client
    # }
    
    # return context

    return Context(
        request_context=RequestContext(
            request_id="test_log_explore_request_id",
            meta=None,
            session=None,
            lifespan_context={
                "cms_client": None,
                "sls_client": SLSClientWrapper(
                    credential=CredentialWrapper(
                        access_key_id=os.getenv("ALIBABA_CLOUD_ACCESS_KEY_ID"),
                        access_key_secret=os.getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET"),
                        knowledge_config=None,
                    ),
                ),
                "arms_client": None,
            },
        )
    )


@pytest.fixture
def mcp_server():
    """创建带有IaaS工具的FastMCP服务器"""
    server = FastMCP(
        "test-iaas-server",
        lifespan=create_lifespan(
            credential=CredentialWrapper(
                access_key_id=os.getenv("ALIYUN_ACCESS_KEY_ID"),
                access_key_secret=os.getenv("ALIYUN_ACCESS_KEY_SECRET"),
                knowledge_config=None,
            ),
        ),
    )
    register_iaas_tools(server)
    return server


class TestIaaSToolkit:
    """IaaS工具包测试类"""
    
    @pytest.mark.asyncio
    async def test_execute_sql_with_relative_time(self, mcp_server: FastMCP, mock_request_context: Context):
        """测试SQL执行功能 - 相对时间参数"""
        tool = mcp_server._tool_manager.get_tool("sls_execute_sql")
        assert tool is not None, "sls_execute_sql 工具未找到"
        
        result = await tool.run(
            {
                "project": TEST_PROJECT,
                "logStore": TEST_LOGSTORE,
                "query": TEST_SQL_QUERY,
                "from_time": "now-1h",
                "to_time": "now",
                "limit": 10,
                "regionId": TEST_REGION,
            },
            context=mock_request_context,
        )
        
        assert "data" in result
        assert "message" in result
        assert len(result["data"]) == 2
        assert result["data"][0]["message"] == "test log"
        
    @pytest.mark.asyncio
    async def test_execute_sql_with_absolute_time(self, mcp_server: FastMCP, mock_request_context: Context):
        """测试SQL执行功能 - 绝对时间戳"""
        tool = mcp_server._tool_manager.get_tool("sls_execute_sql")
        assert tool is not None, "sls_execute_sql 工具未找到"
        
        result = await tool.run(
            {
                "project": TEST_PROJECT,
                "logStore": TEST_LOGSTORE,
                "query": TEST_SQL_QUERY,
                "from_time": 1704067200,  # 2024-01-01 00:00:00 UTC
                "to_time": 1704070800,    # 2024-01-01 01:00:00 UTC
                "limit": 5,
                "regionId": TEST_REGION,
            },
            context=mock_request_context,
        )
        
        assert "data" in result
        assert "message" in result
        assert len(result["data"]) == 2
    
    @pytest.mark.asyncio
    async def test_execute_promql_with_time_params(self, mcp_server: FastMCP, mock_request_context: Context):
        """测试PromQL执行功能"""
        tool = mcp_server._tool_manager.get_tool("sls_execute_promql")
        assert tool is not None, "sls_execute_promql 工具未找到"
        
        result = await tool.run(
            {
                "project": TEST_PROJECT,
                "metricStore": TEST_LOGSTORE,
                "query": TEST_PROMQL_QUERY,
                "from_time": "now-30m",
                "to_time": "now",
                "regionId": TEST_REGION,
            },
            context=mock_request_context,
        )
        
        assert "data" in result
        assert "message" in result
        
    @pytest.mark.asyncio
    async def test_list_projects_no_domain_params(self, mcp_server: FastMCP, mock_request_context: Context):
        """测试项目列表功能 - 验证不需要domain参数"""
        tool = mcp_server._tool_manager.get_tool("sls_list_projects")
        assert tool is not None, "sls_list_projects 工具未找到"
        
        # 测试不传递domain参数
        result = await tool.run(
            {
                "regionId": TEST_REGION,
                "limit": 50,
            },
            context=mock_request_context,
        )
        
        assert "projects" in result
        assert "message" in result
        assert "entity_context" not in result  # 验证没有entity_context
        assert len(result["projects"]) == 2
        assert result["projects"][0]["project_name"] == "project1"
        
    @pytest.mark.asyncio
    async def test_list_logstores_no_domain_params(self, mcp_server: FastMCP, mock_request_context: Context):
        """测试日志库列表功能 - 验证不需要domain参数"""
        tool = mcp_server._tool_manager.get_tool("sls_list_logstores")
        assert tool is not None, "sls_list_logstores 工具未找到"
        
        # 测试不传递domain参数
        result = await tool.run(
            {
                "project": TEST_PROJECT,
                "regionId": TEST_REGION,
                "limit": 10,
                "isMetricStore": False,
            },
            context=mock_request_context,
        )
        
        assert "total" in result
        assert "logstores" in result
        assert "message" in result
        assert "entity_context" not in result  # 验证没有entity_context
        assert result["total"] == 2
        assert len(result["logstores"]) == 2

    @pytest.mark.asyncio
    async def test_log_explore(self, mcp_server: FastMCP, mock_request_context: Context):
        """测试日志探索"""
        tool = mcp_server._tool_manager.get_tool("sls_log_explore")
        assert tool is not None, "sls_log_explore 工具未找到"
        
        result = await tool.run(
            {
                "project": LOG_EXPLORE_PROJECT,
                "logStore": LOG_EXPLORE_LOGSTORE,
                "regionId": LOG_EXPLORE_REGION,
                "logField": "Content",
                "from_time": 1738302900,
                "to_time": 1738313880,
                "groupField": "Level"
            },
            context=mock_request_context
        )

        assert "patterns" in result
        assert "message" in result and result["message"] == "success"
        assert len(result["patterns"]) > 0
        for pattern in result["patterns"]:
            assert "pattern" in pattern
            assert "pattern_regexp" in pattern
            assert "event_num" in pattern
            assert "group" in pattern
            assert "histogram" in pattern
            assert "variables" in pattern
            for histogram in pattern["histogram"]:
                assert "from_timestamp" in histogram
                assert "to_timestamp" in histogram
                assert "count" in histogram
            for variable in pattern["variables"]:
                assert "index" in variable
                assert "type" in variable
                assert "format" in variable
                assert "candidates" in variable
        print (f"len(result['patterns']): {len(result['patterns'])}")
        print (result["patterns"][2])

    @pytest.mark.asyncio
    async def test_log_compare(self, mcp_server: FastMCP, mock_request_context: Context):
        """测试日志比较"""
        tool = mcp_server._tool_manager.get_tool("sls_log_compare")
        assert tool is not None, "sls_log_compare 工具未找到"
        
        result = await tool.run(
            {
                "project": LOG_EXPLORE_PROJECT,
                "logStore": LOG_EXPLORE_LOGSTORE,
                "regionId": LOG_EXPLORE_REGION,
                "logField": "Content",
                "test_from_time": 1738302900,
                "test_to_time": 1738313880,
                "control_from_time": 1738317300,
                "control_to_time": 1738328280,
                "groupField": "Level",
            },
            context=mock_request_context
        )

        assert "patterns" in result
        assert "message" in result and result["message"] == "success"
        assert len(result["patterns"]) > 0
        for pattern in result["patterns"]:
            assert "pattern" in pattern
            assert "pattern_regexp" in pattern
            assert "test_event_num" in pattern
            assert "control_event_num" in pattern
            assert "group" in pattern
            assert "test_variables" in pattern
            assert "control_variables" in pattern
            for variable in pattern["test_variables"]:
                assert "index" in variable
                assert "type" in variable
                assert "format" in variable
                assert "candidates" in variable
            for variable in pattern["control_variables"]:
                assert "index" in variable
                assert "type" in variable
                assert "format" in variable
                assert "candidates" in variable
        print (f"len(result['patterns']): {len(result['patterns'])}")
        print (result["patterns"][2])

if __name__ == "__main__":
    pytest.main([__file__, "-s", "-v", "-k" "test_log_explore"])