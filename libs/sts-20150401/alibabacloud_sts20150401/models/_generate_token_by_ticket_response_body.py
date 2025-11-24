# -*- coding: utf-8 -*-
# This file is auto-generated, don't edit it. Thanks.
from __future__ import annotations

from darabonba.model import DaraModel

from alibabacloud_sts20150401 import models as main_models


class GenerateTokenByTicketResponseBody(DaraModel):
    def __init__(
        self,
        *,
        assumed_role_user: main_models.GenerateTokenByTicketResponseBodyAssumedRoleUser = None,
        credentials: main_models.GenerateTokenByTicketResponseBodyCredentials = None,
        request_id: str = None,
    ):
        self.assumed_role_user = assumed_role_user
        self.credentials = credentials
        self.request_id = request_id

    def validate(self):
        if self.assumed_role_user:
            self.assumed_role_user.validate()
        if self.credentials:
            self.credentials.validate()

    def to_map(self):
        _map = super().to_map()
        if _map is not None:
            return _map

        result = dict()
        if self.assumed_role_user is not None:
            result["AssumedRoleUser"] = self.assumed_role_user.to_map()

        if self.credentials is not None:
            result["Credentials"] = self.credentials.to_map()

        if self.request_id is not None:
            result["RequestId"] = self.request_id
        return result

    def from_map(self, m: dict = None):
        m = m or dict()
        if m.get("AssumedRoleUser") is not None:
            temp_model = main_models.GenerateTokenByTicketResponseBodyAssumedRoleUser()
            self.assumed_role_user = temp_model.from_map(m.get("AssumedRoleUser"))

        if m.get("Credentials") is not None:
            temp_model = main_models.GenerateTokenByTicketResponseBodyCredentials()
            self.credentials = temp_model.from_map(m.get("Credentials"))

        if m.get("RequestId") is not None:
            self.request_id = m.get("RequestId")
        return self
