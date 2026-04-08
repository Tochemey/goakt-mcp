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

package config

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goaktlog "github.com/tochemey/goakt/v4/log"

	"github.com/tochemey/goakt-mcp/mcp"
)

func TestDefaultConstants(t *testing.T) {
	assert.Equal(t, 5*time.Minute, mcp.DefaultSessionIdleTimeout)
	assert.Equal(t, 30*time.Second, mcp.DefaultRequestTimeout)
	assert.Equal(t, 10*time.Second, mcp.DefaultStartupTimeout)
}

func TestConfigZeroValue(t *testing.T) {
	var cfg mcp.Config
	assert.Empty(t, cfg.Tools)
	assert.Empty(t, cfg.Tenants)
	assert.Zero(t, cfg.Runtime.RequestTimeout)
}

func TestRuntimeConfig(t *testing.T) {
	cfg := mcp.RuntimeConfig{
		SessionIdleTimeout: mcp.DefaultSessionIdleTimeout,
		RequestTimeout:     mcp.DefaultRequestTimeout,
		StartupTimeout:     mcp.DefaultStartupTimeout,
	}
	assert.Equal(t, 5*time.Minute, cfg.SessionIdleTimeout)
	assert.Equal(t, 30*time.Second, cfg.RequestTimeout)
	assert.Equal(t, 10*time.Second, cfg.StartupTimeout)
}

// testDiscoveryProvider is a minimal DiscoveryProvider for config tests.
type testDiscoveryProvider struct{}

func (t *testDiscoveryProvider) ID() string                                        { return "test" }
func (t *testDiscoveryProvider) Start(_ context.Context) error                     { return nil }
func (t *testDiscoveryProvider) DiscoverPeers(_ context.Context) ([]string, error) { return nil, nil }
func (t *testDiscoveryProvider) Stop(_ context.Context) error                      { return nil }

func TestClusterConfig(t *testing.T) {
	cfg := mcp.ClusterConfig{
		Enabled:           true,
		DiscoveryProvider: &testDiscoveryProvider{},
		RegistrarRole:     "control-plane",
		PeersPort:         15000,
		RemotingPort:      15001,
	}
	assert.True(t, cfg.Enabled)
	assert.NotNil(t, cfg.DiscoveryProvider)
	assert.Equal(t, "test", cfg.DiscoveryProvider.ID())
	assert.Equal(t, "control-plane", cfg.RegistrarRole)
	assert.Equal(t, 15000, cfg.PeersPort)
	assert.Equal(t, 15001, cfg.RemotingPort)
}

func TestTelemetryConfig(t *testing.T) {
	cfg := mcp.TelemetryConfig{OTLPEndpoint: "http://otel-collector:4318"}
	assert.Equal(t, "http://otel-collector:4318", cfg.OTLPEndpoint)
}

func TestTenantConfig(t *testing.T) {
	cfg := mcp.TenantConfig{
		ID: mcp.TenantID("acme-dev"),
		Quotas: mcp.TenantQuotaConfig{
			RequestsPerMinute:  1000,
			ConcurrentSessions: 200,
		},
	}
	assert.Equal(t, mcp.TenantID("acme-dev"), cfg.ID)
	assert.Equal(t, 1000, cfg.Quotas.RequestsPerMinute)
	assert.Equal(t, 200, cfg.Quotas.ConcurrentSessions)
}

func TestStdioToolConfig(t *testing.T) {
	cfg := ToolConfig{
		ID:                  mcp.ToolID("filesystem"),
		Transport:           mcp.TransportStdio,
		Command:             "npx",
		Args:                []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
		Env:                 map[string]string{},
		WorkingDirectory:    "/",
		StartupTimeout:      10 * time.Second,
		RequestTimeout:      30 * time.Second,
		IdleTimeout:         5 * time.Minute,
		Routing:             mcp.RoutingSticky,
		CredentialPolicy:    mcp.CredentialPolicyOptional,
		AuthorizationPolicy: mcp.AuthorizationPolicyTenantAllowlist,
	}

	assert.Equal(t, mcp.ToolID("filesystem"), cfg.ID)
	assert.Equal(t, mcp.TransportStdio, cfg.Transport)
	assert.Equal(t, "npx", cfg.Command)
	require.Len(t, cfg.Args, 3)
	assert.Equal(t, mcp.RoutingSticky, cfg.Routing)
	assert.Equal(t, mcp.CredentialPolicyOptional, cfg.CredentialPolicy)
	assert.Equal(t, mcp.AuthorizationPolicyTenantAllowlist, cfg.AuthorizationPolicy)
	assert.Equal(t, 10*time.Second, cfg.StartupTimeout)
	assert.Equal(t, 30*time.Second, cfg.RequestTimeout)
	assert.Equal(t, 5*time.Minute, cfg.IdleTimeout)
	assert.Equal(t, "/", cfg.WorkingDirectory)
	assert.Equal(t, map[string]string{}, cfg.Env)
}

func TestHTTPToolConfig(t *testing.T) {
	cfg := ToolConfig{
		ID:                  mcp.ToolID("docs-search"),
		Transport:           mcp.TransportHTTP,
		URL:                 "https://mcp.internal.example.com",
		RequestTimeout:      15 * time.Second,
		IdleTimeout:         2 * time.Minute,
		Routing:             mcp.RoutingLeastLoaded,
		CredentialPolicy:    mcp.CredentialPolicyRequired,
		AuthorizationPolicy: mcp.AuthorizationPolicyTenantAllowlist,
	}

	assert.Equal(t, mcp.ToolID("docs-search"), cfg.ID)
	assert.Equal(t, mcp.TransportHTTP, cfg.Transport)
	assert.Equal(t, "https://mcp.internal.example.com", cfg.URL)
	assert.Equal(t, mcp.RoutingLeastLoaded, cfg.Routing)
	assert.Equal(t, mcp.CredentialPolicyRequired, cfg.CredentialPolicy)
	assert.Equal(t, mcp.AuthorizationPolicyTenantAllowlist, cfg.AuthorizationPolicy)
	assert.Equal(t, 15*time.Second, cfg.RequestTimeout)
	assert.Equal(t, 2*time.Minute, cfg.IdleTimeout)
}

func TestToolConfigToTool(t *testing.T) {
	defaults := mcp.RuntimeConfig{
		SessionIdleTimeout: mcp.DefaultSessionIdleTimeout,
		RequestTimeout:     mcp.DefaultRequestTimeout,
		StartupTimeout:     mcp.DefaultStartupTimeout,
	}

	t.Run("stdio tool with explicit values", func(t *testing.T) {
		cfg := ToolConfig{
			ID:                  mcp.ToolID("fs"),
			Transport:           mcp.TransportStdio,
			Command:             "npx",
			Args:                []string{"-y", "mcp-server"},
			StartupTimeout:      15 * time.Second,
			RequestTimeout:      45 * time.Second,
			IdleTimeout:         10 * time.Minute,
			Routing:             mcp.RoutingSticky,
			CredentialPolicy:    mcp.CredentialPolicyOptional,
			AuthorizationPolicy: mcp.AuthorizationPolicyTenantAllowlist,
		}
		tool := ToolConfigToTool(cfg, defaults)
		assert.Equal(t, mcp.ToolID("fs"), tool.ID)
		assert.Equal(t, mcp.TransportStdio, tool.Transport)
		require.NotNil(t, tool.Stdio)
		assert.Equal(t, "npx", tool.Stdio.Command)
		require.Len(t, tool.Stdio.Args, 2)
		assert.Equal(t, 15*time.Second, tool.StartupTimeout)
		assert.Equal(t, 45*time.Second, tool.RequestTimeout)
		assert.Equal(t, 10*time.Minute, tool.IdleTimeout)
		assert.Equal(t, mcp.ToolStateEnabled, tool.State)
	})

	t.Run("http tool with zero values uses defaults", func(t *testing.T) {
		cfg := ToolConfig{
			ID:        mcp.ToolID("http-tool"),
			Transport: mcp.TransportHTTP,
			URL:       "http://localhost:8080",
		}
		tool := ToolConfigToTool(cfg, defaults)
		assert.Equal(t, mcp.ToolID("http-tool"), tool.ID)
		assert.Equal(t, mcp.TransportHTTP, tool.Transport)
		require.NotNil(t, tool.HTTP)
		assert.Equal(t, "http://localhost:8080", tool.HTTP.URL)
		assert.Nil(t, tool.HTTP.TLS)
		assert.Equal(t, mcp.DefaultStartupTimeout, tool.StartupTimeout)
		assert.Equal(t, mcp.DefaultRequestTimeout, tool.RequestTimeout)
		assert.Equal(t, mcp.DefaultSessionIdleTimeout, tool.IdleTimeout)
		assert.Equal(t, mcp.ToolStateEnabled, tool.State)
	})

	t.Run("negative tool timeouts are replaced with defaults", func(t *testing.T) {
		cfg := ToolConfig{
			ID:             mcp.ToolID("neg-tool"),
			Transport:      mcp.TransportStdio,
			Command:        "echo",
			StartupTimeout: -5 * time.Second,
			RequestTimeout: -10 * time.Second,
			IdleTimeout:    -1 * time.Minute,
		}
		tool := ToolConfigToTool(cfg, defaults)
		assert.Equal(t, mcp.DefaultStartupTimeout, tool.StartupTimeout)
		assert.Equal(t, mcp.DefaultRequestTimeout, tool.RequestTimeout)
		assert.Equal(t, mcp.DefaultSessionIdleTimeout, tool.IdleTimeout)
	})

	t.Run("zero runtime defaults fall through to mcp defaults", func(t *testing.T) {
		cfg := ToolConfig{
			ID:        mcp.ToolID("zero-defaults"),
			Transport: mcp.TransportStdio,
			Command:   "echo",
		}
		// Pass zero-value RuntimeConfig so all timeouts are 0
		tool := ToolConfigToTool(cfg, mcp.RuntimeConfig{})
		assert.Equal(t, mcp.DefaultStartupTimeout, tool.StartupTimeout)
		assert.Equal(t, mcp.DefaultRequestTimeout, tool.RequestTimeout)
		assert.Equal(t, mcp.DefaultSessionIdleTimeout, tool.IdleTimeout)
	})

	t.Run("http tool with HTTPTLS maps to runtime", func(t *testing.T) {
		cfg := ToolConfig{
			ID:        mcp.ToolID("mcp-secure"),
			Transport: mcp.TransportHTTP,
			URL:       "https://mcp.internal.example.com",
			HTTPTLS: &mcp.TLSClientConfig{
				CACertFile:         "/etc/ca.crt",
				ClientCertFile:     "/etc/client.crt",
				ClientKeyFile:      "/etc/client.key",
				InsecureSkipVerify: false,
			},
		}
		tool := ToolConfigToTool(cfg, defaults)
		assert.Equal(t, mcp.ToolID("mcp-secure"), tool.ID)
		require.NotNil(t, tool.HTTP)
		assert.Equal(t, "https://mcp.internal.example.com", tool.HTTP.URL)
		require.NotNil(t, tool.HTTP.TLS)
		assert.Equal(t, "/etc/ca.crt", tool.HTTP.TLS.CACertFile)
		assert.Equal(t, "/etc/client.crt", tool.HTTP.TLS.ClientCertFile)
		assert.Equal(t, "/etc/client.key", tool.HTTP.TLS.ClientKeyFile)
		assert.False(t, tool.HTTP.TLS.InsecureSkipVerify)
	})
}

func TestParseLogLevel(t *testing.T) {
	cases := []struct {
		input    string
		expected goaktlog.Level
	}{
		{"debug", goaktlog.DebugLevel},
		{"info", goaktlog.InfoLevel},
		{"warning", goaktlog.WarningLevel},
		{"warn", goaktlog.WarningLevel},
		{"error", goaktlog.ErrorLevel},
		{"fatal", goaktlog.FatalLevel},
		{"panic", goaktlog.PanicLevel},
		{"", goaktlog.InvalidLevel},
		{"unknown", goaktlog.InvalidLevel},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := ParseLogLevel(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestNewLogger(t *testing.T) {
	t.Run("empty level returns default logger", func(t *testing.T) {
		l := NewLogger("")
		require.NotNil(t, l)
	})

	t.Run("invalid level returns default logger", func(t *testing.T) {
		l := NewLogger("nonsense")
		require.NotNil(t, l)
	})

	t.Run("valid level returns slog logger", func(t *testing.T) {
		l := NewLogger("info")
		require.NotNil(t, l)
	})

	t.Run("debug level returns slog logger", func(t *testing.T) {
		l := NewLogger("debug")
		require.NotNil(t, l)
	})
}

func TestFullConfig(t *testing.T) {
	cfg := mcp.Config{
		Runtime: mcp.RuntimeConfig{
			SessionIdleTimeout: mcp.DefaultSessionIdleTimeout,
			RequestTimeout:     mcp.DefaultRequestTimeout,
			StartupTimeout:     mcp.DefaultStartupTimeout,
		},
		Cluster: mcp.ClusterConfig{
			Enabled:           true,
			DiscoveryProvider: &testDiscoveryProvider{},
			RegistrarRole:     "control-plane",
		},
		Telemetry: mcp.TelemetryConfig{OTLPEndpoint: "http://otel-collector:4318"},
		Tenants: []mcp.TenantConfig{
			{ID: "acme-dev", Quotas: mcp.TenantQuotaConfig{RequestsPerMinute: 1000, ConcurrentSessions: 200}},
		},
		Tools: []mcp.Tool{
			{ID: "filesystem", Transport: mcp.TransportStdio, Stdio: &mcp.StdioTransportConfig{Command: "npx"}, State: mcp.ToolStateEnabled},
		},
	}

	assert.True(t, cfg.Cluster.Enabled)
	require.Len(t, cfg.Tenants, 1)
	require.Len(t, cfg.Tools, 1)
	assert.Equal(t, mcp.ToolID("filesystem"), cfg.Tools[0].ID)
}
