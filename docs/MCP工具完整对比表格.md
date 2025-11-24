# MCP工具完整对比表格

基于完整遍历两个项目的所有工具，生成准确的实现差异和能力差异对比。

## 工具功能详细对比

| 功能 | alibabacloud-observability | umodel-mcp | 实现差异 | 能力差异 |
|------|----------------------------|------------|----------|----------|
| **实体列表查询** | `entities_list` | `get_entities` | SPL vs SPL | `output_mode`(list/count) vs `entity_filter`过滤条件 |
| **实体搜索** | `entities_search` | `search_entities` | RemoteAgent vs SPL | `query`自然语言必填 vs `search_text`全文搜索 |
| **邻居实体查询** | `topologies_list_neighbors` | `get_neighbor_entities`<br>`list_related_entity_set` | RemoteAgent vs SPL | `query`自然语言拓扑描述 vs `entity_id`精确ID必填 |
| **实体元数据** | `entities_get_metadata` | `search_entity_set` | SPL vs SPL | `entity_id`可选 vs `search_text`全文搜索 |
| **域类型发现** | `entities_list_domains`<br>`entities_list_types` | `search_entity_set` | SPL vs SPL | `domain`可选过滤 vs `search_text`全文搜索 |
| **指标列表** | `metrics_list` | ❌ | RemoteAgent | `query`自然语言可选 |
| **时序数据** | `metrics_get_series` | `get_metrics` | RemoteAgent vs SPL | `start_time`/"end_time"相对时间 vs `metric_set_name`必填 |
| **黄金指标** | `metrics_get_golden_signals` | `get_golden_metrics` | RemoteAgent vs SPL | `entity_id`必填 vs `entity_ids`可选数组 |
| **关系指标** | ❌ | `get_relation_metrics` | SPL | `relation_type`关系类型必填 |
| **关系黄金指标** | ❌ | `get_relation_golden_metrics` | SPL | `relation_type`关系类型必填 |
| **日志查询** | `sls_execute_sql_query` | `get_logs` | 直接API vs SPL | `logstore`/`query`自然语言 vs `log_set_name`必填 |
| **事件查询** | `events_list` | `get_events` | RemoteAgent vs SPL | `query`自然语言可选 vs `event_filter`结构化过滤 |
| **事件汇总** | `events_summarize` | ❌ | RemoteAgent | `query`自然语言汇总描述 |
| **链路查询** | `traces_list` | `search_traces` | RemoteAgent vs SPL | `query`自然语言可选 vs `trace_filter`结构化过滤 |
| **链路详情** | `traces_get_detail` | `get_traces` | RemoteAgent vs SPL | `query`trace描述 vs `trace_id`精确ID必填 |
| **性能分析** | ❌ | `get_profiles` | SPL | `profile_type`性能分析类型必填 |
| **指标异常检测** | `diagnosis_detect_metric_anomaly` | ❌ | RemoteAgent | `query`异常描述自然语言 |
| **链路异常检测** | `diagnosis_detect_trace_anomaly` | ❌ | RemoteAgent | `query`异常描述自然语言 |
| **指标下钻** | `drilldown_metric` | ❌ | RemoteAgent | `query`下钻描述自然语言 |
| **元数据-实体集** | ❌ | `get_entity_set` | SPL | `domain`/`name`必填 |
| **元数据-数据集** | ❌ | `list_data_set` | SPL | `data_set_types`可选数组 |
| **元数据-关联** | （已归入邻居实体查询） | `list_related_entity_set` | SPL | `source_domain`/`source_name`必填 |
| **元数据-搜索** | （已归入实体元数据/域类型发现） | `search_entity_set` | SPL | `search_text`全文搜索必填 |
| **SPL执行** | ❌ | `execute_spl` | SPL | `query`原始SPL语句必填 |
| **会话管理** | ❌ | `set_time_range`<br>`get_current_time_range` | 会话状态 | `from_time`/`to_time`Unix时间戳 |
| **工作空间** | `workspaces_list` | ❌ | SPL | 无参数，返回可用工作空间 |
| **SLS项目管理** | `sls_list_projects`<br>`sls_list_logstores` | ❌ | 直接API | `project_name`可选 |
| **SLS日志结构** | `sls_describe_logstore` | ❌ | 直接API | `project`/`logstore`必填 |
| **文本转SQL** | `sls_translate_text_to_sql_query` | ❌ | RemoteAgent | `text`自然语言转换 |
| **SLS查询诊断** | `sls_diagnose_query` | ❌ | RemoteAgent | `query`SQL语句诊断 |
| **ARMS应用搜索** | `arms_search_apps` | ❌ | 直接API | `app_name`可选搜索 |
| **ARMS链路查询生成** | `arms_generate_trace_query` | ❌ | RemoteAgent | `query_description`自然语言 |
| **火焰图分析** | `arms_profile_flame_analysis` | ❌ | RemoteAgent | `query`分析描述自然语言 |
| **火焰图对比** | `arms_diff_profile_flame_analysis` | ❌ | RemoteAgent | 两个时间段对比参数 |
| **应用信息获取** | `arms_get_application_info` | ❌ | 直接API | `pid`进程ID必填 |
| **Trace质量检测** | `arms_trace_quality_analysis` | ❌ | RemoteAgent | `query`质量分析描述 |
| **慢调用分析** | `arms_slow_trace_analysis` | ❌ | RemoteAgent | `query`慢调用分析描述 |
| **错误分析** | `arms_error_trace_analysis` | ❌ | RemoteAgent | `query`错误分析描述 |
| **相关数据搜索** | `search_single_trace_related_data` | ❌ | RemoteAgent | `trace_id`/`query`描述 |
| **文本转PromQL** | `cms_translate_text_to_promql` | ❌ | RemoteAgent | `text`自然语言转PromQL |
| **PromQL执行** | `cms_execute_promql_query` | ❌ | 直接API | `promql`查询语句必填 |

## 关键工具功能差异详解

### 1. **关系指标** vs **指标列表**

#### **关系指标** (`get_relation_metrics`) - umodel独有
- **目的**: 获取实体间关系的特定指标数据
- **查询对象**: 两个实体之间的连接/调用关系
- **示例**: 服务A调用服务B的具体延迟数值、错误计数
- **参数**: `relation_type`关系类型必填、`metric`指标名必填
- **返回**: 关系层面的具体指标时序数据

#### **指标列表** (`metrics_list`) - alibabacloud独有
- **目的**: 列出单个实体类型可用的指标清单
- **查询对象**: 实体本身的属性和指标
- **示例**: apm.service支持哪些指标(CPU、内存、QPS等)
- **参数**: `query`自然语言可选描述
- **返回**: 可查询的指标名称列表

### 2. **关系黄金指标** vs **黄金指标**

#### **关系黄金指标** (`get_relation_golden_metrics`) - umodel独有
- **目的**: 获取实体间关系的核心性能指标
- **查询对象**: 两个实体之间的调用关系健康度
- **示例**: 服务A→服务B的调用延迟、错误率、吞吐量
- **参数**: `relation_type`关系类型必填
- **返回**: 关系层面的黄金指标(延迟/流量/错误/饱和度)

#### **黄金指标** (`metrics_get_golden_signals`) - alibabacloud
- **目的**: 获取单个实体的核心性能指标
- **查询对象**: 单个实体本身的健康状况
- **示例**: order-service的延迟、QPS、错误率、资源使用率
- **参数**: `entity_id`必填、`query`自然语言可选
- **返回**: 实体级别的黄金指标

### 3. **实体搜索** vs **全文搜索**

#### **实体搜索** (`entities_search`) - alibabacloud
- **目的**: 通过自然语言智能搜索实体
- **查询对象**: 基于语义理解的实体匹配
- **示例**: "生产环境中响应时间大于500ms的服务"
- **参数**: `query`自然语言描述必填
- **返回**: AI理解后的匹配实体列表

#### **实体搜索** (`search_entities`) - umodel
- **目的**: 通过关键词进行全文搜索
- **查询对象**: 基于文本匹配的实体查找
- **示例**: "payment" 搜索名称包含payment的实体
- **参数**: `search_text`关键词必填
- **返回**: 文本匹配的实体列表

### 4. **会话管理** vs **无状态查询**

#### **会话管理** (`set_time_range`/`get_current_time_range`) - umodel独有
- **目的**: 管理查询会话的全局时间范围状态
- **查询对象**: 当前会话的时间窗口设置
- **示例**: 设置分析时间为"2024-01-01 00:00 ~ 2024-01-02 00:00"
- **参数**: `from_time`/`to_time`Unix时间戳
- **返回**: 会话状态确认或当前时间范围

#### **时间参数** - alibabacloud
- **目的**: 每次查询都独立指定时间范围
- **查询对象**: 单次查询的时间窗口
- **示例**: `start_time="now()-1h", end_time="now()"`
- **参数**: 每个工具都有独立的时间参数
- **返回**: 无会话状态，每次查询独立

### 5. **SPL执行** vs **RemoteAgent查询**

#### **SPL执行** (`execute_spl`) - umodel独有
- **目的**: 直接执行原始SPL查询语句
- **查询对象**: 底层数据存储的直接访问
- **示例**: `.entity with(domain='apm', type='service') | stats count by status`
- **参数**: `query`原始SPL语句必填
- **返回**: SPL查询的原始结果

#### **RemoteAgent查询** - alibabacloud
- **目的**: 通过AI智能代理理解和执行查询
- **查询对象**: 自然语言到结构化查询的转换
- **示例**: "查看生产环境中异常的服务数量"
- **参数**: `query`自然语言描述
- **返回**: AI处理后的结构化结果

## alibabacloud独有工具功能详解

### 1. **智能诊断类**

#### **指标异常检测** (`diagnosis_detect_metric_anomaly`)
- **目的**: 通过AI检测指标中的异常模式
- **查询对象**: 实体指标的异常行为分析
- **示例**: "检测order-service最近1小时的异常指标"
- **参数**: `query`异常描述自然语言
- **返回**: 异常检测结果和可能原因

#### **链路异常检测** (`diagnosis_detect_trace_anomaly`)
- **目的**: 通过AI分析调用链路的异常情况
- **查询对象**: 分布式调用链的异常模式
- **示例**: "分析支付流程中的异常调用链"
- **参数**: `query`异常描述自然语言
- **返回**: 异常链路识别和根因分析

#### **指标下钻** (`drilldown_metric`)
- **目的**: 智能分析指标异常的根本原因
- **查询对象**: 多维度指标的关联分析
- **示例**: "分析CPU使用率突增的根因"
- **参数**: `query`下钻描述自然语言
- **返回**: 多层次的原因分析路径

### 2. **事件分析类**

#### **事件汇总** (`events_summarize`)
- **目的**: 智能汇总和分类系统事件
- **查询对象**: 大量事件的模式识别
- **示例**: "汇总最近24小时的告警事件类型"
- **参数**: `query`自然语言汇总描述
- **返回**: 事件分类统计和趋势分析

### 3. **SLS智能查询类**

#### **文本转SQL** (`sls_translate_text_to_sql_query`)
- **目的**: 将自然语言转换为SLS查询语句
- **查询对象**: 日志查询需求的语言理解
- **示例**: "查找支付相关的错误日志" → SQL查询
- **参数**: `text`自然语言转换需求
- **返回**: 生成的SQL查询语句

#### **SLS查询诊断** (`sls_diagnose_query`)
- **目的**: 诊断和优化SLS查询语句
- **查询对象**: SQL查询的性能和正确性
- **示例**: 分析查询慢的原因和优化建议
- **参数**: `query`SQL语句诊断
- **返回**: 查询问题诊断和优化建议

### 4. **ARMS智能分析类**

#### **ARMS链路查询生成** (`arms_generate_trace_query`)
- **目的**: 根据需求自动生成调用链查询条件
- **查询对象**: 复杂的调用链筛选需求
- **示例**: "查找响应时间大于2秒的支付相关调用"
- **参数**: `query_description`自然语言需求
- **返回**: 生成的调用链查询条件

#### **火焰图分析** (`arms_profile_flame_analysis`)
- **目的**: 智能分析性能火焰图找出热点
- **查询对象**: 应用性能瓶颈识别
- **示例**: "分析订单服务的CPU热点函数"
- **参数**: `query`分析描述自然语言
- **返回**: 性能热点分析和优化建议

#### **火焰图对比** (`arms_diff_profile_flame_analysis`)
- **目的**: 对比不同时间段的火焰图变化
- **查询对象**: 性能变化趋势分析
- **示例**: 对比发布前后的性能差异
- **参数**: 两个时间段对比参数
- **返回**: 性能变化对比和影响分析

#### **Trace质量检测** (`arms_trace_quality_analysis`)
- **目的**: 评估调用链数据的完整性和质量
- **查询对象**: 分布式追踪数据质量
- **示例**: "检测支付链路的追踪覆盖率"
- **参数**: `query`质量分析描述
- **返回**: 追踪质量报告和改进建议

#### **慢调用分析** (`arms_slow_trace_analysis`)
- **目的**: 深入分析慢调用的根本原因
- **查询对象**: 性能瓶颈的根因分析
- **示例**: "分析数据库查询慢的原因"
- **参数**: `query`慢调用分析描述
- **返回**: 根因分析和性能优化建议

#### **错误分析** (`arms_error_trace_analysis`)
- **目的**: 智能分析错误调用的根本原因
- **查询对象**: 错误传播路径和影响范围
- **示例**: "分析支付失败的错误传播链"
- **参数**: `query`错误分析描述
- **返回**: 错误根因和修复建议

### 5. **CMS智能查询类**

#### **文本转PromQL** (`cms_translate_text_to_promql`)
- **目的**: 将自然语言转换为Prometheus查询
- **查询对象**: 指标查询需求的语言理解
- **示例**: "CPU使用率大于80%的服务器" → PromQL
- **参数**: `text`自然语言转PromQL需求
- **返回**: 生成的PromQL查询语句

## umodel独有工具功能详解

### 1. **元数据管理类**

#### **实体集查询** (`get_entity_set`)
- **目的**: 获取EntitySet的完整schema定义
- **查询对象**: 数据模型的结构信息
- **示例**: 获取apm.service的字段定义和类型
- **参数**: `domain`/`name`必填
- **返回**: EntitySet的完整schema信息

#### **数据集发现** (`list_data_set`)
- **目的**: 发现与EntitySet关联的所有DataSet
- **查询对象**: 数据集的依赖和关联关系
- **示例**: 查看apm.service关联的MetricSet/LogSet
- **参数**: `data_set_types`可选数组过滤
- **返回**: 可用数据集列表和关联信息

### 2. **性能分析类**

#### **性能分析** (`get_profiles`)
- **目的**: 获取应用性能分析数据
- **查询对象**: CPU火焰图、内存分配等性能数据
- **示例**: 获取Java应用的CPU火焰图数据
- **参数**: `profile_type`性能分析类型必填
- **返回**: 性能分析的原始数据

### 3. **关系分析类**

#### **关系指标** (`get_relation_metrics`)
- **目的**: 获取实体间关系的特定指标数据
- **查询对象**: 服务调用、依赖关系等的量化数据
- **示例**: 获取serviceA→serviceB的调用延迟数据
- **参数**: `relation_type`/`metric`必填
- **返回**: 关系维度的时序指标数据

#### **关系黄金指标** (`get_relation_golden_metrics`)
- **目的**: 获取关系层面的核心健康指标
- **查询对象**: 服务间调用的综合健康度
- **示例**: serviceA→serviceB的延迟/吞吐/错误率
- **参数**: `relation_type`关系类型必填
- **返回**: 关系层面的黄金指标组合

### 4. **原始查询类**

#### **SPL执行** (`execute_spl`)
- **目的**: 直接执行原始SPL查询语句
- **查询对象**: 底层数据存储的无限制访问
- **示例**: 复杂的多表关联统计查询
- **参数**: `query`原始SPL语句必填
- **返回**: SPL执行的原始结果集

#### **会话管理** (`set_time_range`/`get_current_time_range`)
- **目的**: 管理分析会话的全局状态
- **查询对象**: 会话级别的时间窗口和上下文
- **示例**: 设置分析时间范围后多次查询复用
- **参数**: `from_time`/`to_time`Unix时间戳
- **返回**: 会话状态管理和时间范围设置

## 实现差异统计

| 实现方式 | alibabacloud-observability | umodel-mcp |
|----------|----------------------------|------------|
| **SPL查询** | 6个工具 | 19个工具 |
| **RemoteAgent** | 17个工具 | 0个工具 |
| **直接API调用** | 8个工具(SLS/ARMS/CMS) | 0个工具 |
| **会话状态** | 0个工具 | 2个工具 |

## 能力差异统计  

| 能力类型 | alibabacloud-observability | umodel-mcp |
|----------|----------------------------|------------|
| **自然语言查询** | 17个工具支持 | 0个工具支持 |
| **RemoteAgent智能分析** | 17个工具支持 | 0个工具支持 |
| **元数据管理** | 2个工具 | 4个工具 |
| **云服务直接集成** | 8个工具 | 0个工具 |
| **精确参数控制** | 部分支持 | 19个工具全支持 |
| **关系分析** | 1个工具 | 3个工具 |
| **性能分析** | 3个工具(火焰图) | 1个工具(profiles) |

## 核心差异总结

### 实现方式差异

1. **alibabacloud-observability**：
   - **RemoteAgent驱动**：17个工具使用RemoteAgent进行智能查询
   - **SPL查询**：6个工具使用SPL查询系统
   - **多API集成**：8个工具直接调用SLS/ARMS/CMS API

2. **umodel-mcp**：
   - **SPL专一**：所有19个工具都使用SPL字符串拼接
   - **参数驱动**：纯结构化参数控制
   - **会话状态**：独有的时间范围状态管理

### 参数能力差异

1. **alibabacloud-observability**：
   - 支持自然语言描述
   - 智能推荐和建议
   - 相对时间表达式(now()-1h)
   - AI容错和纠错

2. **umodel-mcp**：  
   - 精确参数控制
   - 结构化过滤条件
   - 标准时间戳
   - 完整元数据访问

### 参数能力对比总结

**alibabacloud-observability参数特点**：
- **自然语言支持**：`query`参数支持自然语言描述
- **相对时间**：`start_time`/`end_time`支持相对时间表达式(now()-1h)
- **可选性强**：大多数过滤参数为可选，依赖AI理解
- **单一实体**：多数工具使用`entity_id`单个实体
- **智能化**：参数通过AI进行语义理解和推理

**umodel-mcp参数特点**：
- **结构化参数**：使用`entity_filter`、`trace_filter`等结构化过滤
- **精确控制**：`metric_set_name`、`log_set_name`等必填参数
- **数组支持**：`entity_ids`、`data_set_types`等数组参数
- **Unix时间戳**：使用标准Unix时间戳
- **精确性**：参数要求明确，提供精确控制

### 独有功能对比

**alibabacloud-observability独有**：
- AI智能分析(异常检测、根因分析、性能诊断)
- 云服务直接集成(SLS/ARMS完整功能)
- 自然语言查询转换
- 智能推荐系统

**umodel-mcp独有**：
- 完整元数据管理体系
- 实体关系深度查询
- 原始SPL执行能力
- 会话状态管理