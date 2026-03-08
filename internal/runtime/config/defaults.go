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
func ApplyDefaults(cfg *Config) {
	if cfg.Runtime.SessionIdleTimeout == 0 {
		cfg.Runtime.SessionIdleTimeout = DefaultSessionIdleTimeout
	}
	if cfg.Runtime.RequestTimeout == 0 {
		cfg.Runtime.RequestTimeout = DefaultRequestTimeout
	}
	if cfg.Runtime.StartupTimeout == 0 {
		cfg.Runtime.StartupTimeout = DefaultStartupTimeout
	}
	if cfg.Runtime.HealthProbeInterval == 0 {
		cfg.Runtime.HealthProbeInterval = DefaultHealthProbeInterval
	}
	if cfg.Runtime.ShutdownTimeout == 0 {
		cfg.Runtime.ShutdownTimeout = DefaultShutdownTimeout
	}
	if cfg.Tools == nil {
		cfg.Tools = []mcp.Tool{}
	}
	if cfg.Tenants == nil {
		cfg.Tenants = []TenantConfig{}
	}
}
