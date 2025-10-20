import os
import sys

import click
import dotenv

# Avoid importing heavy modules at package import time; import them inside main()
from mcp_server_aliyun_observability.settings import (
    GlobalSettings,
    SLSSettings,
    ArmsSettings,
    configure_settings,
    build_endpoint_mapping,
)

dotenv.load_dotenv()


@click.command()
@click.option(
    "--access-key-id",
    type=str,
    help="aliyun access key id",
    required=False,
)
@click.option(
    "--access-key-secret",
    type=str,
    help="aliyun access key secret",
    required=False,
)
@click.option(
    "--knowledge-config",
    type=str,
    help="knowledge config file path",
    required=False,
)
@click.option("--host", type=str, help="host", default="0.0.0.0")
@click.option(
    "--transport",
    type=click.Choice(["stdio", "sse", "streamable-http"]),
    help="transport type: stdio or sse or streamable-http",
    default="stdio",
)
@click.option("--log-level", type=str, help="log level", default="INFO")
@click.option("--transport-port", type=int, help="transport port", default=8000)
@click.option(
    "--sls-endpoints",
    "sls_endpoints",
    type=str,
    help="REGION=HOST pairs (comma/space separated)",
)
@click.option(
    "--arms-endpoints",
    "arms_endpoints",
    type=str,
    help="REGION=HOST pairs (comma/space separated) for ARMS.",
)
def main(
    access_key_id,
    access_key_secret,
    knowledge_config,
    transport,
    log_level,
    transport_port,
    host,
    sls_endpoints,
    arms_endpoints,
):
    # Import here to avoid side effects for library users / tests importing submodules
    from mcp_server_aliyun_observability.server import server
    from mcp_server_aliyun_observability.utils import CredentialWrapper

    # Configure global settings (process-wide, frozen)
    try:
        sls_mapping = build_endpoint_mapping(cli_pairs=None, combined=sls_endpoints)
        arms_mapping = build_endpoint_mapping(cli_pairs=None, combined=arms_endpoints)
        settings = GlobalSettings(
            sls=SLSSettings(endpoints=sls_mapping),
            arms=ArmsSettings(endpoints=arms_mapping),
        )
        configure_settings(settings)
    except Exception as e:
        # Do not crash on settings issues; log to stderr and continue with defaults
        click.echo(f"[warn] failed to configure SLS endpoints: {e}", err=True)

    if access_key_id and access_key_secret:
        credential = CredentialWrapper(
            access_key_id, access_key_secret, knowledge_config
        )
    else:
        credential = None
    server(credential, transport, log_level, transport_port, host=host)
