---
name: deploy-observability
description: "Deploy, start, and update the Alibaba Cloud Observability MCP Server (阿里云可观测 MCP Server). Use this skill whenever the user mentions deploying, installing, starting, updating, or configuring the observability MCP server, Alibaba Cloud SLS/CMS MCP tools, or wants to connect Alibaba Cloud monitoring to their AI coding agent. Also trigger when the user says things like 'set up MCP server', 'install observability tools', 'deploy aliyun MCP', 'configure SLS MCP', or 'update MCP server tools'."
---

# Deploy Alibaba Cloud Observability MCP Server

## Overview

This skill guides users through deploying and managing the Alibaba Cloud Observability MCP Server. It covers:
- **First-time deployment** — clone, build, configure credentials, start, generate agent config
- **Docker deployment** — containerized setup for servers
- **Project update** — pull latest code, detect changes, rebuild, report new tools

The server provides 60+ tools for Alibaba Cloud Log Service (SLS), CloudMonitor (CMS), and AI-powered observability. It can run locally (stdio) or as a remote HTTP service (sse/streamable-http).

## Phase 0: Pre-flight Checks

Before starting any deployment, run these checks to understand the environment:

### Detect Existing Installation

```bash
# Check if already installed in common locations
for dir in ~/alibabacloud-observability-mcp-server ~/.alibabacloud-observability-mcp-server /opt/alibabacloud-observability-mcp-server; do
  if [ -d "$dir" ] && [ -f "$dir/bin/alibabacloud-observability-mcp-server" ]; then
    echo "Found installation at: $dir"
    "$dir/bin/alibabacloud-observability-mcp-server" version
  fi
done
```

If an existing installation is found, ask the user:
> 检测到已有安装，是否要更新？还是全新安装到其他位置？

### Check Go Environment

```bash
go version 2>/dev/null && echo "Go OK" || echo "Go NOT FOUND"
```

If Go is not installed:
> Go 未安装（需要 >= 1.23）。请前往 https://go.dev/dl/ 下载安装，或使用包管理器：
> - macOS: `brew install go`
> - Ubuntu/Debian: `sudo apt install golang-go`
> - CentOS: `sudo yum install golang`

### Check Network

```bash
curl -s --connect-timeout 5 https://github.com > /dev/null && echo "GitHub OK" || echo "GitHub UNREACHABLE"
```

If GitHub is unreachable (common in China):
> GitHub 无法访问。请检查网络连接，或使用镜像站：
> - `git clone https://ghproxy.com/https://github.com/aliyun/alibabacloud-observability-mcp-server.git`

---

## Phase 1: First-Time Deployment

Ask the user where to install (default: `~/Projects/alibabacloud-observability-mcp-server`).

There are two ways to get the binary. Let the user choose:

### Step 1.1a — Download Pre-built Binary (fastest, recommended)

```bash
# macOS arm64 (M1/M2/M3/M4)
wget https://github.com/aliyun/alibabacloud-observability-mcp-server/releases/latest/download/alibabacloud-observability-mcp-server-darwin-arm64.tar.gz
tar -xzf alibabacloud-observability-mcp-server-darwin-arm64.tar.gz
cd alibabacloud-observability-mcp-server

# macOS amd64 (Intel)
wget https://github.com/aliyun/alibabacloud-observability-mcp-server/releases/latest/download/alibabacloud-observability-mcp-server-darwin-amd64.tar.gz
tar -xzf alibabacloud-observability-mcp-server-darwin-amd64.tar.gz
cd alibabacloud-observability-mcp-server
```

**⚠️ macOS "无法验证开发者"提示：**
首次运行时，macOS 可能阻止执行并提示"无法验证开发者"。解决方法：
1. 系统设置 → 隐私与安全性 → 找到被阻止的文件 → 点击"仍然允许"
2. 或者在终端运行：`xattr -d com.apple.quarantine bin/alibabacloud-observability-mcp-server`

### Step 1.1b — Clone and Build from Source

```bash
git clone https://github.com/aliyun/alibabacloud-observability-mcp-server.git <install_path>
cd <install_path>
```

**Optional: Pin to a specific release version:**
```bash
# List available versions
git tag -l 'go/v*'

# Checkout a specific version
git checkout go/v0.1.8
```

Build:
```bash
go mod tidy
make build
chmod +x bin/alibabacloud-observability-mcp-server
```

Verify:
```bash
./bin/alibabacloud-observability-mcp-server version
```

If `make build` fails:
- Check Go version: `go version` (needs >= 1.23)
- Try `go mod download` first
- Check for C compiler issues (should not be needed — pure Go)

### Step 1.2 — Configure Credentials

```bash
cp .env.example .env
```

Present the credential table to the user:

| 环境变量 | 说明 | 必需 |
|---------|------|------|
| `ALIBABA_CLOUD_ACCESS_KEY_ID` | 阿里云 AccessKey ID | 是* |
| `ALIBABA_CLOUD_ACCESS_KEY_SECRET` | 阿里云 AccessKey Secret | 是* |
| `ALIBABA_CLOUD_REGION` | 默认地域 | 否（默认 cn-hangzhou） |
| `ALIBABA_CLOUD_WORKSPACE` | 默认工作空间 | 否 |
| `ALIBABA_CLOUD_SECURITY_TOKEN` | STS 临时凭证 | 否 |

> \* 如果在 ECS/FC 上运行且已配置 RAM Role，可以跳过 AccessKey，服务会自动使用默认凭据链。

Tell the user:
> 请编辑 `.env` 文件，填入你的阿里云 AccessKey。
> 获取方式：https://ram.console.aliyun.com/manage/ak

**Wait for the user to confirm they've filled in the credentials.**

### Step 1.3 — Choose Transport Mode

Ask the user to choose:

| 模式 | 说明 | 适用场景 |
|------|------|---------|
| **stdio** | 标准输入输出 | IDE 集成（Cursor / Claude Code 等），本地使用 |
| **sse** | Server-Sent Events | 远程 HTTP 访问 |
| **streamable-http** | HTTP 流式传输 | 远程 HTTP 访问（推荐生产环境） |

**If stdio:**
Tell the user:
> stdio 模式通常由 IDE 自动启动，无需手动运行。直接跳到 Step 1.4 生成配置。

**If sse or streamable-http:**

Edit `config.yaml`:
```yaml
server:
  transport: streamable-http  # 或 sse
  host: 0.0.0.0
  port: 8180
```

Start the server:
```bash
./bin/alibabacloud-observability-mcp-server start --config config.yaml
```

Verify:
```bash
curl http://localhost:8180/health
```

If health check fails:
- Check logs (stderr output)
- Verify `.env` has valid credentials
- Check if port 8180 is already in use: `lsof -i :8180`

### Step 1.4 — Generate Agent Integration Config

Ask the user which AI agent they want to connect to, then generate the config.

**For streamable-http / sse mode (remote server):**

```json
{
  "mcpServers": {
    "alibaba_cloud_observability": {
      "url": "http://<server_ip>:8180"
    }
  }
}
```

**For stdio mode (local):**

```json
{
  "mcpServers": {
    "alibaba_cloud_observability": {
      "command": "<install_path>/bin/alibabacloud-observability-mcp-server",
      "args": ["start", "--stdio"],
      "env": {
        "ALIBABA_CLOUD_ACCESS_KEY_ID": "<your_access_key_id>",
        "ALIBABA_CLOUD_ACCESS_KEY_SECRET": "<your_access_key_secret>"
      }
    }
  }
}
```

Tell the user where to paste the config:

| Agent | 配置文件路径 |
|-------|------------|
| **Cursor** | Settings → MCP → Add Server（UI 操作） |
| **Claude Code** | `~/.claude/settings.json` 或项目 `.claude/settings.json` |
| **Kiro** | Settings → MCP Servers（UI 操作） |
| **Windsurf** | Settings → MCP Configuration |
| **Cline** | `.cline/mcp_settings.json` 在项目根目录 |

> 将以上 JSON 配置粘贴到你的 AI Agent 中。如果是远程服务器，将 `<server_ip>` 替换为实际 IP 地址。

### Step 1.5 — Paid Tools Warning

**IMPORTANT: Inform the user about paid tools before they finish:**

> ⚠️ **付费工具提醒**
>
> 以下 AI 智能工具每次调用会产生 STAROps 费用：
> - `sls_text_to_sql` — 自然语言转 SQL
> - `sls_text_to_spl` — 自然语言转 SPL
> - `sls_sop` — SLS 智能运维助手
> - `cms_natural_language_query` — 自然语言数据查询
>
> 如果不需要 AI 能力，可以在 `config.yaml` 的 `enabled_tools` 中仅启用免费工具，避免意外费用。
>
> 计费详情：https://www.aliyun.com/price/product#/starops/detail

---

## Phase 1b: Docker Deployment (alternative)

For users who prefer containerized deployment (servers, CI/CD, etc.):

### Build and Run with Docker

```bash
# Build image
docker build -t observability-mcp-server .

# Run with environment variables
docker run -d \
  --name observability-mcp \
  -p 8180:8180 \
  -e ALIBABA_CLOUD_ACCESS_KEY_ID=<your_key> \
  -e ALIBABA_CLOUD_ACCESS_KEY_SECRET=<your_secret> \
  -e ALIBABA_CLOUD_REGION=cn-hangzhou \
  observability-mcp-server
```

### Or use Docker Compose

```bash
# Create .env file with credentials
cat > .env << 'EOF'
ALIBABA_CLOUD_ACCESS_KEY_ID=<your_key>
ALIBABA_CLOUD_ACCESS_KEY_SECRET=<your_secret>
ALIBABA_CLOUD_REGION=cn-hangzhou
EOF

# Start
docker-compose up -d
```

Verify:
```bash
curl http://localhost:8180/health
```

After Docker deployment, generate the agent config using `http://<server_ip>:8180` as the URL (same as Step 1.5).

---

## Phase 2: Project Update

When the user wants to update an existing deployment:

### Step 2.1 — Locate Installation

If the user doesn't know the install path:
```bash
# Search for existing installations
find ~ -maxdepth 3 -name "alibabacloud-observability-mcp-server" -type d 2>/dev/null
```

### Step 2.2 — Record Current State

```bash
cd <install_path>

# Save current version and tool list
./bin/alibabacloud-observability-mcp-server version > /tmp/version_before.txt 2>/dev/null || true
./bin/alibabacloud-observability-mcp-server tools > /tmp/tools_before.txt 2>/dev/null || true
```

### Step 2.3 — Pull Latest Code

```bash
git pull origin master
go mod tidy
```

### Step 2.4 — Detect Changes and Report

Run these checks and present results clearly:

**1. Dependency changes:**
```bash
git diff HEAD@{1} --name-only 2>/dev/null | grep -E 'go\.(mod|sum)' && echo "DEPENDENCIES CHANGED" || echo "No dependency changes"
```

**2. Config changes:**
```bash
git diff HEAD@{1} -- config.yaml .env.example 2>/dev/null
```
If `.env.example` changed, show new variables and ask the user to update `.env`.

**3. Rebuild and compare tools:**
```bash
make build
chmod +x bin/alibabacloud-observability-mcp-server

# Compare tool lists
diff /tmp/tools_before.txt /tmp/tools_after.txt 2>/dev/null || true
```

### Step 2.5 — Report Summary

Present a clear summary:

```
📦 更新完成

版本: <old_version> → <new_version>
依赖: 有变更 / 无变更
配置: 需要更新 .env / 无需更新
工具变更:
  ✅ 新增 N 个工具: tool_a, tool_b, ...
  ❌ 移除 N 个工具: tool_x, ...
服务状态: 需要手动重启 / 已在运行
```

If the server was running, restart with the same transport mode.

---

## Troubleshooting

| 问题 | 解决方案 |
|------|---------|
| `command not found: go` | 安装 Go >= 1.23: https://go.dev/dl/ |
| `credentials not configured` | 编辑 `.env`，填入 AccessKey ID 和 Secret |
| `config file not found` | 确保 `config.yaml` 存在于项目目录 |
| `address already in use` | `lsof -i :8180` 查看占用进程，`kill -9 <PID>` 或改端口 |
| Health check 返回错误 | 检查 stderr 日志，验证 `.env` 中凭证是否正确 |
| `git clone` 超时 | 国内用户尝试镜像: `https://ghproxy.com/https://github.com/...` |
| `go mod tidy` 失败 | 检查网络连接，设置 GOPROXY: `export GOPROXY=https://goproxy.cn,direct` |
| 构建报 `undefined` 错误 | `go mod tidy` 重新下载依赖 |
| `permission denied` | `chmod +x bin/alibabacloud-observability-mcp-server` |
| 付费工具产生意外费用 | 在 `config.yaml` 中用 `enabled_tools` 仅启用免费工具 |
