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

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamingResult_Collect(t *testing.T) {
	t.Run("collects final result after draining progress", func(t *testing.T) {
		progressCh := make(chan ProgressEvent, 3)
		finalCh := make(chan *ExecutionResult, 1)

		progressCh <- ProgressEvent{Progress: 0.25, Total: 1.0, Message: "step 1"}
		progressCh <- ProgressEvent{Progress: 0.50, Total: 1.0, Message: "step 2"}
		progressCh <- ProgressEvent{Progress: 1.00, Total: 1.0, Message: "done"}
		close(progressCh)

		expected := &ExecutionResult{
			Status: ExecutionStatusSuccess,
			Output: map[string]any{"result": "ok"},
		}
		finalCh <- expected
		close(finalCh)

		sr := &StreamingResult{
			Progress: progressCh,
			Final:    finalCh,
		}

		result := sr.Collect()
		require.NotNil(t, result)
		assert.Equal(t, ExecutionStatusSuccess, result.Status)
		assert.Equal(t, "ok", result.Output["result"])
	})

	t.Run("collects result with no progress events", func(t *testing.T) {
		progressCh := make(chan ProgressEvent)
		finalCh := make(chan *ExecutionResult, 1)
		close(progressCh)

		expected := &ExecutionResult{
			Status:   ExecutionStatusFailure,
			Err:      NewRuntimeError(ErrCodeInternal, "failed"),
			Duration: time.Second,
		}
		finalCh <- expected
		close(finalCh)

		sr := &StreamingResult{
			Progress: progressCh,
			Final:    finalCh,
		}

		result := sr.Collect()
		require.NotNil(t, result)
		assert.Equal(t, ExecutionStatusFailure, result.Status)
	})

	t.Run("returns nil when final channel is closed without value", func(t *testing.T) {
		progressCh := make(chan ProgressEvent)
		finalCh := make(chan *ExecutionResult)
		close(progressCh)
		close(finalCh)

		sr := &StreamingResult{
			Progress: progressCh,
			Final:    finalCh,
		}

		result := sr.Collect()
		assert.Nil(t, result)
	})
}

func TestProgressEvent_Fields(t *testing.T) {
	evt := ProgressEvent{
		ProgressToken: "token-123",
		Progress:      0.75,
		Total:         1.0,
		Message:       "processing",
	}
	assert.Equal(t, "token-123", evt.ProgressToken)
	assert.InDelta(t, 0.75, evt.Progress, 0.001)
	assert.InDelta(t, 1.0, evt.Total, 0.001)
	assert.Equal(t, "processing", evt.Message)
}
