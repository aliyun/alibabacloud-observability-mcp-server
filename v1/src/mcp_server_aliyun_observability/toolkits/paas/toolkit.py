from mcp.server.fastmcp import FastMCP

from mcp_server_aliyun_observability.toolkits.paas.data_agent_toolkit import (
    DataAgentToolkit,
)
from mcp_server_aliyun_observability.toolkits.paas.data_toolkit import PaasDataToolkit
from mcp_server_aliyun_observability.toolkits.paas.dataset_toolkit import (
    PaaSDatasetToolkit,
)
from mcp_server_aliyun_observability.toolkits.paas.entity_toolkit import (
    PaaSEntityToolkit,
)


class PaaSToolkit:
    """Platform as a Service Layer Toolkit

    Provides structured query tools ported from umodel-mcp handlers.
    All tools use umodel_ prefix and execute SPL queries with precise parameter control.
    No natural language parameters - only structured data.

    Also provides natural language data query capability through data-agent-pro skill.
    """

    def __init__(self, server: FastMCP):
        """Initialize the PaaS toolkit

        Args:
            server: FastMCP server instance
        """
        self.server = server
        self._register_toolkits()

    def _register_toolkits(self):
        """Register all PaaS sub-toolkits"""

        # Initialize sub-toolkits
        PaaSEntityToolkit(self.server)
        PaaSDatasetToolkit(self.server)
        PaasDataToolkit(self.server)
        DataAgentToolkit(self.server)


def register_paas_tools(server: FastMCP):
    """Register PaaS toolkit tools with the FastMCP server

    Args:
        server: FastMCP server instance
    """
    PaaSToolkit(server)
