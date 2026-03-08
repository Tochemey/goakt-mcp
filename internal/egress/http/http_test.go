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

package http

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tochemey/goakt-mcp/internal/runtime"
)

func TestNewHTTPExecutor_Validation(t *testing.T) {
	t.Run("nil config returns error", func(t *testing.T) {
		exec, err := NewHTTPExecutor(nil, nil, time.Second)
		assert.Nil(t, exec)
		require.Error(t, err)
		var rErr *runtime.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, runtime.ErrCodeInvalidRequest, rErr.Code)
	})

	t.Run("empty URL returns error", func(t *testing.T) {
		cfg := &runtime.HTTPTransportConfig{URL: ""}
		exec, err := NewHTTPExecutor(cfg, nil, time.Second)
		assert.Nil(t, exec)
		require.Error(t, err)
		var rErr *runtime.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, runtime.ErrCodeInvalidRequest, rErr.Code)
	})

	t.Run("unreachable endpoint returns transport failure", func(t *testing.T) {
		cfg := &runtime.HTTPTransportConfig{URL: "http://127.0.0.1:1/unreachable"}
		exec, err := NewHTTPExecutor(cfg, nil, 500*time.Millisecond)
		assert.Nil(t, exec)
		require.Error(t, err)
		var rErr *runtime.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, runtime.ErrCodeTransportFailure, rErr.Code)
	})
}

func TestHTTPExecutor_Execute_NilSession(t *testing.T) {
	e := &HTTPExecutor{}
	inv := &runtime.Invocation{
		ToolID: "test",
		Correlation: runtime.CorrelationMeta{
			RequestID: "req-1",
		},
	}
	result, err := e.Execute(context.Background(), inv)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, runtime.ExecutionStatusFailure, result.Status)
	assert.Equal(t, runtime.ErrCodeTransportFailure, result.Err.Code)
}

func TestHTTPExecutor_Close_Idempotent(t *testing.T) {
	e := &HTTPExecutor{}
	require.NoError(t, e.Close())
	require.NoError(t, e.Close())
}

func TestHTTPExecutorFactory_NonHTTPTool(t *testing.T) {
	factory := NewHTTPExecutorFactory(nil, time.Second)
	tool := runtime.Tool{
		ID:        "stdio-tool",
		Transport: runtime.TransportStdio,
		Stdio:     &runtime.StdioTransportConfig{Command: "echo"},
	}
	exec, err := factory.Create(context.Background(), tool, nil)
	require.NoError(t, err)
	assert.Nil(t, exec)
}

func TestHTTPExecutorFactory_NilHTTPConfig(t *testing.T) {
	factory := NewHTTPExecutorFactory(nil, time.Second)
	tool := runtime.Tool{
		ID:        "http-tool",
		Transport: runtime.TransportHTTP,
		HTTP:      nil,
	}
	exec, err := factory.Create(context.Background(), tool, nil)
	require.NoError(t, err)
	assert.Nil(t, exec)
}

func TestHTTPExecutorFactory_DefaultTimeout(t *testing.T) {
	factory := NewHTTPExecutorFactory(nil, 0)
	assert.NotNil(t, factory)
}
