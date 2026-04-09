"""PaaS Data Toolkit 辅助方法单元测试

测试 _parse_time_range, _build_standard_response, _validate_required_params 等辅助方法。
这些测试不需要真实的阿里云凭证，属于单元测试。

Requirements: 1.1, 1.3, 3.1, 4.1
"""
import time
from unittest.mock import MagicMock, patch

import pytest
from mcp.server.fastmcp import FastMCP

from mcp_server_aliyun_observability.toolkits.paas.data_toolkit import PaasDataToolkit


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
    """创建 PaasDataToolkit 实例用于测试辅助方法"""
    with patch.object(PaasDataToolkit, '_register_tools'):
        toolkit = PaasDataToolkit(mock_server)
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

    def test_whitespace_only_defaults_to_last_1h(self, toolkit):
        """测试仅空白字符输入默认使用 last_1h"""
        from_ts, to_ts = toolkit._parse_time_range("   ")
        
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

    def test_valid_relative_preset_last_1h(self, toolkit):
        """测试有效的相对预设 last_1h"""
        from_ts, to_ts = toolkit._parse_time_range("last_1h")
        
        duration = to_ts - from_ts
        assert 3590 <= duration <= 3610

    def test_valid_relative_preset_last_3d(self, toolkit):
        """测试有效的相对预设 last_3d"""
        from_ts, to_ts = toolkit._parse_time_range("last_3d")
        
        duration = to_ts - from_ts
        expected_duration = 3 * 24 * 3600  # 3 天
        assert expected_duration - 10 <= duration <= expected_duration + 10

    def test_grafana_style_now_minus_15m_to_now_minus_5m(self, toolkit):
        """测试 Grafana 风格 now-15m~now-5m"""
        from_ts, to_ts = toolkit._parse_time_range("now-15m~now-5m")
        
        assert isinstance(from_ts, int)
        assert isinstance(to_ts, int)
        assert from_ts < to_ts
        
        # 验证时间范围约为 10 分钟
        duration = to_ts - from_ts
        assert 590 <= duration <= 610

    def test_grafana_style_now_minus_1h_to_now(self, toolkit):
        """测试 Grafana 风格 now-1h~now"""
        from_ts, to_ts = toolkit._parse_time_range("now-1h~now")
        
        duration = to_ts - from_ts
        assert 3590 <= duration <= 3610

    def test_keyword_today(self, toolkit):
        """测试关键字 today"""
        from_ts, to_ts = toolkit._parse_time_range("today")
        
        assert isinstance(from_ts, int)
        assert isinstance(to_ts, int)
        assert from_ts < to_ts
        
        # 验证 from_ts 是今天的开始（午夜）
        # to_ts 应该是当前时间
        now = int(time.time())
        assert to_ts <= now + 5  # 允许几秒误差

    def test_keyword_yesterday(self, toolkit):
        """测试关键字 yesterday"""
        from_ts, to_ts = toolkit._parse_time_range("yesterday")
        
        assert isinstance(from_ts, int)
        assert isinstance(to_ts, int)
        assert from_ts < to_ts
        
        # 验证时间范围约为 1 天
        duration = to_ts - from_ts
        assert 86390 <= duration <= 86410

    def test_absolute_timestamps(self, toolkit):
        """测试绝对时间戳 1706864400~1706868000"""
        from_ts, to_ts = toolkit._parse_time_range("1706864400~1706868000")
        
        assert from_ts == 1706864400
        assert to_ts == 1706868000

    def test_human_readable_format(self, toolkit):
        """测试人类可读格式 2024-02-02 10:10:10~2024-02-02 10:20:10"""
        from_ts, to_ts = toolkit._parse_time_range(
            "2024-02-02 10:10:10~2024-02-02 10:20:10"
        )
        
        assert isinstance(from_ts, int)
        assert isinstance(to_ts, int)
        assert from_ts < to_ts
        
        # 验证时间范围约为 10 分钟
        duration = to_ts - from_ts
        assert duration == 600

    def test_invalid_expression_raises_valueerror(self, toolkit):
        """测试无效表达式抛出 ValueError 并包含建议"""
        with pytest.raises(ValueError) as exc_info:
            toolkit._parse_time_range("invalid_time_expression")
        
        error_message = str(exc_info.value)
        assert "无效的时间表达式" in error_message
        # 验证错误消息包含建议的格式
        assert "last_5m" in error_message or "支持格式" in error_message

    def test_invalid_range_expression_raises_valueerror(self, toolkit):
        """测试无效的范围表达式抛出 ValueError"""
        with pytest.raises(ValueError) as exc_info:
            toolkit._parse_time_range("invalid~also_invalid")
        
        error_message = str(exc_info.value)
        assert "无效" in error_message or "无法解析" in error_message


# ============================================================================
# _build_standard_response 测试
# ============================================================================

class TestBuildStandardResponse:
    """测试 _build_standard_response 方法"""

    def test_response_contains_all_required_fields(self, toolkit):
        """测试响应包含所有必需字段: error, data, message, query, time_range"""
        response = toolkit._build_standard_response(
            data=[{"metric": "cpu", "value": 0.5}],
            query=".entity_set with(domain='apm') | entity-call get_metric(...)",
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

    def test_time_range_values_correct(self, toolkit):
        """测试 time_range 字段值正确"""
        response = toolkit._build_standard_response(
            data=[],
            query="test query",
            time_range=(1706864400, 1706868000),
            time_range_expression="last_1h"
        )
        
        time_range = response["time_range"]
        assert time_range["from"] == 1706864400
        assert time_range["to"] == 1706868000
        assert time_range["expression"] == "last_1h"
        
        # 验证可读时间格式
        assert isinstance(time_range["from_readable"], str)
        assert isinstance(time_range["to_readable"], str)
        # 验证格式类似 "YYYY-MM-DD HH:MM:SS"
        assert len(time_range["from_readable"]) == 19
        assert len(time_range["to_readable"]) == 19

    def test_default_message_for_successful_query_with_data(self, toolkit):
        """测试成功查询有数据时的默认消息"""
        response = toolkit._build_standard_response(
            data=[{"metric": "cpu", "value": 0.5}],
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

    def test_default_message_for_none_data(self, toolkit):
        """测试 data 为 None 时的默认消息"""
        response = toolkit._build_standard_response(
            data=None,
            query="test query",
            time_range=(1706864400, 1706868000)
        )
        
        assert response["error"] is False
        assert response["message"] == "No data found"

    def test_default_message_for_error_response(self, toolkit):
        """测试错误响应的默认消息"""
        response = toolkit._build_standard_response(
            data=None,
            query="test query",
            time_range=(1706864400, 1706868000),
            error=True
        )
        
        assert response["error"] is True
        assert response["message"] == "Query failed"

    def test_custom_message_overrides_default(self, toolkit):
        """测试自定义消息覆盖默认消息"""
        custom_message = "Custom success message"
        response = toolkit._build_standard_response(
            data=[{"metric": "cpu"}],
            query="test query",
            time_range=(1706864400, 1706868000),
            message=custom_message
        )
        
        assert response["message"] == custom_message

    def test_query_field_preserved(self, toolkit):
        """测试 query 字段正确保留"""
        test_query = ".entity_set with(domain='apm', name='apm.service') | entity-call get_metric(...)"
        response = toolkit._build_standard_response(
            data=[],
            query=test_query,
            time_range=(1706864400, 1706868000)
        )
        
        assert response["query"] == test_query

    def test_data_field_preserved(self, toolkit):
        """测试 data 字段正确保留"""
        test_data = [
            {"metric": "cpu", "value": 0.5},
            {"metric": "memory", "value": 0.8}
        ]
        response = toolkit._build_standard_response(
            data=test_data,
            query="test query",
            time_range=(1706864400, 1706868000)
        )
        
        assert response["data"] == test_data

    def test_empty_time_range_expression(self, toolkit):
        """测试不提供 time_range_expression 时默认为空字符串"""
        response = toolkit._build_standard_response(
            data=[],
            query="test query",
            time_range=(1706864400, 1706868000)
        )
        
        assert response["time_range"]["expression"] == ""


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

    def test_multiple_missing_parameters_listed_in_error(self, toolkit):
        """测试多个缺失参数在错误消息中列出"""
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(
                {},
                ["domain", "entity_set_name", "workspace"]
            )
        
        error_message = str(exc_info.value)
        assert "domain" in error_message
        assert "entity_set_name" in error_message
        assert "workspace" in error_message

    def test_empty_string_treated_as_missing(self, toolkit):
        """测试空字符串被视为缺失"""
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(
                {"domain": "", "entity_set_name": "apm.service"},
                ["domain", "entity_set_name"]
            )
        
        error_message = str(exc_info.value)
        assert "domain" in error_message
        assert "缺少必填参数" in error_message

    def test_whitespace_only_treated_as_missing(self, toolkit):
        """测试仅空白字符被视为缺失"""
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(
                {"domain": "   ", "entity_set_name": "apm.service"},
                ["domain", "entity_set_name"]
            )
        
        error_message = str(exc_info.value)
        assert "domain" in error_message
        assert "缺少必填参数" in error_message

    def test_none_value_treated_as_missing(self, toolkit):
        """测试 None 值被视为缺失"""
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(
                {"domain": None, "entity_set_name": "apm.service"},
                ["domain", "entity_set_name"]
            )
        
        error_message = str(exc_info.value)
        assert "domain" in error_message
        assert "缺少必填参数" in error_message

    def test_wildcard_with_whitespace_raises_valueerror(self, toolkit):
        """测试带空白的通配符 ' * ' 抛出 ValueError"""
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(
                {"domain": " * ", "entity_set_name": "apm.service"},
                ["domain", "entity_set_name"]
            )
        
        error_message = str(exc_info.value)
        assert "domain" in error_message
        assert "不能为" in error_message

    def test_non_string_values_pass_validation(self, toolkit):
        """测试非字符串值（如整数）通过验证"""
        # 不应抛出异常
        toolkit._validate_required_params(
            {"domain": "apm", "entity_set_name": "apm.service", "limit": 100},
            ["domain", "entity_set_name", "limit"]
        )

    def test_error_message_contains_suggestion(self, toolkit):
        """测试错误消息包含建议的修复方法"""
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(
                {"domain": "*"},
                ["domain"]
            )
        
        error_message = str(exc_info.value)
        # 验证错误消息包含获取正确值的工具名称
        assert "umodel_search_entity_set" in error_message

    def test_empty_required_list_passes(self, toolkit):
        """测试空的必填参数列表通过验证"""
        # 不应抛出异常
        toolkit._validate_required_params(
            {"domain": "apm"},
            []
        )

    def test_extra_params_ignored(self, toolkit):
        """测试额外参数被忽略"""
        # 不应抛出异常
        toolkit._validate_required_params(
            {"domain": "apm", "entity_set_name": "apm.service", "extra_param": "value"},
            ["domain", "entity_set_name"]
        )

    def test_src_domain_wildcard_raises_valueerror(self, toolkit):
        """测试 src_domain = '*' 抛出 ValueError 并包含正确消息

        Requirements: 2.2, 5.1
        """
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(
                {"src_domain": "*", "src_entity_set_name": "apm.service"},
                ["src_domain", "src_entity_set_name"]
            )
        
        error_message = str(exc_info.value)
        assert "domain" in error_message
        assert "*" in error_message or "不能为" in error_message
        assert "umodel_search_entity_set" in error_message

    def test_src_entity_set_name_wildcard_raises_valueerror(self, toolkit):
        """测试 src_entity_set_name = '*' 抛出 ValueError 并包含正确消息

        Requirements: 2.2, 5.1
        """
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(
                {"src_domain": "apm", "src_entity_set_name": "*"},
                ["src_domain", "src_entity_set_name"]
            )
        
        error_message = str(exc_info.value)
        assert "entity_set_name" in error_message
        assert "*" in error_message or "不能为" in error_message
        assert "umodel_search_entity_set" in error_message

    def test_src_domain_wildcard_with_whitespace_raises_valueerror(self, toolkit):
        """测试带空白的 src_domain 通配符 ' * ' 抛出 ValueError

        Requirements: 2.2, 5.1
        """
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(
                {"src_domain": " * ", "src_entity_set_name": "apm.service"},
                ["src_domain", "src_entity_set_name"]
            )
        
        error_message = str(exc_info.value)
        assert "domain" in error_message
        assert "不能为" in error_message

    def test_src_entity_set_name_wildcard_with_whitespace_raises_valueerror(self, toolkit):
        """测试带空白的 src_entity_set_name 通配符 ' * ' 抛出 ValueError

        Requirements: 2.2, 5.1
        """
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(
                {"src_domain": "apm", "src_entity_set_name": " * "},
                ["src_domain", "src_entity_set_name"]
            )
        
        error_message = str(exc_info.value)
        assert "entity_set_name" in error_message
        assert "不能为" in error_message

    def test_valid_src_domain_and_src_entity_set_name_pass_validation(self, toolkit):
        """测试有效的 src_domain 和 src_entity_set_name 通过验证

        Requirements: 2.2, 5.1
        """
        # 不应抛出异常
        toolkit._validate_required_params(
            {"src_domain": "apm", "src_entity_set_name": "apm.service"},
            ["src_domain", "src_entity_set_name"]
        )


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

    def test_whitespace_only_returns_empty_string(self, toolkit):
        """测试仅空白字符输入返回空字符串"""
        result = toolkit._build_entity_filter_param("   ")
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

    def test_three_conditions_with_and(self, toolkit):
        """测试三个条件 and 连接"""
        result = toolkit._build_entity_filter_param("name=payment and status!=inactive and type=service")
        assert result == ', query=`"name"=\'payment\' and "status"!=\'inactive\' and "type"=\'service\'`'

    def test_expression_with_extra_whitespace(self, toolkit):
        """测试带额外空白的表达式"""
        result = toolkit._build_entity_filter_param("  name = payment  and  status != inactive  ")
        assert result == ', query=`"name"=\'payment\' and "status"!=\'inactive\'`'

    def test_quoted_value_with_double_quotes(self, toolkit):
        """测试带双引号的值"""
        result = toolkit._build_entity_filter_param('name="my service"')
        assert result == ', query=`"name"=\'my service\'`'

    def test_quoted_value_with_single_quotes(self, toolkit):
        """测试带单引号的值"""
        result = toolkit._build_entity_filter_param("name='my service'")
        assert result == ', query=`"name"=\'my service\'`'

    def test_quoted_field_with_double_quotes(self, toolkit):
        """测试带双引号的字段名"""
        result = toolkit._build_entity_filter_param('"field name"=value')
        assert result == ', query=`"field name"=\'value\'`'

    def test_invalid_expression_no_operator_raises_valueerror(self, toolkit):
        """测试无操作符的无效表达式抛出 ValueError"""
        with pytest.raises(ValueError) as exc_info:
            toolkit._build_entity_filter_param("invalid_expression")
        
        error_message = str(exc_info.value)
        assert "操作符" in error_message or "operator" in error_message.lower()
        assert "=" in error_message or "!=" in error_message

    def test_invalid_expression_empty_field_raises_valueerror(self, toolkit):
        """测试空字段名的无效表达式抛出 ValueError"""
        with pytest.raises(ValueError) as exc_info:
            toolkit._build_entity_filter_param("=value")
        
        error_message = str(exc_info.value)
        assert "空" in error_message or "empty" in error_message.lower()

    def test_invalid_expression_empty_value_raises_valueerror(self, toolkit):
        """测试空值的无效表达式抛出 ValueError"""
        with pytest.raises(ValueError) as exc_info:
            toolkit._build_entity_filter_param("field=")
        
        error_message = str(exc_info.value)
        assert "空" in error_message or "empty" in error_message.lower()

    def test_error_message_contains_examples(self, toolkit):
        """测试错误消息包含示例"""
        with pytest.raises(ValueError) as exc_info:
            toolkit._build_entity_filter_param("invalid")
        
        error_message = str(exc_info.value)
        # 验证错误消息包含示例
        assert "name=payment" in error_message or "status!=inactive" in error_message

    def test_value_with_special_characters(self, toolkit):
        """测试值包含特殊字符"""
        result = toolkit._build_entity_filter_param("name=my-service_v1.0")
        assert result == ', query=`"name"=\'my-service_v1.0\'`'

    def test_numeric_value(self, toolkit):
        """测试数值类型的值"""
        result = toolkit._build_entity_filter_param("count=100")
        assert result == ', query=`"count"=\'100\'`'


# ============================================================================
# _trim_quotes 测试
# ============================================================================

class TestTrimQuotes:
    """测试 _trim_quotes 方法"""

    def test_double_quoted_string(self, toolkit):
        """测试去除双引号"""
        result = toolkit._trim_quotes('"hello"')
        assert result == "hello"

    def test_single_quoted_string(self, toolkit):
        """测试去除单引号"""
        result = toolkit._trim_quotes("'world'")
        assert result == "world"

    def test_no_quotes(self, toolkit):
        """测试无引号字符串"""
        result = toolkit._trim_quotes("no_quotes")
        assert result == "no_quotes"

    def test_mismatched_quotes_not_trimmed(self, toolkit):
        """测试不匹配的引号不被去除"""
        result = toolkit._trim_quotes('"hello\'')
        assert result == '"hello\''

    def test_empty_string(self, toolkit):
        """测试空字符串"""
        result = toolkit._trim_quotes("")
        assert result == ""

    def test_single_character(self, toolkit):
        """测试单字符"""
        result = toolkit._trim_quotes("a")
        assert result == "a"

    def test_only_quotes(self, toolkit):
        """测试仅引号"""
        result = toolkit._trim_quotes('""')
        assert result == ""


# ============================================================================
# _convert_to_sql_syntax 测试
# ============================================================================

class TestConvertToSqlSyntax:
    """测试 _convert_to_sql_syntax 方法"""

    def test_single_equals_condition(self, toolkit):
        """测试单个等于条件"""
        result = toolkit._convert_to_sql_syntax("name=value")
        assert result == '"name"=\'value\''

    def test_single_not_equals_condition(self, toolkit):
        """测试单个不等于条件"""
        result = toolkit._convert_to_sql_syntax("status!=inactive")
        assert result == '"status"!=\'inactive\''

    def test_multiple_conditions(self, toolkit):
        """测试多个条件"""
        result = toolkit._convert_to_sql_syntax("name=payment and status!=inactive")
        assert result == '"name"=\'payment\' and "status"!=\'inactive\''

    def test_empty_conditions_after_split_raises_valueerror(self, toolkit):
        """测试分割后无有效条件抛出 ValueError"""
        with pytest.raises(ValueError):
            toolkit._convert_to_sql_syntax(" and ")


# ============================================================================
# _parse_condition 测试
# ============================================================================

class TestParseCondition:
    """测试 _parse_condition 方法"""

    def test_equals_condition(self, toolkit):
        """测试等于条件"""
        result = toolkit._parse_condition("name=value")
        assert result == '"name"=\'value\''

    def test_not_equals_condition(self, toolkit):
        """测试不等于条件"""
        result = toolkit._parse_condition("status!=inactive")
        assert result == '"status"!=\'inactive\''

    def test_condition_with_whitespace(self, toolkit):
        """测试带空白的条件"""
        result = toolkit._parse_condition("  name  =  value  ")
        assert result == '"name"=\'value\''

    def test_no_operator_raises_valueerror(self, toolkit):
        """测试无操作符抛出 ValueError"""
        with pytest.raises(ValueError):
            toolkit._parse_condition("invalid")

    def test_empty_field_raises_valueerror(self, toolkit):
        """测试空字段抛出 ValueError"""
        with pytest.raises(ValueError):
            toolkit._parse_condition("=value")

    def test_empty_value_raises_valueerror(self, toolkit):
        """测试空值抛出 ValueError"""
        with pytest.raises(ValueError):
            toolkit._parse_condition("field=")


# ============================================================================
# 运行测试
# ============================================================================

if __name__ == "__main__":
    pytest.main([__file__, "-v"])
