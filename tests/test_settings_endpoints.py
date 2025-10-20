import os
import json

import pytest

from mcp_server_aliyun_observability.settings import (
    build_endpoint_mapping,
    SLSSettings,
    ArmsSettings,
    _reset_settings,
    configure_settings,
    GlobalSettings,
)


def test_build_endpoint_mapping_precedence():
    # combined (base)
    combined = "cn-beijing=combined.example.com,cn-hangzhou=combined-hz.example.com"

    # repeated CLI (override)
    cli_pairs = [
        "cn-beijing=cli.example.com",
        "cn-shanghai=cli-sh.example.com",
    ]

    mapping = build_endpoint_mapping(cli_pairs, combined)
    assert mapping["cn-beijing"] == "cli.example.com"
    assert mapping["cn-shanghai"] == "cli-sh.example.com"
    assert mapping["cn-hangzhou"] == "combined-hz.example.com"


def test_sls_settings_resolve_and_normalize():
    s = SLSSettings(endpoints={
        "cn-hangzhou": "https://foo.bar/",
    })
    assert s.resolve("cn-hangzhou") == "foo.bar"
    assert s.resolve("cn-beijing") == "cn-beijing.log.aliyuncs.com"


def test_configure_and_get_settings(monkeypatch):
    _reset_settings()
    gs = GlobalSettings(
        sls=SLSSettings(endpoints={"cn-shanghai": "sls.internal"}),
        arms=ArmsSettings(endpoints={"cn-shanghai": "arms.internal"}),
    )
    configure_settings(gs)
    from mcp_server_aliyun_observability.settings import get_settings

    assert get_settings().sls.resolve("cn-shanghai") == "sls.internal"
    assert get_settings().arms.resolve("cn-shanghai") == "arms.internal"


def test_build_arms_endpoint_mapping():
    mapping = build_endpoint_mapping(None, "cn-hangzhou=arms.hz.example.com")
    assert mapping["cn-hangzhou"] == "arms.hz.example.com"
