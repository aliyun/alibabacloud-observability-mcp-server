# -*- coding: utf-8 -*-
# This file is auto-generated, don't edit it. Thanks.
from __future__ import annotations

from darabonba.model import DaraModel


class GenerateSessionAccessKeyRequest(DaraModel):
    def __init__(
        self,
        *,
        duration_seconds: int = None,
    ):
        self.duration_seconds = duration_seconds

    def validate(self):
        pass

    def to_map(self):
        _map = super().to_map()
        if _map is not None:
            return _map

        result = dict()
        if self.duration_seconds is not None:
            result["DurationSeconds"] = self.duration_seconds
        return result

    def from_map(self, m: dict = None):
        m = m or dict()
        if m.get("DurationSeconds") is not None:
            self.duration_seconds = m.get("DurationSeconds")
        return self
