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

// TransportType defines the underlying MCP transport protocol used by a tool.
type TransportType string

const (
	TransportStdio TransportType = "stdio"
	TransportHTTP  TransportType = "http"
)

// RoutingMode defines how the runtime selects an execution target for a tool.
type RoutingMode string

const (
	RoutingSticky      RoutingMode = "sticky"
	RoutingLeastLoaded RoutingMode = "least_loaded"
)

// ToolState describes the current operational availability of a registered tool.
type ToolState string

const (
	ToolStateEnabled     ToolState = "enabled"
	ToolStateDisabled    ToolState = "disabled"
	ToolStateDegraded    ToolState = "degraded"
	ToolStateUnavailable ToolState = "unavailable"
)

// CredentialPolicy defines the credential requirement for a tool.
type CredentialPolicy string

const (
	CredentialPolicyOptional CredentialPolicy = "optional"
	CredentialPolicyRequired CredentialPolicy = "required"
)

// AuthorizationPolicy defines the authorization model for a tool.
type AuthorizationPolicy string

const (
	AuthorizationPolicyTenantAllowlist AuthorizationPolicy = "tenant_allowlist"
)

// StdioTransportConfig holds the execution parameters for tools launched as local
// child processes and communicated with over stdin/stdout.
type StdioTransportConfig struct {
	Command          string
	Args             []string
	Env              map[string]string
	WorkingDirectory string
}

// HTTPTransportConfig holds the connection parameters for tools reached over HTTP.
type HTTPTransportConfig struct {
	URL string
	TLS *EgressTLSConfig
}

// EgressTLSConfig holds TLS settings for outbound HTTP connections to MCP servers.
type EgressTLSConfig struct {
	CACertFile         string
	ClientCertFile     string
	ClientKeyFile      string
	InsecureSkipVerify bool
}

// Tool represents a registered MCP capability with its full execution metadata.
type Tool struct {
	ID                  ToolID
	Transport           TransportType
	Stdio               *StdioTransportConfig
	HTTP                *HTTPTransportConfig
	StartupTimeout      time.Duration
	RequestTimeout      time.Duration
	IdleTimeout         time.Duration
	Routing             RoutingMode
	CredentialPolicy    CredentialPolicy
	AuthorizationPolicy AuthorizationPolicy
	State               ToolState
}

// IsStdio reports whether the tool uses the stdio transport.
func (t Tool) IsStdio() bool { return t.Transport == TransportStdio }

// IsHTTP reports whether the tool uses the HTTP transport.
func (t Tool) IsHTTP() bool { return t.Transport == TransportHTTP }

// IsAvailable reports whether the tool can accept new requests.
func (t Tool) IsAvailable() bool {
	return t.State == ToolStateEnabled || t.State == ToolStateDegraded
}
