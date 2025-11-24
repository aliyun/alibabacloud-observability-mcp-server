import pytest

from mcp_server_aliyun_observability.settings import (
    CMSSettings,
    SLSSettings,
    build_endpoint_mapping,
)


def test_build_endpoint_mapping_precedence_and_normalization():
    combined = (
        "cn-beijing=https://combined.example.com,cn-hangzhou=combined-hz.example.com"
    )
    cli_pairs = [
        "cn-beijing=cli.example.com",
        "cn-shanghai=http://cli-sh.example.com/",
    ]

    mapping = build_endpoint_mapping(cli_pairs, combined)

    assert mapping["cn-beijing"] == "cli.example.com"
    assert mapping["cn-shanghai"] == "cli-sh.example.com"
    assert mapping["cn-hangzhou"] == "combined-hz.example.com"


def test_settings_resolve_fallback_templates():
    sls_settings = SLSSettings(endpoints={"cn-beijing": "custom.example.com"})
    cms_settings = CMSSettings(endpoints={"cn-hangzhou": "cms.hz.example.com"})

    assert sls_settings.resolve("cn-beijing") == "custom.example.com"
    assert sls_settings.resolve("cn-shanghai") == "cn-shanghai.log.aliyuncs.com"

    assert cms_settings.resolve("cn-hangzhou") == "cms.hz.example.com"
    assert cms_settings.resolve("cn-shanghai") == "cms.cn-shanghai.aliyuncs.com"


def test_settings_resolve_requires_region():
    with pytest.raises(ValueError):
        SLSSettings().resolve("")

    with pytest.raises(ValueError):
        CMSSettings().resolve("")
