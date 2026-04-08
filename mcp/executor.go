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

// ToolStreamExecutor is an optional extension of ToolExecutor that supports
// streaming progress notifications from the backend during execution.
//
// Executors may implement this interface alongside ToolExecutor. The session
// actor checks for this interface at runtime via type assertion. When present,
// ExecuteStream is called instead of Execute, enabling progress forwarding
// to the client.
//
// Executors that do not support streaming need not implement this interface;
// the session falls back to the standard Execute path.
type ToolStreamExecutor interface {
	ToolExecutor
	ExecuteStream(ctx context.Context, inv *Invocation) (*StreamingResult, error)
}

// ResourceExecutor is an optional extension of ToolExecutor that supports
// reading MCP resources from backend servers.
//
// Executors may implement this interface alongside ToolExecutor. The session
// actor checks for this interface at runtime via type assertion. When present,
// ReadResource is called for resources/read invocations.
//
// Executors that do not support resources need not implement this interface;
// the session returns an error for resources/read requests.
type ResourceExecutor interface {
	ReadResource(ctx context.Context, inv *Invocation) (*ExecutionResult, error)
}

// ResourceFetcher connects to a backend MCP server and retrieves the resources
// and resource templates it exposes. Implementations create a temporary connection,
// call resources/list and resources/templates/list, and close the connection.
// The returned metadata is cached by the registrar.
type ResourceFetcher interface {
	FetchResources(ctx context.Context, tool Tool) ([]ResourceSchema, []ResourceTemplateSchema, error)
}

// ExecutorFactory creates ToolExecutor instances for a given tool.
//
// The factory is invoked when a session is created. The returned executor
// is bound to that session's lifecycle. When nil is returned, the session
// uses stub execution (returns success with empty output).
type ExecutorFactory interface {
	Create(ctx context.Context, tool Tool, credentials map[string]string) (ToolExecutor, error)
}
