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

package mcp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tochemey/goakt-mcp/mcp"
)

func TestActorNameConstants(t *testing.T) {
	assert.Equal(t, "gateway-manager", mcp.ActorNameGatewayManager)
	assert.Equal(t, "registrar", mcp.ActorNameRegistrar)
	assert.Equal(t, "health", mcp.ActorNameHealth)
	assert.Equal(t, "journal", mcp.ActorNameJournal)
	assert.Equal(t, "credential-broker", mcp.ActorNameCredentialBroker)
	assert.Equal(t, "router", mcp.ActorNameRouter)
	assert.Equal(t, "policy", mcp.ActorNamePolicy)
}

func TestToolSupervisorName(t *testing.T) {
	assert.Equal(t, "supervisor-my-tool", mcp.ToolSupervisorName("my-tool"))
}

func TestSessionName(t *testing.T) {
	assert.Equal(t, "session-t1-c1-tool-a", mcp.SessionName("t1", "c1", "tool-a"))
}
