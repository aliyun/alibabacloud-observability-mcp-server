---
description: This rule provides a comprehensive reference for SPL (Search Processing Language) queries in EntityStore/USearch systems.  如果查询可观测SPL语句或者UMODEL相关概念比如实体，关系等，可以使用它
globs: 
alwaysApply: false
---
# SPL查询参考手册

This rule provides a comprehensive reference for SPL (Search Processing Language) queries in EntityStore/USearch systems. 

## Rule Purpose
- **查询语法参考**: 提供完整的SPL语法规范和使用方法
- **数据源覆盖**: 支持.entity、.topo、.umodel、.logstore、.metricstore等所有数据源
- **实际业务场景**: 包含基于真实业务场景的查询模板和最佳实践
- **性能优化指导**: 提供查询优化策略和注意事项

## When to Use This Rule
- 需要构建SPL查询语句时
- 分析实体关系和拓扑结构时  
- 进行模型数据分析时
- 优化查询性能时
- 解决复杂的多数据源联合分析问题时

## Key Features
- **语法完整性**: 覆盖所有SPL操作符和数据源
- **场景化模板**: 按业务场景分类的即用查询模板
- **实例丰富**: 基于真实服务名和实际业务场景的示例
- **最佳实践**: 包含性能优化和常见陷阱避免指南

## 目录
1. [基础语法](mdc:#基础语法)
2. [实体查询(.entity/USearch)](mdc:#实体查询entityusearch)
3. [图查询(.topo)](mdc:#图查询topo)
4. [模型查询(.umodel)](mdc:#模型查询umodel)
5. [日志查询(.logstore)](mdc:#日志查询logstore)
6. [指标查询(.metricstore)](mdc:#指标查询metricstore)
7. [SPL操作符](mdc:#spl操作符)
8. [常用查询模板](mdc:#常用查询模板)
9. [复杂分析场景](mdc:#复杂分析场景)
10. [性能优化和注意事项](mdc:#性能优化和注意事项)

---

## 基础语法

### 数据源类型
```spl
.entity      # 实体数据查询(USearch)
.topo        # 拓扑关系数据查询
.umodel      # 模型数据查询
.logstore    # 日志数据查询
.metricstore # 指标数据查询
```

### 管道操作
```spl
数据源 | 操作1 | 操作2 | 操作3 ...
```

### 字段引用规则
- **普通字段**: 直接使用字段名
- **特殊字符字段**: 用双引号 `"字段名"`
- **嵌套字段**: `json_extract_scalar(字段, '$.路径')`
- **对象属性**: `对象.属性`

### ⚠️ 重要语法规则
**数据源不能混用**：
- **实体查询**: 必须以`.entity`开头，用于查询实体数据
- **拓扑查询**: 必须以`.topo`开头，用于查询关系数据
- **模型查询**: 必须以`.umodel`开头，用于查询模型定义
- **✅ 正确**: 分步查询 - 先用`.entity`获取实体ID，再用`.topo`查询关系

---

## 实体查询(.entity/USearch)

### 数据模型
实体数据采用三层结构存储：
1. **workspace**: 不同workspace相互隔离
2. **domain**: 域层级
3. **entity_type**: 实体类型层级

在每个entity_type下，`__entity_id__`列保证唯一性。

### 基本语法
```spl
.entity with(参数列表) [| SPL操作...]
```

### 参数详解

#### 必需参数
- **domain**: 域名过滤，支持fnmatch通配符语法
- **type**: 实体类型过滤，支持fnmatch通配符语法

#### 可选参数
- **entityIds**: 逗号分隔的实体ID列表（精确匹配）
- **query**: 搜索条件（全文搜索、key:value等）
- **topk**: 返回结果数量限制（默认100）
- **groupTopk**: 每个entity_type的最大数据量
- **columns**: 指定返回字段列表（扫描模式）

### 查询模式

#### 1. 检索模式（搜索模式）
用于多类型数据联合检索，支持打分排序。

```spl
# 精确ID查询
.entity with(domain='apm', type='apm.service', entityIds='实体ID1,实体ID2')

# 全文搜索
.entity with(domain='apm', type='apm.service', query='服务名关键词', topk=50)

# 多类型联合查询
.entity with(domain='a*', type='*service', query='关键词')

# Key-Value搜索
.entity with(domain='apm', type='apm.service', query='service: "具体服务名"')

# 短语搜索（处理连字符连接的词）
.entity with(domain='*', type='*', query='cms-cloud-d-user-sls-test')

# 逻辑条件查询
.entity with(query='desc: "ecs for rag" or instance_id')
.entity with(query='desc: "ecs for rag" and region: "cn-hangzhou"')
.entity with(query='desc: "ecs for rag" not instance_id: "deprecated"')
```

#### 2. 扫描模式
用于读取指定domain/entity_type的所有数据，主要用于SPL计算。

```spl
# 获取所有数据
.entity with(domain='apm', type='apm.service')

# 指定返回字段（性能优化）
.entity with(domain='apm', type='apm.service', columns=['__entity_id__', 'service', 'host'])

# 多类型扫描
.entity with(domain='*', type='acs*', columns=['__entity_id__', 'desc'])
```

### Query语法详解

#### 1. 全文搜索
```spl
# 多词or关系
.entity with(domain='*', type='*', query='ecs for rag')  # 包含ecs OR for OR rag

# 特殊字符必须用双引号
.entity with(domain='*', type='*', query='"ecs|for:rag"')
```

#### 2. 字段搜索
```spl
# 单字段搜索
.entity with(domain='*', type='*', query='desc: "ecs for rag"')

# 多字段搜索
.entity with(domain='*', type='*', query='service: "web" and region: "cn-hangzhou"')
```

#### 3. 逻辑操作符
```spl
# AND: 同时满足
.entity with(query='service: "web" and region: "cn-hangzhou"')

# OR: 满足任一条件
.entity with(query='service: "web" or service: "api"')

# NOT: 排除条件
.entity with(query='service: "web" not status: "deprecated"')
```

### 输出字段
检索模式会额外附加两个字段：
- **__score__**: 搜索评分（相关度）
- **__query__**: 查询条件

### 常用查询模板

#### APM域查询
```spl
# 查询特定服务
.entity with(domain='apm', type='apm.service', query='service: "shippingservice"')

# 查询服务实例
.entity with(domain='apm', type='apm.instance', query='host: "主机名"')

# 查询服务操作
.entity with(domain='apm', type='apm.service.operation', query='操作名')

# 按语言类型查询服务
.entity with(domain='apm', type='apm.service', query='language: "golang"')
```

#### ACS域查询
```spl
# 查询ECS实例
.entity with(domain='acs', type='acs.ecs.instance', query='instance_name: "实例名"')

# 查询VPC
.entity with(domain='acs', type='acs.vpc.vpc', query='vpc_name: "VPC名"')

# 查询K8S集群
.entity with(domain='acs', type='acs.ack.cluster', query='cluster_name: "集群名"')
```

#### 跨域联合查询
```spl
# 查询所有包含特定关键词的实体
.entity with(domain='*', type='*', query='shippingservice', topk=100)

# 按实体类型统计
.entity with(domain='*', type='*', query='kubernetes') 
| stats cnt=count(1) by __entity_type__ 
| sort cnt desc
```

---

## 图查询(.topo)

### 基础概念
拓扑数据表示实体间的关系，支持有向和无向图查询。

### 节点和边描述
```spl
# 节点格式
(变量名:"domain@entity_type" {属性: '值'})

# 边格式  
[变量名:关系类型 {属性: '值'}]

# 路径格式
(起点)-[边]->(终点)    # 有向
(起点)-[边]-(终点)     # 无向
(起点)<-[边]-(终点)    # 反向
```

### graph-match 语法
```spl
.topo | graph-match 路径模式 project 输出字段 [| SPL操作...]
```

#### 基础查询模板
```spl
# 查询直接邻居
.topo | graph-match (s:"apm@apm.service" {__entity_id__: '实体ID'})-[e]-(d)

# 查询特定关系类型
.topo | graph-match (s:"apm@apm.service" {__entity_id__: 'ID'})-[e:calls]->(d)

# 查询指定类型的邻居
.topo | graph-match (s:"apm@apm.service" {__entity_id__: 'ID'})-[e:consists_of]-(d:"apm@apm.instance")

# 多跳查询
.topo | graph-match (s:"apm@apm.service" {__entity_id__: 'ID'})-[e1]-(v1)-[e2:runs_on]->(v2)
```

#### 高级图查询
```spl
# 链路分析（上游下游）
.topo | graph-match (s:"apm@apm.service.operation" {__entity_id__: 'ID'})<-[e1]-(v1)-[e2]->(v2)

# 统计邻居类型分布
.topo | graph-match (s:"apm@apm.service" {__entity_id__: 'ID'})-[e]-(d) 
      | stats cnt=count(1) by "e.__type__", "d.__label__"
      | sort cnt desc
```

### graph-call 函数

#### getNeighborNodes
```spl
.topo | graph-call getNeighborNodes(方向类型, 深度, 节点列表)
```

**参数说明**：
- **方向类型**: `'sequence'`(推荐), `'sequence_in'`(入向), `'sequence_out'`(出向), `'full'`(全向)
- **深度**: 遍历层数，通常使用1
- **节点列表**: 起始节点数组

**⭐ 最佳实践**：
- **推荐用法**: `'sequence'` + 深度1 - 性能最好，覆盖度适中
- **语法简洁**: 直接使用`(:"domain@entity_type" {__entity_id__: 'ID'})`，无需变量名

**基础语法模板**：
```spl
# ⭐ 推荐：简洁的邻居查询
.topo | graph-call getNeighborNodes('sequence', 1, [
    (:"apm@apm.operation" {__entity_id__: '1a8d2460f88cb1970db2ad39b1814ded'})
])

# 查询APM服务的关联
.topo | graph-call getNeighborNodes('sequence', 1, [
    (:"apm@apm.service" {__entity_id__: '实体ID'})
])

# 查询APM实例的关联
.topo | graph-call getNeighborNodes('sequence', 1, [
    (:"apm@apm.instance" {__entity_id__: '实体ID'})
])
```

**常用实体类型查询**：
```spl
# APM接口下游关联
.topo | graph-call getNeighborNodes('sequence', 1, [
    (:"apm@apm.operation" {__entity_id__: '接口实体ID'})
])

# APM服务下游关联
.topo | graph-call getNeighborNodes('sequence', 1, [
    (:"apm@apm.service" {__entity_id__: '服务实体ID'})
])

# ACS资源关联
.topo | graph-call getNeighborNodes('sequence', 1, [
    (:"acs@acs.ecs.instance" {__entity_id__: 'ECS实体ID'})
])
```

**返回字段**：
- **srcNode**: 起始节点信息（JSON格式）
- **destNode**: 目标节点信息（JSON格式）
- **relation**: 关系信息（JSON格式）
- **distance**: 距离层级

**简单的字段提取**：
```spl
# 基础查询（无需复杂处理）
.topo | graph-call getNeighborNodes('sequence', 1, [
    (:"apm@apm.operation" {__entity_id__: '实体ID'})
])

# 如需字段提取，使用json_extract_scalar
.topo | graph-call getNeighborNodes('sequence', 1, [
    (:"apm@apm.operation" {__entity_id__: '实体ID'})
])
| extend dest_type = json_extract_scalar(destNode, '$.__entity_type__'),
         relation_type = json_extract_scalar(relation, '$.__type__')
| project dest_type, relation_type, distance
```

#### getDirectRelations
```spl
.topo | graph-call getDirectRelations(节点列表)
```

```spl
# 查询多个节点间的直接关系
.topo | graph-call getDirectRelations([
    (:"apm@apm.service" {__entity_id__: 'ID1'}),
    (:"apm@apm.service" {__entity_id__: 'ID2'})
])
```

### Cypher 查询
```spl
.topo | graph-call cypher(`Cypher语句` [, 'pure-topo'])
```

#### 基础三段式
```spl
.topo | graph-call cypher(`
    MATCH (节点模式)-[边模式]->(节点模式)
    WHERE 筛选条件
    RETURN 返回字段
`)
```

#### 常用Cypher模板
```spl
# 基础节点查询
.topo | graph-call cypher(`
    MATCH (n:``apm@apm.service``)
    WHERE n.__domain__ = 'apm'
    RETURN n
`)

# 指定属性查询
.topo | graph-call cypher(`
    MATCH (s:``apm@apm.service`` {service:'ad-recommend-gin-server'})-[e:consists_of]-(d) 
    RETURN s,e,d
`)

# 多级跳查询
.topo | graph-call cypher(`
    MATCH (src:``apm@apm.service``)-[e:calls*2..3]->(dest)
    WHERE dest.__domain__ = 'apm'
    RETURN src, dest, dest.__entity_type__
`)

# 聚合统计
.topo | graph-call cypher(`
    MATCH (s:``apm@apm.service``)-[e:calls]->(d:``apm@apm.service``)
    RETURN s.service as service, count(d) as dependency_count
    ORDER BY dependency_count DESC
`)

# 路径长度分析
.topo | graph-call cypher(`
    MATCH (s:``apm@apm.service``)-[e:calls*1..5]->(d:``apm@apm.service``)
    RETURN s.service as source, d.service as target, length(e) as path_length
    ORDER BY path_length DESC
`)
```

---

## 模型查询(.umodel)

### 数据模型
UmodelData位于可观测实体数据的上一层，用于描述数据模型定义，也具有图的性质。

### 基本查询
```spl
# 查询所有模型
.umodel

# 分页查询
.umodel | limit 0, 100
```

### 字段结构
每条umodel数据包含以下字段：
- **__type__**: 系统字段，分为`link`和`node`两种类型
- **kind**: 模型类型
- **metadata**: 元数据信息
- **schema**: 模式定义
- **spec**: 规格说明

### 过滤查询
```spl
# 按类型过滤
.umodel | where kind = 'entity_set'
.umodel | where kind = 'entity_set_link'
.umodel | where kind = 'metric_set'
.umodel | where kind = 'sls_metricstore'

# 按系统类型过滤
.umodel | where __type__ = 'link'
.umodel | where __type__ = 'node'

# 按名称过滤
.umodel | where json_extract_scalar(metadata, '$.name') = 'acs.ack.cluster'

# 按域过滤
.umodel | where json_extract_scalar(metadata, '$.domain') = 'acs'
.umodel | where json_extract_scalar(metadata, '$.domain') in ('acs', 'apm')

# 统计各类型数量
.umodel | project kind | stats cnt = count(1) by kind | sort cnt desc
```

### 模型图查询

#### UmodelId格式
UmodelData的唯一标识符格式为：`kind::domain::name`

```spl
# 查询模型关系（基础语法）
.umodel | graph-match (s: "domain@kind" {__entity_id__: 'kind::domain::name'})-[e]-(d)

# 实际示例
.umodel | graph-match (s: "acs@entity_set" {__entity_id__: 'entity_set::acs::acs.ack.cluster'})-[e]-(d)

# 查询MetricSet关系
.umodel | graph-match (s: "apm@metric_set" {__entity_id__: 'metric_set::apm::apm_http_client_dot_metric_set'})-[e]-(d)

# 过滤特定类型的邻居
.umodel | graph-match (s: "acs@metric_set" {__entity_id__: 'metric_set::acs::kube_pod_metric_set'})-[e]-(d: "acs@storage_link")

# 排除特定类型
.umodel | graph-match (s: "acs@metric_set" {__entity_id__: 'metric_set::acs::kube_pod_metric_set'})-[e]-(d) 
        | extend kind = json_extract_scalar(d, '$.kind') 
        | where kind != 'explorer' and kind != 'entity_set'
```

#### 获取存储信息
```spl
# 查询MetricSet的存储链路
.umodel | graph-match (s: "apm@metric_set" {__entity_id__: 'metric_set::apm::apm_http_client_dot_metric_set'})<-[e1]-(v1)-[e2:runs_on]->(v2)-[e3]->(v3)

# 查询SLS MetricStore
.umodel | where kind = 'sls_metricstore'
```

### 链接关系查询（Link类型）

#### 查询MetricSet到SLS MetricStore的映射关系
```spl
# 查询apm域中metric_set到sls_metricstore的链接关系
.umodel | where json_extract_scalar(metadata, '$.domain') in ('apm') 
        and json_extract_scalar(spec, '$.src.kind') in ('metric_set') 
        and json_extract_scalar(spec, '$.dest.kind') in ('sls_metricstore')


# 统计各域的metric_set到metricstore映射数量
.umodel | where json_extract_scalar(spec, '$.src.kind') in ('metric_set') 
        and json_extract_scalar(spec, '$.dest.kind') in ('sls_metricstore')
| stats link_count=count(1) by json_extract_scalar(metadata, '$.domain')
| sort link_count desc

# 查询特定域的所有存储链接关系
.umodel | where json_extract_scalar(metadata, '$.domain') = 'apm' 
        and json_extract_scalar(spec, '$.src.kind') in ('metric_set') 
        and json_extract_scalar(spec, '$.dest.kind') in ('sls_metricstore')
| extend src_full_name = concat(json_extract_scalar(spec, '$.src.domain'), '.', json_extract_scalar(spec, '$.src.name'))
| extend dest_full_name = concat(json_extract_scalar(spec, '$.dest.project'), '/', json_extract_scalar(spec, '$.dest.metricstore'))
```

### 实体存储关联查询 (.let + graph-match 模式)

#### ⭐ 标准语法模板
```spl
.let umodelDest = .umodel 
| graph-match (entity_set: "域@entity_set" {__entity_id__: 'entity_set::域::实体类型'})-[e]-(metric_set: "域@metric_set")-[e2]->(storage) 
  project storage
| parse-json storage 
| project id; 
.umodel 
| join $umodelDest on concat(kind, '::', json_extract_scalar(metadata, '$.domain'), '::', json_extract_scalar(metadata, '$.name')) = $umodelDest.id
```

#### 核心语法要素
1. **`.let`子查询**: 定义可重用的查询变量
2. **图路径查询**: `entity_set -> metric_set -> storage` 三级关联
3. **parse-json**: 解析图查询返回的JSON节点
4. **UmodelId格式**: `'entity_set::domain::name'` 标准格式
5. **join条件**: 使用`concat(kind, '::', domain, '::', name)`拼接完整ID

#### 实际应用示例

```spl
# 查询apm.service实体对应的存储
.let umodelDest = .umodel 
| graph-match (entity_set: "apm@entity_set" {__entity_id__: 'entity_set::apm::apm.service'})-[e]-(metric_set: "apm@metric_set")-[e2]->(storage) 
  project storage
| parse-json storage 
| project id; 
.umodel 
| join $umodelDest on concat(kind, '::', json_extract_scalar(metadata, '$.domain'), '::', json_extract_scalar(metadata, '$.name')) = $umodelDest.id

# 查询apm.operation实体对应的存储
.let umodelDest = .umodel 
| graph-match (entity_set: "apm@entity_set" {__entity_id__: 'entity_set::apm::apm.operation'})-[e]-(metric_set: "apm@metric_set")-[e2]->(storage) 
  project storage
| parse-json storage 
| project id; 
.umodel 
| join $umodelDest on concat(kind, '::', json_extract_scalar(metadata, '$.domain'), '::', json_extract_scalar(metadata, '$.name')) = $umodelDest.id

# 查询acs.ecs.instance实体对应的存储
.let umodelDest = .umodel 
| graph-match (entity_set: "acs@entity_set" {__entity_id__: 'entity_set::acs::acs.ecs.instance'})-[e]-(metric_set: "acs@metric_set")-[e2]->(storage) 
  project storage
| parse-json storage 
| project id; 
.umodel 
| join $umodelDest on concat(kind, '::', json_extract_scalar(metadata, '$.domain'), '::', json_extract_scalar(metadata, '$.name')) = $umodelDest.id
```

#### 增强版查询（同时获取MetricSet信息）

```spl
.let umodelDest = .umodel 
| graph-match (entity_set: "apm@entity_set" {__entity_id__: 'entity_set::apm::apm.service'})-[e]-(metric_set: "apm@metric_set")-[e2]->(storage) 
  project storage, metric_set
| parse-json storage 
| parse-json metric_set
| project storage_id = storage.id, metric_set_name = metric_set.metadata.name; 
.umodel 
| join $umodelDest on concat(kind, '::', json_extract_scalar(metadata, '$.domain'), '::', json_extract_scalar(metadata, '$.name')) = $umodelDest.storage_id
```

---

## 日志查询(.logstore)

### 基本语法
```spl
.logstore with(project='项目名', logstore='日志库名', query='查询条件') [| SPL操作...]
```

### 参数说明
- **project**: SLS项目名
- **logstore**: 日志库名
- **query**: 日志查询条件

### 使用示例
```spl
# 基础日志查询
.logstore with(project='oss-log-1654218965343050-cn-heyuan', logstore='oss-log-store', query='* and bucket_location: oss-cn-heyuan-d') 
| project response_body_length, bucket
| extend response_body_length = cast(response_body_length as double)
| stats avg_resp_body = avg(response_body_length) by bucket

# 日志时间序列分析
.logstore with(project='notebook-demo', logstore='app-log', query='*') 
| extend time = cast(__time__ as bigint) 
| extend time = time - time % 60 
| stats cnt = count(1) by time
| extend time = second_to_nano(time)
| make-series cnt = cnt default = 'nan' on time from 'sls_begin_time' to 'sls_end_time' step '1m'
| extend ret = series_forecast(cnt, 10)
| extend time_series = ret.time_series, metric_series = ret.metric_series, forecast_metric_series = ret.forecast_metric_series
```

---

## 指标查询(.metricstore)

### 基本语法
```spl
.metricstore with(参数列表) [| SPL操作...]
```

### 使用场景
主要用于查询时序指标数据，支持PromQL等查询方式。

---

## 高级查询语法

### .let 子查询变量定义

#### 基本语法
```spl
.let 变量名 = 子查询语句; 
主查询 | join $变量名 on 连接条件
```

#### 使用场景
- **复杂关联查询**: 当需要在不同数据源间建立复杂关联时
- **实体存储查询**: 查找实体对应的MetricSet和存储信息
- **图路径分析**: 需要多步图遍历并与其他数据关联
- **数据预处理**: 先计算中间结果，再与主查询合并

#### 语法要素
1. **变量定义**: `.let 变量名 = 子查询;` - 注意末尾的分号
2. **变量引用**: `$变量名` - 在主查询中使用`$`前缀引用
3. **字段访问**: `$变量名.字段名` - 访问子查询结果的特定字段
4. **多变量支持**: 可以定义多个`.let`变量

#### 标准模板示例
```spl
# 模板1：实体存储查询
.let entityStorage = .umodel 
| graph-match (entity_set: "域@entity_set" {__entity_id__: 'entity_set::域::实体类型'})-[e]-(metric_set)-[e2]->(storage) 
  project storage
| parse-json storage 
| project id; 
.umodel 
| join $entityStorage on concat(kind, '::', json_extract_scalar(metadata, '$.domain'), '::', json_extract_scalar(metadata, '$.name')) = $entityStorage.id

# 模板2：多重关联查询
.let relationStats = .topo 
| graph-match (s)-[e]-(d) 
  project relation_type = "e.__type__", src_domain = "s.__domain__"
| stats relation_count = count(1) by relation_type, src_domain; 
.entity with(domain='apm', type='apm.service') 
| join $relationStats on __domain__ = $relationStats.src_domain

# 模板3：预聚合统计
.let serviceStats = .entity with(domain='apm', type='apm.service') 
| stats service_count = count(1) by region
| extend service_level = case(service_count > 100, 'high', service_count > 10, 'medium', 'low'); 
.entity with(domain='apm', type='apm.instance') 
| join $serviceStats on region = $serviceStats.region
```

### parse-json 节点解析

#### 语法格式
```spl
| parse-json 字段名
| project 新字段 = 字段名.路径
```

#### 使用场景
- **图查询结果**: 解析`graph-match`返回的JSON节点
- **嵌套字段提取**: 从复杂JSON结构中提取特定字段
- **UmodelData处理**: 解析umodel节点的metadata和spec字段

#### 实用示例
```spl
# 解析图查询节点
.umodel | graph-match (s)-[e]-(d)
| parse-json s
| parse-json d

# 解析嵌套JSON字段
.entity with(domain='apm', type='apm.service')
| parse-json metadata_field
```

## SPL操作符

### 过滤 (where)
```spl
| where 条件表达式

# 基础条件
| where service like '%cart%'
| where __entity_type__ = 'apm.service'
| where cnt > 100

# JSON字段提取
| where json_extract_scalar(metadata, '$.domain') in ('acs', 'apm')

# 时间条件
| where __last_observed_time__ > (now() - 3600)
```

### 投影 (project)
```spl
| project 字段列表

# 基础投影
| project __entity_id__, service, host

# 字段重命名
| project serviceName="service", hostInfo="host"

# 嵌套字段投影
| project "src.__domain__", "dest.__entity_type__"
```

### 统计 (stats)
```spl
| stats 聚合函数 by 分组字段

# 计数统计
| stats cnt=count(1) by __entity_type__

# 多聚合函数
| stats cnt=count(1), avg_score=avg(__score__) by __domain__

# 时间聚合
| stats min_time=min(__first_observed_time__), max_time=max(__last_observed_time__) by __domain__

# 条件聚合
| stats create_count=countif(__method__ = 'Create'), delete_count=countif(__method__ = 'Delete') by __relation_type__
```

### 排序 (sort)
```spl
| sort 字段 [desc|asc]

# 基础排序
| sort cnt desc
| sort __last_observed_time__ desc
| sort service asc

# 多字段排序
| sort __domain__ asc, cnt desc
```

### 限制 (limit)
```spl
| limit [起始位置,] 数量

# 基础限制
| limit 10
| limit 100

# 分页
| limit 0, 100
| limit 50, 20
```

### 扩展 (extend)
```spl
| extend 新字段 = 表达式

# 字符串处理
| extend new_key = split(service_name, '-')[0]
| extend domain_type = concat(__domain__, '::', __entity_type__)

# 条件表达式
| extend service_type = case(
    service like '%web%', 'web', 
    service like '%api%', 'api', 
    'other'
)

# 时间计算
| extend last_seen_hours = (now() - __last_observed_time__) / 3600
| extend status = case(
    last_seen_hours <= 1, 'healthy',
    last_seen_hours <= 24, 'warning', 
    'critical'
)
```

### 连接 (join)
```spl
| join [kind=inner|leftouter|rightouter] (子查询) on 连接条件

# 实体与拓扑关联
.entity with(domain='apm', type='apm.service') as services
| join kind=leftouter (
    .topo | graph-match (s:"apm@apm.service")-[e]-(d) 
            project service_id="s.__entity_id__"
            | stats relation_count=count(1) by service_id
  ) on __entity_id__ = service_id
```

---

## 常用查询模板

### 简洁的分步查询示例
```spl
# ✅ 第一步：查询实体获取ID
.entity with(domain='apm', type='apm.operation') 
| where operation='/admin/console'

# ✅ 第二步：查询关联拓扑（替换实际的entity_id）
.topo | graph-call getNeighborNodes('sequence', 1, [
    (:"apm@apm.operation" {__entity_id__: '1a8d2460f88cb1970db2ad39b1814ded'})
])

# ❌ 错误做法：不能混用数据源
# .entity with(...) | join (.topo | graph-match(...)) on ...
```

### 实体发现和分析
```spl
# 1. 搜索APM接口
.entity with(domain='apm', type='apm.operation') 
| where operation like '/admin%'

# 2. 搜索APM服务
.entity with(domain='apm', type='apm.service', query='service: "服务名"')

# 3. 统计实体类型分布
.entity with(domain='apm') | stats cnt=count(1) by __entity_type__ | sort cnt desc

# 4. 查询最近活跃的实体
.entity with(domain='apm', type='apm.operation') 
| sort __last_observed_time__ desc 
| limit 10
```

### 拓扑关系分析
```spl
# 1. APM接口关联查询
.topo | graph-call getNeighborNodes('sequence', 1, [
    (:"apm@apm.operation" {__entity_id__: '接口实体ID'})
])

# 2. APM服务关联查询
.topo | graph-call getNeighborNodes('sequence', 1, [
    (:"apm@apm.service" {__entity_id__: '服务实体ID'})
])

# 3. 查询特定关系类型
.topo | graph-match (s:"apm@apm.service" {__entity_id__: 'ID'})-[e:calls]->(d)

# 4. 查询运行环境关系
.topo | graph-match (s:"apm@apm.service" {__entity_id__: 'ID'})-[e:runs_on]->(h)
```

### 模型分析
```spl
# 1. 获取实体的指标集合
.entity with(domain='*', type='*', query='shippingservice') 
| project __domain__, __entity_type__, __entity_id__
| limit 1
| extend entity_key = concat('entity_set::', __domain__, '::', __entity_type__)
| join (
    .umodel | where kind = 'metric_set' 
            | extend domain = json_extract_scalar(metadata, '$.domain'), 
                     name = json_extract_scalar(metadata, '$.name') 
            | project domain, name, spec
  ) on __domain__ = domain

# 2. 查找MetricSet的存储信息
.umodel | graph-match (s: "apm@metric_set" {__entity_id__: 'metric_set::apm::apm_http_client_dot_metric_set'})<-[e1]-(v1)-[e2:runs_on]->(v2)-[e3]->(v3) 
          project s, e3, v3
        | where json_extract_scalar(v3, '$.kind') = 'sls_metricstore'

# 3. 模型关系统计
.umodel | project kind | stats cnt = count(1) by kind | sort cnt desc

# 4. 域模型分布
.umodel | extend domain = json_extract_scalar(metadata, '$.domain') 
        | stats cnt = count(1) by domain, kind 
        | sort cnt desc
```

---

## 复杂分析场景

### 全链路分析
```spl
# 1. 端到端服务链路追踪
.topo | graph-call cypher(`
    MATCH path = (start:``apm@apm.service`` {service: '起始服务'})-[e:calls*1..5]->(end:``apm@apm.service``)
    RETURN start.service as start_service, end.service as end_service, length(path) as path_length
    ORDER BY path_length
`)

# 2. 服务集群影响面分析
.topo | graph-call getNeighborNodes('full', 3, [
    (:"apm@apm.service" {service: '核心服务名'})
])
| extend node_type = json_extract_scalar(node, '$.__entity_type__')
| stats cnt=count(1) by node_type
| sort cnt desc

# 3. 异常传播路径分析
# 注意：需要分步执行，不能混用.entity和.topo
# 第一步：查找异常服务
.entity with(domain='apm', type='apm.service', query='status: "error"')
| project __entity_id__, service

# 第二步：查询调用链路（使用具体的entity_id）
.topo | graph-match (s:"apm@apm.service")-[e:calls*1..3]->(d:"apm@apm.service" {__entity_id__: '异常服务ID'}) 
        project upstream_service="s.service", downstream_service="d.service", path_length=length(e)
```

### 容量和性能分析
```spl
# 1. 服务实例分布分析
.topo | graph-match (s:"apm@apm.service")-[e:consists_of]->(i:"apm@apm.instance") 
        project service_name="s.service", instance_id="i.__entity_id__", host="i.host"
      | stats instance_count=count(1) by service_name
      | sort instance_count desc

# 2. 跨域资源关联分析
# 注意：需要分步执行，不能混用.entity和.topo
# 第一步：获取APM服务列表
.entity with(domain='apm', type='apm.service')
| project __entity_id__, service

# 第二步：查询跨域关联（使用具体的entity_id）
.topo | graph-match (a:"apm@apm.service" {__entity_id__: '服务ID'})-[e*1..2]-(c:"acs@*") 
        project apm_service="a.service", acs_type="c.__entity_type__", acs_id="c.__entity_id__"

# 3. 时间序列变化趋势
.entity with(domain='apm', type='apm.service') 
| extend time_bucket = __last_observed_time__ - __last_observed_time__ % 3600
| stats service_count=count(1) by time_bucket
| sort time_bucket
```

### 运维和监控场景
```spl
# 1. 实体生命周期分析
.entity with(domain='*') 
| extend lifetime_hours = (__last_observed_time__ - __first_observed_time__) / 3600
| extend lifetime_category = case(
    lifetime_hours < 1, 'short',
    lifetime_hours < 24, 'medium',
    'long'
  )
| stats cnt=count(1) by __entity_type__, lifetime_category

# 2. 关系变化监控
.topo | where __method__ in ('Create', 'Delete') 
      | extend change_time = __last_observed_time__ - __last_observed_time__ % 300  # 5分钟聚合
      | stats create_count=countif(__method__ = 'Create'), 
              delete_count=countif(__method__ = 'Delete') 
        by __relation_type__, change_time
      | sort change_time desc

# 3. 数据质量检查
.entity with(domain='*') 
| extend has_name = isnotnull(service) or isnotnull(instance_name) or isnotnull(name)
| extend has_location = isnotnull(region) or isnotnull(zone) or isnotnull(host)
| stats total=count(1), 
        has_name_cnt=countif(has_name), 
        has_location_cnt=countif(has_location) 
  by __domain__, __entity_type__
| extend name_completeness = round(has_name_cnt * 100.0 / total, 2),
         location_completeness = round(has_location_cnt * 100.0 / total, 2)
```

---

## 性能优化和注意事项

### 查询性能优化

#### 1. USearch优化
```spl
# ✅ 好的做法：指定具体的domain和type
.entity with(domain='apm', type='apm.service', query='关键词')

# ❌ 避免：过度使用通配符
.entity with(domain='*', type='*', query='关键词')

# ✅ 使用columns限制返回字段
.entity with(domain='apm', type='apm.service', columns=['__entity_id__', 'service'])

# ✅ 合理设置topk
.entity with(domain='apm', type='apm.service', query='关键词', topk=50)
```

#### 2. 图查询优化
```spl
# ✅ 指定起始点减少搜索空间
.topo | graph-match (s:"apm@apm.service" {__entity_id__: 'ID'})-[e]-(d)

# ✅ 优先使用sequence方向类型（性能最佳）
.topo | graph-call getNeighborNodes('sequence', 1, [...])  # 推荐使用sequence

# ✅ 限制遍历深度
.topo | graph-call getNeighborNodes('sequence', 3, [...])  # 而不是过大的深度

# ✅ 避免过度使用full方向（性能较差）
# ❌ .topo | graph-call getNeighborNodes('full', 5, [...])  # 避免
# ✅ .topo | graph-call getNeighborNodes('sequence', 3, [...])  # 推荐

# ✅ 使用pure-topo模式降级
.topo | graph-call cypher(`查询语句`, 'pure-topo')
```

#### 3. SPL操作优化
```spl
# ✅ 早期过滤减少数据量
.entity with(domain='apm', type='apm.service') 
| where service like 'web%'  # 先过滤
| stats cnt=count(1) by region  # 再聚合

# ✅ 使用limit限制输出
| limit 100

# ✅ 合理使用project减少字段
| project __entity_id__, service, region
```

### 数据模型理解

#### 1. 字段命名规范
- **系统字段**: 以`__`开头和结尾，如`__entity_id__`、`__domain__`
- **时间字段**: 
  - `__first_observed_time__`: 首次观察时间（秒级时间戳）
  - `__last_observed_time__`: 最后观察时间（秒级时间戳）
  - `__keep_alive_seconds__`: 存活时间（秒）

#### 2. 实体ID规范
- 实体ID为128位hex值
- 不符合规范时系统自动进行xxhash转换
- 查询时使用`__entity_id__`字段

#### 3. Method字段含义
- `Create`: 创建实体/关系
- `Update`: 更新实体/关系（推荐使用）
- `Delete`: 直接删除（慎用）
- `Expire`: 标记过期
- `Revise`: 强制订正

### 引号使用规则
```spl
# graph-match中用双引号
.topo | graph-match (s:"apm@apm.service")

# Cypher中用反引号
.topo | graph-call cypher(`MATCH (n:``apm@apm.service``)`)

# SPL字符串用单引号
| where __entity_type__ = 'apm.service'

# 特殊字符用双引号
| project "src.__domain__"
```

### 数据完整性注意事项

#### 1. 依赖关系
- Cypher查询需要Umodel + Entity + Topo三方数据完备
- 可使用`'pure-topo'`模式降级到只依赖Topo数据
- 注意检查数据缺失情况

#### 2. 时间窗口
- 注意实体和关系的时效性
- 使用`__last_observed_time__`和`__keep_alive_seconds__`判断有效性
- 考虑数据延迟和同步问题

#### 3. 数据一致性
- 跨数据源查询时注意时间一致性
- 使用join时考虑数据匹配度
- 大量数据统计时使用采样策略

---

## 快速参考

### 常用实体类型
```
# APM域
apm.service              # APM服务
apm.operation    # APM服务操作  
apm.instance             # APM实例
apm.host                 # APM主机

# ACS域
acs.ecs.instance         # ECS实例
acs.vpc.vpc              # VPC
acs.alb.loadbalancer     # 负载均衡器
acs.ack.cluster          # K8S集群
acs.k8s.node             # K8S节点
```

### 常用关系类型
```
calls        # 调用关系
consists_of  # 组成关系
runs_on      # 运行在
contains     # 包含关系
depends_on   # 依赖关系
```

### 常用模型类型
```
entity_set       # 实体集合
entity_set_link  # 实体集合关联
metric_set       # 指标集合
sls_metricstore  # SLS指标存储
storage_link     # 存储链接
```

### 常用查询速查

#### 实体查询模板
```spl
# APM接口查询
.entity with(domain='apm', type='apm.operation') 
| where operation='/admin/console'

# APM服务查询  
.entity with(domain='apm', type='apm.service', query='service: "服务名"')

# 统计实体类型
.entity with(domain='apm') | stats cnt=count(1) by __entity_type__ | sort cnt desc
```

#### 拓扑查询模板
```spl
# 查询关联关系（推荐用法）
.topo | graph-call getNeighborNodes('sequence', 1, [
    (:"apm@apm.operation" {__entity_id__: '实体ID'})
])

# 查询特定关系类型
.topo | graph-match (s:"apm@apm.operation" {__entity_id__: 'ID'})-[e:calls]->(d) 
        project s, e, d

# 统计关系类型
.topo | stats cnt=count(1) by __relation_type__ | sort cnt desc
```

#### 重要注意事项
- **分步查询**: 先用`.entity`获取实体ID，再用`.topo`查询关系
- **字符串引号**: SPL中使用单引号，graph-match中用双引号
- **推荐语法**: 优先使用`getNeighborNodes('sequence', 1, [...])`
- **简洁原则**: 避免过度复杂的字段处理和嵌套查询 