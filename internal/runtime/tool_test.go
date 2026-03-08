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

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tochemey/goakt-mcp/mcp"
)

func TestTransportTypeConstants(t *testing.T) {
	assert.Equal(t, mcp.TransportType("stdio"), mcp.TransportStdio)
	assert.Equal(t, mcp.TransportType("http"), mcp.TransportHTTP)
}

func TestRoutingModeConstants(t *testing.T) {
	assert.Equal(t, mcp.RoutingMode("sticky"), mcp.RoutingSticky)
	assert.Equal(t, mcp.RoutingMode("least_loaded"), mcp.RoutingLeastLoaded)
}

func TestToolStateConstants(t *testing.T) {
	assert.Equal(t, mcp.ToolState("enabled"), mcp.ToolStateEnabled)
	assert.Equal(t, mcp.ToolState("disabled"), mcp.ToolStateDisabled)
	assert.Equal(t, mcp.ToolState("degraded"), mcp.ToolStateDegraded)
	assert.Equal(t, mcp.ToolState("unavailable"), mcp.ToolStateUnavailable)
}

func TestCredentialPolicyConstants(t *testing.T) {
	assert.Equal(t, mcp.CredentialPolicy("optional"), mcp.CredentialPolicyOptional)
	assert.Equal(t, mcp.CredentialPolicy("required"), mcp.CredentialPolicyRequired)
}

func TestAuthorizationPolicyConstants(t *testing.T) {
	assert.Equal(t, mcp.AuthorizationPolicy("tenant_allowlist"), mcp.AuthorizationPolicyTenantAllowlist)
}

func TestToolIsStdio(t *testing.T) {
	t.Run("returns true for stdio transport", func(t *testing.T) {
		tool := mcp.Tool{Transport: mcp.TransportStdio}
		assert.True(t, tool.IsStdio())
		assert.False(t, tool.IsHTTP())
	})
	t.Run("returns false for http transport", func(t *testing.T) {
		tool := mcp.Tool{Transport: mcp.TransportHTTP}
		assert.False(t, tool.IsStdio())
		assert.True(t, tool.IsHTTP())
	})
}

func TestToolIsAvailable(t *testing.T) {
	tests := []struct {
		state     mcp.ToolState
		available bool
	}{
		{mcp.ToolStateEnabled, true},
		{mcp.ToolStateDegraded, true},
		{mcp.ToolStateDisabled, false},
		{mcp.ToolStateUnavailable, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			tool := mcp.Tool{State: tt.state}
			assert.Equal(t, tt.available, tool.IsAvailable())
		})
	}
}

func TestStdioToolConstruction(t *testing.T) {
	tool := mcp.Tool{
		ID:        mcp.ToolID("filesystem"),
		Transport: mcp.TransportStdio,
		Stdio: &mcp.StdioTransportConfig{
			Command:          "npx",
			Args:             []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
			Env:              map[string]string{"NODE_ENV": "production"},
			WorkingDirectory: "/",
		},
		StartupTimeout:      10 * time.Second,
		RequestTimeout:      30 * time.Second,
		IdleTimeout:         5 * time.Minute,
		Routing:             mcp.RoutingSticky,
		CredentialPolicy:    mcp.CredentialPolicyOptional,
		AuthorizationPolicy: mcp.AuthorizationPolicyTenantAllowlist,
		State:               mcp.ToolStateEnabled,
	}

	assert.Equal(t, mcp.ToolID("filesystem"), tool.ID)
	assert.True(t, tool.IsStdio())
	assert.False(t, tool.IsHTTP())
	assert.NotNil(t, tool.Stdio)
	assert.Nil(t, tool.HTTP)
	assert.Equal(t, "npx", tool.Stdio.Command)
	assert.Equal(t, mcp.RoutingSticky, tool.Routing)
	assert.True(t, tool.IsAvailable())
}

func TestHTTPToolConstruction(t *testing.T) {
	tool := mcp.Tool{
		ID:        mcp.ToolID("docs-search"),
		Transport: mcp.TransportHTTP,
		HTTP: &mcp.HTTPTransportConfig{
			URL: "https://mcp.internal.example.com",
		},
		RequestTimeout:      15 * time.Second,
		IdleTimeout:         2 * time.Minute,
		Routing:             mcp.RoutingLeastLoaded,
		CredentialPolicy:    mcp.CredentialPolicyRequired,
		AuthorizationPolicy: mcp.AuthorizationPolicyTenantAllowlist,
		State:               mcp.ToolStateEnabled,
	}

	assert.Equal(t, mcp.ToolID("docs-search"), tool.ID)
	assert.False(t, tool.IsStdio())
	assert.True(t, tool.IsHTTP())
	assert.Nil(t, tool.Stdio)
	assert.NotNil(t, tool.HTTP)
	assert.Equal(t, "https://mcp.internal.example.com", tool.HTTP.URL)
	assert.Equal(t, mcp.RoutingLeastLoaded, tool.Routing)
}
