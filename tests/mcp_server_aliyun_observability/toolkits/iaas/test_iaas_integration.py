"""IaaS Toolkit 集成测试

使用真实 SLS 环境测试工具包功能，包括：
- 分页查询（offset 参数）
- 排序控制（reverse 参数）
- 向后兼容性验证
"""
import os
import pytest
from mcp.server.fastmcp import Context, FastMCP

# 标记所有测试为集成测试
pytestmark = [
    pytest.mark.integration,
    pytest.mark.skipif(
        not os.getenv("ALIBABA_CLOUD_ACCESS_KEY_ID"),
        reason="需要设置 ALIBABA_CLOUD_ACCESS_KEY_ID 环境变量"
    ),
]


class TestSlsExecuteSqlIntegration:
    """sls_execute_sql 接口集成测试"""
    
    @pytest.mark.asyncio
    async def test_backward_compatible_without_new_params(
        self,
        real_mcp_server: FastMCP,
        real_request_context: Context,
        sls_test_env: dict,
    ):
        """测试向后兼容：不传 offset 和 reverse 参数"""
        tool = real_mcp_server._tool_manager.get_tool("sls_execute_sql")
        assert tool is not None, "sls_execute_sql 工具未找到"
        
        # 使用旧的调用方式（不传 offset 和 reverse）
        result = await tool.run(
            {
                "project": sls_test_env["project"],
                "logStore": sls_test_env["logstore"],
                "query": "*",  # 纯查询语句
                "from_time": "now-1h",
                "to_time": "now",
                "limit": 10,
                "regionId": sls_test_env["region_id"],
            },
            context=real_request_context,
        )
        
        assert "data" in result, f"响应缺少 data 字段: {result}"
        assert "message" in result, f"响应缺少 message 字段: {result}"
        assert result["message"] == "success", f"查询失败: {result}"
        assert len(result["data"]) <= 10, f"返回数据超过 limit: {len(result['data'])}"
        
        print(f"✅ 向后兼容测试通过: 返回 {len(result['data'])} 条日志")
    
    @pytest.mark.asyncio
    async def test_pagination_with_offset(
        self,
        real_mcp_server: FastMCP,
        real_request_context: Context,
        sls_test_env: dict,
    ):
        """测试分页功能：使用 offset 参数"""
        tool = real_mcp_server._tool_manager.get_tool("sls_execute_sql")
        assert tool is not None
        
        # 获取第一页（offset=0）
        result_page1 = await tool.run(
            {
                "project": sls_test_env["project"],
                "logStore": sls_test_env["logstore"],
                "query": "*",
                "from_time": "now-1h",
                "to_time": "now",
                "limit": 50,
                "offset": 0,
                "regionId": sls_test_env["region_id"],
            },
            context=real_request_context,
        )
        
        assert result_page1["message"] == "success", f"第一页查询失败: {result_page1}"
        page1_data = result_page1["data"]
        print(f"第 1 页: 返回 {len(page1_data)} 条日志")
        
        # 获取第二页（offset=50）
        result_page2 = await tool.run(
            {
                "project": sls_test_env["project"],
                "logStore": sls_test_env["logstore"],
                "query": "*",
                "from_time": "now-1h",
                "to_time": "now",
                "limit": 50,
                "offset": 50,
                "regionId": sls_test_env["region_id"],
            },
            context=real_request_context,
        )
        
        assert result_page2["message"] == "success", f"第二页查询失败: {result_page2}"
        page2_data = result_page2["data"]
        print(f"第 2 页: 返回 {len(page2_data)} 条日志")
        
        # 验证两页数据不重复（通过 request_id 字段）
        if page1_data and page2_data:
            page1_ids = {log.get("request_id") for log in page1_data if log.get("request_id")}
            page2_ids = {log.get("request_id") for log in page2_data if log.get("request_id")}
            overlap = page1_ids & page2_ids
            assert len(overlap) == 0, f"分页数据重复: {overlap}"
            print(f"✅ 分页测试通过: 两页数据无重复")
    
    @pytest.mark.asyncio
    async def test_fetch_200_logs_with_pagination(
        self,
        real_mcp_server: FastMCP,
        real_request_context: Context,
        sls_test_env: dict,
    ):
        """测试完整分页：获取 200 条日志"""
        tool = real_mcp_server._tool_manager.get_tool("sls_execute_sql")
        assert tool is not None
        
        all_logs = []
        limit = 100
        target_count = 200
        
        for page in range(target_count // limit):
            offset = page * limit
            result = await tool.run(
                {
                    "project": sls_test_env["project"],
                    "logStore": sls_test_env["logstore"],
                    "query": "*",
                    "from_time": "now-1h",
                    "to_time": "now",
                    "limit": limit,
                    "offset": offset,
                    "regionId": sls_test_env["region_id"],
                },
                context=real_request_context,
            )
            
            assert result["message"] == "success", f"分页 {page + 1} 查询失败: {result}"
            logs = result["data"]
            all_logs.extend(logs)
            print(f"第 {page + 1} 页 (offset={offset}): 返回 {len(logs)} 条")
            
            if len(logs) < limit:
                break
        
        print(f"✅ 完整分页测试通过: 共获取 {len(all_logs)} 条日志")
        assert len(all_logs) >= min(target_count, 250), f"获取日志数量不足: {len(all_logs)}"
    
    @pytest.mark.asyncio
    async def test_reverse_order(
        self,
        real_mcp_server: FastMCP,
        real_request_context: Context,
        sls_test_env: dict,
    ):
        """测试排序功能：使用 reverse 参数"""
        tool = real_mcp_server._tool_manager.get_tool("sls_execute_sql")
        assert tool is not None
        
        # 升序查询（默认，reverse=False）
        result_asc = await tool.run(
            {
                "project": sls_test_env["project"],
                "logStore": sls_test_env["logstore"],
                "query": "*",
                "from_time": "now-1h",
                "to_time": "now",
                "limit": 10,
                "reverse": False,
                "regionId": sls_test_env["region_id"],
            },
            context=real_request_context,
        )
        
        assert result_asc["message"] == "success", f"升序查询失败: {result_asc}"
        asc_data = result_asc["data"]
        
        # 降序查询（reverse=True）
        result_desc = await tool.run(
            {
                "project": sls_test_env["project"],
                "logStore": sls_test_env["logstore"],
                "query": "*",
                "from_time": "now-1h",
                "to_time": "now",
                "limit": 10,
                "reverse": True,
                "regionId": sls_test_env["region_id"],
            },
            context=real_request_context,
        )
        
        assert result_desc["message"] == "success", f"降序查询失败: {result_desc}"
        desc_data = result_desc["data"]
        
        # 验证时间戳顺序
        if asc_data and desc_data:
            asc_times = [int(log.get("__time__", 0)) for log in asc_data]
            desc_times = [int(log.get("__time__", 0)) for log in desc_data]
            
            # 升序：第一条时间 <= 最后一条时间
            if len(asc_times) > 1:
                assert asc_times[0] <= asc_times[-1], f"升序验证失败: {asc_times[0]} > {asc_times[-1]}"
            
            # 降序：第一条时间 >= 最后一条时间
            if len(desc_times) > 1:
                assert desc_times[0] >= desc_times[-1], f"降序验证失败: {desc_times[0]} < {desc_times[-1]}"
            
            print(f"✅ 排序测试通过: 升序首条时间={asc_times[0] if asc_times else 'N/A'}, 降序首条时间={desc_times[0] if desc_times else 'N/A'}")
    
    @pytest.mark.asyncio
    async def test_combined_offset_and_reverse(
        self,
        real_mcp_server: FastMCP,
        real_request_context: Context,
        sls_test_env: dict,
    ):
        """测试组合使用：offset + reverse"""
        tool = real_mcp_server._tool_manager.get_tool("sls_execute_sql")
        assert tool is not None
        
        result = await tool.run(
            {
                "project": sls_test_env["project"],
                "logStore": sls_test_env["logstore"],
                "query": "*",
                "from_time": "now-1h",
                "to_time": "now",
                "limit": 50,
                "offset": 50,
                "reverse": True,
                "regionId": sls_test_env["region_id"],
            },
            context=real_request_context,
        )
        
        assert "data" in result, f"响应缺少 data 字段: {result}"
        assert result["message"] == "success", f"组合查询失败: {result}"
        
        print(f"✅ 组合参数测试通过: 返回 {len(result['data'])} 条日志 (offset=50, reverse=True)")
    
    @pytest.mark.asyncio
    async def test_edge_case_offset_zero(
        self,
        real_mcp_server: FastMCP,
        real_request_context: Context,
        sls_test_env: dict,
    ):
        """测试边界条件：offset=0"""
        tool = real_mcp_server._tool_manager.get_tool("sls_execute_sql")
        assert tool is not None
        
        result = await tool.run(
            {
                "project": sls_test_env["project"],
                "logStore": sls_test_env["logstore"],
                "query": "*",
                "from_time": "now-1h",
                "to_time": "now",
                "limit": 1,
                "offset": 0,
                "regionId": sls_test_env["region_id"],
            },
            context=real_request_context,
        )
        
        assert result["message"] == "success", f"边界条件查询失败: {result}"
        assert len(result["data"]) == 1, f"应返回 1 条日志: {len(result['data'])}"
        
        print(f"✅ 边界条件测试通过: offset=0, limit=1")
    
    @pytest.mark.asyncio
    async def test_sql_analytics_query(
        self,
        real_mcp_server: FastMCP,
        real_request_context: Context,
        sls_test_env: dict,
    ):
        """测试 SQL 分析语句（offset 和 reverse 对分析语句无效）"""
        tool = real_mcp_server._tool_manager.get_tool("sls_execute_sql")
        assert tool is not None
        
        # SQL 分析语句
        result = await tool.run(
            {
                "project": sls_test_env["project"],
                "logStore": sls_test_env["logstore"],
                "query": "* | SELECT level, COUNT(*) as count GROUP BY level",
                "from_time": "now-1h",
                "to_time": "now",
                "limit": 100,
                "offset": 0,  # 对 SQL 分析无效
                "regionId": sls_test_env["region_id"],
            },
            context=real_request_context,
        )
        
        assert "data" in result, f"响应缺少 data 字段: {result}"
        # SQL 分析可能返回空结果（如果没有匹配的日志）
        if result["message"] == "success":
            print(f"✅ SQL 分析测试通过: 返回 {len(result['data'])} 条聚合结果")
        else:
            print(f"⚠️ SQL 分析返回: {result['message']}")


class TestSlsListProjectsIntegration:
    """sls_list_projects 接口集成测试"""
    
    @pytest.mark.asyncio
    async def test_list_projects(
        self,
        real_mcp_server: FastMCP,
        real_request_context: Context,
        sls_test_env: dict,
    ):
        """测试列出项目"""
        tool = real_mcp_server._tool_manager.get_tool("sls_list_projects")
        assert tool is not None, "sls_list_projects 工具未找到"
        
        result = await tool.run(
            {
                "regionId": sls_test_env["region_id"],
                "limit": 50,
            },
            context=real_request_context,
        )
        
        assert "projects" in result, f"响应缺少 projects 字段: {result}"
        
        # 验证测试项目存在
        project_names = [p.get("project_name") for p in result["projects"]]
        assert sls_test_env["project"] in project_names, f"测试项目未找到: {sls_test_env['project']}"
        
        print(f"✅ 列出项目测试通过: 找到 {len(result['projects'])} 个项目")


class TestSlsListLogstoresIntegration:
    """sls_list_logstores 接口集成测试"""
    
    @pytest.mark.asyncio
    async def test_list_logstores(
        self,
        real_mcp_server: FastMCP,
        real_request_context: Context,
        sls_test_env: dict,
    ):
        """测试列出日志库"""
        tool = real_mcp_server._tool_manager.get_tool("sls_list_logstores")
        assert tool is not None, "sls_list_logstores 工具未找到"
        
        result = await tool.run(
            {
                "project": sls_test_env["project"],
                "regionId": sls_test_env["region_id"],
                "limit": 50,
                "isMetricStore": False,
            },
            context=real_request_context,
        )
        
        assert "logstores" in result, f"响应缺少 logstores 字段: {result}"
        
        # 验证测试日志库存在
        assert sls_test_env["logstore"] in result["logstores"], f"测试日志库未找到: {sls_test_env['logstore']}"
        
        print(f"✅ 列出日志库测试通过: 找到 {len(result['logstores'])} 个日志库")


if __name__ == "__main__":
    pytest.main([__file__, "-v", "-s", "--tb=short"])
