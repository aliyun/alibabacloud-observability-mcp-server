# 高质量UMODEL使用笔记

## 核心概念

### 什么是UMODEL
UMODEL (Unified Model) 是阿里云可观测2.0中的统一数据模型，它将底层的存储细节抽象为高层的实体和数据集概念，提供统一的SPL查询接口。

### 架构层次
```
用户SPL查询 → UMODEL转换 → 底层存储查询 (SLS/Prometheus等)
```

## 两种核心模式

### Phase 1: Table模式 (直接数据集访问)
直接访问具体的数据集，适合明确知道数据源的场景。

#### MetricSet查询
```spl
-- 指标数据查询
.metric_set with(
    domain='apm', 
    name='apm.metric.apm.operation', 
    source='metrics', 
    metric='avg_request_latency_ms',
    query_type='range',
    step='1m',
    aggregate_labels=['host', 'service']
) | where service_id = 'checkout-service'

-- 标签值查询
.metric_set with(
    domain='apm',
    name='apm.metric.apm.operation',
    source='labels',
    label='service_id'
) | where operation_name = 'GET /api/orders'
```

#### 关键参数
- `domain`: 数据域 (apm, rum, infra等)
- `name`: 数据集完整名称
- `source`: 数据源类型 (metrics/labels)
- `query_type`: 查询类型 (range/instant)
- `step`: 时间聚合粒度
- `aggregate_labels`: 聚合维度

### Phase 2: Object模式 (实体驱动访问)
通过实体抽象访问相关数据，适合业务场景和多数据源关联查询。

#### 实体集合操作
```spl
-- 列出实体支持的方法
.entity_set(domain='apm', name='apm.operation') 
| entity-call __list_method__()

-- 获取相关数据集
.entity_set(domain='apm', name='apm.operation') 
| entity-call list_data_set(['metric_set', 'log_set'], true)

-- 查询实体关系
.entity_set(domain='apm', name='apm.operation') 
| entity-call list_related_entity_set(['calls'], 'out', true)

-- 获取指标数据 (自动处理实体映射)
.entity_set(domain='apm', name='apm.operation', ids=['op-001', 'op-002']) 
| entity-call get_metric('apm', 'apm.metric.apm.operation', 'request_count', 'range', '5m')
```

## 高质量查询设计原则

### 1. 实体优先原则
优先使用Object模式，让UMODEL自动处理底层映射：
```spl
✅ 推荐: .entity_set(...) | entity-call get_metric(...)
❌ 避免: 直接硬编码storage查询
```

### 2. 模糊搜索策略
使用like操作符进行模糊匹配：
```spl
-- 正确的模糊查询
| where name like '%operation%'
| where service_name like 'checkout%'

-- 多条件组合
| where name like '%apm%' and name not like '%test%'
```

### 3. 分层查询策略
```spl
-- 第一步：发现可用数据集
.entity_set(domain='apm', name='apm.operation') 
| entity-call list_data_set(['metric_set'])

-- 第二步：基于发现的数据集查询指标
.entity_set(domain='apm', name='apm.operation') 
| entity-call get_metric('apm', 'apm.metric.apm.operation', 'latency')
```

### 4. 配置驱动开发
利用配置管理减少重复参数：
```python
# 初始化配置
cmsv2_init_config(workspace_name='apm', region_id='cn-hangzhou')

# 后续调用自动使用配置
cmsv2_list_data_sets(domain='apm', entity_type='apm.operation')
```

## 常用查询模式

### 1. 实体发现模式
```spl
-- 模糊搜索相关实体
.entity with(domain='apm', type='service', query='checkout', topk=10)

-- 获取实体详细信息
.entity_set(domain='apm', name='apm.service', ids=['service-001'])
```

### 2. 数据集探索模式
```spl
-- 探索可用数据集
.entity_set(domain='apm', name='apm.operation') 
| entity-call list_data_set(['metric_set'], true)
| project name, fields_mapping

-- 过滤特定模式的数据集
| where name like '%latency%' or name like '%request%'
```

### 3. 关系分析模式
```spl
-- 分析服务调用关系
.entity_set(domain='apm', name='apm.service') 
| entity-call list_related_entity_set(['calls'], 'out', true)

-- 分析完整调用链
.entity_set(domain='apm', name='apm.operation') 
| entity-call list_related_entity_set(['calls'], 'both', true)
```

### 4. 多维度分析模式
```spl
-- 按多维度聚合指标
.metric_set with(
    domain='apm',
    name='apm.metric.apm.operation', 
    source='metrics',
    metric='request_count',
    aggregate_labels=['service_id', 'region', 'status']
)
| stats sum(__value__) by service_id, region, status
```

## 性能优化技巧

### 1. 合理使用聚合
```spl
-- 优化前：过多维度
aggregate_labels=['host', 'pod', 'container', 'service', 'method', 'status']

-- 优化后：核心维度
aggregate_labels=['service', 'status']
```

### 2. 时间窗口控制
```spl
-- 大时间窗口用较大step
step='5m'  // 查询1天以上数据

-- 小时间窗口用较小step  
step='30s' // 查询1小时内数据
```

### 3. 结果集限制
```spl
-- 使用topk限制结果
| limit 100

-- 使用过滤减少数据量
| where service_id in ('core-service', 'api-service')
```

### 4. 批量查询
```spl
-- 批量查询多个实体
.entity_set(domain='apm', name='apm.operation', ids=['op-1', 'op-2', 'op-3'])
| entity-call get_metric(...)
```

## 常见错误和解决方案

### 1. 存储映射问题
```
错误: "MetricSet 不存在"
解决: 检查domain和name参数，确认UMODEL定义存在
```

### 2. 字段映射问题
```
错误: "字段不可过滤"
解决: 使用list_data_set查看可过滤字段列表
```

### 3. 权限问题
```
错误: "无访问权限"
解决: 确认workspace配置和用户权限
```

### 4. 查询超时
```
错误: "查询超时"
解决: 减少时间窗口、增加step、使用过滤条件
```

## 最佳实践总结

### Do's (推荐做法)
1. ✅ 使用Object模式进行业务导向的查询
2. ✅ 利用实体自动映射简化查询逻辑
3. ✅ 使用like操作符进行模糊搜索
4. ✅ 分步骤查询：先发现再查询
5. ✅ 合理设置聚合维度和时间粒度
6. ✅ 使用配置管理避免重复参数

### Don'ts (避免做法)
1. ❌ 硬编码底层存储查询
2. ❌ 使用过多聚合维度影响性能
3. ❌ 查询过大时间窗口而不限制结果
4. ❌ 忽略字段映射关系
5. ❌ 不使用过滤条件直接查询全量数据

### 调试技巧
1. 使用`dry_run`模式查看生成的底层查询
2. 使用`__list_method__`探索可用功能
3. 使用`list_data_set`了解数据结构
4. 逐步构建复杂查询，避免一次性写太复杂

## 工具集成示例

### MCP工具调用
```json
{
  "tool": "cmsv2_list_data_sets",
  "arguments": {
    "domain": "apm",
    "entity_type": "apm.operation",
    "data_set_types": ["metric_set"],
    "name_filter": "latency",
    "detail": true,
    "topk": 20
  }
}
```

### 典型工作流
1. **配置初始化**: `cmsv2_init_config`
2. **实体发现**: `cmsv2_search_entities`  
3. **数据集探索**: `cmsv2_list_data_sets`
4. **指标查询**: `cmsv2_query_metric`
5. **关系分析**: `cmsv2_list_entity_relations`

通过遵循这些原则和实践，可以充分发挥UMODEL的优势，构建高效、可维护的可观测查询。 