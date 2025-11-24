#!/usr/bin/env python3
"""
Alibaba Cloud Observability MCP Server CLI

A modern CLI tool for managing Alibaba Cloud observability MCP server.
"""

import argparse
import json
import os
import subprocess
import sys
from pathlib import Path
from typing import Dict, List, Optional

import click
from rich.console import Console
from rich.table import Table
from rich.panel import Panel
from rich.text import Text
from rich import box

console = Console()

# Version info
__version__ = "1.0.0"

# Tool categories and their descriptions
TOOLKIT_CATEGORIES = {
    "iaas": {
        "name": "IaaS Layer",
        "description": "Infrastructure SQL/PromQL tools for expert users",
        "tools": 6,
        "prefix": "sls_",
        "color": "blue"
    },
    "paas": {
        "name": "PaaS Layer", 
        "description": "Platform API tools for developers",
        "tools": 15,
        "prefix": "umodel_",
        "color": "green"
    },
    "agent": {
        "name": "Agent Layer",
        "description": "AI-powered insight tools for business users", 
        "tools": 1,
        "prefix": "agent_",
        "color": "magenta"
    },
    "all": {
        "name": "All Layers",
        "description": "Complete three-layer architecture",
        "tools": 22,
        "prefix": "mixed",
        "color": "yellow"
    }
}

DEFAULT_PORT = 8080
DEFAULT_TRANSPORT = "streamable-http"


@click.group(invoke_without_command=True)
@click.option('--version', is_flag=True, help='Show version information')
@click.pass_context
def cli(ctx, version):
    """
    🔍 Alibaba Cloud Observability MCP Server
    
    A powerful MCP server providing AI-driven observability insights
    through three-layer architecture: IaaS, PaaS, and Agent.
    """
    if ctx.invoked_subcommand is None:
        if version:
            show_version()
        else:
            show_welcome()


def show_welcome():
    """Display welcome screen with usage examples"""
    welcome_panel = Panel(
        "[bold blue]🔍 Alibaba Cloud Observability MCP Server[/bold blue]\n\n"
        "[dim]A powerful MCP server providing AI-driven observability insights[/dim]\n\n"
        "[bold]Quick Start:[/bold]\n"
        "  [cyan]aom start[/cyan]                    # Start with all tools\n"
        "  [cyan]aom start --toolkits iaas[/cyan]   # Start with IaaS tools only\n"
        "  [cyan]aom list tools[/cyan]              # List available tools\n"
        "  [cyan]aom config show[/cyan]             # Show current configuration\n\n"
        "[dim]Use 'aom --help' for more information.[/dim]",
        title="Welcome",
        border_style="blue",
        padding=(1, 2)
    )
    console.print(welcome_panel)


def show_version():
    """Display version information"""
    console.print(f"[bold blue]aom[/bold blue] version [green]{__version__}[/green]")


@cli.command()
@click.option('--transport', '-t', 
              type=click.Choice(['streamable-http', 'sse', 'stdio']),
              default=DEFAULT_TRANSPORT,
              help='Transport protocol')
@click.option('--port', '-p',
              type=int, 
              default=DEFAULT_PORT,
              help=f'Server port (default: {DEFAULT_PORT})')
@click.option('--toolkits', '-k',
              type=click.Choice(['iaas', 'paas', 'agent', 'all']),
              default='all',
              help='Toolkit categories to load')
@click.option('--access-key-id', 
              help='Alibaba Cloud Access Key ID')
@click.option('--access-key-secret',
              help='Alibaba Cloud Access Key Secret')
@click.option('--region', '-r',
              default='cn-hangzhou',
              help='Default Alibaba Cloud region')
@click.option('--config-file', '-c',
              type=click.Path(exists=True),
              help='Load configuration from file')
@click.option('--verbose', '-v', 
              is_flag=True,
              help='Enable verbose logging')
@click.option('--daemon', '-d',
              is_flag=True, 
              help='Run as daemon process')
def start(transport, port, toolkits, access_key_id, access_key_secret, 
          region, config_file, verbose, daemon):
    """
    🚀 Start the MCP server
    
    Examples:
      aom start                                    # Start with all tools
      aom start --toolkits iaas --port 8080       # Start IaaS tools only
      aom start --transport sse --daemon          # Start as SSE daemon
    """
    
    # Show startup banner
    show_startup_banner(transport, port, toolkits)
    
    # Build command
    cmd = build_start_command(
        transport, port, toolkits, access_key_id, 
        access_key_secret, region, config_file, verbose
    )
    
    if daemon:
        start_daemon(cmd)
    else:
        start_interactive(cmd)


def show_startup_banner(transport, port, toolkits):
    """Display startup configuration"""
    toolkit_info = TOOLKIT_CATEGORIES[toolkits]
    
    startup_text = f"""[bold]Starting MCP Server...[/bold]
    
🌍 Transport: [cyan]{transport}[/cyan]
🔌 Port: [cyan]{port}[/cyan]
🔧 Toolkits: [{toolkit_info['color']}]{toolkit_info['name']}[/{toolkit_info['color']}] ([dim]{toolkit_info['tools']} tools[/dim])
📝 Description: [dim]{toolkit_info['description']}[/dim]"""

    panel = Panel(
        startup_text,
        title="🚀 Server Configuration",
        border_style="green",
        padding=(1, 2)
    )
    console.print(panel)


def build_start_command(transport, port, toolkits, access_key_id, 
                       access_key_secret, region, config_file, verbose):
    """Build the python command to start server"""
    cmd = [
        sys.executable, '-m', 'mcp_server_aliyun_observability',
        '--transport', transport,
        '--transport-port', str(port),
        '--toolkits', toolkits
    ]
    
    if access_key_id:
        cmd.extend(['--access-key-id', access_key_id])
    if access_key_secret:
        cmd.extend(['--access-key-secret', access_key_secret])
    if verbose:
        cmd.append('--verbose')
        
    return cmd


def start_interactive(cmd):
    """Start server in interactive mode"""
    console.print("\n[green]✓[/green] Server starting...")
    console.print("[dim]Press Ctrl+C to stop[/dim]\n")
    
    try:
        subprocess.run(cmd)
    except KeyboardInterrupt:
        console.print("\n[yellow]⚠[/yellow] Server stopped by user")
    except Exception as e:
        console.print(f"\n[red]✗[/red] Server failed to start: {e}")


def start_daemon(cmd):
    """Start server as daemon process"""
    console.print("[green]✓[/green] Starting server as daemon...")
    # TODO: Implement daemon mode
    console.print("[yellow]⚠[/yellow] Daemon mode not yet implemented")


@cli.group()
def list():
    """📋 List tools and configuration information"""
    pass


@list.command(name='tools')
@click.option('--scope', '-s',
              type=click.Choice(['iaas', 'paas', 'all']),
              default='all',
              help='Scope of tools to list')
@click.option('--format', '-f',
              type=click.Choice(['table', 'json', 'simple']),
              default='table',
              help='Output format')
def list_tools(scope, format):
    """
    List available MCP tools
    
    Examples:
      aom list tools                    # List all tools  
      aom list tools --scope iaas      # List IaaS tools only
      aom list tools --format json     # Output as JSON
    """
    
    if format == 'table':
        show_tools_table(scope)
    elif format == 'json':
        show_tools_json(scope)
    else:
        show_tools_simple(scope)


def show_tools_table(scope):
    """Display tools in table format"""
    
    if scope == 'all':
        # Show summary table for all categories
        table = Table(title="🔧 MCP Tools Overview", box=box.ROUNDED)
        table.add_column("Layer", style="bold")
        table.add_column("Description", style="dim")
        table.add_column("Tools", justify="center")
        table.add_column("Prefix", style="cyan")
        
        for key, info in TOOLKIT_CATEGORIES.items():
            if key != 'all':
                table.add_row(
                    f"[{info['color']}]{info['name']}[/{info['color']}]",
                    info['description'],
                    str(info['tools']),
                    info['prefix']
                )
        
        console.print(table)
        console.print(f"\n[bold]Total: {TOOLKIT_CATEGORIES['all']['tools']} tools across 3 layers[/bold]")
    else:
        # Show detailed tools for specific scope
        show_detailed_tools(scope)


def show_detailed_tools(scope):
    """Show detailed tool list for specific scope"""
    # This would need to import and inspect the actual toolkits
    # For now, show a placeholder
    info = TOOLKIT_CATEGORIES[scope]
    
    console.print(f"\n[bold {info['color']}]{info['name']} Tools[/bold {info['color']}]")
    console.print(f"[dim]{info['description']}[/dim]\n")
    
    # Placeholder - in real implementation, would dynamically load tools
    console.print(f"[yellow]ℹ[/yellow] {info['tools']} tools available with prefix '{info['prefix']}'")
    console.print("[dim]Use --format json for detailed tool information[/dim]")


def show_tools_json(scope):
    """Display tools in JSON format"""
    if scope == 'all':
        data = TOOLKIT_CATEGORIES
    else:
        data = {scope: TOOLKIT_CATEGORIES[scope]}
    
    console.print(json.dumps(data, indent=2))


def show_tools_simple(scope):
    """Display tools in simple list format"""
    if scope == 'all':
        for key, info in TOOLKIT_CATEGORIES.items():
            if key != 'all':
                console.print(f"{info['name']}: {info['tools']} tools ({info['prefix']})")
    else:
        info = TOOLKIT_CATEGORIES[scope]
        console.print(f"{info['name']}: {info['tools']} tools ({info['prefix']})")


@list.command(name='config')
def list_config():
    """Show current configuration"""
    config_panel = Panel(
        "[bold]Current Configuration[/bold]\n\n"
        f"[dim]Version:[/dim] {__version__}\n"
        f"[dim]Default Transport:[/dim] {DEFAULT_TRANSPORT}\n" 
        f"[dim]Default Port:[/dim] {DEFAULT_PORT}\n"
        f"[dim]Config File:[/dim] ~/.aom/config.json\n"
        f"[dim]Python Executable:[/dim] {sys.executable}",
        title="⚙️ Configuration",
        border_style="cyan"
    )
    console.print(config_panel)


@cli.group()
def config():
    """⚙️ Configuration management"""
    pass


@config.command(name='show')
def config_show():
    """Show current configuration"""
    list_config()


@config.command(name='init')
@click.option('--force', is_flag=True, help='Overwrite existing configuration')
def config_init(force):
    """Initialize configuration file"""
    config_dir = Path.home() / '.aom'
    config_file = config_dir / 'config.json'
    
    if config_file.exists() and not force:
        console.print(f"[yellow]⚠[/yellow] Configuration already exists at {config_file}")
        console.print("Use --force to overwrite")
        return
    
    # Create config directory
    config_dir.mkdir(exist_ok=True)
    
    # Default configuration
    default_config = {
        "default_transport": DEFAULT_TRANSPORT,
        "default_port": DEFAULT_PORT,
        "default_toolkits": "all",
        "default_region": "cn-hangzhou"
    }
    
    with open(config_file, 'w') as f:
        json.dump(default_config, f, indent=2)
    
    console.print(f"[green]✓[/green] Configuration initialized at {config_file}")


@cli.command()
@click.argument('topic', required=False)
def help(topic):
    """
    📚 Show help information
    
    Examples:
      aom help           # General help
      aom help start     # Help for start command
    """
    if not topic:
        ctx = click.get_current_context()
        click.echo(ctx.get_help())
    else:
        # Show help for specific command
        console.print(f"[dim]Help for topic: {topic}[/dim]")


if __name__ == '__main__':
    cli()