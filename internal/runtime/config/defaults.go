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

import "github.com/tochemey/goakt-mcp/mcp"

// ApplyDefaults fills zero values in cfg with defaults.
func ApplyDefaults(config *Config) {
	if config.Runtime.SessionIdleTimeout <= 0 {
		config.Runtime.SessionIdleTimeout = DefaultSessionIdleTimeout
	}
	if config.Runtime.RequestTimeout <= 0 {
		config.Runtime.RequestTimeout = DefaultRequestTimeout
	}
	if config.Runtime.StartupTimeout <= 0 {
		config.Runtime.StartupTimeout = DefaultStartupTimeout
	}
	if config.Runtime.HealthProbeInterval <= 0 {
		config.Runtime.HealthProbeInterval = DefaultHealthProbeInterval
	}
	if config.Runtime.ShutdownTimeout <= 0 {
		config.Runtime.ShutdownTimeout = DefaultShutdownTimeout
	}
	if config.Tools == nil {
		config.Tools = []mcp.Tool{}
	}
	if config.HealthProbe.Timeout <= 0 {
		config.HealthProbe.Timeout = mcp.DefaultHealthProbeTimeout
	}
	if config.Credentials.MaxCacheEntries <= 0 {
		config.Credentials.MaxCacheEntries = mcp.DefaultMaxCacheEntries
	}
	if config.Audit.MailboxSize <= 0 {
		config.Audit.MailboxSize = mcp.DefaultAuditMailboxSize
	}
	if config.Tenants == nil {
		config.Tenants = []TenantConfig{}
	}
}
