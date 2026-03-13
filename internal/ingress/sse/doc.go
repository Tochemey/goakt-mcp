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

// Package sse implements the MCP SSE ingress handler for goakt-mcp.
//
// It wraps the MCP go-sdk [sdkmcp.SSEHandler] and routes incoming tool-call
// requests through the Gateway routing layer. This transport follows the
// 2024-11-05 version of the MCP specification where sessions are initiated
// via a hanging GET request that streams server-to-client messages as
// Server-Sent Events.
//
// # Session lifecycle
//
// The client opens a GET request which initiates an SSE stream. The first
// event is an "endpoint" event containing the session endpoint URL. The
// client then POSTs JSON-RPC messages to that endpoint. Responses are
// delivered as SSE "message" events on the original GET stream.
//
// # Identity resolution
//
// Identity (tenantID + clientID) is resolved once per MCP session via the
// configured [mcp.IdentityResolver] and captured in per-tool handler
// closures, so no allocation occurs on the hot per-request path.
package sse
