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

package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvokeRequest_Validate(t *testing.T) {
	valid := InvokeRequest{
		TenantID:  "acme",
		ClientID:  "client-1",
		RequestID: "req-1",
		TraceID:   "trace-1",
		Method:    "tools/call",
		Params:    map[string]any{"name": "test"},
	}
	require.NoError(t, valid.Validate())

	t.Run("rejects empty tenant_id", func(t *testing.T) {
		r := valid
		r.TenantID = ""
		require.Error(t, r.Validate())
	})
	t.Run("rejects empty client_id", func(t *testing.T) {
		r := valid
		r.ClientID = ""
		require.Error(t, r.Validate())
	})
	t.Run("rejects empty request_id", func(t *testing.T) {
		r := valid
		r.RequestID = ""
		require.Error(t, r.Validate())
	})
	t.Run("rejects empty trace_id", func(t *testing.T) {
		r := valid
		r.TraceID = ""
		require.Error(t, r.Validate())
	})
	t.Run("rejects empty method", func(t *testing.T) {
		r := valid
		r.Method = ""
		require.Error(t, r.Validate())
	})
	t.Run("rejects nil params", func(t *testing.T) {
		r := valid
		r.Params = nil
		err := r.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "params")
	})
}
