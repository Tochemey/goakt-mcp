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

package shared

import (
	"context"
	"encoding/json"
	"net/http"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tochemey/goakt-mcp/mcp"
)

// OpenObjectSchema is the fallback JSON Schema that permits any JSON object.
// Used when no backend schema is available for a tool.
var OpenObjectSchema = json.RawMessage(`{"type":"object"}`)

// BuildGetServer returns the per-session server factory consumed by SDK handler
// constructors ([sdkmcp.NewStreamableHTTPHandler], [sdkmcp.NewSSEHandler]).
//
// The returned function is called once per new MCP session. Returning nil
// causes the SDK to send HTTP 400 Bad Request to the client, which is the
// correct response when identity resolution fails (malformed credentials,
// unknown tenant, etc.).
//
// Tool registration is performed eagerly at session creation. If ListTools
// fails, the session is rejected (returns nil) so the client cannot invoke
// tools against an incomplete server instance.
func BuildGetServer(gw Invoker, resolver mcp.IdentityResolver) func(*http.Request) *sdkmcp.Server {
	return func(r *http.Request) *sdkmcp.Server {
		tenantID, clientID, err := resolver.ResolveIdentity(r)
		if err != nil {
			// nil causes the SDK to return 400 Bad Request.
			return nil
		}

		tools, err := gw.ListTools(r.Context())
		if err != nil {
			// Reject the session; the client should retry.
			return nil
		}

		srv := sdkmcp.NewServer(
			&sdkmcp.Implementation{Name: "goakt-mcp"},
			nil,
		)

		for _, t := range tools {
			RegisterTool(srv, gw, t, tenantID, clientID)
		}

		return srv
	}
}

// RegisterTool adds tool capabilities to srv with handler closures that capture
// the resolved session identity.
//
// When the tool has cached backend schemas (from tools/list at registration),
// each schema is registered as a separate SDK tool with its actual name,
// description, and input schema. When no schemas are available, a single
// pass-through registration with [OpenObjectSchema] is used as a fallback.
//
// The handler is deliberately kept allocation-free on the hot path: the
// tenantID and clientID values are captured by value in the closure.
func RegisterTool(srv *sdkmcp.Server, gw Invoker, tool mcp.Tool, tenantID mcp.TenantID, clientID mcp.ClientID) {
	toolID := tool.ID
	if len(tool.Schemas) == 0 {
		sdkTool := &sdkmcp.Tool{
			Name:        string(tool.ID),
			InputSchema: OpenObjectSchema,
		}
		srv.AddTool(sdkTool, func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			return DispatchToolCall(ctx, gw, req, toolID, tenantID, clientID)
		})
		return
	}

	for _, schema := range tool.Schemas {
		inputSchema := schema.InputSchema
		if inputSchema == nil {
			inputSchema = OpenObjectSchema
		}
		sdkTool := &sdkmcp.Tool{
			Name:        schema.Name,
			Description: schema.Description,
			InputSchema: inputSchema,
		}
		srv.AddTool(sdkTool, func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			return DispatchToolCall(ctx, gw, req, toolID, tenantID, clientID)
		})
	}
}
