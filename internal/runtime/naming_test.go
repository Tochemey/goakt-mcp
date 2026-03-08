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

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActorNameConstants(t *testing.T) {
	assert.Equal(t, "gateway-manager", ActorNameGatewayManager)
	assert.Equal(t, "registrar", ActorNameRegistrar)
	assert.Equal(t, "health", ActorNameHealth)
	assert.Equal(t, "journal", ActorNameJournal)
	assert.Equal(t, "credential-broker", ActorNameCredentialBroker)
}

func TestToolSupervisorName(t *testing.T) {
	tests := []struct {
		toolID   ToolID
		expected string
	}{
		{"filesystem", "supervisor-filesystem"},
		{"docs-search", "supervisor-docs-search"},
		{"my-tool-123", "supervisor-my-tool-123"},
	}
	for _, tt := range tests {
		t.Run(string(tt.toolID), func(t *testing.T) {
			assert.Equal(t, tt.expected, ToolSupervisorName(tt.toolID))
		})
	}
}

func TestSessionName(t *testing.T) {
	tests := []struct {
		tenantID TenantID
		clientID ClientID
		toolID   ToolID
		expected string
	}{
		{"acme", "user-1", "filesystem", "session-acme-user-1-filesystem"},
		{"tenant-b", "client-123", "docs-search", "session-tenant-b-client-123-docs-search"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, SessionName(tt.tenantID, tt.clientID, tt.toolID))
		})
	}
}

func TestSessionNameUniqueness(t *testing.T) {
	name1 := SessionName("acme", "user-1", "filesystem")
	name2 := SessionName("acme", "user-2", "filesystem")
	name3 := SessionName("other", "user-1", "filesystem")
	name4 := SessionName("acme", "user-1", "docs-search")

	assert.NotEqual(t, name1, name2, "different clients must produce different session names")
	assert.NotEqual(t, name1, name3, "different tenants must produce different session names")
	assert.NotEqual(t, name1, name4, "different tools must produce different session names")
}
