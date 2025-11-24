# -*- coding: utf-8 -*-
# This file is auto-generated, don't edit it. Thanks.
from __future__ import annotations

from darabonba.model import DaraModel


class AssumeRoleWithSAMLResponseBodySAMLAssertionInfo(DaraModel):
    def __init__(
        self,
        *,
        issuer: str = None,
        recipient: str = None,
        subject: str = None,
        subject_type: str = None,
    ):
        # The value in the `Issuer` element in the SAML assertion.
        self.issuer = issuer
        # The `Recipient` attribute of the SubjectConfirmationData sub-element. SubjectConfirmationData is a sub-element of the `Subject` element in the SAML assertion.
        self.recipient = recipient
        # The value in the NameID sub-element of the `Subject` element in the SAML assertion.
        self.subject = subject
        # The Format attribute of the `NameID` element in the SAML assertion. If the Format attribute is prefixed with `urn:oasis:names:tc:SAML:2.0:nameid-format:`, the prefix is not included in the value of this parameter. For example, if the value of the Format attribute is urn:oasis:names:tc:SAML:2.0:nameid-format:persistent/transient, the value of this parameter is `persistent/transient`.
        self.subject_type = subject_type

    def validate(self):
        pass

    def to_map(self):
        _map = super().to_map()
        if _map is not None:
            return _map

        result = dict()
        if self.issuer is not None:
            result["Issuer"] = self.issuer
        if self.recipient is not None:
            result["Recipient"] = self.recipient
        if self.subject is not None:
            result["Subject"] = self.subject
        if self.subject_type is not None:
            result["SubjectType"] = self.subject_type
        return result

    def from_map(self, m: dict = None):
        m = m or dict()
        if m.get("Issuer") is not None:
            self.issuer = m.get("Issuer")
        if m.get("Recipient") is not None:
            self.recipient = m.get("Recipient")
        if m.get("Subject") is not None:
            self.subject = m.get("Subject")
        if m.get("SubjectType") is not None:
            self.subject_type = m.get("SubjectType")
        return self
