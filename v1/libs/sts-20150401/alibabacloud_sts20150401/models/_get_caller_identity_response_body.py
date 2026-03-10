# -*- coding: utf-8 -*-
# This file is auto-generated, don't edit it. Thanks.
from __future__ import annotations

from darabonba.model import DaraModel


class GetCallerIdentityResponseBody(DaraModel):
    def __init__(
        self,
        *,
        account_id: str = None,
        arn: str = None,
        identity_type: str = None,
        principal_id: str = None,
        request_id: str = None,
        role_id: str = None,
        user_id: str = None,
    ):
        self.account_id = account_id
        self.arn = arn
        self.identity_type = identity_type
        self.principal_id = principal_id
        self.request_id = request_id
        self.role_id = role_id
        self.user_id = user_id

    def validate(self):
        pass

    def to_map(self):
        _map = super().to_map()
        if _map is not None:
            return _map

        result = dict()
        if self.account_id is not None:
            result["AccountId"] = self.account_id
        if self.arn is not None:
            result["Arn"] = self.arn
        if self.identity_type is not None:
            result["IdentityType"] = self.identity_type
        if self.principal_id is not None:
            result["PrincipalId"] = self.principal_id
        if self.request_id is not None:
            result["RequestId"] = self.request_id
        if self.role_id is not None:
            result["RoleId"] = self.role_id
        if self.user_id is not None:
            result["UserId"] = self.user_id
        return result

    def from_map(self, m: dict = None):
        m = m or dict()
        if m.get("AccountId") is not None:
            self.account_id = m.get("AccountId")
        if m.get("Arn") is not None:
            self.arn = m.get("Arn")
        if m.get("IdentityType") is not None:
            self.identity_type = m.get("IdentityType")
        if m.get("PrincipalId") is not None:
            self.principal_id = m.get("PrincipalId")
        if m.get("RequestId") is not None:
            self.request_id = m.get("RequestId")
        if m.get("RoleId") is not None:
            self.role_id = m.get("RoleId")
        if m.get("UserId") is not None:
            self.user_id = m.get("UserId")
        return self
