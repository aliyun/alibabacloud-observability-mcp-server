# UModel-Assistant 用户使用指南

## 概述

UModel-Assistant 是一个将 PaaS 层 SPL 查询转换为底层可执行 SPL 的服务。它支持两种模式：
- **Phase 1 Table 模式**：直接访问数据集（DataSet）
- **Phase 2 Object 模式**：通过实体（EntitySet）访问相关数据

## Phase 1 Table 模式

### 1. MetricSet 查询

MetricSet 支持两种数据源：指标数据（metrics）和标签值（labels）。

#### 1.1 指标查询（metrics）

**SPL 格式：**
```spl
.metric_set with(domain='域名', name='数据集名称', source='metrics', metric='指标名', [其他可选参数]) | [后续 Pipeline]
```

**参数说明：**
- `domain` (必填): MetricSet 所属域
- `name` (必填): MetricSet 名称
- `source` (必填): 固定为 'metrics'
- `metric` (必填): 要查询的指标名称
- `query_type` (可选): 查询类型，'range'（默认）或 'instant'
- `step` (可选): 时间步长，如 '1m'、'5m' 等
- `aggregate` (可选): 是否聚合，true/false
- `aggregate_labels` (可选): 聚合维度列表，如 ['host', 'service']

**示例 1：PromQL 模式查询**
```spl
.metric_set with(domain='apm', name='apm.metric.apm.operation', source='metrics', metric='avg_request_latency_ms', step='1m', aggregate_labels=['host']) | where service_id = 'xxx'
```

**返回的 SPL：**
```spl
.metricstore with(project='cn-hangzhou', metricstore='umodel-test') | prom-call promql_query_range('sum by (host) (...)', '1m') | ...
```

**示例 2：SPL 模式查询**
```spl
.metric_set with(domain='rum', name='rum.metric.api', source='metrics', metric='api_request_duration', step='1m', aggregate_labels=['service_id']) | where service_id = 'xxx'
```

**返回的 SPL：**
```spl
.logstore with(project='$project', logstore='logstore-rum', query='...') | extend __ts__ = second_to_nano(...) | stats __value__ = avg(...) by __ts__, service_id | ...
```

#### 1.2 标签值查询（labels）

**SPL 格式：**
```spl
.metric_set with(domain='域名', name='数据集名称', source='labels', label='标签名') | [where 条件]
```

**参数说明：**
- `domain` (必填): MetricSet 所属域
- `name` (必填): MetricSet 名称
- `source` (必填): 固定为 'labels'
- `label` (必填): 要查询的标签名称

**示例：**
```spl
.metric_set with(domain='apm', name='apm.metric.apm.operation', source='labels', label='rpc') | where service_id = 'xxx'
```

**返回的 SPL：**
```spl
.metricstore with(project='cn-hangzhou', metricstore='umodel-test') | prom-call promql_label_values('operation', '...')
```

### 2. 其他 DataSet 类型

LogSet、TraceSet、EventSet 等其他数据集类型的支持正在开发中。

## Phase 2 Object 模式

### 1. 基本语法

**SPL 格式：**
```spl
.entity_set(domain='域名', name='实体名称', [ids=['id1', 'id2']]) | entity-call 方法名(参数...)
```

### 2. 内置方法

#### 2.1 列出可用方法 (__list_method__)

获取当前 EntitySet 支持的所有方法。

**示例：**
```spl
.entity_set(domain='apm', name='apm.operation') | entity-call __list_method__()
```

**返回结果：**
| name | display_name | description | params | returns |
|------|-------------|-------------|---------|---------|
| __list_method__ | List Available Methods | Get all methods supported by current EntitySet | [] | [...] |
| list_data_set | List DataSets | Get DataSets related to EntitySet | [...] | [...] |
| get_metric | Get Metric | Get specified metric data from a MetricSet | [...] | [...] |
| ... | ... | ... | ... | ... |

#### 2.2 列出相关数据集 (list_data_set)

获取 EntitySet 关联的所有数据集。

**参数：**
- `data_set_types` (可选): 数据集类型列表，如 ['metric_set', 'log_set']
- `detail` (可选): 是否返回详细信息，默认 false

**示例：**
```spl
.entity_set(domain='apm', name='apm.operation') | entity-call list_data_set(['metric_set'], true)
```

**返回结果：**
| data_set_id | type | domain | name | fields_mapping | fields |
|-------------|------|--------|------|----------------|---------|
| apm@metric_set@apm.metric.apm.operation | metric_set | apm | apm.metric.apm.operation | {...} | [...] |

#### 2.3 列出相关实体 (list_related_entity_set)

获取与当前 EntitySet 相关的其他实体。

**参数：**
- `relations` (可选): 关系类型列表，如 ['contains', 'calls']
- `direction` (可选): 方向，'in'/'out'/'both'（默认）
- `detail` (可选): 是否返回详细信息，默认 true

**示例：**
```spl
.entity_set(domain='apm', name='apm.operation') | entity-call list_related_entity_set(['calls'], 'out', true)
```

**返回结果：**
| entity_set_id | relation | direction | domain | name | entity_link_detail |
|---------------|----------|-----------|--------|------|--------------------|
| apm@entity_set@apm.instance | calls | out | apm | apm.instance | {...} |
| apm@entity_set@apm.host | calls | out | apm | apm.host | {...} |

### 3. DataSet 方法

根据 EntitySet 关联的数据集类型，自动提供相应的方法。

#### 3.1 MetricSet 方法

##### get_metric - 获取指标数据

**参数：**
- `domain` (必填): MetricSet 域名
- `name` (必填): MetricSet 名称  
- `metric` (必填): 指标名称
- `query_type` (可选): 查询类型，'range'（默认）或 'instant'
- `step` (可选): 时间步长

**示例：**
```spl
.entity_set(domain='apm', name='apm.operation', ids=['op-001', 'op-002']) | entity-call get_metric('apm', 'apm.metric.apm.operation', 'request_count', 'range', '5m')
```

**返回的 SPL：**
自动根据 MetricSet 类型（PromQL 或 SPL）生成相应的查询，并自动处理实体 ID 到存储字段的映射。

##### get_label_values - 获取标签值

**参数：**
- `domain` (必填): MetricSet 域名
- `name` (必填): MetricSet 名称
- `label` (必填): 标签名称

**示例：**
```spl
.entity_set(domain='apm', name='apm.operation') | entity-call get_label_values('apm', 'apm.metric.apm.operation', 'rpc')
```

##### get_golden_metrics - 获取黄金指标

获取 EntitySet 相关的所有黄金指标（标记为 golden=true 的指标）。

**注意：** 该功能正在开发中，即将支持。

**参数：**
- `query_type` (可选): 查询类型，'range'（默认）或 'instant'
- `step` (可选): 时间步长

**示例：**
```spl
.entity_set(domain='apm', name='apm.operation') | entity-call get_golden_metrics()
```

## 高级特性

### 1. Where 条件过滤

两种模式都支持 where 条件过滤：

```spl
# Phase 1
.metric_set with(...) | where service_id = 'xxx' and status = 'success'

# Phase 2
.entity_set(...) | where service_id = 'xxx' | entity-call get_metric(...) 
```

### 2. 变量引用语法

在 UModel 定义中的过滤器（Filter）字段支持变量引用语法：

**支持的语法格式：**
- `${变量名}` - 基础变量引用
- `${变量名|默认值}` - 带默认值的变量引用
- `${{变量名|默认值}}` - 双花括号格式（与单花括号功能相同）

**示例：**
```yaml
# 在 UModel 的 labels filter 中使用
filter: 'service_id="${service_id}" and region="${region|cn-hangzhou}"'
```

当执行查询时，系统会：
1. 从 where 条件中提取对应的变量值
2. 替换 filter 中的变量引用
3. 如果变量不存在，使用默认值（如果提供）

### 3. EntityData 支持

当通过 EntityData 传入实体信息时，系统会自动：
1. 根据 DataLink 的字段映射关系转换字段名
2. 生成相应的过滤条件
3. 只使用可过滤字段（在 DataSet 中标记为 filterable 的字段）

**EntityData 格式：**
```json
{
  "version": 1,
  "header": ["entity_id", "service", "operation", "type"],
  "data": [
    ["op-001", "user-service", "GET /api/users", "HTTP"],
    ["op-002", "order-service", "createOrder", "RPC"]
  ]
}
```

**字段映射处理流程：**
1. **DataLink 中的 fields_mapping**：定义实体字段到存储字段的映射
   ```yaml
   fields_mapping:
     "app.id": "service_id"        # 存储中的 app.id 对应实体的 service_id
     "resource.name": "api_name"
   ```

2. **自动过滤条件生成**：
   - 如果字段在 `fields_mapping` 中，使用映射后的字段名
   - 如果字段在 DataSet 的可过滤字段列表中，直接使用原字段名
   - 单个值使用 `=` 操作符，多个值使用 `IN` 操作符

3. **示例转换**：
   ```
   EntityData: service = ["user-service", "order-service"]
   fields_mapping: {"app.id": "service"}
   生成的过滤条件: app.id IN ('user-service', 'order-service')
   ```

### 4. 运行模式

支持 dry_run 模式，只返回生成的查询而不执行：

```spl
set umodel_paas_mode='dry_run';
.metric_set with(...) | ...
```

## 错误处理

常见错误及解决方法：

1. **"MetricSet 不存在"**
   - 检查 domain 和 name 参数是否正确
   - 确认 UModel 定义中存在该 MetricSet

2. **"未找到存储"**
   - 检查 MetricSet 是否配置了 StorageLink
   - 如有多个存储，需要指定 storage_name 参数

3. **"方法不被支持"**
   - 使用 __list_method__ 查看可用方法
   - 确认 EntitySet 关联了相应类型的 DataSet

## 最佳实践

1. **使用 __list_method__ 探索功能**
   ```spl
   .entity_set(domain='apm', name='apm.operation') | entity-call __list_method__()
   ```

2. **先查看数据集再查询数据**
   ```spl
   .entity_set(domain='apm', name='apm.operation') | entity-call list_data_set(['metric_set'])
   ```

3. **利用 EntityData 批量查询**
   - 传入多个实体 ID 和属性
   - 系统自动生成优化的过滤条件

4. **合理使用聚合维度**
   - 根据实际需求选择 aggregate_labels
   - 避免过多维度导致性能问题 