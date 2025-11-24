# -*- coding: utf-8 -*-
# This file is auto-generated, don't edit it. Thanks.
from __future__ import annotations

from typing import Dict

from alibabacloud_tea_openapi import utils_models as open_api_util_models
from alibabacloud_tea_openapi.client import Client as OpenApiClient
from alibabacloud_tea_openapi.utils import Utils
from darabonba.core import DaraCore as DaraCore
from darabonba.runtime import RuntimeOptions

from alibabacloud_sts20150401 import models as main_models

"""
"""


class Client(OpenApiClient):
    def __init__(
        self,
        config: open_api_util_models.Config,
    ):
        super().__init__(config)
        self._signature_algorithm = "v2"
        self._endpoint_rule = "regional"
        self._endpoint_map = {
            "ap-northeast-2-pop": "sts.aliyuncs.com",
            "cn-beijing-finance-1": "sts.aliyuncs.com",
            "cn-beijing-finance-pop": "sts.aliyuncs.com",
            "cn-beijing-gov-1": "sts.aliyuncs.com",
            "cn-beijing-nu16-b01": "sts.aliyuncs.com",
            "cn-edge-1": "sts.aliyuncs.com",
            "cn-fujian": "sts.aliyuncs.com",
            "cn-haidian-cm12-c01": "sts.aliyuncs.com",
            "cn-hangzhou-bj-b01": "sts.aliyuncs.com",
            "cn-hangzhou-finance": "sts.aliyuncs.com",
            "cn-hangzhou-internal-prod-1": "sts.aliyuncs.com",
            "cn-hangzhou-internal-test-1": "sts.aliyuncs.com",
            "cn-hangzhou-internal-test-2": "sts.aliyuncs.com",
            "cn-hangzhou-internal-test-3": "sts.aliyuncs.com",
            "cn-hangzhou-test-306": "sts.aliyuncs.com",
            "cn-hongkong-finance-pop": "sts.aliyuncs.com",
            "cn-huhehaote-nebula-1": "sts.aliyuncs.com",
            "cn-north-2-gov-1": "sts-vpc.cn-north-2-gov-1.aliyuncs.com",
            "cn-qingdao-nebula": "sts.aliyuncs.com",
            "cn-shanghai-et15-b01": "sts.aliyuncs.com",
            "cn-shanghai-et2-b01": "sts.aliyuncs.com",
            "cn-shanghai-inner": "sts.aliyuncs.com",
            "cn-shanghai-internal-test-1": "sts.aliyuncs.com",
            "cn-shenzhen-finance-1": "sts-vpc.cn-shenzhen-finance-1.aliyuncs.com",
            "cn-shenzhen-inner": "sts.aliyuncs.com",
            "cn-shenzhen-st4-d01": "sts.aliyuncs.com",
            "cn-shenzhen-su18-b01": "sts.aliyuncs.com",
            "cn-wuhan": "sts.aliyuncs.com",
            "cn-yushanfang": "sts.aliyuncs.com",
            "cn-zhangbei": "sts.aliyuncs.com",
            "cn-zhangbei-na61-b01": "sts.aliyuncs.com",
            "cn-zhangjiakou-na62-a01": "sts.aliyuncs.com",
            "cn-zhengzhou-nebula-1": "sts.aliyuncs.com",
            "eu-west-1-oxs": "sts.aliyuncs.com",
            "rus-west-1-pop": "sts.aliyuncs.com",
        }
        self.check_config(config)
        self._endpoint = self.get_endpoint(
            "sts", self._region_id, self._endpoint_rule, self._network, self._suffix, self._endpoint_map, self._endpoint
        )

    def get_endpoint(
        self,
        product_id: str,
        region_id: str,
        endpoint_rule: str,
        network: str,
        suffix: str,
        endpoint_map: Dict[str, str],
        endpoint: str,
    ) -> str:
        if not DaraCore.is_null(endpoint):
            return endpoint
        if not DaraCore.is_null(endpoint_map) and not DaraCore.is_null(endpoint_map.get(region_id)):
            return endpoint_map.get(region_id)
        return Utils.get_endpoint_rules(product_id, region_id, endpoint_rule, network, suffix)

    def assume_role_with_options(
        self,
        request: main_models.AssumeRoleRequest,
        runtime: RuntimeOptions,
    ) -> main_models.AssumeRoleResponse:
        request.validate()
        query = {}
        if not DaraCore.is_null(request.duration_seconds):
            query["DurationSeconds"] = request.duration_seconds
        if not DaraCore.is_null(request.external_id):
            query["ExternalId"] = request.external_id
        if not DaraCore.is_null(request.policy):
            query["Policy"] = request.policy
        if not DaraCore.is_null(request.role_arn):
            query["RoleArn"] = request.role_arn
        if not DaraCore.is_null(request.role_session_name):
            query["RoleSessionName"] = request.role_session_name
        req = open_api_util_models.OpenApiRequest(query=Utils.query(query))
        params = open_api_util_models.Params(
            action="AssumeRole",
            version="2015-04-01",
            protocol="HTTPS",
            pathname="/",
            method="POST",
            auth_type="AK",
            style="RPC",
            req_body_type="formData",
            body_type="json",
        )
        return DaraCore.from_map(main_models.AssumeRoleResponse(), self.call_api(params, req, runtime))

    async def assume_role_with_options_async(
        self,
        request: main_models.AssumeRoleRequest,
        runtime: RuntimeOptions,
    ) -> main_models.AssumeRoleResponse:
        request.validate()
        query = {}
        if not DaraCore.is_null(request.duration_seconds):
            query["DurationSeconds"] = request.duration_seconds
        if not DaraCore.is_null(request.external_id):
            query["ExternalId"] = request.external_id
        if not DaraCore.is_null(request.policy):
            query["Policy"] = request.policy
        if not DaraCore.is_null(request.role_arn):
            query["RoleArn"] = request.role_arn
        if not DaraCore.is_null(request.role_session_name):
            query["RoleSessionName"] = request.role_session_name
        req = open_api_util_models.OpenApiRequest(query=Utils.query(query))
        params = open_api_util_models.Params(
            action="AssumeRole",
            version="2015-04-01",
            protocol="HTTPS",
            pathname="/",
            method="POST",
            auth_type="AK",
            style="RPC",
            req_body_type="formData",
            body_type="json",
        )
        return DaraCore.from_map(main_models.AssumeRoleResponse(), await self.call_api_async(params, req, runtime))

    def assume_role(
        self,
        request: main_models.AssumeRoleRequest,
    ) -> main_models.AssumeRoleResponse:
        runtime = RuntimeOptions()
        return self.assume_role_with_options(request, runtime)

    async def assume_role_async(
        self,
        request: main_models.AssumeRoleRequest,
    ) -> main_models.AssumeRoleResponse:
        runtime = RuntimeOptions()
        return await self.assume_role_with_options_async(request, runtime)

    def assume_role_with_oidcwith_options(
        self,
        request: main_models.AssumeRoleWithOIDCRequest,
        runtime: RuntimeOptions,
    ) -> main_models.AssumeRoleWithOIDCResponse:
        request.validate()
        query = {}
        if not DaraCore.is_null(request.duration_seconds):
            query["DurationSeconds"] = request.duration_seconds
        if not DaraCore.is_null(request.oidcprovider_arn):
            query["OIDCProviderArn"] = request.oidcprovider_arn
        if not DaraCore.is_null(request.oidctoken):
            query["OIDCToken"] = request.oidctoken
        if not DaraCore.is_null(request.policy):
            query["Policy"] = request.policy
        if not DaraCore.is_null(request.role_arn):
            query["RoleArn"] = request.role_arn
        if not DaraCore.is_null(request.role_session_name):
            query["RoleSessionName"] = request.role_session_name
        req = open_api_util_models.OpenApiRequest(query=Utils.query(query))
        params = open_api_util_models.Params(
            action="AssumeRoleWithOIDC",
            version="2015-04-01",
            protocol="HTTPS",
            pathname="/",
            method="POST",
            auth_type="Anonymous",
            style="RPC",
            req_body_type="formData",
            body_type="json",
        )
        return DaraCore.from_map(main_models.AssumeRoleWithOIDCResponse(), self.call_api(params, req, runtime))

    async def assume_role_with_oidcwith_options_async(
        self,
        request: main_models.AssumeRoleWithOIDCRequest,
        runtime: RuntimeOptions,
    ) -> main_models.AssumeRoleWithOIDCResponse:
        request.validate()
        query = {}
        if not DaraCore.is_null(request.duration_seconds):
            query["DurationSeconds"] = request.duration_seconds
        if not DaraCore.is_null(request.oidcprovider_arn):
            query["OIDCProviderArn"] = request.oidcprovider_arn
        if not DaraCore.is_null(request.oidctoken):
            query["OIDCToken"] = request.oidctoken
        if not DaraCore.is_null(request.policy):
            query["Policy"] = request.policy
        if not DaraCore.is_null(request.role_arn):
            query["RoleArn"] = request.role_arn
        if not DaraCore.is_null(request.role_session_name):
            query["RoleSessionName"] = request.role_session_name
        req = open_api_util_models.OpenApiRequest(query=Utils.query(query))
        params = open_api_util_models.Params(
            action="AssumeRoleWithOIDC",
            version="2015-04-01",
            protocol="HTTPS",
            pathname="/",
            method="POST",
            auth_type="Anonymous",
            style="RPC",
            req_body_type="formData",
            body_type="json",
        )
        return DaraCore.from_map(
            main_models.AssumeRoleWithOIDCResponse(), await self.call_api_async(params, req, runtime)
        )

    def assume_role_with_oidc(
        self,
        request: main_models.AssumeRoleWithOIDCRequest,
    ) -> main_models.AssumeRoleWithOIDCResponse:
        runtime = RuntimeOptions()
        return self.assume_role_with_oidcwith_options(request, runtime)

    async def assume_role_with_oidc_async(
        self,
        request: main_models.AssumeRoleWithOIDCRequest,
    ) -> main_models.AssumeRoleWithOIDCResponse:
        runtime = RuntimeOptions()
        return await self.assume_role_with_oidcwith_options_async(request, runtime)

    def assume_role_with_samlwith_options(
        self,
        request: main_models.AssumeRoleWithSAMLRequest,
        runtime: RuntimeOptions,
    ) -> main_models.AssumeRoleWithSAMLResponse:
        request.validate()
        query = {}
        if not DaraCore.is_null(request.duration_seconds):
            query["DurationSeconds"] = request.duration_seconds
        if not DaraCore.is_null(request.policy):
            query["Policy"] = request.policy
        if not DaraCore.is_null(request.role_arn):
            query["RoleArn"] = request.role_arn
        if not DaraCore.is_null(request.samlassertion):
            query["SAMLAssertion"] = request.samlassertion
        if not DaraCore.is_null(request.samlprovider_arn):
            query["SAMLProviderArn"] = request.samlprovider_arn
        req = open_api_util_models.OpenApiRequest(query=Utils.query(query))
        params = open_api_util_models.Params(
            action="AssumeRoleWithSAML",
            version="2015-04-01",
            protocol="HTTP",
            pathname="/",
            method="POST",
            auth_type="Anonymous",
            style="RPC",
            req_body_type="formData",
            body_type="json",
        )
        return DaraCore.from_map(main_models.AssumeRoleWithSAMLResponse(), self.call_api(params, req, runtime))

    async def assume_role_with_samlwith_options_async(
        self,
        request: main_models.AssumeRoleWithSAMLRequest,
        runtime: RuntimeOptions,
    ) -> main_models.AssumeRoleWithSAMLResponse:
        request.validate()
        query = {}
        if not DaraCore.is_null(request.duration_seconds):
            query["DurationSeconds"] = request.duration_seconds
        if not DaraCore.is_null(request.policy):
            query["Policy"] = request.policy
        if not DaraCore.is_null(request.role_arn):
            query["RoleArn"] = request.role_arn
        if not DaraCore.is_null(request.samlassertion):
            query["SAMLAssertion"] = request.samlassertion
        if not DaraCore.is_null(request.samlprovider_arn):
            query["SAMLProviderArn"] = request.samlprovider_arn
        req = open_api_util_models.OpenApiRequest(query=Utils.query(query))
        params = open_api_util_models.Params(
            action="AssumeRoleWithSAML",
            version="2015-04-01",
            protocol="HTTP",
            pathname="/",
            method="POST",
            auth_type="Anonymous",
            style="RPC",
            req_body_type="formData",
            body_type="json",
        )
        return DaraCore.from_map(
            main_models.AssumeRoleWithSAMLResponse(), await self.call_api_async(params, req, runtime)
        )

    def assume_role_with_saml(
        self,
        request: main_models.AssumeRoleWithSAMLRequest,
    ) -> main_models.AssumeRoleWithSAMLResponse:
        runtime = RuntimeOptions()
        return self.assume_role_with_samlwith_options(request, runtime)

    async def assume_role_with_saml_async(
        self,
        request: main_models.AssumeRoleWithSAMLRequest,
    ) -> main_models.AssumeRoleWithSAMLResponse:
        runtime = RuntimeOptions()
        return await self.assume_role_with_samlwith_options_async(request, runtime)

    def assume_role_with_service_identity_with_options(
        self,
        request: main_models.AssumeRoleWithServiceIdentityRequest,
        runtime: RuntimeOptions,
    ) -> main_models.AssumeRoleWithServiceIdentityResponse:
        request.validate()
        query = {}
        if not DaraCore.is_null(request.assume_role_for):
            query["AssumeRoleFor"] = request.assume_role_for
        if not DaraCore.is_null(request.duration_seconds):
            query["DurationSeconds"] = request.duration_seconds
        if not DaraCore.is_null(request.policy):
            query["Policy"] = request.policy
        if not DaraCore.is_null(request.role_arn):
            query["RoleArn"] = request.role_arn
        if not DaraCore.is_null(request.role_session_name):
            query["RoleSessionName"] = request.role_session_name
        req = open_api_util_models.OpenApiRequest(query=Utils.query(query))
        params = open_api_util_models.Params(
            action="AssumeRoleWithServiceIdentity",
            version="2015-04-01",
            protocol="HTTPS",
            pathname="/",
            method="POST",
            auth_type="AK",
            style="RPC",
            req_body_type="formData",
            body_type="json",
        )
        return DaraCore.from_map(
            main_models.AssumeRoleWithServiceIdentityResponse(), self.call_api(params, req, runtime)
        )

    async def assume_role_with_service_identity_with_options_async(
        self,
        request: main_models.AssumeRoleWithServiceIdentityRequest,
        runtime: RuntimeOptions,
    ) -> main_models.AssumeRoleWithServiceIdentityResponse:
        request.validate()
        query = {}
        if not DaraCore.is_null(request.assume_role_for):
            query["AssumeRoleFor"] = request.assume_role_for
        if not DaraCore.is_null(request.duration_seconds):
            query["DurationSeconds"] = request.duration_seconds
        if not DaraCore.is_null(request.policy):
            query["Policy"] = request.policy
        if not DaraCore.is_null(request.role_arn):
            query["RoleArn"] = request.role_arn
        if not DaraCore.is_null(request.role_session_name):
            query["RoleSessionName"] = request.role_session_name
        req = open_api_util_models.OpenApiRequest(query=Utils.query(query))
        params = open_api_util_models.Params(
            action="AssumeRoleWithServiceIdentity",
            version="2015-04-01",
            protocol="HTTPS",
            pathname="/",
            method="POST",
            auth_type="AK",
            style="RPC",
            req_body_type="formData",
            body_type="json",
        )
        return DaraCore.from_map(
            main_models.AssumeRoleWithServiceIdentityResponse(), await self.call_api_async(params, req, runtime)
        )

    def assume_role_with_service_identity(
        self,
        request: main_models.AssumeRoleWithServiceIdentityRequest,
    ) -> main_models.AssumeRoleWithServiceIdentityResponse:
        runtime = RuntimeOptions()
        return self.assume_role_with_service_identity_with_options(request, runtime)

    async def assume_role_with_service_identity_async(
        self,
        request: main_models.AssumeRoleWithServiceIdentityRequest,
    ) -> main_models.AssumeRoleWithServiceIdentityResponse:
        runtime = RuntimeOptions()
        return await self.assume_role_with_service_identity_with_options_async(request, runtime)

    def generate_session_access_key_with_options(
        self,
        request: main_models.GenerateSessionAccessKeyRequest,
        runtime: RuntimeOptions,
    ) -> main_models.GenerateSessionAccessKeyResponse:
        request.validate()
        query = {}
        if not DaraCore.is_null(request.duration_seconds):
            query["DurationSeconds"] = request.duration_seconds
        req = open_api_util_models.OpenApiRequest(query=Utils.query(query))
        params = open_api_util_models.Params(
            action="GenerateSessionAccessKey",
            version="2015-04-01",
            protocol="HTTPS",
            pathname="/",
            method="POST",
            auth_type="AK",
            style="RPC",
            req_body_type="formData",
            body_type="json",
        )
        return DaraCore.from_map(main_models.GenerateSessionAccessKeyResponse(), self.call_api(params, req, runtime))

    async def generate_session_access_key_with_options_async(
        self,
        request: main_models.GenerateSessionAccessKeyRequest,
        runtime: RuntimeOptions,
    ) -> main_models.GenerateSessionAccessKeyResponse:
        request.validate()
        query = {}
        if not DaraCore.is_null(request.duration_seconds):
            query["DurationSeconds"] = request.duration_seconds
        req = open_api_util_models.OpenApiRequest(query=Utils.query(query))
        params = open_api_util_models.Params(
            action="GenerateSessionAccessKey",
            version="2015-04-01",
            protocol="HTTPS",
            pathname="/",
            method="POST",
            auth_type="AK",
            style="RPC",
            req_body_type="formData",
            body_type="json",
        )
        return DaraCore.from_map(
            main_models.GenerateSessionAccessKeyResponse(), await self.call_api_async(params, req, runtime)
        )

    def generate_session_access_key(
        self,
        request: main_models.GenerateSessionAccessKeyRequest,
    ) -> main_models.GenerateSessionAccessKeyResponse:
        runtime = RuntimeOptions()
        return self.generate_session_access_key_with_options(request, runtime)

    async def generate_session_access_key_async(
        self,
        request: main_models.GenerateSessionAccessKeyRequest,
    ) -> main_models.GenerateSessionAccessKeyResponse:
        runtime = RuntimeOptions()
        return await self.generate_session_access_key_with_options_async(request, runtime)

    def generate_token_by_ticket_with_options(
        self,
        request: main_models.GenerateTokenByTicketRequest,
        runtime: RuntimeOptions,
    ) -> main_models.GenerateTokenByTicketResponse:
        request.validate()
        query = {}
        if not DaraCore.is_null(request.ticket):
            query["Ticket"] = request.ticket
        if not DaraCore.is_null(request.ticket_type):
            query["TicketType"] = request.ticket_type
        req = open_api_util_models.OpenApiRequest(query=Utils.query(query))
        params = open_api_util_models.Params(
            action="GenerateTokenByTicket",
            version="2015-04-01",
            protocol="HTTPS",
            pathname="/",
            method="POST",
            auth_type="AK",
            style="RPC",
            req_body_type="formData",
            body_type="json",
        )
        return DaraCore.from_map(main_models.GenerateTokenByTicketResponse(), self.call_api(params, req, runtime))

    async def generate_token_by_ticket_with_options_async(
        self,
        request: main_models.GenerateTokenByTicketRequest,
        runtime: RuntimeOptions,
    ) -> main_models.GenerateTokenByTicketResponse:
        request.validate()
        query = {}
        if not DaraCore.is_null(request.ticket):
            query["Ticket"] = request.ticket
        if not DaraCore.is_null(request.ticket_type):
            query["TicketType"] = request.ticket_type
        req = open_api_util_models.OpenApiRequest(query=Utils.query(query))
        params = open_api_util_models.Params(
            action="GenerateTokenByTicket",
            version="2015-04-01",
            protocol="HTTPS",
            pathname="/",
            method="POST",
            auth_type="AK",
            style="RPC",
            req_body_type="formData",
            body_type="json",
        )
        return DaraCore.from_map(
            main_models.GenerateTokenByTicketResponse(), await self.call_api_async(params, req, runtime)
        )

    def generate_token_by_ticket(
        self,
        request: main_models.GenerateTokenByTicketRequest,
    ) -> main_models.GenerateTokenByTicketResponse:
        runtime = RuntimeOptions()
        return self.generate_token_by_ticket_with_options(request, runtime)

    async def generate_token_by_ticket_async(
        self,
        request: main_models.GenerateTokenByTicketRequest,
    ) -> main_models.GenerateTokenByTicketResponse:
        runtime = RuntimeOptions()
        return await self.generate_token_by_ticket_with_options_async(request, runtime)

    def get_caller_identity_with_options(
        self,
        runtime: RuntimeOptions,
    ) -> main_models.GetCallerIdentityResponse:
        req = open_api_util_models.OpenApiRequest()
        params = open_api_util_models.Params(
            action="GetCallerIdentity",
            version="2015-04-01",
            protocol="HTTPS",
            pathname="/",
            method="POST",
            auth_type="AK",
            style="RPC",
            req_body_type="formData",
            body_type="json",
        )
        return DaraCore.from_map(main_models.GetCallerIdentityResponse(), self.call_api(params, req, runtime))

    async def get_caller_identity_with_options_async(
        self,
        runtime: RuntimeOptions,
    ) -> main_models.GetCallerIdentityResponse:
        req = open_api_util_models.OpenApiRequest()
        params = open_api_util_models.Params(
            action="GetCallerIdentity",
            version="2015-04-01",
            protocol="HTTPS",
            pathname="/",
            method="POST",
            auth_type="AK",
            style="RPC",
            req_body_type="formData",
            body_type="json",
        )
        return DaraCore.from_map(
            main_models.GetCallerIdentityResponse(), await self.call_api_async(params, req, runtime)
        )

    def get_caller_identity(self) -> main_models.GetCallerIdentityResponse:
        runtime = RuntimeOptions()
        return self.get_caller_identity_with_options(runtime)

    async def get_caller_identity_async(self) -> main_models.GetCallerIdentityResponse:
        runtime = RuntimeOptions()
        return await self.get_caller_identity_with_options_async(runtime)

    def get_federation_token_with_options(
        self,
        request: main_models.GetFederationTokenRequest,
        runtime: RuntimeOptions,
    ) -> main_models.GetFederationTokenResponse:
        request.validate()
        query = {}
        if not DaraCore.is_null(request.duration_seconds):
            query["DurationSeconds"] = request.duration_seconds
        if not DaraCore.is_null(request.name):
            query["Name"] = request.name
        if not DaraCore.is_null(request.policy):
            query["Policy"] = request.policy
        req = open_api_util_models.OpenApiRequest(query=Utils.query(query))
        params = open_api_util_models.Params(
            action="GetFederationToken",
            version="2015-04-01",
            protocol="HTTPS",
            pathname="/",
            method="POST",
            auth_type="AK",
            style="RPC",
            req_body_type="formData",
            body_type="json",
        )
        return DaraCore.from_map(main_models.GetFederationTokenResponse(), self.call_api(params, req, runtime))

    async def get_federation_token_with_options_async(
        self,
        request: main_models.GetFederationTokenRequest,
        runtime: RuntimeOptions,
    ) -> main_models.GetFederationTokenResponse:
        request.validate()
        query = {}
        if not DaraCore.is_null(request.duration_seconds):
            query["DurationSeconds"] = request.duration_seconds
        if not DaraCore.is_null(request.name):
            query["Name"] = request.name
        if not DaraCore.is_null(request.policy):
            query["Policy"] = request.policy
        req = open_api_util_models.OpenApiRequest(query=Utils.query(query))
        params = open_api_util_models.Params(
            action="GetFederationToken",
            version="2015-04-01",
            protocol="HTTPS",
            pathname="/",
            method="POST",
            auth_type="AK",
            style="RPC",
            req_body_type="formData",
            body_type="json",
        )
        return DaraCore.from_map(
            main_models.GetFederationTokenResponse(), await self.call_api_async(params, req, runtime)
        )

    def get_federation_token(
        self,
        request: main_models.GetFederationTokenRequest,
    ) -> main_models.GetFederationTokenResponse:
        runtime = RuntimeOptions()
        return self.get_federation_token_with_options(request, runtime)

    async def get_federation_token_async(
        self,
        request: main_models.GetFederationTokenRequest,
    ) -> main_models.GetFederationTokenResponse:
        runtime = RuntimeOptions()
        return await self.get_federation_token_with_options_async(request, runtime)
