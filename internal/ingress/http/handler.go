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
	"encoding/json"
	"errors"
	"net/http"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tochemey/goakt-mcp/mcp"
)

// Invoker is the minimal Gateway interface required by the ingress handler.
//
// Using a narrow interface rather than coupling directly to *Gateway keeps the
// ingress layer independently testable and prevents import cycles.
type Invoker interface {
	// Invoke executes a single tool-call invocation and returns its result.
	// A non-nil error from Invoke is treated as a tool error, not a protocol error.
	Invoke(ctx context.Context, inv *mcp.Invocation) (*mcp.ExecutionResult, error)

	// ListTools returns the current set of registered tools.
	// It is called once per new MCP session to populate the session's tool list.
	ListTools(ctx context.Context) ([]mcp.Tool, error)
}

// openObjectSchema is the fallback JSON Schema that permits any JSON object.
// Used when no backend schema is available for a tool.
var openObjectSchema = json.RawMessage(`{"type":"object"}`)

// New returns an [http.Handler] that serves MCP Streamable HTTP sessions and
// routes each tool call through gw.
//
// Identity (tenantID + clientID) is resolved once per new MCP session via
// cfg.IdentityResolver. On resolution failure, getServer returns nil which
// causes the SDK to reply with HTTP 400 Bad Request. The resolved identity is
// captured in per-tool handler closures, so no allocation occurs on the
// per-request hot path.
//
// New validates that cfg.IdentityResolver is non-nil and returns an error
// if it is not. The gateway does not need to be started at the time New is
// called; tool registration happens lazily on first session creation.
func New(gw Invoker, cfg mcp.IngressConfig) (http.Handler, error) {
	if cfg.IdentityResolver == nil {
		return nil, errors.New("ingress: IdentityResolver must not be nil")
	}

	getServer := buildGetServer(gw, cfg.IdentityResolver)

	opts := &sdkmcp.StreamableHTTPOptions{
		Stateless:      cfg.Stateless,
		SessionTimeout: cfg.SessionIdleTimeout,
	}

	return sdkmcp.NewStreamableHTTPHandler(getServer, opts), nil
}

// buildGetServer returns the per-session server factory consumed by
// [sdkmcp.NewStreamableHTTPHandler].
//
// The returned function is called once per new MCP session. Returning nil
// causes the SDK to send HTTP 400 Bad Request to the client, which is the
// correct response when identity resolution fails (malformed credentials,
// unknown tenant, etc.).
//
// Tool registration is performed eagerly at session creation. If ListTools
// fails, the session is rejected (returns nil) so the client cannot invoke
// tools against an incomplete server instance. This is a deliberate
// correctness trade-off: a partial tool list could cause confusing LLM
// behaviour, and the client can simply retry to open a new session.
func buildGetServer(gw Invoker, resolver mcp.IdentityResolver) func(*http.Request) *sdkmcp.Server {
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
			registerTool(srv, gw, t, tenantID, clientID)
		}

		return srv
	}
}

// registerTool adds tool capabilities to srv with handler closures that capture
// the resolved session identity.
//
// When the tool has cached backend schemas (from tools/list at registration),
// each schema is registered as a separate SDK tool with its actual name,
// description, and input schema. When no schemas are available, a single
// pass-through registration with [openObjectSchema] is used as a fallback.
//
// The handler is deliberately kept allocation-free on the hot path: the
// tenantID and clientID values are captured by value in the closure.
func registerTool(
	srv *sdkmcp.Server,
	gw Invoker,
	t mcp.Tool,
	tenantID mcp.TenantID,
	clientID mcp.ClientID,
) {
	toolID := t.ID
	if len(t.Schemas) == 0 {
		sdkTool := &sdkmcp.Tool{
			Name:        string(t.ID),
			InputSchema: openObjectSchema,
		}
		srv.AddTool(sdkTool, func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			return dispatchToolCall(ctx, gw, req, toolID, tenantID, clientID)
		})
		return
	}

	for _, schema := range t.Schemas {
		inputSchema := schema.InputSchema
		if inputSchema == nil {
			inputSchema = openObjectSchema
		}
		sdkTool := &sdkmcp.Tool{
			Name:        schema.Name,
			Description: schema.Description,
			InputSchema: inputSchema,
		}
		srv.AddTool(sdkTool, func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			return dispatchToolCall(ctx, gw, req, toolID, tenantID, clientID)
		})
	}
}
