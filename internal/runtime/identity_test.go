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

	"github.com/stretchr/testify/assert"
)

func TestTenantID(t *testing.T) {
	t.Run("IsZero returns true for empty value", func(t *testing.T) {
		var id TenantID
		assert.True(t, id.IsZero())
	})
	t.Run("IsZero returns false for non-empty value", func(t *testing.T) {
		id := TenantID("acme")
		assert.False(t, id.IsZero())
	})
	t.Run("String returns the underlying string", func(t *testing.T) {
		id := TenantID("acme-dev")
		assert.Equal(t, "acme-dev", id.String())
	})
}

func TestClientID(t *testing.T) {
	t.Run("IsZero returns true for empty value", func(t *testing.T) {
		var id ClientID
		assert.True(t, id.IsZero())
	})
	t.Run("IsZero returns false for non-empty value", func(t *testing.T) {
		id := ClientID("client-app-1")
		assert.False(t, id.IsZero())
	})
	t.Run("String returns the underlying string", func(t *testing.T) {
		id := ClientID("client-app-1")
		assert.Equal(t, "client-app-1", id.String())
	})
}

func TestToolID(t *testing.T) {
	t.Run("IsZero returns true for empty value", func(t *testing.T) {
		var id ToolID
		assert.True(t, id.IsZero())
	})
	t.Run("IsZero returns false for non-empty value", func(t *testing.T) {
		id := ToolID("filesystem")
		assert.False(t, id.IsZero())
	})
	t.Run("String returns the underlying string", func(t *testing.T) {
		id := ToolID("filesystem")
		assert.Equal(t, "filesystem", id.String())
	})
}

func TestSessionID(t *testing.T) {
	t.Run("IsZero returns true for empty value", func(t *testing.T) {
		var id SessionID
		assert.True(t, id.IsZero())
	})
	t.Run("IsZero returns false for non-empty value", func(t *testing.T) {
		id := SessionID("sess-abc123")
		assert.False(t, id.IsZero())
	})
	t.Run("String returns the underlying string", func(t *testing.T) {
		id := SessionID("sess-abc123")
		assert.Equal(t, "sess-abc123", id.String())
	})
}

func TestRequestID(t *testing.T) {
	t.Run("IsZero returns true for empty value", func(t *testing.T) {
		var id RequestID
		assert.True(t, id.IsZero())
	})
	t.Run("IsZero returns false for non-empty value", func(t *testing.T) {
		id := RequestID("req-01JXYZ")
		assert.False(t, id.IsZero())
	})
	t.Run("String returns the underlying string", func(t *testing.T) {
		id := RequestID("req-01JXYZ")
		assert.Equal(t, "req-01JXYZ", id.String())
	})
}

func TestTraceID(t *testing.T) {
	t.Run("IsZero returns true for empty value", func(t *testing.T) {
		var id TraceID
		assert.True(t, id.IsZero())
	})
	t.Run("IsZero returns false for non-empty value", func(t *testing.T) {
		id := TraceID("trace-01JXYZ")
		assert.False(t, id.IsZero())
	})
	t.Run("String returns the underlying string", func(t *testing.T) {
		id := TraceID("trace-01JXYZ")
		assert.Equal(t, "trace-01JXYZ", id.String())
	})
}
