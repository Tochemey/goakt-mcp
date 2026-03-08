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

import "github.com/tochemey/goakt-mcp/mcp"

// Routing command and response types for RouterActor.
//
// The router is the runtime entry point for tool invocations. It performs
// tool lookup, supervisor availability check, session resolution, and
// execution. Routing decisions are deterministic and tenant-aware.

// RouteInvocation is a request to route and execute an invocation.
//
// The router looks up the tool, checks supervisor availability, resolves or
// creates a session, and forwards the invocation. The response is RouteResult.
// Must be used with Ask.
type RouteInvocation struct {
	Invocation *mcp.Invocation
}

// RouteResult is the response to RouteInvocation.
//
// On success, Result holds the execution outcome. On failure, Err describes
// the routing or execution failure (tool not found, circuit open, etc.).
type RouteResult struct {
	Result *mcp.ExecutionResult
	Err    error
}
