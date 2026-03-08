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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tochemey/goakt-mcp/mcp"
)

func TestValidateTool(t *testing.T) {
	tests := []struct {
		name    string
		tool    mcp.Tool
		wantErr bool
		code    mcp.ErrorCode
	}{
		{
			name: "valid stdio tool",
			tool: mcp.Tool{
				ID:        mcp.ToolID("my-tool"),
				Transport: mcp.TransportStdio,
				Stdio:     &mcp.StdioTransportConfig{Command: "npx"},
			},
			wantErr: false,
		},
		{
			name: "valid http tool",
			tool: mcp.Tool{
				ID:        mcp.ToolID("http-tool"),
				Transport: mcp.TransportHTTP,
				HTTP:      &mcp.HTTPTransportConfig{URL: "http://localhost:8080"},
			},
			wantErr: false,
		},
		{
			name: "empty tool ID",
			tool: mcp.Tool{
				ID:        "",
				Transport: mcp.TransportStdio,
				Stdio:     &mcp.StdioTransportConfig{Command: "npx"},
			},
			wantErr: true,
			code:    mcp.ErrCodeInvalidRequest,
		},
		{
			name: "stdio without command",
			tool: mcp.Tool{
				ID:        mcp.ToolID("bad-stdio"),
				Transport: mcp.TransportStdio,
				Stdio:     &mcp.StdioTransportConfig{Command: ""},
			},
			wantErr: true,
			code:    mcp.ErrCodeInvalidRequest,
		},
		{
			name: "stdio with nil config",
			tool: mcp.Tool{
				ID:        mcp.ToolID("bad-stdio"),
				Transport: mcp.TransportStdio,
				Stdio:     nil,
			},
			wantErr: true,
			code:    mcp.ErrCodeInvalidRequest,
		},
		{
			name: "http without URL",
			tool: mcp.Tool{
				ID:        mcp.ToolID("bad-http"),
				Transport: mcp.TransportHTTP,
				HTTP:      &mcp.HTTPTransportConfig{URL: ""},
			},
			wantErr: true,
			code:    mcp.ErrCodeInvalidRequest,
		},
		{
			name: "http with nil config",
			tool: mcp.Tool{
				ID:        mcp.ToolID("bad-http"),
				Transport: mcp.TransportHTTP,
				HTTP:      nil,
			},
			wantErr: true,
			code:    mcp.ErrCodeInvalidRequest,
		},
		{
			name: "invalid transport",
			tool: mcp.Tool{
				ID:        mcp.ToolID("bad-transport"),
				Transport: mcp.TransportType("grpc"),
				Stdio:     &mcp.StdioTransportConfig{Command: "npx"},
			},
			wantErr: true,
			code:    mcp.ErrCodeInvalidRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mcp.ValidateTool(tt.tool)
			if tt.wantErr {
				require.Error(t, err)
				var rErr *mcp.RuntimeError
				require.True(t, errors.As(err, &rErr), "expected RuntimeError")
				assert.Equal(t, tt.code, rErr.Code)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
