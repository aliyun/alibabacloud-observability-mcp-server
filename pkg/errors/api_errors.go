// Package errors – API error code mapping table.
// The full mapping table will be populated in Task 6.2.
package errors

import "strings"

// ErrorMapping defines a mapping from an Alibaba Cloud API error code
// (and optional message pattern) to a structured APIError with Chinese
// description and suggested solution.
type ErrorMapping struct {
	HTTPStatus  int
	Code        string
	Pattern     string // substring match against error message (empty = match any)
	Description string // 中文描述
	Solution    string // 建议解决方案
}

// KnownErrors is the predefined error code mapping table.
// Populated from the Python version's api_error.py.
var KnownErrors = []ErrorMapping{
	// 400
	{400, "RequestTimeExpired", "", "请求时间和服务端时间差别超过15分钟。", "请您检查请求端时间，稍后重试。"},
	{400, "ProjectAlreadyExist", "", "Project名称已存在。Project名称在阿里云地域内全局唯一。", "请您更换Project名称后重试。"},
	{400, "InvalidSPLFormat", "", "SPL查询语法格式无效。", "请检查SPL查询语法是否正确，参考阿里云SPL文档。"},
	{400, "EntityNotFound", "", "实体未找到。可能原因：实体已过期、entity_set_name不匹配、或实体ID格式错误。", "请使用 umodel_get_entities 或 umodel_fuzzy_search_entities 获取当前有效的实体ID。k8s域使用32位hash格式ID，apm域使用包含@符号的ID格式。"},
	{400, "NoRelatedDataSetFound", "", "未找到关联的数据集。metric_domain_name与entity_set_name可能不兼容。", "请先调用 umodel_list_data_set 确认可用的数据集，确保metric_domain_name与entity_set_name的组合正确。"},

	// 401 – SignatureNotMatch
	{401, "SignatureNotMatch", "", "请求的数字签名不匹配。", "请您重试或更换AccessKey后重试。"},

	// 401 – Unauthorized (pattern-based)
	{401, "Unauthorized", "security token you provided is invalid", "STS Token不合法。", "请检查您的STS接口请求，确认STS Token是合法有效的。"},
	{401, "Unauthorized", "security token you provided has expired", "STS Token已经过期。", "请重新申请STS Token后发起请求。"},
	{401, "Unauthorized", "accesskeyid not found", "AccessKey ID不存在。", "请检查您的AccessKey ID，重新获取后再发起请求。"},
	{401, "Unauthorized", "accesskeyid is disabled", "AccessKey ID是禁用状态。", "请检查您的AccessKey ID，确认为已启用状态后重新发起请求。"},
	{401, "Unauthorized", "service has been forbidden", "日志服务已经被禁用。", "请检查您的日志服务状态，例如是否已欠费。"},
	{401, "Unauthorized", "project does not belong to you", "Project不属于当前访问用户。", "请更换Project或者访问用户后重试。"},
	// 401 – Unauthorized (code-only fallback, must be AFTER pattern entries)
	{401, "Unauthorized", "", "提供的AccessKey ID值未授权。", "请确认您的AccessKey ID有访问日志服务权限。"},

	// 401 – InvalidAccessKeyId (pattern-based)
	{401, "InvalidAccessKeyId", "service has not opened", "日志服务没有开通。", "请登录日志服务控制台或者通过API开通日志服务后，重新发起请求。"},
	// 401 – InvalidAccessKeyId (code-only fallback)
	{401, "InvalidAccessKeyId", "", "AccessKey ID不合法。", "请检查您的AccessKey ID，确认AccessKey ID是合法有效的。"},

	// 403
	{403, "WriteQuotaExceed", "", "超过写入日志限额。", "请您优化写入日志请求，减少写入日志数量。"},
	{403, "ReadQuotaExceed", "", "超过读取日志限额。", "请您优化读取日志请求，减少读取日志数量。"},
	{403, "MetaOperationQpsLimitExceeded", "", "超出默认设置的QPS阈值。", "请您优化资源操作请求，减少资源操作次数。建议您延迟几秒后重试。"},
	{403, "ProjectForbidden", "", "Project已经被禁用。", "请检查Project状态，您的Project当前可能已经欠费。"},

	// 404
	{404, "ProjectNotExist", "", "日志项目（Project）不存在。", "请您检查Project名称，确认已存在该Project或者地域是否正确。"},

	// 413
	{413, "PostBodyTooLarge", "", "请求消息体body不能超过10M。", "请您调整请求消息体的大小后重试。"},

	// 500
	{500, "InternalServerError", "", "服务器内部错误。", "请您稍后重试。"},
	{500, "RequestTimeout", "", "请求处理超时。", "请您稍后重试。"},
}

// LookupKnownError searches KnownErrors for a match. It first tries an exact
// code + message-pattern match, then falls back to a code-only match (entry
// with empty Pattern). Returns nil if no match is found.
func LookupKnownError(code, message string) *APIError {
	var codeOnlyMatch *ErrorMapping

	for i := range KnownErrors {
		e := &KnownErrors[i]
		if !strings.EqualFold(e.Code, code) {
			continue
		}
		// If the entry has a pattern, check for substring match.
		if e.Pattern != "" {
			if strings.Contains(strings.ToLower(message), strings.ToLower(e.Pattern)) {
				return &APIError{
					HTTPStatus:  e.HTTPStatus,
					Code:        e.Code,
					Message:     message,
					Description: e.Description,
					Solution:    e.Solution,
				}
			}
		} else if codeOnlyMatch == nil {
			codeOnlyMatch = e
		}
	}

	if codeOnlyMatch != nil {
		return &APIError{
			HTTPStatus:  codeOnlyMatch.HTTPStatus,
			Code:        codeOnlyMatch.Code,
			Message:     message,
			Description: codeOnlyMatch.Description,
			Solution:    codeOnlyMatch.Solution,
		}
	}

	return nil
}
