import os

import click
import dotenv

from mcp_server_aliyun_observability.settings import (
    CMSSettings,
    GlobalSettings,
    SLSSettings,
    build_endpoint_mapping,
    configure_settings,
)

dotenv.load_dotenv()


@click.command()
@click.option(
    "--access-key-id",
    type=str,
    default=lambda: os.environ.get("ALIBABA_CLOUD_ACCESS_KEY_ID"),
    help="aliyun access key id (default: $ALIBABA_CLOUD_ACCESS_KEY_ID)",
    required=False,
)
@click.option(
    "--access-key-secret",
    type=str,
    default=lambda: os.environ.get("ALIBABA_CLOUD_ACCESS_KEY_SECRET"),
    help="aliyun access key secret (default: $ALIBABA_CLOUD_ACCESS_KEY_SECRET)",
    required=False,
)
@click.option(
    "--knowledge-config",
    type=str,
    help="knowledge config file path",
    required=False,
)
@click.option(
    "--transport",
    type=click.Choice(["stdio", "sse", "streamable-http"]),
    help="transport type: stdio or sse (streamableHttp coming soon)",
    default="streamable-http",
)
@click.option("--host", type=str, help="host", default="127.0.0.1")
@click.option("--log-level", type=str, help="log level", default="INFO")
@click.option("--transport-port", type=int, help="transport port", default=8080)
@click.option(
    "--sls-endpoints",
    "sls_endpoints",
    type=str,
    help="REGION=HOST pairs (comma/space separated) for SLS",
)
@click.option(
    "--cms-endpoints",
    "cms_endpoints",
    type=str,
    help="REGION=HOST pairs (comma/space separated) for CMS",
)
@click.option(
    "--scope",
    type=click.Choice(["paas", "iaas", "all"]),
    help="工具范围: paas(平台API), iaas(基础设施), all(全部)",
    default="all",
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
    cms_endpoints,
    scope,
):
    # 打印凭据信息（脱敏显示）
    has_ak = access_key_id is not None
    has_sk = access_key_secret is not None
    print(f"[Startup] has_access_key_id: {has_ak}, has_access_key_secret: {has_sk}")

    # Lazy import heavy modules to keep package import light for library/test usage
    from mcp_server_aliyun_observability.server import server
    from mcp_server_aliyun_observability.utils import CredentialWrapper

    # Configure global endpoint settings (process-wide, frozen)
    try:
        sls_mapping = build_endpoint_mapping(cli_pairs=None, combined=sls_endpoints)
        cms_mapping = build_endpoint_mapping(cli_pairs=None, combined=cms_endpoints)
        settings = GlobalSettings(
            sls=SLSSettings(endpoints=sls_mapping),
            cms=CMSSettings(endpoints=cms_mapping),
        )
        configure_settings(settings)
    except Exception as e:
        click.echo(f"[warn] failed to configure endpoints: {e}", err=True)

    if access_key_id and access_key_secret:
        credential = CredentialWrapper(
            access_key_id, access_key_secret, knowledge_config
        )
    else:
        credential = None

    # 设置环境变量，传递给服务器
    if scope and scope != "all":
        os.environ["MCP_TOOLKIT_SCOPE"] = scope

    server(credential, transport, log_level, transport_port, host)
