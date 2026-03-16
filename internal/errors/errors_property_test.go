package errors

import (
	"fmt"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// teaError is a minimal implementation of teaErrorAccessor for testing.
type teaError struct {
	code       string
	message    string
	statusCode int
}

func (e *teaError) Error() string      { return fmt.Sprintf("%s: %s", e.code, e.message) }
func (e *teaError) GetCode() string    { return e.code }
func (e *teaError) GetMessage() string { return e.message }
func (e *teaError) GetStatusCode() int { return e.statusCode }

// TestProperty_TeaExceptionMappingCompleteness verifies that for any known error
// code in the mapping table, MapTeaException returns an APIError with non-empty
// Description and non-empty Solution fields.
//
// Feature: go-mcp-server-rewrite, Property 11: TeaException 映射完整性
// Validates: Requirements 12.2, 15.2
func TestProperty_TeaExceptionMappingCompleteness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator: pick a random index into KnownErrors to get a known code+pattern pair.
	genKnownErrorIndex := gen.IntRange(0, len(KnownErrors)-1)

	properties.Property("known error codes produce APIError with non-empty Description and Solution", prop.ForAll(
		func(idx int) bool {
			entry := KnownErrors[idx]

			// Build a message that will match the pattern (if any).
			message := "test error message"
			if entry.Pattern != "" {
				message = "prefix " + entry.Pattern + " suffix"
			}

			err := &teaError{
				code:       entry.Code,
				message:    message,
				statusCode: entry.HTTPStatus,
			}

			apiErr := MapTeaException(err)
			if apiErr == nil {
				t.Logf("MapTeaException returned nil for code=%q message=%q", entry.Code, message)
				return false
			}
			if apiErr.Description == "" {
				t.Logf("Description is empty for code=%q", entry.Code)
				return false
			}
			if apiErr.Solution == "" {
				t.Logf("Solution is empty for code=%q", entry.Code)
				return false
			}
			return true
		},
		genKnownErrorIndex,
	))

	properties.TestingRun(t)
}

// TestProperty_UnknownErrorCodeFallback verifies that for any error code NOT in
// the mapping table, MapTeaException returns an APIError that preserves the
// original error message and marks the Code as "UnknownError" (or the original
// code if provided).
//
// Feature: go-mcp-server-rewrite, Property 12: 未知错误码回退
// Validates: Requirements 15.3
func TestProperty_UnknownErrorCodeFallback(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Collect all known codes for filtering.
	knownCodes := make(map[string]bool)
	for _, e := range KnownErrors {
		knownCodes[e.Code] = true
	}

	// Generator: produce error codes that are NOT in the known mapping table.
	genUnknownCode := gen.RegexMatch(`[A-Z][a-zA-Z]{5,15}Error`).SuchThat(func(v interface{}) bool {
		code := v.(string)
		return !knownCodes[code]
	})

	genMessage := gen.RegexMatch(`[a-zA-Z ]{5,30}`)

	properties.Property("unknown error codes produce APIError with original message and unknown-type Code", prop.ForAll(
		func(code, message string) bool {
			err := &teaError{
				code:       code,
				message:    message,
				statusCode: 418,
			}

			apiErr := MapTeaException(err)
			if apiErr == nil {
				t.Logf("MapTeaException returned nil for unknown code=%q", code)
				return false
			}

			// The original message must be preserved.
			if apiErr.Message != message {
				t.Logf("Message not preserved: got %q, want %q", apiErr.Message, message)
				return false
			}

			// The code should be the original code (not remapped to a known code).
			if apiErr.Code != code {
				t.Logf("Code changed: got %q, want %q", apiErr.Code, code)
				return false
			}

			// HTTP status from the tea error should be preserved.
			if apiErr.HTTPStatus != 418 {
				t.Logf("HTTPStatus not preserved: got %d, want 418", apiErr.HTTPStatus)
				return false
			}

			return true
		},
		genUnknownCode,
		genMessage,
	))

	// Also test with plain (unstructured) errors — no code at all.
	properties.Property("plain errors without code get UnknownError code", prop.ForAll(
		func(message string) bool {
			err := fmt.Errorf("%s", message)

			apiErr := MapTeaException(err)
			if apiErr == nil {
				return false
			}

			// Code should be "UnknownError" for unstructured errors.
			if apiErr.Code != "UnknownError" {
				t.Logf("Expected UnknownError code, got %q", apiErr.Code)
				return false
			}

			// Original message must be preserved.
			if apiErr.Message != message {
				t.Logf("Message not preserved: got %q, want %q", apiErr.Message, message)
				return false
			}

			return true
		},
		gen.RegexMatch(`[a-zA-Z ]{5,30}`),
	))

	properties.TestingRun(t)
}
