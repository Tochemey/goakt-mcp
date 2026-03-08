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
	"time"

	goaktactor "github.com/tochemey/goakt/v4/actor"
	goaktlog "github.com/tochemey/goakt/v4/log"
	"github.com/tochemey/goakt/v4/passivation"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	actorextension "github.com/tochemey/goakt-mcp/internal/runtime/actor/extension"
	"github.com/tochemey/goakt-mcp/internal/runtime/config"
	"github.com/tochemey/goakt-mcp/mcp"
)

// toolSupervisor is the ToolSupervisor Actor.
//
// There is one supervisor per registered tool. The supervisor owns the tool's
// circuit breaker state, failure counting, and recovery timing. It decides
// whether work should proceed or fail fast based on circuit state and
// administrative disable status.
//
// Spawn: Registrar spawns ToolSupervisor in spawnSupervisor via
// ctx.Spawn(mcp.ToolSupervisorName(tool.ID), newToolSupervisor(),
// WithDependencies(extension.NewToolDependency(tool))) as a child of Registrar. One supervisor
// per registered tool; spawned on RegisterTool or BootstrapTools.
//
// Relocation: Follows Registrar. In single-node mode, runs on the local node.
// In cluster mode, Registrar is a cluster singleton, so ToolSupervisor
// runs on whichever node hosts the Registrar singleton. If the Registrar relocates,
// its children (including all supervisors) are recreated on the new node.
//
// The tool definition is resolved from dependencies in PreStart. Circuit
// parameters use runtime defaults when not overridden.
//
// All fields are unexported to enforce actor immutability rules.
type toolSupervisor struct {
	tool             mcp.Tool
	circuitState     mcp.CircuitState
	failureCount     int
	openAt           time.Time
	halfOpenRequests int
	circuitConfig    mcp.CircuitConfig
	logger           goaktlog.Logger
	self             *goaktactor.PID
}

// enforce that toolSupervisor satisfies the GoAkt Actor interface at compile time.
var _ goaktactor.Actor = (*toolSupervisor)(nil)

// newToolSupervisor creates a ToolSupervisor Actor instance.
// The tool is injected via WithDependencies when spawning.
func newToolSupervisor() *toolSupervisor {
	return &toolSupervisor{}
}

// PreStart resolves the tool dependency and initializes circuit state.
func (x *toolSupervisor) PreStart(ctx *goaktactor.Context) error {
	x.logger = ctx.Logger()

	dependency := ctx.Dependency(actorextension.ToolDependencyID)
	if dependency != nil {
		if toolDep, ok := dependency.(*actorextension.ToolDependency); ok {
			x.tool = toolDep.Tool()
		}
	}

	if x.tool.ID.IsZero() {
		return mcp.NewRuntimeError(mcp.ErrCodeInternal, "tool dependency not found")
	}

	x.circuitState = mcp.CircuitClosed
	x.circuitConfig = mcp.CircuitConfig{
		FailureThreshold:    mcp.DefaultCircuitFailureThreshold,
		OpenDuration:        mcp.DefaultCircuitOpenDuration,
		HalfOpenMaxRequests: mcp.DefaultCircuitHalfOpenMaxRequests,
	}

	circuitDependency := ctx.Dependency(actorextension.CircuitConfigDependencyID)
	if circuitDependency != nil {
		if config, ok := circuitDependency.(*actorextension.CircuitConfigDependency); ok {
			x.circuitConfig = config.Config()
		}
	}

	x.logger.Infof("actor supervisor:%s started circuit=closed", x.tool.ID)
	return nil
}

// Receive handles messages delivered to ToolSupervisor.
func (x *toolSupervisor) Receive(ctx *goaktactor.ReceiveContext) {
	switch msg := ctx.Message().(type) {
	case *goaktactor.PostStart:
		x.logger.Debugf("actor supervisor:%s post-start", x.tool.ID)
		x.self = ctx.Self()
	case *runtime.CanAcceptWork:
		x.handleCanAcceptWork(ctx, msg)
	case *runtime.GetOrCreateSession:
		x.handleGetOrCreateSession(ctx, msg)
	case *runtime.ReportFailure:
		x.handleReportFailure(ctx, msg)
	case *runtime.ReportSuccess:
		x.handleReportSuccess(ctx, msg)
	default:
		ctx.Unhandled()
	}
}

// PostStop performs cleanup after ToolSupervisor has stopped.
func (x *toolSupervisor) PostStop(ctx *goaktactor.Context) error {
	x.logger.Infof("actor supervisor:%s stopped", x.tool.ID)
	return nil
}

// handleCanAcceptWork checks whether this supervisor can accept new work by
// evaluating tool disable status, circuit state, and half-open probe limits.
// Responds with CanAcceptWorkResult indicating accept/reject and the reason.
func (x *toolSupervisor) handleCanAcceptWork(ctx *goaktactor.ReceiveContext, msg *runtime.CanAcceptWork) {
	if msg.ToolID != x.tool.ID {
		ctx.Response(&runtime.CanAcceptWorkResult{Accept: false, Reason: "tool ID mismatch"})
		return
	}

	if x.tool.State == mcp.ToolStateDisabled {
		ctx.Response(&runtime.CanAcceptWorkResult{Accept: false, Reason: "tool is disabled"})
		return
	}

	x.maybeTransitionFromOpen()
	if !x.circuitState.CanAccept() {
		ctx.Response(&runtime.CanAcceptWorkResult{Accept: false, Reason: "circuit is open"})
		return
	}

	if x.circuitState == mcp.CircuitHalfOpen && x.halfOpenRequests >= x.circuitConfig.HalfOpenMaxRequests {
		ctx.Response(&runtime.CanAcceptWorkResult{Accept: false, Reason: "half-open probe limit reached"})
		return
	}

	if x.circuitState == mcp.CircuitHalfOpen {
		x.halfOpenRequests++
	}

	ctx.Response(&runtime.CanAcceptWorkResult{Accept: true})
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
		return
	}

	if x.circuitState == mcp.CircuitClosed && x.failureCount >= x.circuitConfig.FailureThreshold {
		x.circuitState = mcp.CircuitOpen
		x.openAt = time.Now()
		x.logger.Warnf("actor supervisor:%s circuit opened after %d failures", x.tool.ID, x.failureCount)
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
	}
}
