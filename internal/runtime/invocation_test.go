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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCorrelationMetaIsZero(t *testing.T) {
	t.Run("zero value returns true", func(t *testing.T) {
		var m CorrelationMeta
		assert.True(t, m.IsZero())
	})
	t.Run("partially filled returns false", func(t *testing.T) {
		m := CorrelationMeta{TenantID: "acme"}
		assert.False(t, m.IsZero())
	})
	t.Run("fully filled returns false", func(t *testing.T) {
		m := CorrelationMeta{
			TenantID:  "acme",
			ClientID:  "user-1",
			RequestID: "req-001",
			TraceID:   "trace-001",
		}
		assert.False(t, m.IsZero())
	})
}

func TestExecutionStatusConstants(t *testing.T) {
	assert.Equal(t, ExecutionStatus("success"), ExecutionStatusSuccess)
	assert.Equal(t, ExecutionStatus("failure"), ExecutionStatusFailure)
	assert.Equal(t, ExecutionStatus("timeout"), ExecutionStatusTimeout)
	assert.Equal(t, ExecutionStatus("denied"), ExecutionStatusDenied)
	assert.Equal(t, ExecutionStatus("throttled"), ExecutionStatusThrottled)
}

func TestExecutionResultStatusHelpers(t *testing.T) {
	tests := []struct {
		status    ExecutionStatus
		succeeded bool
		failed    bool
		timedOut  bool
		denied    bool
		throttled bool
	}{
		{ExecutionStatusSuccess, true, false, false, false, false},
		{ExecutionStatusFailure, false, true, false, false, false},
		{ExecutionStatusTimeout, false, false, true, false, false},
		{ExecutionStatusDenied, false, false, false, true, false},
		{ExecutionStatusThrottled, false, false, false, false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			r := ExecutionResult{Status: tt.status}
			assert.Equal(t, tt.succeeded, r.Succeeded())
			assert.Equal(t, tt.failed, r.Failed())
			assert.Equal(t, tt.timedOut, r.TimedOut())
			assert.Equal(t, tt.denied, r.Denied())
			assert.Equal(t, tt.throttled, r.Throttled())
		})
	}
}

func TestInvocationConstruction(t *testing.T) {
	now := time.Now()
	corr := CorrelationMeta{
		TenantID:  "acme-dev",
		ClientID:  "cursor-user-1",
		RequestID: "req-01JXYZ",
		TraceID:   "trace-01JXYZ",
	}

	inv := Invocation{
		Correlation: corr,
		ToolID:      "filesystem",
		SessionID:   "sess-abc",
		Method:      "tools/call",
		Params: map[string]any{
			"name":      "search_docs",
			"arguments": map[string]any{"query": "actor supervision"},
		},
		Metadata:   map[string]string{"source": "cursor"},
		ReceivedAt: now,
	}

	require.Equal(t, corr, inv.Correlation)
	assert.Equal(t, ToolID("filesystem"), inv.ToolID)
	assert.Equal(t, SessionID("sess-abc"), inv.SessionID)
	assert.Equal(t, "tools/call", inv.Method)
	assert.NotEmpty(t, inv.Params)
	assert.Equal(t, "cursor", inv.Metadata["source"])
	assert.Equal(t, now, inv.ReceivedAt)
}

func TestExecutionResultConstruction(t *testing.T) {
	corr := CorrelationMeta{
		TenantID:  "acme-dev",
		ClientID:  "cursor-user-1",
		RequestID: "req-01JXYZ",
		TraceID:   "trace-01JXYZ",
	}

	t.Run("successful result", func(t *testing.T) {
		result := ExecutionResult{
			Status:      ExecutionStatusSuccess,
			Output:      map[string]any{"content": "hello"},
			Duration:    50 * time.Millisecond,
			Correlation: corr,
		}
		assert.True(t, result.Succeeded())
		assert.Nil(t, result.Err)
		assert.Equal(t, corr, result.Correlation)
	})

	t.Run("failed result carries error", func(t *testing.T) {
		runtimeErr := NewRuntimeError(ErrCodeTransportFailure, "transport disconnected")
		result := ExecutionResult{
			Status:      ExecutionStatusFailure,
			Err:         runtimeErr,
			Duration:    200 * time.Millisecond,
			Correlation: corr,
		}
		assert.True(t, result.Failed())
		require.NotNil(t, result.Err)
		assert.Equal(t, ErrCodeTransportFailure, result.Err.Code)
	})
}
