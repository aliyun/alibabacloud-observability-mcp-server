"""PaaS Entity Toolkit 辅助方法单元测试

测试 PaaSEntityToolkit 中的 _parse_time_range, _build_standard_response, 
_validate_required_params, _build_entity_filter_param 等辅助方法。
这些测试不需要真实的阿里云凭证，属于单元测试。

Requirements: 1.1, 3.1, 4.2
"""
import time
from unittest.mock import MagicMock, patch

import pytest
from mcp.server.fastmcp import FastMCP

from mcp_server_aliyun_observability.toolkits.paas.entity_toolkit import PaaSEntityToolkit


# ============================================================================
# Fixtures
# ============================================================================

@pytest.fixture
def mock_server():
    """创建模拟的 FastMCP 服务器实例"""
    server = MagicMock(spec=FastMCP)
    server.tool = MagicMock(return_value=lambda f: f)
    return server


@pytest.fixture
def toolkit(mock_server):
    """创建 PaaSEntityToolkit 实例用于测试辅助方法"""
    with patch.object(PaaSEntityToolkit, '_register_tools'):
        toolkit = PaaSEntityToolkit(mock_server)
    return toolkit


# ============================================================================
# _parse_time_range 测试
# ============================================================================

class TestParseTimeRange:
    """测试 _parse_time_range 方法"""

    def test_none_input_defaults_to_last_1h(self, toolkit):
        """测试 None 输入默认使用 last_1h"""
        from_ts, to_ts = toolkit._parse_time_range(None)
        
        # 验证返回的是有效的时间戳
        assert isinstance(from_ts, int)
        assert isinstance(to_ts, int)
        assert from_ts < to_ts
        
        # 验证时间范围约为 1 小时（允许几秒误差）
        duration = to_ts - from_ts
        assert 3590 <= duration <= 3610  # 约 1 小时

    def test_empty_string_defaults_to_last_1h(self, toolkit):
        """测试空字符串输入默认使用 last_1h"""
        from_ts, to_ts = toolkit._parse_time_range("")
        
        # 验证返回的是有效的时间戳
        assert isinstance(from_ts, int)
        assert isinstance(to_ts, int)
        assert from_ts < to_ts
        
        # 验证时间范围约为 1 小时
        duration = to_ts - from_ts
        assert 3590 <= duration <= 3610

    def test_valid_relative_preset_last_5m(self, toolkit):
        """测试有效的相对预设 last_5m"""
        from_ts, to_ts = toolkit._parse_time_range("last_5m")
        
        assert isinstance(from_ts, int)
        assert isinstance(to_ts, int)
        assert from_ts < to_ts
        
        # 验证时间范围约为 5 分钟
        duration = to_ts - from_ts
        assert 290 <= duration <= 310

    def test_grafana_style_now_minus_15m_to_now_minus_5m(self, toolkit):
        """测试 Grafana 风格 now-15m~now-5m"""
        from_ts, to_ts = toolkit._parse_time_range("now-15m~now-5m")
        
        assert isinstance(from_ts, int)
        assert isinstance(to_ts, int)
        assert from_ts < to_ts
        
        # 验证时间范围约为 10 分钟
        duration = to_ts - from_ts
        assert 590 <= duration <= 610

    def test_invalid_expression_raises_valueerror(self, toolkit):
        """测试无效表达式抛出 ValueError 并包含建议"""
        with pytest.raises(ValueError) as exc_info:
            toolkit._parse_time_range("invalid_time_expression")
        
        error_message = str(exc_info.value)
        assert "无效的时间表达式" in error_message
        # 验证错误消息包含建议的格式
        assert "last_5m" in error_message or "支持格式" in error_message


# ============================================================================
# _build_standard_response 测试
# ============================================================================

class TestBuildStandardResponse:
    """测试 _build_standard_response 方法"""

    def test_response_contains_all_required_fields(self, toolkit):
        """测试响应包含所有必需字段: error, data, message, query, time_range"""
        response = toolkit._build_standard_response(
            data=[{"entity_id": "service-1", "name": "payment"}],
            query=".entity_set with(domain='apm') | entity-call get_entities()",
            time_range=(1706864400, 1706868000),
            message="Query executed successfully",
            time_range_expression="last_1h"
        )
        
        # 验证所有必需字段存在
        assert "error" in response
        assert "data" in response
        assert "message" in response
        assert "query" in response
        assert "time_range" in response

    def test_time_range_contains_all_required_fields(self, toolkit):
        """测试 time_range 包含所有必需字段: from, to, from_readable, to_readable, expression"""
        response = toolkit._build_standard_response(
            data=[],
            query="test query",
            time_range=(1706864400, 1706868000),
            time_range_expression="last_1h"
        )
        
        time_range = response["time_range"]
        assert "from" in time_range
        assert "to" in time_range
        assert "from_readable" in time_range
        assert "to_readable" in time_range
        assert "expression" in time_range

    def test_default_message_for_successful_query_with_data(self, toolkit):
        """测试成功查询有数据时的默认消息"""
        response = toolkit._build_standard_response(
            data=[{"entity_id": "service-1"}],
            query="test query",
            time_range=(1706864400, 1706868000)
        )
        
        assert response["error"] is False
        assert response["message"] == "Query executed successfully"

    def test_default_message_for_successful_query_with_no_data(self, toolkit):
        """测试成功查询无数据时的默认消息 'No data found'"""
        response = toolkit._build_standard_response(
            data=[],
            query="test query",
            time_range=(1706864400, 1706868000)
        )
        
        assert response["error"] is False
        assert response["message"] == "No data found"


# ============================================================================
# _validate_required_params 测试
# ============================================================================

class TestValidateRequiredParams:
    """测试 _validate_required_params 方法"""

    def test_valid_parameters_pass_validation(self, toolkit):
        """测试有效参数通过验证"""
        # 不应抛出异常
        toolkit._validate_required_params(
            {"domain": "apm", "entity_set_name": "apm.service"},
            ["domain", "entity_set_name"]
        )

    def test_domain_wildcard_raises_valueerror(self, toolkit):
        """测试 domain = '*' 抛出 ValueError 并包含正确消息"""
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(
                {"domain": "*", "entity_set_name": "apm.service"},
                ["domain", "entity_set_name"]
            )
        
        error_message = str(exc_info.value)
        assert "domain" in error_message
        assert "*" in error_message or "不能为" in error_message
        assert "umodel_search_entity_set" in error_message

    def test_entity_set_name_wildcard_raises_valueerror(self, toolkit):
        """测试 entity_set_name = '*' 抛出 ValueError 并包含正确消息"""
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(
                {"domain": "apm", "entity_set_name": "*"},
                ["domain", "entity_set_name"]
            )
        
        error_message = str(exc_info.value)
        assert "entity_set_name" in error_message
        assert "*" in error_message or "不能为" in error_message
        assert "umodel_search_entity_set" in error_message

    def test_missing_required_parameter_raises_valueerror(self, toolkit):
        """测试缺少必填参数抛出 ValueError"""
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(
                {"domain": "apm"},
                ["domain", "entity_set_name"]
            )
        
        error_message = str(exc_info.value)
        assert "entity_set_name" in error_message
        assert "缺少必填参数" in error_message


# ============================================================================
# _build_entity_filter_param 测试
# ============================================================================

class TestBuildEntityFilterParam:
    """测试 _build_entity_filter_param 方法

    Requirements: 4.2
    """

    def test_none_input_returns_empty_string(self, toolkit):
        """测试 None 输入返回空字符串"""
        result = toolkit._build_entity_filter_param(None)
        assert result == ""

    def test_empty_string_returns_empty_string(self, toolkit):
        """测试空字符串输入返回空字符串"""
        result = toolkit._build_entity_filter_param("")
        assert result == ""

    def test_simple_equals_expression(self, toolkit):
        """测试简单等于表达式 name=payment"""
        result = toolkit._build_entity_filter_param("name=payment")
        assert result == ', query=`"name"=\'payment\'`'

    def test_simple_not_equals_expression(self, toolkit):
        """测试简单不等于表达式 status!=inactive"""
        result = toolkit._build_entity_filter_param("status!=inactive")
        assert result == ', query=`"status"!=\'inactive\'`'

    def test_multiple_conditions_with_and(self, toolkit):
        """测试多条件 and 连接 name=payment and status!=inactive"""
        result = toolkit._build_entity_filter_param("name=payment and status!=inactive")
        assert result == ', query=`"name"=\'payment\' and "status"!=\'inactive\'`'


# ============================================================================
# _build_entity_ids_param 测试
# ============================================================================

class TestBuildEntityIdsParam:
    """测试 _build_entity_ids_param 方法"""

    def test_none_input_returns_empty_string(self, toolkit):
        """测试 None 输入返回空字符串"""
        result = toolkit._build_entity_ids_param(None)
        assert result == ""

    def test_empty_string_returns_empty_string(self, toolkit):
        """测试空字符串输入返回空字符串"""
        result = toolkit._build_entity_ids_param("")
        assert result == ""

    def test_single_id(self, toolkit):
        """测试单个 ID"""
        result = toolkit._build_entity_ids_param("service-1")
        assert result == ", ids=['service-1']"

    def test_multiple_ids(self, toolkit):
        """测试多个 ID"""
        result = toolkit._build_entity_ids_param("service-1,service-2,service-3")
        assert result == ", ids=['service-1','service-2','service-3']"

    def test_ids_with_whitespace(self, toolkit):
        """测试带空白的 ID 列表"""
        result = toolkit._build_entity_ids_param("  service-1  ,  service-2  ")
        assert result == ", ids=['service-1','service-2']"

    def test_ids_with_empty_parts(self, toolkit):
        """测试包含空部分的 ID 列表"""
        result = toolkit._build_entity_ids_param("service-1,,service-2")
        assert result == ", ids=['service-1','service-2']"


# ============================================================================
# 运行测试
# ============================================================================

if __name__ == "__main__":
    pytest.main([__file__, "-v"])


# ============================================================================
# _parse_entity_ids_to_spl_param 测试
# ============================================================================

class TestParseEntityIdsToSplParam:
    """测试 _parse_entity_ids_to_spl_param 方法"""

    def test_single_id(self, toolkit):
        """测试单个 ID 转换为 SPL 数组格式"""
        result = toolkit._parse_entity_ids_to_spl_param("service-1")
        assert result == "['service-1']"

    def test_multiple_ids(self, toolkit):
        """测试多个 ID 转换为 SPL 数组格式"""
        result = toolkit._parse_entity_ids_to_spl_param("svc-1,svc-2,svc-3")
        assert result == "['svc-1','svc-2','svc-3']"

    def test_ids_with_whitespace(self, toolkit):
        """测试带空白的 ID 列表"""
        result = toolkit._parse_entity_ids_to_spl_param("  svc-1  ,  svc-2  ")
        assert result == "['svc-1','svc-2']"


# ============================================================================
# _parse_string_to_spl_param 测试
# ============================================================================

class TestParseStringToSplParam:
    """测试 _parse_string_to_spl_param 方法"""

    def test_none_returns_empty_quotes(self, toolkit):
        """测试 None 返回空引号"""
        result = toolkit._parse_string_to_spl_param(None)
        assert result == "''"

    def test_empty_string_returns_empty_quotes(self, toolkit):
        """测试空字符串返回空引号"""
        result = toolkit._parse_string_to_spl_param("")
        assert result == "''"

    def test_valid_string(self, toolkit):
        """测试有效字符串"""
        result = toolkit._parse_string_to_spl_param("apm.service")
        assert result == "'apm.service'"

    def test_string_with_whitespace(self, toolkit):
        """测试带空白的字符串"""
        result = toolkit._parse_string_to_spl_param("  apm.service  ")
        assert result == "'apm.service'"


# ============================================================================
# _parse_direction_to_spl_param 测试
# ============================================================================

class TestParseDirectionToSplParam:
    """测试 _parse_direction_to_spl_param 方法"""

    def test_none_returns_both(self, toolkit):
        """测试 None 返回默认值 'both'"""
        result = toolkit._parse_direction_to_spl_param(None)
        assert result == "'both'"

    def test_empty_string_returns_both(self, toolkit):
        """测试空字符串返回默认值 'both'"""
        result = toolkit._parse_direction_to_spl_param("")
        assert result == "'both'"

    def test_direction_in(self, toolkit):
        """测试 direction='in'"""
        result = toolkit._parse_direction_to_spl_param("in")
        assert result == "'in'"

    def test_direction_out(self, toolkit):
        """测试 direction='out'"""
        result = toolkit._parse_direction_to_spl_param("out")
        assert result == "'out'"

    def test_direction_both(self, toolkit):
        """测试 direction='both'"""
        result = toolkit._parse_direction_to_spl_param("both")
        assert result == "'both'"
