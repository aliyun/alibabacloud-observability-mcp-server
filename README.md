# 阿里云可观测 MCP Server（Go 版）

<p align="center">
  <a href="./README.md"><img alt="中文" src="https://img.shields.io/badge/简体中文-d9d9d9"></a>
</p>

---

> **📌 重要提示**
>
> 本项目已使用 **Go 语言重构**。如需使用原 Python 版本，请访问 [`v1`](./v1) 目录：
> - 📖 [v1/README.md](./v1/README.md) - Python 版本文档
> - 📦 Python 版本通过 `pip install mcp-server-aliyun-observability` 安装

---

阿里云可观测 MCP Server 的 Go 语言实现，为 AI 模型提供对阿里云日志服务（SLS）和云监控（CMS）的结构化数据访问能力。基于 [Model Context Protocol](https://modelcontextprotocol.io/) 协议，可与 Cursor、Kiro、Cline、Windsurf 等 AI 工具无缝集成。

## 特性

- 支持 stdio、SSE、streamable-http 三种传输模式
- 模块化工具集架构：PaaS（云监控 2.0）、IaaS（SLS/CMS 直接访问）、Shared
- 灵活的时间表达式解析：相对时间、绝对时间戳、Grafana 风格、预设关键词
- 时序数据对比分析：统计计算、趋势分析、差异评分
- 结构化错误处理：英文错误描述和解决方案建议
- 稳定性保障：重试（指数退避）、熔断器、优雅关闭
- 结构化 JSON 日志（slog）
- 单一二进制文件，零运行时依赖

## 快速开始

### 下载与安装

从 [Releases](https://github.com/aliyun/alibabacloud-observability-mcp-server/releases) 页面下载对应平台的二进制文件：

```bash
# Linux amd64
wget https://github.com/aliyun/alibabacloud-observability-mcp-server/releases/latest/download/alibabacloud-observability-mcp-server-linux-amd64.tar.gz
tar -xzf alibabacloud-observability-mcp-server-linux-amd64.tar.gz

# macOS arm64 (M1/M2)
wget https://github.com/aliyun/alibabacloud-observability-mcp-server/releases/latest/download/alibabacloud-observability-mcp-server-darwin-arm64.tar.gz
tar -xzf alibabacloud-observability-mcp-server-darwin-arm64.tar.gz
```

解压后包含：
- `alibabacloud-observability-mcp-server` - 可执行文件
- `config.yaml` - 默认配置文件

### 配置凭证

```bash
# 设置阿里云 AccessKey
export ALIBABA_CLOUD_ACCESS_KEY_ID=<your_access_key_id>
export ALIBABA_CLOUD_ACCESS_KEY_SECRET=<your_access_key_secret>
```

> AccessKey 获取方式：[阿里云 AccessKey 管理](https://help.aliyun.com/document_detail/53045.html)

### 启动服务

```bash
# 以 stdio 模式启动（MCP 客户端直接调用）
./alibabacloud-observability-mcp-server start --stdio

# 以网络模式启动（默认 transport 在 config.yaml 中配置）
./alibabacloud-observability-mcp-server start --config config.yaml
```

### CLI 命令

```bash
# 查看版本信息
./alibabacloud-observability-mcp-server version

# 列出所有已注册工具
./alibabacloud-observability-mcp-server tools
```

---

## 从源码构建

### 前置条件

- Go 1.23+

### 构建

```bash
# 克隆仓库
git clone https://github.com/aliyun/alibabacloud-observability-mcp-server.git
cd alibabacloud-observability-mcp-server

# 构建当前平台
make build

# 构建所有平台（linux/darwin/windows × amd64/arm64）
make build-all
```

生成的二进制文件位于 `bin/` 目录。

## 配置

配置采用两层结构：

1. `config.yaml` - 服务器配置（传输模式、日志、网络等）
2. `.env` 文件或环境变量 - 凭证和运行时参数

### 配置文件

```bash
cp config.yaml config.yaml.bak       # 备份默认配置（可选）
cp .env.example .env                  # 凭证（AccessKey）
```

`config.yaml` 搜索路径：当前目录 → `./config/`

`.env` 文件从当前目录加载，适合存放不宜提交到版本控制的凭证信息。

### config.yaml 结构

```yaml
# 服务器配置
server:
  transport: streamable-http  # stdio, sse, streamable-http
  host: "0.0.0.0"
  port: 8080

# 日志配置
logging:
  level: info                 # debug, info, warn, error
  debug_mode: false

# 工具集配置
toolkit:
  scope: all                  # all, paas, iaas
  # 精细化工具选择（可选，非空时仅注册列表中的工具）
  # enabled_tools:
  #   - list_workspace
  #   - umodel_get_entities
  #   - sls_execute_sql

# 网络配置
network:
  max_retry: 1
  retry_wait_seconds: 1
  read_timeout_ms: 610000
  connect_timeout_ms: 30000

# 本地化配置
locale:
  timezone: Asia/Shanghai
  language: zh-CN

# 运行时默认值（可选）
# 优先级: 环境变量 > .env 文件 > config.yaml
runtime:
  region: cn-hangzhou
  # workspace: ""

# 端点覆盖（可选，用于内网访问）
# endpoints:
#   sls:
#     cn-hongkong: "cn-hongkong-intranet.log.aliyuncs.com"
#   cms:
#     cn-hongkong: "cms.cn-hongkong.aliyuncs.com"
```

#### 精细化工具选择

默认情况下，`toolkit.scope` 控制按类别启用工具（`all`/`paas`/`iaas`）。如果需要更细粒度的控制，可以使用 `toolkit.enabled_tools` 指定要启用的工具列表：

```yaml
toolkit:
  scope: all
  enabled_tools:
    - list_workspace
    - list_domains
    - umodel_get_entities
    - umodel_get_metrics
    - sls_execute_sql
```

当 `enabled_tools` 非空时，只有列表中的工具会被注册，其余工具不可用。`scope` 仍然决定加载哪些 toolkit 模块，`enabled_tools` 在此基础上进一步过滤。

完整的工具列表及分类说明请参考 `config.yaml` 中的注释模板。

### CLI 参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--config` | 指定配置文件路径 | 自动搜索 |
| `--stdio` | 强制使用 stdio 传输模式 | false |

### 环境变量（凭证和运行时参数）

| 环境变量 | 说明 | 必需 |
|---------|------|------|
| `ALIBABA_CLOUD_ACCESS_KEY_ID` | AccessKey ID | 是 |
| `ALIBABA_CLOUD_ACCESS_KEY_SECRET` | AccessKey Secret | 是 |
| `ALIBABA_CLOUD_SECURITY_TOKEN` | STS Token（临时凭证） | 否 |
| `ALIBABA_CLOUD_REGION` | 默认区域 | 否 |
| `ALIBABA_CLOUD_WORKSPACE` | 默认工作空间（PaaS 工具需要） | 否 |

凭证优先从 `.env` 文件读取，如未找到则从 shell 环境变量读取。

> **💡 默认值自动填充**
>
> 当设置了 `ALIBABA_CLOUD_REGION` 或 `ALIBABA_CLOUD_WORKSPACE` 时，如果工具调用中未提供 `regionId` 或 `workspace` 参数，服务会自动使用环境变量中的值作为默认值。用户显式传入的值不会被覆盖。

## AI 工具集成

### Cursor / Kiro / Cline

**streamable-http 模式（推荐）：**

1. 配置 `config.yaml`（设置 `server.transport: streamable-http`）
2. 启动服务：
```bash
./bin/alibabacloud-observability-mcp-server start
```

3. 配置 `mcp.json`：
```json
{
  "mcpServers": {
    "alibaba_cloud_observability": {
      "url": "http://localhost:8080"
    }
  }
}
```

**stdio 模式：**

1. 配置 `mcp.json`：
```json
{
  "mcpServers": {
    "alibaba_cloud_observability": {
      "command": "./bin/alibabacloud-observability-mcp-server",
      "args": ["start", "--stdio"],
      "env": {
        "ALIBABA_CLOUD_ACCESS_KEY_ID": "<your_access_key_id>",
        "ALIBABA_CLOUD_ACCESS_KEY_SECRET": "<your_access_key_secret>"
      }
    }
  }
}
```

注意：stdio 模式下，如果 `config.yaml` 不存在，将使用内置默认值。

## 工具集

共 33 个工具，分为三个层级。

### PaaS 工具集（云监控 2.0，推荐）

基于统一数据模型，工具名以 `umodel_` 或 `cms_` 为前缀。共 16 个工具。

#### 实体管理工具

| 工具 | 说明 | 关键参数 |
|------|------|---------|
| `umodel_get_entities` | 获取实体列表 | `workspace`、`domain`、`entity_set_name`、`regionId`（必需）；`entity_filter`（可选） |
| `umodel_get_neighbor_entities` | 获取实体邻居关系 | `workspace`、`src_entity_domain`、`src_name`、`src_entity_ids`、`regionId`（必需） |
| `umodel_search_entities` | 搜索实体 | `workspace`、`search_text`、`regionId`（必需） |

#### 数据集管理工具

| 工具 | 说明 | 关键参数 |
|------|------|---------|
| `umodel_list_data_set` | 列出数据集 | `workspace`、`domain`、`entity_set_name`、`regionId`（必需）；`data_set_types`（可选） |
| `umodel_search_entity_set` | 搜索实体集合 | `workspace`、`search_text`、`regionId`（必需） |
| `umodel_get_entity_set` | 获取实体集合 Schema 定义 | `domain`、`entity_set_name`、`workspace`、`regionId`（必需）；`detail`（可选） |
| `umodel_list_related_entity_set` | 列出关联实体集合 | `workspace`、`domain`、`entity_set_name`、`regionId`（必需） |

#### 数据查询工具

| 工具 | 说明 | 关键参数 |
|------|------|---------|
| `umodel_get_metrics` | 查询指标数据 | `workspace`、`domain`、`entity_set_name`、`metric_domain_name`、`metric`、`regionId`（必需）；`analysis_mode`（basic/cluster/forecast/anomaly_detection）、`offset`（时序对比）、`time_range`（可选） |
| `umodel_get_golden_metrics` | 查询黄金指标 | `workspace`、`domain`、`entity_set_name`、`regionId`（必需）；`offset`、`time_range`（可选） |
| `umodel_get_relation_metrics` | 查询关系指标 | `workspace`、`src_domain`、`src_entity_set_name`、`relation_type`、`direction`（in/out）、`metric`、`metric_set_domain`、`regionId`（必需）；`dest_entity_set_name`（可选） |
| `umodel_get_logs` | 查询日志数据 | `workspace`、`domain`、`entity_set_name`、`log_set_domain`、`log_set_name`、`regionId`（必需）；`time_range`、`limit`（可选） |
| `umodel_get_events` | 查询事件数据 | `workspace`、`domain`、`entity_set_name`、`event_set_domain`、`event_set_name`、`regionId`（必需）；`time_range`、`limit`（可选） |
| `umodel_get_traces` | 查询链路数据 | `workspace`、`domain`、`entity_set_name`、`trace_set_domain`、`trace_set_name`、`trace_ids`、`regionId`（必需）；`time_range`（可选） |
| `umodel_search_traces` | 搜索链路 | `workspace`、`domain`、`entity_set_name`、`trace_set_domain`、`trace_set_name`、`regionId`（必需）；`conditions`、`limit`、`time_range`（可选） |
| `umodel_get_profiles` | 查询性能剖析数据 | `workspace`、`domain`、`entity_set_name`、`profile_set_domain`、`profile_set_name`、`entity_ids`、`regionId`（必需）；`time_range`、`limit`（可选） |
| `cms_natural_language_query` | 自然语言数据查询 | `query`、`workspace`、`regionId`（必需）；`time_range`（可选） |

### IaaS 工具集（SLS/CMS 直接访问）

直接访问底层 API，工具名以 `sls_` 或 `cms_` 为前缀。共 14 个工具。

#### SLS 工具

| 工具 | 说明 | 关键参数 |
|------|------|---------|
| `sls_list_projects` | 列出项目 | `regionId`（必需）；`project`（可选，模糊搜索） |
| `sls_list_logstores` | 列出日志库 | `project`、`regionId`（必需） |
| `sls_text_to_sql` | 自然语言转 SQL | `text`、`project`、`logStore`、`regionId`（必需） |
| `sls_text_to_sql_old` | 自然语言转 SQL（旧版，兼容 Python 版本） | `text`、`project`、`logStore`、`regionId`（必需） |
| `sls_text_to_promql` | 自然语言转 PromQL | `text`、`project`、`metricStore`、`regionId`（必需） |
| `sls_text_to_spl` | 自然语言转 SPL | `text`、`project`、`logStore`、`data_sample`、`regionId`（必需） |
| `sls_execute_sql` | 执行 SQL 查询 | `project`、`logStore`、`query`、`regionId`（必需）；`from_time`、`to_time`（可选） |
| `sls_execute_spl` | 执行原生 SPL 查询 | `query`、`workspace`、`regionId`（必需）；`from_time`、`to_time`（可选） |
| `sls_get_context_logs` | 获取日志上下文 | `project`、`logStore`、`pack_id`、`pack_meta`、`regionId`（必需）；`back_lines`、`forward_lines`（可选） |
| `sls_log_explore` | 日志探索分析 | `project`、`logStore`、`logField`、`regionId`（必需）；`from_time`、`to_time`、`filter_query`、`groupField`（可选） |
| `sls_log_compare` | 日志对比分析 | `project`、`logStore`、`logField`、`regionId`（必需）；`test_from_time`、`test_to_time`、`control_from_time`、`control_to_time`、`filter_query`、`groupField`（可选） |
| `sls_sop` | SLS 运维助手 | `text`、`regionId`（必需） |

#### CMS 工具

| 工具 | 说明 | 关键参数 |
|------|------|---------|
| `cms_execute_promql` | 执行 PromQL 查询 | `project`、`metricStore`、`query`、`regionId`（必需）；`from_time`、`to_time`（可选） |
| `cms_text_to_promql` | 自然语言转 PromQL | `text`、`project`、`metricStore`、`regionId`（必需） |

### Shared 工具集

共 3 个工具。

| 工具 | 说明 | 关键参数 |
|------|------|---------|
| `list_workspace` | 列出工作空间 | `regionId`（必需） |
| `list_domains` | 列出实体域 | `workspace`、`regionId`（必需） |
| `introduction` | 服务介绍 | 无参数 |

## 时间表达式

所有数据查询工具支持灵活的时间范围格式：

| 格式 | 示例 |
|------|------|
| 相对预设 | `last_5m`、`last_1h`、`last_3d`、`last_1w`、`last_1M`、`last_1y` |
| 相对时间 | `now()-1h`、`now-30m`、`now()-7d` |
| Grafana 风格 | `now-15m~now-5m`、`now/d`、`now-1d/d` |
| 关键词 | `today`、`yesterday` |
| 绝对时间戳 | `1718451045`（秒）、`1718451045000`（毫秒） |
| 日期时间字符串 | `2024-01-01 00:00:00`、`2024-01-01T00:00:00Z` |

## 高级功能

### 时序对比分析

`umodel_get_metrics` 和 `umodel_get_golden_metrics` 支持通过 `offset` 参数进行时序对比：

```
# 对比当前1小时与1天前的数据
umodel_get_metrics(
    domain="apm", entity_set_name="apm.service",
    metric_domain_name="apm.metric.apm.service", metric="request_count",
    time_range="last_1h", offset="1d"
)
```

返回结果包含：
- `current`: 当前时段统计（max, min, avg, count）
- `compare`: 对比时段统计
- `diff`: 变化分析（trend, avg_change, avg_change_percent）
- `diff_score`: 差异评分（0-1，越大差异越显著）

### 高级分析模式

`umodel_get_metrics` 支持四种分析模式：

| 模式 | 说明 | 输出字段 |
|------|------|---------|
| `basic` | 原始时序数据（默认） | `__ts__`, `__value__`, `__labels__` |
| `cluster` | K-Means时序聚类 | `__cluster_index__`, `__entities__`, `__sample_value__` |
| `forecast` | 时序预测（需1-5天历史数据） | `__forecast_ts__`, `__forecast_value__`, `__forecast_lower/upper_value__` |
| `anomaly_detection` | 异常检测（需1-3天数据） | `__anomaly_list_`, `__anomaly_msg__`, `__value_min/max/avg__` |

## 项目结构

```
├── cmd/server/          # CLI 入口（cobra）
├── pkg/
│   ├── client/          # SLS/CMS 客户端封装
│   ├── config/          # 配置管理（viper + sync.Once）
│   ├── endpoint/        # 端点解析
│   ├── errors/          # 结构化错误与错误码映射
│   ├── logger/          # 结构化日志（slog）
│   ├── server/          # MCP Server 核心（传输层、生命周期、健康检查）
│   ├── stability/       # 重试与熔断器
│   ├── timeparse/       # 时间表达式解析
│   └── toolkit/         # 工具集接口与注册中心
│       ├── paas/        # PaaS 工具集（umodel_*、cms_natural_language_query）
│       ├── iaas/        # IaaS 工具集（sls_*、cms_execute_promql、cms_text_to_promql）
│       └── shared/      # Shared 工具集（list_workspace、list_domains、introduction）
├── v1/                  # Python 版本（历史参考）
├── Makefile
├── go.mod
└── go.sum
```

## 开发

```bash
# 构建
make build

# 运行测试
make test

# 代码检查
make lint

# 清理构建产物
make clean
```

### 测试

项目采用单元测试 + 属性测试 + 回归测试三轨策略：

- 单元测试：表驱动测试，覆盖具体示例和边界条件
- 属性测试：使用 [gopter](https://github.com/leanovate/gopter)，验证跨所有输入的通用正确性属性
- 回归测试：集成测试（`//go:build integration`），对比 Python 版本参数一致性，需要真实阿里云凭证

```bash
# 运行所有单元测试
go test ./... -v

# 仅运行属性测试
go test ./... -run TestProperty_

# 运行回归测试（需要配置环境变量）
ALIBABA_CLOUD_ACCESS_KEY_ID=xxx \
ALIBABA_CLOUD_ACCESS_KEY_SECRET=xxx \
ALIBABA_CLOUD_REGION=cn-hongkong \
ALIBABA_CLOUD_WORKSPACE=xxx \
go test -tags=integration ./pkg/toolkit/... -v
```

### AI Agent 开发规范

参见 [docs/AGENTS.md](docs/AGENTS.md)，包含项目结构说明、代码风格约定、新增工具流程、测试规范等。

## 权限要求

为了确保 MCP Server 能够成功访问和操作您的阿里云可观测性资源，您需要配置以下权限：

### 阿里云访问密钥 (AccessKey)

- 服务运行需要有效的阿里云 AccessKey ID 和 AccessKey Secret
- 获取和管理 AccessKey，请参考 [阿里云 AccessKey 管理官方文档](https://help.aliyun.com/document_detail/53045.html)
- 支持使用 STS Token 临时凭证（设置 `ALIBABA_CLOUD_SECURITY_TOKEN` 环境变量）

### RAM 授权

与 AccessKey 关联的 RAM 用户或角色**必须**被授予访问相关云服务所需的权限。

**强烈建议遵循"最小权限原则"**：仅授予运行您计划使用的 MCP 工具所必需的最小权限集。

根据您需要使用的工具，参考以下文档进行权限配置：

| 服务 | 权限文档 | 说明 |
|------|---------|------|
| 日志服务 (SLS) | [SLS 权限说明](https://help.aliyun.com/zh/sls/overview-8) | `sls_*` 工具需要 |
| 应用实时监控 (ARMS) | [ARMS 权限说明](https://help.aliyun.com/zh/arms/security-and-compliance/overview-8) | `umodel_*` 工具需要 |
| 云监控 (CMS) | [CMS 权限说明](https://help.aliyun.com/zh/cms/cloudmonitor-2-0/) | `cms_*` 工具需要 |

**特殊权限说明**：
- 使用 SQL 生成类工具（如 `sls_text_to_sql`）需要单独授予 `sls:CallAiTools` 权限
- 使用自然语言查询功能（`cms_natural_language_query`）需要授予：`cms:CreateChat`、`cms:CreateThread`、`cms:GetThread`、`cms:ListThreads`

## 安全建议

- 服务不会存储 AccessKey，仅在运行时用于 API 调用
- SSE/HTTP 模式下，务必自行做好接入点的访问控制
- 建议部署在内部网络或 VPC 内，避免直接暴露于公网
- 切勿在无认证的情况下将配置了 AccessKey 的服务端点暴露在公网
- 推荐使用阿里云函数计算 (FC) 部署，并配置为仅 VPC 内访问

## 许可证

本项目遵循与原 Python 版相同的许可协议。
