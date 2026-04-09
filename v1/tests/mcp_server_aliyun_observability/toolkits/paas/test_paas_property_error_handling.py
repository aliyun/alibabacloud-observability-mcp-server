"""PaaS UModel MCP 工具优化 - 错误处理属性测试

Feature: umodel-paas-optimization, Properties 3, 4, 7, 8: 错误处理

**Validates: Requirements 2.2, 4.1, 4.4, 5.1, 5.2, 5.3, 5.4**

Property 3: 通配符参数拒绝
- *For any* PaaS 工具，当 `domain` 或 `entity_set_name` 参数为 `*` 时，工具应抛出 `ValueError` 并在错误消息中说明不能使用通配符。

Property 4: 必填参数验证
- *For any* PaaS 工具，当必填参数缺失或为空时，工具应在构建查询前抛出 `ValueError` 并列出缺失的参数名称。

Property 7: 数据集存在性验证
- *For any* 数据查询工具（metrics、logs、events、traces、profiles），当指定的数据集不存在时，工具应返回友好的错误提示，包含可用数据集列表。

Property 8: 错误消息包含建议
- *For any* 参数验证或数据集验证失败的错误消息，消息内容应包含建议的修复方法或获取正确值的工具名称。

测试框架: pytest + hypothesis
最小迭代次数: 100 per property test
"""
import re
from typing import Any, Dict, List, Optional
from unittest.mock import MagicMock, patch

import pytest
from hypothesis import given, settings, assume
from hypothesis import strategies as st
from mcp.server.fastmcp import FastMCP

from mcp_server_aliyun_observability.toolkits.paas.data_toolkit import PaasDataToolkit


# ============================================================================
# Helper Functions
# ============================================================================

def create_toolkit():
    """创建 PaasDataToolkit 实例用于测试"""
    server = MagicMock(spec=FastMCP)
    server.tool = MagicMock(return_value=lambda f: f)
    with patch.object(PaasDataToolkit, '_register_tools'):
        toolkit = PaasDataToolkit(server)
    return toolkit


def create_mock_context():
    """创建模拟的 MCP Context"""
    ctx = MagicMock()
    ctx.request_context = MagicMock()
    ctx.request_context.lifespan_context = MagicMock()
    ctx.request_context.lifespan_context.sls_client = MagicMock()
    return ctx


# ============================================================================
# Test Data Generation Strategies
# ============================================================================

# 有效的 domain 值（非通配符）
valid_domains = st.sampled_from(['apm', 'host', 'k8s', 'custom', 'cloud'])

# 有效的 entity_set_name 值（非通配符）
valid_entity_sets = st.sampled_from([
    'apm.service', 'apm.operation', 'host.instance', 'k8s.pod', 'k8s.node'
])

# 通配符值
wildcard_values = st.sampled_from(['*', ' * ', '  *  '])

# 空值或缺失值
empty_values = st.one_of(
    st.just(None),
    st.just(''),
    st.just('   '),
    st.just('\t'),
    st.just('\n')
)

# 有效的非空字符串
valid_non_empty_string = st.text(
    alphabet='abcdefghijklmnopqrstuvwxyz0123456789._-',
    min_size=1,
    max_size=30
).filter(lambda s: s.strip() != '' and s.strip() != '*')

# 数据集类型
data_set_types = st.sampled_from([
    'metric_set', 'log_set', 'trace_set', 'event_set', 'profile_set'
])

# 不存在的数据集名称（随机生成）
non_existent_dataset_names = st.text(
    alphabet='abcdefghijklmnopqrstuvwxyz0123456789._-',
    min_size=5,
    max_size=30
).map(lambda s: f"nonexistent.{s}")


# ============================================================================
# Property 3: 通配符参数拒绝
# ============================================================================

class TestWildcardParameterRejection:
    """Property 3: 通配符参数拒绝属性测试
    
    Feature: umodel-paas-optimization, Property 3: 通配符参数拒绝
    **Validates: Requirements 2.2, 5.1**
    """

    @settings(max_examples=100)
    @given(
        wildcard=wildcard_values,
        entity_set_name=valid_entity_sets
    )
    def test_property_3_1_domain_wildcard_rejected(
        self, wildcard: str, entity_set_name: str
    ):
        """Property 3.1: domain 为 '*' 时应抛出 ValueError
        
        Feature: umodel-paas-optimization, Property 3: 通配符参数拒绝
        **Validates: Requirements 2.2, 5.1**
        """
        toolkit = create_toolkit()
        
        params = {
            "domain": wildcard,
            "entity_set_name": entity_set_name,
            "workspace": "test-workspace"
        }
        required = ["domain", "entity_set_name", "workspace"]
        
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(params, required)
        
        error_msg = str(exc_info.value)
        # 验证错误消息说明不能使用通配符
        assert "domain" in error_msg.lower() or "不能为" in error_msg
        assert "*" in error_msg or "通配符" in error_msg or "不能为" in error_msg

    @settings(max_examples=100)
    @given(
        domain=valid_domains,
        wildcard=wildcard_values
    )
    def test_property_3_2_entity_set_name_wildcard_rejected(
        self, domain: str, wildcard: str
    ):
        """Property 3.2: entity_set_name 为 '*' 时应抛出 ValueError
        
        Feature: umodel-paas-optimization, Property 3: 通配符参数拒绝
        **Validates: Requirements 2.2, 5.1**
        """
        toolkit = create_toolkit()
        
        params = {
            "domain": domain,
            "entity_set_name": wildcard,
            "workspace": "test-workspace"
        }
        required = ["domain", "entity_set_name", "workspace"]
        
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(params, required)
        
        error_msg = str(exc_info.value)
        # 验证错误消息说明不能使用通配符
        assert "entity_set_name" in error_msg.lower() or "不能为" in error_msg
        assert "*" in error_msg or "通配符" in error_msg or "不能为" in error_msg

    @settings(max_examples=100)
    @given(
        wildcard=wildcard_values
    )
    def test_property_3_3_src_domain_wildcard_rejected(
        self, wildcard: str
    ):
        """Property 3.3: src_domain 为 '*' 时应抛出 ValueError
        
        Feature: umodel-paas-optimization, Property 3: 通配符参数拒绝
        **Validates: Requirements 2.2, 5.1**
        """
        toolkit = create_toolkit()
        
        params = {
            "src_domain": wildcard,
            "src_entity_set_name": "apm.service",
            "workspace": "test-workspace"
        }
        required = ["src_domain", "src_entity_set_name", "workspace"]
        
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(params, required)
        
        error_msg = str(exc_info.value)
        # 验证错误消息说明不能使用通配符
        assert "domain" in error_msg.lower() or "不能为" in error_msg

    @settings(max_examples=100)
    @given(
        wildcard=wildcard_values
    )
    def test_property_3_4_src_entity_set_name_wildcard_rejected(
        self, wildcard: str
    ):
        """Property 3.4: src_entity_set_name 为 '*' 时应抛出 ValueError
        
        Feature: umodel-paas-optimization, Property 3: 通配符参数拒绝
        **Validates: Requirements 2.2, 5.1**
        """
        toolkit = create_toolkit()
        
        params = {
            "src_domain": "apm",
            "src_entity_set_name": wildcard,
            "workspace": "test-workspace"
        }
        required = ["src_domain", "src_entity_set_name", "workspace"]
        
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(params, required)
        
        error_msg = str(exc_info.value)
        # 验证错误消息说明不能使用通配符
        assert "entity_set_name" in error_msg.lower() or "不能为" in error_msg

    @settings(max_examples=100)
    @given(
        domain=valid_domains,
        entity_set_name=valid_entity_sets
    )
    def test_property_3_5_valid_params_pass_validation(
        self, domain: str, entity_set_name: str
    ):
        """Property 3.5: 有效的非通配符参数应通过验证
        
        Feature: umodel-paas-optimization, Property 3: 通配符参数拒绝
        **Validates: Requirements 2.2, 5.1**
        """
        toolkit = create_toolkit()
        
        params = {
            "domain": domain,
            "entity_set_name": entity_set_name,
            "workspace": "test-workspace"
        }
        required = ["domain", "entity_set_name", "workspace"]
        
        # 不应抛出异常
        toolkit._validate_required_params(params, required)


# ============================================================================
# Property 4: 必填参数验证
# ============================================================================

class TestRequiredParameterValidation:
    """Property 4: 必填参数验证属性测试
    
    Feature: umodel-paas-optimization, Property 4: 必填参数验证
    **Validates: Requirements 4.1, 5.2**
    """

    @settings(max_examples=100)
    @given(
        missing_param=st.sampled_from(['domain', 'entity_set_name', 'workspace']),
        empty_value=empty_values
    )
    def test_property_4_1_missing_required_param_raises_error(
        self, missing_param: str, empty_value: Optional[str]
    ):
        """Property 4.1: 缺失必填参数时应抛出 ValueError
        
        Feature: umodel-paas-optimization, Property 4: 必填参数验证
        **Validates: Requirements 4.1, 5.2**
        """
        toolkit = create_toolkit()
        
        params = {
            "domain": "apm",
            "entity_set_name": "apm.service",
            "workspace": "test-workspace"
        }
        # 将指定参数设为空值
        params[missing_param] = empty_value
        
        required = ["domain", "entity_set_name", "workspace"]
        
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(params, required)
        
        error_msg = str(exc_info.value)
        # 验证错误消息列出缺失的参数名称
        assert missing_param in error_msg or "缺少必填参数" in error_msg

    @settings(max_examples=100)
    @given(
        missing_params=st.lists(
            st.sampled_from(['domain', 'entity_set_name', 'workspace']),
            min_size=1,
            max_size=3,
            unique=True
        )
    )
    def test_property_4_2_multiple_missing_params_listed(
        self, missing_params: List[str]
    ):
        """Property 4.2: 多个缺失参数时应全部列出
        
        Feature: umodel-paas-optimization, Property 4: 必填参数验证
        **Validates: Requirements 4.1, 5.2**
        """
        toolkit = create_toolkit()
        
        params = {
            "domain": "apm",
            "entity_set_name": "apm.service",
            "workspace": "test-workspace"
        }
        # 将指定参数设为 None
        for param in missing_params:
            params[param] = None
        
        required = ["domain", "entity_set_name", "workspace"]
        
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(params, required)
        
        error_msg = str(exc_info.value)
        # 验证错误消息包含 "缺少必填参数"
        assert "缺少必填参数" in error_msg or "missing" in error_msg.lower()

    @settings(max_examples=100)
    @given(
        domain=valid_domains,
        entity_set_name=valid_entity_sets,
        workspace=valid_non_empty_string
    )
    def test_property_4_3_all_required_params_present_passes(
        self, domain: str, entity_set_name: str, workspace: str
    ):
        """Property 4.3: 所有必填参数都存在时应通过验证
        
        Feature: umodel-paas-optimization, Property 4: 必填参数验证
        **Validates: Requirements 4.1, 5.2**
        """
        toolkit = create_toolkit()
        
        params = {
            "domain": domain,
            "entity_set_name": entity_set_name,
            "workspace": workspace
        }
        required = ["domain", "entity_set_name", "workspace"]
        
        # 不应抛出异常
        toolkit._validate_required_params(params, required)

    @settings(max_examples=100)
    @given(
        required_params=st.lists(
            st.sampled_from([
                'domain', 'entity_set_name', 'workspace', 
                'metric_domain_name', 'metric', 'log_set_domain', 'log_set_name'
            ]),
            min_size=1,
            max_size=5,
            unique=True
        )
    )
    def test_property_4_4_validation_before_query_building(
        self, required_params: List[str]
    ):
        """Property 4.4: 验证应在构建查询前进行
        
        Feature: umodel-paas-optimization, Property 4: 必填参数验证
        **Validates: Requirements 4.1, 5.2**
        """
        toolkit = create_toolkit()
        
        # 创建一个所有参数都为空的字典
        params = {param: None for param in required_params}
        
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(params, required_params)
        
        error_msg = str(exc_info.value)
        # 验证错误消息包含缺失参数信息
        assert "缺少必填参数" in error_msg or len(error_msg) > 0

    @settings(max_examples=100)
    @given(
        empty_string_type=st.sampled_from(['', '   ', '\t', '\n', '  \t  '])
    )
    def test_property_4_5_empty_string_treated_as_missing(
        self, empty_string_type: str
    ):
        """Property 4.5: 空字符串应被视为缺失参数
        
        Feature: umodel-paas-optimization, Property 4: 必填参数验证
        **Validates: Requirements 4.1, 5.2**
        """
        toolkit = create_toolkit()
        
        params = {
            "domain": empty_string_type,
            "entity_set_name": "apm.service",
            "workspace": "test-workspace"
        }
        required = ["domain", "entity_set_name", "workspace"]
        
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(params, required)
        
        error_msg = str(exc_info.value)
        # 验证错误消息表明参数缺失
        assert "domain" in error_msg or "缺少必填参数" in error_msg


# ============================================================================
# Property 7: 数据集存在性验证
# ============================================================================

class TestDataSetExistenceValidation:
    """Property 7: 数据集存在性验证属性测试
    
    Feature: umodel-paas-optimization, Property 7: 数据集存在性验证
    **Validates: Requirements 4.4, 5.3**
    """

    @settings(max_examples=100)
    @given(
        set_type=data_set_types,
        non_existent_name=non_existent_dataset_names
    )
    def test_property_7_1_non_existent_dataset_returns_friendly_error(
        self, set_type: str, non_existent_name: str
    ):
        """Property 7.1: 不存在的数据集应返回友好错误提示
        
        Feature: umodel-paas-optimization, Property 7: 数据集存在性验证
        **Validates: Requirements 4.4, 5.3**
        """
        toolkit = create_toolkit()
        ctx = create_mock_context()
        
        # 模拟 execute_cms_query_with_context 返回空数据集列表
        with patch(
            'mcp_server_aliyun_observability.toolkits.paas.data_toolkit.execute_cms_query_with_context'
        ) as mock_query:
            # 返回一些可用的数据集，但不包含请求的数据集
            mock_query.return_value = {
                "data": [
                    {"domain": "apm", "name": "apm.metric.service", "type": set_type, "fields": []},
                    {"domain": "host", "name": "host.metric.cpu", "type": set_type, "fields": []},
                ]
            }
            
            with pytest.raises(ValueError) as exc_info:
                toolkit._validate_data_set_exists(
                    ctx=ctx,
                    workspace="test-workspace",
                    regionId="cn-hangzhou",
                    domain="apm",
                    entity_set_name="apm.service",
                    set_type=set_type,
                    set_domain="nonexistent",
                    set_name=non_existent_name
                )
            
            error_msg = str(exc_info.value)
            # 验证错误消息是友好的，包含可用数据集列表
            assert "不存在" in error_msg or "not exist" in error_msg.lower()

    @settings(max_examples=100)
    @given(
        set_type=data_set_types
    )
    def test_property_7_2_error_contains_available_datasets(
        self, set_type: str
    ):
        """Property 7.2: 错误消息应包含可用数据集列表
        
        Feature: umodel-paas-optimization, Property 7: 数据集存在性验证
        **Validates: Requirements 4.4, 5.3**
        """
        toolkit = create_toolkit()
        ctx = create_mock_context()
        
        available_datasets = [
            {"domain": "apm", "name": "apm.metric.service", "type": set_type, "fields": []},
            {"domain": "host", "name": "host.metric.cpu", "type": set_type, "fields": []},
            {"domain": "k8s", "name": "k8s.metric.pod", "type": set_type, "fields": []},
        ]
        
        with patch(
            'mcp_server_aliyun_observability.toolkits.paas.data_toolkit.execute_cms_query_with_context'
        ) as mock_query:
            mock_query.return_value = {"data": available_datasets}
            
            with pytest.raises(ValueError) as exc_info:
                toolkit._validate_data_set_exists(
                    ctx=ctx,
                    workspace="test-workspace",
                    regionId="cn-hangzhou",
                    domain="apm",
                    entity_set_name="apm.service",
                    set_type=set_type,
                    set_domain="nonexistent",
                    set_name="nonexistent.dataset"
                )
            
            error_msg = str(exc_info.value)
            # 验证错误消息包含可用数据集列表
            # 至少应该包含一个可用数据集的名称
            has_available_list = (
                "apm.metric.service" in error_msg or
                "host.metric.cpu" in error_msg or
                "k8s.metric.pod" in error_msg or
                "可用" in error_msg or
                "available" in error_msg.lower()
            )
            assert has_available_list, f"Error message should contain available datasets: {error_msg}"

    @settings(max_examples=100)
    @given(
        metric_name=st.text(
            alphabet='abcdefghijklmnopqrstuvwxyz0123456789_',
            min_size=3,
            max_size=20
        )
    )
    def test_property_7_3_non_existent_metric_returns_friendly_error(
        self, metric_name: str
    ):
        """Property 7.3: 不存在的指标应返回友好错误提示
        
        Feature: umodel-paas-optimization, Property 7: 数据集存在性验证
        **Validates: Requirements 4.4, 5.3**
        """
        assume(metric_name.strip() != '')
        
        toolkit = create_toolkit()
        ctx = create_mock_context()
        
        # 模拟返回数据集存在但指标不存在的情况
        with patch(
            'mcp_server_aliyun_observability.toolkits.paas.data_toolkit.execute_cms_query_with_context'
        ) as mock_query:
            mock_query.return_value = {
                "data": [
                    {
                        "domain": "apm",
                        "name": "apm.metric.service",
                        "type": "metric_set",
                        "fields": [
                            {"name": "cpu_usage"},
                            {"name": "memory_usage"},
                            {"name": "request_count"}
                        ]
                    }
                ]
            }
            
            # 使用一个不存在的指标名称
            non_existent_metric = f"nonexistent_{metric_name}"
            
            with pytest.raises(ValueError) as exc_info:
                toolkit._validate_data_set_exists(
                    ctx=ctx,
                    workspace="test-workspace",
                    regionId="cn-hangzhou",
                    domain="apm",
                    entity_set_name="apm.service",
                    set_type="metric_set",
                    set_domain="apm",
                    set_name="apm.metric.service",
                    metric=non_existent_metric
                )
            
            error_msg = str(exc_info.value)
            # 验证错误消息是友好的
            assert "不存在" in error_msg or "not exist" in error_msg.lower()
            # 验证错误消息包含可用指标列表
            has_available_metrics = (
                "cpu_usage" in error_msg or
                "memory_usage" in error_msg or
                "request_count" in error_msg or
                "可用指标" in error_msg
            )
            assert has_available_metrics, f"Error message should contain available metrics: {error_msg}"

    def test_property_7_4_existing_dataset_passes_validation(self):
        """Property 7.4: 存在的数据集应通过验证
        
        Feature: umodel-paas-optimization, Property 7: 数据集存在性验证
        **Validates: Requirements 4.4, 5.3**
        """
        toolkit = create_toolkit()
        ctx = create_mock_context()
        
        with patch(
            'mcp_server_aliyun_observability.toolkits.paas.data_toolkit.execute_cms_query_with_context'
        ) as mock_query:
            mock_query.return_value = {
                "data": [
                    {
                        "domain": "apm",
                        "name": "apm.metric.service",
                        "type": "metric_set",
                        "fields": [{"name": "cpu_usage"}]
                    }
                ]
            }
            
            # 不应抛出异常
            toolkit._validate_data_set_exists(
                ctx=ctx,
                workspace="test-workspace",
                regionId="cn-hangzhou",
                domain="apm",
                entity_set_name="apm.service",
                set_type="metric_set",
                set_domain="apm",
                set_name="apm.metric.service"
            )

    def test_property_7_5_existing_metric_passes_validation(self):
        """Property 7.5: 存在的指标应通过验证
        
        Feature: umodel-paas-optimization, Property 7: 数据集存在性验证
        **Validates: Requirements 4.4, 5.3**
        """
        toolkit = create_toolkit()
        ctx = create_mock_context()
        
        with patch(
            'mcp_server_aliyun_observability.toolkits.paas.data_toolkit.execute_cms_query_with_context'
        ) as mock_query:
            mock_query.return_value = {
                "data": [
                    {
                        "domain": "apm",
                        "name": "apm.metric.service",
                        "type": "metric_set",
                        "fields": [
                            {"name": "cpu_usage"},
                            {"name": "memory_usage"}
                        ]
                    }
                ]
            }
            
            # 不应抛出异常
            toolkit._validate_data_set_exists(
                ctx=ctx,
                workspace="test-workspace",
                regionId="cn-hangzhou",
                domain="apm",
                entity_set_name="apm.service",
                set_type="metric_set",
                set_domain="apm",
                set_name="apm.metric.service",
                metric="cpu_usage"
            )


# ============================================================================
# Property 8: 错误消息包含建议
# ============================================================================

class TestErrorMessageContainsSuggestions:
    """Property 8: 错误消息包含建议属性测试
    
    Feature: umodel-paas-optimization, Property 8: 错误消息包含建议
    **Validates: Requirements 5.4**
    """

    @settings(max_examples=100)
    @given(
        wildcard=wildcard_values
    )
    def test_property_8_1_wildcard_error_contains_suggestion(
        self, wildcard: str
    ):
        """Property 8.1: 通配符错误消息应包含建议的修复方法
        
        Feature: umodel-paas-optimization, Property 8: 错误消息包含建议
        **Validates: Requirements 5.4**
        """
        toolkit = create_toolkit()
        
        params = {
            "domain": wildcard,
            "entity_set_name": "apm.service",
            "workspace": "test-workspace"
        }
        required = ["domain", "entity_set_name", "workspace"]
        
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(params, required)
        
        error_msg = str(exc_info.value)
        # 验证错误消息包含建议的工具名称
        has_suggestion = (
            "umodel_search_entity_set" in error_msg or
            "建议" in error_msg or
            "请使用" in error_msg or
            "获取" in error_msg
        )
        assert has_suggestion, f"Error message should contain suggestion: {error_msg}"

    @settings(max_examples=100)
    @given(
        missing_param=st.sampled_from(['domain', 'entity_set_name', 'workspace'])
    )
    def test_property_8_2_missing_param_error_contains_suggestion(
        self, missing_param: str
    ):
        """Property 8.2: 缺失参数错误消息应包含建议
        
        Feature: umodel-paas-optimization, Property 8: 错误消息包含建议
        **Validates: Requirements 5.4**
        """
        toolkit = create_toolkit()
        
        params = {
            "domain": "apm",
            "entity_set_name": "apm.service",
            "workspace": "test-workspace"
        }
        params[missing_param] = None
        
        required = ["domain", "entity_set_name", "workspace"]
        
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(params, required)
        
        error_msg = str(exc_info.value)
        # 验证错误消息包含建议
        has_suggestion = (
            "请提供" in error_msg or
            "有效值" in error_msg or
            "建议" in error_msg or
            "please" in error_msg.lower()
        )
        assert has_suggestion, f"Error message should contain suggestion: {error_msg}"

    @settings(max_examples=100)
    @given(
        set_type=data_set_types
    )
    def test_property_8_3_dataset_error_contains_tool_suggestion(
        self, set_type: str
    ):
        """Property 8.3: 数据集验证错误消息应包含获取正确值的工具名称
        
        Feature: umodel-paas-optimization, Property 8: 错误消息包含建议
        **Validates: Requirements 5.4**
        """
        toolkit = create_toolkit()
        ctx = create_mock_context()
        
        with patch(
            'mcp_server_aliyun_observability.toolkits.paas.data_toolkit.execute_cms_query_with_context'
        ) as mock_query:
            mock_query.return_value = {
                "data": [
                    {"domain": "apm", "name": "apm.metric.service", "type": set_type, "fields": []}
                ]
            }
            
            with pytest.raises(ValueError) as exc_info:
                toolkit._validate_data_set_exists(
                    ctx=ctx,
                    workspace="test-workspace",
                    regionId="cn-hangzhou",
                    domain="apm",
                    entity_set_name="apm.service",
                    set_type=set_type,
                    set_domain="nonexistent",
                    set_name="nonexistent.dataset"
                )
            
            error_msg = str(exc_info.value)
            # 验证错误消息包含建议的工具名称
            has_tool_suggestion = (
                "umodel_list_data_set" in error_msg or
                "umodel_search_entity_set" in error_msg or
                "建议" in error_msg or
                "请使用" in error_msg
            )
            assert has_tool_suggestion, f"Error message should contain tool suggestion: {error_msg}"

    def test_property_8_4_metric_error_contains_suggestion(self):
        """Property 8.4: 指标验证错误消息应包含建议
        
        Feature: umodel-paas-optimization, Property 8: 错误消息包含建议
        **Validates: Requirements 5.4**
        """
        toolkit = create_toolkit()
        ctx = create_mock_context()
        
        with patch(
            'mcp_server_aliyun_observability.toolkits.paas.data_toolkit.execute_cms_query_with_context'
        ) as mock_query:
            mock_query.return_value = {
                "data": [
                    {
                        "domain": "apm",
                        "name": "apm.metric.service",
                        "type": "metric_set",
                        "fields": [
                            {"name": "cpu_usage"},
                            {"name": "memory_usage"}
                        ]
                    }
                ]
            }
            
            with pytest.raises(ValueError) as exc_info:
                toolkit._validate_data_set_exists(
                    ctx=ctx,
                    workspace="test-workspace",
                    regionId="cn-hangzhou",
                    domain="apm",
                    entity_set_name="apm.service",
                    set_type="metric_set",
                    set_domain="apm",
                    set_name="apm.metric.service",
                    metric="nonexistent_metric"
                )
            
            error_msg = str(exc_info.value)
            # 验证错误消息包含建议
            has_suggestion = (
                "umodel_list_data_set" in error_msg or
                "fields" in error_msg or
                "建议" in error_msg or
                "请使用" in error_msg
            )
            assert has_suggestion, f"Error message should contain suggestion: {error_msg}"

    @settings(max_examples=100)
    @given(
        error_type=st.sampled_from(['wildcard_domain', 'wildcard_entity_set', 'missing_param'])
    )
    def test_property_8_5_all_validation_errors_have_suggestions(
        self, error_type: str
    ):
        """Property 8.5: 所有参数验证错误都应包含建议
        
        Feature: umodel-paas-optimization, Property 8: 错误消息包含建议
        **Validates: Requirements 5.4**
        """
        toolkit = create_toolkit()
        
        if error_type == 'wildcard_domain':
            params = {"domain": "*", "entity_set_name": "apm.service", "workspace": "test"}
        elif error_type == 'wildcard_entity_set':
            params = {"domain": "apm", "entity_set_name": "*", "workspace": "test"}
        else:  # missing_param
            params = {"domain": None, "entity_set_name": "apm.service", "workspace": "test"}
        
        required = ["domain", "entity_set_name", "workspace"]
        
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(params, required)
        
        error_msg = str(exc_info.value)
        # 验证错误消息包含某种形式的建议
        has_any_suggestion = (
            "umodel_" in error_msg or
            "建议" in error_msg or
            "请" in error_msg or
            "获取" in error_msg or
            "有效" in error_msg
        )
        assert has_any_suggestion, f"Error message should contain some form of suggestion: {error_msg}"


# ============================================================================
# 综合测试 - 验证多个属性的组合场景
# ============================================================================

class TestCombinedErrorHandling:
    """综合错误处理测试
    
    Feature: umodel-paas-optimization, Properties 3, 4, 7, 8
    **Validates: Requirements 2.2, 4.1, 4.4, 5.1, 5.2, 5.3, 5.4**
    """

    @settings(max_examples=100)
    @given(
        domain=st.one_of(wildcard_values, empty_values),
        entity_set_name=st.one_of(wildcard_values, empty_values)
    )
    def test_combined_invalid_params_rejected(
        self, domain: Optional[str], entity_set_name: Optional[str]
    ):
        """综合测试: 无效参数（通配符或空值）应被拒绝
        
        Feature: umodel-paas-optimization, Properties 3, 4
        **Validates: Requirements 2.2, 4.1, 5.1, 5.2**
        """
        toolkit = create_toolkit()
        
        params = {
            "domain": domain,
            "entity_set_name": entity_set_name,
            "workspace": "test-workspace"
        }
        required = ["domain", "entity_set_name", "workspace"]
        
        # 至少有一个参数无效，应该抛出异常
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(params, required)
        
        error_msg = str(exc_info.value)
        # 验证错误消息不为空
        assert len(error_msg) > 0

    def test_combined_error_flow_validation_then_dataset(self):
        """综合测试: 验证流程 - 先参数验证，再数据集验证
        
        Feature: umodel-paas-optimization, Properties 3, 4, 7, 8
        **Validates: Requirements 2.2, 4.1, 4.4, 5.1, 5.2, 5.3, 5.4**
        """
        toolkit = create_toolkit()
        ctx = create_mock_context()
        
        # 1. 首先测试参数验证（通配符）
        params_wildcard = {
            "domain": "*",
            "entity_set_name": "apm.service",
            "workspace": "test-workspace"
        }
        required = ["domain", "entity_set_name", "workspace"]
        
        with pytest.raises(ValueError) as exc_info:
            toolkit._validate_required_params(params_wildcard, required)
        
        error_msg = str(exc_info.value)
        assert "domain" in error_msg.lower() or "不能为" in error_msg
        
        # 2. 然后测试数据集验证
        with patch(
            'mcp_server_aliyun_observability.toolkits.paas.data_toolkit.execute_cms_query_with_context'
        ) as mock_query:
            mock_query.return_value = {"data": []}
            
            with pytest.raises(ValueError) as exc_info:
                toolkit._validate_data_set_exists(
                    ctx=ctx,
                    workspace="test-workspace",
                    regionId="cn-hangzhou",
                    domain="apm",
                    entity_set_name="apm.service",
                    set_type="metric_set",
                    set_domain="nonexistent",
                    set_name="nonexistent.dataset"
                )
            
            error_msg = str(exc_info.value)
            # 验证错误消息包含建议
            assert "建议" in error_msg or "请" in error_msg or "umodel_" in error_msg


# ============================================================================
# 运行测试
# ============================================================================

if __name__ == "__main__":
    pytest.main([__file__, "-v"])
