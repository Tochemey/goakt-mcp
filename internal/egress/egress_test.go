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

package egress

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tochemey/goakt-mcp/internal/runtime"
)

func TestCompositeExecutorFactory_Create(t *testing.T) {
	factory := NewCompositeExecutorFactory(time.Second, nil)
	require.NotNil(t, factory)

	t.Run("unknown transport returns nil executor", func(t *testing.T) {
		tool := runtime.Tool{
			ID:        "unknown-tool",
			Transport: "grpc",
		}
		exec, err := factory.Create(context.Background(), tool, nil)
		require.NoError(t, err)
		assert.Nil(t, exec)
	})

	t.Run("empty transport returns nil executor", func(t *testing.T) {
		tool := runtime.Tool{ID: "empty-tool"}
		exec, err := factory.Create(context.Background(), tool, nil)
		require.NoError(t, err)
		assert.Nil(t, exec)
	})

	t.Run("stdio tool without config returns nil", func(t *testing.T) {
		tool := runtime.Tool{
			ID:        "stdio-no-cfg",
			Transport: runtime.TransportStdio,
			Stdio:     nil,
		}
		exec, err := factory.Create(context.Background(), tool, nil)
		require.NoError(t, err)
		assert.Nil(t, exec)
	})

	t.Run("http tool without config returns nil", func(t *testing.T) {
		tool := runtime.Tool{
			ID:        "http-no-cfg",
			Transport: runtime.TransportHTTP,
			HTTP:      nil,
		}
		exec, err := factory.Create(context.Background(), tool, nil)
		require.NoError(t, err)
		assert.Nil(t, exec)
	})

	t.Run("stdio tool with invalid command returns error", func(t *testing.T) {
		tool := runtime.Tool{
			ID:        "stdio-bad",
			Transport: runtime.TransportStdio,
			Stdio:     &runtime.StdioTransportConfig{Command: "/nonexistent/binary/xyz"},
		}
		exec, err := factory.Create(context.Background(), tool, nil)
		assert.Nil(t, exec)
		require.Error(t, err)
		var rErr *runtime.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, runtime.ErrCodeTransportFailure, rErr.Code)
	})

	t.Run("http tool with unreachable URL returns error", func(t *testing.T) {
		tool := runtime.Tool{
			ID:        "http-bad",
			Transport: runtime.TransportHTTP,
			HTTP:      &runtime.HTTPTransportConfig{URL: "http://127.0.0.1:1/unreachable"},
		}
		exec, err := factory.Create(context.Background(), tool, nil)
		assert.Nil(t, exec)
		require.Error(t, err)
		var rErr *runtime.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, runtime.ErrCodeTransportFailure, rErr.Code)
	})
}

func TestNewCompositeExecutorFactory_DefaultTimeout(t *testing.T) {
	factory := NewCompositeExecutorFactory(0, nil)
	require.NotNil(t, factory)
}

func TestCompositeExecutorFactory_ImplementsInterface(t *testing.T) {
	var _ runtime.ExecutorFactory = (*CompositeExecutorFactory)(nil)
}
