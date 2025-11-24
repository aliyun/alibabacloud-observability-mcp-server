# -*- coding: utf-8 -*-
# This file is auto-generated, don't edit it. Thanks.
from __future__ import annotations

from darabonba.model import DaraModel


class AssumeRoleWithSAMLResponseBodyAssumedRoleUser(DaraModel):
    def __init__(
        self,
        *,
        arn: str = None,
        assumed_role_id: str = None,
    ):
        # The ARN of the temporary identity that you use to assume the RAM role.
        self.arn = arn
        # The ID of the temporary identity that you use to assume the RAM role.
        self.assumed_role_id = assumed_role_id

    def validate(self):
        pass

    def to_map(self):
        _map = super().to_map()
        if _map is not None:
            return _map

        result = dict()
        if self.arn is not None:
            result["Arn"] = self.arn
        if self.assumed_role_id is not None:
            result["AssumedRoleId"] = self.assumed_role_id
        return result

    def from_map(self, m: dict = None):
        m = m or dict()
        if m.get("Arn") is not None:
            self.arn = m.get("Arn")
        if m.get("AssumedRoleId") is not None:
            self.assumed_role_id = m.get("AssumedRoleId")
        return self
