# -*- coding: utf-8 -*-
# This file is auto-generated, don't edit it. Thanks.
from __future__ import annotations

from darabonba.model import DaraModel


class GetFederationTokenRequest(DaraModel):
    def __init__(
        self,
        *,
        duration_seconds: int = None,
        name: str = None,
        policy: str = None,
    ):
        # This parameter is required.
        self.duration_seconds = duration_seconds
        # This parameter is required.
        self.name = name
        # This parameter is required.
        self.policy = policy

    def validate(self):
        pass

    def to_map(self):
        _map = super().to_map()
        if _map is not None:
            return _map

        result = dict()
        if self.duration_seconds is not None:
            result["DurationSeconds"] = self.duration_seconds
        if self.name is not None:
            result["Name"] = self.name
        if self.policy is not None:
            result["Policy"] = self.policy
        return result

    def from_map(self, m: dict = None):
        m = m or dict()
        if m.get("DurationSeconds") is not None:
            self.duration_seconds = m.get("DurationSeconds")
        if m.get("Name") is not None:
            self.name = m.get("Name")
        if m.get("Policy") is not None:
            self.policy = m.get("Policy")
        return self
