# PaaS API工具SPL语句示例

本文档提供了所有PaaS API工具对应的SPL语句示例，基于实际代码实现，便于开发者理解和使用。

## 实体管理工具包

### 1. umodel_get_entities - 获取实体基础信息

```sql
# 基础查询（获取前20个实体）
.entity_set with(domain='apm', name='apm.service') | entity-call get_entities() | limit 20

# 精确ID查询（带多个实体ID）
.entity_set with(domain='apm', name='apm.service', ids=['service-1','service-2']) | entity-call get_entities() | limit 20

# 获取更多实体（最大1000）
.entity_set with(domain='apm', name='apm.service') | entity-call get_entities() | limit 100

# 获取基础设施实体
.entity_set with(domain='infrastructure', name='host.instance') | entity-call get_entities() | limit 50

# 获取K8S Pod实体
.entity_set with(domain='k8s', name='k8s.pod') | entity-call get_entities() | limit 30
```

### 2. umodel_search_entities - 基于关键词搜索实体

```sql
# 在特定域内搜索（注意：使用.entity，不是.entity_set）
.entity with(domain='apm', name='apm.service', query='payment') | limit 20

# 跨域搜索所有实体类型
.entity with(domain='*', name='*', query='nginx') | limit 50

# 搜索主机实例
.entity with(domain='infrastructure', name='host.instance', query='192.168.1') | limit 100

# 搜索容器实例
.entity with(domain='k8s', name='k8s.container', query='mysql') | limit 30
```

### 3. umodel_get_neighbor_entities - 获取邻居实体

```sql
# 获取指定实体的邻居（注意：单个实体ID用字符串，不是数组）
.entity_set with(domain='apm', name='apm.service', ids=['service-123']) | entity-call get_neighbor_entities() | limit 20

# 获取主机的邻居实体
.entity_set with(domain='infrastructure', name='host.instance', ids=['host-456']) | entity-call get_neighbor_entities() | limit 50

# 获取更多邻居实体
.entity_set with(domain='apm', name='apm.service', ids=['payment-service']) | entity-call get_neighbor_entities() | limit 100
```

## 数据集管理工具包

### 4. umodel_get_entity_set - 获取实体集合架构定义

```sql
# 获取实体集合架构信息
.entity_set with(domain='apm', name='apm.service') | entity-call get_entity_set()

# 获取基础设施实体架构
.entity_set with(domain='infrastructure', name='host.instance') | entity-call get_entity_set()
```

### 5. umodel_list_data_set - 列出可用数据集合

```sql
# 获取指标数据集
.entity_set with(domain='apm', name='apm.service') | entity-call list_data_set(['metric_set'])

# 获取所有数据集类型
.entity_set with(domain='apm', name='apm.service') | entity-call list_data_set()

# 获取日志和事件数据集
.entity_set with(domain='apm', name='apm.service') | entity-call list_data_set(['log_set', 'event_set'])
```

### 6. umodel_search_entity_set - 搜索实体集合

```sql
# 搜索包含"service"的实体集合
.entity_set with(domain='apm', name='apm.service', query='service') | entity-call search_entity_set()

# 跨域搜索实体集合
.entity_set with(domain='*', name='*', query='application') | entity-call search_entity_set()
```

### 7. umodel_list_related_entity_set - 列出相关实体集合

```sql
# 获取所有关系（详细信息）
.entity_set with(domain='apm', name='apm.service') | entity-call list_related_entity_set(null, 'both', true)

# 获取上游依赖关系（简要信息）
.entity_set with(domain='apm', name='apm.service') | entity-call list_related_entity_set('dependency', 'upstream', false)

# 获取下游调用关系
.entity_set with(domain='apm', name='apm.service') | entity-call list_related_entity_set('call', 'downstream', true)

# 获取网络关系
.entity_set with(domain='k8s', name='k8s.pod') | entity-call list_related_entity_set('network', 'both', false)
```

## 数据查询工具包

### 8. umodel_get_metrics - 获取时序指标数据

```sql
# 获取CPU使用率指标
.entity_set with(domain='apm', name='apm.service') | entity-call get_metric('apm', 'system', 'cpu.usage', 'range', '1m')

# 指定实体ID获取指标
.entity_set with(domain='apm', name='apm.service', ids=['service-1']) | entity-call get_metric('apm', 'system', 'cpu.usage', 'range', '1m')

# 获取内存指标
.entity_set with(domain='apm', name='apm.service', ids=['service-1']) | entity-call get_metric('apm', 'system', 'memory.usage', 'range', '5m')

# 获取响应时间指标
.entity_set with(domain='apm', name='apm.service') | entity-call get_metric('apm', 'application', 'response_time', 'instant', null)
```

### 9. umodel_get_golden_metrics - 获取黄金指标

```sql
# 获取所有服务的黄金指标
.entity_set with(domain='apm', name='apm.service') | entity-call get_golden_metrics()

# 指定实体的黄金指标
.entity_set with(domain='apm', name='apm.service', ids=['service-1','service-2']) | entity-call get_golden_metrics()

# 获取主机黄金指标
.entity_set with(domain='infrastructure', name='host.instance') | entity-call get_golden_metrics()
```

### 10. umodel_get_relation_metrics - 获取关系级指标

```sql
# 获取调用关系的响应时间
.entity_set with(domain='apm', name='apm.service') | entity-call get_relation_metrics('call', 'response_time')

# 指定实体的依赖关系错误率
.entity_set with(domain='apm', name='apm.service', ids=['service-1']) | entity-call get_relation_metrics('dependency', 'error_rate')

# 获取网络关系指标
.entity_set with(domain='k8s', name='k8s.pod', ids=['pod-123']) | entity-call get_relation_metrics('network', 'throughput')
```

### 11. umodel_get_logs - 获取日志数据

```sql
# 获取应用日志
.entity_set with(domain='apm', name='apm.service') | entity-call get_logs('application_log')

# 指定实体的错误日志
.entity_set with(domain='apm', name='apm.service', ids=['service-1']) | entity-call get_logs('error_log')

# 获取系统日志
.entity_set with(domain='infrastructure', name='host.instance', ids=['host-1']) | entity-call get_logs('system_log')
```

### 12. umodel_get_events - 获取事件数据

```sql
# 获取所有事件
.entity_set with(domain='apm', name='apm.service') | entity-call get_events()

# 带过滤条件获取高严重性事件
.entity_set with(domain='apm', name='apm.service', ids=['service-1']) | entity-call get_events() | where "severity"='high'

# 获取部署事件
.entity_set with(domain='apm', name='apm.service') | entity-call get_events() | where "event_type"='deployment'

# 获取告警事件
.entity_set with(domain='apm', name='apm.service', ids=['service-1']) | entity-call get_events() | where "event_type"='alert'
```

### 13. umodel_get_traces - 获取链路追踪数据

```sql
# 根据trace_id获取链路详情
.entity_set with(domain='apm', name='apm.service') | entity-call get_traces('trace-123456')

# 获取特定链路
.entity_set with(domain='apm', name='apm.service') | entity-call get_traces('trace-789012')
```

### 14. umodel_search_traces - 搜索链路追踪数据

```sql
# 搜索所有链路
.entity_set with(domain='apm', name='apm.service') | entity-call search_traces()

# 搜索超过1秒的慢链路
.entity_set with(domain='apm', name='apm.service', ids=['service-1']) | entity-call search_traces() | where "duration">1000

# 搜索错误链路
.entity_set with(domain='apm', name='apm.service') | entity-call search_traces() | where "status"='error'

# 搜索指定服务的链路
.entity_set with(domain='apm', name='apm.service', ids=['service-payment']) | entity-call search_traces() | where "duration">500
```

### 15. umodel_get_profiles - 获取性能剖析数据

```sql
# 获取CPU性能剖析
.entity_set with(domain='apm', name='apm.service') | entity-call get_profiles('cpu')

# 指定实体的内存剖析
.entity_set with(domain='apm', name='apm.service', ids=['service-1']) | entity-call get_profiles('memory')

# 获取堆内存剖析
.entity_set with(domain='apm', name='apm.service', ids=['service-java']) | entity-call get_profiles('heap')

# 获取I/O剖析
.entity_set with(domain='apm', name='apm.service', ids=['service-1']) | entity-call get_profiles('io')
```

## 通用SPL语句模式

所有PaaS API工具都遵循以下统一模式：

```sql
.entity_set with(domain='<实体域>', name='<实体类型>' [, ids=['<实体ID1>', '<实体ID2>']] [, query='<搜索关键词>']) 
| entity-call <函数名>([参数1, 参数2, ...]) 
[| limit <数量>] 
[| where <过滤条件>]
```

### 参数说明：

- **domain**: 实体域，如 `apm`、`infrastructure`、`k8s` 等
- **name**: 实体类型，如 `apm.service`、`host.instance`、`k8s.pod` 等  
- **ids**: 可选的实体ID数组，用于精确查询指定实体
- **query**: 可选的搜索关键词，用于模糊搜索
- **limit**: 限制返回结果数量
- **where**: 添加过滤条件

### 常用过滤条件示例：

```sql
# 数值比较
| where "cpu_usage">80
| where "response_time"<=1000  
| where "memory_usage">="50%"

# 字符串匹配
| where "service_name"='payment-service'
| where "status"='error'
| where "severity"='high'

# 时间范围（通常在函数参数中指定）
| where "timestamp">='2023-01-01' and "timestamp"<'2023-02-01'

# 复合条件
| where "cpu_usage">80 and "status"='running'
| where "error_rate">0.1 or "response_time">2000
```

这些SPL语句示例涵盖了PaaS API工具的所有常见使用场景，您可以根据具体需求进行调整和组合使用。