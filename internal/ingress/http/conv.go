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

package http

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tochemey/goakt-mcp/internal/ingress/pkg"
	"github.com/tochemey/goakt-mcp/mcp"
)

// The functions below delegate to [shared] so that internal tests in this
// package (conv_test.go) continue to compile and pass without changes to
// their call sites. New code should import [shared] directly.

func dispatchToolCall(
	ctx context.Context,
	gw pkg.Invoker,
	req *sdkmcp.CallToolRequest,
	toolID mcp.ToolID,
	tenantID mcp.TenantID,
	clientID mcp.ClientID,
) (*sdkmcp.CallToolResult, error) {
	return pkg.DispatchToolCall(ctx, gw, req, toolID, tenantID, clientID)
}

func requestToInvocation(
	req *sdkmcp.CallToolRequest,
	toolID mcp.ToolID,
	tenantID mcp.TenantID,
	clientID mcp.ClientID,
) (*mcp.Invocation, error) {
	return pkg.RequestToInvocation(req, toolID, tenantID, clientID)
}

func executionResultToCallToolResult(res *mcp.ExecutionResult, gwErr error) *sdkmcp.CallToolResult {
	return pkg.ExecutionResultToCallToolResult(res, gwErr)
}

func outputToCallToolResult(output map[string]any) *sdkmcp.CallToolResult {
	return pkg.OutputToCallToolResult(output)
}

func newRequestID() mcp.RequestID {
	return pkg.NewRequestID()
}
