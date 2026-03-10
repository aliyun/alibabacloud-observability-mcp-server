# -*- coding: utf-8 -*-
# This file is auto-generated, don't edit it. Thanks.
from __future__ import annotations

from darabonba.model import DaraModel


class AssumeRoleResponseBodyCredentials(DaraModel):
    def __init__(
        self,
        *,
        access_key_id: str = None,
        access_key_secret: str = None,
        expiration: str = None,
        security_token: str = None,
    ):
        # The AccessKey ID.
        self.access_key_id = access_key_id
        # The AccessKey secret.
        self.access_key_secret = access_key_secret
        # The time when the STS token expires. The time is displayed in UTC.
        self.expiration = expiration
        # The STS token.
        #
        # > Alibaba Cloud STS does not impose limits on the length of STS tokens. We strongly recommend that you do not specify a maximum length for STS tokens.
        self.security_token = security_token

    def validate(self):
        pass

    def to_map(self):
        _map = super().to_map()
        if _map is not None:
            return _map

        result = dict()
        if self.access_key_id is not None:
            result["AccessKeyId"] = self.access_key_id
        if self.access_key_secret is not None:
            result["AccessKeySecret"] = self.access_key_secret
        if self.expiration is not None:
            result["Expiration"] = self.expiration
        if self.security_token is not None:
            result["SecurityToken"] = self.security_token
        return result

    def from_map(self, m: dict = None):
        m = m or dict()
        if m.get("AccessKeyId") is not None:
            self.access_key_id = m.get("AccessKeyId")
        if m.get("AccessKeySecret") is not None:
            self.access_key_secret = m.get("AccessKeySecret")
        if m.get("Expiration") is not None:
            self.expiration = m.get("Expiration")
        if m.get("SecurityToken") is not None:
            self.security_token = m.get("SecurityToken")
        return self
