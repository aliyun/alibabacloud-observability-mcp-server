from typing import Any, Dict, Optional
from alibabacloud_cms20240330.client import Client as CmsClient
from alibabacloud_cms20240330.models import (ListWorkspacesRequest,
                                             ListWorkspacesResponse,
                                             ListWorkspacesResponseBody)
from mcp.server.fastmcp import Context, FastMCP
from pydantic import Field
from tenacity import retry, retry_if_exception_type, stop_after_attempt, wait_fixed

from mcp_server_aliyun_observability.utils import handle_tea_exception, execute_cms_query_with_context


class SharedToolkit:
    """Shared Toolkit
    
    Provides common functionality used by both PaaS and DoAI layers, including:
    - Workspace management (list_workspace)
    - Entity discovery (list_domains)
    """

    def __init__(self, server: FastMCP):
        """Initialize the shared toolkit
        
        Args:
            server: FastMCP server instance
        """
        self.server = server
        self.register_tools()

    def register_tools(self):
        """Register all shared tools"""
        self._register_workspace_tools()
        self._register_discovery_tools()
        self._register_info_tools()

    def _register_workspace_tools(self):
        """Register workspace management tools"""

        @self.server.tool()
        @retry(
            wait=wait_fixed(1),
            stop=stop_after_attempt(3),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def list_workspace(
            ctx: Context,
            regionId: str = Field(..., description="阿里云区域ID"),
        ) -> Dict[str, Any]:
            """列出可用的CMS工作空间
            
            ## 功能概述
            获取指定区域内可用的Cloud Monitor Service (CMS)工作空间列表。
            工作空间是CMS中用于组织和管理监控数据的逻辑容器。
            
            ## 参数说明
            - regionId: 阿里云区域标识符，如 "cn-hangzhou", "cn-beijing" 等
            
            ## 返回结果
            返回包含工作空间信息的字典，包括：
            - workspaces: 工作空间列表，每个工作空间包含名称、ID、描述等信息
            - total_count: 工作空间总数
            - region: 查询的区域ID
            
            ## 使用场景
            - 在使用PaaS层API之前，需要先获取可用的工作空间
            - 为DoAI层查询提供工作空间选择
            - 管理和监控多个工作空间的资源使用情况
            
            ## 注意事项
            - 不同区域的工作空间是独立的
            - 工作空间的可见性取决于当前用户的权限
            - 这是一个基础工具，为其他PaaS和DoAI工具提供工作空间选择
            """
            try:
                # 获取CMS客户端
                cms_client: CmsClient = ctx.request_context.lifespan_context.get("cms_client")
                if not cms_client:
                    return {
                        "error": True,
                        "workspaces": [],
                        "total_count": 0,
                        "region": regionId,
                        "message": "CMS客户端未初始化",
                    }
                
                cms_client = cms_client.with_region(regionId)
                
                # 构建请求 - 获取所有工作空间
                request = ListWorkspacesRequest(
                    max_results=100,
                    next_token=None,
                    region=regionId,
                    workspace_name=None  # 获取所有工作空间
                )
                
                # 调用CMS API
                response: ListWorkspacesResponse = cms_client.list_workspaces(request)
                body: ListWorkspacesResponseBody = response.body
                
                # 处理响应
                workspaces = []
                if body.workspaces:
                    workspaces = [w.to_map() for w in body.workspaces]
                
                return {
                    "error": False,
                    "workspaces": workspaces,
                    "total_count": body.total if body.total else len(workspaces),
                    "region": regionId,
                    "message": f"Successfully retrieved {len(workspaces)} workspaces from region {regionId}"
                }
                
            except Exception as e:
                return {
                    "error": True,
                    "workspaces": [],
                    "total_count": 0,
                    "region": regionId,
                    "message": f"Failed to retrieve workspaces: {str(e)}"
                }

    def _register_discovery_tools(self):
        """Register discovery tools"""

        @self.server.tool()
        @retry(
            wait=wait_fixed(1),
            stop=stop_after_attempt(3),
            retry=retry_if_exception_type(Exception),
            reraise=True,
        )
        @handle_tea_exception
        def list_domains(
            ctx: Context,
            workspace: str = Field(..., description="CMS工作空间名称，可通过list_workspace获取"),
            regionId: str = Field(..., description="阿里云区域ID"),
        ) -> Dict[str, Any]:
            """列出所有可用的实体域
            
            ## 功能概述
            获取系统中所有可用的实体域（domain）列表。实体域是实体的最高级分类，
            如 APM、容器、云产品等。这是发现系统支持实体类型的第一步。
            
            ## 使用场景
            - 了解系统支持的所有实体域
            - 为后续查询选择正确的domain参数
            - 构建动态的域选择界面
            
            ## 返回数据
            每个域包含：
            - __domain__: 域名称（如 apm, k8s, cloud）  
            - cnt: 该域下的实体总数量
            
            Args:
                ctx: MCP上下文
                workspace: CMS工作空间名称
                regionId: 阿里云区域ID
                
            Returns:
                包含实体域列表的响应对象
            """
            # 使用.entity查询来获取所有域的统计信息
            query = ".entity with(domain='*', type='*', topk=1000) | stats cnt=count(1) by __domain__ | project __domain__, cnt | sort cnt desc"
            return execute_cms_query_with_context(ctx, query, workspace, regionId, "now-24h", "now", 1000)

    def _register_info_tools(self):
        """Register information and help tools"""

        @self.server.tool()
        def introduction() -> Dict[str, Any]:
            """获取阿里云可观测性MCP Server的介绍和使用说明
            ## 功能概述
            返回阿里云可观测性 MCP Server 的服务概述、核心能力和使用限制说明。
            帮助用户快速了解服务能做什么，以及使用各层工具的前提条件。
            Observable MCP Server 是阿里云可观测官方推出的 MCP (Model Context Protocol) 服务，
            提供统一的 AI 可观测数据访问能力。
            ## 使用场景
            - 首次接入时了解服务能力和限制
            - 了解不同工具层的使用前提

            ## 注意事项
            - 此工具不需要任何参数，可直接调用
            - 返回信息包含各层工具的使用前提条件
            """
            return {
                "name": "Alibaba Cloud Observability MCP Server",
                "version": "1.0.0",
                "description": "阿里云可观测性 MCP 服务 - 提供 AI 驱动的可观测数据访问能力",
                "capabilities": {
                    "data_access": [
                        "查询日志数据（SLS 日志库）",
                        "查询指标数据（时序指标）",
                        "查询链路数据（分布式追踪）",
                        "查询事件数据（异常事件）",
                        "查询实体信息（应用、容器、云产品等）",
                        "性能剖析数据查询"
                    ],
                    "ai_features": [
                        "自然语言转 SQL 查询",
                        "自然语言转 PromQL 查询",
                        "智能实体发现和关系分析"
                    ]
                },
                "tool_layers": {
                    "paas": {
                        "description": "PaaS 层工具集（推荐）- 基于云监控 2.0 的现代化可观测能力",
                        "capabilities": [
                            "实体发现和管理",
                            "指标、日志、事件、链路、性能剖析的统一查询",
                            "数据集和元数据管理"
                        ],
                        "prerequisites": "⚠️ 需要开通阿里云监控 2.0 服务",
                        "note": "适用于需要统一数据模型和实体关系分析的场景"
                    },
                    "iaas": {
                        "description": "IaaS 层工具集 - 直接访问底层存储服务",
                        "capabilities": [
                            "直接查询 SLS 日志库（Log Store）",
                            "直接查询 SLS 指标库（Metric Store）",
                            "执行原生 SQL/PromQL 查询",
                            "日志库和项目管理"
                        ],
                        "prerequisites": "✓ 无需云监控 2.0，仅需 SLS 服务权限",
                        "note": "适用于直接访问 SLS 数据或不依赖云监控 2.0 的场景"
                    },
                    "shared": {
                        "description": "共享工具集 - 基础服务发现和管理",
                        "capabilities": [
                            "工作空间管理",
                            "实体域发现",
                            "服务介绍"
                        ],
                        "prerequisites": "✓ 所有场景可用"
                    }
                },
                "important_notes": [
                    "PaaS 层工具（umodel_* 系列）依赖云监控 2.0，需要先开通服务",
                    "IaaS 层工具（sls_* 系列）直接访问 SLS，无需云监控 2.0",
                    "建议优先使用 PaaS 层工具以获得更好的实体关系和统一数据模型体验",
                    "如果未开通云监控 2.0，可使用 IaaS 层工具直接查询 SLS 数据"
                ],
                "references": {
                    "cloudmonitor_2_0": "https://help.aliyun.com/zh/cms/cloudmonitor-2-0/product-overview/what-is-cloud-monitor-2-0",
                    "sls_overview": "https://help.aliyun.com/zh/sls/?spm=5176.29508878.J_AHgvE-XDhTWrtotIBlDQQ.8.79815c7ffN3uWE",
                    "github": "https://github.com/aliyun/alibabacloud-observability-mcp-server"
                }
            }


def register_shared_tools(server: FastMCP):
    """Register shared toolkit tools with the FastMCP server
    
    Args:
        server: FastMCP server instance
    """
    SharedToolkit(server)