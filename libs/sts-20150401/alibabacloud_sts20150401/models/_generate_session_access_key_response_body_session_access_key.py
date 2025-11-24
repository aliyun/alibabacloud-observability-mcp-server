# -*- coding: utf-8 -*-
# This file is auto-generated, don't edit it. Thanks.
from __future__ import annotations

from darabonba.model import DaraModel


class GenerateSessionAccessKeyResponseBodySessionAccessKey(DaraModel):
    def __init__(
        self,
        *,
        expiration: str = None,
        session_access_key_id: str = None,
        session_access_key_secret: str = None,
    ):
        self.expiration = expiration
        self.session_access_key_id = session_access_key_id
        self.session_access_key_secret = session_access_key_secret

    def validate(self):
        pass

    def to_map(self):
        _map = super().to_map()
        if _map is not None:
            return _map

        result = dict()
        if self.expiration is not None:
            result["Expiration"] = self.expiration
        if self.session_access_key_id is not None:
            result["SessionAccessKeyId"] = self.session_access_key_id
        if self.session_access_key_secret is not None:
            result["SessionAccessKeySecret"] = self.session_access_key_secret
        return result

    def from_map(self, m: dict = None):
        m = m or dict()
        if m.get("Expiration") is not None:
            self.expiration = m.get("Expiration")
        if m.get("SessionAccessKeyId") is not None:
            self.session_access_key_id = m.get("SessionAccessKeyId")
        if m.get("SessionAccessKeySecret") is not None:
            self.session_access_key_secret = m.get("SessionAccessKeySecret")
        return self
