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

package mcp

import "fmt"

// ErrorCode is a stable, machine-readable identifier for a runtime error category.
type ErrorCode string

const (
	ErrCodeToolNotFound            ErrorCode = "TOOL_NOT_FOUND"
	ErrCodeToolUnavailable         ErrorCode = "TOOL_UNAVAILABLE"
	ErrCodeToolDisabled            ErrorCode = "TOOL_DISABLED"
	ErrCodeSessionNotFound         ErrorCode = "SESSION_NOT_FOUND"
	ErrCodeSessionUnavailable      ErrorCode = "SESSION_UNAVAILABLE"
	ErrCodePolicyDenied            ErrorCode = "POLICY_DENIED"
	ErrCodeQuotaExceeded           ErrorCode = "QUOTA_EXCEEDED"
	ErrCodeRateLimited             ErrorCode = "RATE_LIMITED"
	ErrCodeConcurrencyLimitReached ErrorCode = "CONCURRENCY_LIMIT_REACHED"
	ErrCodeCredentialUnavailable   ErrorCode = "CREDENTIAL_UNAVAILABLE" //nolint:gosec
	ErrCodeInvocationTimeout       ErrorCode = "INVOCATION_TIMEOUT"
	ErrCodeTransportFailure        ErrorCode = "TRANSPORT_FAILURE"
	ErrCodeInvalidRequest          ErrorCode = "INVALID_REQUEST"
	ErrCodeInternal                ErrorCode = "INTERNAL"
)

// RuntimeError is the standard error type used within the runtime.
//
// All errors produced by the runtime are either RuntimeError values or wrap one.
// Callers inspect the Code field to determine the error category.
type RuntimeError struct {
	Code    ErrorCode
	Message string
	Cause   error
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
var ErrToolNotFound = NewRuntimeError(ErrCodeToolNotFound, "tool not found")
