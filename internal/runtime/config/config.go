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
// gateway. Public configuration types are defined in the mcp/ package and
// re-exported here via type aliases for backward compatibility with internal code.
package config

import (
	"os"
	"time"

	goaktlog "github.com/tochemey/goakt/v4/log"

	"github.com/tochemey/goakt-mcp/mcp"
)

// Type aliases to mcp/ package for backward compatibility.
type Config = mcp.Config
type RuntimeConfig = mcp.RuntimeConfig
type ClusterConfig = mcp.ClusterConfig
type KubernetesDiscoveryConfig = mcp.KubernetesDiscoveryConfig
type DNSSDDiscoveryConfig = mcp.DNSSDDiscoveryConfig
type TelemetryConfig = mcp.TelemetryConfig
type AuditConfig = mcp.AuditConfig
type CredentialsConfig = mcp.CredentialsConfig
type TenantConfig = mcp.TenantConfig
type TenantQuotaConfig = mcp.TenantQuotaConfig

// Re-export default constants from mcp/ for internal use.
const (
	DefaultSessionIdleTimeout  = mcp.DefaultSessionIdleTimeout
	DefaultRequestTimeout      = mcp.DefaultRequestTimeout
	DefaultStartupTimeout      = mcp.DefaultStartupTimeout
	DefaultHealthProbeInterval = mcp.DefaultHealthProbeInterval
	DefaultShutdownTimeout     = mcp.DefaultShutdownTimeout
)

// ToolConfig defines the static configuration for a single registered tool.
// This is an internal type for building mcp.Tool with defaults; users typically
// construct mcp.Tool directly.
type ToolConfig struct {
	ID                  mcp.ToolID
	Transport           mcp.TransportType
	Command             string
	Args                []string
	Env                 map[string]string
	WorkingDirectory    string
	URL                 string
	HTTPTLS             *mcp.TLSClientConfig
	StartupTimeout      time.Duration
	RequestTimeout      time.Duration
	IdleTimeout         time.Duration
	Routing             mcp.RoutingMode
	CredentialPolicy    mcp.CredentialPolicy
	AuthorizationPolicy mcp.AuthorizationPolicy
}

// ToolConfigToTool converts a ToolConfig to a mcp.Tool, applying runtime defaults
// when tool-level values are zero.
func ToolConfigToTool(toolConfig ToolConfig, defaults RuntimeConfig) mcp.Tool {
	tool := mcp.Tool{
		ID:                  toolConfig.ID,
		Transport:           toolConfig.Transport,
		StartupTimeout:      toolConfig.StartupTimeout,
		RequestTimeout:      toolConfig.RequestTimeout,
		IdleTimeout:         toolConfig.IdleTimeout,
		Routing:             toolConfig.Routing,
		CredentialPolicy:    toolConfig.CredentialPolicy,
		AuthorizationPolicy: toolConfig.AuthorizationPolicy,
		State:               mcp.ToolStateEnabled,
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
	case mcp.TransportStdio:
		tool.Stdio = &mcp.StdioTransportConfig{
			Command:          toolConfig.Command,
			Args:             toolConfig.Args,
			Env:              toolConfig.Env,
			WorkingDirectory: toolConfig.WorkingDirectory,
		}
	case mcp.TransportHTTP:
		tool.HTTP = &mcp.HTTPTransportConfig{
			URL: toolConfig.URL,
			TLS: toolConfig.HTTPTLS,
		}
	}

	return tool
}

// NewLogger creates a goaktlog.Logger based on the configured LogLevel string.
// Uses GoAkt's slog implementation. When the level is empty or invalid,
// the GoAkt DefaultLogger is returned (InfoLevel to stdout).
func NewLogger(level string) goaktlog.Logger {
	parsed := ParseLogLevel(level)
	if parsed == goaktlog.InvalidLevel {
		return goaktlog.DiscardLogger
	}
	return goaktlog.NewSlog(parsed, os.Stdout)
}

// ParseLogLevel converts a log level string to a goaktlog.Level.
func ParseLogLevel(s string) goaktlog.Level {
	switch s {
	case "debug":
		return goaktlog.DebugLevel
	case "info":
		return goaktlog.InfoLevel
	case "warning", "warn":
		return goaktlog.WarningLevel
	case "error":
		return goaktlog.ErrorLevel
	case "fatal":
		return goaktlog.FatalLevel
	case "panic":
		return goaktlog.PanicLevel
	case "":
		return goaktlog.InvalidLevel
	default:
		return goaktlog.InvalidLevel
	}
}
