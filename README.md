# 阿里云可观测 MCP Server（Go 版）

<p align="center">
  <a href="./README.md"><img alt="中文" src="https://img.shields.io/badge/简体中文-d9d9d9"></a>
  <a href="./README_EN.md"><img alt="English" src="https://img.shields.io/badge/English-d9d9d9"></a>
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

## 从源码构建

```bash
git clone https://github.com/aliyun/alibabacloud-observability-mcp-server.git
cd alibabacloud-observability-mcp-server
make build
```

## 配置

配置采用两层结构：

1. `config.yaml` - 服务器配置（传输模式、日志、网络等）
2. `.env` 文件或环境变量 - 凭证和运行时参数

详细的配置项说明请参考 `config.yaml` 中的注释。

## AI 工具集成

### streamable-http 模式（推荐）

1. 配置 `config.yaml`（设置 `server.transport: streamable-http`）
2. 启动服务：`./bin/alibabacloud-observability-mcp-server start`
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

### stdio 模式

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

## 工具集

### 查看完整工具列表

运行以下命令查看当前注册的所有工具：

```bash
./bin/alibabacloud-observability-mcp-server tools
```

### 付费功能说明

以下 AI 智能工具每次调用会产生 STAROps 费用：

| 工具 | 功能 |
|------|------|
| `sls_text_to_sql` | 自然语言转 SQL |
| `sls_text_to_spl` | 自然语言转 SPL |
| `sls_sop` | SLS 智能运维助手 |
| `cms_natural_language_query` | 自然语言数据查询 |

计费详情查看 [STAROps 计费说明](https://www.aliyun.com/price/product#/starops/detail)。如不需要 AI 能力，可在 `config.yaml` 的 `enabled_tools` 中仅启用免费工具。

### 权限要求

| 服务 | 权限文档 | 适用工具 |
|------|---------|------|
| 日志服务 (SLS) | [SLS 权限](https://help.aliyun.com/zh/sls/overview-8) | `sls_*` |
| 应用实时监控 (ARMS) | [ARMS 权限](https://help.aliyun.com/zh/arms/security-and-compliance/overview-8) | `umodel_*` |
| 云监控 (CMS) | [CMS 权限](https://help.aliyun.com/zh/cms/cloudmonitor-2-0/) | `cms_*` |

**特殊权限**：使用 AI 智能工具（如 `sls_text_to_sql`、`cms_natural_language_query`）需要授予 CMS 的 `cms:CreateChat`、`cms:CreateThread` 权限。

### 时间表达式

所有数据查询工具支持灵活的时间范围格式：

| 格式 | 示例 |
|------|------|
| 相对预设 | `last_5m`、`last_1h`、`last_1d` |
| 相对时间 | `now()-1h`、`now-30m` |
| Grafana 风格 | `now-15m~now-5m`、`now/d`、`now-1d/d` |
| 绝对时间戳 | `1718451045`（秒）、`1718451045000`（毫秒） |
| 日期时间字符串 | `2024-01-01 00:00:00`、`2024-01-01T00:00:00Z` |

## 项目结构

```
├── cmd/server/          # CLI 入口
├── pkg/
│   ├── client/          # SLS/CMS 客户端
│   ├── config/          # 配置管理
│   ├── server/          # MCP Server 核心
│   └── toolkit/         # 工具集（PaaS/IaaS/Shared）
└── v1/                  # Python 版本
```

## 安全建议

- 服务不存储 AccessKey，仅在运行时用于 API 调用
- SSE/HTTP 模式下需自行做好访问控制
- 建议部署在内网或 VPC，避免暴露公网
- 推荐使用函数计算 (FC) 部署，仅 VPC 内访问

## Skill 自动化部署

本项目提供 AI Agent Skill，支持通过自然语言指令完成部署、启动和更新。

### 安装 Skill

**方式一：npx skills add（推荐）**

由 [Vercel Labs](https://github.com/vercel-labs/skills) 维护的通用 skill 安装器，支持 69+ 种 coding agents：

```bash
# 全局安装（所有项目可用）
npx skills add aliyun/alibabacloud-observability-mcp-server

# 项目级安装（仅当前项目可用）
npx skills add aliyun/alibabacloud-observability-mcp-server --local
```

**方式二：curl（无需 Node.js）**

```bash
# 下载并安装 skill 到 ~/.claude/skills/
curl -fsSL https://raw.githubusercontent.com/aliyun/alibabacloud-observability-mcp-server/master/skills/deploy-observability/SKILL.md -o ~/.claude/skills/deploy-observability/SKILL.md
```

**方式三：从项目仓库复制**

如果你已经克隆了本项目：

```bash
# 全局安装
cp skills/deploy-observability/SKILL.md ~/.claude/skills/deploy-observability/

# 或项目级安装（在项目根目录下）
cp skills/deploy-observability/SKILL.md .claude/skills/deploy-observability/
```

### 使用 Skill

安装 skill 后，将以下内容复制到支持 Skill 的 AI Agent（如 Claude Code）中：

**首次部署**：
```
请使用 deploy-observability skill 帮我完成以下操作：
1. 克隆项目到 ~/alibabacloud-observability-mcp-server
2. 下载依赖并构建
3. 复制 .env.example 为 .env，并提示我填写阿里云 AccessKey
4. 让我选择启动模式（stdio / sse / streamable-http）
5. 生成 JSON 配置，供我粘贴到 AI Agent 中使用
```

**项目更新**：
```
请使用 deploy-observability skill 帮我更新项目：
1. 拉取最新代码
2. 检测是否有依赖、配置或环境变量变更
3. 对比工具列表变化，告诉我新增/删除了哪些工具
4. 重新构建并重启服务
```

详细使用文档请参考 [skills/deploy-observability/SKILL.md](skills/deploy-observability/SKILL.md)。

## 许可证

本项目遵循与原 Python 版相同的许可协议。
