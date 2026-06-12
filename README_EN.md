# Alibaba Cloud Observability MCP Server (Go)

<p align="center">
  <a href="./README.md"><img alt="中文" src="https://img.shields.io/badge/简体中文-d9d9d9"></a>
  <a href="./README_EN.md"><img alt="English" src="https://img.shields.io/badge/English-d9d9d9"></a>
</p>

---

> **Important**
>
> This project has been **rewritten in Go**. For the original Python version, see the [`v1`](./v1) directory:
> - [v1/README.md](./v1/README.md) - Python version documentation
> - Install via `pip install mcp-server-aliyun-observability`

---

A Go implementation of the Alibaba Cloud Observability MCP Server, providing AI models with structured data access to Alibaba Cloud Log Service (SLS) and CloudMonitor (CMS). Built on the [Model Context Protocol](https://modelcontextprotocol.io/), it integrates seamlessly with AI tools such as Cursor, Kiro, Cline, and Windsurf.

## Features

- Supports stdio, SSE, and streamable-http transport modes
- Modular toolkit architecture: PaaS (CloudMonitor 2.0), IaaS (SLS/CMS direct access), Shared
- Flexible time expression parsing: relative time, absolute timestamps, Grafana-style, preset keywords
- Time-series comparison analysis: statistical calculations, trend analysis, difference scoring
- Structured error handling: English error descriptions and resolution suggestions
- Reliability: retry with exponential backoff, circuit breaker, graceful shutdown
- Structured JSON logging (slog)
- Single binary, zero runtime dependencies

## Quick Start

### Download & Install

Download the binary for your platform from the [Releases](https://github.com/aliyun/alibabacloud-observability-mcp-server/releases) page:

```bash
# Linux amd64
wget https://github.com/aliyun/alibabacloud-observability-mcp-server/releases/latest/download/alibabacloud-observability-mcp-server-linux-amd64.tar.gz
tar -xzf alibabacloud-observability-mcp-server-linux-amd64.tar.gz

# macOS arm64 (M1/M2)
wget https://github.com/aliyun/alibabacloud-observability-mcp-server/releases/latest/download/alibabacloud-observability-mcp-server-darwin-arm64.tar.gz
tar -xzf alibabacloud-observability-mcp-server-darwin-arm64.tar.gz
```

The extracted archive contains:
- `alibabacloud-observability-mcp-server` - executable binary
- `config.yaml` - default configuration file

### Configure Credentials

```bash
# Set Alibaba Cloud AccessKey
export ALIBABA_CLOUD_ACCESS_KEY_ID=<your_access_key_id>
export ALIBABA_CLOUD_ACCESS_KEY_SECRET=<your_access_key_secret>
```

> AccessKey management: [Alibaba Cloud AccessKey Management](https://help.aliyun.com/document_detail/53045.html)

### Start the Server

```bash
# Start in stdio mode (for MCP client direct invocation)
./alibabacloud-observability-mcp-server start --stdio

# Start in network mode (transport configured in config.yaml)
./alibabacloud-observability-mcp-server start --config config.yaml
```

### CLI Commands

```bash
# Show version info
./alibabacloud-observability-mcp-server version

# List all registered tools
./alibabacloud-observability-mcp-server tools
```

## Build from Source

```bash
git clone https://github.com/aliyun/alibabacloud-observability-mcp-server.git
cd alibabacloud-observability-mcp-server
make build
```

## Configuration

Configuration uses a two-layer structure:

1. `config.yaml` - Server configuration (transport mode, logging, network, etc.)
2. `.env` file or environment variables - Credentials and runtime parameters

For detailed configuration options, refer to the comments in `config.yaml`.

## AI Tool Integration

### streamable-http Mode (Recommended)

1. Configure `config.yaml` (set `server.transport: streamable-http`)
2. Start the server: `./bin/alibabacloud-observability-mcp-server start`
3. Configure `mcp.json`:

```json
{
  "mcpServers": {
    "alibaba_cloud_observability": {
      "url": "http://localhost:8080"
    }
  }
}
```

### stdio Mode

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

## Tools

### View Complete Tool List

Run the following command to see all currently registered tools:

```bash
./bin/alibabacloud-observability-mcp-server tools
```

### Paid Features

The following AI-powered tools incur STAROps charges per invocation:

| Tool | Function |
|------|----------|
| `sls_text_to_sql` | Natural language to SQL |
| `sls_text_to_spl` | Natural language to SPL |
| `sls_sop` | SLS intelligent ops assistant |
| `cms_natural_language_query` | Natural language data query |

See [STAROps Pricing](https://www.aliyun.com/price/product#/starops/detail) for details. To avoid charges, enable only free tools via `enabled_tools` in `config.yaml`.

### Permission Requirements

| Service | Permission Documentation | Applicable Tools |
|---------|------------------------|------------------|
| Log Service (SLS) | [SLS Permissions](https://help.aliyun.com/zh/sls/overview-8) | `sls_*` |
| Application Real-Time Monitoring (ARMS) | [ARMS Permissions](https://help.aliyun.com/zh/arms/security-and-compliance/overview-8) | `umodel_*` |
| CloudMonitor (CMS) | [CMS Permissions](https://help.aliyun.com/zh/cms/cloudmonitor-2-0/) | `cms_*` |

**Special Permissions**: AI tools (e.g., `sls_text_to_sql`, `cms_natural_language_query`) require CMS permissions: `cms:CreateChat`, `cms:CreateThread`.

### Time Expressions

All data query tools support flexible time range formats:

| Format | Example |
|--------|---------|
| Relative preset | `last_5m`, `last_1h`, `last_1d` |
| Relative time | `now()-1h`, `now-30m` |
| Grafana style | `now-15m~now-5m`, `now/d`, `now-1d/d` |
| Absolute timestamp | `1718451045` (seconds), `1718451045000` (milliseconds) |
| DateTime string | `2024-01-01 00:00:00`, `2024-01-01T00:00:00Z` |

## Project Structure

```
├── cmd/server/          # CLI entry point
├── pkg/
│   ├── client/          # SLS/CMS client wrappers
│   ├── config/          # Configuration management
│   ├── server/          # MCP Server core
│   └── toolkit/         # Toolkits (PaaS/IaaS/Shared)
└── v1/                  # Python version
```

## Security Recommendations

- The server does not store AccessKeys; they are only used at runtime for API calls
- In SSE/HTTP mode, ensure proper access control for the endpoint
- Deploy within an internal network or VPC to avoid direct public exposure
- Recommended: deploy on Alibaba Cloud Function Compute (FC) with VPC-only access

## Skill-Based Deployment

This project provides an AI Agent Skill for automated deployment, startup, and updates via natural language instructions.

### Install Skill

**Option 1: npx skills add (recommended)**

Universal skill installer maintained by [Vercel Labs](https://github.com/vercel-labs/skills), supports 69+ coding agents:

```bash
# Global install (available in all projects)
npx skills add aliyun/alibabacloud-observability-mcp-server

# Project-level install (current project only)
npx skills add aliyun/alibabacloud-observability-mcp-server --local
```

**Option 2: curl (no Node.js required)**

```bash
# Download and install skill to ~/.claude/skills/
curl -fsSL https://raw.githubusercontent.com/aliyun/alibabacloud-observability-mcp-server/master/skills/deploy-observability/SKILL.md -o ~/.claude/skills/deploy-observability/SKILL.md
```

**Option 3: Copy from cloned repository**

If you've already cloned the project:

```bash
# Global install
cp skills/deploy-observability/SKILL.md ~/.claude/skills/deploy-observability/

# Or project-level install (from project root)
cp skills/deploy-observability/SKILL.md .claude/skills/deploy-observability/
```

### Use the Skill

After installing the skill, copy the following to an AI Agent that supports Skills (e.g., Claude Code):

**First-Time Deployment**:
```
Please use the deploy-observability skill to help me:
1. Clone the project to ~/alibabacloud-observability-mcp-server
2. Download dependencies and build
3. Copy .env.example to .env and prompt me to fill in Alibaba Cloud AccessKey
4. Let me choose the startup mode (stdio / sse / streamable-http)
5. Generate JSON config for me to paste into my AI Agent
```

**Project Updates**:
```
Please use the deploy-observability skill to update the project:
1. Pull the latest code
2. Detect if there are dependency, config, or env variable changes
3. Compare tool list changes and tell me which tools were added/removed
4. Rebuild and restart the server
```

For detailed usage documentation, see [skills/deploy-observability/SKILL.md](skills/deploy-observability/SKILL.md).

## License

This project follows the same license as the original Python version.
