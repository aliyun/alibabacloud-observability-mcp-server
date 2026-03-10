// Package errors provides structured error types for the MCP server.
// It defines APIError for representing Alibaba Cloud API errors with
// Chinese descriptions and suggested solutions, and provides mapping
// from Tea SDK exceptions to structured errors.
package errors

import "fmt"

// APIError represents a structured API error with HTTP status, error code,
// human-readable message, Chinese description, and a suggested solution.
type APIError struct {
	HTTPStatus  int    `json:"httpStatus"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	Description string `json:"description"` // 中文描述
	Solution    string `json:"solution"`    // 建议解决方案
}

// Error implements the error interface, returning a formatted string that
// includes the HTTP status, error code, and message.
func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("[%d] %s: %s", e.HTTPStatus, e.Code, e.Message)
	}
	return fmt.Sprintf("[%d] %s", e.HTTPStatus, e.Message)
}

// teaErrorAccessor is satisfied by tea.SDKError and similar types that
// expose Code, Message, and StatusCode via getter methods. We use an
// interface so this package does not need to import the Tea SDK directly.
type teaErrorAccessor interface {
	error
	GetCode() string
	GetMessage() string
	GetStatusCode() int
}

// MapTeaException converts an error (typically a Tea SDK exception) into a
// structured APIError. It attempts to extract code, message, and status from
// the error using interface-based introspection.
//
// If the error does not carry structured fields, a generic 500 APIError is
// returned with the original error message preserved.
func MapTeaException(err error) *APIError {
	if err == nil {
		return nil
	}

	code, message, status := extractTeaFields(err)

	// Look up a known error mapping if we have a code.
	if code != "" {
		if mapped := lookupError(code, message); mapped != nil {
			// Override HTTP status if the Tea error provided one.
			if status != 0 {
				mapped.HTTPStatus = status
			}
			return mapped
		}
	}

	// Fallback: unknown or unstructured error.
	if status == 0 {
		status = 500
	}
	if code == "" {
		code = "UnknownError"
	}
	if message == "" {
		message = err.Error()
	}

	return &APIError{
		HTTPStatus:  status,
		Code:        code,
		Message:     message,
		Description: message,
		Solution:    "",
	}
}

// extractTeaFields tries to pull code, message, and HTTP status out of an
// error value via the teaErrorAccessor interface. Falls back to the raw
// error string when the error doesn't implement the interface.
func extractTeaFields(err error) (code, message string, status int) {
	if acc, ok := err.(teaErrorAccessor); ok {
		return acc.GetCode(), acc.GetMessage(), acc.GetStatusCode()
	}

	// Unstructured error — use the error string as the message.
	return "", err.Error(), 0
}

// lookupError searches the known error mappings for a match. It first tries
// an exact code+message match, then falls back to code-only match.
// This function will be fully populated in Task 6.2 (api_errors.go).
func lookupError(code, message string) *APIError {
	// Delegate to the mapping table defined in api_errors.go.
	return LookupKnownError(code, message)
}
