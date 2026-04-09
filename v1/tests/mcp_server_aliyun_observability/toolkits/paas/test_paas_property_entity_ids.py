"""PaaS UModel MCP 工具优化 - entity_ids 格式解析属性测试

Feature: umodel-paas-optimization, Property 6: entity_ids 格式解析

**Validates: Requirements 4.3**

Property 6: entity_ids 格式解析
- *For any* 逗号分隔的 entity_ids 字符串（如 `"id1,id2,id3"`），`_build_entity_ids_param` 方法应正确解析为 SPL 格式 `ids=['id1','id2','id3']`，并正确处理空格和空值。

测试框架: pytest + hypothesis
最小迭代次数: 100 per property test
"""
import re
from typing import List, Optional
from unittest.mock import MagicMock, patch

import pytest
from hypothesis import given, settings
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


def _join_with_empty_parts(ids: List[str], empty_counts: List[int]) -> str:
    """将 ID 列表与空部分连接"""
    if len(ids) == 0:
        return ''
    if len(ids) == 1:
        return ids[0]
    
    result = ids[0]
    for i, id in enumerate(ids[1:]):
        # 添加额外的逗号（空部分）
        extra_commas = ',' * empty_counts[i] if i < len(empty_counts) else ''
        result += ',' + extra_commas + id
    return result


# ============================================================================
# Test Data Generation Strategies
# ============================================================================

# 有效的 entity_id 字符集（字母、数字、连字符、下划线）
valid_id_alphabet = 'abcdefghijklmnopqrstuvwxyz0123456789-_'

# 单个有效的 entity_id
single_entity_id = st.text(
    alphabet=valid_id_alphabet,
    min_size=1,
    max_size=20
)

# entity_ids 格式策略
entity_ids_formats = st.one_of(
    st.just(None),
    st.just(''),
    # 单个 ID
    single_entity_id,
    # 多个 ID 逗号分隔
    st.lists(
        single_entity_id,
        min_size=1,
        max_size=5
    ).map(lambda ids: ','.join(ids))
)

# 带空格的 entity_ids 格式
entity_ids_with_whitespace = st.lists(
    single_entity_id,
    min_size=1,
    max_size=5
).flatmap(lambda ids: st.tuples(
    st.just(ids),
    st.lists(
        st.sampled_from(['', ' ', '  ', '   ']),
        min_size=len(ids),
        max_size=len(ids)
    ),
    st.lists(
        st.sampled_from(['', ' ', '  ', '   ']),
        min_size=len(ids),
        max_size=len(ids)
    )
)).map(lambda t: ','.join(
    f"{before}{id}{after}" 
    for id, before, after in zip(t[0], t[1], t[2])
))

# 带空部分的 entity_ids 格式（如 "id1,,id2" 或 "id1,,,id2"）
entity_ids_with_empty_parts = st.lists(
    single_entity_id,
    min_size=1,
    max_size=5
).flatmap(lambda ids: st.tuples(
    st.just(ids),
    st.lists(
        st.integers(min_value=0, max_value=3),
        min_size=len(ids) - 1 if len(ids) > 1 else 0,
        max_size=len(ids) - 1 if len(ids) > 1 else 0
    )
)).map(lambda t: _join_with_empty_parts(t[0], t[1]))


# ============================================================================
# Property Tests - Property 6: entity_ids 格式解析
# ============================================================================

class TestEntityIdsFormatParsing:
    """Property 6: entity_ids 格式解析属性测试
    
    Feature: umodel-paas-optimization, Property 6: entity_ids 格式解析
    **Validates: Requirements 4.3**
    """

    @settings(max_examples=100)
    @given(entity_ids=st.one_of(st.just(None), st.just('')))
    def test_property_6_1_empty_input_returns_empty_string(
        self, entity_ids: Optional[str]
    ):
        """Property 6.1: 空输入（None 或空字符串）应返回空字符串
        
        Feature: umodel-paas-optimization, Property 6: entity_ids 格式解析
        **Validates: Requirements 4.3**
        """
        toolkit = create_toolkit()
        
        result = toolkit._build_entity_ids_param(entity_ids)
        
        assert result == "", f"Expected empty string for input {entity_ids!r}, got {result!r}"

    @settings(max_examples=100)
    @given(entity_id=single_entity_id)
    def test_property_6_2_single_id_parsed_correctly(
        self, entity_id: str
    ):
        """Property 6.2: 单个 entity_id 应正确解析为 SPL 格式 ids=['id']
        
        Feature: umodel-paas-optimization, Property 6: entity_ids 格式解析
        **Validates: Requirements 4.3**
        """
        toolkit = create_toolkit()
        
        result = toolkit._build_entity_ids_param(entity_id)
        
        expected = f", ids=['{entity_id}']"
        assert result == expected, f"Expected {expected!r}, got {result!r}"

    @settings(max_examples=100)
    @given(entity_ids=st.lists(single_entity_id, min_size=2, max_size=5))
    def test_property_6_3_multiple_ids_parsed_correctly(
        self, entity_ids: List[str]
    ):
        """Property 6.3: 多个 entity_ids 应正确解析为 SPL 格式 ids=['id1','id2','id3']
        
        Feature: umodel-paas-optimization, Property 6: entity_ids 格式解析
        **Validates: Requirements 4.3**
        """
        toolkit = create_toolkit()
        
        input_str = ','.join(entity_ids)
        result = toolkit._build_entity_ids_param(input_str)
        
        # 构建期望的 SPL 格式
        quoted_ids = [f"'{id}'" for id in entity_ids]
        expected = f", ids=[{','.join(quoted_ids)}]"
        
        assert result == expected, f"Expected {expected!r}, got {result!r}"

    @settings(max_examples=100)
    @given(entity_ids_str=entity_ids_with_whitespace)
    def test_property_6_4_whitespace_handled_correctly(
        self, entity_ids_str: str
    ):
        """Property 6.4: 带空格的 entity_ids 应正确处理，去除空格后解析
        
        Feature: umodel-paas-optimization, Property 6: entity_ids 格式解析
        **Validates: Requirements 4.3**
        """
        toolkit = create_toolkit()
        
        result = toolkit._build_entity_ids_param(entity_ids_str)
        
        # 提取期望的 ID 列表（去除空格和空部分）
        expected_ids = [id.strip() for id in entity_ids_str.split(',') if id.strip()]
        
        if not expected_ids:
            assert result == "", f"Expected empty string for input {entity_ids_str!r}, got {result!r}"
        else:
            # 验证结果格式正确
            assert result.startswith(", ids=["), f"Result should start with ', ids=[': {result!r}"
            assert result.endswith("]"), f"Result should end with ']': {result!r}"
            
            # 提取结果中的 ID 列表
            ids_match = re.match(r", ids=\[(.*)\]", result)
            assert ids_match, f"Result format invalid: {result!r}"
            
            ids_content = ids_match.group(1)
            # 解析结果中的 ID
            result_ids = [id.strip("'") for id in ids_content.split("','")]
            
            assert result_ids == expected_ids, f"Expected IDs {expected_ids}, got {result_ids}"

    @settings(max_examples=100)
    @given(entity_ids_str=entity_ids_with_empty_parts)
    def test_property_6_5_empty_parts_filtered_correctly(
        self, entity_ids_str: str
    ):
        """Property 6.5: 包含空部分的 entity_ids（如 "id1,,id2"）应正确过滤空部分
        
        Feature: umodel-paas-optimization, Property 6: entity_ids 格式解析
        **Validates: Requirements 4.3**
        """
        toolkit = create_toolkit()
        
        result = toolkit._build_entity_ids_param(entity_ids_str)
        
        # 提取期望的 ID 列表（过滤空部分）
        expected_ids = [id.strip() for id in entity_ids_str.split(',') if id.strip()]
        
        if not expected_ids:
            assert result == "", f"Expected empty string for input {entity_ids_str!r}, got {result!r}"
        else:
            # 验证结果格式正确
            assert result.startswith(", ids=["), f"Result should start with ', ids=[': {result!r}"
            assert result.endswith("]"), f"Result should end with ']': {result!r}"
            
            # 提取结果中的 ID 列表
            ids_match = re.match(r", ids=\[(.*)\]", result)
            assert ids_match, f"Result format invalid: {result!r}"
            
            ids_content = ids_match.group(1)
            # 解析结果中的 ID
            result_ids = [id.strip("'") for id in ids_content.split("','")]
            
            assert result_ids == expected_ids, f"Expected IDs {expected_ids}, got {result_ids}"

    @settings(max_examples=100)
    @given(entity_ids=st.lists(single_entity_id, min_size=1, max_size=5))
    def test_property_6_6_output_format_is_valid_spl(
        self, entity_ids: List[str]
    ):
        """Property 6.6: 输出格式应为有效的 SPL 参数格式 ", ids=['id1','id2']"
        
        Feature: umodel-paas-optimization, Property 6: entity_ids 格式解析
        **Validates: Requirements 4.3**
        """
        toolkit = create_toolkit()
        
        input_str = ','.join(entity_ids)
        result = toolkit._build_entity_ids_param(input_str)
        
        # 验证 SPL 格式
        # 1. 以 ", ids=[" 开头
        assert result.startswith(", ids=["), f"Result should start with ', ids=[': {result!r}"
        
        # 2. 以 "]" 结尾
        assert result.endswith("]"), f"Result should end with ']': {result!r}"
        
        # 3. 每个 ID 用单引号包裹
        ids_match = re.match(r", ids=\[(.*)\]", result)
        assert ids_match, f"Result format invalid: {result!r}"
        
        ids_content = ids_match.group(1)
        # 验证每个 ID 都被单引号包裹
        for id in entity_ids:
            assert f"'{id}'" in ids_content, f"ID '{id}' should be quoted in result: {result!r}"

    @settings(max_examples=100)
    @given(entity_ids=st.lists(single_entity_id, min_size=1, max_size=5))
    def test_property_6_7_id_order_preserved(
        self, entity_ids: List[str]
    ):
        """Property 6.7: ID 顺序应保持不变
        
        Feature: umodel-paas-optimization, Property 6: entity_ids 格式解析
        **Validates: Requirements 4.3**
        """
        toolkit = create_toolkit()
        
        input_str = ','.join(entity_ids)
        result = toolkit._build_entity_ids_param(input_str)
        
        # 提取结果中的 ID 列表
        ids_match = re.match(r", ids=\[(.*)\]", result)
        assert ids_match, f"Result format invalid: {result!r}"
        
        ids_content = ids_match.group(1)
        result_ids = [id.strip("'") for id in ids_content.split("','")]
        
        # 验证顺序保持不变
        assert result_ids == entity_ids, f"ID order should be preserved. Expected {entity_ids}, got {result_ids}"

    @settings(max_examples=100)
    @given(whitespace=st.sampled_from(['', ' ', '  ', '   ', '\t', '\n', ' \t ', '  \n  ']))
    def test_property_6_8_whitespace_only_returns_empty(
        self, whitespace: str
    ):
        """Property 6.8: 仅包含空白字符的输入应返回空字符串
        
        Feature: umodel-paas-optimization, Property 6: entity_ids 格式解析
        **Validates: Requirements 4.3**
        """
        toolkit = create_toolkit()
        
        result = toolkit._build_entity_ids_param(whitespace)
        
        assert result == "", f"Expected empty string for whitespace input {whitespace!r}, got {result!r}"


# ============================================================================
# 运行测试
# ============================================================================

if __name__ == "__main__":
    pytest.main([__file__, "-v"])
