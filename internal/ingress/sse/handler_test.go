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

package sse_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tochemey/goakt-mcp/internal/ingress/shared"
	ingresssse "github.com/tochemey/goakt-mcp/internal/ingress/sse"
	"github.com/tochemey/goakt-mcp/mcp"
)

// --- test doubles ------------------------------------------------------------

type fixedResolver struct {
	tenantID mcp.TenantID
	clientID mcp.ClientID
}

func (r *fixedResolver) ResolveIdentity(_ *http.Request) (mcp.TenantID, mcp.ClientID, error) {
	return r.tenantID, r.clientID, nil
}

type errResolver struct{}

func (r *errResolver) ResolveIdentity(_ *http.Request) (mcp.TenantID, mcp.ClientID, error) {
	return "", "", errors.New("unauthorized")
}

type fakeInvoker struct {
	tools   []mcp.Tool
	result  *mcp.ExecutionResult
	err     error
	listErr error
}

func (f *fakeInvoker) Invoke(_ context.Context, _ *mcp.Invocation) (*mcp.ExecutionResult, error) {
	return f.result, f.err
}

func (f *fakeInvoker) ListTools(_ context.Context) ([]mcp.Tool, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.tools, nil
}

// --- helpers -----------------------------------------------------------------

func newTestSSESession(
	t *testing.T,
	gw shared.Invoker,
	cfg mcp.IngressConfig,
) *sdkmcp.ClientSession {
	t.Helper()

	h, err := ingresssse.New(gw, cfg)
	require.NoError(t, err)

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-sse-client"}, nil)
	transport := &sdkmcp.SSEClientTransport{
		Endpoint: srv.URL,
	}

	ctx := context.Background()
	session, err := client.Connect(ctx, transport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	return session
}

// --- tests -------------------------------------------------------------------

func TestNew_NilIdentityResolver(t *testing.T) {
	_, err := ingresssse.New(&fakeInvoker{}, mcp.IngressConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "IdentityResolver")
}

func TestHandler_ToolCallSuccess(t *testing.T) {
	gw := &fakeInvoker{
		tools: []mcp.Tool{{ID: "echo"}},
		result: &mcp.ExecutionResult{
			Status: mcp.ExecutionStatusSuccess,
			Output: map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "pong"},
				},
			},
		},
	}

	cfg := mcp.IngressConfig{
		IdentityResolver: &fixedResolver{tenantID: "acme", clientID: "c1"},
	}

	session := newTestSSESession(t, gw, cfg)

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "echo",
		Arguments: map[string]any{"msg": "ping"},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
	require.Len(t, result.Content, 1)
	txt, ok := result.Content[0].(*sdkmcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, "pong", txt.Text)
}

func TestHandler_ToolCallError(t *testing.T) {
	gw := &fakeInvoker{
		tools: []mcp.Tool{{ID: "fail-tool"}},
		result: &mcp.ExecutionResult{
			Status: mcp.ExecutionStatusFailure,
			Err:    mcp.NewRuntimeError(mcp.ErrCodeInternal, "backend error"),
		},
	}

	cfg := mcp.IngressConfig{
		IdentityResolver: &fixedResolver{tenantID: "acme", clientID: "c2"},
	}

	session := newTestSSESession(t, gw, cfg)

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "fail-tool",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestHandler_IdentityResolutionFailure(t *testing.T) {
	gw := &fakeInvoker{tools: []mcp.Tool{{ID: "any"}}}
	cfg := mcp.IngressConfig{
		IdentityResolver: &errResolver{},
	}

	h, err := ingresssse.New(gw, cfg)
	require.NoError(t, err)

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "bad-client"}, nil)
	transport := &sdkmcp.SSEClientTransport{
		Endpoint: srv.URL,
	}

	_, err = client.Connect(context.Background(), transport, nil)
	require.Error(t, err)
}

func TestHandler_ToolWithSchemasRegistersPerSchema(t *testing.T) {
	gw := &fakeInvoker{
		tools: []mcp.Tool{{
			ID: "multi-tool",
			Schemas: []mcp.ToolSchema{
				{Name: "read_file", Description: "Read a file", InputSchema: []byte(`{"type":"object","properties":{"path":{"type":"string"}}}`)},
				{Name: "write_file", Description: "Write a file", InputSchema: []byte(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}}}`)},
			},
		}},
		result: &mcp.ExecutionResult{
			Status: mcp.ExecutionStatusSuccess,
			Output: map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "ok"},
				},
			},
		},
	}

	cfg := mcp.IngressConfig{
		IdentityResolver: &fixedResolver{tenantID: "acme", clientID: "c1"},
	}

	session := newTestSSESession(t, gw, cfg)

	toolsResult, err := session.ListTools(context.Background(), &sdkmcp.ListToolsParams{})
	require.NoError(t, err)
	require.Len(t, toolsResult.Tools, 2)

	names := make(map[string]bool, 2)
	for _, tool := range toolsResult.Tools {
		names[tool.Name] = true
	}
	assert.True(t, names["read_file"])
	assert.True(t, names["write_file"])

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "read_file",
		Arguments: map[string]any{"path": "/tmp/test"},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
}

func TestHandler_ListToolsFailure(t *testing.T) {
	gw := &fakeInvoker{listErr: errors.New("registry unavailable")}
	cfg := mcp.IngressConfig{
		IdentityResolver: &fixedResolver{tenantID: "acme", clientID: "c3"},
	}

	h, err := ingresssse.New(gw, cfg)
	require.NoError(t, err)

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "list-err-client"}, nil)
	transport := &sdkmcp.SSEClientTransport{
		Endpoint: srv.URL,
	}

	_, err = client.Connect(context.Background(), transport, nil)
	require.Error(t, err)
}
