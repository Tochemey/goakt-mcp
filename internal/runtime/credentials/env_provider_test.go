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

package credentials

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tochemey/goakt-mcp/internal/runtime"
)

func TestEnvProvider(t *testing.T) {
	ctx := context.Background()
	p := NewEnvProvider()

	t.Run("returns nil when no matching env vars", func(t *testing.T) {
		creds, err := p.Resolve(ctx, runtime.TenantID("tenant"), runtime.ToolID("nonexistent-tool-xyz"))
		require.NoError(t, err)
		assert.Nil(t, creds)
	})

	t.Run("returns credentials from env", func(t *testing.T) {
		key := "MCP_CRED_TEST_TOOL_API_KEY"
		os.Setenv(key, "secret-value")
		defer os.Unsetenv(key)

		creds, err := p.Resolve(ctx, runtime.TenantID("tenant"), runtime.ToolID("test-tool"))
		require.NoError(t, err)
		require.NotNil(t, creds)
		assert.Equal(t, "secret-value", creds["api-key"])
	})

	t.Run("ID returns env", func(t *testing.T) {
		assert.Equal(t, "env", p.ID())
	})
}
