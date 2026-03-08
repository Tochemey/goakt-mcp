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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tochemey/goakt-mcp/internal/runtime"
)

func TestDefaultConstants(t *testing.T) {
	assert.Equal(t, 5*time.Minute, DefaultSessionIdleTimeout)
	assert.Equal(t, 30*time.Second, DefaultRequestTimeout)
	assert.Equal(t, 10*time.Second, DefaultStartupTimeout)
	assert.Equal(t, ":8080", DefaultHTTPListenAddress)
}

func TestConfigZeroValue(t *testing.T) {
	var cfg Config
	assert.Empty(t, cfg.Tools)
	assert.Empty(t, cfg.Tenants)
	assert.Empty(t, cfg.HTTP.ListenAddress)
	assert.Zero(t, cfg.Runtime.RequestTimeout)
}

func TestHTTPConfig(t *testing.T) {
	cfg := HTTPConfig{ListenAddress: ":9090"}
	assert.Equal(t, ":9090", cfg.ListenAddress)
}

func TestRuntimeConfig(t *testing.T) {
	cfg := RuntimeConfig{
		SessionIdleTimeout: DefaultSessionIdleTimeout,
		RequestTimeout:     DefaultRequestTimeout,
		StartupTimeout:     DefaultStartupTimeout,
	}
	assert.Equal(t, 5*time.Minute, cfg.SessionIdleTimeout)
	assert.Equal(t, 30*time.Second, cfg.RequestTimeout)
	assert.Equal(t, 10*time.Second, cfg.StartupTimeout)
}

func TestClusterConfig(t *testing.T) {
	cfg := ClusterConfig{
		Enabled:       true,
		Discovery:     "kubernetes",
		SingletonRole: "control-plane",
		Kubernetes: &KubernetesDiscoveryConfig{
			Namespace:         "default",
			DiscoveryPortName: "gossip",
			RemotingPortName:  "remoting",
			PeersPortName:     "cluster",
			PodLabels:         map[string]string{"app": "goakt-mcp"},
		},
	}
	assert.True(t, cfg.Enabled)
	assert.Equal(t, "kubernetes", cfg.Discovery)
	assert.Equal(t, "control-plane", cfg.SingletonRole)
	require.NotNil(t, cfg.Kubernetes)
	assert.Equal(t, "default", cfg.Kubernetes.Namespace)
	assert.Equal(t, "gossip", cfg.Kubernetes.DiscoveryPortName)
}

func TestTelemetryConfig(t *testing.T) {
	cfg := TelemetryConfig{OTLPEndpoint: "http://otel-collector:4318"}
	assert.Equal(t, "http://otel-collector:4318", cfg.OTLPEndpoint)
}

func TestAuditConfig(t *testing.T) {
	cfg := AuditConfig{Backend: "s3", Bucket: "goakt-mcp-audit"}
	assert.Equal(t, "s3", cfg.Backend)
	assert.Equal(t, "goakt-mcp-audit", cfg.Bucket)
}

func TestCredentialsConfig(t *testing.T) {
	cfg := CredentialsConfig{Providers: []string{"env", "vault"}}
	require.Len(t, cfg.Providers, 2)
	assert.Equal(t, "env", cfg.Providers[0])
	assert.Equal(t, "vault", cfg.Providers[1])
}

func TestTenantConfig(t *testing.T) {
	cfg := TenantConfig{
		ID: runtime.TenantID("acme-dev"),
		Quotas: TenantQuotaConfig{
			RequestsPerMinute:  1000,
			ConcurrentSessions: 200,
		},
	}
	assert.Equal(t, runtime.TenantID("acme-dev"), cfg.ID)
	assert.Equal(t, 1000, cfg.Quotas.RequestsPerMinute)
	assert.Equal(t, 200, cfg.Quotas.ConcurrentSessions)
}

func TestStdioToolConfig(t *testing.T) {
	cfg := ToolConfig{
		ID:                  runtime.ToolID("filesystem"),
		Transport:           runtime.TransportStdio,
		Command:             "npx",
		Args:                []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
		Env:                 map[string]string{},
		WorkingDirectory:    "/",
		StartupTimeout:      10 * time.Second,
		RequestTimeout:      30 * time.Second,
		IdleTimeout:         5 * time.Minute,
		Routing:             runtime.RoutingSticky,
		CredentialPolicy:    runtime.CredentialPolicyOptional,
		AuthorizationPolicy: runtime.AuthorizationPolicyTenantAllowlist,
	}

	assert.Equal(t, runtime.ToolID("filesystem"), cfg.ID)
	assert.Equal(t, runtime.TransportStdio, cfg.Transport)
	assert.Equal(t, "npx", cfg.Command)
	require.Len(t, cfg.Args, 3)
	assert.Equal(t, runtime.RoutingSticky, cfg.Routing)
	assert.Equal(t, runtime.CredentialPolicyOptional, cfg.CredentialPolicy)
}

func TestHTTPToolConfig(t *testing.T) {
	cfg := ToolConfig{
		ID:                  runtime.ToolID("docs-search"),
		Transport:           runtime.TransportHTTP,
		URL:                 "https://mcp.internal.example.com",
		RequestTimeout:      15 * time.Second,
		IdleTimeout:         2 * time.Minute,
		Routing:             runtime.RoutingLeastLoaded,
		CredentialPolicy:    runtime.CredentialPolicyRequired,
		AuthorizationPolicy: runtime.AuthorizationPolicyTenantAllowlist,
	}

	assert.Equal(t, runtime.ToolID("docs-search"), cfg.ID)
	assert.Equal(t, runtime.TransportHTTP, cfg.Transport)
	assert.Equal(t, "https://mcp.internal.example.com", cfg.URL)
	assert.Equal(t, runtime.RoutingLeastLoaded, cfg.Routing)
	assert.Equal(t, runtime.CredentialPolicyRequired, cfg.CredentialPolicy)
}

func TestToolConfigToTool(t *testing.T) {
	defaults := RuntimeConfig{
		SessionIdleTimeout: DefaultSessionIdleTimeout,
		RequestTimeout:     DefaultRequestTimeout,
		StartupTimeout:     DefaultStartupTimeout,
	}

	t.Run("stdio tool with explicit values", func(t *testing.T) {
		cfg := ToolConfig{
			ID:                  runtime.ToolID("fs"),
			Transport:           runtime.TransportStdio,
			Command:             "npx",
			Args:                []string{"-y", "mcp-server"},
			StartupTimeout:      15 * time.Second,
			RequestTimeout:      45 * time.Second,
			IdleTimeout:         10 * time.Minute,
			Routing:             runtime.RoutingSticky,
			CredentialPolicy:    runtime.CredentialPolicyOptional,
			AuthorizationPolicy: runtime.AuthorizationPolicyTenantAllowlist,
		}
		tool := ToolConfigToTool(cfg, defaults)
		assert.Equal(t, runtime.ToolID("fs"), tool.ID)
		assert.Equal(t, runtime.TransportStdio, tool.Transport)
		require.NotNil(t, tool.Stdio)
		assert.Equal(t, "npx", tool.Stdio.Command)
		require.Len(t, tool.Stdio.Args, 2)
		assert.Equal(t, 15*time.Second, tool.StartupTimeout)
		assert.Equal(t, 45*time.Second, tool.RequestTimeout)
		assert.Equal(t, 10*time.Minute, tool.IdleTimeout)
		assert.Equal(t, runtime.ToolStateEnabled, tool.State)
	})

	t.Run("http tool with zero values uses defaults", func(t *testing.T) {
		cfg := ToolConfig{
			ID:        runtime.ToolID("http-tool"),
			Transport: runtime.TransportHTTP,
			URL:       "http://localhost:8080",
		}
		tool := ToolConfigToTool(cfg, defaults)
		assert.Equal(t, runtime.ToolID("http-tool"), tool.ID)
		assert.Equal(t, runtime.TransportHTTP, tool.Transport)
		require.NotNil(t, tool.HTTP)
		assert.Equal(t, "http://localhost:8080", tool.HTTP.URL)
		assert.Equal(t, DefaultStartupTimeout, tool.StartupTimeout)
		assert.Equal(t, DefaultRequestTimeout, tool.RequestTimeout)
		assert.Equal(t, DefaultSessionIdleTimeout, tool.IdleTimeout)
		assert.Equal(t, runtime.ToolStateEnabled, tool.State)
	})
}

func TestFullConfig(t *testing.T) {
	cfg := Config{
		HTTP: HTTPConfig{ListenAddress: DefaultHTTPListenAddress},
		Runtime: RuntimeConfig{
			SessionIdleTimeout: DefaultSessionIdleTimeout,
			RequestTimeout:     DefaultRequestTimeout,
			StartupTimeout:     DefaultStartupTimeout,
		},
		Cluster: ClusterConfig{
			Enabled:       true,
			Discovery:     "kubernetes",
			SingletonRole: "control-plane",
			Kubernetes: &KubernetesDiscoveryConfig{
				Namespace:         "default",
				DiscoveryPortName: "gossip",
				RemotingPortName:  "remoting",
				PeersPortName:     "cluster",
				PodLabels:         map[string]string{"app": "goakt-mcp"},
			},
		},
		Telemetry:   TelemetryConfig{OTLPEndpoint: "http://otel-collector:4318"},
		Audit:       AuditConfig{Backend: "s3", Bucket: "goakt-mcp-audit"},
		Credentials: CredentialsConfig{Providers: []string{"env", "vault"}},
		Tenants: []TenantConfig{
			{ID: "acme-dev", Quotas: TenantQuotaConfig{RequestsPerMinute: 1000, ConcurrentSessions: 200}},
		},
		Tools: []ToolConfig{
			{ID: "filesystem", Transport: runtime.TransportStdio, Command: "npx"},
		},
	}

	assert.Equal(t, ":8080", cfg.HTTP.ListenAddress)
	assert.True(t, cfg.Cluster.Enabled)
	require.Len(t, cfg.Tenants, 1)
	require.Len(t, cfg.Tools, 1)
	assert.Equal(t, runtime.ToolID("filesystem"), cfg.Tools[0].ID)
}
