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

package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tochemey/goakt-mcp/mcp"
)

// --- helpers -----------------------------------------------------------------

func makeCallToolRequest(name string, args map[string]any) *sdkmcp.CallToolRequest {
	raw, _ := json.Marshal(args)
	return &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{
			Name:      name,
			Arguments: raw,
		},
	}
}

// stubInvoker is a minimal Invoker for testing.
type stubInvoker struct {
	result  *mcp.ExecutionResult
	err     error
	listErr error
	tools   []mcp.Tool
}

func (s *stubInvoker) Invoke(_ context.Context, _ *mcp.Invocation) (*mcp.ExecutionResult, error) {
	return s.result, s.err
}

func (s *stubInvoker) ListTools(_ context.Context) ([]mcp.Tool, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.tools, nil
}

// --- RequestToInvocation -----------------------------------------------------

func TestRequestToInvocation(t *testing.T) {
	t.Run("flat shape: no nested name uses gateway tool ID as backend name", func(t *testing.T) {
		req := makeCallToolRequest("my-tool", map[string]any{"key": "val"})
		inv, err := RequestToInvocation(req, "my-tool", "tenant-1", "client-1")
		require.NoError(t, err)

		assert.Equal(t, mcp.ToolID("my-tool"), inv.ToolID)
		assert.Equal(t, "my-tool", inv.Params["name"])
		assert.Equal(t, "tools/call", inv.Method)
		assert.Equal(t, mcp.TenantID("tenant-1"), inv.Correlation.TenantID)
		assert.Equal(t, mcp.ClientID("client-1"), inv.Correlation.ClientID)
		assert.NotEmpty(t, inv.Correlation.RequestID)
		assert.False(t, inv.ReceivedAt.IsZero())

		params, ok := inv.Params["arguments"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "val", params["key"])
	})

	t.Run("nested shape: args name used as backend tool name", func(t *testing.T) {
		req := makeCallToolRequest("filesystem", map[string]any{
			"name":      "list_directory",
			"arguments": map[string]any{"path": "/tmp"},
		})
		inv, err := RequestToInvocation(req, "filesystem", "acme", "agent-1")
		require.NoError(t, err)

		assert.Equal(t, mcp.ToolID("filesystem"), inv.ToolID)
		assert.Equal(t, "list_directory", inv.Params["name"])

		backendArgs, ok := inv.Params["arguments"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "/tmp", backendArgs["path"])
	})

	t.Run("nil arguments produces nil args map", func(t *testing.T) {
		req := &sdkmcp.CallToolRequest{
			Params: &sdkmcp.CallToolParamsRaw{Name: "tool-x"},
		}
		inv, err := RequestToInvocation(req, "tool-x", "t", "c")
		require.NoError(t, err)
		assert.Equal(t, "tool-x", inv.Params["name"])
		assert.Nil(t, inv.Params["arguments"])
	})

	t.Run("nil request returns error", func(t *testing.T) {
		_, err := RequestToInvocation(nil, "tool", "t", "c")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "request and params are required")
	})

	t.Run("nil params returns error", func(t *testing.T) {
		_, err := RequestToInvocation(&sdkmcp.CallToolRequest{}, "tool", "t", "c")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "request and params are required")
	})

	t.Run("invalid JSON arguments returns error", func(t *testing.T) {
		req := &sdkmcp.CallToolRequest{
			Params: &sdkmcp.CallToolParamsRaw{
				Name:      "tool-y",
				Arguments: json.RawMessage(`not-json`),
			},
		}
		_, err := RequestToInvocation(req, "tool-y", "t", "c")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid tool arguments")
	})
}

// --- ExecutionResultToCallToolResult -----------------------------------------

func TestExecutionResultToCallToolResult(t *testing.T) {
	t.Run("non-nil gateway error with nil result returns tool error", func(t *testing.T) {
		r := ExecutionResultToCallToolResult(nil, errors.New("boom"))
		require.NotNil(t, r)
		assert.True(t, r.IsError)
	})

	t.Run("nil result with nil error returns empty error", func(t *testing.T) {
		r := ExecutionResultToCallToolResult(nil, nil)
		require.NotNil(t, r)
		assert.True(t, r.IsError)
	})

	t.Run("non-success status returns tool error", func(t *testing.T) {
		res := &mcp.ExecutionResult{
			Status: mcp.ExecutionStatusDenied,
			Err:    mcp.NewRuntimeError(mcp.ErrCodePolicyDenied, "policy denied"),
		}
		r := ExecutionResultToCallToolResult(res, nil)
		require.NotNil(t, r)
		assert.True(t, r.IsError)
	})

	t.Run("non-success status without RuntimeError uses status string", func(t *testing.T) {
		res := &mcp.ExecutionResult{
			Status: mcp.ExecutionStatusThrottled,
		}
		r := ExecutionResultToCallToolResult(res, nil)
		require.NotNil(t, r)
		assert.True(t, r.IsError)
	})

	t.Run("success status with output returns content", func(t *testing.T) {
		res := &mcp.ExecutionResult{
			Status: mcp.ExecutionStatusSuccess,
			Output: map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "hello"},
				},
			},
		}
		r := ExecutionResultToCallToolResult(res, nil)
		require.NotNil(t, r)
		assert.False(t, r.IsError)
		require.Len(t, r.Content, 1)
		txt, ok := r.Content[0].(*sdkmcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "hello", txt.Text)
	})

	t.Run("gateway error with non-nil result uses result", func(t *testing.T) {
		res := &mcp.ExecutionResult{
			Status: mcp.ExecutionStatusSuccess,
			Output: map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}},
		}
		r := ExecutionResultToCallToolResult(res, errors.New("soft-err"))
		require.NotNil(t, r)
		assert.False(t, r.IsError)
	})
}

// --- OutputToCallToolResult --------------------------------------------------

func TestOutputToCallToolResult(t *testing.T) {
	t.Run("nil output returns empty result", func(t *testing.T) {
		r := OutputToCallToolResult(nil)
		require.NotNil(t, r)
		assert.Nil(t, r.Content)
	})

	t.Run("text content items are reconstructed", func(t *testing.T) {
		out := map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "line one"},
				{"type": "text", "text": "line two"},
			},
		}
		r := OutputToCallToolResult(out)
		require.Len(t, r.Content, 2)
		assert.Equal(t, "line one", r.Content[0].(*sdkmcp.TextContent).Text)
		assert.Equal(t, "line two", r.Content[1].(*sdkmcp.TextContent).Text)
		assert.Equal(t, out, r.StructuredContent)
	})

	t.Run("text item with empty text is skipped", func(t *testing.T) {
		out := map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": ""},
				{"type": "text", "text": "keep"},
			},
		}
		r := OutputToCallToolResult(out)
		require.Len(t, r.Content, 1)
		assert.Equal(t, "keep", r.Content[0].(*sdkmcp.TextContent).Text)
	})

	t.Run("non-text content falls back to JSON serialization", func(t *testing.T) {
		out := map[string]any{
			"content": []map[string]any{
				{"type": "image", "data": "base64abc"},
			},
		}
		r := OutputToCallToolResult(out)
		require.Len(t, r.Content, 1)
		txt, ok := r.Content[0].(*sdkmcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, txt.Text, "image")
	})

	t.Run("output without content key falls back to JSON", func(t *testing.T) {
		out := map[string]any{"result": 42}
		r := OutputToCallToolResult(out)
		require.Len(t, r.Content, 1)
		txt, ok := r.Content[0].(*sdkmcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, txt.Text, "42")
	})

	t.Run("JSON-decoded content ([]any) is reconstructed correctly", func(t *testing.T) {
		raw := map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "before"},
			},
		}
		encoded, err := json.Marshal(raw)
		require.NoError(t, err)
		var decoded map[string]any
		require.NoError(t, json.Unmarshal(encoded, &decoded))

		r := OutputToCallToolResult(decoded)
		require.Len(t, r.Content, 1)
		txt, ok := r.Content[0].(*sdkmcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "before", txt.Text)
	})
}

// --- DispatchToolCall --------------------------------------------------------

func TestDispatchToolCall(t *testing.T) {
	t.Run("successful invocation returns text content", func(t *testing.T) {
		gw := &stubInvoker{
			result: &mcp.ExecutionResult{
				Status: mcp.ExecutionStatusSuccess,
				Output: map[string]any{
					"content": []map[string]any{{"type": "text", "text": "done"}},
				},
			},
		}
		req := makeCallToolRequest("tool-a", map[string]any{"x": 1})
		r, err := DispatchToolCall(context.Background(), gw, req, "tool-a", "t1", "c1")
		require.NoError(t, err)
		require.NotNil(t, r)
		assert.False(t, r.IsError)
	})

	t.Run("invalid JSON arguments returns tool error not Go error", func(t *testing.T) {
		gw := &stubInvoker{}
		req := &sdkmcp.CallToolRequest{
			Params: &sdkmcp.CallToolParamsRaw{
				Name:      "tool-b",
				Arguments: json.RawMessage(`{bad json`),
			},
		}
		r, err := DispatchToolCall(context.Background(), gw, req, "tool-b", "t1", "c1")
		require.NoError(t, err)
		require.NotNil(t, r)
		assert.True(t, r.IsError)
	})

	t.Run("gateway invoke error is surfaced as tool error", func(t *testing.T) {
		gw := &stubInvoker{err: errors.New("internal failure")}
		req := makeCallToolRequest("tool-c", nil)
		r, err := DispatchToolCall(context.Background(), gw, req, "tool-c", "t1", "c1")
		require.NoError(t, err)
		require.NotNil(t, r)
		assert.True(t, r.IsError)
	})
}

// --- Scope propagation -------------------------------------------------------

func TestDispatchToolCall_ScopePropagation(t *testing.T) {
	t.Run("scopes from TokenInfo are attached to invocation", func(t *testing.T) {
		// capturingInvoker records the invocation so we can inspect it.
		var captured *mcp.Invocation
		gw := &capturingInvoker{
			result: &mcp.ExecutionResult{
				Status: mcp.ExecutionStatusSuccess,
				Output: map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}},
			},
			capture: func(inv *mcp.Invocation) { captured = inv },
		}

		info := &auth.TokenInfo{
			Scopes:     []string{"tools:read", "tools:write"},
			Expiration: time.Now().Add(time.Hour),
			UserID:     "user-1",
		}
		ctx := contextWithTokenInfo(info)

		req := makeCallToolRequest("tool-x", map[string]any{"key": "val"})
		r, err := DispatchToolCall(ctx, gw, req, "tool-x", "t1", "c1")
		require.NoError(t, err)
		require.NotNil(t, r)
		assert.False(t, r.IsError)
		require.NotNil(t, captured)
		assert.Equal(t, []string{"tools:read", "tools:write"}, captured.Scopes)
	})

	t.Run("no TokenInfo means empty scopes", func(t *testing.T) {
		var captured *mcp.Invocation
		gw := &capturingInvoker{
			result: &mcp.ExecutionResult{
				Status: mcp.ExecutionStatusSuccess,
				Output: map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}},
			},
			capture: func(inv *mcp.Invocation) { captured = inv },
		}

		req := makeCallToolRequest("tool-y", map[string]any{"a": 1})
		r, err := DispatchToolCall(context.Background(), gw, req, "tool-y", "t1", "c1")
		require.NoError(t, err)
		require.NotNil(t, r)
		assert.False(t, r.IsError)
		require.NotNil(t, captured)
		assert.Nil(t, captured.Scopes)
	})

	t.Run("TokenInfo with empty scopes does not set scopes", func(t *testing.T) {
		var captured *mcp.Invocation
		gw := &capturingInvoker{
			result: &mcp.ExecutionResult{
				Status: mcp.ExecutionStatusSuccess,
				Output: map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}},
			},
			capture: func(inv *mcp.Invocation) { captured = inv },
		}

		info := &auth.TokenInfo{
			Scopes:     nil,
			Expiration: time.Now().Add(time.Hour),
		}
		ctx := contextWithTokenInfo(info)

		req := makeCallToolRequest("tool-z", nil)
		r, err := DispatchToolCall(ctx, gw, req, "tool-z", "t1", "c1")
		require.NoError(t, err)
		require.NotNil(t, r)
		require.NotNil(t, captured)
		assert.Nil(t, captured.Scopes)
	})
}

// capturingInvoker records the invocation for assertion.
type capturingInvoker struct {
	result  *mcp.ExecutionResult
	err     error
	listErr error
	tools   []mcp.Tool
	capture func(*mcp.Invocation)
}

func (c *capturingInvoker) Invoke(_ context.Context, inv *mcp.Invocation) (*mcp.ExecutionResult, error) {
	if c.capture != nil {
		c.capture(inv)
	}
	return c.result, c.err
}

func (c *capturingInvoker) ListTools(_ context.Context) ([]mcp.Tool, error) {
	if c.listErr != nil {
		return nil, c.listErr
	}
	return c.tools, nil
}

// contextWithTokenInfo creates a context.Context with auth.TokenInfo injected
// via the SDK's RequireBearerToken middleware so TokenInfoFromContext works.
func contextWithTokenInfo(info *auth.TokenInfo) context.Context {
	verifier := auth.TokenVerifier(func(_ context.Context, _ string, _ *http.Request) (*auth.TokenInfo, error) {
		return info, nil
	})
	middleware := auth.RequireBearerToken(verifier, nil)

	var captured context.Context
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = r.Context()
	})

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	middleware(inner).ServeHTTP(rec, req)

	if captured == nil {
		return context.Background()
	}
	return captured
}

// --- NewRequestID ------------------------------------------------------------

func TestNewRequestID(t *testing.T) {
	id1 := NewRequestID()
	id2 := NewRequestID()
	assert.NotEmpty(t, string(id1))
	assert.NotEmpty(t, string(id2))
	assert.NotEqual(t, id1, id2, "request IDs must be unique")
	assert.Len(t, string(id1), 16, "expected 16 hex characters")
}
