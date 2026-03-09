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

package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tochemey/goakt-mcp/mcp"
)

func TestRecordFunctions_NoPanicWhenUnregistered(t *testing.T) {
	ctx := context.Background()
	UnregisterMetrics()

	// Record* should be no-ops when metrics are not registered
	assert.NotPanics(t, func() { RecordToolAvailability(ctx, mcp.ToolID("tool1"), true) })
	assert.NotPanics(t, func() { RecordInvocationLatency(ctx, mcp.ToolID("tool1"), mcp.TenantID("tenant1"), 42.5) })
	assert.NotPanics(t, func() { RecordInvocationFailure(ctx, mcp.ToolID("tool1"), mcp.TenantID("tenant1"), "timeout") })
	assert.NotPanics(t, func() { RecordCircuitState(ctx, mcp.ToolID("tool1"), "open") })
}

func TestRegisterMetrics(t *testing.T) {
	t.Run("creates instruments from meter", func(t *testing.T) {
		m, err := RegisterMetrics(nil)
		require.NoError(t, err)
		require.NotNil(t, m)
		t.Cleanup(UnregisterMetrics)
	})
}
