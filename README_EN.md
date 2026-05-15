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

> How to obtain an AccessKey: [Alibaba Cloud AccessKey Management](https://help.aliyun.com/document_detail/53045.html)

### Start the Server

```bash
# Start in stdio mode (invoked directly by MCP clients)
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

---

## Building from Source

### Prerequisites

- Go 1.23+

### Build

```bash
# Clone the repository
git clone https://github.com/aliyun/alibabacloud-observability-mcp-server.git
cd alibabacloud-observability-mcp-server

# Build for current platform
make build

# Build for all platforms (linux/darwin/windows x amd64/arm64)
make build-all
```

Built binaries are output to the `bin/` directory.

## Configuration

Configuration uses a two-layer structure:

1. `config.yaml` - server configuration (transport mode, logging, network, etc.)
2. `.env` file or environment variables - credentials and runtime parameters

### Configuration Files

```bash
cp config.yaml config.yaml.bak       # Back up default config (optional)
cp .env.example .env                  # Credentials (AccessKey)
```

`config.yaml` search path: current directory -> `./config/`

The `.env` file is loaded from the current directory, ideal for credentials that should not be committed to version control.

### config.yaml Structure

```yaml
# Server configuration
server:
  transport: streamable-http  # stdio, sse, streamable-http
  host: "0.0.0.0"
  port: 8080

# Logging configuration
logging:
  level: info                 # debug, info, warn, error
  debug_mode: false

# Toolkit configuration
toolkit:
  scope: all                  # all, paas, iaas
  # Fine-grained tool selection (optional; when non-empty, only listed tools are registered)
  # enabled_tools:
  #   - list_workspace
  #   - umodel_get_entities
  #   - sls_execute_sql

# Network configuration
network:
  max_retry: 1
  retry_wait_seconds: 1
  read_timeout_ms: 610000
  connect_timeout_ms: 30000

# Locale configuration
locale:
  timezone: Asia/Shanghai
  language: zh-CN

# Runtime defaults (optional)
# Priority: environment variables > .env file > config.yaml
runtime:
  region: cn-hangzhou
  # workspace: ""

# Endpoint overrides (optional, for internal network access)
# endpoints:
#   sls:
#     cn-hongkong: "cn-hongkong-intranet.log.aliyuncs.com"
#   cms:
#     cn-hongkong: "cms.cn-hongkong.aliyuncs.com"
```

#### Fine-Grained Tool Selection

By default, `toolkit.scope` controls which tool categories are enabled (`all`/`paas`/`iaas`). For more granular control, use `toolkit.enabled_tools` to specify the exact tools to enable:

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

When `enabled_tools` is non-empty, only the listed tools are registered; all others are unavailable. `scope` still determines which toolkit modules are loaded, and `enabled_tools` filters further on top of that.

Refer to the comment template in `config.yaml` for the complete tool list and categories.

### CLI Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `--config` | Specify config file path | Auto-search |
| `--stdio` | Force stdio transport mode | false |

### Environment Variables (Credentials and Runtime Parameters)

| Variable | Description | Required |
|----------|-------------|----------|
| `ALIBABA_CLOUD_ACCESS_KEY_ID` | AccessKey ID | No* |
| `ALIBABA_CLOUD_ACCESS_KEY_SECRET` | AccessKey Secret | No* |
| `ALIBABA_CLOUD_SECURITY_TOKEN` | STS Token (temporary credentials) | No |
| `ALIBABA_CLOUD_REGION` | Default region | No |
| `ALIBABA_CLOUD_WORKSPACE` | Default workspace (required for PaaS tools) | No |

> \* When no AccessKey is configured, the server automatically uses the [default credential chain](https://help.aliyun.com/zh/sdk/developer-reference/v2-manage-go-access-credentials) to obtain credentials (supports ECS RAM Role, OIDC, credential profiles, etc.). No manual AccessKey configuration is needed in cloud environments such as ECS or Function Compute.

Credential resolution priority: CLI parameters / `.env` file > shell environment variables > default credential chain.

> **Default Value Auto-Fill**
>
> When `ALIBABA_CLOUD_REGION` or `ALIBABA_CLOUD_WORKSPACE` is set, if a tool call does not provide the `regionId` or `workspace` parameter, the server automatically uses the value from the environment variable as the default. Explicitly provided values are never overridden.

## AI Tool Integration

### Cursor / Kiro / Cline

**streamable-http mode (recommended):**

1. Configure `config.yaml` (set `server.transport: streamable-http`)
2. Start the server:
```bash
./bin/alibabacloud-observability-mcp-server start
```

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

**stdio mode:**

1. Configure `mcp.json`:
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

Note: In stdio mode, if `config.yaml` does not exist, built-in defaults are used.

## Toolkits

33 tools in total, organized into three tiers.

### PaaS Toolkit (CloudMonitor 2.0, Recommended)

Based on a unified data model, tool names are prefixed with `umodel_` or `cms_`. 16 tools in total.

#### Entity Management Tools

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `umodel_get_entities` | Get entity list | `workspace`, `domain`, `entity_set_name`, `regionId` (required); `entity_filter` (optional) |
| `umodel_get_neighbor_entities` | Get entity neighbor relationships | `workspace`, `src_entity_domain`, `src_name`, `src_entity_ids`, `regionId` (required) |
| `umodel_search_entities` | Search entities | `workspace`, `search_text`, `regionId` (required) |

#### Dataset Management Tools

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `umodel_list_data_set` | List datasets | `workspace`, `domain`, `entity_set_name`, `regionId` (required); `data_set_types` (optional) |
| `umodel_search_entity_set` | Search entity sets | `workspace`, `search_text`, `regionId` (required) |
| `umodel_get_entity_set` | Get entity set schema definition | `domain`, `entity_set_name`, `workspace`, `regionId` (required); `detail` (optional) |
| `umodel_list_related_entity_set` | List related entity sets | `workspace`, `domain`, `entity_set_name`, `regionId` (required) |

#### Data Query Tools

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `umodel_get_metrics` | Query metric data | `workspace`, `domain`, `entity_set_name`, `metric_domain_name`, `metric`, `regionId` (required); `analysis_mode` (basic/cluster/forecast/anomaly_detection), `offset` (time-series comparison), `time_range` (optional) |
| `umodel_get_golden_metrics` | Query golden metrics | `workspace`, `domain`, `entity_set_name`, `regionId` (required); `offset`, `time_range` (optional) |
| `umodel_get_relation_metrics` | Query relation metrics | `workspace`, `src_domain`, `src_entity_set_name`, `relation_type`, `direction` (in/out), `metric`, `metric_set_domain`, `regionId` (required); `dest_entity_set_name` (optional) |
| `umodel_get_logs` | Query log data | `workspace`, `domain`, `entity_set_name`, `log_set_domain`, `log_set_name`, `regionId` (required); `time_range`, `limit` (optional) |
| `umodel_get_events` | Query event data | `workspace`, `domain`, `entity_set_name`, `event_set_domain`, `event_set_name`, `regionId` (required); `time_range`, `limit` (optional) |
| `umodel_get_traces` | Query trace data | `workspace`, `domain`, `entity_set_name`, `trace_set_domain`, `trace_set_name`, `trace_ids`, `regionId` (required); `time_range` (optional) |
| `umodel_search_traces` | Search traces | `workspace`, `domain`, `entity_set_name`, `trace_set_domain`, `trace_set_name`, `regionId` (required); `conditions`, `limit`, `time_range` (optional) |
| `umodel_get_profiles` | Query profiling data | `workspace`, `domain`, `entity_set_name`, `profile_set_domain`, `profile_set_name`, `entity_ids`, `regionId` (required); `time_range`, `limit` (optional) |
| `cms_natural_language_query` | Natural language data query | `query`, `workspace`, `regionId` (required); `time_range` (optional) |

### IaaS Toolkit (SLS/CMS Direct Access)

Direct access to underlying APIs, tool names are prefixed with `sls_` or `cms_`. 14 tools in total.

#### SLS Tools

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `sls_list_projects` | List projects | `regionId` (required); `project` (optional, fuzzy search) |
| `sls_list_logstores` | List logstores | `project`, `regionId` (required) |
| `sls_text_to_sql` | Natural language to SQL | `text`, `project`, `logStore`, `regionId` (required) |
| `sls_text_to_sql_old` | Natural language to SQL (legacy, compatible with Python version) | `text`, `project`, `logStore`, `regionId` (required) |
| `sls_text_to_spl` | Natural language to SPL | `text`, `project`, `logStore`, `data_sample`, `regionId` (required) |
| `sls_execute_sql` | Execute SQL query | `project`, `logStore`, `query`, `regionId` (required); `from_time`, `to_time` (optional) |
| `sls_execute_spl` | Execute native SPL query | `query`, `workspace`, `regionId` (required); `from_time`, `to_time` (optional) |
| `sls_get_context_logs` | Get log context | `project`, `logStore`, `pack_id`, `pack_meta`, `regionId` (required); `back_lines`, `forward_lines` (optional) |
| `sls_log_explore` | Log exploration analysis | `project`, `logStore`, `logField`, `regionId` (required); `from_time`, `to_time`, `filter_query`, `groupField` (optional) |
| `sls_log_compare` | Log comparison analysis | `project`, `logStore`, `logField`, `regionId` (required); `test_from_time`, `test_to_time`, `control_from_time`, `control_to_time`, `filter_query`, `groupField` (optional) |
| `sls_sop` | SLS operations assistant | `text`, `regionId` (required) |

#### CMS Tools

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `cms_execute_promql` | Execute PromQL query | `project`, `metricStore`, `query`, `regionId` (required); `from_time`, `to_time` (optional) |
| `cms_text_to_promql` | Natural language to PromQL | `text`, `project`, `metricStore`, `regionId` (required) |

### Shared Toolkit

3 tools in total.

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `list_workspace` | List workspaces | `regionId` (required) |
| `list_domains` | List entity domains | `workspace`, `regionId` (required) |
| `introduction` | Service introduction | No parameters |

## Time Expressions

All data query tools support flexible time range formats:

| Format | Example |
|--------|---------|
| Relative preset | `last_5m`, `last_1h`, `last_3d`, `last_1w`, `last_1M`, `last_1y` |
| Relative time | `now()-1h`, `now-30m`, `now()-7d` |
| Grafana-style | `now-15m~now-5m`, `now/d`, `now-1d/d` |
| Keywords | `today`, `yesterday` |
| Absolute timestamp | `1718451045` (seconds), `1718451045000` (milliseconds) |
| Datetime string | `2024-01-01 00:00:00`, `2024-01-01T00:00:00Z` |

## Advanced Features

### Time-Series Comparison

`umodel_get_metrics` and `umodel_get_golden_metrics` support time-series comparison via the `offset` parameter:

```
# Compare data from the last 1 hour with 1 day ago
umodel_get_metrics(
    domain="apm", entity_set_name="apm.service",
    metric_domain_name="apm.metric.apm.service", metric="request_count",
    time_range="last_1h", offset="1d"
)
```

The response includes:
- `current`: current period statistics (max, min, avg, count)
- `compare`: comparison period statistics
- `diff`: change analysis (trend, avg_change, avg_change_percent)
- `diff_score`: difference score (0-1, higher means more significant)

### Advanced Analysis Modes

`umodel_get_metrics` supports four analysis modes:

| Mode | Description | Output Fields |
|------|-------------|---------------|
| `basic` | Raw time-series data (default) | `__ts__`, `__value__`, `__labels__` |
| `cluster` | K-Means time-series clustering | `__cluster_index__`, `__entities__`, `__sample_value__` |
| `forecast` | Time-series forecasting (requires 1-5 days of historical data) | `__forecast_ts__`, `__forecast_value__`, `__forecast_lower/upper_value__` |
| `anomaly_detection` | Anomaly detection (requires 1-3 days of data) | `__anomaly_list_`, `__anomaly_msg__`, `__value_min/max/avg__` |

## Project Structure

```
├── cmd/server/          # CLI entry point (cobra)
├── pkg/
│   ├── client/          # SLS/CMS client wrappers
│   ├── config/          # Configuration management (viper + sync.Once)
│   ├── endpoint/        # Endpoint resolution
│   ├── errors/          # Structured errors and error code mapping
│   ├── logger/          # Structured logging (slog)
│   ├── server/          # MCP Server core (transport layer, lifecycle, health checks)
│   ├── stability/       # Retry and circuit breaker
│   ├── timeparse/       # Time expression parsing
│   └── toolkit/         # Toolkit interface and registry
│       ├── paas/        # PaaS toolkit (umodel_*, cms_natural_language_query)
│       ├── iaas/        # IaaS toolkit (sls_*, cms_execute_promql, cms_text_to_promql)
│       └── shared/      # Shared toolkit (list_workspace, list_domains, introduction)
├── v1/                  # Python version (historical reference)
├── Makefile
├── go.mod
└── go.sum
```

## Development

```bash
# Build
make build

# Run tests
make test

# Lint
make lint

# Clean build artifacts
make clean
```

### Testing

The project follows a three-track testing strategy: unit tests + property tests + regression tests:

- Unit tests: table-driven tests covering specific examples and edge cases
- Property tests: using [gopter](https://github.com/leanovate/gopter), verifying general correctness properties across all inputs
- Regression tests: integration tests (`//go:build integration`), comparing parameter consistency with the Python version, requiring real Alibaba Cloud credentials

```bash
# Run all unit tests
go test ./... -v

# Run property tests only
go test ./... -run TestProperty_

# Run regression tests (requires environment variables)
ALIBABA_CLOUD_ACCESS_KEY_ID=xxx \
ALIBABA_CLOUD_ACCESS_KEY_SECRET=xxx \
ALIBABA_CLOUD_REGION=cn-hongkong \
ALIBABA_CLOUD_WORKSPACE=xxx \
go test -tags=integration ./pkg/toolkit/... -v
```

### AI Agent Development Guidelines

See [docs/AGENTS.md](docs/AGENTS.md) for project structure, code style conventions, adding new tools, and testing guidelines.

## Permission Requirements

To ensure the MCP Server can successfully access and operate on your Alibaba Cloud observability resources, you need to configure the following permissions:

### Alibaba Cloud AccessKey

- The server requires valid Alibaba Cloud credentials, supporting the following methods (in priority order):
  1. AccessKey ID + AccessKey Secret (via `.env` file, environment variables, or CLI parameters)
  2. STS temporary credentials (set `ALIBABA_CLOUD_SECURITY_TOKEN` environment variable)
  3. [Default credential chain](https://help.aliyun.com/zh/sdk/developer-reference/v2-manage-go-access-credentials) auto-discovery (ECS RAM Role, OIDC, credential profiles, etc.)
- For obtaining and managing AccessKeys, refer to the [Alibaba Cloud AccessKey Management Documentation](https://help.aliyun.com/document_detail/53045.html)

### RAM Authorization

The RAM user or role associated with the AccessKey **must** be granted the permissions required to access the relevant cloud services.

**It is strongly recommended to follow the "principle of least privilege"**: only grant the minimum set of permissions required to run the MCP tools you plan to use.

Based on the tools you need, refer to the following documentation for permission configuration:

| Service | Permission Documentation | Description |
|---------|------------------------|-------------|
| Log Service (SLS) | [SLS Permissions](https://help.aliyun.com/zh/sls/overview-8) | Required for `sls_*` tools |
| Application Real-Time Monitoring (ARMS) | [ARMS Permissions](https://help.aliyun.com/zh/arms/security-and-compliance/overview-8) | Required for `umodel_*` tools |
| CloudMonitor (CMS) | [CMS Permissions](https://help.aliyun.com/zh/cms/cloudmonitor-2-0/) | Required for `cms_*` tools |

**Special Permission Notes**:
- SQL generation tools (e.g., `sls_text_to_sql`) require the `sls:CallAiTools` permission
- Natural language query (`cms_natural_language_query`) requires: `cms:CreateChat`, `cms:CreateThread`, `cms:GetThread`, `cms:ListThreads`

## Security Recommendations

- The server does not store AccessKeys; they are only used at runtime for API calls
- In SSE/HTTP mode, ensure proper access control for the endpoint
- Deploy within an internal network or VPC to avoid direct public exposure
- Never expose an endpoint configured with an AccessKey to the public internet without authentication
- Recommended: deploy on Alibaba Cloud Function Compute (FC) with VPC-only access

## License

This project follows the same license as the original Python version.
