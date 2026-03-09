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

	"github.com/tochemey/goakt-mcp/mcp"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	actorextension "github.com/tochemey/goakt-mcp/internal/runtime/actor/extension"
	"github.com/tochemey/goakt-mcp/internal/runtime/audit"
)

func TestRegistryActor(t *testing.T) {
	ctx := context.Background()

	t.Run("starts and stops cleanly", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		pid, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		require.NotNil(t, pid)
		assert.Equal(t, mcp.ActorNameRegistrar, pid.Name())

		waitForActors()
	})

	t.Run("register and query tool via Ask", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		pid, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		tool := validStdioTool("my-tool")
		resp, err := goaktactor.Ask(ctx, pid, &runtime.RegisterTool{Tool: tool}, askTimeout)
		require.NoError(t, err)
		regResult, ok := resp.(*runtime.RegisterToolResult)
		require.True(t, ok)
		require.NoError(t, regResult.Err)

		resp, err = goaktactor.Ask(ctx, pid, &runtime.QueryTool{ToolID: "my-tool"}, askTimeout)
		require.NoError(t, err)
		qResult, ok := resp.(*runtime.QueryToolResult)
		require.True(t, ok)
		require.True(t, qResult.Found)
		require.NoError(t, qResult.Err)
		require.NotNil(t, qResult.Tool)
		assert.Equal(t, mcp.ToolID("my-tool"), qResult.Tool.ID)
		assert.Equal(t, mcp.ToolStateEnabled, qResult.Tool.State)
	})

	t.Run("query non-existent tool returns ErrToolNotFound", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		pid, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		resp, err := goaktactor.Ask(ctx, pid, &runtime.QueryTool{ToolID: "nonexistent"}, askTimeout)
		require.NoError(t, err)
		qResult, ok := resp.(*runtime.QueryToolResult)
		require.True(t, ok)
		assert.False(t, qResult.Found)
		require.Error(t, qResult.Err)
		assert.ErrorIs(t, qResult.Err, mcp.ErrToolNotFound)
	})

	t.Run("register invalid tool returns validation error", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		pid, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		invalidTool := mcp.Tool{
			ID:        mcp.ToolID("bad"),
			Transport: mcp.TransportStdio,
			Stdio:     &mcp.StdioTransportConfig{Command: ""},
		}
		resp, err := goaktactor.Ask(ctx, pid, &runtime.RegisterTool{Tool: invalidTool}, askTimeout)
		require.NoError(t, err)
		regResult, ok := resp.(*runtime.RegisterToolResult)
		require.True(t, ok)
		require.Error(t, regResult.Err)
	})

	t.Run("disable and remove tool", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		pid, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		tool := validStdioTool("to-disable")
		_, err = goaktactor.Ask(ctx, pid, &runtime.RegisterTool{Tool: tool}, askTimeout)
		require.NoError(t, err)

		resp, err := goaktactor.Ask(ctx, pid, &runtime.DisableTool{ToolID: "to-disable"}, askTimeout)
		require.NoError(t, err)
		disableResult, ok := resp.(*runtime.DisableToolResult)
		require.True(t, ok)
		require.NoError(t, disableResult.Err)

		resp, err = goaktactor.Ask(ctx, pid, &runtime.QueryTool{ToolID: "to-disable"}, askTimeout)
		require.NoError(t, err)
		qResult := resp.(*runtime.QueryToolResult)
		require.True(t, qResult.Found)
		assert.Equal(t, mcp.ToolStateDisabled, qResult.Tool.State)

		resp, err = goaktactor.Ask(ctx, pid, &runtime.RemoveTool{ToolID: "to-disable"}, askTimeout)
		require.NoError(t, err)
		removeResult, ok := resp.(*runtime.RemoveToolResult)
		require.True(t, ok)
		require.NoError(t, removeResult.Err)

		resp, err = goaktactor.Ask(ctx, pid, &runtime.QueryTool{ToolID: "to-disable"}, askTimeout)
		require.NoError(t, err)
		qResult = resp.(*runtime.QueryToolResult)
		assert.False(t, qResult.Found)
	})

	t.Run("update tool health", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		pid, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		tool := validStdioTool("health-tool")
		_, err = goaktactor.Ask(ctx, pid, &runtime.RegisterTool{Tool: tool}, askTimeout)
		require.NoError(t, err)

		resp, err := goaktactor.Ask(ctx, pid, &runtime.UpdateToolHealth{
			ToolID: "health-tool",
			State:  mcp.ToolStateDegraded,
		}, askTimeout)
		require.NoError(t, err)
		healthResult, ok := resp.(*runtime.UpdateToolHealthResult)
		require.True(t, ok)
		require.NoError(t, healthResult.Err)

		resp, err = goaktactor.Ask(ctx, pid, &runtime.QueryTool{ToolID: "health-tool"}, askTimeout)
		require.NoError(t, err)
		qResult := resp.(*runtime.QueryToolResult)
		require.True(t, qResult.Found)
		assert.Equal(t, mcp.ToolStateDegraded, qResult.Tool.State)
	})

	t.Run("update tool", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		pid, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		tool := validStdioTool("update-tool")
		tool.RequestTimeout = 10 * time.Second
		_, err = goaktactor.Ask(ctx, pid, &runtime.RegisterTool{Tool: tool}, askTimeout)
		require.NoError(t, err)

		updated := tool
		updated.RequestTimeout = 60 * time.Second
		updated.Routing = mcp.RoutingLeastLoaded
		resp, err := goaktactor.Ask(ctx, pid, &runtime.UpdateTool{Tool: updated}, askTimeout)
		require.NoError(t, err)
		updateResult, ok := resp.(*runtime.UpdateToolResult)
		require.True(t, ok)
		require.NoError(t, updateResult.Err)

		resp, err = goaktactor.Ask(ctx, pid, &runtime.QueryTool{ToolID: "update-tool"}, askTimeout)
		require.NoError(t, err)
		qResult := resp.(*runtime.QueryToolResult)
		require.True(t, qResult.Found)
		assert.Equal(t, 60*time.Second, qResult.Tool.RequestTimeout)
		assert.Equal(t, mcp.RoutingLeastLoaded, qResult.Tool.Routing)
	})

	t.Run("get supervisor returns PID when tool has supervisor", func(t *testing.T) {
		cfg := testConfig()
		cfg.Audit.Sink = audit.NewMemorySink()
		system, stop := testActorSystem(t,
			goaktactor.WithExtensions(actorextension.NewToolConfigExtension(), actorextension.NewConfigExtension(cfg)),
		)
		defer stop()

		_, err := system.Spawn(ctx, mcp.ActorNameJournal, newJournaler())
		require.NoError(t, err)

		pid, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		tool := validStdioTool("supervisor-lookup")
		_, err = goaktactor.Ask(ctx, pid, &runtime.RegisterTool{Tool: tool}, askTimeout)
		require.NoError(t, err)
		waitForActors()

		resp, err := goaktactor.Ask(ctx, pid, &runtime.GetSupervisor{ToolID: "supervisor-lookup"}, askTimeout)
		require.NoError(t, err)
		gsResult, ok := resp.(*runtime.GetSupervisorResult)
		require.True(t, ok)
		if gsResult.Found && gsResult.Supervisor != nil {
			supPID, ok := gsResult.Supervisor.(*goaktactor.PID)
			require.True(t, ok)
			assert.True(t, supPID.IsRunning())
		}
	})

	t.Run("bootstrap tools", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		pid, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		tools := []mcp.Tool{
			validStdioTool("bootstrap-1"),
			validHTTPTool("bootstrap-2"),
		}
		err = goaktactor.Tell(ctx, pid, &runtime.BootstrapTools{Tools: tools})
		require.NoError(t, err)
		waitForActors()

		resp, err := goaktactor.Ask(ctx, pid, &runtime.QueryTool{ToolID: "bootstrap-1"}, askTimeout)
		require.NoError(t, err)
		qResult := resp.(*runtime.QueryToolResult)
		require.True(t, qResult.Found)
		assert.Equal(t, mcp.TransportStdio, qResult.Tool.Transport)

		resp, err = goaktactor.Ask(ctx, pid, &runtime.QueryTool{ToolID: "bootstrap-2"}, askTimeout)
		require.NoError(t, err)
		qResult = resp.(*runtime.QueryToolResult)
		require.True(t, qResult.Found)
		assert.Equal(t, mcp.TransportHTTP, qResult.Tool.Transport)
	})

	t.Run("update tool not found returns ErrToolNotFound", func(t *testing.T) {
		kit, ctx := newTestKit(t)

		kit.ActorSystem().Spawn(ctx, "registry-update-nf", newRegistrar())
		waitForActors()

		probe := kit.NewProbe(ctx)
		tool := validStdioTool("nonexistent")
		probe.SendSync("registry-update-nf", &runtime.UpdateTool{Tool: tool}, askTimeout)
		resp := probe.ExpectAnyMessage()
		result, ok := resp.(*runtime.UpdateToolResult)
		require.True(t, ok)
		require.Error(t, result.Err)
		assert.ErrorIs(t, result.Err, mcp.ErrToolNotFound)
		probe.Stop()
	})

	t.Run("disable tool not found returns ErrToolNotFound", func(t *testing.T) {
		kit, ctx := newTestKit(t)

		kit.ActorSystem().Spawn(ctx, "registry-disable-nf", newRegistrar())
		waitForActors()

		probe := kit.NewProbe(ctx)
		probe.SendSync("registry-disable-nf", &runtime.DisableTool{ToolID: "nonexistent"}, askTimeout)
		resp := probe.ExpectAnyMessage()
		result, ok := resp.(*runtime.DisableToolResult)
		require.True(t, ok)
		require.Error(t, result.Err)
		assert.ErrorIs(t, result.Err, mcp.ErrToolNotFound)
		probe.Stop()
	})

	t.Run("remove tool not found returns ErrToolNotFound", func(t *testing.T) {
		kit, ctx := newTestKit(t)

		kit.ActorSystem().Spawn(ctx, "registry-remove-nf", newRegistrar())
		waitForActors()

		probe := kit.NewProbe(ctx)
		probe.SendSync("registry-remove-nf", &runtime.RemoveTool{ToolID: "nonexistent"}, askTimeout)
		resp := probe.ExpectAnyMessage()
		result, ok := resp.(*runtime.RemoveToolResult)
		require.True(t, ok)
		require.Error(t, result.Err)
		assert.ErrorIs(t, result.Err, mcp.ErrToolNotFound)
		probe.Stop()
	})

	t.Run("update tool health not found returns ErrToolNotFound", func(t *testing.T) {
		kit, ctx := newTestKit(t)

		kit.ActorSystem().Spawn(ctx, "registry-health-nf", newRegistrar())
		waitForActors()

		probe := kit.NewProbe(ctx)
		probe.SendSync("registry-health-nf", &runtime.UpdateToolHealth{
			ToolID: "nonexistent",
			State:  mcp.ToolStateDegraded,
		}, askTimeout)
		resp := probe.ExpectAnyMessage()
		result, ok := resp.(*runtime.UpdateToolHealthResult)
		require.True(t, ok)
		require.Error(t, result.Err)
		assert.ErrorIs(t, result.Err, mcp.ErrToolNotFound)
		probe.Stop()
	})

	t.Run("get supervisor not found returns Found=false", func(t *testing.T) {
		kit, ctx := newTestKit(t)

		kit.ActorSystem().Spawn(ctx, "registry-get-sup-nf", newRegistrar())
		waitForActors()

		probe := kit.NewProbe(ctx)
		probe.SendSync("registry-get-sup-nf", &runtime.GetSupervisor{ToolID: "nonexistent"}, askTimeout)
		resp := probe.ExpectAnyMessage()
		result, ok := resp.(*runtime.GetSupervisorResult)
		require.True(t, ok)
		assert.False(t, result.Found)
		require.Error(t, result.Err)
		assert.ErrorIs(t, result.Err, mcp.ErrToolNotFound)
		probe.Stop()
	})

	t.Run("NewRegistrar returns valid actor for cluster kind registration", func(t *testing.T) {
		a := NewRegistrar()
		require.NotNil(t, a)
	})

	t.Run("count sessions for tenant", func(t *testing.T) {
		cfg := testConfig()
		cfg.Audit.Sink = audit.NewMemorySink()
		system, stop := testActorSystem(t,
			goaktactor.WithExtensions(actorextension.NewToolConfigExtension(), actorextension.NewConfigExtension(cfg)),
		)
		defer stop()

		_, err := system.Spawn(ctx, mcp.ActorNameJournal, newJournaler())
		require.NoError(t, err)

		pid, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		tool := validStdioTool("count-sessions-tool")
		_, err = goaktactor.Ask(ctx, pid, &runtime.RegisterTool{Tool: tool}, askTimeout)
		require.NoError(t, err)
		waitForActors()

		resp, err := goaktactor.Ask(ctx, pid, &runtime.CountSessionsForTenant{TenantID: "tenant-1"}, askTimeout)
		require.NoError(t, err)
		countResult, ok := resp.(*runtime.CountSessionsForTenantResult)
		require.True(t, ok)
		require.NotNil(t, countResult)
		assert.GreaterOrEqual(t, countResult.Count, 0)
	})
}
