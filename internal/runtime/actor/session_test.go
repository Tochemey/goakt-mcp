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
	noopmetric "go.opentelemetry.io/otel/metric/noop"

	"github.com/tochemey/goakt-mcp/mcp"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	actorextension "github.com/tochemey/goakt-mcp/internal/runtime/actor/extension"
	"github.com/tochemey/goakt-mcp/internal/runtime/audit"
	"github.com/tochemey/goakt-mcp/internal/runtime/telemetry"
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

	t.Run("recovers executor on transport failure result (nil Go error)", func(t *testing.T) {
		// Simulate how the built-in executors surface transport failures:
		// result.Err has ErrCodeTransportFailure, but the Go error is nil.
		transportFailExec := &mockExecutor{
			result: &mcp.ExecutionResult{
				Status: mcp.ExecutionStatusFailure,
				Err:    mcp.NewRuntimeError(mcp.ErrCodeTransportFailure, "connection reset"),
			},
		}
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

		tool := validStdioTool("transport-fail-tool")
		tool.IdleTimeout = 5 * time.Minute

		dep := actorextension.NewSessionDependency("tenant1", "client1", tool.ID, tool, transportFailExec, nil)
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
		assert.True(t, result.Result.Succeeded(), "retry after transport failure recovery should succeed")
	})

	t.Run("no recovery attempted for non-transport result errors", func(t *testing.T) {
		// An executor that returns a failure result with a non-transport error code
		// should NOT trigger recovery.
		nonTransportExec := &mockExecutor{
			result: &mcp.ExecutionResult{
				Status: mcp.ExecutionStatusFailure,
				Err:    mcp.NewRuntimeError(mcp.ErrCodeInternal, "tool logic error"),
			},
		}

		system, stop := testActorSystem(t)
		defer stop()

		tool := validStdioTool("no-recovery-tool")
		tool.IdleTimeout = 5 * time.Minute

		dep := actorextension.NewSessionDependency("tenant1", "client1", tool.ID, tool, nonTransportExec, nil)
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
		// The original failure is returned unchanged — no recovery attempted
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

func TestSessionGetIdentity(t *testing.T) {
	ctx := context.Background()
	system, stop := testActorSystem(t)
	defer stop()

	tool := validStdioTool("identity-tool")
	tool.IdleTimeout = 5 * time.Minute

	dep := actorextension.NewSessionDependency("tenant-id", "client-id", tool.ID, tool, nil, nil)
	name := mcp.SessionName("tenant-id", "client-id", tool.ID)
	pid, err := system.Spawn(ctx, name, newSession(),
		goaktactor.WithDependencies(dep),
		goaktactor.WithPassivationStrategy(passivation.NewTimeBasedStrategy(tool.IdleTimeout)))
	require.NoError(t, err)
	waitForActors()

	resp, err := goaktactor.Ask(ctx, pid, &runtime.GetSessionIdentity{}, askTimeout)
	require.NoError(t, err)
	result, ok := resp.(*runtime.GetSessionIdentityResult)
	require.True(t, ok)
	assert.Equal(t, mcp.TenantID("tenant-id"), result.TenantID)
	assert.Equal(t, mcp.ClientID("client-id"), result.ClientID)
	assert.Equal(t, tool.ID, result.ToolID)
}

func TestIsTransportFailure(t *testing.T) {
	t.Run("nil result is not a transport failure", func(t *testing.T) {
		assert.False(t, isTransportFailure(nil))
	})

	t.Run("result with nil error is not a transport failure", func(t *testing.T) {
		r := &mcp.ExecutionResult{Status: mcp.ExecutionStatusSuccess}
		assert.False(t, isTransportFailure(r))
	})

	t.Run("result with non-transport error is not a transport failure", func(t *testing.T) {
		r := &mcp.ExecutionResult{
			Status: mcp.ExecutionStatusFailure,
			Err:    mcp.NewRuntimeError(mcp.ErrCodeInternal, "logic error"),
		}
		assert.False(t, isTransportFailure(r))
	})

	t.Run("result with transport failure error is a transport failure", func(t *testing.T) {
		r := &mcp.ExecutionResult{
			Status: mcp.ExecutionStatusFailure,
			Err:    mcp.NewRuntimeError(mcp.ErrCodeTransportFailure, "connection reset"),
		}
		assert.True(t, isTransportFailure(r))
	})
}

func TestSessionInvokeStream(t *testing.T) {
	ctx := context.Background()

	t.Run("rejects nil invocation", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		tool := validStdioTool("stream-nil")
		tool.IdleTimeout = 5 * time.Minute

		dep := actorextension.NewSessionDependency("tenant1", "client1", tool.ID, tool, nil, nil)
		name := mcp.SessionName("tenant1", "client1", tool.ID)
		pid, err := system.Spawn(ctx, name, newSession(),
			goaktactor.WithDependencies(dep),
			goaktactor.WithPassivationStrategy(passivation.NewTimeBasedStrategy(tool.IdleTimeout)))
		require.NoError(t, err)
		waitForActors()

		resp, err := goaktactor.Ask(ctx, pid, &runtime.SessionInvokeStream{Invocation: nil}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.SessionInvokeStreamResult)
		require.True(t, ok)
		require.Error(t, result.Err)
		assert.Contains(t, result.Err.Error(), "invocation is required")
	})

	t.Run("rejects tool ID mismatch", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		tool := validStdioTool("stream-mismatch")
		tool.IdleTimeout = 5 * time.Minute

		dep := actorextension.NewSessionDependency("tenant1", "client1", tool.ID, tool, nil, nil)
		name := mcp.SessionName("tenant1", "client1", tool.ID)
		pid, err := system.Spawn(ctx, name, newSession(),
			goaktactor.WithDependencies(dep),
			goaktactor.WithPassivationStrategy(passivation.NewTimeBasedStrategy(tool.IdleTimeout)))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation("other-tool", "tenant1", "client1")
		resp, err := goaktactor.Ask(ctx, pid, &runtime.SessionInvokeStream{Invocation: inv}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.SessionInvokeStreamResult)
		require.True(t, ok)
		require.Error(t, result.Err)
		assert.Contains(t, result.Err.Error(), "tool ID mismatch")
	})

	t.Run("fallback to Execute when executor is nil (stub mode)", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		tool := validStdioTool("stream-stub")
		tool.IdleTimeout = 5 * time.Minute

		dep := actorextension.NewSessionDependency("tenant1", "client1", tool.ID, tool, nil, nil)
		name := mcp.SessionName("tenant1", "client1", tool.ID)
		pid, err := system.Spawn(ctx, name, newSession(),
			goaktactor.WithDependencies(dep),
			goaktactor.WithPassivationStrategy(passivation.NewTimeBasedStrategy(tool.IdleTimeout)))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation(tool.ID, "tenant1", "client1")
		resp, err := goaktactor.Ask(ctx, pid, &runtime.SessionInvokeStream{Invocation: inv}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.SessionInvokeStreamResult)
		require.True(t, ok)
		require.NoError(t, result.Err)
		require.NotNil(t, result.Result)
		assert.True(t, result.Result.Succeeded())
	})

	t.Run("fallback to Execute when executor does not implement ToolStreamExecutor", func(t *testing.T) {
		exec := &mockExecutor{}
		system, stop := testActorSystem(t)
		defer stop()

		tool := validStdioTool("stream-no-iface")
		tool.IdleTimeout = 5 * time.Minute

		dep := actorextension.NewSessionDependency("tenant1", "client1", tool.ID, tool, exec, nil)
		name := mcp.SessionName("tenant1", "client1", tool.ID)
		pid, err := system.Spawn(ctx, name, newSession(),
			goaktactor.WithDependencies(dep),
			goaktactor.WithPassivationStrategy(passivation.NewTimeBasedStrategy(tool.IdleTimeout)))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation(tool.ID, "tenant1", "client1")
		resp, err := goaktactor.Ask(ctx, pid, &runtime.SessionInvokeStream{Invocation: inv}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.SessionInvokeStreamResult)
		require.True(t, ok)
		require.NoError(t, result.Err)
		require.NotNil(t, result.Result)
		assert.True(t, result.Result.Succeeded())
	})

	t.Run("fallback Execute error returns failure result", func(t *testing.T) {
		exec := &mockExecutor{err: errors.New("exec failed")}
		system, stop := testActorSystem(t)
		defer stop()

		tool := validStdioTool("stream-exec-fail")
		tool.IdleTimeout = 5 * time.Minute

		dep := actorextension.NewSessionDependency("tenant1", "client1", tool.ID, tool, exec, nil)
		name := mcp.SessionName("tenant1", "client1", tool.ID)
		pid, err := system.Spawn(ctx, name, newSession(),
			goaktactor.WithDependencies(dep),
			goaktactor.WithPassivationStrategy(passivation.NewTimeBasedStrategy(tool.IdleTimeout)))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation(tool.ID, "tenant1", "client1")
		resp, err := goaktactor.Ask(ctx, pid, &runtime.SessionInvokeStream{Invocation: inv}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.SessionInvokeStreamResult)
		require.True(t, ok)
		require.NoError(t, result.Err)
		require.NotNil(t, result.Result)
		assert.Equal(t, mcp.ExecutionStatusFailure, result.Result.Status)
	})

	t.Run("streaming executor returns StreamResult with progress and final", func(t *testing.T) {
		progCh := make(chan mcp.ProgressEvent, 1)
		finalCh := make(chan *mcp.ExecutionResult, 1)
		progCh <- mcp.ProgressEvent{Message: "step1"}
		close(progCh)
		finalCh <- &mcp.ExecutionResult{Status: mcp.ExecutionStatusSuccess, Output: map[string]any{}}
		close(finalCh)

		streamExec := &mockStreamExecutor{
			streamResult: &mcp.StreamingResult{
				Progress: progCh,
				Final:    finalCh,
			},
		}
		system, stop := testActorSystem(t)
		defer stop()

		tool := validStdioTool("stream-real")
		tool.IdleTimeout = 5 * time.Minute

		dep := actorextension.NewSessionDependency("tenant1", "client1", tool.ID, tool, streamExec, nil)
		name := mcp.SessionName("tenant1", "client1", tool.ID)
		pid, err := system.Spawn(ctx, name, newSession(),
			goaktactor.WithDependencies(dep),
			goaktactor.WithPassivationStrategy(passivation.NewTimeBasedStrategy(tool.IdleTimeout)))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation(tool.ID, "tenant1", "client1")
		resp, err := goaktactor.Ask(ctx, pid, &runtime.SessionInvokeStream{Invocation: inv}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.SessionInvokeStreamResult)
		require.True(t, ok)
		require.NoError(t, result.Err)
		require.NotNil(t, result.StreamResult)

		// Drain progress
		var progEvents []mcp.ProgressEvent
		for evt := range result.StreamResult.Progress {
			progEvents = append(progEvents, evt)
		}
		assert.Len(t, progEvents, 1)
		assert.Equal(t, "step1", progEvents[0].Message)

		// Read final
		final := <-result.StreamResult.Final
		require.NotNil(t, final)
		assert.Equal(t, mcp.ExecutionStatusSuccess, final.Status)
	})

	t.Run("streaming executor error returns failure result", func(t *testing.T) {
		streamExec := &mockStreamExecutor{err: errors.New("stream init failed")}
		system, stop := testActorSystem(t)
		defer stop()

		tool := validStdioTool("stream-init-fail")
		tool.IdleTimeout = 5 * time.Minute

		dep := actorextension.NewSessionDependency("tenant1", "client1", tool.ID, tool, streamExec, nil)
		name := mcp.SessionName("tenant1", "client1", tool.ID)
		pid, err := system.Spawn(ctx, name, newSession(),
			goaktactor.WithDependencies(dep),
			goaktactor.WithPassivationStrategy(passivation.NewTimeBasedStrategy(tool.IdleTimeout)))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation(tool.ID, "tenant1", "client1")
		resp, err := goaktactor.Ask(ctx, pid, &runtime.SessionInvokeStream{Invocation: inv}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.SessionInvokeStreamResult)
		require.True(t, ok)
		require.NoError(t, result.Err)
		require.NotNil(t, result.Result)
		assert.Equal(t, mcp.ExecutionStatusFailure, result.Result.Status)
	})
}

func TestSessionMetricsIntegration(t *testing.T) {
	ctx := context.Background()

	t.Run("session lifecycle records metrics when registered", func(t *testing.T) {
		meter := noopmetric.NewMeterProvider().Meter("test")
		_, err := telemetry.RegisterMetrics(meter)
		require.NoError(t, err)
		t.Cleanup(telemetry.UnregisterMetrics)

		system, stop := testActorSystem(t)
		defer stop()

		tool := validStdioTool("metrics-session")
		tool.IdleTimeout = 5 * time.Minute

		dep := actorextension.NewSessionDependency("tenant1", "client1", tool.ID, tool, nil, nil)
		name := mcp.SessionName("tenant1", "client1", tool.ID)
		pid, err := system.Spawn(ctx, name, newSession(),
			goaktactor.WithDependencies(dep),
			goaktactor.WithPassivationStrategy(passivation.NewTimeBasedStrategy(tool.IdleTimeout)))
		require.NoError(t, err)
		waitForActors()

		assert.True(t, pid.IsRunning())

		// Stop the actor to trigger PostStop → RecordSessionDestroyed
		require.NoError(t, pid.Shutdown(ctx))
		waitForActors()
		assert.False(t, pid.IsRunning())
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
