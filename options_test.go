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

package goaktmcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goaktlog "github.com/tochemey/goakt/v4/log"

	"github.com/tochemey/goakt-mcp/mcp"
)

func TestWithLogger(t *testing.T) {
	t.Run("InvalidLevel sets DiscardLogger", func(t *testing.T) {
		gw, err := New(mcp.Config{}, WithLogger(goaktlog.InvalidLevel))
		require.NoError(t, err)
		require.NotNil(t, gw)
		assert.Equal(t, goaktlog.DiscardLogger, gw.logger)
	})

	t.Run("DebugLevel sets slog logger", func(t *testing.T) {
		gw, err := New(mcp.Config{}, WithLogger(goaktlog.DebugLevel))
		require.NoError(t, err)
		require.NotNil(t, gw)
		assert.NotNil(t, gw.logger)
		assert.NotEqual(t, goaktlog.DiscardLogger, gw.logger)
	})

	t.Run("InfoLevel sets slog logger", func(t *testing.T) {
		gw, err := New(mcp.Config{}, WithLogger(goaktlog.InfoLevel))
		require.NoError(t, err)
		require.NotNil(t, gw)
		assert.NotNil(t, gw.logger)
	})
}

func TestWithMetrics(t *testing.T) {
	gw, err := New(mcp.Config{}, WithLogger(goaktlog.InvalidLevel), WithMetrics())
	require.NoError(t, err)
	require.NotNil(t, gw)
	assert.True(t, gw.metrics)
}

func TestWithTracing(t *testing.T) {
	gw, err := New(mcp.Config{}, WithLogger(goaktlog.InvalidLevel), WithTracing())
	require.NoError(t, err)
	require.NotNil(t, gw)
	assert.True(t, gw.tracing)
}

func TestOptions_Combined(t *testing.T) {
	gw, err := New(mcp.Config{}, WithLogger(goaktlog.InvalidLevel), WithMetrics(), WithTracing())
	require.NoError(t, err)
	require.NotNil(t, gw)
	assert.Equal(t, goaktlog.DiscardLogger, gw.logger)
	assert.True(t, gw.metrics)
	assert.True(t, gw.tracing)
}
