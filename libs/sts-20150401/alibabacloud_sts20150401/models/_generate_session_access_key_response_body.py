# -*- coding: utf-8 -*-
# This file is auto-generated, don't edit it. Thanks.
from __future__ import annotations

from darabonba.model import DaraModel

from alibabacloud_sts20150401 import models as main_models


class GenerateSessionAccessKeyResponseBody(DaraModel):
    def __init__(
        self,
        *,
        request_id: str = None,
        session_access_key: main_models.GenerateSessionAccessKeyResponseBodySessionAccessKey = None,
    ):
        self.request_id = request_id
        self.session_access_key = session_access_key

    def validate(self):
        if self.session_access_key:
            self.session_access_key.validate()

    def to_map(self):
        _map = super().to_map()
        if _map is not None:
            return _map

        result = dict()
        if self.request_id is not None:
            result["RequestId"] = self.request_id
        if self.session_access_key is not None:
            result["SessionAccessKey"] = self.session_access_key.to_map()

        return result

    def from_map(self, m: dict = None):
        m = m or dict()
        if m.get("RequestId") is not None:
            self.request_id = m.get("RequestId")
        if m.get("SessionAccessKey") is not None:
            temp_model = main_models.GenerateSessionAccessKeyResponseBodySessionAccessKey()
            self.session_access_key = temp_model.from_map(m.get("SessionAccessKey"))

        return self
