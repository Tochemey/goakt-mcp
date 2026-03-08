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
	"fmt"
	"os"
	"strings"
	"time"

	goaktlog "github.com/tochemey/goakt/v4/log"
	"gopkg.in/yaml.v3"

	"github.com/tochemey/goakt-mcp/internal/runtime"
)

// LoadFile reads and parses a YAML configuration file, applies defaults, and validates.
// Returns an error if the file cannot be read, parsed, or fails validation.
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	return Load(data)
}

// Load parses YAML configuration from data, applies defaults, and validates.
func Load(data []byte) (*Config, error) {
	var raw loadableConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	config, err := raw.toConfig()
	if err != nil {
		return nil, err
	}

	ApplyDefaults(&config)
	if err := Validate(&config); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &config, nil
}

// loadableConfig mirrors Config with YAML tags for snake_case keys.
type loadableConfig struct {
	LogLevel    string                    `yaml:"log_level"`
	HTTP        loadableHTTPConfig        `yaml:"http"`
	Runtime     loadableRuntimeConfig     `yaml:"runtime"`
	Cluster     loadableClusterConfig     `yaml:"cluster"`
	Telemetry   loadableTelemetryConfig   `yaml:"telemetry"`
	Audit       loadableAuditConfig       `yaml:"audit"`
	Credentials loadableCredentialsConfig `yaml:"credentials"`
	Tenants     []loadableTenantConfig    `yaml:"tenants"`
	Tools       []loadableToolConfig      `yaml:"tools"`
}

// loadableHTTPConfig is the YAML-tagged mirror of HTTPConfig.
type loadableHTTPConfig struct {
	ListenAddress string              `yaml:"listen_address"`
	TLS           *loadableIngressTLS `yaml:"tls"`
}

// loadableIngressTLS is the YAML-tagged mirror of IngressTLSConfig.
type loadableIngressTLS struct {
	CertFile     string `yaml:"cert_file"`
	KeyFile      string `yaml:"key_file"`
	ClientCAFile string `yaml:"client_ca_file"`
}

// loadableRuntimeConfig is the YAML-tagged mirror of RuntimeConfig.
type loadableRuntimeConfig struct {
	SessionIdleTimeout  duration `yaml:"session_idle_timeout"`
	RequestTimeout      duration `yaml:"request_timeout"`
	StartupTimeout      duration `yaml:"startup_timeout"`
	HealthProbeInterval duration `yaml:"health_probe_interval"`
	ShutdownTimeout     duration `yaml:"shutdown_timeout"`
}

// loadableClusterConfig is the YAML-tagged mirror of ClusterConfig.
type loadableClusterConfig struct {
	Enabled       bool                         `yaml:"enabled"`
	Discovery     string                       `yaml:"discovery"`
	SingletonRole string                       `yaml:"singleton_role"`
	PeersPort     int                          `yaml:"peers_port"`
	RemotingPort  int                          `yaml:"remoting_port"`
	Kubernetes    *loadableKubernetesDiscovery `yaml:"kubernetes"`
	DNSSD         *loadableDNSSDDiscovery      `yaml:"dnssd"`
}

// loadableKubernetesDiscovery is the YAML-tagged mirror of KubernetesDiscoveryConfig.
type loadableKubernetesDiscovery struct {
	Namespace         string            `yaml:"namespace"`
	DiscoveryPortName string            `yaml:"discovery_port_name"`
	RemotingPortName  string            `yaml:"remoting_port_name"`
	PeersPortName     string            `yaml:"peers_port_name"`
	PodLabels         map[string]string `yaml:"pod_labels"`
}

// loadableDNSSDDiscovery is the YAML-tagged mirror of DNSSDDiscoveryConfig.
type loadableDNSSDDiscovery struct {
	DomainName string `yaml:"domain_name"`
	IPv6       bool   `yaml:"ipv6"`
}

// loadableTelemetryConfig is the YAML-tagged mirror of TelemetryConfig.
type loadableTelemetryConfig struct {
	OTLPEndpoint string `yaml:"otlp_endpoint"`
}

// loadableAuditConfig is the YAML-tagged mirror of AuditConfig.
type loadableAuditConfig struct {
	Backend string `yaml:"backend"`
	Bucket  string `yaml:"bucket"`
}

// loadableCredentialsConfig is the YAML-tagged mirror of CredentialsConfig.
type loadableCredentialsConfig struct {
	Providers []string `yaml:"providers"`
}

// loadableTenantConfig is the YAML-tagged mirror of TenantConfig.
type loadableTenantConfig struct {
	ID     string              `yaml:"id"`
	Quotas loadableTenantQuota `yaml:"quotas"`
}

// loadableTenantQuota is the YAML-tagged mirror of TenantQuotaConfig.
type loadableTenantQuota struct {
	RequestsPerMinute  int `yaml:"requests_per_minute"`
	ConcurrentSessions int `yaml:"concurrent_sessions"`
}

// loadableToolConfig is the YAML-tagged mirror of ToolConfig.
type loadableToolConfig struct {
	ID                  string             `yaml:"id"`
	Transport           string             `yaml:"transport"`
	Command             string             `yaml:"command"`
	Args                []string           `yaml:"args"`
	Env                 map[string]string  `yaml:"env"`
	WorkingDirectory    string             `yaml:"working_directory"`
	URL                 string             `yaml:"url"`
	HTTPTLS             *loadableEgressTLS `yaml:"http_tls"`
	StartupTimeout      duration           `yaml:"startup_timeout"`
	RequestTimeout      duration           `yaml:"request_timeout"`
	IdleTimeout         duration           `yaml:"idle_timeout"`
	Routing             string             `yaml:"routing"`
	CredentialPolicy    string             `yaml:"credential_policy"`
	AuthorizationPolicy string             `yaml:"authorization_policy"`
}

// loadableEgressTLS is the YAML-tagged mirror of runtime.EgressTLSConfig.
type loadableEgressTLS struct {
	CACertFile         string `yaml:"ca_cert_file"`
	ClientCertFile     string `yaml:"client_cert_file"`
	ClientKeyFile      string `yaml:"client_key_file"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

// duration wraps time.Duration for YAML unmarshaling of strings like "5m", "30s".
type duration struct {
	value int64
}

// UnmarshalYAML decodes a YAML value into a duration. It accepts Go duration
// strings (e.g. "5m", "30s") and integer nanosecond values.
func (x *duration) UnmarshalYAML(unmarshal func(any) error) error {
	var v any
	if err := unmarshal(&v); err != nil {
		return err
	}
	switch typ := v.(type) {
	case string:
		dur, err := time.ParseDuration(typ)
		if err != nil {
			return err
		}
		x.value = int64(dur)
		return nil
	case int:
		x.value = int64(typ)
		return nil
	case int64:
		x.value = typ
		return nil
	default:
		return fmt.Errorf("duration: unsupported type %T", v)
	}
}

// Duration returns the parsed duration as a time.Duration.
func (x *duration) Duration() time.Duration { return time.Duration(x.value) }

// toConfig converts the YAML-tagged loadable representation into the canonical
// Config type. Enum string fields are parsed and validated during conversion.
func (x loadableConfig) toConfig() (Config, error) {
	logLevel, err := parseLogLevel(x.LogLevel)
	if err != nil {
		return Config{}, err
	}

	config := Config{
		LogLevel: logLevel,
		HTTP: HTTPConfig{
			ListenAddress: x.HTTP.ListenAddress,
			TLS:           loadableTLSToIngress(x.HTTP.TLS),
		},
		Runtime: RuntimeConfig{
			SessionIdleTimeout:  x.Runtime.SessionIdleTimeout.Duration(),
			RequestTimeout:      x.Runtime.RequestTimeout.Duration(),
			StartupTimeout:      x.Runtime.StartupTimeout.Duration(),
			HealthProbeInterval: x.Runtime.HealthProbeInterval.Duration(),
			ShutdownTimeout:     x.Runtime.ShutdownTimeout.Duration(),
		},
		Cluster: ClusterConfig{
			Enabled:       x.Cluster.Enabled,
			Discovery:     x.Cluster.Discovery,
			SingletonRole: x.Cluster.SingletonRole,
			PeersPort:     x.Cluster.PeersPort,
			RemotingPort:  x.Cluster.RemotingPort,
			Kubernetes:    loadableK8sToRuntime(x.Cluster.Kubernetes),
			DNSSD:         loadableDNSSDToRuntime(x.Cluster.DNSSD),
		},
		Telemetry:   TelemetryConfig{OTLPEndpoint: x.Telemetry.OTLPEndpoint},
		Audit:       AuditConfig{Backend: x.Audit.Backend, Bucket: x.Audit.Bucket},
		Credentials: CredentialsConfig{Providers: x.Credentials.Providers},
		Tenants:     make([]TenantConfig, len(x.Tenants)),
		Tools:       make([]ToolConfig, len(x.Tools)),
	}
	for i, t := range x.Tenants {
		config.Tenants[i] = TenantConfig{
			ID: runtime.TenantID(t.ID),
			Quotas: TenantQuotaConfig{
				RequestsPerMinute:  t.Quotas.RequestsPerMinute,
				ConcurrentSessions: t.Quotas.ConcurrentSessions,
			},
		}
	}
	var errs []error
	for i, t := range x.Tools {
		tc, err := loadableToolToConfig(t)
		if err != nil {
			errs = append(errs, fmt.Errorf("tools[%d] %q: %w", i, t.ID, err))
		}
		config.Tools[i] = tc
	}
	if len(errs) > 0 {
		return config, fmt.Errorf("config parse errors: %v", errs)
	}
	return config, nil
}

// loadableTLSToIngress converts a loadable TLS block into an IngressTLSConfig.
// Returns nil when the block is absent or both cert and key are empty.
func loadableTLSToIngress(t *loadableIngressTLS) *IngressTLSConfig {
	if t == nil || (t.CertFile == "" && t.KeyFile == "") {
		return nil
	}
	return &IngressTLSConfig{CertFile: t.CertFile, KeyFile: t.KeyFile, ClientCAFile: t.ClientCAFile}
}

// loadableK8sToRuntime converts a loadable Kubernetes discovery block into a
// KubernetesDiscoveryConfig. Returns nil when the block is absent.
func loadableK8sToRuntime(k *loadableKubernetesDiscovery) *KubernetesDiscoveryConfig {
	if k == nil {
		return nil
	}
	return &KubernetesDiscoveryConfig{
		Namespace:         k.Namespace,
		DiscoveryPortName: k.DiscoveryPortName,
		RemotingPortName:  k.RemotingPortName,
		PeersPortName:     k.PeersPortName,
		PodLabels:         k.PodLabels,
	}
}

// loadableDNSSDToRuntime converts a loadable DNS-SD discovery block into a
// DNSSDDiscoveryConfig. Returns nil when the block is absent.
func loadableDNSSDToRuntime(d *loadableDNSSDDiscovery) *DNSSDDiscoveryConfig {
	if d == nil {
		return nil
	}
	return &DNSSDDiscoveryConfig{DomainName: d.DomainName, IPv6: d.IPv6}
}

// loadableToolToConfig converts a single loadable tool definition into a
// ToolConfig, parsing transport, routing, and policy enum strings. Returns an
// error when any enum value is unrecognized.
func loadableToolToConfig(t loadableToolConfig) (ToolConfig, error) {
	transport, err := parseTransport(t.Transport)
	if err != nil {
		return ToolConfig{}, err
	}
	routing, err := parseRouting(t.Routing)
	if err != nil {
		return ToolConfig{}, err
	}
	credPolicy, err := parseCredentialPolicy(t.CredentialPolicy)
	if err != nil {
		return ToolConfig{}, err
	}
	authPolicy, err := parseAuthorizationPolicy(t.AuthorizationPolicy)
	if err != nil {
		return ToolConfig{}, err
	}
	tc := ToolConfig{
		ID:                  runtime.ToolID(t.ID),
		Transport:           transport,
		Command:             t.Command,
		Args:                t.Args,
		Env:                 t.Env,
		WorkingDirectory:    t.WorkingDirectory,
		URL:                 t.URL,
		StartupTimeout:      t.StartupTimeout.Duration(),
		RequestTimeout:      t.RequestTimeout.Duration(),
		IdleTimeout:         t.IdleTimeout.Duration(),
		Routing:             routing,
		CredentialPolicy:    credPolicy,
		AuthorizationPolicy: authPolicy,
	}
	if t.HTTPTLS != nil {
		tc.HTTPTLS = &runtime.EgressTLSConfig{
			CACertFile:         t.HTTPTLS.CACertFile,
			ClientCertFile:     t.HTTPTLS.ClientCertFile,
			ClientKeyFile:      t.HTTPTLS.ClientKeyFile,
			InsecureSkipVerify: t.HTTPTLS.InsecureSkipVerify,
		}
	}
	return tc, nil
}

// parseTransport converts a YAML transport string ("stdio", "http") into a
// runtime.TransportType. An empty string defaults to stdio.
func parseTransport(s string) (runtime.TransportType, error) {
	switch strings.ToLower(s) {
	case "http":
		return runtime.TransportHTTP, nil
	case "stdio":
		return runtime.TransportStdio, nil
	case "":
		return runtime.TransportStdio, nil
	default:
		return "", fmt.Errorf("unknown transport %q (expected \"stdio\" or \"http\")", s)
	}
}

// parseRouting converts a YAML routing string ("sticky", "least_loaded") into
// a runtime.RoutingMode. An empty string defaults to sticky.
func parseRouting(s string) (runtime.RoutingMode, error) {
	switch strings.ToLower(s) {
	case "least_loaded":
		return runtime.RoutingLeastLoaded, nil
	case "sticky":
		return runtime.RoutingSticky, nil
	case "":
		return runtime.RoutingSticky, nil
	default:
		return "", fmt.Errorf("unknown routing %q (expected \"sticky\" or \"least_loaded\")", s)
	}
}

// parseCredentialPolicy converts a YAML credential_policy string ("optional",
// "required") into a runtime.CredentialPolicy. An empty string defaults to optional.
func parseCredentialPolicy(s string) (runtime.CredentialPolicy, error) {
	switch strings.ToLower(s) {
	case "required":
		return runtime.CredentialPolicyRequired, nil
	case "optional":
		return runtime.CredentialPolicyOptional, nil
	case "":
		return runtime.CredentialPolicyOptional, nil
	default:
		return "", fmt.Errorf("unknown credential_policy %q (expected \"optional\" or \"required\")", s)
	}
}

// parseAuthorizationPolicy converts a YAML authorization_policy string
// ("tenant_allowlist") into a runtime.AuthorizationPolicy. An empty string
// defaults to tenant_allowlist.
func parseAuthorizationPolicy(s string) (runtime.AuthorizationPolicy, error) {
	switch strings.ToLower(s) {
	case "tenant_allowlist":
		return runtime.AuthorizationPolicyTenantAllowlist, nil
	case "":
		return runtime.AuthorizationPolicyTenantAllowlist, nil
	default:
		return "", fmt.Errorf("unknown authorization_policy %q (expected \"tenant_allowlist\")", s)
	}
}

// parseLogLevel converts a YAML log_level string into a goaktlog.Level.
// An empty string returns goaktlog.InvalidLevel so that callers can
// distinguish "not set" from an explicit level and fall back to the GoAkt
// DefaultLogger.
func parseLogLevel(s string) (goaktlog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return goaktlog.DebugLevel, nil
	case "info":
		return goaktlog.InfoLevel, nil
	case "warning", "warn":
		return goaktlog.WarningLevel, nil
	case "error":
		return goaktlog.ErrorLevel, nil
	case "fatal":
		return goaktlog.FatalLevel, nil
	case "panic":
		return goaktlog.PanicLevel, nil
	case "":
		return goaktlog.InvalidLevel, nil
	default:
		return goaktlog.InvalidLevel, fmt.Errorf("unknown log_level %q (expected \"debug\", \"info\", \"warning\", \"error\", \"fatal\", or \"panic\")", s)
	}
}
