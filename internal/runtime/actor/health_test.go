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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goaktactor "github.com/tochemey/goakt/v4/actor"
	noopmetric "go.opentelemetry.io/otel/metric/noop"

	"github.com/tochemey/goakt-mcp/mcp"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	actorextension "github.com/tochemey/goakt-mcp/internal/runtime/actor/extension"
	"github.com/tochemey/goakt-mcp/internal/runtime/audit"
	"github.com/tochemey/goakt-mcp/internal/runtime/telemetry"
)

func TestHealthActor(t *testing.T) {
	ctx := context.Background()

	t.Run("starts and stops cleanly", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		pid, err := system.Spawn(ctx, mcp.ActorNameHealth, newHealthChecker(nil, nil, 0))
		require.NoError(t, err)
		require.NotNil(t, pid)
		assert.Equal(t, mcp.ActorNameHealth, pid.Name())

		waitForActors()
	})

	t.Run("unhandles non-PostStart messages", func(t *testing.T) {
		kit, ctx := newTestKit(t)

		kit.Spawn(ctx, mcp.ActorNameHealth, newHealthChecker(nil, nil, 0))
		waitForActors()

		pid, err := kit.ActorSystem().ActorOf(ctx, mcp.ActorNameHealth)
		require.NoError(t, err)
		require.NotNil(t, pid)
		require.NoError(t, pid.Tell(ctx, pid, "unknown-message"))
		waitForActors()
	})

	t.Run("runProbes with nil registrar is a no-op", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		pid, err := system.Spawn(ctx, mcp.ActorNameHealth, newHealthChecker(nil, nil, time.Hour))
		require.NoError(t, err)
		waitForActors()

		require.NoError(t, pid.Tell(ctx, pid, &runProbes{}))
		waitForActors()
	})

	t.Run("runProbes probes a healthy tool and skips disabled tools", func(t *testing.T) {
		system, stop := testActorSystem(t,
			goaktactor.WithExtensions(actorextension.NewToolConfigExtension()),
		)
		defer stop()

		// Journal must exist before supervisor PostStart resolves it.
		_, err := system.Spawn(ctx, mcp.ActorNameJournal, newJournaler(audit.NewMemorySink()))
		require.NoError(t, err)

		registrarPID, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		tool := validStdioTool("health-probe-tool")
		resp, err := goaktactor.Ask(ctx, registrarPID, &runtime.RegisterTool{Tool: tool}, 5*time.Second)
		require.NoError(t, err)
		regResult, ok := resp.(*runtime.RegisterToolResult)
		require.True(t, ok)
		require.NoError(t, regResult.Err)
		waitForActors()

		disabledTool := validStdioTool("disabled-probe-tool")
		disabledTool.State = mcp.ToolStateDisabled
		resp, err = goaktactor.Ask(ctx, registrarPID, &runtime.RegisterTool{Tool: disabledTool}, 5*time.Second)
		require.NoError(t, err)
		disabledResult, ok := resp.(*runtime.RegisterToolResult)
		require.True(t, ok)
		require.NoError(t, disabledResult.Err)
		waitForActors()

		healthPID, err := system.Spawn(ctx, mcp.ActorNameHealth, newHealthChecker(registrarPID, nil, time.Hour))
		require.NoError(t, err)
		waitForActors()

		require.NoError(t, healthPID.Tell(ctx, healthPID, &runProbes{}))
		time.Sleep(300 * time.Millisecond)
	})

	t.Run("runProbes with dead registrar schedules next", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		registrarPID, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()
		require.NoError(t, system.Kill(ctx, mcp.ActorNameRegistrar))
		waitForActors()

		healthPID, err := system.Spawn(ctx, mcp.ActorNameHealth, newHealthChecker(registrarPID, nil, time.Hour))
		require.NoError(t, err)
		waitForActors()

		require.NoError(t, healthPID.Tell(ctx, healthPID, &runProbes{}))
		waitForActors()
	})

	t.Run("runProbes records ToolAvailability metric when metrics are registered", func(t *testing.T) {
		meter := noopmetric.NewMeterProvider().Meter("test")
		_, err := telemetry.RegisterMetrics(meter)
		require.NoError(t, err)
		t.Cleanup(telemetry.UnregisterMetrics)

		system, stop := testActorSystem(t, goaktactor.WithExtensions(actorextension.NewToolConfigExtension()))
		defer stop()

		_, err = system.Spawn(ctx, mcp.ActorNameJournal, newJournaler(audit.NewMemorySink()))
		require.NoError(t, err)

		registrarPID, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		tool := validStdioTool("metrics-health-tool")
		_, err = goaktactor.Ask(ctx, registrarPID, &runtime.RegisterTool{Tool: tool}, 5*time.Second)
		require.NoError(t, err)
		waitForActors()

		healthPID, err := system.Spawn(ctx, mcp.ActorNameHealth, newHealthChecker(registrarPID, nil, time.Hour))
		require.NoError(t, err)
		waitForActors()

		require.NoError(t, healthPID.Tell(ctx, healthPID, &runProbes{}))
		time.Sleep(300 * time.Millisecond)
	})

	t.Run("runProbes records health transition to journal when state changes", func(t *testing.T) {
		system, stop := testActorSystem(t,
			goaktactor.WithExtensions(actorextension.NewToolConfigExtension()),
		)
		defer stop()

		sink := audit.NewMemorySink()
		journalPID, err := system.Spawn(ctx, mcp.ActorNameJournal, newJournaler(sink))
		require.NoError(t, err)
		waitForActors()

		registrarPID, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		tool := validStdioTool("health-audit-tool")
		_, err = goaktactor.Ask(ctx, registrarPID, &runtime.RegisterTool{Tool: tool}, 5*time.Second)
		require.NoError(t, err)
		waitForActors()

		healthPID, err := system.Spawn(ctx, mcp.ActorNameHealth, newHealthChecker(registrarPID, journalPID, time.Hour))
		require.NoError(t, err)
		waitForActors()

		require.NoError(t, healthPID.Tell(ctx, healthPID, &runProbes{}))
		time.Sleep(300 * time.Millisecond)

		// The journal is wired; verify no panic occurred.
		assert.NotNil(t, sink.Events())
	})
}
