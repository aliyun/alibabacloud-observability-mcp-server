# -*- coding: utf-8 -*-
# This file is auto-generated, don't edit it. Thanks.
from __future__ import annotations

from darabonba.model import DaraModel


class AssumeRoleWithOIDCResponseBodyOIDCTokenInfo(DaraModel):
    def __init__(
        self,
        *,
        client_ids: str = None,
        expiration_time: str = None,
        issuance_time: str = None,
        issuer: str = None,
        subject: str = None,
        verification_info: str = None,
    ):
        # The audience. If multiple audiences are returned, the audiences are separated by commas (,).
        #
        # The audience is represented by the `aud` field in the OIDC Token.
        self.client_ids = client_ids
        # The time when the OIDC token expires.
        self.expiration_time = expiration_time
        # The time when the OIDC token was issued.
        self.issuance_time = issuance_time
        # The URL of the issuer,
        #
        # which is represented by the `iss` field in the OIDC Token.
        self.issuer = issuer
        # The subject,
        #
        # which is represented by the `sub` field in the OIDC Token.
        self.subject = subject
        # The verification information about the OIDC token. For more information, see [Manage an OIDC IdP](https://help.aliyun.com/document_detail/327123.html).
        self.verification_info = verification_info

    def validate(self):
        pass

    def to_map(self):
        _map = super().to_map()
        if _map is not None:
            return _map

        result = dict()
        if self.client_ids is not None:
            result["ClientIds"] = self.client_ids
        if self.expiration_time is not None:
            result["ExpirationTime"] = self.expiration_time
        if self.issuance_time is not None:
            result["IssuanceTime"] = self.issuance_time
        if self.issuer is not None:
            result["Issuer"] = self.issuer
        if self.subject is not None:
            result["Subject"] = self.subject
        if self.verification_info is not None:
            result["VerificationInfo"] = self.verification_info
        return result

    def from_map(self, m: dict = None):
        m = m or dict()
        if m.get("ClientIds") is not None:
            self.client_ids = m.get("ClientIds")
        if m.get("ExpirationTime") is not None:
            self.expiration_time = m.get("ExpirationTime")
        if m.get("IssuanceTime") is not None:
            self.issuance_time = m.get("IssuanceTime")
        if m.get("Issuer") is not None:
            self.issuer = m.get("Issuer")
        if m.get("Subject") is not None:
            self.subject = m.get("Subject")
        if m.get("VerificationInfo") is not None:
            self.verification_info = m.get("VerificationInfo")
        return self
