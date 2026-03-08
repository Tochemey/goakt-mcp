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
	"net/http"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tochemey/goakt-mcp/internal/egress/mcpconv"
	"github.com/tochemey/goakt-mcp/internal/runtime"
)

// HTTPExecutor executes MCP tool invocations over HTTP using the streamable transport.
type HTTPExecutor struct {
	client   *mcp.Client
	sess     *mcp.ClientSession
	closed   sync.Once
	closeErr error
}

// Execute runs the MCP tools/call invocation and returns the result.
func (e *HTTPExecutor) Execute(ctx context.Context, inv *runtime.Invocation) (*runtime.ExecutionResult, error) {
	if e.sess == nil {
		return &runtime.ExecutionResult{
			Status:      runtime.ExecutionStatusFailure,
			Err:         runtime.NewRuntimeError(runtime.ErrCodeTransportFailure, "session not connected"),
			Correlation: inv.Correlation,
		}, nil
	}

	name, args := mcpconv.ParamsFromInvocation(inv)
	params := &mcp.CallToolParams{Name: name, Arguments: args}

	res, err := e.sess.CallTool(ctx, params)
	if err != nil {
		if ctx.Err() != nil {
			return &runtime.ExecutionResult{
				Status:      runtime.ExecutionStatusTimeout,
				Err:         runtime.WrapRuntimeError(runtime.ErrCodeInvocationTimeout, "invocation timed out", err),
				Correlation: inv.Correlation,
			}, nil
		}
		return &runtime.ExecutionResult{
			Status:      runtime.ExecutionStatusFailure,
			Err:         runtime.WrapRuntimeError(runtime.ErrCodeTransportFailure, "call failed", err),
			Correlation: inv.Correlation,
		}, nil
	}

	output := mcpconv.CallResultToOutput(res)
	status := runtime.ExecutionStatusSuccess
	var rErr *runtime.RuntimeError
	if res.IsError {
		status = runtime.ExecutionStatusFailure
		rErr = runtime.NewRuntimeError(runtime.ErrCodeInternal, mcpconv.ContentErrorText(res))
	}

	return &runtime.ExecutionResult{
		Status:      status,
		Output:      output,
		Err:         rErr,
		Correlation: inv.Correlation,
	}, nil
}

// Close releases the HTTP session and connection.
func (e *HTTPExecutor) Close() error {
	e.closed.Do(func() {
		if e.sess != nil {
			e.closeErr = e.sess.Close()
			e.sess = nil
		}
		e.client = nil
	})
	return e.closeErr
}

// NewHTTPExecutor creates an executor by connecting to the configured HTTP endpoint.
// The httpClient should not set a global Timeout since request-level deadlines are
// managed by the session's context. When httpClient is nil a default client without
// a global timeout is used.
func NewHTTPExecutor(cfg *runtime.HTTPTransportConfig, httpClient *http.Client, startupTimeout time.Duration) (*HTTPExecutor, error) {
	if cfg == nil || cfg.URL == "" {
		return nil, runtime.NewRuntimeError(runtime.ErrCodeInvalidRequest, "http config required")
	}

	if httpClient == nil {
		httpClient = &http.Client{}
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "goakt-mcp", Version: "v0.1.0"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint:   cfg.URL,
		HTTPClient: httpClient,
		MaxRetries: 2,
	}

	ctx := context.Background()
	if startupTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, startupTimeout)
		defer cancel()
	}

	sess, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, runtime.WrapRuntimeError(runtime.ErrCodeTransportFailure, "http connect failed", err)
	}

	return &HTTPExecutor{client: client, sess: sess}, nil
}
