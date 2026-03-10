# -*- coding: utf-8 -*-
# This file is auto-generated, don't edit it. Thanks.
from __future__ import annotations

from darabonba.model import DaraModel


class AssumeRoleWithServiceIdentityRequest(DaraModel):
    def __init__(
        self,
        *,
        assume_role_for: str = None,
        duration_seconds: int = None,
        policy: str = None,
        role_arn: str = None,
        role_session_name: str = None,
    ):
        # This parameter is required.
        self.assume_role_for = assume_role_for
        self.duration_seconds = duration_seconds
        self.policy = policy
        # This parameter is required.
        self.role_arn = role_arn
        # This parameter is required.
        self.role_session_name = role_session_name

    def validate(self):
        pass

    def to_map(self):
        _map = super().to_map()
        if _map is not None:
            return _map

        result = dict()
        if self.assume_role_for is not None:
            result["AssumeRoleFor"] = self.assume_role_for
        if self.duration_seconds is not None:
            result["DurationSeconds"] = self.duration_seconds
        if self.policy is not None:
            result["Policy"] = self.policy
        if self.role_arn is not None:
            result["RoleArn"] = self.role_arn
        if self.role_session_name is not None:
            result["RoleSessionName"] = self.role_session_name
        return result

    def from_map(self, m: dict = None):
        m = m or dict()
        if m.get("AssumeRoleFor") is not None:
            self.assume_role_for = m.get("AssumeRoleFor")
        if m.get("DurationSeconds") is not None:
            self.duration_seconds = m.get("DurationSeconds")
        if m.get("Policy") is not None:
            self.policy = m.get("Policy")
        if m.get("RoleArn") is not None:
            self.role_arn = m.get("RoleArn")
        if m.get("RoleSessionName") is not None:
            self.role_session_name = m.get("RoleSessionName")
        return self
