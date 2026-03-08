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

import "time"

// TransportType defines the underlying MCP transport protocol used by a tool.
type TransportType string

const (
	// TransportStdio indicates the tool is reached by launching a local child process
	// and communicating over stdin/stdout.
	TransportStdio TransportType = "stdio"

	// TransportHTTP indicates the tool is reached over an HTTP-based MCP endpoint.
	TransportHTTP TransportType = "http"
)

// RoutingMode defines how the runtime selects an execution target for a tool.
type RoutingMode string

const (
	// RoutingSticky pins a tenant+client combination to a single session for the
	// duration of the session lifetime. This is required for stateful MCP conversations.
	RoutingSticky RoutingMode = "sticky"

	// RoutingLeastLoaded routes each request to the least-loaded eligible target.
	// This is appropriate for stateless or pooled tools.
	RoutingLeastLoaded RoutingMode = "least_loaded"
)

// ToolState describes the current operational availability of a registered tool.
type ToolState string

const (
	// ToolStateEnabled means the tool is registered and accepting requests.
	ToolStateEnabled ToolState = "enabled"

	// ToolStateDisabled means the tool has been administratively disabled.
	// Requests are rejected without attempting execution.
	ToolStateDisabled ToolState = "disabled"

	// ToolStateDegraded means the tool is experiencing failures but may still
	// accept limited traffic. The circuit breaker is in a half-open state.
	ToolStateDegraded ToolState = "degraded"

	// ToolStateUnavailable means the tool cannot accept any requests.
	// The circuit is open and recovery probes are pending.
	ToolStateUnavailable ToolState = "unavailable"
)

// CredentialPolicy defines the credential requirement for a tool.
type CredentialPolicy string

const (
	// CredentialPolicyOptional means the tool may be invoked without credentials.
	// When credentials are available they are provided; when absent the call proceeds.
	CredentialPolicyOptional CredentialPolicy = "optional"

	// CredentialPolicyRequired means the tool must have credentials resolved before
	// invocation. Requests without valid credentials are rejected.
	CredentialPolicyRequired CredentialPolicy = "required"
)

// AuthorizationPolicy defines the authorization model for a tool.
type AuthorizationPolicy string

const (
	// AuthorizationPolicyTenantAllowlist restricts access to explicitly permitted tenants.
	AuthorizationPolicyTenantAllowlist AuthorizationPolicy = "tenant_allowlist"
)

// StdioTransportConfig holds the execution parameters for tools launched as local
// child processes and communicated with over stdin/stdout.
type StdioTransportConfig struct {
	// Command is the executable to launch (e.g., "npx", "python").
	Command string

	// Args are the arguments passed to the command.
	Args []string

	// Env holds environment variables injected into the child process.
	// Variables defined here are merged with the gateway process environment.
	Env map[string]string

	// WorkingDirectory is the working directory for the child process.
	WorkingDirectory string
}

// HTTPTransportConfig holds the connection parameters for tools reached over HTTP.
type HTTPTransportConfig struct {
	// URL is the base URL of the remote MCP server.
	URL string

	// TLS holds optional TLS configuration for outbound connections.
	// When nil, the default system CA pool is used for https:// URLs.
	TLS *EgressTLSConfig
}

// EgressTLSConfig holds TLS settings for outbound HTTP connections to MCP servers.
type EgressTLSConfig struct {
	// CACertFile is the path to a CA certificate (PEM) for verifying the server.
	// When empty, the system default CA pool is used.
	CACertFile string

	// ClientCertFile is the path to the client certificate (PEM) for mutual TLS.
	ClientCertFile string

	// ClientKeyFile is the path to the client private key (PEM).
	ClientKeyFile string

	// InsecureSkipVerify, when true, disables server certificate verification.
	// Use only for development or testing.
	InsecureSkipVerify bool
}

// Tool represents a registered MCP capability with its full execution metadata.
//
// The Tool model separates three concerns:
//
//   - Static metadata: identity and transport configuration that does not change
//     at runtime (ID, Transport, Stdio, HTTP).
//   - Timing configuration: per-tool deadline and idle-timeout values.
//   - Policy metadata: routing mode, credential, and authorization requirements.
//   - Operational state: the current availability state managed by the registry.
//
// Tool values are immutable after registration. Operational state transitions are
// communicated through runtime messages rather than direct field mutation.
type Tool struct {
	// ID is the unique identifier for this tool within the runtime.
	ID ToolID

	// Transport identifies the execution transport used to reach the tool.
	Transport TransportType

	// Stdio holds the configuration for stdio-based tool execution.
	// This field is non-nil only when Transport is TransportStdio.
	Stdio *StdioTransportConfig

	// HTTP holds the configuration for HTTP-based tool execution.
	// This field is non-nil only when Transport is TransportHTTP.
	HTTP *HTTPTransportConfig

	// StartupTimeout is the maximum time the runtime waits for the tool to become ready.
	StartupTimeout time.Duration

	// RequestTimeout is the maximum time allowed for a single MCP invocation.
	RequestTimeout time.Duration

	// IdleTimeout is the duration after which an idle session is passivated.
	IdleTimeout time.Duration

	// Routing defines the strategy used to select an execution target.
	Routing RoutingMode

	// CredentialPolicy describes the credential requirement for this tool.
	CredentialPolicy CredentialPolicy

	// AuthorizationPolicy describes the authorization model for this tool.
	AuthorizationPolicy AuthorizationPolicy

	// State is the current operational availability of the tool.
	// The registry owns and transitions this field through runtime messages.
	State ToolState
}

// IsStdio reports whether the tool uses the stdio transport.
func (t Tool) IsStdio() bool { return t.Transport == TransportStdio }

// IsHTTP reports whether the tool uses the HTTP transport.
func (t Tool) IsHTTP() bool { return t.Transport == TransportHTTP }

// IsAvailable reports whether the tool can accept new requests.
// A tool is available when it is in the enabled or degraded state.
func (t Tool) IsAvailable() bool {
	return t.State == ToolStateEnabled || t.State == ToolStateDegraded
}
