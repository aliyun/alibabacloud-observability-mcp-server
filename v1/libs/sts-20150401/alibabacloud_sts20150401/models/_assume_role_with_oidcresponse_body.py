# -*- coding: utf-8 -*-
# This file is auto-generated, don't edit it. Thanks.
from __future__ import annotations

from darabonba.model import DaraModel

from alibabacloud_sts20150401 import models as main_models


class AssumeRoleWithOIDCResponseBody(DaraModel):
    def __init__(
        self,
        *,
        assumed_role_user: main_models.AssumeRoleWithOIDCResponseBodyAssumedRoleUser = None,
        credentials: main_models.AssumeRoleWithOIDCResponseBodyCredentials = None,
        oidctoken_info: main_models.AssumeRoleWithOIDCResponseBodyOIDCTokenInfo = None,
        request_id: str = None,
    ):
        # The temporary identity that you use to assume the RAM role.
        self.assumed_role_user = assumed_role_user
        # The access credentials.
        self.credentials = credentials
        # The information about the OIDC token.
        self.oidctoken_info = oidctoken_info
        # The ID of the request.
        self.request_id = request_id

    def validate(self):
        if self.assumed_role_user:
            self.assumed_role_user.validate()
        if self.credentials:
            self.credentials.validate()
        if self.oidctoken_info:
            self.oidctoken_info.validate()

    def to_map(self):
        _map = super().to_map()
        if _map is not None:
            return _map

        result = dict()
        if self.assumed_role_user is not None:
            result["AssumedRoleUser"] = self.assumed_role_user.to_map()

        if self.credentials is not None:
            result["Credentials"] = self.credentials.to_map()

        if self.oidctoken_info is not None:
            result["OIDCTokenInfo"] = self.oidctoken_info.to_map()

        if self.request_id is not None:
            result["RequestId"] = self.request_id
        return result

    def from_map(self, m: dict = None):
        m = m or dict()
        if m.get("AssumedRoleUser") is not None:
            temp_model = main_models.AssumeRoleWithOIDCResponseBodyAssumedRoleUser()
            self.assumed_role_user = temp_model.from_map(m.get("AssumedRoleUser"))

        if m.get("Credentials") is not None:
            temp_model = main_models.AssumeRoleWithOIDCResponseBodyCredentials()
            self.credentials = temp_model.from_map(m.get("Credentials"))

        if m.get("OIDCTokenInfo") is not None:
            temp_model = main_models.AssumeRoleWithOIDCResponseBodyOidctokenInfo()
            self.oidctoken_info = temp_model.from_map(m.get("OIDCTokenInfo"))

        if m.get("RequestId") is not None:
            self.request_id = m.get("RequestId")
        return self
