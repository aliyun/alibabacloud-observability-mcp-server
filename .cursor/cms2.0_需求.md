
可观测2.0 基于UMODEL的MCP工具

计划 新增几个MCP工具，实现在 cmsv2_toolkit.py 中
使用的client是 cms_client，实现方式可参考 sls_toolkit.py 中的实现方式


一共包含几个MCP工具

### 工具一：模糊搜索实体

MCP 工具:
输入
说明
查询某个域的实体信息
{
  "domain":"*",
  "domain_type":"*",
  "name":"checkouty"
}
1. 传入 domain 和 domain_type
2. name 等同于 service,operation 这些？ 这样就可以避免不需要设定 name 多参数
3. 实现上使用模糊搜索，然后由模型去选择合适的实体
.entity with(domain='', type='', query='shippingservice',topk=10)
1. 先不提扩展查询，支持简单查询
2. 比如查询特定服务，应用，接口等信息，不支持多个筛选条件

### 工具二:
查询实体关系
设计
1. 查询实体上下游的依赖关系，回答某个应用有哪些依赖服务的问题
1. 基于 UMODEL 高质量 API
.entity_set(domain='apm', name='apm.operation') | entity-call list_related_entity_set(['calls'], 'out', true)
MCP 工具
MCP 工具
输入
输出
查询某个服务的调用依赖
{
  "domain":"xxx",
  "domain_type":"xxxx",
  "relation":"call?"
}

● 返回关系实体列表，这个可能要支持分页，因为实体依赖会有很多 


### 工具三
查询存储地址类信息
设计
1. 查询某个 trace 或者指标的 metric store 地址，对于 text to promql 之类需要
1. 目前高质量 umodel 不直接提供，但是可以通过空跑的方式获取 metric_store 地址
set umodel_paas_mode='dry_run';
.metric_set with(...) | ...

MCP 工具
MCP 工具
输入
输出
查询某个实体的 metric _store 信息
{
  "domain":"xxx",
  "domain_type":"xxxx",
}

返回 project logstore 信息

### 工具四
查询指标数据
设计:
1. 解决指标查询的问题
1. 基于.metric_set 的高质量 API 来提供指标查询能力

```spl
.entity_set(domain='apm', name='apm.operation') | entity-call list_data_set(['metric_set'], true)
```
.metric_set with(domain='apm', name='apm.metric.apm.operation', source='metrics', metric='avg_request_latency_ms', step='1m', aggregate_labels=['host']) | where service_id = 'xxx'
输出
MCP 工具
入参
返回
query_metric
{
  "metric":"avg_request_latency_ms",
  "domain_type":"service,operation,host",
  "filters":"service_id=xxxx"
}
1. 首先获取到所有的 metric_set 返回给模型
1. 然后 LLM 从中选择合适的 metric 来执行查询
1. 不直接使用 PROMQL
返回实际执行查询的 语句 以及指标元数据结果




### 获取标签值
获取标签值的最终SPL是 ```spl
.entity_set with(domain='apm', name='apm.service', query='service_id = "order-service"') 
| entity-call get_label_values('apm', 'apm.metric.apm.service', 'region')
```
但是完成这个需要 ，确定应用名
1. 明确entity_set的domain和name
1. 'apm.metric.apm.service' 是metric_set名称，需要从 .entity_set with(domain='apm', name='apm.service') 
| entity-call list_data_set(['metric_set'], true) |project name  这样的方式来获取到所有的name列表


查询某个应用服务的



