# -*- coding: utf-8 -*-
# This file is auto-generated, don't edit it. Thanks.
from __future__ import annotations

from darabonba.model import DaraModel


class GetFederationTokenResponseBodyFederatedUser(DaraModel):
    def __init__(
        self,
        *,
        arn: str = None,
        federated_user_id: str = None,
    ):
        self.arn = arn
        self.federated_user_id = federated_user_id

    def validate(self):
        pass

    def to_map(self):
        _map = super().to_map()
        if _map is not None:
            return _map

        result = dict()
        if self.arn is not None:
            result["Arn"] = self.arn
        if self.federated_user_id is not None:
            result["FederatedUserId"] = self.federated_user_id
        return result

    def from_map(self, m: dict = None):
        m = m or dict()
        if m.get("Arn") is not None:
            self.arn = m.get("Arn")
        if m.get("FederatedUserId") is not None:
            self.federated_user_id = m.get("FederatedUserId")
        return self
