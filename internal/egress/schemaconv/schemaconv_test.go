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

package schemaconv

import (
	"encoding/json"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSDKToolsToSchemas(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		assert.Nil(t, SDKToolsToSchemas(nil))
	})

	t.Run("empty input returns nil", func(t *testing.T) {
		assert.Nil(t, SDKToolsToSchemas([]*sdkmcp.Tool{}))
	})

	t.Run("nil tool entries are skipped", func(t *testing.T) {
		tools := []*sdkmcp.Tool{nil, {Name: "valid"}, nil}
		schemas := SDKToolsToSchemas(tools)
		require.Len(t, schemas, 1)
		assert.Equal(t, "valid", schemas[0].Name)
	})

	t.Run("converts tools with all fields", func(t *testing.T) {
		inputSchema := json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`)
		tools := []*sdkmcp.Tool{
			{Name: "read_file", Description: "Read a file", InputSchema: inputSchema},
			{Name: "write_file", Description: "Write a file"},
		}
		schemas := SDKToolsToSchemas(tools)
		require.Len(t, schemas, 2)
		assert.Equal(t, "read_file", schemas[0].Name)
		assert.Equal(t, "Read a file", schemas[0].Description)
		assert.NotNil(t, schemas[0].InputSchema)
		assert.Equal(t, "write_file", schemas[1].Name)
		assert.Nil(t, schemas[1].InputSchema)
	})
}

func TestMarshalInputSchema(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		assert.Nil(t, MarshalInputSchema(nil))
	})

	t.Run("json.RawMessage passthrough", func(t *testing.T) {
		raw := json.RawMessage(`{"type":"object"}`)
		result := MarshalInputSchema(raw)
		assert.Equal(t, raw, result)
	})

	t.Run("marshals arbitrary value", func(t *testing.T) {
		v := map[string]any{"type": "string"}
		result := MarshalInputSchema(v)
		require.NotNil(t, result)
		assert.Contains(t, string(result), `"type"`)
	})

	t.Run("unmarshalable value returns nil", func(t *testing.T) {
		// channels cannot be marshaled to JSON
		ch := make(chan int)
		result := MarshalInputSchema(ch)
		assert.Nil(t, result)
	})
}
