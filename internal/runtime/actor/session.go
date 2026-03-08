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
	"time"

	goaktactor "github.com/tochemey/goakt/v4/actor"
	goaktlog "github.com/tochemey/goakt/v4/log"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	actorextension "github.com/tochemey/goakt-mcp/internal/runtime/actor/extension"
	"github.com/tochemey/goakt-mcp/internal/runtime/config"
	"github.com/tochemey/goakt-mcp/mcp"
)

// session is the SessionActor.
//
// There is one session per tenant+client+tool combination. The session owns
// lifecycle, request sequencing, and passivation. It serializes invocations
// through its mailbox (one message at a time).
//
// Spawn: ToolSupervisorActor spawns SessionActor in handleGetOrCreateSession via
// ctx.Self().SpawnChild(ctx, name, newSessionActor(), WithDependencies(sessionDep))
// as a child of the ToolSupervisorActor. Created on demand when GetOrCreateSession
// is received and no session exists for the tenant+client+tool triple. Uses
// passivation (TimeBasedStrategy) for idle session reclamation.
//
// Relocation: Follows ToolSupervisorActor. Runs on the same node as its parent.
// In cluster mode, that node is the Registry singleton host. SessionActor does
// not independently relocate; it is recreated if its parent (supervisor) is
// recreated after a Registry relocation.
//
// Passivation uses GoAkt's built-in TimeBasedStrategy (configured at spawn).
// PausePassivation and ResumePassivation protect in-flight work from premature
// passivation when invocations take longer than the idle timeout.
//
// Transport binding: when an ExecutorFactory extension is registered, sessions
// receive a real executor (stdio or HTTP) from the factory. Otherwise stub mode.
//
// All fields are unexported to enforce actor immutability rules.
type session struct {
	tenantID mcp.TenantID
	clientID mcp.ClientID
	toolID   mcp.ToolID
	tool     mcp.Tool
	executor mcp.ToolExecutor
	logger   goaktlog.Logger
}

// enforce that sessionActor satisfies the GoAkt Actor interface at compile time.
var _ goaktactor.Actor = (*session)(nil)

// newSession creates a SessionActor instance.
// Identity and tool are injected via WithDependencies when spawning.
func newSession() *session {
	return &session{}
}

// PreStart resolves the session dependency and initializes state.
func (x *session) PreStart(ctx *goaktactor.Context) error {
	x.logger = ctx.Logger()
	dependency := ctx.Dependency(actorextension.SessionDependencyID)

	if dependency != nil {
		if dep, ok := dependency.(*actorextension.SessionDependency); ok {
			x.tenantID = dep.TenantID()
			x.clientID = dep.ClientID()
			x.toolID = dep.ToolID()
			x.tool = dep.Tool()
			x.executor = dep.Executor()
		}
	}

	if x.toolID.IsZero() {
		return mcp.NewRuntimeError(mcp.ErrCodeInternal, "session dependency not found")
	}

	x.logger.Infof("actor session:%s-%s-%s started", x.tenantID, x.clientID, x.toolID)
	return nil
}

// Receive handles messages delivered to SessionActor.
func (x *session) Receive(ctx *goaktactor.ReceiveContext) {
	switch msg := ctx.Message().(type) {
	case *goaktactor.PostStart:
		x.logger.Debugf("actor session:%s-%s-%s post-start", x.tenantID, x.clientID, x.toolID)
	case *runtime.SessionInvoke:
		x.handleSessionInvoke(ctx, msg)
	default:
		ctx.Unhandled()
	}
}

// PostStop performs cleanup after SessionActor has stopped.
// Closes the executor if one was in use.
func (x *session) PostStop(ctx *goaktactor.Context) error {
	if x.executor != nil {
		_ = x.executor.Close()
	}
	x.logger.Infof("actor session:%s-%s-%s stopped", x.tenantID, x.clientID, x.toolID)
	return nil
}

// handleSessionInvoke validates and executes a tool invocation through this session.
// Passivation is paused for the duration of the invocation to prevent the session
// from being reclaimed while work is in flight. When an executor is configured,
// it performs real MCP execution; otherwise a stub result is returned.
func (x *session) handleSessionInvoke(ctx *goaktactor.ReceiveContext, msg *runtime.SessionInvoke) {
	if msg.Invocation == nil {
		ctx.Response(&runtime.SessionInvokeResult{Err: mcp.NewRuntimeError(mcp.ErrCodeInvalidRequest, "invocation is required")})
		return
	}
	if msg.Invocation.ToolID != x.toolID {
		ctx.Response(&runtime.SessionInvokeResult{Err: mcp.NewRuntimeError(mcp.ErrCodeInvalidRequest, "tool ID mismatch")})
		return
	}

	// Pause passivation during invocation so we are not passivated while
	// waiting for transport or during processing.
	_ = goaktactor.Tell(ctx.Context(), ctx.Self(), &goaktactor.PausePassivation{})

	start := time.Now()
	var result *mcp.ExecutionResult

	if x.executor != nil {
		execCtx := ctx.Context()
		timeout := x.tool.RequestTimeout
		if timeout == 0 {
			timeout = config.DefaultRequestTimeout
		}
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(execCtx, timeout)
		defer cancel()

		var err error
		result, err = x.executor.Execute(execCtx, msg.Invocation)
		duration := time.Since(start)
		if err != nil {
			result = &mcp.ExecutionResult{
				Status:      mcp.ExecutionStatusFailure,
				Err:         mcp.WrapRuntimeError(mcp.ErrCodeInternal, "execution failed", err),
				Duration:    duration,
				Correlation: msg.Invocation.Correlation,
			}
		} else if result != nil && result.Duration == 0 {
			result.Duration = duration
		}
	} else {
		// Stub mode: no executor configured.
		result = &mcp.ExecutionResult{
			Status:      mcp.ExecutionStatusSuccess,
			Output:      map[string]any{},
			Duration:    time.Since(start),
			Correlation: msg.Invocation.Correlation,
		}
	}

	ctx.Response(&runtime.SessionInvokeResult{Result: result})

	// Resume passivation now that invocation is complete.
	_ = goaktactor.Tell(ctx.Context(), ctx.Self(), &goaktactor.ResumePassivation{})
}
