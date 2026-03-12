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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigDefaults(t *testing.T) {
	t.Run("DefaultHealthProbeTimeout is set", func(t *testing.T) {
		assert.Equal(t, 10*time.Second, DefaultHealthProbeTimeout)
	})

	t.Run("DefaultMaxCacheEntries is set", func(t *testing.T) {
		assert.Equal(t, 1000, DefaultMaxCacheEntries)
	})

	t.Run("CredentialsConfig MaxCacheEntries field exists", func(t *testing.T) {
		cfg := CredentialsConfig{MaxCacheEntries: 500}
		assert.Equal(t, 500, cfg.MaxCacheEntries)
	})

	t.Run("HealthProbeConfig Timeout field exists", func(t *testing.T) {
		cfg := HealthProbeConfig{Timeout: 5 * time.Second}
		assert.Equal(t, 5*time.Second, cfg.Timeout)
	})

	t.Run("DefaultAuditMailboxSize is set", func(t *testing.T) {
		assert.Equal(t, 1024, DefaultAuditMailboxSize)
	})

	t.Run("AuditConfig MailboxSize field exists", func(t *testing.T) {
		cfg := AuditConfig{MailboxSize: 512}
		assert.Equal(t, 512, cfg.MailboxSize)
	})
}
