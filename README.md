## 阿里云可观测MCP服务
<p align="center">
  <a href="./README.md"><img alt="中文自述文件" src="https://img.shields.io/badge/简体中文-d9d9d9""></a>
  <a href="./README_EN.md"><img alt="英文自述文件" src="https://img.shields.io/badge/English-d9d9d9")
</p>

### 简介

阿里云可观测 MCP服务，提供了一系列访问阿里云可观测各产品的工具能力，覆盖产品包含阿里云日志服务SLS、阿里云应用实时监控服务ARMS、阿里云云监控等，任意支持 MCP 协议的智能体助手都可快速接入。

目前提供的 MCP 工具以阿里云日志服务为主，其他产品会陆续支持，工具详细如下:

### 版本记录
可以查看 [CHANGELOG.md](./CHANGELOG.md)

### 常见问题
可以查看 [FAQ.md](./FAQ.md)

### 工具列表
#### 日志相关
| 工具名称 | 用途 | 关键参数 | 最佳实践 |  
|---------|------|---------|---------|  
| `sls_list_projects` | 列出SLS项目，支持模糊搜索和分页 | `projectName`：项目名称（可选，模糊搜索）<br>`limit`：返回项目数量上限（默认50，范围1-100）<br>`regionId`：阿里云区域ID | - 在不确定可用项目时，首先使用此工具<br>- 使用合理的`limit`值避免返回过多结果 |  
| `sls_list_logstores` | 列出项目内的日志存储，支持名称模糊搜索 | `project`：SLS项目名称（必需）<br>`logStore`：日志存储名称（可选，模糊搜索）<br>`limit`：返回结果数量上限（默认10）<br>`isMetricStore`：是否筛选指标存储<br>`logStoreType`：日志存储类型<br>`regionId`：阿里云区域ID | - 确定项目后使用此工具查找相关日志存储<br>- 可通过`logStoreType`筛选特定类型日志存储 |  
| `sls_describe_logstore` | 检索日志存储的结构和索引信息 | `project`：SLS项目名称（必需）<br>`logStore`：SLS日志存储名称（必需）<br>`regionId`：阿里云区域ID | - 在查询前使用此工具了解可用字段及其类型<br>- 检查所需字段是否启用了索引 |  
| `sls_execute_sql_query` | 在指定时间范围内对日志存储执行SQL查询 | `project`：SLS项目名称（必需）<br>`logStore`：SLS日志存储名称（必需）<br>`query`：SQL查询语句（必需）<br>`fromTimestampInSeconds`：查询开始时间戳（必需）<br>`toTimestampInSeconds`：查询结束时间戳（必需）<br>`limit`：返回结果数量上限（默认10）<br>`regionId`：阿里云区域ID | - 使用适当的时间范围优化查询性能<br>- 限制返回结果数量避免获取过多数据 |  
| `sls_translate_text_to_sql_query` | 将自然语言描述转换为SLS SQL查询语句 | `text`：查询的自然语言描述（必需）<br>`project`：SLS项目名称（必需）<br>`logStore`：SLS日志存储名称（必需）<br>`regionId`：阿里云区域ID | - 适用于不熟悉SQL语法的用户<br>- 对于复杂查询，可能需要优化生成的SQL |  
| `sls_diagnose_query` | 诊断SLS查询问题，提供失败原因分析 | `query`：待诊断的SLS查询（必需）<br>`errorMessage`：查询失败的错误信息（必需）<br>`project`：SLS项目名称（必需）<br>`logStore`：SLS日志存储名称（必需）<br>`regionId`：阿里云区域ID | - 查询失败时使用此工具了解根本原因<br>- 根据诊断建议修改查询语句 |  

##### 应用相关
| 工具名称 | 用途 | 关键参数 | 最佳实践 |  
|---------|------|---------|---------|  
| `arms_search_apps` | 根据应用名称搜索ARMS应用 | `appNameQuery`: 应用名称查询字符串（必需）<br>`regionId`: 阿里云区域ID（必需，格式：'cn-hangzhou'）<br>`pageSize`: 每页结果数量（默认：20，范围：1-100）<br>`pageNumber`: 页码（默认：1） | - 用于查找特定名称的应用<br>- 用于获取其他ARMS操作所需的应用PID<br>- 使用合理的分页参数优化查询结果<br>- 查看用户拥有的应用列表 |  
| `arms_generate_trace_query` | 根据自然语言问题生成ARMS追踪数据的SLS查询 | `user_id`: 阿里云账户ID（必需）<br>`pid`: 应用PID（必需）<br>`region_id`: 阿里云区域ID（必需）<br>`question`: 关于追踪的自然语言问题（必需） | - 用于查询应用的追踪信息<br>- 分析应用性能问题<br>- 跟踪特定请求的执行路径<br>- 分析服务调用关系<br>- 集成了自动重试机制处理瞬态错误 |  
| `arms_get_application_info` | 获取特定ARMS应用的详细信息 | `pid`: 应用PID（必需）<br>`regionId`: 阿里云区域ID（必需） | - 当用户明确请求应用信息时使用<br>- 确定应用的开发语言<br>- 在执行其他操作前先获取应用基本信息 |  
| `arms_profile_flame_analysis` | 分析ARMS应用火焰图性能热点 | `pid`: 应用PID（必需）<br>`startMs`: 分析开始时间戳（必需）<br>`endMs`: 分析结束时间戳（必需）<br>`profileType`: 分析类型，如'cpu'、'memory'（默认：'cpu'）<br>`ip`: 服务主机IP（可选）<br>`thread`: 线程ID（可选）<br>`threadGroup`: 线程组（可选）<br>`regionId`: 阿里云区域ID（必需） | - 用于分析应用性能热点问题<br>- 支持CPU和内存类型的性能分析<br>- 可筛选特定IP、线程或线程组<br>- 适用于Java和Go应用 |
| `arms_diff_profile_flame_analysis` | 对比不同时间段的火焰图性能变化 | `pid`: 应用PID（必需）<br>`currentStartMs`: 当前时间段开始时间戳（必需）<br>`currentEndMs`: 当前时间段结束时间戳（必需）<br>`referenceStartMs`: 参考时间段开始时间戳（必需）<br>`referenceEndMs`: 参考时间段结束时间戳（必需）<br>`profileType`: 分析类型，如'cpu'、'memory'（默认：'cpu'）<br>`ip`: 服务主机IP（可选）<br>`thread`: 线程ID（可选）<br>`threadGroup`: 线程组（可选）<br>`regionId`: 阿里云区域ID（必需） | - 用于发布前后性能对比<br>- 分析性能优化效果<br>- 识别性能退化点<br>- 支持CPU和内存类型的性能对比<br>- 适用于Java和Go应用 |

##### 指标相关

| 工具名称 | 用途 | 关键参数 | 最佳实践 |  
|---------|------|---------|---------|  
| `cms_translate_text_to_promql` | 将自然语言描述转换为PromQL查询语句 | `text`: 要转换的自然语言文本（必需）<br>`project`: SLS项目名称（必需）<br>`metricStore`: SLS指标存储名称（必需）<br>`regionId`: 阿里云区域ID（必需） | - 提供清晰、具体的指标描述<br>- 如已知，可在描述中提及特定的指标名称、标签或操作<br>- 排除项目或指标存储名称本身<br>- 检查并优化生成的查询以提高准确性和性能 |


### 权限要求

为了确保 MCP Server 能够成功访问和操作您的阿里云可观测性资源，您需要配置以下权限：

1.  **阿里云访问密钥 (AccessKey)**：
    *   服务运行需要有效的阿里云 AccessKey ID 和 AccessKey Secret。
    *   获取和管理 AccessKey，请参考 [阿里云 AccessKey 管理官方文档](https://help.aliyun.com/document_detail/53045.html)。
  
2. 当你初始化时候不传入 AccessKey 和 AccessKey Secret 时，会使用[默认凭据链进行登录](https://www.alibabacloud.com/help/zh/sdk/developer-reference/v2-manage-python-access-credentials#62bf90d04dztq)
   1. 如果环境变量 中的ALIBABA_CLOUD_ACCESS_KEY_ID 和 ALIBABA_CLOUD_ACCESS_KEY_SECRET均存在且非空，则使用它们作为默认凭据。
   2. 如果同时设置了ALIBABA_CLOUD_ACCESS_KEY_ID、ALIBABA_CLOUD_ACCESS_KEY_SECRET和ALIBABA_CLOUD_SECURITY_TOKEN，则使用STS Token作为默认凭据。
   
3.  **RAM 授权 (重要)**：
    *   与 AccessKey 关联的 RAM 用户或角色**必须**被授予访问相关云服务所需的权限。
    *   **强烈建议遵循"最小权限原则"**：仅授予运行您计划使用的 MCP 工具所必需的最小权限集，以降低安全风险。
    *   根据您需要使用的工具，参考以下文档进行权限配置：
        *   **日志服务 (SLS)**：如果您需要使用 `sls_*` 相关工具，请参考 [日志服务权限说明](https://help.aliyun.com/zh/sls/overview-8)，并授予必要的读取、查询等权限。
        *   **应用实时监控服务 (ARMS)**：如果您需要使用 `arms_*` 相关工具，请参考 [ARMS 权限说明](https://help.aliyun.com/zh/arms/security-and-compliance/overview-8?scm=20140722.H_74783._.OR_help-T_cn~zh-V_1)，并授予必要的查询权限。
    * 特殊权限说明，如果使用了SQL生成之类的工具，需要单独授予`sls:CallAiTools`的权限
    *   请根据您的实际应用场景，精细化配置所需权限。

  

### 安全与部署建议

请务必关注以下安全事项和部署最佳实践：

1.  **密钥安全**：
    *   本 MCP Server 在运行时会使用您提供的 AccessKey 调用阿里云 OpenAPI，但**不会以任何形式存储您的 AccessKey**，也不会将其用于设计功能之外的任何其他用途。

2.  **访问控制 (关键)**：
    *   当您选择通过 **SSE (Server-Sent Events) 协议** 访问 MCP Server 时，**您必须自行负责该服务接入点的访问控制和安全防护**。
    *   **强烈建议**将 MCP Server 部署在**内部网络或受信环境**中，例如您的私有 VPC (Virtual Private Cloud) 内，避免直接暴露于公共互联网。
    *   推荐的部署方式是使用**阿里云函数计算 (FC)**，并配置其网络设置为**仅 VPC 内访问**，以实现网络层面的隔离和安全。
    *   **注意**：**切勿**在没有任何身份验证或访问控制机制的情况下，将配置了您 AccessKey 的 MCP Server SSE 端点暴露在公共互联网上，这会带来极高的安全风险。

### 使用说明


在使用 MCP Server 之前，需要先获取阿里云的 AccessKeyId 和 AccessKeySecret，请参考 [阿里云 AccessKey 管理](https://help.aliyun.com/document_detail/53045.html)


#### 使用 pip 安装
> ⚠️ 需要 Python 3.10 及以上版本。

直接使用 pip 安装即可，安装命令如下：

```bash
pip install mcp-server-aliyun-observability
```
1. 安装之后，直接运行即可，运行命令如下：

```bash
python -m mcp_server_aliyun_observability --transport sse --access-key-id <your_access_key_id> --access-key-secret <your_access_key_secret>
```
可通过命令行传递指定参数:
- `--transport` 指定传输方式，可选值为 `sse` 或 `stdio`，默认值为 `stdio`
- `--access-key-id` 指定阿里云 AccessKeyId，不指定时会使用环境变量中的ALIBABA_CLOUD_ACCESS_KEY_ID
- `--access-key-secret` 指定阿里云 AccessKeySecret，不指定时会使用环境变量中的ALIBABA_CLOUD_ACCESS_KEY_SECRET
- `--log-level` 指定日志级别，可选值为 `DEBUG`、`INFO`、`WARNING`、`ERROR`，默认值为 `INFO`
- `--transport-port` 指定传输端口，默认值为 `8000`,仅当 `--transport` 为 `sse` 时有效

2. 使用uv 命令启动
   可以指定下版本号，会自动拉取对应依赖，默认是 studio 方式启动
```bash
uvx --from 'mcp-server-aliyun-observability==0.2.1' mcp-server-aliyun-observability 
```

3. 使用 uvx 命令启动

```bash
uvx run mcp-server-aliyun-observability
```

### 从源码安装

```bash

# clone 源码
git clone git@github.com:aliyun/alibabacloud-observability-mcp-server.git
# 进入源码目录
cd alibabacloud-observability-mcp-server
# 安装
pip install -e .
# 运行
python -m mcp_server_aliyun_observability --transport sse --access-key-id <your_access_key_id> --access-key-secret <your_access_key_secret>
```


### AI 工具集成

> 以 SSE 启动方式为例,transport 端口为 8888,实际使用时需要根据实际情况修改

#### Cursor，Cline 等集成
1. 使用 SSE 启动方式
```json
{
  "mcpServers": {
    "alibaba_cloud_observability": {
      "url": "http://localhost:7897/sse"
        }
  }
}
```
2. 使用 stdio 启动方式
   直接从源码目录启动,注意
    1. 需要指定 `--directory` 参数,指定源码目录，最好是绝对路径
    2. uv命令 最好也使用绝对路径，如果使用了虚拟环境，则需要使用虚拟环境的绝对路径
```json
{
  "mcpServers": {
    "alibaba_cloud_observability": {
      "command": "uv",
      "args": [
        "--directory",
        "/path/to/your/alibabacloud-observability-mcp-server",
        "run",
        "mcp-server-aliyun-observability"
      ],
      "env": {
        "ALIBABA_CLOUD_ACCESS_KEY_ID": "<your_access_key_id>",
        "ALIBABA_CLOUD_ACCESS_KEY_SECRET": "<your_access_key_secret>"
      }
    }
  }
}
```
1. 使用 stdio 启动方式-从 module 启动
```json
{
  "mcpServers": {
    "alibaba_cloud_observability": {
      "command": "uv",
      "args": [
        "run",
        "mcp-server-aliyun-observability"
      ],
      "env": {
        "ALIBABA_CLOUD_ACCESS_KEY_ID": "<your_access_key_id>",
        "ALIBABA_CLOUD_ACCESS_KEY_SECRET": "<your_access_key_secret>"
      }
    }
  }
}
```

#### Cherry Studio集成

![image](./images/cherry_studio_inter.png)

![image](./images/cherry_studio_demo.png)


#### Cursor集成

![image](./images/cursor_inter.png)

![image](./images/cursor_tools.png)

![image](./images/cursor_demo.png)


#### ChatWise集成

![image](./images/chatwise_inter.png)

![image](./images/chatwise_demo.png)


### 端点映射与全局配置（SLS/ARMS）

支持在启动时通过命令行覆盖各区域的服务端点，并在运行时打印实际生效的区域与端点（source=explicit/mapping/template）。

- SLS 端点映射（CLI）
```
python -m mcp_server_aliyun_observability \
  --transport streamable-http --transport-port 8083 \
  --sls-endpoints "cn-shanghai=cn-hangzhou.log.aliyuncs.com"
```

- ARMS 端点映射（CLI）
```
python -m mcp_server_aliyun_observability --arms-endpoints "cn-shanghai=arms.internal"
```

- 仅支持命令行参数方式（不支持环境变量与 @file 加载）

- 默认回退模板（未命中映射时）：
  - SLS: `{region}.log.aliyuncs.com`
  - ARMS: `arms.{region}.aliyuncs.com`

- CMS 说明：CMS 工具内部使用 SLS 客户端，自动复用 `--sls-endpoints` 的映射。

- 日志示例（控制台与文件均会输出）：
```
SLS endpoint resolved: region=cn-shanghai, endpoint=cn-hangzhou.log.aliyuncs.com, source=mapping
ARMS endpoint resolved: region=cn-shanghai, endpoint=arms.cn-shanghai.aliyuncs.com, source=template
```
