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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tochemey/goakt-mcp/mcp"
)

func TestResolveResult_Resolved(t *testing.T) {
	t.Run("returns true when credentials resolved successfully", func(t *testing.T) {
		r := &ResolveResult{
			Credentials: map[string]string{"key": "value"},
			Err:         nil,
		}
		assert.True(t, r.Resolved())
	})

	t.Run("returns false when error is set", func(t *testing.T) {
		r := &ResolveResult{
			Credentials: map[string]string{"key": "value"},
			Err:         errors.New("resolution failed"),
		}
		assert.False(t, r.Resolved())
	})

	t.Run("returns false when credentials map is empty", func(t *testing.T) {
		r := &ResolveResult{
			Credentials: nil,
			Err:         nil,
		}
		assert.False(t, r.Resolved())
	})

	t.Run("returns false when credentials map is empty with nil err", func(t *testing.T) {
		r := &ResolveResult{
			Credentials: map[string]string{},
			Err:         nil,
		}
		assert.False(t, r.Resolved())
	})
}

func TestResolveRequest(t *testing.T) {
	req := ResolveRequest{
		TenantID: mcp.TenantID("tenant-1"),
		ToolID:   mcp.ToolID("tool-1"),
	}
	assert.Equal(t, mcp.TenantID("tenant-1"), req.TenantID)
	assert.Equal(t, mcp.ToolID("tool-1"), req.ToolID)
}
