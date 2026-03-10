package errors

import (
	"errors"
	"fmt"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers: fake Tea-style error types for testing
// ---------------------------------------------------------------------------

// fakeTeaError simulates a *tea.SDKError with getter methods.
type fakeTeaError struct {
	code       string
	message    string
	statusCode int
}

func (e *fakeTeaError) Error() string {
	return fmt.Sprintf("[%d] %s: %s", e.statusCode, e.code, e.message)
}
func (e *fakeTeaError) GetCode() string      { return e.code }
func (e *fakeTeaError) GetMessage() string    { return e.message }
func (e *fakeTeaError) GetStatusCode() int    { return e.statusCode }

// plainError is a plain error with no structured fields.
type plainError struct{ msg string }

func (e *plainError) Error() string { return e.msg }

// ---------------------------------------------------------------------------
// Unit Tests: APIError
// ---------------------------------------------------------------------------

func TestAPIError_Error_WithCode(t *testing.T) {
	e := &APIError{
		HTTPStatus: 401,
		Code:       "Unauthorized",
		Message:    "access denied",
	}
	want := "[401] Unauthorized: access denied"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAPIError_Error_WithoutCode(t *testing.T) {
	e := &APIError{
		HTTPStatus: 500,
		Message:    "something broke",
	}
	want := "[500] something broke"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAPIError_ImplementsErrorInterface(t *testing.T) {
	var _ error = (*APIError)(nil)
}

// ---------------------------------------------------------------------------
// Unit Tests: MapTeaException
// ---------------------------------------------------------------------------

func TestMapTeaException_Nil(t *testing.T) {
	if got := MapTeaException(nil); got != nil {
		t.Errorf("MapTeaException(nil) = %v, want nil", got)
	}
}

func TestMapTeaException_StructuredError(t *testing.T) {
	err := &fakeTeaError{
		code:       "ProjectNotExist",
		message:    "The Project does not exist : my-project",
		statusCode: 404,
	}

	got := MapTeaException(err)
	if got == nil {
		t.Fatal("MapTeaException returned nil for structured error")
	}
	if got.Code != "ProjectNotExist" {
		t.Errorf("Code = %q, want %q", got.Code, "ProjectNotExist")
	}
	if got.HTTPStatus != 404 {
		t.Errorf("HTTPStatus = %d, want 404", got.HTTPStatus)
	}
}

func TestMapTeaException_PlainError(t *testing.T) {
	err := &plainError{msg: "connection refused"}

	got := MapTeaException(err)
	if got == nil {
		t.Fatal("MapTeaException returned nil for plain error")
	}
	if got.Code != "UnknownError" {
		t.Errorf("Code = %q, want %q", got.Code, "UnknownError")
	}
	if got.HTTPStatus != 500 {
		t.Errorf("HTTPStatus = %d, want 500", got.HTTPStatus)
	}
	if got.Message != "connection refused" {
		t.Errorf("Message = %q, want %q", got.Message, "connection refused")
	}
}

func TestMapTeaException_StdError(t *testing.T) {
	err := errors.New("timeout waiting for response")

	got := MapTeaException(err)
	if got == nil {
		t.Fatal("MapTeaException returned nil")
	}
	if got.Code != "UnknownError" {
		t.Errorf("Code = %q, want %q", got.Code, "UnknownError")
	}
	if got.Message != "timeout waiting for response" {
		t.Errorf("Message = %q, want original error message", got.Message)
	}
}

func TestMapTeaException_WithKnownMapping(t *testing.T) {
	// Temporarily add a known error mapping for this test.
	original := KnownErrors
	KnownErrors = []ErrorMapping{
		{
			HTTPStatus:  401,
			Code:        "SignatureNotMatch",
			Description: "请求的数字签名不匹配。",
			Solution:    "请您重试或更换AccessKey后重试。",
		},
	}
	defer func() { KnownErrors = original }()

	err := &fakeTeaError{
		code:       "SignatureNotMatch",
		message:    "Signature abc123 not matched.",
		statusCode: 401,
	}

	got := MapTeaException(err)
	if got == nil {
		t.Fatal("MapTeaException returned nil for known error")
	}
	if got.Description != "请求的数字签名不匹配。" {
		t.Errorf("Description = %q, want Chinese description", got.Description)
	}
	if got.Solution != "请您重试或更换AccessKey后重试。" {
		t.Errorf("Solution = %q, want solution text", got.Solution)
	}
}

func TestMapTeaException_KnownMappingWithPattern(t *testing.T) {
	original := KnownErrors
	KnownErrors = []ErrorMapping{
		{
			HTTPStatus:  401,
			Code:        "Unauthorized",
			Pattern:     "security token",
			Description: "STS Token不合法。",
			Solution:    "请检查您的STS接口请求。",
		},
		{
			HTTPStatus:  401,
			Code:        "Unauthorized",
			Description: "提供的AccessKey ID值未授权。",
			Solution:    "请确认您的AccessKey ID有访问日志服务权限。",
		},
	}
	defer func() { KnownErrors = original }()

	// Should match the pattern entry.
	err := &fakeTeaError{
		code:       "Unauthorized",
		message:    "The security token you provided is invalid.",
		statusCode: 401,
	}
	got := MapTeaException(err)
	if got == nil {
		t.Fatal("MapTeaException returned nil")
	}
	if got.Description != "STS Token不合法。" {
		t.Errorf("Description = %q, want pattern-matched description", got.Description)
	}

	// Should fall back to code-only match.
	err2 := &fakeTeaError{
		code:       "Unauthorized",
		message:    "some other unauthorized error",
		statusCode: 401,
	}
	got2 := MapTeaException(err2)
	if got2 == nil {
		t.Fatal("MapTeaException returned nil for code-only match")
	}
	if got2.Description != "提供的AccessKey ID值未授权。" {
		t.Errorf("Description = %q, want code-only fallback description", got2.Description)
	}
}

// ---------------------------------------------------------------------------
// Unit Tests: LookupKnownError
// ---------------------------------------------------------------------------

func TestLookupKnownError_NoMappings(t *testing.T) {
	original := KnownErrors
	KnownErrors = nil
	defer func() { KnownErrors = original }()

	if got := LookupKnownError("SomeCode", "some message"); got != nil {
		t.Errorf("LookupKnownError with empty table = %v, want nil", got)
	}
}

func TestLookupKnownError_CaseInsensitiveCode(t *testing.T) {
	original := KnownErrors
	KnownErrors = []ErrorMapping{
		{
			HTTPStatus:  500,
			Code:        "InternalServerError",
			Description: "服务器内部错误。",
			Solution:    "请您稍后重试。",
		},
	}
	defer func() { KnownErrors = original }()

	got := LookupKnownError("internalservererror", "something")
	if got == nil {
		t.Fatal("LookupKnownError should match case-insensitively")
	}
	if got.Code != "InternalServerError" {
		t.Errorf("Code = %q, want original casing", got.Code)
	}
}

// ---------------------------------------------------------------------------
// Unit Tests: KnownErrors mapping table (Task 6.2)
// ---------------------------------------------------------------------------

func TestKnownErrors_IsPopulated(t *testing.T) {
	// The Python version has 20 entries; Go version adds 3 more (EntityNotFound, NoRelatedDataSetFound, InvalidSPLFormat).
	if len(KnownErrors) != 23 {
		t.Errorf("KnownErrors has %d entries, want 23", len(KnownErrors))
	}
}

func TestKnownErrors_AllEntriesHaveRequiredFields(t *testing.T) {
	for i, e := range KnownErrors {
		if e.Code == "" {
			t.Errorf("KnownErrors[%d]: Code is empty", i)
		}
		if e.Description == "" {
			t.Errorf("KnownErrors[%d] (%s): Description is empty", i, e.Code)
		}
		if e.Solution == "" {
			t.Errorf("KnownErrors[%d] (%s): Solution is empty", i, e.Code)
		}
		if e.HTTPStatus == 0 {
			t.Errorf("KnownErrors[%d] (%s): HTTPStatus is 0", i, e.Code)
		}
	}
}

func TestLookupKnownError_AllCodes(t *testing.T) {
	// Verify every unique code in the table can be looked up.
	seen := map[string]bool{}
	for _, e := range KnownErrors {
		if seen[e.Code] {
			continue
		}
		seen[e.Code] = true

		got := LookupKnownError(e.Code, "any message")
		if got == nil {
			t.Errorf("LookupKnownError(%q, ...) returned nil", e.Code)
		}
	}
}

func TestLookupKnownError_UnauthorizedPatterns(t *testing.T) {
	tests := []struct {
		message     string
		wantDesc    string
	}{
		{"The security token you provided is invalid.", "STS Token不合法。"},
		{"The security token you provided has expired.", "STS Token已经过期。"},
		{"AccessKeyId not found: LTAI1234", "AccessKey ID不存在。"},
		{"AccessKeyId is disabled: LTAI1234", "AccessKey ID是禁用状态。"},
		{"Your SLS service has been forbidden.", "日志服务已经被禁用。"},
		{"The project does not belong to you.", "Project不属于当前访问用户。"},
		{"some other unauthorized error", "提供的AccessKey ID值未授权。"}, // fallback
	}

	for _, tt := range tests {
		got := LookupKnownError("Unauthorized", tt.message)
		if got == nil {
			t.Errorf("LookupKnownError(Unauthorized, %q) = nil", tt.message)
			continue
		}
		if got.Description != tt.wantDesc {
			t.Errorf("LookupKnownError(Unauthorized, %q).Description = %q, want %q",
				tt.message, got.Description, tt.wantDesc)
		}
	}
}

func TestLookupKnownError_InvalidAccessKeyIdPatterns(t *testing.T) {
	tests := []struct {
		message  string
		wantDesc string
	}{
		{"Your SLS service has not opened.", "日志服务没有开通。"},
		{"The access key id you provided is invalid: LTAI1234.", "AccessKey ID不合法。"}, // fallback
	}

	for _, tt := range tests {
		got := LookupKnownError("InvalidAccessKeyId", tt.message)
		if got == nil {
			t.Errorf("LookupKnownError(InvalidAccessKeyId, %q) = nil", tt.message)
			continue
		}
		if got.Description != tt.wantDesc {
			t.Errorf("LookupKnownError(InvalidAccessKeyId, %q).Description = %q, want %q",
				tt.message, got.Description, tt.wantDesc)
		}
	}
}

func TestLookupKnownError_UnknownCodeReturnsNil(t *testing.T) {
	got := LookupKnownError("CompletelyUnknownCode", "some message")
	if got != nil {
		t.Errorf("LookupKnownError for unknown code returned %v, want nil", got)
	}
}

func TestMapTeaException_UnknownCodeFallback(t *testing.T) {
	err := &fakeTeaError{
		code:       "NeverHeardOfThis",
		message:    "something unexpected happened",
		statusCode: 418,
	}

	got := MapTeaException(err)
	if got == nil {
		t.Fatal("MapTeaException returned nil for unknown code")
	}
	// Req 15.3: unknown codes return original info marked as unknown type
	if got.Code != "NeverHeardOfThis" {
		t.Errorf("Code = %q, want original code preserved", got.Code)
	}
	if got.Message != "something unexpected happened" {
		t.Errorf("Message = %q, want original message preserved", got.Message)
	}
	if got.HTTPStatus != 418 {
		t.Errorf("HTTPStatus = %d, want 418", got.HTTPStatus)
	}
}

func TestLookupKnownError_SpecificCodes(t *testing.T) {
	tests := []struct {
		code     string
		wantHTTP int
		wantDesc string
	}{
		{"RequestTimeExpired", 400, "请求时间和服务端时间差别超过15分钟。"},
		{"ProjectAlreadyExist", 400, "Project名称已存在。Project名称在阿里云地域内全局唯一。"},
		{"SignatureNotMatch", 401, "请求的数字签名不匹配。"},
		{"WriteQuotaExceed", 403, "超过写入日志限额。"},
		{"ReadQuotaExceed", 403, "超过读取日志限额。"},
		{"MetaOperationQpsLimitExceeded", 403, "超出默认设置的QPS阈值。"},
		{"ProjectForbidden", 403, "Project已经被禁用。"},
		{"ProjectNotExist", 404, "日志项目（Project）不存在。"},
		{"PostBodyTooLarge", 413, "请求消息体body不能超过10M。"},
		{"InternalServerError", 500, "服务器内部错误。"},
		{"RequestTimeout", 500, "请求处理超时。"},
	}

	for _, tt := range tests {
		got := LookupKnownError(tt.code, "any message")
		if got == nil {
			t.Errorf("LookupKnownError(%q) = nil", tt.code)
			continue
		}
		if got.HTTPStatus != tt.wantHTTP {
			t.Errorf("LookupKnownError(%q).HTTPStatus = %d, want %d", tt.code, got.HTTPStatus, tt.wantHTTP)
		}
		if got.Description != tt.wantDesc {
			t.Errorf("LookupKnownError(%q).Description = %q, want %q", tt.code, got.Description, tt.wantDesc)
		}
	}
}
