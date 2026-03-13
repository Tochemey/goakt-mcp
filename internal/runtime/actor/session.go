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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/tochemey/goakt-mcp/mcp"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	actorextension "github.com/tochemey/goakt-mcp/internal/runtime/actor/extension"
	"github.com/tochemey/goakt-mcp/internal/runtime/config"
	"github.com/tochemey/goakt-mcp/internal/runtime/telemetry"
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
	tenantID    mcp.TenantID
	clientID    mcp.ClientID
	toolID      mcp.ToolID
	tool        mcp.Tool
	executor    mcp.ToolExecutor
	credentials map[string]string
	logger      goaktlog.Logger
}

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
			x.credentials = dep.Credentials()
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
	case *runtime.GetSessionIdentity:
		ctx.Response(&runtime.GetSessionIdentityResult{
			TenantID: x.tenantID,
			ClientID: x.clientID,
			ToolID:   x.toolID,
		})
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

	corr := &telemetry.CorrelationFields{
		TenantID:  msg.Invocation.Correlation.TenantID,
		ClientID:  msg.Invocation.Correlation.ClientID,
		RequestID: msg.Invocation.Correlation.RequestID,
		TraceID:   msg.Invocation.Correlation.TraceID,
		ToolID:    msg.Invocation.ToolID,
	}

	log := x.logger
	if kvs := corr.LogKeyValues(); len(kvs) > 0 {
		log = x.logger.With(kvs...)
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
		if telemetry.TracingEnabled() {
			var span trace.Span
			execCtx, span = telemetry.Tracer().Start(execCtx, "goaktmcp.session.execute",
				trace.WithAttributes(
					attribute.String("mcp.tool_id", string(x.toolID)),
					attribute.String("mcp.tenant_id", string(x.tenantID)),
					attribute.String("mcp.client_id", string(x.clientID)),
				),
				trace.WithSpanKind(trace.SpanKindInternal),
			)
			defer func() {
				if err != nil {
					span.SetStatus(codes.Error, err.Error())
					span.RecordError(err)
				}
				span.End()
			}()
		}

		result, err = x.executor.Execute(execCtx, msg.Invocation)
		duration := time.Since(start)
		if err != nil {
			log.Warnf("actor session:%s-%s-%s execution failed, attempting recovery: %v", x.tenantID, x.clientID, x.toolID, err)
			if recovered := x.tryRecoverExecutor(ctx); recovered {
				log.Infof("actor session:%s-%s-%s executor recovered, retrying", x.tenantID, x.clientID, x.toolID)
				// Create a fresh timeout context for the retry — the original
				// execCtx may be nearly exhausted or already cancelled.
				retryCtx, retryCancel := context.WithTimeout(ctx.Context(), timeout)
				defer retryCancel()
				retryStart := time.Now()
				result, err = x.executor.Execute(retryCtx, msg.Invocation)
				duration = time.Since(retryStart)
			}
			if err != nil {
				log.Warnf("actor session:%s-%s-%s execution failed: %v", x.tenantID, x.clientID, x.toolID, err)
				result = &mcp.ExecutionResult{
					Status:      mcp.ExecutionStatusFailure,
					Err:         mcp.WrapRuntimeError(mcp.ErrCodeInternal, "execution failed", err),
					Duration:    duration,
					Correlation: msg.Invocation.Correlation,
				}
			}
		}
		if err == nil && result != nil && result.Duration == 0 {
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

	// Report success or failure to the supervisor for circuit breaker state.
	// The session is a child of ToolSupervisor; Parent() returns the supervisor PID.
	x.reportOutcomeToSupervisor(ctx, result)

	// Resume passivation now that invocation is complete.
	_ = goaktactor.Tell(ctx.Context(), ctx.Self(), &goaktactor.ResumePassivation{})
}

// tryRecoverExecutor attempts to replace a failed executor with a fresh one
// created via the ExecutorFactory extension. Returns true if a new executor was
// successfully created and installed. The old executor is closed before replacement.
func (x *session) tryRecoverExecutor(ctx *goaktactor.ReceiveContext) bool {
	ext := ctx.Extension(actorextension.ExecutorFactoryExtensionID)
	ef, ok := ext.(*actorextension.ExecutorFactoryExtension)
	if !ok || ef == nil {
		return false
	}

	newExec, err := ef.Factory().Create(ctx.Context(), x.tool, x.credentials)
	if err != nil {
		x.logger.Warnf("actor session:%s-%s-%s executor recovery failed: %v", x.tenantID, x.clientID, x.toolID, err)
		return false
	}

	// Close the old executor before replacing.
	if x.executor != nil {
		_ = x.executor.Close()
	}
	x.executor = newExec
	return true
}

// reportOutcomeToSupervisor notifies the ToolSupervisor of invocation success or
// failure for circuit breaker state management. Uses Tell (fire-and-forget) so
// the invocation response is not blocked. Parent() returns nil for top-level
// actors; session is always a child of ToolSupervisor.
func (x *session) reportOutcomeToSupervisor(ctx *goaktactor.ReceiveContext, result *mcp.ExecutionResult) {
	parent := ctx.Self().Parent()
	if parent == nil || !parent.IsRunning() {
		return
	}
	success := result != nil && result.Status == mcp.ExecutionStatusSuccess && result.Err == nil
	if success {
		_ = goaktactor.Tell(ctx.Context(), parent, &runtime.ReportSuccess{ToolID: x.toolID})
	} else {
		_ = goaktactor.Tell(ctx.Context(), parent, &runtime.ReportFailure{ToolID: x.toolID})
	}
}
