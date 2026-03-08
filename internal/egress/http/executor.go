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
// Each executor owns a single MCP client session and is bound to one tool endpoint.
type HTTPExecutor struct {
	client   *mcp.Client
	sess     *mcp.ClientSession
	closed   sync.Once
	closeErr error
}

// Execute runs the MCP tools/call invocation and returns the result.
// The context should carry a request-scoped deadline. Execute does not
// set its own timeout.
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

// Close releases the HTTP session and connection. Close is safe to call
// multiple times; only the first call performs cleanup.
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
//
// When cfg.TLS is set, a per-tool HTTP client is created with the appropriate
// TLS configuration (custom CA, client certificates, or InsecureSkipVerify).
// When cfg.TLS is nil and fallbackClient is non-nil, fallbackClient is used.
// When both are nil, a default http.Client is used.
//
// The fallbackClient should not set a global Timeout since request-level
// deadlines are managed by the session's context.
func NewHTTPExecutor(cfg *runtime.HTTPTransportConfig, fallbackClient *http.Client, startupTimeout time.Duration) (*HTTPExecutor, error) {
	if cfg == nil || cfg.URL == "" {
		return nil, runtime.NewRuntimeError(runtime.ErrCodeInvalidRequest, "http config required")
	}

	httpClient, err := buildHTTPClient(cfg, fallbackClient)
	if err != nil {
		return nil, err
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

// buildHTTPClient returns the http.Client to use for this tool.
// When the tool has TLS configuration, a new client with a cloned transport
// is created so that TLS settings are isolated per tool. Otherwise the
// fallback client (or a default) is returned.
func buildHTTPClient(cfg *runtime.HTTPTransportConfig, fallback *http.Client) (*http.Client, error) {
	if cfg.TLS == nil {
		if fallback != nil {
			return fallback, nil
		}
		return &http.Client{}, nil
	}

	tlsCfg, err := runtime.BuildClientTLSConfig(cfg.TLS)
	if err != nil {
		return nil, err
	}

	base := http.DefaultTransport
	if fallback != nil && fallback.Transport != nil {
		base = fallback.Transport
	}

	if t, ok := base.(*http.Transport); ok {
		clone := t.Clone()
		clone.TLSClientConfig = tlsCfg
		return &http.Client{Transport: clone}, nil
	}

	return &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
	}, nil
}
