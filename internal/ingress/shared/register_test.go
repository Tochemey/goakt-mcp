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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tochemey/goakt-mcp/mcp"
)

// --- test doubles for register tests -----------------------------------------

type testResolver struct {
	tenantID mcp.TenantID
	clientID mcp.ClientID
	err      error
}

func (r *testResolver) ResolveIdentity(_ *http.Request) (mcp.TenantID, mcp.ClientID, error) {
	return r.tenantID, r.clientID, r.err
}

// --- BuildGetServer ----------------------------------------------------------

func TestBuildGetServer_IdentityFailure(t *testing.T) {
	gw := &stubInvoker{tools: []mcp.Tool{{ID: "tool-1"}}}
	resolver := &testResolver{err: errors.New("bad auth")}
	getServer := BuildGetServer(gw, resolver)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	srv := getServer(req)
	assert.Nil(t, srv, "should return nil when identity resolution fails")
}

func TestBuildGetServer_ListToolsFailure(t *testing.T) {
	gw := &stubInvoker{listErr: errors.New("registry down")}
	resolver := &testResolver{tenantID: "acme", clientID: "c1"}
	getServer := BuildGetServer(gw, resolver)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	srv := getServer(req)
	assert.Nil(t, srv, "should return nil when ListTools fails")
}

func TestBuildGetServer_Success(t *testing.T) {
	gw := &stubInvoker{tools: []mcp.Tool{{ID: "tool-1"}}}
	resolver := &testResolver{tenantID: "acme", clientID: "c1"}
	getServer := BuildGetServer(gw, resolver)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	srv := getServer(req)
	assert.NotNil(t, srv, "should return a server when identity and tools succeed")
}

func TestBuildGetServer_EmptyTools(t *testing.T) {
	gw := &stubInvoker{tools: []mcp.Tool{}}
	resolver := &testResolver{tenantID: "acme", clientID: "c1"}
	getServer := BuildGetServer(gw, resolver)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	srv := getServer(req)
	assert.NotNil(t, srv, "should return a server even with zero tools")
}

// --- RegisterTool ------------------------------------------------------------

func TestRegisterTool_NoSchemas(t *testing.T) {
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test"}, nil)
	gw := &stubInvoker{
		result: &mcp.ExecutionResult{
			Status: mcp.ExecutionStatusSuccess,
			Output: map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}},
		},
	}
	tool := mcp.Tool{ID: "simple"}
	RegisterTool(srv, gw, tool, "t1", "c1")

	// Connect to verify the tool is registered.
	serverT, clientT := sdkmcp.NewInMemoryTransports()
	_, err := srv.Connect(context.Background(), serverT, nil)
	require.NoError(t, err)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client"}, nil)
	sess, err := client.Connect(context.Background(), clientT, nil)
	require.NoError(t, err)
	defer sess.Close()

	toolsResult, err := sess.ListTools(context.Background(), &sdkmcp.ListToolsParams{})
	require.NoError(t, err)
	require.Len(t, toolsResult.Tools, 1)
	assert.Equal(t, "simple", toolsResult.Tools[0].Name)
}

func TestRegisterTool_WithSchemas(t *testing.T) {
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test"}, nil)
	gw := &stubInvoker{
		result: &mcp.ExecutionResult{
			Status: mcp.ExecutionStatusSuccess,
			Output: map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}},
		},
	}
	tool := mcp.Tool{
		ID: "multi",
		Schemas: []mcp.ToolSchema{
			{Name: "read", Description: "Read op", InputSchema: []byte(`{"type":"object"}`)},
			{Name: "write", Description: "Write op"},
		},
	}
	RegisterTool(srv, gw, tool, "t1", "c1")

	serverT, clientT := sdkmcp.NewInMemoryTransports()
	_, err := srv.Connect(context.Background(), serverT, nil)
	require.NoError(t, err)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client"}, nil)
	sess, err := client.Connect(context.Background(), clientT, nil)
	require.NoError(t, err)
	defer sess.Close()

	toolsResult, err := sess.ListTools(context.Background(), &sdkmcp.ListToolsParams{})
	require.NoError(t, err)
	require.Len(t, toolsResult.Tools, 2)

	names := make(map[string]bool, 2)
	for _, t := range toolsResult.Tools {
		names[t.Name] = true
	}
	assert.True(t, names["read"])
	assert.True(t, names["write"])
}
