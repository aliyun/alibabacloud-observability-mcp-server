# CMS MCP

[《MCP 6月份规划》](https://alidocs.dingtalk.com/api/doc/transit?dentryUuid=MNDoBb60VLYDGNPytr16GevDJlemrZQ3&queryString=utm_medium%3Ddingdoc_doc_plugin_card%26utm_source%3Ddingdoc_doc)

:::
用户问题:

1、提供的 PaaS 工具要比较通用，能进行 APM 域、容器域、云产品域的指标、日志、链路、告警的取数，分析

2、想要一套 AIOps 场景下如何调用我们 MCP 工具的最佳实践（例子）

3、通过 CMS 2.0 的 Copilot 功能，我可以了解到一个真实的问答场景 MCP 工具是怎么调用的，并且可以用在我自建的平台上
:::

## MCP 工具分类

### 划分思路

在 CMS2.0 MCP 里面，可以按照实体-> 数据->诊断/行动三层来分类，当然行动我们目前还没有，覆盖 APM 域 ，容器域等各种域。因此我们在划分 MCP 工具时候，有两种策略:

1.  按照域来划分，比如 APM,容器域这样，当前我们对外输出的就是这么分的，按域分包所有能力，每个团队的人负责开发
    
2.  **按照取数分析流程来划分成实体-> 数据->诊断/行动 三层，先圈定实体，然后获取数据，最后分析加行动。**
    

更倾向于第二种，几个原因:

1.  流程导向更符合数据分析 SRE 排查逻辑，和日常排障步骤能保持一致，先锁定“受影响实体”，再拉指标→Trace→日志→诊断→行动
    
2.  实体是最小公约数，实体查询只需要固定一个入口即可，这样用户和 LLM 只需要了解一种过滤公式,entity\_selector
    
3.  前缀少，语义清晰，LLM 前缀只需要定义 entity\_,metrcs\_,logs\_,traces\_,events\_,topology\_ diagnosis\_以及 actions(有的话), 就不需要变成 apm\_xxx,k8s\_xxx 这样。
    

### 工具分类

体验地址: [http://8.153.195.170:8080/mcp](http://8.153.195.170:8080/mcp) 记得用 streamhttp 连接方式

在请求头里面增加 header ：X-User-ID=1819385687343877

![CleanShot 2025-08-14 at 17.22.56@2x.png](https://alidocs.oss-cn-zhangjiakou.aliyuncs.com/res/YdgOk2bKa1bV8q4B/img/c336d461-f9e7-4b11-a963-d95008c25bdc.png)

| **前缀(复数)** | **工具名** | 输入 | **描述** | 进展更新 |
| --- | --- | --- | --- | --- |
| **entities** | **entities\_list** | *   entity\_selector<br>    <br>    *   domain<br>        <br>    *   type<br>        <br>    *   domain\_ids<br>        <br>    *   labels<br>        <br>*   workspace<br>    <br>*   regionId | 按 domain/type/labels/ids 列出实体（服务、Pod、集群…） | *   [x] SPL，ProblemAgent<br>    <br>1.  有明确的 domain,type 直接调用.entity spl，避免一次远程调用<br>    <br>2.  如果是复杂的 query 查询，直接走 problem-agent |
|  | **entities\_get\_metadata** | *   entity\_selector<br>    <br>    *   domain<br>        <br>    *   type<br>        <br>    *   entity\_id<br>        <br>*   workspace<br>    <br>*   regionId | 获取单个实体的元数据／字段映射 | *   [x] SPL<br>    <br>1.  有明确地实体，直接走 SPL 查询即可 |
|  | **entities\_fuzzy\_search (新增)** | *   entity\_selector<br>    <br>    *   domain<br>        <br>    *   type<br>        <br>    *   query<br>        <br>*   workspace<br>    <br>*   regionId | 调用 usearch spl,直接进行快速的模糊搜索，比如判断某个 ECS 实例 ID 是否存在，直接用实例 ID 模糊搜索即可 | *   [x] SPL |
| **metrics** | **metrics\_list** | *   entity\_selector | 列出实体类型可用指标集合 | *   [x] ProblemAgent |
|  | **metrics\_get\_series** | *   entity\_selector<br>    <br>*   指标名 | 查询指定指标的时序数据， 指标过滤条件，lables 暂时不支持或者提供有限的 labels，step 默认一分钟 | *   [x] ProblemAgent |
|  | **metrics\_get\_golden\_signals** | *   实体<br>    <br>*   entity\_selector | 一键拉取黄金指标：延迟 / QPS / 错误率 / 吞吐 ,[https://sre.google/sre-book/monitoring-distributed-systems/](https://sre.google/sre-book/monitoring-distributed-systems/) | *   [x] ProblemAgent<br>    <br>1.  像指标类的都没限定一定要指定 entiti\_id，就可能变成查询某个领域下面所有实体的黄金指标，此时查询会超出 60S 的限制，受限于 API 最多是 60 秒 |
| **traces** | **traces\_list** | ```json<br>{<br>   "error":true,or false,<br>  "rt":"xxx"<br>}<br>``` | 获取 Trace 列表<br>*   错<br>    <br>*   慢。可自定义 RT<br>    <br>.... | *   [x] ProblemAgent |
|  | **traces\_get\_detail** | *   traceIds:\[xxxx,xxxx\] | 获取 Trace 全量 Span 详情 | *   [x] ProblemAgent |
| **events** | **events\_list** | *   alert\_type: change,alert<br>    <br>*   entity\_selector | 根据实体来过滤事件类型,获取列表 | *   [x] ProblemAgent |
|  | events\_summarize | *   entity\_selector | 根据实体来统计事件 | *   [x] ProblemAgent |
| logs | 暂时走 IAAS 层 MCP |  |  |  |
| **topologies** | **topologies\_list\_neighbors** | *   relation\_types\[runs,calls\]<br>    <br>*   source\_entity\_selector<br>    <br>*   target\_entity\_selector | 下游或者上游服务依赖 | *   [x] ProblemAgent |
| **diagnosis** | diagnosis\_detect\_metric\_anomaly | *   metric name<br>    <br>*   entity\_selector | 对时序指标做异常检测 | *   [x] ProblemAgent |
|  | diagnosis\_detect\_trace\_anomaly |  |  | *   [x] ProblemAgent |
| drilldown | drilldown\_metric | *   entity\_selector | 对时序指标做下钻分析 | *   [x] ProblemAgent |

*   所有工具都会有一个基础参数，entity\_selector，这样只需要统一 entity\_selector 的选择语法即可
    
*   时间区间：
    
    *   start\_time: now()-1h 
        
    *   end\_time now()-5m
        

```json
{
  "tool": "metrics_get_series",
  "input": {
    "entity_selector": {
      "domain": "apm",
      "type":   "apm.service",
      "filters": {"service": "order-service", "env": "prod"} #举个例子
    },
    "metric": "request_latency_p95_ms",
    "start_time": "now()-1h",
    "end_time":   "now()",
    "step": "1m"
  }
}
```

### 工具实现

1.  现在我们内部使用的工具都包含了 CMS2.0 特有的参数，或者需要足够上下文才能提取出参数，不适合直接对客输出，因此上述的 MCP 工具都要基于现有的能力封装一层
    
2.  实现方案
    

1.  可以直接基于 ProblemAgent 的输出能力来做封装，基于同一个查询接口，只需要设定好不同的 Prompt 即可实现，这样好处是 MCP 层会比较灵活，只需要封装请求，返回结果即可，相当于只是套个壳，升级维护都方便，大部分的 MCP 都是这么去做的。 坏处是 1.  用户在 Copilot 上看到的工具可能和 MCP 工具名称不一样，虽然实现都是一致的，主要是我们内部工具会做一些改造，比如流式输出日志，上下文保存记录等，这些都不适合直接放在 MCP Server 里面。2. 链路比较长，需要走 POP 网关->ML-SERVICE->ProblemAgent, 而不是直接从网关到 SLS Server.
    
2.  直接把 SPL 的拼装，结果的处理全部放在 MCP SERVER 端，这样 MCP 实现部分会有非常多的数据解析工作，这样好处是调用链路会缩短，网管->SLS Server,坏处是不易维护，以及不灵活，只能拼装一些固定的 SPL，并且升级维护需要升级 MCP SDK。 我们把内部的很多逻辑实现都放在 MCP SERVER，并且会受限于 MCP 框架的限制，比如流式支持等，因为 llm tool 的实现形式不只有 MCP，MCP 只是个输出界面。
    
3.  折中方案，为了要保证内部和外部输出的工具名是一致的，那么在 Problem 侧可以封装一下这个工具列表，实现上不一致(出于流式以及上下文记录等原因). 
    

## 如何调用我们 MCP 工具的最佳实践（例子）

一些 AIOPS 场景下典型场景

| **典型场景** | **触发信号** | **诊断重点** | **统一命名后的 MCP 工具链** |
| --- | --- | --- | --- |
| **延迟飙高** | p95／p99 RT ↑ | 关键慢调用、下游依赖 | entities\_list → metrics\_get\_golden\_signals → diagnosis\_detect\_anomaly → traces\_list → traces\_get\_detail → topologies\_list\_neighbors |
| **错误率激增** | error\_rate ↑、5xx 告警 | 异常 Trace、错误日志 | metrics\_get\_golden\_signals → traces\_list → traces\_get\_detail → logs\_query\_entries |
| **资源瓶颈** | CPU／内存／IO 饱和 | 同机房实例、热点服务 | entities\_list (集群／节点) → metrics\_get\_series (cpu |
| **Pod CrashLoop／节点重启** | K8s Event ／ Agent 重启 | 失效范围、重启原因 | entities\_list (k8s.pod) → events\_list ／ events\_summarize → metrics\_get\_golden\_signals |

以上是出现问题时候理想状态下的排查流程，这个取决于用户所使用大模型的智能程度以及上下文是否已经做出了足够的提示。

> 在 Copilot 中，对于复杂的分析任务当前有两种实现策略，一种是基于 LLM AGENT 方式，在分析前会先生成一个 Plan，这个 Plan 的原始输入来自于定好的一些规则以及知识库， 另外一种是借助于磁力地图，磁力地图是纯算法下钻的实现方式，具备高效快速的方式。后续磁力地图的分析功能可以考虑作为一个 MCP 提供出来

当输入：最近五分钟内有个实体指标出现了下跌，排查下问题。那么实现可以生成一个 plan，这个 plan 里面可以制定下步骤以及每个工具该做什么**（目前这个 PLAN 可以在 Prompt 加个说明，要模型来预先生成）**，类似于如下格式 :

```json
# 假设告警已触发，输入实体标签 service=order-service
steps:
  # 1️⃣ 拉黄金指标并检测错误率 >5%
  - id: gold
    tool: metrics_get_golden_signals
    input:
      entity_selector: {domain: apm, type: apm.service, labels: {service: order-service}}
      start_time: "now()-15m"
      end_time:   "now()"
      step: "30s"

  - id: anomaly
    tool: diagnosis_detect_anomaly
    input:
      series: "{{ gold.data.error_rate }}"
      method: threshold
      threshold: 0.05

  # 2️⃣ 若异常成立，采样异常 Trace + 错误日志
  - if: "{{ anomaly.data.is_anomalous }}"
    steps:
      - id: trace_sample
        tool: traces_query
        input:
          entity_selector: {domain: apm, type: apm.service, labels: {service: order-service}}
          condition: {"status": ["ERROR","EXCEPTION"]}
          start_time: "{{ gold.data.time_range.start }}"
          end_time:   "{{ gold.data.time_range.end }}"
          limit: 50

      - id: trace_detail
        tool: traces_get_detail
        input: {trace_id: "{{ trace_sample.data[0].trace_id }}", stream: true}

      - id: logs
        tool: logs_query_entries
        input:
          entity_selector: {domain: apm, type: apm.service, labels: {service: order-service}}
          query: "level in (ERROR,WARN)"
          start_time: "{{ gold.data.time_range.start }}"
          end_time:   "{{ gold.data.time_range.end }}"
          limit: 1000

      - id: graph
        tool: topology_get_service_graph
        input:
          entity_selector: {domain: apm, type: apm.service, labels: {service: order-service}}
          start_time: "{{ gold.data.time_range.start }}"
          end_time:   "{{ gold.data.time_range.end }}"

      # 3️⃣ 归因：是否下游依赖异常？
      - id: related
        tool: entity_list_related
        input:
          src_selector: {domain: apm, type: apm.service, labels: {service: order-service}}
          relation: calls
          direction: out

      - id: db_metrics
        tool: metrics_get_series
        input:
          entity_selector: {domain: apm, type: apm.service, labels: {service: "{{ related.data[0].labels.service }}"}}
          metric: request_latency_p95_ms
          start_time: "{{ gold.data.time_range.start }}"
          end_time:   "{{ gold.data.time_range.end }}"
          step: "30s"

      # 4️⃣ 汇总证据 → GPT 归因
      - id: rca
        tool: diagnosis_classify_root_cause
        input:
          evidence:
            err_series: "{{ gold.data.error_rate }}"
            traces:     "{{ trace_detail.stream }}"
            logs:       "{{ logs.data }}"
            latency_db: "{{ db_metrics.data }}"
            topology:   "{{ graph.data }}"
```

接下来模型就按照这个步骤来做执行

| **步骤** | **工具** | **说明** |
| --- | --- | --- |
| 1-a | metrics\_get\_golden\_signals | 8获取延迟、错误率等四黄金信号 |
| 1-b | diagnosis\_detect\_anomaly | 阈值或 ML 模式检测 error\_rate 是否异常 |
| 2-a | traces\_query | 采样含 ERROR 状态 Trace（≈根因的“放大镜”） |
| 2-b | logs\_query\_entries | 并行拉 ERROR/WARN 日志，辅助文本证据 |
| 2-c | topology\_get\_service\_graph | 生成 order-service 的上下游依赖图 |
| 3-a | entity\_list\_related | 找下游服务；示例假设是 mysql-prod |
| 3-b | metrics\_get\_series | 拉下游服务延迟，验证是否引起级联错误 |
| 4 | diagnosis\_classify\_root\_cause | LLM 汇总多模态证据输出人类可读 RCA |

> 实践经验:

> 1.  事先由模型来生成 Plan，每次处理完毕之后更新步骤状态，帮助模型做小抄，这些也有开源的 MCP 工具，比如 [task-manager](https://github.com/eyaltoledano/claude-task-master) 或者 [sequence-thinking](https://github.com/spences10/mcp-sequentialthinking-tools)

> 2. 控制下钻深度以及时长，可以在更新状态之后告知下钻深度

### 待完成事项

#### 1. 工具分层与覆盖范围

##### 工具分层架构

项目采用三层架构设计，覆盖从基础数据获取到智能诊断的完整链路：

**第一层：实体层（Entities）**
- **作用**：定位和管理监控对象
- **覆盖范围**：APM域（服务、应用）、容器域（Pod、节点、集群）、云产品域（ECS、RDS等）
- **工具列表**：
  - `entities_list` - 列出符合条件的实体
  - `entities_search` - 搜索实体
  - `entities_list_domains` - 列出所有实体域
  - `entities_list_types` - 列出域下的实体类型
  - `entities_get_metadata` - 获取实体元数据

**第二层：数据层（Data）**
- **作用**：获取和分析监控数据
- **覆盖范围**：
  - 指标数据（Metrics）：CPU、内存、延迟、QPS等
  - 链路数据（Traces）：分布式调用链、span详情
  - 事件数据（Events）：告警、变更、异常事件
  - 拓扑数据（Topologies）：服务依赖关系
- **工具列表**：
  - Metrics: `metrics_list`, `metrics_get_series`, `metrics_get_golden_signals`
  - Traces: `traces_list`, `traces_get_detail`
  - Events: `events_list`, `events_summarize`
  - Topologies: `topologies_list_neighbors`

**第三层：诊断/行动层（Diagnosis/Actions）**
- **作用**：智能分析和问题定位
- **覆盖范围**：异常检测、根因分析、下钻分析
- **工具列表**：
  - `diagnosis_detect_metric_anomaly` - 指标异常检测
  - `diagnosis_detect_trace_anomaly` - 链路异常检测
  - `drilldown_metric` - 指标下钻分析

**兼容层：IaaS工具集（V1向后兼容）**
- **作用**：保持与传统工具的兼容性
- **覆盖范围**：SLS日志服务、ARMS应用监控、CMS云监控
- **工具列表**：
  - SLS: 6个工具（日志查询、SQL生成等）
  - ARMS: 8个工具（应用搜索、火焰图分析、链路分析等）
  - CMS: 2个工具（PromQL生成和执行）

#### 2. 按需加载不同工具

##### 工具包加载策略

系统支持通过环境变量 `MCP_ENABLED_TOOLKITS` 灵活配置加载的工具包：

**预定义工具组**：
```bash
# CMS工具集（推荐，包含所有可观测2.0工具）
export MCP_ENABLED_TOOLKITS=cms
# 包含: entities, metrics, traces, events, topologies, diagnosis, drilldown, workspace

# 所有工具（包含CMS和IaaS）
export MCP_ENABLED_TOOLKITS=all

# IaaS传统工具集
export MCP_ENABLED_TOOLKITS=iaas
```

**自定义组合**：
```bash
# 仅加载实体和指标工具
export MCP_ENABLED_TOOLKITS=entities,metrics

# APM场景：实体、指标、链路、诊断
export MCP_ENABLED_TOOLKITS=entities,metrics,traces,diagnosis

# 容器监控场景：实体、事件、拓扑
export MCP_ENABLED_TOOLKITS=entities,events,topologies

# 日志分析场景：仅加载IaaS层的SLS工具
export MCP_ENABLED_TOOLKITS=iaas
```

**使用场景建议**：

| 场景 | 推荐配置 | 说明 |
|------|---------|------|
| **AIOps全场景** | `cms` 或 `all` | 包含完整的诊断分析能力 |
| **APM性能监控** | `entities,metrics,traces,diagnosis` | 专注于应用性能分析 |
| **容器运维** | `entities,events,topologies,metrics` | K8s集群管理和监控 |
| **日志分析** | `iaas` | 使用SLS进行日志查询和分析 |
| **轻量级监控** | `entities,metrics` | 仅基础指标监控 |
| **告警响应** | `entities,events,diagnosis` | 快速定位告警根因 |

**配置方式**：
1. 环境变量：`export MCP_ENABLED_TOOLKITS=cms`
2. 命令行参数：`--toolkits cms`
3. 默认行为：不配置时加载所有工具（all）

**实现原理**：
- 工具包采用动态注册机制
- 启动时根据配置仅加载指定的工具包
- 减少内存占用和启动时间
- 避免加载不需要的依赖

#### 3. 最佳工具组合推荐

详细的工具组合推荐请参考：[最佳工具组合推荐文档](./best_practices_toolkit_combinations.md)

**快速选择指南**：

> 📌 **注意**：`entities` 工具包是基础工具包，在所有场景中都会自动包含

| 使用场景 | 推荐配置 | 实际加载 | 工具数量 | 说明 |
|---------|---------|---------|---------|------|
| **快速开始** | `metrics` | entities + metrics | 8个 | 最简配置，适合初次使用 |
| **生产环境** | `cms` | entities + 7个工具包 | 17个 | 完整的可观测2.0能力 |
| **完整功能** | `all` | entities + 所有工具包 | 33个 | 包含新旧所有工具 |
| **APM专用** | `metrics,traces,diagnosis` | entities + 指定工具包 | 12个 | 应用性能监控专用 |
| **容器运维** | `events,topologies,metrics` | entities + 指定工具包 | 11个 | K8s集群管理 |

**选择决策流程**：
1. 是否需要兼容旧系统？→ 是：使用 `all`
2. 是否需要完整诊断能力？→ 是：使用 `cms`
3. 主要关注什么场景？
   - 应用性能 → `entities,metrics,traces,diagnosis`
   - 容器管理 → `entities,events,topologies,metrics`
   - 日志分析 → `iaas`
   - 告警处理 → `entities,events,diagnosis`
   - 基础监控 → `entities,metrics`

### 总结

通过以上三层架构设计和灵活的工具包加载机制，阿里云可观测MCP服务能够：

1. **满足不同规模团队需求**：从初创团队的轻量级监控到大型企业的全方位可观测性
2. **适配多种技术场景**：覆盖微服务、容器、Serverless等现代架构
3. **提供渐进式升级路径**：从基础监控逐步扩展到智能诊断
4. **保持良好的性能**：按需加载，避免资源浪费
5. **兼容新旧系统**：同时支持CMS 2.0和传统IaaS工具

建议用户根据实际需求选择合适的工具组合，并随着业务发展逐步调整配置。
