# MCP 贡献指南

## 步骤
1. 从 master 分支创建一个分支
2. 在分支上进行开发测试
3. 测试完毕之后提交PR
4. 合并PR到Release分支
5. 基于 Release 分支发布新版本
6. 更新 master 分支
7. 生成版本 tag

## 项目结构

```
mcp_server_aliyun_observability/
├── src/
│ ├── mcp_server_aliyun_observability/
│ │ ├── server.py                    # MCP 服务端核心
│ │ ├── core/                        # 核心基础设施
│ │ │ ├── models.py                  # 数据模型
│ │ │ ├── decorators.py              # 装饰器
│ │ │ ├── utils.py                   # 工具函数
│ │ │ └── inner/                     # 内部模块
│ │ ├── toolkits/                    # 工具包目录
│ │ │ ├── entities/                  # 实体查询工具包
│ │ │ ├── metrics/                   # 指标查询工具包
│ │ │ ├── traces/                    # 链路查询工具包
│ │ │ ├── events/                    # 事件查询工具包
│ │ │ ├── topologies/                # 拓扑查询工具包
│ │ │ ├── diagnosis/                 # 诊断查询工具包
│ │ │ ├── drilldown/                 # 下钻查询工具包
│ │ │ ├── workspace/                 # 工作空间工具包
│ │ │ └── iaas/                      # V1兼容工具包
│ │ └── utils.py                     # 客户端包装器
│ └── tests/                         # 测试目录
│   ├── mcp_server_aliyun_observability/
│   │ ├── core/                      # 核心模块测试
│   │ └── toolkits/                  # 工具包测试
│   └── conftest.py
```

### 架构说明
1. **server.py**: MCP 服务端代码，负责处理 MCP 请求和动态工具包注册
2. **core/**: 核心基础设施，包含通用模型、装饰器、工具函数和内部模块
3. **toolkits/**: 模块化工具包目录，按功能域组织：
   - **CMS工具集**: entities, metrics, traces, events, topologies, diagnosis, drilldown, workspace (可观测2.0)
   - **IaaS工具集**: iaas/ (V1兼容架构，包含传统SLS、ARMS、CMS工具)
4. **utils.py**: 客户端包装器和通用工具函数
5. **tests/**: 按模块组织的测试用例

## 如何增加一个 MCP 工具

Python 版本要求 >=3.10（MCP SDK 的版本要求）,建议通过venv或者 conda 来创建虚拟环境

## 任务拆解

1. 首先需要明确提供什么样的场景，然后再根据场景拆解需要提供什么功能
2. 对于复杂的场景不建议提供一个工具，而是拆分成多个工具，然后由 LLM 来组合完成任务
   - 好处：提升工具的执行成功率
   - 如果其中一步失败，模型也可以尝试纠正
   - 示例：查询 APM 一个应用的慢调用可拆解为查询应用信息、生成查询慢调用 SQL、执行查询慢调用 SQL 等步骤
3. 尽量复用已有工具，不要新增相同含义的工具

## 工具定义
1. 新增的工具位于 `src/mcp_server_aliyun_observability/toolkit` 目录下，通过增加 `@self.server.tool()` 注解来定义一个工具。
2. 当前可按照产品来组织文件，比如 `src/mcp_server_aliyun_observability/toolkit/sls_toolkit.py` 来定义SLS相关的工具，`src/mcp_server_aliyun_observability/toolkit/arms_toolkit.py` 来定义ARMS相关的工具。
3. 工具上需要增加@tool 注解

### 1. 工具命名

* 格式为 `{product_name}_{function_name}`
* 示例：`sls_describe_logstore`、`arms_search_apps` 等
* 优势：方便模型识别，当用户集成的工具较多时不会造成歧义和冲突

### 2. 参数描述

* 需要尽可能详细，包括输入输出明确定义、示例、使用指导
* 参数使用 pydantic 的模型来定义，示例：

```python
@self.server.tool()
def sls_list_projects(
    ctx: Context,
    project_name_query: str = Field(
        None, description="project name,fuzzy search"
    ),
    limit: int = Field(
        default=10, description="limit,max is 100", ge=1, le=100
    ),
    region_id: str = Field(default=..., description="aliyun region id"),
) -> list[dict[str, Any]]:
```

* 参数注意事项：
  - 参数个数尽量控制在五个以内，超过需考虑拆分工具
  - 相同含义字段定义保持一致（避免一会叫 `project_name`，一会叫 `project`）
  - 参数类型使用基础类型（str, int, list, dict 等），不使用自定义类型
  - 如果参数可选值是固定枚举类，在字段描述中要说明可选择的值，同时在代码方法里面也要增加可选值的校验

### 3. 返回值设计

* 优先使用基础类型，不使用自定义类型
* 控制返回内容长度，特别是数据查询类场景考虑分页返回，防止用户上下文占用过大
* 返回内容字段清晰，数据类最好转换为明确的 key-value 形式
* 针对无返回值的情况，比如数据查询为空，不要直接返回空列表，可以返回文本提示比如 `"没有找到相关数据"`供大模型使用

### 4. 异常处理

* 直接调用 API 且异常信息清晰的情况下可不做处理，直接抛出原始错误日志有助于模型识别
* 如遇 SYSTEM_ERROR 等模糊不清的异常，应处理后返回友好提示
* 做好重试机制，比如网络抖动、服务端限流等，避免模型因此类问题而重复调用

### 5. 工具描述

* 添加工具描述有两种方法：
  - 在 `@self.server.tool()` 中增加 description 参数
  - 使用 Python 的 docstring 描述
* 描述内容应包括：功能概述、使用场景、返回数据结构、查询示例、参数说明等，示例：

```
列出阿里云日志服务中的所有项目。

## 功能概述

该工具可以列出指定区域中的所有SLS项目，支持通过项目名进行模糊搜索。如果不提供项目名称，则返回该区域的所有项目。

## 使用场景

- 当需要查找特定项目是否存在时
- 当需要获取某个区域下所有可用的SLS项目列表时
- 当需要根据项目名称的部分内容查找相关项目时

## 返回数据结构

返回的项目信息包含：
- project_name: 项目名称
- description: 项目描述
- region_id: 项目所在区域

## 查询示例

- "有没有叫 XXX 的 project"
- "列出所有SLS项目"

Args:
    ctx: MCP上下文，用于访问SLS客户端
    project_name_query: 项目名称查询字符串，支持模糊搜索
    limit: 返回结果的最大数量，范围1-100，默认10
    region_id: 阿里云区域ID

Returns:
    包含项目信息的字典列表，每个字典包含project_name、description和region_id
```
* 可以使用 LLM 生成初步描述，然后根据需要进行调整完善

### 如何测试

#### [阶段1] 不基于 LLM，使用测试用例测试

1. 补充下测试用例，在 tests目录下,可有参考 test_sls_toolkit.py 的实现
2. 使用 `pytest` 运行测试用例，保证功能是正确可用

#### [阶段2] 基于 LLM，使用测试用例测试
1. 通过 Cursor,Client 等客户端来测试和大模型集成后的最终效果