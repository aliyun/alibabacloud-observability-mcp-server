# 版本更新

## 0.3.2
- 新增全局配置 Settings：支持 SLS/ARMS endpoint 映射与模板回退
  - 启动参数：`--sls-endpoints`、`--arms-endpoints`（仅支持 `REGION=HOST`，逗号/空格分隔）比如 `--sls-endpoints "cn-shanghai=cn-hangzhou.log.aliyuncs.com"`
  - 未命中映射时回退：SLS `"{region}.log.aliyuncs.com"`，ARMS `"arms.{region}.aliyuncs.com"`
- 统一客户端端点解析，并打印使用的 region/endpoint/source（explicit/mapping/template）
- CLI 清理：仅保留 `--sls-endpoints`（移除其他变体），不再支持环境变量与 @file 加载

## 0.3.1
- 修复ARMS工具返回结果类型错误问题,会导致高版本的MCP出现返回值提取错误

## 0.3.0
- 修改日志库依赖，使用 rich 替换标准 logging

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
