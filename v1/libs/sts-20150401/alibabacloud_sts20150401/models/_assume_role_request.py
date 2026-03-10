# -*- coding: utf-8 -*-
# This file is auto-generated, don't edit it. Thanks.
from __future__ import annotations

from darabonba.model import DaraModel


class AssumeRoleRequest(DaraModel):
    def __init__(
        self,
        *,
        duration_seconds: int = None,
        external_id: str = None,
        policy: str = None,
        role_arn: str = None,
        role_session_name: str = None,
    ):
        # The validity period of the STS token. Unit: seconds.
        #
        # Minimum value: 900. Maximum value: the value of the `MaxSessionDuration` parameter. Default value: 3600.
        #
        # You can call the CreateRole or UpdateRole operation to configure the `MaxSessionDuration` parameter. For more information, see [CreateRole](https://help.aliyun.com/document_detail/28710.html) or [UpdateRole](https://help.aliyun.com/document_detail/28712.html).
        self.duration_seconds = duration_seconds
        # The external ID of the RAM role.
        #
        # This parameter is provided by an external party and is used to prevent the confused deputy problem. For more information, see [Use ExternalId to prevent the confused deputy problem](https://help.aliyun.com/document_detail/2361741.html).
        #
        # The value must be 2 to 1,224 characters in length and can contain letters, digits, and the following special characters: `= , . @ : / - _`. The regular expression for this parameter is `[\\w+=,.@:\\/-]*`.
        self.external_id = external_id
        # The policy that specifies the permissions of the returned STS token. You can use this parameter to grant the STS token fewer permissions than the permissions granted to the RAM role.
        #
        # *   If you specify this parameter, the permissions of the returned STS token are the permissions that are included in the value of this parameter and owned by the RAM role.
        # *   If you do not specify this parameter, the returned STS token has all the permissions of the RAM role.
        #
        # The value must be 1 to 2,048 characters in length.
        #
        # For more information about policy elements and sample policies, see [Policy elements](https://help.aliyun.com/document_detail/93738.html) and [Overview of sample policies](https://help.aliyun.com/document_detail/210969.html).
        self.policy = policy
        # The Alibaba Cloud Resource Name (ARN) of the RAM role.
        #
        # The trusted entity of the RAM role is an Alibaba Cloud account. For more information, see [Create a RAM role for a trusted Alibaba Cloud account](https://help.aliyun.com/document_detail/93691.html) or [CreateRole](https://help.aliyun.com/document_detail/28710.html).
        #
        # Format: `acs:ram::<account_id>:role/<role_name>`.
        #
        # You can view the ARN in the RAM console or by calling operations. The following items describe the validity periods of storage addresses:
        #
        # *   For more information about how to view the ARN in the RAM console, see [How do I find the ARN of the RAM role?](https://help.aliyun.com/document_detail/39744.html)
        # *   For more information about how to view the ARN by calling operations, see [ListRoles](https://help.aliyun.com/document_detail/28713.html) or [GetRole](https://help.aliyun.com/document_detail/28711.html).
        #
        # This parameter is required.
        self.role_arn = role_arn
        # The custom name of the role session.
        #
        # Set this parameter based on your business requirements. In most cases, you can set this parameter to the identity of the API caller. For example, you can specify a username. You can specify `RoleSessionName` to identify API callers that assume the same RAM role in ActionTrail logs. This allows you to track the users that perform the operations.
        #
        # The value must be 2 to 64 characters in length and can contain letters, digits, and the following special characters: `. @ - _`.
        #
        # This parameter is required.
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
        if self.external_id is not None:
            result["ExternalId"] = self.external_id
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
        if m.get("ExternalId") is not None:
            self.external_id = m.get("ExternalId")
        if m.get("Policy") is not None:
            self.policy = m.get("Policy")
        if m.get("RoleArn") is not None:
            self.role_arn = m.get("RoleArn")
        if m.get("RoleSessionName") is not None:
            self.role_session_name = m.get("RoleSessionName")
        return self
