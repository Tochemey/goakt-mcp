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

package mcp

import "context"

// ToolExecutor executes MCP tool invocations over a specific transport (stdio, HTTP).
//
// Implementations are provided by the egress layer. Execute must respect the
// invocation's context for cancellation and timeout. Close releases transport
// resources and is safe to call multiple times.
type ToolExecutor interface {
	Execute(ctx context.Context, inv *Invocation) (*ExecutionResult, error)
	Close() error
}

// ExecutorFactory creates ToolExecutor instances for a given tool.
//
// The factory is invoked when a session is created. The returned executor
// is bound to that session's lifecycle. When nil is returned, the session
// uses stub execution (returns success with empty output).
type ExecutorFactory interface {
	Create(ctx context.Context, tool Tool, credentials map[string]string) (ToolExecutor, error)
}
