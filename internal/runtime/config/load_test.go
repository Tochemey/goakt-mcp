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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goaktlog "github.com/tochemey/goakt/v4/log"

	"github.com/tochemey/goakt-mcp/internal/runtime"
)

func TestLoad_MinimalYAML(t *testing.T) {
	data := `
http:
  listen_address: ":9090"
`
	cfg, err := Load([]byte(data))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, ":9090", cfg.HTTP.ListenAddress)
	assert.Equal(t, DefaultSessionIdleTimeout, cfg.Runtime.SessionIdleTimeout)
	assert.Equal(t, DefaultRequestTimeout, cfg.Runtime.RequestTimeout)
	assert.Equal(t, DefaultStartupTimeout, cfg.Runtime.StartupTimeout)
	assert.Equal(t, DefaultHealthProbeInterval, cfg.Runtime.HealthProbeInterval)
}

func TestLoad_EmptyFile(t *testing.T) {
	cfg, err := Load([]byte(""))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, DefaultHTTPListenAddress, cfg.HTTP.ListenAddress)
	assert.NotNil(t, cfg.Tools)
	assert.NotNil(t, cfg.Tenants)
}

func TestLoad_FullExample(t *testing.T) {
	data := `
tenants:
  - id: acme-dev
    quotas:
      requests_per_minute: 1000
      concurrent_sessions: 200
http:
  listen_address: ":8080"
runtime:
  session_idle_timeout: 5m
  request_timeout: 30s
  startup_timeout: 10s
  health_probe_interval: 1m
cluster:
  enabled: false
  discovery: ""
telemetry:
  otlp_endpoint: "http://otel:4318"
audit:
  backend: s3
  bucket: audit-bucket
credentials:
  providers: ["env", "vault"]
tools:
  - id: filesystem
    transport: stdio
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    routing: sticky
    credential_policy: optional
    authorization_policy: tenant_allowlist
  - id: docs-search
    transport: http
    url: "https://mcp.internal.example.com"
    request_timeout: 15s
    routing: least_loaded
    credential_policy: required
`
	cfg, err := Load([]byte(data))
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, ":8080", cfg.HTTP.ListenAddress)
	assert.Equal(t, 5*time.Minute, cfg.Runtime.SessionIdleTimeout)
	assert.Equal(t, 30*time.Second, cfg.Runtime.RequestTimeout)
	assert.Equal(t, 10*time.Second, cfg.Runtime.StartupTimeout)
	assert.Equal(t, 1*time.Minute, cfg.Runtime.HealthProbeInterval)

	assert.Equal(t, "http://otel:4318", cfg.Telemetry.OTLPEndpoint)
	assert.Equal(t, "s3", cfg.Audit.Backend)
	assert.Equal(t, "audit-bucket", cfg.Audit.Bucket)
	require.Len(t, cfg.Credentials.Providers, 2)
	assert.Equal(t, "env", cfg.Credentials.Providers[0])

	require.Len(t, cfg.Tenants, 1)
	assert.Equal(t, runtime.TenantID("acme-dev"), cfg.Tenants[0].ID)
	assert.Equal(t, 1000, cfg.Tenants[0].Quotas.RequestsPerMinute)
	assert.Equal(t, 200, cfg.Tenants[0].Quotas.ConcurrentSessions)

	require.Len(t, cfg.Tools, 2)
	assert.Equal(t, runtime.ToolID("filesystem"), cfg.Tools[0].ID)
	assert.Equal(t, runtime.TransportStdio, cfg.Tools[0].Transport)
	assert.Equal(t, "npx", cfg.Tools[0].Command)
	assert.Equal(t, runtime.RoutingSticky, cfg.Tools[0].Routing)
	assert.Equal(t, runtime.CredentialPolicyOptional, cfg.Tools[0].CredentialPolicy)
	assert.Equal(t, runtime.AuthorizationPolicyTenantAllowlist, cfg.Tools[0].AuthorizationPolicy)

	assert.Equal(t, runtime.ToolID("docs-search"), cfg.Tools[1].ID)
	assert.Equal(t, runtime.TransportHTTP, cfg.Tools[1].Transport)
	assert.Equal(t, "https://mcp.internal.example.com", cfg.Tools[1].URL)
	assert.Equal(t, 15*time.Second, cfg.Tools[1].RequestTimeout)
	assert.Equal(t, runtime.RoutingLeastLoaded, cfg.Tools[1].Routing)
	assert.Equal(t, runtime.CredentialPolicyRequired, cfg.Tools[1].CredentialPolicy)
}

func TestLoad_TLSRoundTrip(t *testing.T) {
	t.Run("ingress TLS", func(t *testing.T) {
		data := `
http:
  listen_address: ":8443"
  tls:
    cert_file: /etc/server.crt
    key_file: /etc/server.key
    client_ca_file: /etc/ca.crt
`
		cfg, err := Load([]byte(data))
		require.NoError(t, err)
		require.NotNil(t, cfg.HTTP.TLS)
		assert.Equal(t, "/etc/server.crt", cfg.HTTP.TLS.CertFile)
		assert.Equal(t, "/etc/server.key", cfg.HTTP.TLS.KeyFile)
		assert.Equal(t, "/etc/ca.crt", cfg.HTTP.TLS.ClientCAFile)
	})

	t.Run("egress TLS", func(t *testing.T) {
		data := `
tools:
  - id: secure-tool
    transport: http
    url: "https://mcp.example.com"
    http_tls:
      ca_cert_file: /etc/mcp-ca.crt
      client_cert_file: /etc/client.crt
      client_key_file: /etc/client.key
      insecure_skip_verify: false
`
		cfg, err := Load([]byte(data))
		require.NoError(t, err)
		require.Len(t, cfg.Tools, 1)
		require.NotNil(t, cfg.Tools[0].HTTPTLS)
		assert.Equal(t, "/etc/mcp-ca.crt", cfg.Tools[0].HTTPTLS.CACertFile)
		assert.Equal(t, "/etc/client.crt", cfg.Tools[0].HTTPTLS.ClientCertFile)
		assert.Equal(t, "/etc/client.key", cfg.Tools[0].HTTPTLS.ClientKeyFile)
		assert.False(t, cfg.Tools[0].HTTPTLS.InsecureSkipVerify)
	})

	t.Run("ingress TLS without client CA", func(t *testing.T) {
		data := `
http:
  listen_address: ":8443"
  tls:
    cert_file: /etc/server.crt
    key_file: /etc/server.key
`
		cfg, err := Load([]byte(data))
		require.NoError(t, err)
		require.NotNil(t, cfg.HTTP.TLS)
		assert.Empty(t, cfg.HTTP.TLS.ClientCAFile)
	})

	t.Run("egress TLS insecure skip verify", func(t *testing.T) {
		data := `
tools:
  - id: dev-tool
    transport: http
    url: "https://localhost:9999"
    http_tls:
      insecure_skip_verify: true
`
		cfg, err := Load([]byte(data))
		require.NoError(t, err)
		require.NotNil(t, cfg.Tools[0].HTTPTLS)
		assert.True(t, cfg.Tools[0].HTTPTLS.InsecureSkipVerify)
	})
}

func TestLoad_InvalidYAML(t *testing.T) {
	_, err := Load([]byte("invalid: [unclosed"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse config")
}

func TestLoad_InvalidEnumValues(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "unknown transport",
			yaml: `
tools:
  - id: t1
    transport: grpc
    command: test
`,
			want: "unknown transport",
		},
		{
			name: "unknown routing",
			yaml: `
tools:
  - id: t1
    transport: stdio
    command: test
    routing: random
`,
			want: "unknown routing",
		},
		{
			name: "unknown credential_policy",
			yaml: `
tools:
  - id: t1
    transport: stdio
    command: test
    credential_policy: always
`,
			want: "unknown credential_policy",
		},
		{
			name: "unknown authorization_policy",
			yaml: `
tools:
  - id: t1
    transport: stdio
    command: test
    authorization_policy: open
`,
			want: "unknown authorization_policy",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load([]byte(tt.yaml))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestLoad_ClusterConfig(t *testing.T) {
	t.Run("kubernetes", func(t *testing.T) {
		data := `
cluster:
  enabled: true
  discovery: kubernetes
  singleton_role: control-plane
  peers_port: 15000
  remoting_port: 15001
  kubernetes:
    namespace: default
    discovery_port_name: gossip
    remoting_port_name: remoting
    peers_port_name: cluster
    pod_labels:
      app: goakt-mcp
`
		cfg, err := Load([]byte(data))
		require.NoError(t, err)
		assert.True(t, cfg.Cluster.Enabled)
		assert.Equal(t, "kubernetes", cfg.Cluster.Discovery)
		assert.Equal(t, 15000, cfg.Cluster.PeersPort)
		require.NotNil(t, cfg.Cluster.Kubernetes)
		assert.Equal(t, "default", cfg.Cluster.Kubernetes.Namespace)
		assert.Equal(t, "gossip", cfg.Cluster.Kubernetes.DiscoveryPortName)
		assert.Equal(t, map[string]string{"app": "goakt-mcp"}, cfg.Cluster.Kubernetes.PodLabels)
	})

	t.Run("dnssd", func(t *testing.T) {
		data := `
cluster:
  enabled: true
  discovery: dnssd
  dnssd:
    domain_name: goakt-mcp.local
    ipv6: true
`
		cfg, err := Load([]byte(data))
		require.NoError(t, err)
		require.NotNil(t, cfg.Cluster.DNSSD)
		assert.Equal(t, "goakt-mcp.local", cfg.Cluster.DNSSD.DomainName)
		assert.True(t, cfg.Cluster.DNSSD.IPv6)
	})
}

func TestLoad_DurationParsing(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		wantDur time.Duration
	}{
		{
			name:    "string duration",
			yaml:    "runtime:\n  request_timeout: 45s",
			wantDur: 45 * time.Second,
		},
		{
			name:    "minute duration",
			yaml:    "runtime:\n  session_idle_timeout: 10m",
			wantDur: 10 * time.Minute,
		},
		{
			name:    "invalid duration string",
			yaml:    "runtime:\n  request_timeout: notaduration",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load([]byte(tt.yaml))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.wantDur > 0 {
				if cfg.Runtime.RequestTimeout == tt.wantDur || cfg.Runtime.SessionIdleTimeout == tt.wantDur {
					return
				}
				t.Errorf("expected duration %v somewhere in runtime config", tt.wantDur)
			}
		})
	}
}

func TestLoadFile(t *testing.T) {
	t.Run("nonexistent path", func(t *testing.T) {
		_, err := LoadFile("/nonexistent/config.yaml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read config")
	})

	t.Run("valid file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		data := []byte("http:\n  listen_address: \":9999\"\n")
		require.NoError(t, os.WriteFile(path, data, 0o600))

		cfg, err := LoadFile(path)
		require.NoError(t, err)
		assert.Equal(t, ":9999", cfg.HTTP.ListenAddress)
	})
}

func TestLoad_ShutdownTimeout(t *testing.T) {
	t.Run("explicit value is preserved", func(t *testing.T) {
		data := `
runtime:
  shutdown_timeout: 45s
`
		cfg, err := Load([]byte(data))
		require.NoError(t, err)
		assert.Equal(t, 45*time.Second, cfg.Runtime.ShutdownTimeout)
	})

	t.Run("omitted value gets default", func(t *testing.T) {
		cfg, err := Load([]byte("http:\n  listen_address: \":8080\"\n"))
		require.NoError(t, err)
		assert.Equal(t, DefaultShutdownTimeout, cfg.Runtime.ShutdownTimeout)
	})
}

func TestLoad_DefaultsOnOmittedTransport(t *testing.T) {
	data := `
tools:
  - id: minimal
    command: echo
`
	cfg, err := Load([]byte(data))
	require.NoError(t, err)
	assert.Equal(t, runtime.TransportStdio, cfg.Tools[0].Transport)
}

func TestLoad_DefaultsOnOmittedRouting(t *testing.T) {
	data := `
tools:
  - id: minimal
    transport: stdio
    command: echo
`
	cfg, err := Load([]byte(data))
	require.NoError(t, err)
	assert.Equal(t, runtime.RoutingSticky, cfg.Tools[0].Routing)
	assert.Equal(t, runtime.CredentialPolicyOptional, cfg.Tools[0].CredentialPolicy)
	assert.Equal(t, runtime.AuthorizationPolicyTenantAllowlist, cfg.Tools[0].AuthorizationPolicy)
}

func TestLoad_LogLevel(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantLevel goaktlog.Level
		wantErr   bool
	}{
		{
			name:      "omitted defaults to InvalidLevel",
			yaml:      "http:\n  listen_address: \":8080\"\n",
			wantLevel: goaktlog.InvalidLevel,
		},
		{
			name:      "debug",
			yaml:      "log_level: debug\n",
			wantLevel: goaktlog.DebugLevel,
		},
		{
			name:      "info",
			yaml:      "log_level: info\n",
			wantLevel: goaktlog.InfoLevel,
		},
		{
			name:      "warning",
			yaml:      "log_level: warning\n",
			wantLevel: goaktlog.WarningLevel,
		},
		{
			name:      "warn alias",
			yaml:      "log_level: warn\n",
			wantLevel: goaktlog.WarningLevel,
		},
		{
			name:      "error",
			yaml:      "log_level: error\n",
			wantLevel: goaktlog.ErrorLevel,
		},
		{
			name:      "fatal",
			yaml:      "log_level: fatal\n",
			wantLevel: goaktlog.FatalLevel,
		},
		{
			name:      "panic",
			yaml:      "log_level: panic\n",
			wantLevel: goaktlog.PanicLevel,
		},
		{
			name:      "case insensitive",
			yaml:      "log_level: DEBUG\n",
			wantLevel: goaktlog.DebugLevel,
		},
		{
			name:    "unknown value",
			yaml:    "log_level: verbose\n",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load([]byte(tt.yaml))
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unknown log_level")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantLevel, cfg.LogLevel)
		})
	}
}

func TestNewLogger(t *testing.T) {
	t.Run("InvalidLevel returns DefaultLogger", func(t *testing.T) {
		logger := NewLogger(goaktlog.InvalidLevel)
		assert.Equal(t, goaktlog.DefaultLogger, logger)
	})

	t.Run("explicit level returns non-nil logger", func(t *testing.T) {
		logger := NewLogger(goaktlog.DebugLevel)
		assert.NotNil(t, logger)
	})

	t.Run("InfoLevel returns non-nil logger", func(t *testing.T) {
		logger := NewLogger(goaktlog.InfoLevel)
		assert.NotNil(t, logger)
	})
}
