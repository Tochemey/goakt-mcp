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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goaktactor "github.com/tochemey/goakt/v4/actor"
	"github.com/tochemey/goakt/v4/passivation"

	"github.com/tochemey/goakt-mcp/mcp"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	actorextension "github.com/tochemey/goakt-mcp/internal/runtime/actor/extension"
	"github.com/tochemey/goakt-mcp/internal/runtime/audit"
)

func TestSessionActor(t *testing.T) {
	ctx := context.Background()

	t.Run("spawns with session dependency and accepts invocation", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		tool := validStdioTool("session-tool")
		tool.IdleTimeout = 5 * time.Minute

		dep := actorextension.NewSessionDependency("tenant1", "client1", tool.ID, tool, nil, nil)
		name := mcp.SessionName("tenant1", "client1", tool.ID)
		pid, err := system.Spawn(ctx, name, newSession(),
			goaktactor.WithDependencies(dep),
			goaktactor.WithPassivationStrategy(passivation.NewTimeBasedStrategy(tool.IdleTimeout)))
		require.NoError(t, err)
		require.NotNil(t, pid)

		waitForActors()

		inv := sessionInvocation(tool.ID, "tenant1", "client1")
		resp, err := goaktactor.Ask(ctx, pid, &runtime.SessionInvoke{Invocation: inv}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.SessionInvokeResult)
		require.True(t, ok)
		require.NoError(t, result.Err)
		require.NotNil(t, result.Result)
		assert.True(t, result.Result.Succeeded())
	})

	t.Run("rejects invocation with nil invocation", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		tool := validStdioTool("session-nil")
		tool.IdleTimeout = 5 * time.Minute

		dep := actorextension.NewSessionDependency("tenant1", "client1", tool.ID, tool, nil, nil)
		name := mcp.SessionName("tenant1", "client1", tool.ID)
		pid, err := system.Spawn(ctx, name, newSession(),
			goaktactor.WithDependencies(dep),
			goaktactor.WithPassivationStrategy(passivation.NewTimeBasedStrategy(tool.IdleTimeout)))
		require.NoError(t, err)
		waitForActors()

		resp, err := goaktactor.Ask(ctx, pid, &runtime.SessionInvoke{Invocation: nil}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.SessionInvokeResult)
		require.True(t, ok)
		require.Error(t, result.Err)
	})

	t.Run("rejects invocation with tool ID mismatch", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		tool := validStdioTool("session-tool")
		tool.IdleTimeout = 5 * time.Minute

		dep := actorextension.NewSessionDependency("tenant1", "client1", tool.ID, tool, nil, nil)
		name := mcp.SessionName("tenant1", "client1", tool.ID)
		pid, err := system.Spawn(ctx, name, newSession(),
			goaktactor.WithDependencies(dep),
			goaktactor.WithPassivationStrategy(passivation.NewTimeBasedStrategy(tool.IdleTimeout)))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation("other-tool", "tenant1", "client1")
		resp, err := goaktactor.Ask(ctx, pid, &runtime.SessionInvoke{Invocation: inv}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.SessionInvokeResult)
		require.True(t, ok)
		require.Error(t, result.Err)
	})

	t.Run("recovers executor on execution failure and retries", func(t *testing.T) {
		failingExec := &mockExecutor{err: errors.New("connection reset")}
		successExec := &mockExecutor{}
		factory := &mockExecutorFactory{executor: successExec}

		cfg := testConfig()
		cfg.Audit.Sink = audit.NewMemorySink()
		system, stop := testActorSystem(t,
			goaktactor.WithExtensions(
				actorextension.NewExecutorFactoryExtension(factory),
				actorextension.NewConfigExtension(cfg),
			),
		)
		defer stop()

		tool := validStdioTool("recovery-tool")
		tool.IdleTimeout = 5 * time.Minute

		dep := actorextension.NewSessionDependency("tenant1", "client1", tool.ID, tool, failingExec, nil)
		name := mcp.SessionName("tenant1", "client1", tool.ID)
		pid, err := system.Spawn(ctx, name, newSession(),
			goaktactor.WithDependencies(dep),
			goaktactor.WithPassivationStrategy(passivation.NewTimeBasedStrategy(tool.IdleTimeout)))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation(tool.ID, "tenant1", "client1")
		resp, err := goaktactor.Ask(ctx, pid, &runtime.SessionInvoke{Invocation: inv}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.SessionInvokeResult)
		require.True(t, ok)
		require.NotNil(t, result.Result)
		assert.True(t, result.Result.Succeeded(), "retry with recovered executor should succeed")
	})

	t.Run("executor recovery failure returns original error", func(t *testing.T) {
		failingExec := &mockExecutor{err: errors.New("connection reset")}
		factory := &mockExecutorFactory{err: errors.New("cannot create executor")}

		cfg := testConfig()
		cfg.Audit.Sink = audit.NewMemorySink()
		system, stop := testActorSystem(t,
			goaktactor.WithExtensions(
				actorextension.NewExecutorFactoryExtension(factory),
				actorextension.NewConfigExtension(cfg),
			),
		)
		defer stop()

		tool := validStdioTool("recovery-fail-tool")
		tool.IdleTimeout = 5 * time.Minute

		dep := actorextension.NewSessionDependency("tenant1", "client1", tool.ID, tool, failingExec, nil)
		name := mcp.SessionName("tenant1", "client1", tool.ID)
		pid, err := system.Spawn(ctx, name, newSession(),
			goaktactor.WithDependencies(dep),
			goaktactor.WithPassivationStrategy(passivation.NewTimeBasedStrategy(tool.IdleTimeout)))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation(tool.ID, "tenant1", "client1")
		resp, err := goaktactor.Ask(ctx, pid, &runtime.SessionInvoke{Invocation: inv}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.SessionInvokeResult)
		require.True(t, ok)
		require.NotNil(t, result.Result)
		assert.Equal(t, mcp.ExecutionStatusFailure, result.Result.Status)
	})

	t.Run("passivates after idle timeout", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		tool := validStdioTool("passivate-tool")
		tool.IdleTimeout = 200 * time.Millisecond

		dep := actorextension.NewSessionDependency("tenant1", "client1", tool.ID, tool, nil, nil)
		name := mcp.SessionName("tenant1", "client1", tool.ID)
		pid, err := system.Spawn(ctx, name, newSession(),
			goaktactor.WithDependencies(dep),
			goaktactor.WithPassivationStrategy(passivation.NewTimeBasedStrategy(tool.IdleTimeout)))
		require.NoError(t, err)
		waitForActors()

		assert.True(t, pid.IsRunning())

		// Wait for passivation (idle timeout + buffer)
		time.Sleep(400 * time.Millisecond)

		assert.False(t, pid.IsRunning(), "session should have passivated")
	})
}

func TestGetOrCreateSession(t *testing.T) {
	ctx := context.Background()

	t.Run("supervisor creates session and returns same session on reuse", func(t *testing.T) {
		cfg := testConfig()
		cfg.Audit.Sink = audit.NewMemorySink()
		system, stop := testActorSystem(t,
			goaktactor.WithExtensions(actorextension.NewToolConfigExtension(), actorextension.NewConfigExtension(cfg)),
		)
		defer stop()

		_, err := system.Spawn(ctx, mcp.ActorNameJournal, newJournaler())
		require.NoError(t, err)

		// Use Registrar flow (matches production) so supervisor is child of Registrar.
		regPID, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		tool := validStdioTool("get-or-create-tool")
		tool.IdleTimeout = 5 * time.Minute

		_, err = goaktactor.Ask(ctx, regPID, &runtime.RegisterTool{Tool: tool}, askTimeout)
		require.NoError(t, err)
		waitForActors()

		supResp, err := goaktactor.Ask(ctx, regPID, &runtime.GetSupervisor{ToolID: tool.ID}, askTimeout)
		require.NoError(t, err)
		supResult, ok := supResp.(*runtime.GetSupervisorResult)
		require.True(t, ok)
		require.True(t, supResult.Found)
		supPID, ok := supResult.Supervisor.(*goaktactor.PID)
		require.True(t, ok)
		require.True(t, supPID.IsRunning())

		req := &runtime.GetOrCreateSession{
			TenantID: "tenant1",
			ClientID: "client1",
			ToolID:   tool.ID,
		}

		resp, err := goaktactor.Ask(ctx, supPID, req, askTimeout)
		require.NoError(t, err)
		gsResult, ok := resp.(*runtime.GetOrCreateSessionResult)
		require.True(t, ok)
		require.NoError(t, gsResult.Err)
		require.True(t, gsResult.Found)
		require.NotNil(t, gsResult.Session)

		sessionPID, ok := gsResult.Session.(*goaktactor.PID)
		require.True(t, ok)
		waitForActors() // allow session to finish startup before asserting
		assert.True(t, sessionPID.IsRunning())

		// Reuse: ask again for same tenant+client+tool
		resp2, err := goaktactor.Ask(ctx, supPID, req, askTimeout)
		require.NoError(t, err)
		gsResult2, ok := resp2.(*runtime.GetOrCreateSessionResult)
		require.True(t, ok)
		require.NoError(t, gsResult2.Err)
		require.True(t, gsResult2.Found)
		sessionPID2, ok := gsResult2.Session.(*goaktactor.PID)
		require.True(t, ok)
		assert.Equal(t, sessionPID.Name(), sessionPID2.Name(), "same session should be returned")
	})

	t.Run("supervisor rejects GetOrCreateSession when tool ID mismatch", func(t *testing.T) {
		tool := validStdioTool("mismatch-tool")
		cfg := testConfig()
		cfg.Audit.Sink = audit.NewMemorySink()
		toolCfgExt := actorextension.NewToolConfigExtension()
		toolCfgExt.Register(tool)
		system, stop := testActorSystem(t, goaktactor.WithExtensions(toolCfgExt, actorextension.NewConfigExtension(cfg)))
		defer stop()

		_, err := system.Spawn(ctx, mcp.ActorNameJournal, newJournaler())
		require.NoError(t, err)

		name := mcp.ToolSupervisorName(tool.ID)
		supPID, err := system.Spawn(ctx, name, newToolSupervisor())
		require.NoError(t, err)
		waitForActors()

		req := &runtime.GetOrCreateSession{
			TenantID: "tenant1",
			ClientID: "client1",
			ToolID:   "other-tool",
		}

		resp, err := goaktactor.Ask(ctx, supPID, req, askTimeout)
		require.NoError(t, err)
		gsResult, ok := resp.(*runtime.GetOrCreateSessionResult)
		require.True(t, ok)
		require.Error(t, gsResult.Err)
	})
}

func TestSessionDependencyMarshalUnmarshal(t *testing.T) {
	tool := validStdioTool("marshal-tool")
	dep := actorextension.NewSessionDependency("tenant-1", "client-1", "marshal-tool", tool, nil, nil)

	data, err := dep.MarshalBinary()
	require.NoError(t, err)
	require.NotEmpty(t, data)

	dep2 := &actorextension.SessionDependency{}
	err = dep2.UnmarshalBinary(data)
	require.NoError(t, err)
	assert.Equal(t, "tenant-1", string(dep2.TenantID()))
	assert.Equal(t, "client-1", string(dep2.ClientID()))
	assert.Equal(t, "marshal-tool", string(dep2.ToolID()))
}
