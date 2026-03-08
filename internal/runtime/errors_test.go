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

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorCodeConstants(t *testing.T) {
	assert.Equal(t, ErrorCode("TOOL_NOT_FOUND"), ErrCodeToolNotFound)
	assert.Equal(t, ErrorCode("TOOL_UNAVAILABLE"), ErrCodeToolUnavailable)
	assert.Equal(t, ErrorCode("TOOL_DISABLED"), ErrCodeToolDisabled)
	assert.Equal(t, ErrorCode("SESSION_NOT_FOUND"), ErrCodeSessionNotFound)
	assert.Equal(t, ErrorCode("SESSION_UNAVAILABLE"), ErrCodeSessionUnavailable)
	assert.Equal(t, ErrorCode("POLICY_DENIED"), ErrCodePolicyDenied)
	assert.Equal(t, ErrorCode("QUOTA_EXCEEDED"), ErrCodeQuotaExceeded)
	assert.Equal(t, ErrorCode("RATE_LIMITED"), ErrCodeRateLimited)
	assert.Equal(t, ErrorCode("CONCURRENCY_LIMIT_REACHED"), ErrCodeConcurrencyLimitReached)
	assert.Equal(t, ErrorCode("CREDENTIAL_UNAVAILABLE"), ErrCodeCredentialUnavailable)
	assert.Equal(t, ErrorCode("INVOCATION_TIMEOUT"), ErrCodeInvocationTimeout)
	assert.Equal(t, ErrorCode("TRANSPORT_FAILURE"), ErrCodeTransportFailure)
	assert.Equal(t, ErrorCode("INVALID_REQUEST"), ErrCodeInvalidRequest)
	assert.Equal(t, ErrorCode("INTERNAL"), ErrCodeInternal)
}

func TestNewRuntimeError(t *testing.T) {
	err := NewRuntimeError(ErrCodeToolNotFound, "tool not found")
	require.NotNil(t, err)
	assert.Equal(t, ErrCodeToolNotFound, err.Code)
	assert.Equal(t, "tool not found", err.Message)
	assert.Nil(t, err.Cause)
}

func TestRuntimeErrorError(t *testing.T) {
	t.Run("without cause", func(t *testing.T) {
		err := NewRuntimeError(ErrCodePolicyDenied, "access denied")
		assert.Equal(t, "[POLICY_DENIED] access denied", err.Error())
	})
	t.Run("with cause", func(t *testing.T) {
		cause := fmt.Errorf("connection refused")
		err := WrapRuntimeError(ErrCodeTransportFailure, "transport disconnected", cause)
		assert.Equal(t, "[TRANSPORT_FAILURE] transport disconnected: connection refused", err.Error())
	})
}

func TestWrapRuntimeError(t *testing.T) {
	cause := fmt.Errorf("underlying network error")
	err := WrapRuntimeError(ErrCodeTransportFailure, "transport failed", cause)

	require.NotNil(t, err)
	assert.Equal(t, ErrCodeTransportFailure, err.Code)
	assert.Equal(t, "transport failed", err.Message)
	assert.Equal(t, cause, err.Cause)
}

func TestRuntimeErrorUnwrap(t *testing.T) {
	sentinel := fmt.Errorf("sentinel error")
	wrapped := WrapRuntimeError(ErrCodeInternal, "something went wrong", sentinel)

	assert.True(t, errors.Is(wrapped, sentinel), "errors.Is should find the sentinel through Unwrap")
}

func TestRuntimeErrorImplementsError(t *testing.T) {
	var err error = NewRuntimeError(ErrCodeInternal, "internal error")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "INTERNAL")
}

func TestErrToolNotFound(t *testing.T) {
	assert.Error(t, ErrToolNotFound)
	assert.True(t, errors.Is(ErrToolNotFound, ErrToolNotFound))
	var rErr *RuntimeError
	require.True(t, errors.As(ErrToolNotFound, &rErr))
	assert.Equal(t, ErrCodeToolNotFound, rErr.Code)
}
