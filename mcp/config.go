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

package mcp

import (
	"net/http"
	"time"
)

// Default configuration values applied when fields are not explicitly set.
const (
	DefaultSessionIdleTimeout  = 5 * time.Minute
	DefaultRequestTimeout      = 30 * time.Second
	DefaultStartupTimeout      = 10 * time.Second
	DefaultHealthProbeInterval = 30 * time.Second
	DefaultHealthProbeTimeout  = 10 * time.Second
	DefaultShutdownTimeout     = 30 * time.Second
	DefaultMaxCacheEntries     = 1000
	DefaultAuditMailboxSize    = 1024
)

// Config is the root configuration for the goakt-mcp gateway.
type Config struct {
	// LogLevel sets the gateway-wide logging verbosity.
	// Accepted values: "debug", "info", "warning", "error", "fatal", "panic".
	// When empty, the default (info) is used.
	LogLevel string

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

	// HealthProbe configures health probe settings.
	HealthProbe HealthProbeConfig

	// Tools holds the tool definitions to register at startup.
	Tools []Tool
}

// RuntimeConfig holds core runtime tuning parameters.
type RuntimeConfig struct {
	// SessionIdleTimeout is how long the router waits before passivating an
	// idle tool session. Zero means use DefaultSessionIdleTimeout.
	SessionIdleTimeout time.Duration
	// RequestTimeout is the maximum wall-clock time allowed for a single tool
	// invocation, including policy evaluation and egress round-trip.
	// Zero means use DefaultRequestTimeout.
	RequestTimeout time.Duration
	// StartupTimeout is the maximum time the gateway waits for a backend MCP
	// server process to become ready after it is spawned.
	// Zero means use DefaultStartupTimeout.
	StartupTimeout time.Duration
	// HealthProbeInterval is how often the health actor probes tool supervisors.
	// Zero means use DefaultHealthProbeInterval.
	HealthProbeInterval time.Duration
	// ShutdownTimeout is the maximum time Stop waits for the actor system to
	// drain cleanly before returning an error.
	// Zero means use DefaultShutdownTimeout.
	ShutdownTimeout time.Duration
}

// ClusterConfig holds multi-node operation settings.
//
// Supported discovery backends:
//   - "kubernetes": for production and cloud platforms (in-cluster pod discovery)
//   - "dnssd": for local development (DNS-based service discovery)
type ClusterConfig struct {
	// Enabled activates multi-node cluster mode. When false the gateway runs
	// in single-node mode with no distributed coordination.
	Enabled bool
	// Discovery selects the peer discovery backend: "kubernetes" or "dnssd".
	// Required when Enabled is true.
	Discovery string
	// SingletonRole is the cluster role name used to elect the singleton
	// GatewayManager (registrar, router, policy). Only one node in the cluster
	// holds this role at a time.
	SingletonRole string
	// PeersPort is the TCP port used for memberlist (gossip) communication
	// between cluster nodes.
	PeersPort int
	// RemotingPort is the TCP port used for GoAkt remoting (actor message
	// passing) between cluster nodes.
	RemotingPort int
	// TLS configures TLS for remoting and cluster communication.
	// When set, both the remoting server and client use TLS; cluster memberlist
	// and remoting traffic are encrypted. All nodes must share the same root CA.
	TLS        *RemotingTLSConfig
	Kubernetes KubernetesDiscoveryConfig
	DNSSD      DNSSDDiscoveryConfig
}

// RemotingTLSConfig holds TLS settings for GoAkt remoting and cluster.
//
// Server identity: CertFile and KeyFile are required when TLS is enabled.
// Client verification: CACertFile is used to verify remote servers; omit only
// when InsecureSkipVerify is true (dev/testing only).
// Mutual TLS: set ClientCAFile so the server validates client certs; set
// ClientCertFile and ClientKeyFile so the client presents a cert to remotes.
type RemotingTLSConfig struct {
	// CertFile and KeyFile are the server certificate and private key.
	CertFile string
	KeyFile  string
	// ClientCAFile, when non-empty, enables mTLS: server requires client certs
	// signed by this CA.
	ClientCAFile string
	// CACertFile is the CA used to verify remote server certificates.
	CACertFile string
	// ClientCertFile and ClientKeyFile, when both set, present a client cert
	// to remote nodes (mTLS).
	ClientCertFile string
	ClientKeyFile  string
	// InsecureSkipVerify skips server cert verification. Use only for dev/testing.
	InsecureSkipVerify bool
}

// KubernetesDiscoveryConfig holds settings for Kubernetes pod discovery.
type KubernetesDiscoveryConfig struct {
	Namespace         string
	DiscoveryPortName string
	RemotingPortName  string
	PeersPortName     string
	PodLabels         map[string]string
}

// DNSSDDiscoveryConfig holds settings for DNS-based service discovery.
type DNSSDDiscoveryConfig struct {
	DomainName string
	IPv6       bool
}

// TelemetryConfig holds OpenTelemetry export settings.
type TelemetryConfig struct {
	OTLPEndpoint string
}

// AuditConfig holds audit sink settings.
type AuditConfig struct {
	// Sink is the audit sink to use.
	Sink AuditSink
	// MailboxSize is the maximum number of audit events that can be queued in the
	// journal actor's mailbox. When the mailbox is full, senders block until space
	// is available, providing backpressure. Zero means use DefaultAuditMailboxSize.
	MailboxSize int
}

// CredentialsConfig holds configuration for secret provider backends.
type CredentialsConfig struct {
	// Providers holds the list of credentials providers.
	Providers []CredentialsProvider
	// CacheTTL is the time to live for credentials cache entries. Zero means
	// credentials are not cached and fetched on every invocation.
	CacheTTL time.Duration
	// MaxCacheEntries is the maximum number of entries in the credential cache.
	// When the cache exceeds this limit, expired entries are evicted first,
	// then the least-recently-accessed entry is removed (LRU). Zero means use
	// DefaultMaxCacheEntries.
	MaxCacheEntries int
}

// TenantConfig defines per-tenant settings including quota limits and optional
// custom policy evaluation.
type TenantConfig struct {
	// ID is the identifier for the tenant.
	ID TenantID
	// Quotas is the usage quota limits for the tenant.
	Quotas TenantQuotaConfig
	// Evaluator is an optional custom policy evaluator for this tenant.
	// When set, it is called after all built-in authorization and quota checks
	// pass. Returning a non-nil *RuntimeError from Evaluate denies the invocation.
	// When nil, only the built-in checks apply.
	Evaluator PolicyEvaluator
}

// TenantQuotaConfig defines the usage quota limits for a single tenant.
type TenantQuotaConfig struct {
	// RequestsPerMinute is the maximum number of requests per minute for the tenant.
	RequestsPerMinute int
	// ConcurrentSessions is the maximum number of concurrent sessions for the tenant.
	ConcurrentSessions int
}

// HealthProbeConfig holds health probe settings.
type HealthProbeConfig struct {
	// Interval is the interval between health probes.
	Interval time.Duration
	// Timeout is the maximum duration for a single probe cycle.
	// When zero, DefaultHealthProbeTimeout is used.
	Timeout time.Duration
}

// WSConfig holds WebSocket-specific configuration for [Gateway.WSHandler].
// All zero values use built-in defaults.
type WSConfig struct {
	// ReadBufferSize specifies the I/O buffer size in bytes for reading
	// WebSocket frames. Zero uses the default (4096).
	ReadBufferSize int

	// WriteBufferSize specifies the I/O buffer size in bytes for writing
	// WebSocket frames. Zero uses the default (4096).
	WriteBufferSize int

	// PingInterval is how often the server sends WebSocket ping frames to
	// keep the connection alive. Zero uses the default (30s).
	PingInterval time.Duration

	// CheckOrigin is an optional function that returns true if the request
	// origin is acceptable. When nil, any origin is accepted.
	CheckOrigin func(r *http.Request) bool
}
