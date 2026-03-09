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
	"github.com/tochemey/goakt/v4/testkit"
	noopmetric "go.opentelemetry.io/otel/metric/noop"

	"github.com/tochemey/goakt-mcp/mcp"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	actorextension "github.com/tochemey/goakt-mcp/internal/runtime/actor/extension"
	"github.com/tochemey/goakt-mcp/internal/runtime/audit"
	"github.com/tochemey/goakt-mcp/internal/runtime/telemetry"
)

func TestToolSupervisorActor(t *testing.T) {
	ctx := context.Background()

	t.Run("resolves tool from extension and accepts work when circuit closed", func(t *testing.T) {
		tool := validStdioTool("supervisor-tool")
		system, stop := testActorSystemWithTools(t, tool)
		defer stop()

		_, err := system.Spawn(ctx, mcp.ActorNameJournal, newJournaler())
		require.NoError(t, err)

		name := mcp.ToolSupervisorName(tool.ID)
		pid, err := system.Spawn(ctx, name, newToolSupervisor())
		require.NoError(t, err)
		require.NotNil(t, pid)

		waitForActors()

		resp, err := goaktactor.Ask(ctx, pid, &runtime.CanAcceptWork{ToolID: tool.ID}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.CanAcceptWorkResult)
		require.True(t, ok)
		assert.True(t, result.Accept)
	})

	t.Run("rejects work when circuit opened after failures", func(t *testing.T) {
		tool := validStdioTool("circuit-tool")
		system, stop := testActorSystemWithTools(t, tool)
		defer stop()

		_, err := system.Spawn(ctx, mcp.ActorNameJournal, newJournaler())
		require.NoError(t, err)

		name := mcp.ToolSupervisorName(tool.ID)
		pid, err := system.Spawn(ctx, name, newToolSupervisor())
		require.NoError(t, err)
		waitForActors()

		for i := 0; i < mcp.DefaultCircuitFailureThreshold; i++ {
			err = goaktactor.Tell(ctx, pid, &runtime.ReportFailure{ToolID: tool.ID})
			require.NoError(t, err)
		}
		waitForActors()

		resp, err := goaktactor.Ask(ctx, pid, &runtime.CanAcceptWork{ToolID: tool.ID}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.CanAcceptWorkResult)
		require.True(t, ok)
		assert.False(t, result.Accept)
		assert.Contains(t, result.Reason, "circuit")
	})

	t.Run("rejects work when tool ID mismatch", func(t *testing.T) {
		tool := validStdioTool("mismatch-tool")
		system, stop := testActorSystemWithTools(t, tool)
		defer stop()

		_, err := system.Spawn(ctx, mcp.ActorNameJournal, newJournaler())
		require.NoError(t, err)

		name := mcp.ToolSupervisorName(tool.ID)
		pid, err := system.Spawn(ctx, name, newToolSupervisor())
		require.NoError(t, err)
		waitForActors()

		resp, err := goaktactor.Ask(ctx, pid, &runtime.CanAcceptWork{ToolID: "other-tool"}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.CanAcceptWorkResult)
		require.True(t, ok)
		assert.False(t, result.Accept)
	})

	t.Run("report success closes circuit from half-open", func(t *testing.T) {
		tool := validStdioTool("halfopen-tool")
		circuitCfg := mcp.CircuitConfig{
			FailureThreshold:    mcp.DefaultCircuitFailureThreshold,
			OpenDuration:        100 * time.Millisecond,
			HalfOpenMaxRequests: mcp.DefaultCircuitHalfOpenMaxRequests,
		}
		toolCfgExt := actorextension.NewToolConfigExtension()
		toolCfgExt.Register(tool)
		circuitCfgExt := actorextension.NewCircuitConfigExtension(circuitCfg)
		cfg := testConfig()
		cfg.Audit.Sink = audit.NewMemorySink()
		kit, ctx := newTestKit(t,
			testkit.WithExtensions(toolCfgExt, circuitCfgExt, actorextension.NewConfigExtension(cfg)),
		)

		kit.ActorSystem().Spawn(ctx, mcp.ActorNameJournal, newJournaler())

		name := mcp.ToolSupervisorName(tool.ID)
		kit.ActorSystem().Spawn(ctx, name, newToolSupervisor())
		waitForActors()

		probe := kit.NewProbe(ctx)
		for i := 0; i < mcp.DefaultCircuitFailureThreshold; i++ {
			probe.Send(name, &runtime.ReportFailure{ToolID: tool.ID})
		}
		waitForActors()

		time.Sleep(150 * time.Millisecond)

		probe.SendSync(name, &runtime.CanAcceptWork{ToolID: tool.ID}, askTimeout)
		resp := probe.ExpectAnyMessage()
		result, ok := resp.(*runtime.CanAcceptWorkResult)
		require.True(t, ok)
		assert.True(t, result.Accept)

		probe.Send(name, &runtime.ReportSuccess{ToolID: tool.ID})
		waitForActors()

		probe.SendSync(name, &runtime.CanAcceptWork{ToolID: tool.ID}, askTimeout)
		resp2 := probe.ExpectAnyMessage()
		result2, ok := resp2.(*runtime.CanAcceptWorkResult)
		require.True(t, ok)
		assert.True(t, result2.Accept)
		probe.Stop()
	})

	t.Run("report failure in half-open reopens circuit", func(t *testing.T) {
		tool := validStdioTool("reopen-tool")
		circuitCfg := mcp.CircuitConfig{
			FailureThreshold:    mcp.DefaultCircuitFailureThreshold,
			OpenDuration:        100 * time.Millisecond,
			HalfOpenMaxRequests: mcp.DefaultCircuitHalfOpenMaxRequests,
		}
		toolCfgExt := actorextension.NewToolConfigExtension()
		toolCfgExt.Register(tool)
		circuitCfgExt := actorextension.NewCircuitConfigExtension(circuitCfg)
		cfg := testConfig()
		cfg.Audit.Sink = audit.NewMemorySink()
		kit, ctx := newTestKit(t,
			testkit.WithExtensions(toolCfgExt, circuitCfgExt, actorextension.NewConfigExtension(cfg)),
		)

		kit.ActorSystem().Spawn(ctx, mcp.ActorNameJournal, newJournaler())

		name := mcp.ToolSupervisorName(tool.ID)
		kit.ActorSystem().Spawn(ctx, name, newToolSupervisor())
		waitForActors()

		pid, err := kit.ActorSystem().ActorOf(ctx, name)
		require.NoError(t, err)

		for i := 0; i < mcp.DefaultCircuitFailureThreshold; i++ {
			require.NoError(t, pid.Tell(ctx, pid, &runtime.ReportFailure{ToolID: tool.ID}))
		}
		waitForActors()

		time.Sleep(150 * time.Millisecond)

		probe := kit.NewProbe(ctx)
		probe.SendSync(name, &runtime.CanAcceptWork{ToolID: tool.ID}, askTimeout)
		probe.ExpectAnyMessage()

		pid.Tell(ctx, pid, &runtime.ReportFailure{ToolID: tool.ID})
		time.Sleep(30 * time.Millisecond)

		probe.SendSync(name, &runtime.CanAcceptWork{ToolID: tool.ID}, askTimeout)
		resp := probe.ExpectAnyMessage()
		result, ok := resp.(*runtime.CanAcceptWorkResult)
		require.True(t, ok)
		assert.False(t, result.Accept)
		probe.Stop()
	})

	t.Run("report success with wrong tool ID is ignored", func(t *testing.T) {
		tool := validStdioTool("success-mismatch")
		toolCfgExt := actorextension.NewToolConfigExtension()
		toolCfgExt.Register(tool)
		cfg := testConfig()
		cfg.Audit.Sink = audit.NewMemorySink()
		kit, ctx := newTestKit(t, testkit.WithExtensions(toolCfgExt, actorextension.NewConfigExtension(cfg)))

		kit.ActorSystem().Spawn(ctx, mcp.ActorNameJournal, newJournaler())

		name := mcp.ToolSupervisorName(tool.ID)
		kit.ActorSystem().Spawn(ctx, name, newToolSupervisor())
		waitForActors()

		pid, _ := kit.ActorSystem().ActorOf(ctx, name)
		require.NoError(t, pid.Tell(ctx, pid, &runtime.ReportSuccess{ToolID: "wrong-tool"}))
		waitForActors()

		probe := kit.NewProbe(ctx)
		probe.SendSync(name, &runtime.CanAcceptWork{ToolID: tool.ID}, askTimeout)
		resp := probe.ExpectAnyMessage()
		result, ok := resp.(*runtime.CanAcceptWorkResult)
		require.True(t, ok)
		assert.True(t, result.Accept)
		probe.Stop()
	})

	t.Run("stops when journal is not running at PostStart", func(t *testing.T) {
		tool := validStdioTool("no-journal-tool")
		system, stop := testActorSystemWithTools(t, tool)
		defer stop()

		// No journal spawned — supervisor must stop itself during PostStart.
		name := mcp.ToolSupervisorName(tool.ID)
		pid, err := system.Spawn(ctx, name, newToolSupervisor())
		require.NoError(t, err)
		waitForActors()

		assert.False(t, pid.IsRunning())
	})

	t.Run("stops when tool config extension is absent", func(t *testing.T) {
		// System created WITHOUT ToolConfigExtension — supervisor must stop itself.
		cfg := testConfig()
		cfg.Audit.Sink = audit.NewMemorySink()
		system, stop := testActorSystem(t, goaktactor.WithExtensions(actorextension.NewConfigExtension(cfg)))
		defer stop()

		_, err := system.Spawn(ctx, mcp.ActorNameJournal, newJournaler())
		require.NoError(t, err)

		tool := validStdioTool("no-ext-tool")
		name := mcp.ToolSupervisorName(tool.ID)
		pid, err := system.Spawn(ctx, name, newToolSupervisor())
		require.NoError(t, err)
		waitForActors()

		assert.False(t, pid.IsRunning())
	})

	t.Run("stops when tool is not registered in extension", func(t *testing.T) {
		// ToolConfigExtension registered but empty (tool never registered).
		cfg := testConfig()
		cfg.Audit.Sink = audit.NewMemorySink()
		system, stop := testActorSystem(t,
			goaktactor.WithExtensions(actorextension.NewToolConfigExtension(), actorextension.NewConfigExtension(cfg)),
		)
		defer stop()

		_, err := system.Spawn(ctx, mcp.ActorNameJournal, newJournaler())
		require.NoError(t, err)

		tool := validStdioTool("unregistered-tool")
		name := mcp.ToolSupervisorName(tool.ID)
		pid, err := system.Spawn(ctx, name, newToolSupervisor())
		require.NoError(t, err)
		waitForActors()

		assert.False(t, pid.IsRunning())
	})

	t.Run("circuit open records CircuitState metric when metrics are registered", func(t *testing.T) {
		meter := noopmetric.NewMeterProvider().Meter("test")
		_, err := telemetry.RegisterMetrics(meter)
		require.NoError(t, err)
		t.Cleanup(telemetry.UnregisterMetrics)

		tool := validStdioTool("metrics-circuit-tool")
		toolCfgExt := actorextension.NewToolConfigExtension()
		toolCfgExt.Register(tool)
		cfg := testConfig()
		cfg.Audit.Sink = audit.NewMemorySink()
		system, stop := testActorSystem(t, goaktactor.WithExtensions(toolCfgExt, actorextension.NewConfigExtension(cfg)))
		defer stop()

		_, err = system.Spawn(ctx, mcp.ActorNameJournal, newJournaler())
		require.NoError(t, err)

		name := mcp.ToolSupervisorName(tool.ID)
		pid, err := system.Spawn(ctx, name, newToolSupervisor())
		require.NoError(t, err)
		waitForActors()

		for i := 0; i < mcp.DefaultCircuitFailureThreshold; i++ {
			require.NoError(t, goaktactor.Tell(ctx, pid, &runtime.ReportFailure{ToolID: tool.ID}))
		}
		waitForActors()
	})

	t.Run("circuit open emits audit event when journal is running", func(t *testing.T) {
		tool := validStdioTool("circuit-audit-tool")
		sink := audit.NewMemorySink()
		cfg := testConfig()
		cfg.Audit.Sink = sink
		toolCfgExt := actorextension.NewToolConfigExtension()
		toolCfgExt.Register(tool)
		system, stop := testActorSystem(t, goaktactor.WithExtensions(toolCfgExt, actorextension.NewConfigExtension(cfg)))
		defer stop()

		_, err := system.Spawn(ctx, mcp.ActorNameJournal, newJournaler())
		require.NoError(t, err)
		waitForActors()

		name := mcp.ToolSupervisorName(tool.ID)
		pid, err := system.Spawn(ctx, name, newToolSupervisor())
		require.NoError(t, err)
		waitForActors()

		for i := 0; i < mcp.DefaultCircuitFailureThreshold; i++ {
			require.NoError(t, goaktactor.Tell(ctx, pid, &runtime.ReportFailure{ToolID: tool.ID}))
		}
		waitForActors()

		events := sink.Events()
		require.NotEmpty(t, events, "expected circuit state change audit event")
		var circuitEvent *mcp.AuditEvent
		for _, e := range events {
			if e.Type == mcp.AuditEventTypeCircuitStateChange {
				circuitEvent = e
				break
			}
		}
		require.NotNil(t, circuitEvent)
		assert.Equal(t, string(tool.ID), circuitEvent.ToolID)
		assert.Equal(t, string(mcp.CircuitOpen), circuitEvent.Outcome)
	})
}
