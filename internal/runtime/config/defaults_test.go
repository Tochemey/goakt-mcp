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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestApplyDefaults(t *testing.T) {
	t.Run("fills zero values", func(t *testing.T) {
		cfg := Config{}
		ApplyDefaults(&cfg)
		assert.Equal(t, DefaultHTTPListenAddress, cfg.HTTP.ListenAddress)
		assert.Equal(t, DefaultSessionIdleTimeout, cfg.Runtime.SessionIdleTimeout)
		assert.Equal(t, DefaultRequestTimeout, cfg.Runtime.RequestTimeout)
		assert.Equal(t, DefaultStartupTimeout, cfg.Runtime.StartupTimeout)
		assert.Equal(t, DefaultHealthProbeInterval, cfg.Runtime.HealthProbeInterval)
		assert.Equal(t, DefaultShutdownTimeout, cfg.Runtime.ShutdownTimeout)
		assert.NotNil(t, cfg.Tools)
		assert.NotNil(t, cfg.Tenants)
	})

	t.Run("preserves explicit values", func(t *testing.T) {
		cfg := Config{
			HTTP: HTTPConfig{ListenAddress: ":9999"},
			Runtime: RuntimeConfig{
				SessionIdleTimeout:  10 * time.Minute,
				RequestTimeout:      60 * time.Second,
				StartupTimeout:      20 * time.Second,
				HealthProbeInterval: 1 * time.Minute,
				ShutdownTimeout:     45 * time.Second,
			},
			Tools:   []ToolConfig{{ID: "t"}},
			Tenants: []TenantConfig{{ID: "x"}},
		}
		ApplyDefaults(&cfg)
		assert.Equal(t, ":9999", cfg.HTTP.ListenAddress)
		assert.Equal(t, 10*time.Minute, cfg.Runtime.SessionIdleTimeout)
		assert.Equal(t, 60*time.Second, cfg.Runtime.RequestTimeout)
		assert.Equal(t, 20*time.Second, cfg.Runtime.StartupTimeout)
		assert.Equal(t, 1*time.Minute, cfg.Runtime.HealthProbeInterval)
		assert.Equal(t, 45*time.Second, cfg.Runtime.ShutdownTimeout)
		assert.Len(t, cfg.Tools, 1)
		assert.Len(t, cfg.Tenants, 1)
	})
}
