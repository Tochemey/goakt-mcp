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

import "time"

// Default configuration values applied when fields are not explicitly set.
const (
	DefaultSessionIdleTimeout  = 5 * time.Minute
	DefaultRequestTimeout      = 30 * time.Second
	DefaultStartupTimeout      = 10 * time.Second
	DefaultHealthProbeInterval = 30 * time.Second
	DefaultShutdownTimeout     = 30 * time.Second
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

	// Tools holds the tool definitions to register at startup.
	Tools []Tool
}

// RuntimeConfig holds core runtime tuning parameters.
type RuntimeConfig struct {
	SessionIdleTimeout  time.Duration
	RequestTimeout      time.Duration
	StartupTimeout      time.Duration
	HealthProbeInterval time.Duration
	ShutdownTimeout     time.Duration
}

// ClusterConfig holds multi-node operation settings.
//
// Supported discovery backends:
//   - "kubernetes": for production and cloud platforms (in-cluster pod discovery)
//   - "dnssd": for local development (DNS-based service discovery)
type ClusterConfig struct {
	Enabled       bool
	Discovery     string
	SingletonRole string
	PeersPort     int
	RemotingPort  int
	Kubernetes    *KubernetesDiscoveryConfig
	DNSSD         *DNSSDDiscoveryConfig
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
	Backend string
	Bucket  string
}

// CredentialsConfig holds configuration for secret provider backends.
type CredentialsConfig struct {
	Providers []string
}

// TenantConfig defines per-tenant settings including quota limits.
type TenantConfig struct {
	ID     TenantID
	Quotas TenantQuotaConfig
}

// TenantQuotaConfig defines the usage quota limits for a single tenant.
type TenantQuotaConfig struct {
	RequestsPerMinute  int
	ConcurrentSessions int
}
