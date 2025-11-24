# -*- coding: utf-8 -*-
# This file is auto-generated, don't edit it. Thanks.
from __future__ import annotations

from darabonba.model import DaraModel


class GenerateTokenByTicketRequest(DaraModel):
    def __init__(
        self,
        *,
        ticket: str = None,
        ticket_type: str = None,
    ):
        # This parameter is required.
        self.ticket = ticket
        self.ticket_type = ticket_type

    def validate(self):
        pass

    def to_map(self):
        _map = super().to_map()
        if _map is not None:
            return _map

        result = dict()
        if self.ticket is not None:
            result["Ticket"] = self.ticket
        if self.ticket_type is not None:
            result["TicketType"] = self.ticket_type
        return result

    def from_map(self, m: dict = None):
        m = m or dict()
        if m.get("Ticket") is not None:
            self.ticket = m.get("Ticket")
        if m.get("TicketType") is not None:
            self.ticket_type = m.get("TicketType")
        return self
