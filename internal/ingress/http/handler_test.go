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

package http_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ingresshttp "github.com/tochemey/goakt-mcp/internal/ingress/http"
	"github.com/tochemey/goakt-mcp/mcp"
)

// --- test doubles ------------------------------------------------------------

// fixedResolver always returns the configured identity.
type fixedResolver struct {
	tenantID mcp.TenantID
	clientID mcp.ClientID
}

func (r *fixedResolver) ResolveIdentity(_ *http.Request) (mcp.TenantID, mcp.ClientID, error) {
	return r.tenantID, r.clientID, nil
}

// errResolver always returns an error, causing session rejection.
type errResolver struct{}

func (r *errResolver) ResolveIdentity(_ *http.Request) (mcp.TenantID, mcp.ClientID, error) {
	return "", "", errors.New("unauthorized")
}

// fakeInvoker implements [ingresshttp.Invoker] for tests.
type fakeInvoker struct {
	tools  []mcp.Tool
	result *mcp.ExecutionResult
	err    error

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

// newTestServer spins up an httptest.Server hosting the ingress handler and
// returns both the server and a connected MCP client session. It registers
// t.Cleanup to close both.
func newTestSession(
	t *testing.T,
	gw ingresshttp.Invoker,
	cfg mcp.IngressConfig,
) *sdkmcp.ClientSession {
	t.Helper()

	h, err := ingresshttp.New(gw, cfg)
	require.NoError(t, err)

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client"}, nil)
	transport := &sdkmcp.StreamableClientTransport{
		Endpoint:             srv.URL,
		DisableStandaloneSSE: true,
	}

	ctx := context.Background()
	session, err := client.Connect(ctx, transport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	return session
}

// --- tests -------------------------------------------------------------------

func TestNew_NilIdentityResolver(t *testing.T) {
	_, err := ingresshttp.New(&fakeInvoker{}, mcp.IngressConfig{})
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
		Stateless:        true,
	}

	session := newTestSession(t, gw, cfg)

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
		Stateless:        true,
	}

	session := newTestSession(t, gw, cfg)

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "fail-tool",
	})
	require.NoError(t, err) // protocol-level must not error; tool error is in result
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestHandler_IdentityResolutionFailure(t *testing.T) {
	// When IdentityResolver returns an error, getServer returns nil,
	// which causes the SDK to reply with 400 Bad Request.
	gw := &fakeInvoker{tools: []mcp.Tool{{ID: "any"}}}
	cfg := mcp.IngressConfig{
		IdentityResolver: &errResolver{},
		Stateless:        true,
	}

	h, err := ingresshttp.New(gw, cfg)
	require.NoError(t, err)

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "bad-client"}, nil)
	transport := &sdkmcp.StreamableClientTransport{
		Endpoint:             srv.URL,
		DisableStandaloneSSE: true,
	}

	_, err = client.Connect(context.Background(), transport, nil)
	// Connection must fail since the server returns 400
	require.Error(t, err)
}

func TestHandler_ListToolsFailure(t *testing.T) {
	// When ListTools fails, getServer returns nil → SDK replies 400.
	gw := &fakeInvoker{listErr: errors.New("registry unavailable")}
	cfg := mcp.IngressConfig{
		IdentityResolver: &fixedResolver{tenantID: "acme", clientID: "c3"},
		Stateless:        true,
	}

	h, err := ingresshttp.New(gw, cfg)
	require.NoError(t, err)

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "list-err-client"}, nil)
	transport := &sdkmcp.StreamableClientTransport{
		Endpoint:             srv.URL,
		DisableStandaloneSSE: true,
	}

	_, err = client.Connect(context.Background(), transport, nil)
	require.Error(t, err)
}
