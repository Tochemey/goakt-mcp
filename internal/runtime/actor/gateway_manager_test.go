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

package actor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goaktactor "github.com/tochemey/goakt/v4/actor"
	"github.com/tochemey/goakt/v4/testkit"

	"github.com/tochemey/goakt-mcp/mcp"

	"github.com/tochemey/goakt-mcp/internal/runtime/actor/extension"
	"github.com/tochemey/goakt-mcp/internal/runtime/audit"
	"github.com/tochemey/goakt-mcp/internal/runtime/config"
)

func TestGatewayManager(t *testing.T) {
	t.Run("spawns as a top-level actor successfully", func(t *testing.T) {
		ctx := t.Context()
		config := testConfig()
		system, stop := testActorSystem(t,
			goaktactor.WithExtensions(extension.NewConfigExtension(config)))
		defer stop()

		pid, err := system.Spawn(ctx, mcp.ActorNameGatewayManager, NewGatewayManager())
		require.NoError(t, err)
		require.NotNil(t, pid)
		assert.Equal(t, mcp.ActorNameGatewayManager, pid.Name())
	})

	t.Run("spawns foundational children on PostStart", func(t *testing.T) {
		ctx := t.Context()
		config := testConfig()
		system, stop := testActorSystem(t, goaktactor.WithExtensions(extension.NewConfigExtension(config)))
		defer stop()

		pid, err := system.Spawn(ctx, mcp.ActorNameGatewayManager, NewGatewayManager())
		require.NoError(t, err)

		waitForActors()

		children := pid.Children()
		childNames := make(map[string]bool, len(children))
		for _, c := range children {
			childNames[c.Name()] = true
		}

		assert.True(t, childNames[mcp.ActorNameRegistrar], "RegistryActor must be spawned")
		assert.True(t, childNames[mcp.ActorNameHealth], "HealthActor must be spawned")
		assert.True(t, childNames[mcp.ActorNameJournal], "JournalActor must be spawned")
		assert.True(t, childNames[mcp.ActorNamePolicy], "PolicyActor must be spawned")
		assert.True(t, childNames[mcp.ActorNameCredentialBroker], "CredentialBrokerActor must be spawned")
		assert.True(t, childNames[mcp.ActorNameRouter], "RouterActor must be spawned")
		assert.Len(t, children, 6, "GatewayManager must spawn exactly six foundational actors")
	})

	t.Run("unhandles unknown message", func(t *testing.T) {
		config := testConfig()
		kit, ctx := newTestKit(t, testkit.WithExtensions(extension.NewConfigExtension(config)))

		pid, err := kit.ActorSystem().Spawn(ctx, mcp.ActorNameGatewayManager, NewGatewayManager())
		require.NoError(t, err)
		waitForActors()
		require.NoError(t, pid.Tell(ctx, pid, "unknown"))
		waitForActors()
	})

	t.Run("createAuditSink returns sink when configured", func(t *testing.T) {
		memSink := audit.NewMemorySink()
		cfg := config.AuditConfig{Sink: memSink}
		sink := createAuditSink(cfg)
		require.NotNil(t, sink)
		_ = sink.Close()
	})
}
