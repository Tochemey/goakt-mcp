// MIT License
//
// Copyright (c) 2026 GoAkt Team
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//

package runtime

import "fmt"

// ErrorCode is a stable, machine-readable identifier for a runtime error category.
//
// Ingress adapters use ErrorCode values to produce consistent HTTP status codes
// and error response bodies without parsing message strings.
type ErrorCode string

const (
	// ErrCodeToolNotFound is returned when the requested tool is not registered.
	ErrCodeToolNotFound ErrorCode = "TOOL_NOT_FOUND"

	// ErrCodeToolUnavailable is returned when the tool's circuit is open and
	// the tool cannot accept new requests.
	ErrCodeToolUnavailable ErrorCode = "TOOL_UNAVAILABLE"

	// ErrCodeToolDisabled is returned when the tool has been administratively disabled.
	ErrCodeToolDisabled ErrorCode = "TOOL_DISABLED"

	// ErrCodeSessionNotFound is returned when a requested session does not exist.
	ErrCodeSessionNotFound ErrorCode = "SESSION_NOT_FOUND"

	// ErrCodeSessionUnavailable is returned when a session exists but cannot
	// accept work (e.g., it is draining or recovering).
	ErrCodeSessionUnavailable ErrorCode = "SESSION_UNAVAILABLE"

	// ErrCodePolicyDenied is returned when the authorization policy rejects the request.
	ErrCodePolicyDenied ErrorCode = "POLICY_DENIED"

	// ErrCodeQuotaExceeded is returned when the tenant or tool quota is exhausted.
	ErrCodeQuotaExceeded ErrorCode = "QUOTA_EXCEEDED"

	// ErrCodeRateLimited is returned when the request exceeds the configured rate limit.
	ErrCodeRateLimited ErrorCode = "RATE_LIMITED"

	// ErrCodeConcurrencyLimitReached is returned when the maximum number of concurrent
	// invocations for the tool or tenant has been reached.
	ErrCodeConcurrencyLimitReached ErrorCode = "CONCURRENCY_LIMIT_REACHED"

	// ErrCodeCredentialUnavailable is returned when the credential broker cannot
	// resolve the credentials required for the tool invocation.
	ErrCodeCredentialUnavailable ErrorCode = "CREDENTIAL_UNAVAILABLE" //nolint:gosec // not a credential, it is an error code string

	// ErrCodeInvocationTimeout is returned when the tool invocation exceeds its deadline.
	ErrCodeInvocationTimeout ErrorCode = "INVOCATION_TIMEOUT"

	// ErrCodeTransportFailure is returned when the egress transport layer fails
	// in a way that cannot be recovered within the current invocation.
	ErrCodeTransportFailure ErrorCode = "TRANSPORT_FAILURE"

	// ErrCodeInvalidRequest is returned when the invocation payload fails validation.
	ErrCodeInvalidRequest ErrorCode = "INVALID_REQUEST"

	// ErrCodeInternal is returned for unexpected internal runtime errors that do not
	// map to a more specific error code.
	ErrCodeInternal ErrorCode = "INTERNAL"
)

// RuntimeError is the standard error type used within the runtime.
//
// All errors produced by the runtime are either RuntimeError values or wrap one.
// Ingress adapters inspect the Code field to determine the correct HTTP response
// without parsing message text.
type RuntimeError struct {
	// Code is the stable machine-readable error category.
	Code ErrorCode

	// Message is a human-readable description of the error suitable for logging.
	Message string

	// Cause is the underlying error, if one exists. It is available via errors.Unwrap.
	Cause error
}

// Error implements the error interface.
func (e *RuntimeError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause, enabling use with errors.Is and errors.As.
func (e *RuntimeError) Unwrap() error { return e.Cause }

// NewRuntimeError constructs a RuntimeError with the given code and message.
func NewRuntimeError(code ErrorCode, message string) *RuntimeError {
	return &RuntimeError{Code: code, Message: message}
}

// WrapRuntimeError constructs a RuntimeError that annotates an underlying cause.
func WrapRuntimeError(code ErrorCode, message string, cause error) *RuntimeError {
	return &RuntimeError{Code: code, Message: message, Cause: cause}
}

// ErrToolNotFound is the error returned when a requested tool is not in the registry.
// Use errors.Is(err, ErrToolNotFound) to detect this condition.
var ErrToolNotFound = NewRuntimeError(ErrCodeToolNotFound, "tool not found")
