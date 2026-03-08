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

// Package config provides runtime and tool configuration types for the goakt-mcp
// gateway. It defines the configuration model for runtime settings, tool definitions,
// tenancy, policies, credentials, telemetry, and clustering.
//
// Configuration is loaded at startup and treated as immutable input to the runtime
// after the actor system has been bootstrapped.
package config

import (
	"os"
	"time"

	goaktlog "github.com/tochemey/goakt/v4/log"

	"github.com/tochemey/goakt-mcp/internal/runtime"
)

// Default configuration values applied when fields are not explicitly set.
const (
	// DefaultSessionIdleTimeout is the default idle duration before a session is passivated.
	DefaultSessionIdleTimeout = 5 * time.Minute

	// DefaultRequestTimeout is the default maximum duration for a single tool invocation.
	DefaultRequestTimeout = 30 * time.Second

	// DefaultStartupTimeout is the default maximum duration for a tool to become ready.
	DefaultStartupTimeout = 10 * time.Second

	// DefaultHealthProbeInterval is the default interval between health probe runs.
	DefaultHealthProbeInterval = 30 * time.Second

	// DefaultHTTPListenAddress is the default address the HTTP server binds to.
	DefaultHTTPListenAddress = ":8080"

	// DefaultShutdownTimeout is the default maximum duration granted to the
	// gateway for a graceful shutdown before the process is terminated.
	DefaultShutdownTimeout = 30 * time.Second
)

// Config is the root configuration for the goakt-mcp gateway.
//
// It holds all settings needed to start and operate the gateway, including
// HTTP listener configuration, runtime tuning parameters, cluster settings,
// telemetry export, audit sinks, credential providers, tenant quotas, and
// tool definitions.
type Config struct {
	// LogLevel sets the gateway-wide logging verbosity. When unset (the zero
	// value InvalidLevel) the GoAkt DefaultLogger (InfoLevel) is used.
	// Accepted values: DebugLevel, InfoLevel, WarningLevel, ErrorLevel,
	// FatalLevel, PanicLevel.
	LogLevel goaktlog.Level

	// HTTP configures the inbound HTTP server.
	HTTP HTTPConfig

	// Runtime configures core runtime tuning parameters.
	Runtime RuntimeConfig

	// Cluster configures multi-node operation. When Cluster.Enabled is false
	// the gateway runs in single-node mode with no distributed coordination.
	Cluster ClusterConfig

	// Telemetry configures OpenTelemetry export.
	Telemetry TelemetryConfig

	// Audit configures the durable audit sink.
	Audit AuditConfig

	// Credentials configures secret provider backends.
	Credentials CredentialsConfig

	// Tenants holds per-tenant quota and policy configuration.
	Tenants []TenantConfig

	// Tools holds the static tool definitions loaded at startup.
	// Dynamic registration via the control plane is additive to this list.
	Tools []ToolConfig
}

// HTTPConfig holds listener configuration for the inbound HTTP server.
type HTTPConfig struct {
	// ListenAddress is the network address the HTTP server binds to (e.g., ":8080").
	ListenAddress string

	// TLS configures TLS for the ingress server. When set, the server uses
	// ListenAndServeTLS with the given certificate and key. ClientCAFile
	// enables mutual TLS when non-empty.
	TLS *IngressTLSConfig
}

// IngressTLSConfig holds TLS settings for the ingress HTTP server.
type IngressTLSConfig struct {
	// CertFile is the path to the server certificate (PEM).
	CertFile string

	// KeyFile is the path to the server private key (PEM).
	KeyFile string

	// ClientCAFile is the path to a CA certificate (PEM) for verifying client
	// certificates. When set, mutual TLS is enabled.
	ClientCAFile string
}

// RuntimeConfig holds core runtime tuning parameters.
//
// These values govern session passivation, per-request deadlines, and tool
// startup behaviour. They apply globally unless overridden at the tool level.
type RuntimeConfig struct {
	// SessionIdleTimeout is the duration after which an idle session is passivated.
	// Defaults to DefaultSessionIdleTimeout when zero.
	SessionIdleTimeout time.Duration

	// RequestTimeout is the maximum time allowed for a single MCP invocation.
	// Defaults to DefaultRequestTimeout when zero.
	RequestTimeout time.Duration

	// StartupTimeout is the maximum time the runtime waits for a tool to become ready.
	// Defaults to DefaultStartupTimeout when zero.
	StartupTimeout time.Duration

	// HealthProbeInterval is the interval between health probe runs.
	// Defaults to DefaultHealthProbeInterval when zero.
	HealthProbeInterval time.Duration

	// ShutdownTimeout is the maximum time granted for a graceful shutdown.
	// Defaults to DefaultShutdownTimeout when zero.
	ShutdownTimeout time.Duration
}

// ClusterConfig holds multi-node operation settings.
//
// Supported discovery backends:
//   - "kubernetes": for production and cloud platforms (in-cluster pod discovery)
//   - "dnssd": for local development (DNS-based service discovery)
type ClusterConfig struct {
	// Enabled controls whether the gateway starts in cluster mode.
	// When false the gateway runs as a single node.
	Enabled bool

	// Discovery names the cluster discovery backend: "kubernetes" (production) or "dnssd" (local).
	Discovery string

	// SingletonRole is the role name used to elect the cluster singleton coordinator.
	SingletonRole string

	// PeersPort is the gossip port for cluster membership.
	// Default 15000 when zero.
	PeersPort int

	// RemotingPort is the port for remoting. Used when cluster is enabled.
	// Default 15001 when zero.
	RemotingPort int

	// Kubernetes configures Kubernetes discovery. Required when Discovery is "kubernetes".
	Kubernetes *KubernetesDiscoveryConfig

	// DNSSD configures DNS-SD discovery. Required when Discovery is "dnssd".
	DNSSD *DNSSDDiscoveryConfig
}

// KubernetesDiscoveryConfig holds settings for Kubernetes pod discovery.
// Used when running inside a Kubernetes cluster.
type KubernetesDiscoveryConfig struct {
	// Namespace is the Kubernetes namespace to list pods in.
	Namespace string

	// DiscoveryPortName is the container port name for gossip (e.g. "gossip").
	DiscoveryPortName string

	// RemotingPortName is the container port name for remoting (e.g. "remoting").
	RemotingPortName string

	// PeersPortName is the container port name for cluster (e.g. "cluster").
	PeersPortName string

	// PodLabels matches pods by labels (e.g. map[string]string{"app": "goakt-mcp"}).
	PodLabels map[string]string
}

// DNSSDDiscoveryConfig holds settings for DNS-based service discovery.
// Used for local development (e.g. with Kubernetes headless service or mDNS).
type DNSSDDiscoveryConfig struct {
	// DomainName is the DNS name to resolve for peer addresses (e.g. "goakt-mcp.default.svc.cluster.local").
	DomainName string

	// IPv6, when true, resolves IPv6 addresses only. Default false resolves all.
	IPv6 bool
}

// TelemetryConfig holds OpenTelemetry export settings.
type TelemetryConfig struct {
	// OTLPEndpoint is the OTLP collector endpoint (e.g., "http://otel-collector:4318").
	// When empty, telemetry export is disabled.
	OTLPEndpoint string
}

// AuditConfig holds audit sink settings.
type AuditConfig struct {
	// Backend names the audit storage backend (e.g., "s3", "file").
	Backend string

	// Bucket is the S3-compatible bucket name used when Backend is "s3".
	Bucket string
}

// CredentialsConfig holds configuration for secret provider backends.
type CredentialsConfig struct {
	// Providers lists the credential backend types to enable, in preference order
	// (e.g., ["env", "vault"]). At least one provider must be listed when any tool
	// declares a required credential policy.
	Providers []string
}

// TenantConfig defines per-tenant settings including quota limits.
type TenantConfig struct {
	// ID is the unique identifier for this tenant.
	ID runtime.TenantID

	// Quotas defines the usage limits enforced for this tenant.
	Quotas TenantQuotaConfig
}

// TenantQuotaConfig defines the usage quota limits for a single tenant.
type TenantQuotaConfig struct {
	// RequestsPerMinute is the maximum number of tool invocations allowed per minute.
	// Zero means no limit is enforced.
	RequestsPerMinute int

	// ConcurrentSessions is the maximum number of live sessions this tenant may hold
	// simultaneously across all tools. Zero means no limit is enforced.
	ConcurrentSessions int
}

// ToolConfig defines the static configuration for a single registered tool.
//
// Fields are divided by transport: stdio fields are relevant when Transport is
// TransportStdio; HTTP fields are relevant when Transport is TransportHTTP.
// Timing and policy fields apply to both transports.
type ToolConfig struct {
	// ID is the unique identifier for this tool.
	ID runtime.ToolID

	// Transport determines the MCP execution transport.
	Transport runtime.TransportType

	// --- Stdio transport fields ---

	// Command is the executable to launch for stdio tools (e.g., "npx", "python").
	Command string

	// Args are the arguments passed to the command for stdio tools.
	Args []string

	// Env holds additional environment variables injected into the child process.
	Env map[string]string

	// WorkingDirectory is the working directory for the child process.
	WorkingDirectory string

	// --- HTTP transport fields ---

	// URL is the base URL of the remote MCP server for HTTP tools.
	URL string

	// HTTPTLS configures TLS for outbound HTTP connections to this tool.
	// When nil, the default system CA pool is used for https:// URLs.
	HTTPTLS *runtime.EgressTLSConfig

	// --- Timing configuration ---

	// StartupTimeout overrides the global startup timeout for this tool.
	// Zero inherits the runtime-level default.
	StartupTimeout time.Duration

	// RequestTimeout overrides the global request timeout for this tool.
	// Zero inherits the runtime-level default.
	RequestTimeout time.Duration

	// IdleTimeout overrides the global session idle timeout for this tool.
	// Zero inherits the runtime-level default.
	IdleTimeout time.Duration

	// --- Policy configuration ---

	// Routing defines the routing strategy for this tool.
	Routing runtime.RoutingMode

	// CredentialPolicy defines the credential requirement for this tool.
	CredentialPolicy runtime.CredentialPolicy

	// AuthorizationPolicy defines the authorization model for this tool.
	AuthorizationPolicy runtime.AuthorizationPolicy
}

// ToolConfigToTool converts a ToolConfig to a runtime.Tool, applying runtime defaults
// when tool-level values are zero.
//
// The returned Tool is registered in ToolStateEnabled. Call runtime.ValidateTool
// before registering if validation is required.
func ToolConfigToTool(toolConfig ToolConfig, defaults RuntimeConfig) runtime.Tool {
	tool := runtime.Tool{
		ID:                  toolConfig.ID,
		Transport:           toolConfig.Transport,
		StartupTimeout:      toolConfig.StartupTimeout,
		RequestTimeout:      toolConfig.RequestTimeout,
		IdleTimeout:         toolConfig.IdleTimeout,
		Routing:             toolConfig.Routing,
		CredentialPolicy:    toolConfig.CredentialPolicy,
		AuthorizationPolicy: toolConfig.AuthorizationPolicy,
		State:               runtime.ToolStateEnabled,
	}

	if toolConfig.StartupTimeout == 0 {
		tool.StartupTimeout = defaults.StartupTimeout
		if tool.StartupTimeout == 0 {
			tool.StartupTimeout = DefaultStartupTimeout
		}
	}

	if toolConfig.RequestTimeout == 0 {
		tool.RequestTimeout = defaults.RequestTimeout
		if tool.RequestTimeout == 0 {
			tool.RequestTimeout = DefaultRequestTimeout
		}
	}

	if toolConfig.IdleTimeout == 0 {
		tool.IdleTimeout = defaults.SessionIdleTimeout
		if tool.IdleTimeout == 0 {
			tool.IdleTimeout = DefaultSessionIdleTimeout
		}
	}

	switch toolConfig.Transport {
	case runtime.TransportStdio:
		tool.Stdio = &runtime.StdioTransportConfig{
			Command:          toolConfig.Command,
			Args:             toolConfig.Args,
			Env:              toolConfig.Env,
			WorkingDirectory: toolConfig.WorkingDirectory,
		}
	case runtime.TransportHTTP:
		tool.HTTP = &runtime.HTTPTransportConfig{
			URL: toolConfig.URL,
			TLS: toolConfig.HTTPTLS,
		}
	}

	return tool
}

// NewLogger creates a goaktlog.Logger based on the configured LogLevel.
// When the level is InvalidLevel (i.e. not set in config), the GoAkt
// DefaultLogger is returned, which logs at InfoLevel to stdout.
func NewLogger(level goaktlog.Level) goaktlog.Logger {
	if level == goaktlog.InvalidLevel {
		return goaktlog.DefaultLogger
	}
	return goaktlog.NewZap(level, os.Stdout)
}
