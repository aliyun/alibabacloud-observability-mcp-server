# -*- coding: utf-8 -*-
# This file is auto-generated, don't edit it. Thanks.
from __future__ import annotations

from darabonba.model import DaraModel


class AssumeRoleWithSAMLRequest(DaraModel):
    def __init__(
        self,
        *,
        duration_seconds: int = None,
        policy: str = None,
        role_arn: str = None,
        samlassertion: str = None,
        samlprovider_arn: str = None,
    ):
        # The validity period of the STS token. Unit: seconds.
        #
        # Minimum value: 900. Maximum value: the value of the `MaxSessionDuration` parameter. Default value: 3600.
        #
        # You can call the CreateRole or UpdateRole operation to configure the `MaxSessionDuration` parameter. For more information, see [CreateRole](https://help.aliyun.com/document_detail/28710.html) or [UpdateRole](https://help.aliyun.com/document_detail/28712.html).
        self.duration_seconds = duration_seconds
        # The policy that specifies the permissions of the returned STS token. You can use this parameter to grant the STS token fewer permissions than the permissions granted to the RAM role.
        #
        # *   If you specify this parameter, the permissions of the returned STS token are the permissions that are included in the value of this parameter and owned by the RAM role.
        # *   If you do not specify this parameter, the returned STS token has all the permissions of the RAM role.
        #
        # The value must be 1 to 2,048 characters in length.
        self.policy = policy
        # The ARN of the RAM role.
        #
        # The trust entity of the RAM role is a SAML IdP. For more information, see [Create a RAM role for a trusted IdP](https://help.aliyun.com/document_detail/116805.html) or [CreateRole](https://help.aliyun.com/document_detail/28710.html).
        #
        # Format: `acs:ram::<account_id>:role/<role_name>`.
        #
        # You can view the ARN in the RAM console or by calling operations.
        #
        # *   For more information about how to view the ARN in the RAM console, see [How do I view the ARN of the RAM role?](https://help.aliyun.com/document_detail/39744.html).
        # *   For more information about how to view the ARN by calling operations, see [ListRoles](https://help.aliyun.com/document_detail/28713.html) or [GetRole](https://help.aliyun.com/document_detail/28711.html).
        self.role_arn = role_arn
        # The Base64-encoded SAML assertion.
        #
        # The value must be 4 to 100,000 characters in length.
        #
        # > A complete SAML response rather than a single SAMLAssertion field must be retrieved from the external IdP.
        self.samlassertion = samlassertion
        # The Alibaba Cloud Resource Name (ARN) of the SAML IdP that is created in the RAM console.
        #
        # Format: `acs:ram::<account_id>:saml-provider/<saml_provider_id>`.
        #
        # You can view the ARN in the RAM console or by calling operations.
        #
        # *   For more information about how to view the ARN in the RAM console, see [How do I view the ARN of a RAM role?](https://help.aliyun.com/document_detail/116795.html)
        # *   For more information about how to view the ARN by calling operations, see [GetSAMLProvider](https://help.aliyun.com/document_detail/186833.html) or [ListSAMLProviders](https://help.aliyun.com/document_detail/186851.html).
        self.samlprovider_arn = samlprovider_arn

    def validate(self):
        pass

    def to_map(self):
        _map = super().to_map()
        if _map is not None:
            return _map

        result = dict()
        if self.duration_seconds is not None:
            result["DurationSeconds"] = self.duration_seconds
        if self.policy is not None:
            result["Policy"] = self.policy
        if self.role_arn is not None:
            result["RoleArn"] = self.role_arn
        if self.samlassertion is not None:
            result["SAMLAssertion"] = self.samlassertion
        if self.samlprovider_arn is not None:
            result["SAMLProviderArn"] = self.samlprovider_arn
        return result

    def from_map(self, m: dict = None):
        m = m or dict()
        if m.get("DurationSeconds") is not None:
            self.duration_seconds = m.get("DurationSeconds")
        if m.get("Policy") is not None:
            self.policy = m.get("Policy")
        if m.get("RoleArn") is not None:
            self.role_arn = m.get("RoleArn")
        if m.get("SAMLAssertion") is not None:
            self.samlassertion = m.get("SAMLAssertion")
        if m.get("SAMLProviderArn") is not None:
            self.samlprovider_arn = m.get("SAMLProviderArn")
        return self
