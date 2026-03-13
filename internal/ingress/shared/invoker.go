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

	"github.com/tochemey/goakt-mcp/mcp"
)

// Invoker is the minimal Gateway interface required by all ingress handlers.
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
