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

package stdio

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tochemey/goakt-mcp/mcp"
)

func TestNewStdioExecutor_Validation(t *testing.T) {
	t.Run("nil config returns error", func(t *testing.T) {
		exec, err := NewStdioExecutor(nil, time.Second)
		assert.Nil(t, exec)
		require.Error(t, err)
		var rErr *mcp.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, mcp.ErrCodeInvalidRequest, rErr.Code)
	})

	t.Run("empty command returns error", func(t *testing.T) {
		cfg := &mcp.StdioTransportConfig{Command: ""}
		exec, err := NewStdioExecutor(cfg, time.Second)
		assert.Nil(t, exec)
		require.Error(t, err)
		var rErr *mcp.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, mcp.ErrCodeInvalidRequest, rErr.Code)
	})

	t.Run("non-existent command returns transport failure", func(t *testing.T) {
		cfg := &mcp.StdioTransportConfig{Command: "/nonexistent/binary/xyz"}
		exec, err := NewStdioExecutor(cfg, 500*time.Millisecond)
		assert.Nil(t, exec)
		require.Error(t, err)
		var rErr *mcp.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, mcp.ErrCodeTransportFailure, rErr.Code)
	})
}

func TestStdioExecutor_Execute_NilSession(t *testing.T) {
	e := &StdioExecutor{}
	inv := &mcp.Invocation{
		ToolID: "test",
		Correlation: mcp.CorrelationMeta{
			RequestID: "req-1",
		},
	}
	result, err := e.Execute(context.Background(), inv)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, mcp.ExecutionStatusFailure, result.Status)
	assert.Equal(t, mcp.ErrCodeTransportFailure, result.Err.Code)
}

func TestStdioExecutor_Close_Idempotent(t *testing.T) {
	e := &StdioExecutor{}
	require.NoError(t, e.Close())
	require.NoError(t, e.Close())
}

func TestStdioExecutorFactory_NonStdioTool(t *testing.T) {
	factory := NewStdioExecutorFactory(time.Second)
	tool := mcp.Tool{
		ID:        "http-tool",
		Transport: mcp.TransportHTTP,
		HTTP:      &mcp.HTTPTransportConfig{URL: "http://localhost"},
	}
	exec, err := factory.Create(context.Background(), tool, nil)
	require.NoError(t, err)
	assert.Nil(t, exec)
}

func TestStdioExecutorFactory_NilStdioConfig(t *testing.T) {
	factory := NewStdioExecutorFactory(time.Second)
	tool := mcp.Tool{
		ID:        "stdio-tool",
		Transport: mcp.TransportStdio,
		Stdio:     nil,
	}
	exec, err := factory.Create(context.Background(), tool, nil)
	require.NoError(t, err)
	assert.Nil(t, exec)
}

func TestStdioExecutorFactory_DefaultTimeout(t *testing.T) {
	factory := NewStdioExecutorFactory(0)
	assert.NotNil(t, factory)
}

func TestEnvSlice(t *testing.T) {
	t.Run("nil extra returns base", func(t *testing.T) {
		result := envSlice(nil)
		assert.NotEmpty(t, result)
	})

	t.Run("empty extra returns base", func(t *testing.T) {
		result := envSlice(map[string]string{})
		assert.NotEmpty(t, result)
	})

	t.Run("overrides existing variable", func(t *testing.T) {
		extra := map[string]string{"PATH": "/custom/path"}
		result := envSlice(extra)
		found := false
		for _, e := range result {
			if e == "PATH=/custom/path" {
				found = true
				break
			}
		}
		assert.True(t, found, "PATH should be overridden")
	})

	t.Run("adds new variable", func(t *testing.T) {
		extra := map[string]string{"MY_CUSTOM_VAR_XYZ_TEST": "value123"}
		result := envSlice(extra)
		found := false
		for _, e := range result {
			if e == "MY_CUSTOM_VAR_XYZ_TEST=value123" {
				found = true
				break
			}
		}
		assert.True(t, found, "new variable should be added")
	})
}
