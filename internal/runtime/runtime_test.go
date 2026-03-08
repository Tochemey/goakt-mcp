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

	"github.com/stretchr/testify/require"

	"github.com/tochemey/goakt-mcp/mcp"
)

// TestRuntimePackageContracts verifies that the mcp package exports the
// expected top-level symbols and that their zero values are coherent.
func TestRuntimePackageContracts(t *testing.T) {
	// identity types are string-based; zero values must be empty
	require.True(t, mcp.TenantID("").IsZero())
	require.True(t, mcp.ClientID("").IsZero())
	require.True(t, mcp.ToolID("").IsZero())
	require.True(t, mcp.SessionID("").IsZero())
	require.True(t, mcp.RequestID("").IsZero())
	require.True(t, mcp.TraceID("").IsZero())

	// a zero CorrelationMeta is fully empty
	require.True(t, mcp.CorrelationMeta{}.IsZero())

	// a zero Tool is not available (zero ToolState is not enabled or degraded)
	require.False(t, mcp.Tool{}.IsAvailable())
}
