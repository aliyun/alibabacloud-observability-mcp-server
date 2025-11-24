# -*- coding: utf-8 -*-
# This file is auto-generated, don't edit it. Thanks.
from __future__ import annotations

from darabonba.model import DaraModel


class AssumeRoleWithOIDCRequest(DaraModel):
    def __init__(
        self,
        *,
        duration_seconds: int = None,
        oidcprovider_arn: str = None,
        oidctoken: str = None,
        policy: str = None,
        role_arn: str = None,
        role_session_name: str = None,
    ):
        # The validity period of the STS token. Unit: seconds.
        #
        # Default value: 3600. Minimum value: 900. Maximum value: the value of the `MaxSessionDuration` parameter.
        #
        # For more information about how to specify `MaxSessionDuration`, see [CreateRole](https://help.aliyun.com/document_detail/28710.html) or [UpdateRole](https://help.aliyun.com/document_detail/28712.html).
        self.duration_seconds = duration_seconds
        # The Alibaba Cloud Resource Name (ARN) of the OIDC IdP.
        #
        # You can view the ARN in the RAM console or by calling operations.
        #
        # *   For more information about how to view the ARN in the RAM console, see [View the information about an OIDC IdP](https://help.aliyun.com/document_detail/327123.html).
        # *   For more information about how to view the ARN by calling operations, see [GetOIDCProvider](https://help.aliyun.com/document_detail/327126.html) or [ListOIDCProviders](https://help.aliyun.com/document_detail/327127.html).
        self.oidcprovider_arn = oidcprovider_arn
        # The OIDC token that is issued by the external IdP.
        #
        # The OIDC token must be 4 to 20,000 characters in length.
        #
        # > You must enter the original OIDC token. You do not need to enter the Base64-encoded OIDC token.
        self.oidctoken = oidctoken
        # The policy that specifies the permissions of the returned STS token. You can use this parameter to grant the STS token fewer permissions than the permissions granted to the RAM role.
        #
        # *   If you specify this parameter, the permissions of the returned STS token are the permissions that are included in the value of this parameter and owned by the RAM role.
        # *   If you do not specify this parameter, the returned STS token has all the permissions of the RAM role.
        #
        # The value must be 1 to 2,048 characters in length.
        self.policy = policy
        # The ARN of the RAM role.
        #
        # You can view the ARN in the RAM console or by calling operations.
        #
        # *   For more information about how to view the ARN in the RAM console, see [How do I view the ARN of the RAM role?](https://help.aliyun.com/document_detail/39744.html)
        # *   For more information about how to view the ARN by calling operations, see [ListRoles](https://help.aliyun.com/document_detail/28713.html) or [GetRole](https://help.aliyun.com/document_detail/28711.html).
        self.role_arn = role_arn
        # The custom name of the role session.
        #
        # Set this parameter based on your business requirements. In most cases, this parameter is set to the identity of the user who calls the operation, for example, the username. In ActionTrail logs, you can distinguish the users who assume the same RAM role to perform operations based on the value of the RoleSessionName parameter. This way, you can perform user-specific auditing.
        #
        # The value can contain letters, digits, periods (.), at signs (@), hyphens (-), and underscores (_).
        #
        # The value must be 2 to 64 characters in length.
        self.role_session_name = role_session_name

    def validate(self):
        pass

    def to_map(self):
        _map = super().to_map()
        if _map is not None:
            return _map

        result = dict()
        if self.duration_seconds is not None:
            result["DurationSeconds"] = self.duration_seconds
        if self.oidcprovider_arn is not None:
            result["OIDCProviderArn"] = self.oidcprovider_arn
        if self.oidctoken is not None:
            result["OIDCToken"] = self.oidctoken
        if self.policy is not None:
            result["Policy"] = self.policy
        if self.role_arn is not None:
            result["RoleArn"] = self.role_arn
        if self.role_session_name is not None:
            result["RoleSessionName"] = self.role_session_name
        return result

    def from_map(self, m: dict = None):
        m = m or dict()
        if m.get("DurationSeconds") is not None:
            self.duration_seconds = m.get("DurationSeconds")
        if m.get("OIDCProviderArn") is not None:
            self.oidcprovider_arn = m.get("OIDCProviderArn")
        if m.get("OIDCToken") is not None:
            self.oidctoken = m.get("OIDCToken")
        if m.get("Policy") is not None:
            self.policy = m.get("Policy")
        if m.get("RoleArn") is not None:
            self.role_arn = m.get("RoleArn")
        if m.get("RoleSessionName") is not None:
            self.role_session_name = m.get("RoleSessionName")
        return self
