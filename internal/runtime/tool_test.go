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
)

func TestTransportTypeConstants(t *testing.T) {
	assert.Equal(t, TransportType("stdio"), TransportStdio)
	assert.Equal(t, TransportType("http"), TransportHTTP)
}

func TestRoutingModeConstants(t *testing.T) {
	assert.Equal(t, RoutingMode("sticky"), RoutingSticky)
	assert.Equal(t, RoutingMode("least_loaded"), RoutingLeastLoaded)
}

func TestToolStateConstants(t *testing.T) {
	assert.Equal(t, ToolState("enabled"), ToolStateEnabled)
	assert.Equal(t, ToolState("disabled"), ToolStateDisabled)
	assert.Equal(t, ToolState("degraded"), ToolStateDegraded)
	assert.Equal(t, ToolState("unavailable"), ToolStateUnavailable)
}

func TestCredentialPolicyConstants(t *testing.T) {
	assert.Equal(t, CredentialPolicy("optional"), CredentialPolicyOptional)
	assert.Equal(t, CredentialPolicy("required"), CredentialPolicyRequired)
}

func TestAuthorizationPolicyConstants(t *testing.T) {
	assert.Equal(t, AuthorizationPolicy("tenant_allowlist"), AuthorizationPolicyTenantAllowlist)
}

func TestToolIsStdio(t *testing.T) {
	t.Run("returns true for stdio transport", func(t *testing.T) {
		tool := Tool{Transport: TransportStdio}
		assert.True(t, tool.IsStdio())
		assert.False(t, tool.IsHTTP())
	})
	t.Run("returns false for http transport", func(t *testing.T) {
		tool := Tool{Transport: TransportHTTP}
		assert.False(t, tool.IsStdio())
		assert.True(t, tool.IsHTTP())
	})
}

func TestToolIsAvailable(t *testing.T) {
	tests := []struct {
		state     ToolState
		available bool
	}{
		{ToolStateEnabled, true},
		{ToolStateDegraded, true},
		{ToolStateDisabled, false},
		{ToolStateUnavailable, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			tool := Tool{State: tt.state}
			assert.Equal(t, tt.available, tool.IsAvailable())
		})
	}
}

func TestStdioToolConstruction(t *testing.T) {
	tool := Tool{
		ID:        ToolID("filesystem"),
		Transport: TransportStdio,
		Stdio: &StdioTransportConfig{
			Command:          "npx",
			Args:             []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
			Env:              map[string]string{"NODE_ENV": "production"},
			WorkingDirectory: "/",
		},
		StartupTimeout:      10 * time.Second,
		RequestTimeout:      30 * time.Second,
		IdleTimeout:         5 * time.Minute,
		Routing:             RoutingSticky,
		CredentialPolicy:    CredentialPolicyOptional,
		AuthorizationPolicy: AuthorizationPolicyTenantAllowlist,
		State:               ToolStateEnabled,
	}

	assert.Equal(t, ToolID("filesystem"), tool.ID)
	assert.True(t, tool.IsStdio())
	assert.False(t, tool.IsHTTP())
	assert.NotNil(t, tool.Stdio)
	assert.Nil(t, tool.HTTP)
	assert.Equal(t, "npx", tool.Stdio.Command)
	assert.Equal(t, RoutingSticky, tool.Routing)
	assert.True(t, tool.IsAvailable())
}

func TestHTTPToolConstruction(t *testing.T) {
	tool := Tool{
		ID:        ToolID("docs-search"),
		Transport: TransportHTTP,
		HTTP: &HTTPTransportConfig{
			URL: "https://mcp.internal.example.com",
		},
		RequestTimeout:      15 * time.Second,
		IdleTimeout:         2 * time.Minute,
		Routing:             RoutingLeastLoaded,
		CredentialPolicy:    CredentialPolicyRequired,
		AuthorizationPolicy: AuthorizationPolicyTenantAllowlist,
		State:               ToolStateEnabled,
	}

	assert.Equal(t, ToolID("docs-search"), tool.ID)
	assert.False(t, tool.IsStdio())
	assert.True(t, tool.IsHTTP())
	assert.Nil(t, tool.Stdio)
	assert.NotNil(t, tool.HTTP)
	assert.Equal(t, "https://mcp.internal.example.com", tool.HTTP.URL)
	assert.Equal(t, RoutingLeastLoaded, tool.Routing)
}
