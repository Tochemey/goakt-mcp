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
	"strconv"
	"strings"
	"time"

	goaktactor "github.com/tochemey/goakt/v4/actor"
	goaktlog "github.com/tochemey/goakt/v4/log"
	"github.com/tochemey/goakt/v4/passivation"

	"github.com/tochemey/goakt-mcp/mcp"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	actorextension "github.com/tochemey/goakt-mcp/internal/runtime/actor/extension"
	"github.com/tochemey/goakt-mcp/internal/runtime/audit"
	"github.com/tochemey/goakt-mcp/internal/runtime/config"
	"github.com/tochemey/goakt-mcp/internal/runtime/telemetry"
)

// toolSupervisor is the ToolSupervisor Actor.
//
// There is one supervisor per registered tool. The supervisor owns the tool's
// circuit breaker state, failure counting, and recovery timing. It decides
// whether work should proceed or fail fast based on circuit state and
// administrative disable status.
//
// Spawn: Registrar spawns ToolSupervisor in spawnSupervisor via
// ctx.Spawn(mcp.ToolSupervisorName(tool.ID), newToolSupervisor()) as a child of Registrar.
// One supervisor per registered tool; spawned on RegisterTool or BootstrapTools.
//
// Relocation: Follows Registrar. In single-node mode, runs on the local node.
// In cluster mode, Registrar is a cluster singleton, so ToolSupervisor runs on
// whichever node hosts the Registrar singleton. If the Registrar relocates, its
// children (including all supervisors) are recreated on the new node.
//
// The tool definition is resolved from the ToolConfigExtension system extension in
// PostStart, using the tool ID derived from the actor name. Circuit parameters use
// runtime defaults unless overridden by a CircuitConfigExtension system extension.
//
// All fields are unexported to enforce actor immutability rules.
type toolSupervisor struct {
	tool             mcp.Tool
	circuitState     mcp.CircuitState
	failureCount     int
	openAt           time.Time
	halfOpenRequests int
	circuitConfig    mcp.CircuitConfig
	journal          *goaktactor.PID
	logger           goaktlog.Logger
	self             *goaktactor.PID
}

// enforce that toolSupervisor satisfies the GoAkt Actor interface at compile time.
var _ goaktactor.Actor = (*toolSupervisor)(nil)

// newToolSupervisor creates a ToolSupervisor Actor instance.
// Tool config is resolved from ToolConfigExtension at PostStart.
func newToolSupervisor() *toolSupervisor {
	return &toolSupervisor{}
}

// PreStart initializes the logger and circuit defaults. Tool config is resolved
// later in PostStart once the actor is fully registered in the actor system.
func (x *toolSupervisor) PreStart(ctx *goaktactor.Context) error {
	x.logger = ctx.Logger()
	x.circuitState = mcp.CircuitClosed
	x.circuitConfig = mcp.CircuitConfig{
		FailureThreshold:    mcp.DefaultCircuitFailureThreshold,
		OpenDuration:        mcp.DefaultCircuitOpenDuration,
		HalfOpenMaxRequests: mcp.DefaultCircuitHalfOpenMaxRequests,
	}
	return nil
}

// Receive handles messages delivered to ToolSupervisor.
func (x *toolSupervisor) Receive(ctx *goaktactor.ReceiveContext) {
	switch msg := ctx.Message().(type) {
	case *goaktactor.PostStart:
		x.self = ctx.Self()

		// Resolve the journal by name. PostStart runs once the actor is registered
		// in the system, so ActorOf reliably finds sibling actors.
		pid, err := ctx.ActorSystem().ActorOf(ctx.Context(), mcp.ActorNameJournal)
		if err != nil {
			x.logger.Warnf("actor supervisor failed to resolve journal: %v", err)
			ctx.Err(err)
			return
		}
		x.journal = pid

		// Resolve tool config from the system-level ToolConfigExtension. The Registrar
		// registers the tool there before spawning the supervisor, so it is always
		// present at PostStart time. The tool ID is derived from the actor name.
		toolID := mcp.ToolIDFromSupervisorName(ctx.Self().Name())
		toolExt, ok := ctx.Extension(actorextension.ToolConfigExtensionID).(*actorextension.ToolConfigExtension)
		if !ok || toolExt == nil {
			x.logger.Warnf("actor supervisor:%s tool config extension not found", toolID)
			ctx.Err(mcp.NewRuntimeError(mcp.ErrCodeInternal, "tool config extension not found"))
			return
		}
		tool, found := toolExt.Get(toolID)
		if !found {
			x.logger.Warnf("actor supervisor:%s tool not registered in extension", toolID)
			ctx.Err(mcp.NewRuntimeError(mcp.ErrCodeInternal, "tool config not found"))
			return
		}
		x.tool = tool

		// Optionally override circuit config from the system-level extension.
		// Used in tests to reduce OpenDuration without per-actor injection.
		if circuitExt, ok := ctx.Extension(actorextension.CircuitConfigExtensionID).(*actorextension.CircuitConfigExtension); ok && circuitExt != nil {
			x.circuitConfig = circuitExt.Config()
		}

		x.logger.Infof("actor supervisor:%s started circuit=closed", x.tool.ID)
	case *runtime.CanAcceptWork:
		x.handleCanAcceptWork(ctx, msg)
	case *runtime.GetOrCreateSession:
		x.handleGetOrCreateSession(ctx, msg)
	case *runtime.ReportFailure:
		x.handleReportFailure(ctx, msg)
	case *runtime.ReportSuccess:
		x.handleReportSuccess(ctx, msg)
	case *runtime.SupervisorCountSessionsForTenant:
		x.handleCountSessionsForTenant(ctx, msg)
	default:
		ctx.Unhandled()
	}
}

// PostStop performs cleanup after ToolSupervisor has stopped.
// It derives the tool ID from the actor name instead of reading x.tool,
// because PostStop runs in a different goroutine than Receive.
func (x *toolSupervisor) PostStop(ctx *goaktactor.Context) error {
	toolID := mcp.ToolIDFromSupervisorName(ctx.ActorName())
	if toolID.IsZero() {
		x.logger.Infof("actor supervisor stopped before tool resolved")
	} else {
		x.logger.Infof("actor supervisor:%s stopped", toolID)
	}
	return nil
}

// handleCanAcceptWork checks whether this supervisor can accept new work by
// evaluating tool disable status, circuit state, half-open probe limits, and
// per-tool backpressure (MaxSessionsPerTool). Responds with CanAcceptWorkResult
// indicating accept/reject, the reason, and the current session count.
// SessionCount is always populated so the caller can use it for further
// decisions (e.g. backpressure) without a separate round-trip.
func (x *toolSupervisor) handleCanAcceptWork(ctx *goaktactor.ReceiveContext, msg *runtime.CanAcceptWork) {
	sessionCount := ctx.Self().ChildrenCount()

	if msg.ToolID != x.tool.ID {
		ctx.Response(&runtime.CanAcceptWorkResult{Accept: false, Reason: "tool ID mismatch", SessionCount: sessionCount})
		return
	}

	if x.tool.State == mcp.ToolStateDisabled {
		ctx.Response(&runtime.CanAcceptWorkResult{Accept: false, Reason: "tool is disabled", SessionCount: sessionCount})
		return
	}

	x.maybeTransitionFromOpen()
	if !x.circuitState.CanAccept() {
		ctx.Response(&runtime.CanAcceptWorkResult{Accept: false, Reason: "circuit is open", SessionCount: sessionCount})
		return
	}

	if x.circuitState == mcp.CircuitHalfOpen && x.halfOpenRequests >= x.circuitConfig.HalfOpenMaxRequests {
		ctx.Response(&runtime.CanAcceptWorkResult{Accept: false, Reason: "half-open probe limit reached", SessionCount: sessionCount})
		return
	}

	if x.tool.MaxSessionsPerTool > 0 && sessionCount >= x.tool.MaxSessionsPerTool {
		ctx.Response(&runtime.CanAcceptWorkResult{Accept: false, Reason: "tool session limit reached (backpressure)", SessionCount: sessionCount})
		return
	}

	if x.circuitState == mcp.CircuitHalfOpen {
		x.halfOpenRequests++
	}

	ctx.Response(&runtime.CanAcceptWorkResult{Accept: true, SessionCount: sessionCount})
}

// handleGetOrCreateSession resolves an existing child session for the given
// tenant+client+tool triple, or spawns a new one with passivation. The session
// is a child of this supervisor, ensuring cleanup on supervisor stop.
func (x *toolSupervisor) handleGetOrCreateSession(ctx *goaktactor.ReceiveContext, msg *runtime.GetOrCreateSession) {
	if msg.ToolID != x.tool.ID {
		ctx.Response(&runtime.GetOrCreateSessionResult{Err: mcp.NewRuntimeError(mcp.ErrCodeInvalidRequest, "tool ID mismatch")})
		return
	}

	name := mcp.SessionName(msg.TenantID, msg.ClientID, msg.ToolID)
	cid := ctx.Child(name)
	if cid != nil && cid.IsRunning() {
		ctx.Response(&runtime.GetOrCreateSessionResult{Session: cid, Found: true})
		return
	}

	var executor mcp.ToolExecutor
	if ext := ctx.Extension(actorextension.ExecutorFactoryExtensionID); ext != nil {
		if ef, ok := ext.(*actorextension.ExecutorFactoryExtension); ok {
			var err error
			executor, err = ef.Factory().Create(ctx.Context(), x.tool, msg.Credentials)
			if err != nil {
				x.logger.Warnf("actor supervisor:%s create executor: %v", x.tool.ID, err)
				ctx.Response(&runtime.GetOrCreateSessionResult{Err: mcp.WrapRuntimeError(mcp.ErrCodeInternal, "failed to create executor", err)})
				return
			}
		}
	}

	sessDep := actorextension.NewSessionDependency(msg.TenantID, msg.ClientID, msg.ToolID, x.tool, executor)
	idleTimeout := sessionIdleTimeout(x.tool)

	pid, err := x.self.SpawnChild(ctx.Context(), name, newSession(),
		goaktactor.WithDependencies(sessDep),
		goaktactor.WithPassivationStrategy(passivation.NewTimeBasedStrategy(idleTimeout)))
	if err != nil {
		if executor != nil {
			_ = executor.Close()
		}
		x.logger.Warnf("actor supervisor:%s spawn session: %v", x.tool.ID, err)
		ctx.Response(&runtime.GetOrCreateSessionResult{Err: mcp.WrapRuntimeError(mcp.ErrCodeInternal, "failed to spawn session", err)})
		return
	}

	ctx.Response(&runtime.GetOrCreateSessionResult{Session: pid, Found: true})
}

// sessionIdleTimeout returns the tool's configured idle timeout or the default
// when unset. This controls how long a session can remain idle before passivation.
func sessionIdleTimeout(tool mcp.Tool) time.Duration {
	if tool.IdleTimeout > 0 {
		return tool.IdleTimeout
	}
	return config.DefaultSessionIdleTimeout
}

// handleReportFailure increments the failure counter and transitions the circuit
// to open when the threshold is reached or when a half-open probe fails.
func (x *toolSupervisor) handleReportFailure(_ *goaktactor.ReceiveContext, msg *runtime.ReportFailure) {
	if msg.ToolID != x.tool.ID {
		return
	}

	x.failureCount++
	x.logger.Debugf("actor supervisor:%s failure count=%d circuit=%s", x.tool.ID, x.failureCount, x.circuitState)

	if x.circuitState == mcp.CircuitHalfOpen {
		x.circuitState = mcp.CircuitOpen
		x.openAt = time.Now()
		x.halfOpenRequests = 0
		x.logger.Warnf("actor supervisor:%s circuit reopened after probe failure", x.tool.ID)
		x.recordCircuitStateChange(string(mcp.CircuitOpen), map[string]string{"reason": "half_open_probe_failed"})
		return
	}

	if x.circuitState == mcp.CircuitClosed && x.failureCount >= x.circuitConfig.FailureThreshold {
		x.circuitState = mcp.CircuitOpen
		x.openAt = time.Now()
		x.logger.Warnf("actor supervisor:%s circuit opened after %d failures", x.tool.ID, x.failureCount)
		x.recordCircuitStateChange(string(mcp.CircuitOpen), map[string]string{"failure_count": strconv.Itoa(x.failureCount)})
	}
}

// handleReportSuccess resets the failure counter on closed-circuit success and
// closes the circuit when a half-open probe succeeds.
func (x *toolSupervisor) handleReportSuccess(_ *goaktactor.ReceiveContext, msg *runtime.ReportSuccess) {
	if msg.ToolID != x.tool.ID {
		return
	}

	if x.circuitState == mcp.CircuitHalfOpen {
		x.circuitState = mcp.CircuitClosed
		x.failureCount = 0
		x.halfOpenRequests = 0
		x.logger.Infof("actor supervisor:%s circuit closed after successful probe", x.tool.ID)
		x.recordCircuitStateChange(string(mcp.CircuitClosed), map[string]string{"reason": "half_open_probe_success"})
		return
	}

	if x.circuitState == mcp.CircuitClosed {
		x.failureCount = 0
	}
}

// maybeTransitionFromOpen checks if the circuit has been open long enough to try half-open.
func (x *toolSupervisor) maybeTransitionFromOpen() {
	if x.circuitState != mcp.CircuitOpen {
		return
	}

	if time.Since(x.openAt) >= x.circuitConfig.OpenDuration {
		x.circuitState = mcp.CircuitHalfOpen
		x.halfOpenRequests = 0
		x.logger.Infof("actor supervisor:%s circuit half-open for recovery probe", x.tool.ID)
		x.recordCircuitStateChange(string(mcp.CircuitHalfOpen), map[string]string{"reason": "open_duration_elapsed"})
	}
}

// handleCountSessionsForTenant counts this supervisor's child sessions that
// belong to the given tenant. Session names follow SessionName(tenantID,
// clientID, toolID) = "session-{tenantID}-{clientID}-{toolID}".
func (x *toolSupervisor) handleCountSessionsForTenant(ctx *goaktactor.ReceiveContext, msg *runtime.SupervisorCountSessionsForTenant) {
	prefix := "session-" + string(msg.TenantID) + "-"
	count := 0
	for _, child := range ctx.Self().Children() {
		if child != nil && child.IsRunning() && strings.HasPrefix(child.Name(), prefix) {
			count++
		}
	}
	ctx.Response(&runtime.SupervisorCountSessionsForTenantResult{Count: count})
}

// recordCircuitStateChange sends a circuit state change audit event to the
// JournalActor and records a CircuitState metric when metrics are registered.
func (x *toolSupervisor) recordCircuitStateChange(state string, metadata map[string]string) {
	telemetry.RecordCircuitState(context.Background(), x.tool.ID, state)
	if x.journal == nil || !x.journal.IsRunning() {
		return
	}
	ev := audit.CircuitStateChangeEvent(string(x.tool.ID), state, metadata)
	_ = goaktactor.Tell(context.Background(), x.journal, &runtime.RecordAuditEvent{Event: ev})
}
