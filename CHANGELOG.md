# 版本更新

## 1.0.5 (2026-01-13)
### 新功能
- `sls_execute_sql` 工具新增分页查询支持
  - 新增 `offset` 参数：查询开始行，用于分页查询，默认 0
  - 新增 `reverse` 参数：是否按日志时间戳降序返回，默认 False
  - 支持获取超过 100 条的完整日志数据（通过多次分页调用）
  - 完全向后兼容，新参数均为可选且有默认值

### 测试改进
- 新增 SLS 集成测试框架，支持测试环境自动创建和销毁
  - 新增 `conftest.py`：自动创建/销毁 Project、Logstore、索引和测试数据
  - 新增 `test_iaas_integration.py`：分页查询、排序、向后兼容性等集成测试
- 为所有需要真实凭证的测试添加 `@pytest.mark.integration` 标记
  - 无凭证环境下自动跳过集成测试，不影响 CI 流水线
- `pytest.ini` 新增 `integration` marker 定义

### 文档更新
- 完善 `CONTRIBUTION.md` 测试指引
  - 新增测试分类说明（单元测试 vs 集成测试）
  - 新增环境准备和测试命令文档
  - 新增 PR 提交检查清单
  - 新增测试用例编写示例和目录结构说明

## 1.0.4 (2025-12-26)
### 修复
- `umodel_get_relation_metrics` 工具修复 `get_relation_metric` 查询参数错误
  - 修复前两个参数错误传入空值的问题，现在正确传入 `src_domain` 和 `src_entity_set_name`
  - 新增 `metric_set_name` 自动拼接逻辑：自动拼接 `_{relation_type}_{src_entity_set_name}` 后缀
  - 校验逻辑调整到拼接之前，确保校验用户传入的原始值
  - 添加 `metric_set_name` 拼接逻辑的单元测试

## 1.0.3 (2025-12-09)
### 新功能
- 新增 `sls_log_explore` 工具，支持日志数据概览与聚合分析
  - 提供日志数据概览信息，展示日志模板及数量分布
  - 支持按字段分组统计，快速了解日志数据分布特征
  - 支持自定义过滤查询，灵活筛选目标日志数据
  - 适用场景：日志数据探索、日志分布分析、风险等级统计等

## 1.0.2
### 新功能
- `umodel_get_metrics` 工具新增高级分析模式支持
  - **cluster (时序聚类)**: 使用 K-Means 算法对多实体指标进行聚类分析，自动识别相似行为模式
    - 输出: `__cluster_index__`, `__entities__`, `__sample_ts__`, `__sample_value__`, `__sample_value_max/min/avg__`
    - 聚类数根据实体数量自动计算 (2-7)
  - **forecast (时序预测)**: 基于历史数据预测未来指标趋势，支持自定义预测时长
    - 输出: `__forecast_ts__`, `__forecast_value__`, `__forecast_lower/upper_value__`, `__labels__`, `__name__`, `__entity_id__`
    - 自动调整学习时间范围 (1-5天)
    - 新增 `forecast_duration` 参数，支持 `30m`, `1h`, `2d` 等格式
  - **anomaly_detection (异常检测)**: 使用时序分解算法识别指标中的异常点
    - 输出: `__entity_id__`, `__anomaly_list_`, `__anomaly_msg__`, `__value_min/max/avg__`
    - 自动调整学习时间范围 (1-3天)

- `umodel_get_traces` 工具新增独占耗时计算
  - 新增输出字段 `exclusive_duration_ms`：span 独占耗时（排除子 span 后的实际执行时间）
  - 结果按独占耗时降序排序，便于快速定位性能瓶颈

## 1.0.0 (重建)
- `master` 已重建为 `1.x.x` 最新内容的单提交快照，旧 `master` 历史迁移至 `0.3.x` 分支。
- README 增加分支说明、工具差异对照表，明确后续基于 1.x.x 维护。
- `.gitignore` 补充 `docs/`、`agents.md` 以避免无意提交。

## 0.2.9
- 修复获取logstore时候类型不匹配问题
## 0.2.8
- 增加 streamable-http 支持，可通过 --transport streamable-http 指定
- 增加 host 参数，可通过 --host 指定 MCP Server 的监听地址
- 重构日志系统，使用统一的Logger类替换标准logging
  - 新增自定义MCPLogger类，支持居中显示和富文本格式
  - 所有toolkit模块统一使用新的日志函数(log_error, log_info等)
  - 日志文件自动保存到用户目录~/mcp_server_aliyun_observability/，按日期命名
  - 支持终端彩色输出和文件日志双重记录

## 0.2.7
- 修复sls_list_projects 工具返回结果类型错误问题,会导致高版本的MCP出现返回值提取错误

## 0.2.6
- 增加 用户私有知识库 RAG 支持，在启动 MCP Server 时，设置可选参数--knowledge-config ./knowledge_config.json，配置文件样例请参见sample/config/knowledge_config.json

## 0.2.5
- 增加 ARMS 慢 Trace 分析工具

## 0.2.4
- 增加 ARMS 火焰图工具,支持单火焰图分析以及差分火焰图

## 0.2.3
- 增加 ARMS 应用详情工具
- 优化一些tool 的命名，更加规范，提升模型解析成功率

## 0.2.2
- 优化 SLS 查询工具，时间范围不显示传入，由SQL 生成工具直接返回判定
- sls_list_projects 工具增加个数限制，并且做出提示

## 0.2.1
- 优化 SLS 查询工具，增加 from_timestamp 和 to_timestamp 参数，确保查询语句的正确性
- 增加 SLS 日志查询的 prompts

## 0.2.0
- 增加 cms_translate_natural_language_to_promql 工具，根据自然语言生成 promql 查询语句

## 0.1.9
- 支持 STS Token 方式登录，可通过环境变量ALIBABA_CLOUD_SECURITY_TOKEN 指定
- 修改 README.md 文档，增加 Cursor，Cline 等集成说明以及 UV 命令等说明

## 0.1.8
- 优化 SLS 列出日志库工具，添加日志库类型验证，确保参数符合规范


## 0.1.7
- 优化错误处理机制，简化错误代码，提高系统稳定性
- 改进 SLS 日志服务相关工具
    - 增强 sls_list_logstores 工具，添加日志库类型验证，确保参数符合规范
    - 完善日志库类型描述，明确区分日志类型(logs)和指标类型(metrics)
    - 优化指标类型日志库筛选逻辑，仅当用户明确需要时才返回指标类型

## 0.1.6
### 工具列表
- 增加 SQL 诊断工具, 当 SLS 查询语句执行失败时，可以调用该工具，根据错误信息，生成诊断结果。诊断结果会包含查询语句的正确性、性能分析、优化建议等信息。


## 0.1.0
本次发布版本为 0.1.0，以新增工具为主，主要包含 SLS 日志服务和 ARMS 应用实时监控服务相关工具。


### 工具列表

- 增加 SLS 日志服务相关工具
    - `sls_describe_logstore`
        - 获取 SLS Logstore 的索引信息
    - `sls_list_projects`
        - 获取 SLS 项目列表
    - `sls_list_logstores`
        - 获取 SLS Logstore 列表
    - `sls_describe_logstore`
        - 获取 SLS Logstore 的索引信息
    - `sls_execute_query`
        - 执行SLS 日志查询
    - `sls_translate_natural_language_to_query`
        - 翻译自然语言为SLS 查询语句

- 增加 ARMS 应用实时监控服务相关工具
    - `arms_search_apps`
        - 搜索 ARMS 应用
    - `arms_generate_trace_query`
        - 根据自然语言生成 trace 查询语句

### 场景举例

- 场景一: 快速查询某个 logstore 相关结构
    - 使用工具:
        - `sls_list_logstores`
        - `sls_describe_logstore`
    ![image](./images/search_log_store.png)


- 场景二: 模糊查询最近一天某个 logstore下面访问量最高的应用是什么
    - 分析:
        - 需要判断 logstore 是否存在
        - 获取 logstore 相关结构
        - 根据要求生成查询语句(对于语句用户可确认修改)
        - 执行查询语句
        - 根据查询结果生成响应
    - 使用工具:
        - `sls_list_logstores`
        - `sls_describe_logstore`
        - `sls_translate_natural_language_to_query`
        - `sls_execute_query`
    ![image](./images/fuzzy_search_and_get_logs.png)

    
- 场景三: 查询 ARMS 某个应用下面响应最慢的几条 Trace
    - 分析:
        - 需要判断应用是否存在
        - 获取应用相关结构
        - 根据要求生成查询语句(对于语句用户可确认修改)
        - 执行查询语句
        - 根据查询结果生成响应
    - 使用工具:
        - `arms_search_apps`
        - `arms_generate_trace_query`
        - `sls_translate_natural_language_to_query`
        - `sls_execute_query`
    ![image](./images/find_slowest_trace.png)
