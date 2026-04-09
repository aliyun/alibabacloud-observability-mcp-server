> Branch Update Notice: `master` now uses `1.x.x`; the original `master` full history and code has been migrated to the `0.3.x` branch. If you need to continue using the old version, please iterate on `0.3.x`. Version 1.x has significant feature upgrades based on Observability 2.0, with notable differences in tool design from 0.3.x. See "0.3.x vs 1.x.x Tool Comparison" at the end for details. We will continue to maintain and evolve based on version 1.x.x. Thank you for your understanding and support.

## What is Observable MCP Server

Observable MCP Server is an official MCP service launched by Alibaba Cloud Observability, designed to provide users with a complete set of AI interaction capabilities for observability, supporting natural language access and analysis of multi-modal data. It can seamlessly integrate with Cursor, Cline, Windsurf, and various Agent frameworks, enabling enterprise personnel to use observability data more efficiently and reliably.

## How MCP Works

MCP (Model Context Protocol) establishes a unified context interaction standard between AI models and development environments, allowing models to access real-time domain knowledge in a secure and controlled manner. Observable MCP Server maps natural language requirements to standardized tool calls through this protocol, then transparently schedules underlying log, trace, metric, and other observability product interfaces, allowing agents to obtain structured results without additional adaptation.

Observable MCP Server now supports query and analysis capabilities for multiple products including Log Service SLS, Application Real-Time Monitoring Service ARMS, CloudMonitor, and Prometheus monitoring, and continues to expand more observability services.

## Advantages of Observable MCP Server

1. Multi-source Data Collaboration: One integration enables simultaneous querying of data from multiple observability products including SLS, ARMS, CloudMonitor, and Prometheus, presenting unified log, metric, and trace perspectives.
2. Natural Language Driven: No need to write query statements manually, supports direct retrieval of logs, traces, metrics, and other information through natural language, returning structured answers.
3. Enterprise-grade Security: Based on Alibaba Cloud AccessKey authentication mechanism, the server does not collect additional data, and strictly validates input and output of each tool to ensure data security and control.

## Alibaba Cloud Observability MCP Server
<p align="center">
  <a href="./README.md"><img alt="Chinese README" src="https://img.shields.io/badge/简体中文-d9d9d9"></a>
  <a href="./README_EN.md"><img alt="English README" src="https://img.shields.io/badge/English-d9d9d9"></a>
</p>

### Introduction

Alibaba Cloud Observability MCP Server provides a set of tools for accessing various Alibaba Cloud observability products, covering Alibaba Cloud Log Service SLS, Alibaba Cloud Application Real-Time Monitoring Service ARMS, Alibaba Cloud CloudMonitor, etc. Any intelligent agent that supports the MCP protocol can quickly integrate.

### Tool Architecture
The project adopts a modular architecture with four main toolsets:

- **PaaS Toolkit** (Observability 2.0, Recommended): Contains umodel series tools providing modern observability capabilities with unified data model
  - `entity`: Entity discovery and management (3 tools)
  - `dataset`: Dataset and metadata management (3 tools)
  - `data`: Various data queries supporting metrics, logs, events, traces, profiles (8 tools)
- **IaaS Toolkit** (V1 Compatible): Traditional SLS, CMS native API tools maintaining backward compatibility (11 tools)
- **Shared Toolkit**: Cross-service shared tools like workspace and domain management (3 tools)

### Core Features

#### 🕐 Unified Time Range Expression
All data query tools support flexible time range formats:
- **Relative Presets**: `last_5m`, `last_1h`, `last_3d`, `last_1w`, `last_1M`, `last_1y`
- **Grafana Style**: `now-15m~now-5m`, `now-1h~now`
- **Keywords**: `today`, `yesterday`
- **Absolute Timestamps**: `1706864400~1706868000`
- **Human Readable**: `2024-02-02 10:10:10~2024-02-02 10:20:10`

#### 📊 Time Series Comparison (Metric Compare)
`umodel_get_metrics` and `umodel_get_golden_metrics` support time series comparison via the `offset` parameter:
```python
# Compare current 1 hour with 1 day ago
umodel_get_metrics(
    domain="apm", entity_set_name="apm.service",
    metric_domain_name="apm.metric.apm.service", metric="request_count",
    time_range="last_1h", offset="1d"  # Compare with 1 day ago
)
```
Return results include:
- `current`: Current period statistics (max, min, avg, count)
- `compare`: Comparison period statistics
- `diff`: Change analysis (trend, avg_change, avg_change_percent)
- `diff_score`: Difference score (0-1, higher means more significant difference)

#### 🔬 Advanced Analysis Modes
`umodel_get_metrics` supports four analysis modes:

| Mode | Description | Output Fields |
|------|-------------|---------------|
| `basic` | Raw time series data (default) | `__ts__`, `__value__`, `__labels__` |
| `cluster` | K-Means time series clustering | `__cluster_index__`, `__entities__`, `__sample_value__` |
| `forecast` | Time series prediction (requires 1-5 days of historical data) | `__forecast_ts__`, `__forecast_value__`, `__forecast_lower/upper_value__` |
| `anomaly_detection` | Anomaly detection (requires 1-3 days of data) | `__anomaly_list_`, `__anomaly_msg__`, `__value_min/max/avg__` |

### Version History
Check [CHANGELOG.md](./CHANGELOG.md)

### FAQ
Check [FAQ.md](./FAQ.md)

### Tool List

#### PaaS Toolkit (Observability 2.0)

##### Entity Management Tools (entity)
| Tool Name | Purpose | Key Parameters | Best Practices |
|-----------|---------|----------------|----------------|
| `umodel_get_entities` | Get entity list for specified entity set | `workspace`: Workspace name (required)<br>`domain`: Entity domain (required)<br>`entity_set_name`: Entity type (required)<br>`regionId`: Alibaba Cloud region ID (required) | - Explore available entity resources<br>- Support precise entity queries |
| `umodel_get_neighbor_entities` | Get neighbor nodes of entities | `workspace`: Workspace name (required)<br>`domain`: Entity domain (required)<br>`entity_set_name`: Entity type (required)<br>`entity_ids`: Entity ID list (required)<br>`regionId`: Alibaba Cloud region ID (required) | - Explore service dependencies<br>- Build topology graphs |
| `umodel_search_entities` | Search entities matching conditions | `workspace`: Workspace name (required)<br>`domain`: Entity domain (required)<br>`entity_set_name`: Entity type (required)<br>`search_conditions`: Search conditions<br>`regionId`: Alibaba Cloud region ID (required) | - Support complex query conditions<br>- Flexible entity discovery |

##### Dataset Management Tools (dataset)
| Tool Name | Purpose | Key Parameters | Best Practices |
|-----------|---------|----------------|----------------|
| `umodel_list_data_set` | List datasets of specified type | `workspace`: Workspace name (required)<br>`domain`: Entity domain (required)<br>`entity_set_name`: Entity type (required)<br>`data_set_types`: Dataset type (optional)<br>`regionId`: Alibaba Cloud region ID (required) | - Discover available datasets<br>- Understand data structure and fields |
| `umodel_search_entity_set` | Search entity sets | `workspace`: Workspace name (required)<br>`search_text`: Search keyword (required)<br>`regionId`: Alibaba Cloud region ID (required) | - Discover entity sets by keyword<br>- Support fuzzy search |
| `umodel_list_related_entity_set` | List related entity sets | `workspace`: Workspace name (required)<br>`domain`: Entity domain (required)<br>`entity_set_name`: Entity type (required)<br>`regionId`: Alibaba Cloud region ID (required) | - Understand relationships between entity sets<br>- Explore data dependencies |

##### Data Query Tools (data)
| Tool Name | Purpose | Key Parameters | Best Practices |
|-----------|---------|----------------|----------------|
| `umodel_get_metrics` | Get time series metric data with advanced analysis modes and comparison | `workspace`: Workspace name (required)<br>`domain`: Entity domain (required)<br>`entity_set_name`: Entity type (required)<br>`metric_domain_name`: Metric domain name (required)<br>`metric`: Metric name (required)<br>`analysis_mode`: Analysis mode (optional, default basic)<br>`forecast_duration`: Forecast duration (optional)<br>`offset`: Comparison offset (optional, e.g., 1h/1d/1w)<br>`time_range`: Time range expression (optional)<br>`regionId`: Alibaba Cloud region ID (required) | - Support range/instant queries<br>- **basic**: Raw time series data<br>- **cluster**: K-Means clustering analysis<br>- **forecast**: Time series prediction (requires 1-5 days data)<br>- **anomaly_detection**: Anomaly detection (requires 1-3 days data)<br>- **Time comparison**: Use offset parameter to compare different periods |
| `umodel_get_golden_metrics` | Get golden metrics with time series comparison | `workspace`: Workspace name (required)<br>`domain`: Entity domain (required)<br>`entity_set_name`: Entity type (required)<br>`offset`: Comparison offset (optional, e.g., 1h/1d/1w)<br>`time_range`: Time range expression (optional)<br>`regionId`: Alibaba Cloud region ID (required) | - Quickly get key performance metrics<br>- Includes latency, throughput, error rate, etc.<br>- Support historical period comparison |
| `umodel_get_relation_metrics` | Get relationship-level metrics between entities | `workspace`: Workspace name (required)<br>`src_domain`: Source entity domain (required)<br>`src_entity_set_name`: Source entity type (required)<br>`src_entity_ids`: Source entity ID list (required)<br>`relation_type`: Relation type (required)<br>`direction`: Relation direction (required)<br>`regionId`: Alibaba Cloud region ID (required) | - Analyze microservice call relationships<br>- Support service dependency analysis |
| `umodel_get_logs` | Get entity-related log data | `workspace`: Workspace name (required)<br>`domain`: Entity domain (required)<br>`entity_set_name`: Entity type (required)<br>`log_set_name`: Log set name (required)<br>`log_set_domain`: Log set domain (required)<br>`regionId`: Alibaba Cloud region ID (required) | - For fault diagnosis<br>- Support performance analysis |
| `umodel_get_events` | Get entity event data | `workspace`: Workspace name (required)<br>`domain`: Entity domain (required)<br>`entity_set_name`: Entity type (required)<br>`event_set_domain`: Event set domain (required)<br>`event_set_name`: Event set name (required)<br>`regionId`: Alibaba Cloud region ID (required) | - For anomaly event analysis<br>- Support alert event tracking |
| `umodel_get_traces` | Get detailed trace data with exclusive duration | `workspace`: Workspace name (required)<br>`domain`: Entity domain (required)<br>`entity_set_name`: Entity type (required)<br>`trace_set_domain`: Trace set domain (required)<br>`trace_set_name`: Trace set name (required)<br>`trace_ids`: Trace ID list (required)<br>`regionId`: Alibaba Cloud region ID (required) | - Deep call chain analysis<br>- Includes `exclusive_duration_ms`<br>- Sort by exclusive duration to locate bottlenecks |
| `umodel_search_traces` | Search traces based on conditions | `workspace`: Workspace name (required)<br>`domain`: Entity domain (required)<br>`entity_set_name`: Entity type (required)<br>`trace_set_domain`: Trace set domain (required)<br>`trace_set_name`: Trace set name (required)<br>`regionId`: Alibaba Cloud region ID (required) | - Support filtering by duration, error status<br>- Return trace summary information |
| `umodel_get_profiles` | Get performance profiling data | `workspace`: Workspace name (required)<br>`domain`: Entity domain (required)<br>`entity_set_name`: Entity type (required)<br>`profile_set_domain`: Profile set domain (required)<br>`profile_set_name`: Profile set name (required)<br>`entity_ids`: Entity ID list (required)<br>`regionId`: Alibaba Cloud region ID (required) | - For performance bottleneck analysis<br>- Includes CPU, memory usage |
| `cms_natural_language_query` | Natural language data query | `query`: Natural language query text (required) | - Query observability data using natural language<br>- Support metrics, logs, traces and more<br>- Automatically understand query intent and return results<br>- workspace and regionId from environment variables<br>- Default queries last 15 minutes of data |

#### Shared Toolkit

##### Workspace and Domain Management
| Tool Name | Purpose | Key Parameters | Best Practices |
|-----------|---------|----------------|----------------|
| `introduction` | Get service introduction and usage instructions | No parameters required | - Understand service capabilities on first integration<br>- As LLM Agent self-introduction tool |
| `list_workspace` | Get available workspace list | `regionId`: Alibaba Cloud region ID (required) | - Get workspace before using other tools<br>- Support cross-region workspace queries |
| `list_domains` | Get all entity domains in workspace | `workspace`: Workspace name (required)<br>`regionId`: Alibaba Cloud region ID (required) | - Understand available domains before querying entities<br>- Understand data classification |

#### IaaS Toolkit (V1 Compatible)

##### SLS and CMS Native API Tools
| Tool Name | Purpose | Key Parameters | Best Practices |
|-----------|---------|----------------|----------------|
| `cms_text_to_promql` | Convert natural language to PromQL query | `text`: Natural language question (required)<br>`project`: Project name (required)<br>`metricStore`: Metric store name (required)<br>`regionId`: Alibaba Cloud region ID (required) | - Intelligently generate PromQL statements<br>- Simplify query operations |
| `sls_text_to_sql` | Convert natural language to SQL query | `text`: Natural language question (required)<br>`project`: SLS project name (required)<br>`logStore`: Log store name (required)<br>`regionId`: Alibaba Cloud region ID (required) | - Use CMS Chat API for intelligent SQL generation<br>- Support natural language interaction<br>- Auto-handle unindexed fields |
| `sls_text_to_sql_old` | ⚠️ [Deprecated] Convert natural language to SQL query (Legacy) | `text`: Natural language question (required)<br>`project`: SLS project name (required)<br>`logStore`: Log store name (required)<br>`regionId`: Alibaba Cloud region ID (required) | - Deprecated, use sls_text_to_sql instead<br>- Fallback only |
| `sls_execute_sql` | Execute SLS SQL query | `project`: SLS project name (required)<br>`logStore`: Log store name (required)<br>`query`: SQL query statement (required)<br>`from_time`: Query start time (required)<br>`to_time`: Query end time (required)<br>`limit`: Max log entries, 1-100, default 10 (optional)<br>`offset`: Query start row for pagination, default 0 (optional)<br>`reverse`: Return in descending timestamp order, default False (optional)<br>`regionId`: Alibaba Cloud region ID (required) | - Execute SQL queries directly<br>- Use appropriate time range for performance<br>- Support pagination for more logs |
| `cms_execute_promql` | Execute PromQL query | `project`: Project name (required)<br>`metricStore`: Metric store name (required)<br>`query`: PromQL query statement (required)<br>`from_time`: Query start time (optional, default now-5m)<br>`to_time`: Query end time (optional, default now)<br>`regionId`: Alibaba Cloud region ID (required) | - Query CloudMonitor metric data<br>- Support standard PromQL syntax |
| `sls_list_projects` | List SLS projects | `projectName`: Project name (optional, fuzzy search)<br>`regionId`: Alibaba Cloud region ID (required) | - Discover available SLS projects<br>- Support fuzzy search |
| `sls_execute_spl` | Execute native SPL query | `query`: SPL query statement (required)<br>`workspace`: CMS workspace name (required)<br>`from_time`: Start time (optional)<br>`to_time`: End time (optional)<br>`regionId`: Alibaba Cloud region ID (required) | - Execute complex SLS queries<br>- Support advanced analysis features |
| `sls_list_logstores` | List log stores in specified project | `project`: SLS project name (required)<br>`regionId`: Alibaba Cloud region ID (required) | - Discover log stores in project<br>- Understand data distribution |
| `sls_get_context_logs` | Get log context | `project`: SLS project name (required)<br>`logStore`: Log store name (required)<br>`pack_id`: Log pack ID (required)<br>`pack_meta`: Log pack metadata (required)<br>`regionId`: Alibaba Cloud region ID (required) | - View log context<br>- For troubleshooting |
| `sls_log_explore` | Log exploration analysis | `project`: SLS project name (required)<br>`logStore`: Log store name (required)<br>`from_time`: Start time (required)<br>`to_time`: End time (required)<br>`regionId`: Alibaba Cloud region ID (required) | - Quickly understand log distribution<br>- Discover log patterns |
| `sls_log_compare` | Log comparison analysis | `project`: SLS project name (required)<br>`logStore`: Log store name (required)<br>`test_from_time`: Test group start time (optional)<br>`test_to_time`: Test group end time (optional)<br>`control_from_time`: Control group start time (optional)<br>`control_to_time`: Control group end time (optional)<br>`regionId`: Alibaba Cloud region ID (required) | - Compare logs from different periods<br>- Discover anomalous changes |

### Permission Requirements

To ensure MCP Server can successfully access and operate your Alibaba Cloud observability resources, you need to configure the following permissions:

1.  **Alibaba Cloud Access Key**:
    *   The service requires valid Alibaba Cloud AccessKey ID and AccessKey Secret to run.
    *   For obtaining and managing AccessKey, please refer to [Alibaba Cloud AccessKey Management Documentation](https://help.aliyun.com/document_detail/53045.html).
  
2. When you don't pass AccessKey and AccessKey Secret during initialization, the [default credential chain](https://www.alibabacloud.com/help/zh/sdk/developer-reference/v2-manage-python-access-credentials#62bf90d04dztq) will be used for login:
   1. If ALIBABA_CLOUD_ACCESS_KEY_ID and ALIBABA_CLOUD_ACCESS_KEY_SECRET environment variables exist and are non-empty, they will be used as default credentials.
   2. If ALIBABA_CLOUD_ACCESS_KEY_ID, ALIBABA_CLOUD_ACCESS_KEY_SECRET, and ALIBABA_CLOUD_SECURITY_TOKEN are all set, STS Token will be used as default credentials.
   
3.  **RAM Authorization (Important)**:
    *   The RAM user or role associated with the AccessKey **must** be granted permissions to access the relevant cloud services.
    *   **Strongly recommend following the "Principle of Least Privilege"**: Only grant the minimum set of permissions required to run the MCP tools you plan to use, to reduce security risks.
    *   Based on the tools you need to use, refer to the following documentation for permission configuration:
        *   **Log Service (SLS)**: If you need to use `sls_*` related tools, please refer to [Log Service Permission Documentation](https://help.aliyun.com/zh/sls/overview-8) and grant necessary read, query permissions.
        *   **Application Real-Time Monitoring Service (ARMS)**: If you need to use `arms_*` related tools, please refer to [ARMS Permission Documentation](https://help.aliyun.com/zh/arms/security-and-compliance/overview-8?scm=20140722.H_74783._.OR_help-T_cn~zh-V_1) and grant necessary query permissions.
        *   **Digital Employee (sls_text_to_sql)**: If you need to use `sls_text_to_sql` tool, grant digital employee chat permissions. This tool uses the default digital employee `apsara-ops`. Minimum permissions: `cms:CreateChat`, `cms:CreateThread`, `cms:GetThread`, `cms:ListThreads` on resource `acs:cms:*:*:digitalemployee/*`. See [Digital Employee Permission Configuration](https://help.aliyun.com/zh/cms/cloudmonitor-2-0/digital-employee-permission-configuration).
    * Special permission note: If using SQL generation tools, you need to separately grant `sls:CallAiTools` permission.
    *   Please configure the required permissions based on your actual application scenarios.

### Security and Deployment Recommendations

Please pay attention to the following security matters and deployment best practices:

1.  **Key Security**:
    *   This MCP Server uses the AccessKey you provide to call Alibaba Cloud OpenAPI at runtime, but **will not store your AccessKey in any form**, nor use it for any purposes other than the designed functionality.

2.  **Access Control (Critical)**:
    *   When you choose to access MCP Server via **SSE (Server-Sent Events) protocol**, **you must be responsible for access control and security protection of the service endpoint**.
    *   **Strongly recommend** deploying MCP Server in **internal networks or trusted environments**, such as within your private VPC (Virtual Private Cloud), avoiding direct exposure to the public internet.
    *   The recommended deployment method is using **Alibaba Cloud Function Compute (FC)** and configuring its network settings to **VPC-only access** to achieve network-level isolation and security.
    *   **Note**: **Never** expose the MCP Server SSE endpoint configured with your AccessKey on the public internet without any authentication or access control mechanisms, as this poses extremely high security risks.

### Usage Instructions

Before using MCP Server, you need to obtain Alibaba Cloud AccessKeyId and AccessKeySecret. Please refer to [Alibaba Cloud AccessKey Management](https://help.aliyun.com/document_detail/53045.html)

#### Install via pip
> ⚠️ Requires Python 3.10 or higher.

Install directly using pip:

```bash
# Install (includes all features and dependencies)
pip install mcp-server-aliyun-observability
```

After installation, run directly:

```bash
# Default startup using streamableHttp
python -m mcp_server_aliyun_observability

# Startup with specified access keys
python -m mcp_server_aliyun_observability --access-key-id <your_access_key_id> --access-key-secret <your_access_key_secret>

# Startup using SSE (for remote access)
python -m mcp_server_aliyun_observability --transport sse --transport-port 8000 --host 0.0.0.0
```

Command line parameters:
- `--transport` Specify transport method, options: `stdio`, `sse`, `streamable-http`, default: `streamable-http`
- `--access-key-id` Specify Alibaba Cloud AccessKeyId, uses ALIBABA_CLOUD_ACCESS_KEY_ID environment variable if not specified
- `--access-key-secret` Specify Alibaba Cloud AccessKeySecret, uses ALIBABA_CLOUD_ACCESS_KEY_SECRET environment variable if not specified
- `--sls-endpoints` Override SLS endpoint mapping, format `REGION=HOST`, multiple regions separated by comma/space, e.g., `--sls-endpoints "cn-shanghai=cn-hangzhou.log.aliyuncs.com"`
- `--cms-endpoints` Override CMS endpoint mapping, same format, e.g., `--cms-endpoints "cn-shanghai=cms.internal.aliyuncs.com"`
- `--scope` Specify tool scope, options: `paas`, `iaas`, `all`, default: `all`
- `--log-level` Specify log level, options: `DEBUG`, `INFO`, `WARNING`, `ERROR`, default: `INFO`
- `--transport-port` Specify transport port, default: `8080`, only effective when `--transport` is `sse` or `streamable-http`
- `--host` Specify listen address, default: `127.0.0.1`
- `--knowledge-config` Specify external knowledge base config file path (optional)

**Environment Variables**:
- `ALIBABA_CLOUD_ACCESS_KEY_ID` - Alibaba Cloud AccessKey ID
- `ALIBABA_CLOUD_ACCESS_KEY_SECRET` - Alibaba Cloud AccessKey Secret
- `ALIBABA_CLOUD_SECURITY_TOKEN` - STS Token (optional)
- `LANGUAGE` - Digital employee conversation language (default `zh`)
- `TIMEZONE` - Digital employee conversation timezone (default `Asia/Shanghai`)

### Install and Run via uvx

```bash
# Run latest version directly with uvx
uvx mcp-server-aliyun-observability

# Run specific version
uvx --from 'mcp-server-aliyun-observability==1.0.0' mcp-server-aliyun-observability
```

### Install from Source

```bash
# Clone source code
git clone git@github.com:aliyun/alibabacloud-observability-mcp-server.git
# Enter source directory
cd alibabacloud-observability-mcp-server
# Install
pip install -e .
# Run
python -m mcp_server_aliyun_observability
```

### MCP Test Client

The project provides an MCP protocol-compliant test client for validating server functionality:

```bash
# Enter test client directory
cd mcp_test_client

# Run all tests
python mcp_client.py

# Run specific category tests
python mcp_client.py --category shared  # Shared tools tests
python mcp_client.py --category iaas    # IaaS tools tests
python mcp_client.py --category paas    # PaaS tools tests

# Specify server address
python mcp_client.py --host 127.0.0.1 --port 8080
```

Test client features:
- **MCP Protocol Compatible**: Full implementation of MCP initialization flow (initialize → notifications/initialized)
- **Auto Retry**: Failed tests automatically retry with corrected parameters based on error feedback
- **Smart Truncation**: Output preserves key information, list data shows first complete record
- **Categorized Tests**: Support running tests by shared/iaas/paas categories
- **Resource Discovery Caching**: Automatically caches discovered resources for subsequent tests

### Transport Selection Guide

MCP Server supports three transport protocols. Choose the appropriate protocol based on your use case:

| Transport Type | Use Case | Advantages | Limitations |
|----------------|----------|------------|-------------|
| **streamableHttp** ✅ | Production, Web apps (Recommended) | - Modern HTTP streaming protocol<br>- Excellent performance<br>- Production-grade stability | - Requires network configuration |
| **stdio** | Local development, CLI tools | - Simplest integration<br>- No network configuration required<br>- Direct inter-process communication | - Local use only<br>- No multi-client access |
| **sse** (Server-Sent Events) | Web apps, Remote access | - Supports remote connections<br>- Based on standard HTTP protocol<br>- Supports multiple clients | - Requires maintaining long connections<br>- Slightly lower performance than streamableHttp |

> 💡 **Recommendation**: Use `streamableHttp` (default) for production and web apps, `stdio` for local development, `sse` for special scenarios

### AI Tool Integration

#### Cursor, Cline Integration

**Recommended: Using streamableHttp (default)**
```bash
python -m mcp_server_aliyun_observability
```
```json
{
  "mcpServers": {
    "alibaba_cloud_observability": {
      "url": "http://localhost:8000"
    }
  }
}
```

**Using uvx startup**
```json
{
  "mcpServers": {
    "alibaba_cloud_observability": {
      "command": "uvx",
      "args": [
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

**Using stdio startup (local development)**
```json
{
  "mcpServers": {
    "alibaba_cloud_observability": {
      "command": "uvx",
      "args": [
        "mcp-server-aliyun-observability",
        "--transport",
        "stdio"
      ],
      "env": {
        "ALIBABA_CLOUD_ACCESS_KEY_ID": "<your_access_key_id>",
        "ALIBABA_CLOUD_ACCESS_KEY_SECRET": "<your_access_key_secret>"
      }
    }
  }
}
```

#### Cherry Studio Integration

![image](./images/cherry_studio_inter.png)

![image](./images/cherry_studio_demo.png)


#### Cursor Integration

![image](./images/cursor_inter.png)

![image](./images/cursor_tools.png)

![image](./images/cursor_demo.png)


#### ChatWise Integration

![image](./images/chatwise_inter.png)

![image](./images/chatwise_demo.png)

## 0.3.x vs 1.x.x Tool Comparison

Version 1.x has undergone major upgrades based on Observability 2.0: log/metric/event/trace capabilities are primarily provided through UModel structured interfaces (`umodel_*`), supplemented with a few IaaS direct-connect tools. The table below summarizes added, replaced/renamed, and removed capabilities to facilitate migration from 0.3.x.

### Replaced / Renamed
| 0.3.x Tool | 1.x.x Equivalent | Change Type | Description |
| --- | --- | --- | --- |
| `sls_translate_text_to_sql_query` | `sls_text_to_sql` | Renamed | Still used for generating SLS query statements from natural language. |
| `sls_execute_sql_query` | `sls_execute_sql` | Renamed | Execute SLS queries, parameter names and time parsing adjusted. |
| `cms_translate_text_to_promql` | `cms_text_to_promql` | Renamed | Generate PromQL query text. |
| `cms_execute_promql_query` | `cms_execute_promql` | Renamed | Execute PromQL, underlying implementation changed to SLS metricstore wrapper. |
| `sls_list_projects` | `sls_list_projects` | Retained/Enhanced | Retained with added parameter validation and hints. |
| `sls_list_logstores` | `sls_list_logstores` | Retained/Enhanced | Retained with support for metric store filtering parameters. |

### New (1.x.x Only)
| New Tool | Capability Description |
| --- | --- |
| `sls_execute_spl` | Execute SPL queries directly (advanced usage). |
| `sls_text_to_sql_old` | ⚠️ [Deprecated] Legacy text-to-SQL tool using SLS CallAiTools API. Use sls_text_to_sql instead. |
| `sls_get_context_logs` | Get log context for viewing surrounding logs. |
| `sls_log_explore` | Log exploration analysis for quickly understanding log distribution and patterns. |
| `sls_log_compare` | Log comparison analysis for comparing logs from different periods to discover anomalies. |
| `list_workspace` / `list_domains` / `introduction` | Workspace/domain discovery and service self-description. |
| `umodel_get_entities` / `umodel_get_neighbor_entities` / `umodel_search_entities` | Entity discovery and neighbor queries. |
| `umodel_list_data_set` / `umodel_search_entity_set` / `umodel_list_related_entity_set` | Dataset enumeration, entity set search, and relationship discovery. |
| `umodel_get_metrics` / `umodel_get_golden_metrics` / `umodel_get_relation_metrics` | Metric and relationship-level metric queries. Supports advanced analysis modes: cluster, forecast, anomaly_detection. Supports time series comparison (offset parameter). |
| `umodel_get_logs` / `umodel_get_events` | Log and event queries. |
| `umodel_get_traces` / `umodel_search_traces` | Trace details and search. |
| `umodel_get_profiles` | Performance profiling data queries. |

### Removed / Not Provided
| 0.3.x Tool | Status | Description |
| --- | --- | --- |
| `sls_describe_logstore` | Removed | 1.x focuses on UModel metadata and `umodel_list_data_set`, no longer exposes describe interface. |
| `sls_diagnose_query` | Removed | Not retained in 1.x. |
| `arms_*` series (`arms_search_apps`, `arms_generate_trace_query`, `arms_profile_flame_analysis`, `arms_diff_profile_flame_analysis`, `arms_get_application_info`, `arms_trace_quality_analysis`, `arms_slow_trace_analysis`, `arms_error_trace_analysis`) | Removed | 1.x focuses on Observability 2.0 data model, ARMS-specific tools not provided. |
| `sls_get_regions` / `sls_get_current_time` | Removed | General tools not continued in 1.x. |

Migration recommendation: Prefer using `umodel_*` series for entities, datasets, and metrics/logs/events/traces; only use IaaS tools (`sls_*`, `cms_*`) when direct SLS/CMS operations are needed.
