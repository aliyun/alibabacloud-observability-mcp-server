# -*- coding: utf-8 -*-
# This file is auto-generated, don't edit it. Thanks.
from __future__ import annotations

from darabonba.model import DaraModel

from alibabacloud_sts20150401 import models as main_models


class GetFederationTokenResponseBody(DaraModel):
    def __init__(
        self,
        *,
        credentials: main_models.GetFederationTokenResponseBodyCredentials = None,
        federated_user: main_models.GetFederationTokenResponseBodyFederatedUser = None,
        request_id: str = None,
    ):
        self.credentials = credentials
        self.federated_user = federated_user
        self.request_id = request_id

    def validate(self):
        if self.credentials:
            self.credentials.validate()
        if self.federated_user:
            self.federated_user.validate()

    def to_map(self):
        _map = super().to_map()
        if _map is not None:
            return _map

        result = dict()
        if self.credentials is not None:
            result["Credentials"] = self.credentials.to_map()

        if self.federated_user is not None:
            result["FederatedUser"] = self.federated_user.to_map()

        if self.request_id is not None:
            result["RequestId"] = self.request_id
        return result

    def from_map(self, m: dict = None):
        m = m or dict()
        if m.get("Credentials") is not None:
            temp_model = main_models.GetFederationTokenResponseBodyCredentials()
            self.credentials = temp_model.from_map(m.get("Credentials"))

        if m.get("FederatedUser") is not None:
            temp_model = main_models.GetFederationTokenResponseBodyFederatedUser()
            self.federated_user = temp_model.from_map(m.get("FederatedUser"))

        if m.get("RequestId") is not None:
            self.request_id = m.get("RequestId")
        return self
