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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	"github.com/tochemey/goakt-mcp/internal/runtime/config"
)

func TestGatewayManager(t *testing.T) {
	ctx := context.Background()

	t.Run("spawns as a top-level actor successfully", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		pid, err := system.Spawn(ctx, runtime.ActorNameGatewayManager, newGatewayManager(testConfig()))
		require.NoError(t, err)
		require.NotNil(t, pid)
		assert.Equal(t, runtime.ActorNameGatewayManager, pid.Name())
	})

	t.Run("spawns foundational children on PostStart", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		pid, err := system.Spawn(ctx, runtime.ActorNameGatewayManager, newGatewayManager(testConfig()))
		require.NoError(t, err)

		waitForActors()

		children := pid.Children()
		childNames := make(map[string]bool, len(children))
		for _, c := range children {
			childNames[c.Name()] = true
		}

		assert.True(t, childNames[runtime.ActorNameRegistrar], "RegistryActor must be spawned")
		assert.True(t, childNames[runtime.ActorNameHealth], "HealthActor must be spawned")
		assert.True(t, childNames[runtime.ActorNameJournal], "JournalActor must be spawned")
		assert.True(t, childNames[runtime.ActorNamePolicy], "PolicyActor must be spawned")
		assert.True(t, childNames[runtime.ActorNameRouter], "RouterActor must be spawned")
		assert.Len(t, children, 5, "GatewayManager must spawn exactly five foundational actors")
	})

	t.Run("unhandles unknown message", func(t *testing.T) {
		kit, ctx := newTestKit(t)

		cfg := testConfig()
		pid, err := kit.ActorSystem().Spawn(ctx, runtime.ActorNameGatewayManager, newGatewayManager(cfg))
		require.NoError(t, err)
		waitForActors()
		require.NoError(t, pid.Tell(ctx, pid, "unknown"))
		waitForActors()
	})

	t.Run("createAuditSink returns MemorySink for s3 backend", func(t *testing.T) {
		cfg := config.AuditConfig{Backend: "s3", Bucket: "test"}
		sink := createAuditSink(cfg)
		require.NotNil(t, sink)
		_ = sink.Close()
	})

	t.Run("buildCredentialProviders skips unknown provider", func(t *testing.T) {
		providers := buildCredentialProviders([]string{"vault", "env"})
		assert.Len(t, providers, 1)
	})
}
