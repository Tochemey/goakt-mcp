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
	"time"

	goaktactor "github.com/tochemey/goakt/v4/actor"
	goaktlog "github.com/tochemey/goakt/v4/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/tochemey/goakt-mcp/mcp"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	"github.com/tochemey/goakt-mcp/internal/runtime/audit"
	"github.com/tochemey/goakt-mcp/internal/runtime/config"
	"github.com/tochemey/goakt-mcp/internal/runtime/credentials"
	"github.com/tochemey/goakt-mcp/internal/runtime/policy"
	"github.com/tochemey/goakt-mcp/internal/runtime/telemetry"
)

// routingTimeout is the maximum time for the full routing chain (lookup + session + execute).
const routingTimeout = config.DefaultRequestTimeout

// router is the RouterActor.
//
// RouterActor is the runtime entry point for tool invocations. It performs the
// routing path: tool lookup, supervisor availability check, session resolution,
// and execution. Routing decisions are deterministic and tenant-aware.
//
// Spawn: GatewayManager spawns RouterActor in spawnFoundationalActors via
// ctx.Spawn(ActorNameRouter, newRouterActor(registrar, policy, credentialBroker, journal, hasConcurrencyQuotas))
// as a child of GatewayManager. Dependencies are passed in the constructor because
// RouterActor does not relocate.
//
// Relocation: No. RouterActor runs on the local node as a child of GatewayManager
// and does not relocate in cluster mode.
//
// For Phase 6 single-node, both RoutingSticky and RoutingLeastLoaded use the
// same session resolution (GetOrCreateSession). Health-aware exclusions are
// applied via CanAcceptWork. Cluster and least-loaded selection are Phase 10.
//
// All fields are unexported to enforce actor immutability rules.
type router struct {
	registrar            *goaktactor.PID
	policyPID            *goaktactor.PID
	credentialBroker     *goaktactor.PID
	journaler            *goaktactor.PID
	hasConcurrencyQuotas bool
	logger               goaktlog.Logger
}

// enforce that routerActor satisfies the GoAkt Actor interface at compile time.
var _ goaktactor.Actor = (*router)(nil)

// newRouterActor creates a RouterActor with the given PIDs. Registrar must be
// non-nil and running; policy, credentialBroker, and journal may be nil.
func newRouterActor(registrar, policy, credentialBroker, journal *goaktactor.PID, hasConcurrencyQuotas bool) *router {
	return &router{
		registrar:            registrar,
		policyPID:            policy,
		credentialBroker:     credentialBroker,
		journaler:            journal,
		hasConcurrencyQuotas: hasConcurrencyQuotas,
	}
}

// PreStart validates the registrar and initializes the logger.
func (x *router) PreStart(ctx *goaktactor.Context) error {
	x.logger = ctx.Logger()
	if x.registrar == nil || !x.registrar.IsRunning() {
		return mcp.NewRuntimeError(mcp.ErrCodeInternal, "router dependency (registry) not found")
	}
	x.logger.Infof("actor=%s started", mcp.ActorNameRouter)
	return nil
}

// Receive handles messages delivered to RouterActor.
func (x *router) Receive(ctx *goaktactor.ReceiveContext) {
	switch msg := ctx.Message().(type) {
	case *goaktactor.PostStart:
		x.logger.Debugf("actor=%s post-start", mcp.ActorNameRouter)
	case *runtime.RouteInvocation:
		x.handleRouteInvocation(ctx, msg)
	default:
		ctx.Unhandled()
	}
}

// PostStop performs cleanup after RouterActor has stopped.
func (x *router) PostStop(ctx *goaktactor.Context) error {
	x.logger.Infof("actor=%s stopped", mcp.ActorNameRouter)
	return nil
}

// handleRouteInvocation orchestrates the full routing chain: validates the
// invocation, evaluates policy, looks up the tool and supervisor, checks
// circuit availability, resolves credentials, obtains a session, and
// forwards the call to the session for execution. Audit events are recorded
// at each failure point and on completion.
func (x *router) handleRouteInvocation(ctx *goaktactor.ReceiveContext, msg *runtime.RouteInvocation) {
	if err := x.validateInvocation(msg); err != nil {
		if msg.Invocation != nil {
			x.recordAuditEvent(invocationEvent(msg.Invocation, audit.EventTypeInvocationFailed, "invalid", string(mcp.ErrCodeInvalidRequest), err.Error()))
		}
		ctx.Response(&runtime.RouteResult{Err: err})
		return
	}
	inv := msg.Invocation
	tenantID, clientID := x.resolveTenantClient(inv)
	corr := &telemetry.CorrelationFields{
		TenantID:  inv.Correlation.TenantID,
		ClientID:  inv.Correlation.ClientID,
		RequestID: inv.Correlation.RequestID,
		TraceID:   inv.Correlation.TraceID,
		ToolID:    inv.ToolID,
	}
	log := x.logger
	if kvs := corr.LogKeyValues(); len(kvs) > 0 {
		log = x.logger.With(kvs...)
	}
	log.Debugf("actor=%s routing tool=%s", mcp.ActorNameRouter, inv.ToolID)
	start := time.Now()

	goCtx, cancel := context.WithTimeout(ctx.Context(), routingTimeout)
	defer cancel()

	// routeErr tracks the terminal error for span recording. Using a single
	// variable that all error paths assign to ensures the deferred span closure
	// always captures the actual failure, regardless of block scoping.
	var routeErr error

	if telemetry.TracingEnabled() {
		var span trace.Span
		goCtx, span = telemetry.Tracer().Start(goCtx, "goaktmcp.route.invoke",
			trace.WithAttributes(
				attribute.String("mcp.tool_id", string(inv.ToolID)),
				attribute.String("mcp.tenant_id", string(tenantID)),
				attribute.String("mcp.client_id", string(clientID)),
			),
			trace.WithSpanKind(trace.SpanKindInternal),
		)
		defer func() {
			if routeErr != nil {
				span.SetStatus(codes.Error, routeErr.Error())
				span.RecordError(routeErr)
			}
			span.End()
		}()
	}

	tool, err := x.lookupTool(goCtx, inv.ToolID)
	if err != nil {
		routeErr = err
		telemetry.RecordInvocationFailure(goCtx, inv.ToolID, tenantID, string(mcp.ErrCodeToolNotFound))
		x.recordAuditEvent(invocationEvent(inv, audit.EventTypeInvocationFailed, "error", string(mcp.ErrCodeToolNotFound), err.Error()))
		ctx.Response(&runtime.RouteResult{Err: err})
		return
	}

	if err := x.evaluatePolicy(goCtx, inv, tool, tenantID, clientID); err != nil {
		routeErr = err
		code := mcp.ErrCodePolicyDenied
		if re := (*mcp.RuntimeError)(nil); errors.As(err, &re) {
			code = re.Code
		}
		telemetry.RecordInvocationFailure(goCtx, inv.ToolID, tenantID, string(code))
		x.recordAuditEvent(invocationEvent(inv, audit.EventTypePolicyDecision, outcomeFromError(err), string(code), err.Error()))
		ctx.Response(&runtime.RouteResult{Err: err})
		return
	}

	supervisorPID, err := x.lookupSupervisor(goCtx, inv.ToolID)
	if err != nil {
		routeErr = err
		telemetry.RecordInvocationFailure(goCtx, inv.ToolID, tenantID, string(mcp.ErrCodeInternal))
		x.recordAuditEvent(invocationEvent(inv, audit.EventTypeInvocationFailed, "error", string(mcp.ErrCodeInternal), err.Error()))
		ctx.Response(&runtime.RouteResult{Err: err})
		return
	}

	if err := x.checkAcceptWork(goCtx, supervisorPID, inv.ToolID, tool); err != nil {
		routeErr = err
		code := mcp.ErrCodeToolUnavailable
		if re := (*mcp.RuntimeError)(nil); errors.As(err, &re) {
			code = re.Code
		}
		telemetry.RecordInvocationFailure(goCtx, inv.ToolID, tenantID, string(code))
		x.recordAuditEvent(invocationEvent(inv, audit.EventTypeInvocationFailed, "unavailable", string(code), err.Error()))
		ctx.Response(&runtime.RouteResult{Err: err})
		return
	}

	invToUse, err := x.resolveCredentials(goCtx, inv, tool, tenantID)
	if err != nil {
		routeErr = err
		telemetry.RecordInvocationFailure(goCtx, inv.ToolID, tenantID, string(mcp.ErrCodeCredentialUnavailable))
		x.recordAuditEvent(invocationEvent(inv, audit.EventTypeInvocationFailed, "credential_unavailable", string(mcp.ErrCodeCredentialUnavailable), err.Error()))
		ctx.Response(&runtime.RouteResult{Err: err})
		return
	}

	sessionPID, err := x.resolveSession(goCtx, supervisorPID, tenantID, clientID, inv.ToolID, invToUse.Credentials)
	if err != nil {
		routeErr = err
		telemetry.RecordInvocationFailure(goCtx, inv.ToolID, tenantID, string(mcp.ErrCodeInternal))
		x.recordAuditEvent(invocationEvent(inv, audit.EventTypeInvocationFailed, "session_error", string(mcp.ErrCodeInternal), err.Error()))
		ctx.Response(&runtime.RouteResult{Err: err})
		return
	}

	result, err := x.executeInvocation(goCtx, sessionPID, invToUse, tool)
	if err != nil {
		routeErr = err
		code := mcp.ErrCodeInternal
		if re := (*mcp.RuntimeError)(nil); errors.As(err, &re) {
			code = re.Code
		}
		telemetry.RecordInvocationFailure(goCtx, inv.ToolID, tenantID, string(code))
		x.recordAuditEvent(invocationEvent(inv, audit.EventTypeInvocationFailed, "execution_error", string(code), err.Error()))
		ctx.Response(&runtime.RouteResult{Err: err})
		return
	}
	telemetry.RecordInvocationLatency(goCtx, inv.ToolID, tenantID, float64(time.Since(start).Milliseconds()))
	x.recordAuditEvent(invocationEvent(inv, audit.EventTypeInvocationComplete, string(result.Status), "", ""))
	ctx.Response(&runtime.RouteResult{Result: result})
}

// validateInvocation rejects nil invocations and invocations without a tool ID.
func (x *router) validateInvocation(msg *runtime.RouteInvocation) error {
	if msg.Invocation == nil {
		return mcp.NewRuntimeError(mcp.ErrCodeInvalidRequest, "invocation is required")
	}
	if msg.Invocation.ToolID.IsZero() {
		return mcp.NewRuntimeError(mcp.ErrCodeInvalidRequest, "tool ID is required")
	}
	return nil
}

// resolveCredentials asks the CredentialBrokerActor for credentials when the
// tool has CredentialPolicyRequired. Returns the invocation unchanged when
// credentials are optional, or a copy with credentials attached on success.
func (x *router) resolveCredentials(goCtx context.Context, inv *mcp.Invocation, tool mcp.Tool, tenantID mcp.TenantID) (*mcp.Invocation, error) {
	if tool.CredentialPolicy != mcp.CredentialPolicyRequired {
		return inv, nil
	}
	if x.credentialBroker == nil || !x.credentialBroker.IsRunning() {
		return nil, mcp.NewRuntimeError(mcp.ErrCodeCredentialUnavailable, "credential broker not available")
	}
	resp, err := goaktactor.Ask(goCtx, x.credentialBroker, &credentials.ResolveRequest{
		TenantID: tenantID,
		ToolID:   inv.ToolID,
	}, routingTimeout)
	if err != nil {
		return nil, mcp.WrapRuntimeError(mcp.ErrCodeInternal, "credential resolution failed", err)
	}
	result, ok := resp.(*credentials.ResolveResult)
	if !ok || !result.Resolved() {
		if result != nil && result.Err != nil {
			return nil, result.Err
		}
		return nil, mcp.NewRuntimeError(mcp.ErrCodeCredentialUnavailable, "credentials not resolved")
	}
	invCopy := *inv
	invCopy.Credentials = result.Credentials
	return &invCopy, nil
}

// evaluatePolicy asks the PolicyActor for an authorization decision. Returns nil
// when no PolicyActor is available (policy is optional) or when the request is
// allowed. Returns a RuntimeError with the appropriate denial code on reject.
// ActiveSessionCount is resolved from the registrar for ConcurrentSessions quota.
func (x *router) evaluatePolicy(goCtx context.Context, inv *mcp.Invocation, tool mcp.Tool, tenantID mcp.TenantID, clientID mcp.ClientID) error {
	if x.policyPID == nil || !x.policyPID.IsRunning() {
		return nil
	}
	activeSessions := 0
	if x.hasConcurrencyQuotas && x.registrar != nil && x.registrar.IsRunning() {
		if resp, err := goaktactor.Ask(goCtx, x.registrar, &runtime.CountSessionsForTenant{TenantID: tenantID}, routingTimeout); err == nil {
			if result, ok := resp.(*runtime.CountSessionsForTenantResult); ok {
				activeSessions = result.Count
			}
		}
	}
	in := &policy.Input{
		Invocation:         inv,
		Tool:               tool,
		TenantID:           tenantID,
		ClientID:           clientID,
		ActiveSessionCount: activeSessions,
	}
	resp, err := goaktactor.Ask(goCtx, x.policyPID, &policy.EvaluateRequest{Input: in}, routingTimeout)
	if err != nil {
		return mcp.WrapRuntimeError(mcp.ErrCodeInternal, "policy evaluation failed", err)
	}
	result, ok := resp.(*policy.EvaluateResult)
	if !ok || !result.Result.Allowed() {
		if result != nil && result.Result.Err != nil {
			return result.Result.Err
		}
		return mcp.NewRuntimeError(mcp.ErrCodePolicyDenied, "policy evaluation failed")
	}
	return nil
}

// resolveTenantClient extracts tenant and client identifiers from the
// invocation's correlation metadata, defaulting each to "default" when empty.
func (x *router) resolveTenantClient(inv *mcp.Invocation) (mcp.TenantID, mcp.ClientID) {
	tenantID := inv.Correlation.TenantID
	clientID := inv.Correlation.ClientID
	if tenantID.IsZero() {
		tenantID = "default"
	}
	if clientID.IsZero() {
		clientID = "default"
	}
	return tenantID, clientID
}

// lookupTool queries the RegistryActor for the tool definition. Returns
// ErrToolNotFound when the tool is not registered.
func (x *router) lookupTool(goCtx context.Context, toolID mcp.ToolID) (mcp.Tool, error) {
	qResp, err := goaktactor.Ask(goCtx, x.registrar, &runtime.QueryTool{ToolID: toolID}, routingTimeout)
	if err != nil {
		return mcp.Tool{}, mcp.WrapRuntimeError(mcp.ErrCodeInternal, "registry query failed", err)
	}
	qResult, ok := qResp.(*runtime.QueryToolResult)
	if !ok || !qResult.Found || qResult.Tool == nil {
		return mcp.Tool{}, mcp.ErrToolNotFound
	}
	return *qResult.Tool, nil
}

// lookupSupervisor queries the RegistryActor for the tool's supervisor PID.
// Returns an error when no supervisor exists or the supervisor is not running.
func (x *router) lookupSupervisor(goCtx context.Context, toolID mcp.ToolID) (*goaktactor.PID, error) {
	gsResp, err := goaktactor.Ask(goCtx, x.registrar, &runtime.GetSupervisor{ToolID: toolID}, routingTimeout)
	if err != nil {
		return nil, mcp.WrapRuntimeError(mcp.ErrCodeInternal, "supervisor lookup failed", err)
	}
	gsResult, ok := gsResp.(*runtime.GetSupervisorResult)
	if !ok || !gsResult.Found || gsResult.Supervisor == nil {
		return nil, mcp.ErrToolNotFound
	}
	supervisorPID, ok := gsResult.Supervisor.(*goaktactor.PID)
	if !ok || !supervisorPID.IsRunning() {
		return nil, mcp.NewRuntimeError(mcp.ErrCodeToolUnavailable, "supervisor not available")
	}
	return supervisorPID, nil
}

// checkAcceptWork asks the ToolSupervisorActor whether it can accept new work.
// The supervisor evaluates circuit state, tool availability, and backpressure
// (MaxSessionsPerTool) in a single Ask. Returns a typed RuntimeError
// (ToolDisabled, ToolUnavailable, or ConcurrencyLimitReached) when rejected.
func (x *router) checkAcceptWork(goCtx context.Context, supervisorPID *goaktactor.PID, toolID mcp.ToolID, tool mcp.Tool) error {
	acceptResp, err := goaktactor.Ask(goCtx, supervisorPID, &runtime.CanAcceptWork{ToolID: toolID}, routingTimeout)
	if err != nil {
		return mcp.WrapRuntimeError(mcp.ErrCodeInternal, "availability check failed", err)
	}
	acceptResult, ok := acceptResp.(*runtime.CanAcceptWorkResult)
	if !ok || !acceptResult.Accept {
		reason := "circuit open or tool unavailable"
		if acceptResult != nil && acceptResult.Reason != "" {
			reason = acceptResult.Reason
		}
		switch {
		case tool.State == mcp.ToolStateDisabled:
			return mcp.NewRuntimeError(mcp.ErrCodeToolDisabled, reason)
		case tool.MaxSessionsPerTool > 0 && acceptResult != nil && acceptResult.SessionCount >= tool.MaxSessionsPerTool:
			return mcp.NewRuntimeError(mcp.ErrCodeConcurrencyLimitReached, reason)
		default:
			return mcp.NewRuntimeError(mcp.ErrCodeToolUnavailable, reason)
		}
	}
	return nil
}

// resolveSession asks the supervisor to resolve or create a session for the
// given tenant+client+tool triple. Returns the session PID when available.
func (x *router) resolveSession(goCtx context.Context, supervisorPID *goaktactor.PID, tenantID mcp.TenantID, clientID mcp.ClientID, toolID mcp.ToolID, creds map[string]string) (*goaktactor.PID, error) {
	sessResp, err := goaktactor.Ask(goCtx, supervisorPID, &runtime.GetOrCreateSession{
		TenantID:    tenantID,
		ClientID:    clientID,
		ToolID:      toolID,
		Credentials: creds,
	}, routingTimeout)
	if err != nil {
		return nil, mcp.WrapRuntimeError(mcp.ErrCodeInternal, "session resolution failed", err)
	}
	sessResult, ok := sessResp.(*runtime.GetOrCreateSessionResult)
	if !ok || !sessResult.Found || sessResult.Session == nil {
		err := mcp.NewRuntimeError(mcp.ErrCodeInternal, "session not available")
		if sessResult != nil && sessResult.Err != nil {
			err = mcp.WrapRuntimeError(mcp.ErrCodeInternal, "session not available", sessResult.Err)
		}
		return nil, err
	}
	sessionPID, ok := sessResult.Session.(*goaktactor.PID)
	if !ok || !sessionPID.IsRunning() {
		return nil, mcp.NewRuntimeError(mcp.ErrCodeSessionUnavailable, "session not running")
	}
	return sessionPID, nil
}

// executeInvocation forwards the invocation to the SessionActor via Ask and
// waits for the execution result. Uses the tool's RequestTimeout or the default.
func (x *router) executeInvocation(goCtx context.Context, sessionPID *goaktactor.PID, inv *mcp.Invocation, tool mcp.Tool) (*mcp.ExecutionResult, error) {
	execTimeout := tool.RequestTimeout
	if execTimeout == 0 {
		execTimeout = config.DefaultRequestTimeout
	}
	sessInvResp, err := goaktactor.Ask(goCtx, sessionPID, &runtime.SessionInvoke{Invocation: inv}, execTimeout)
	if err != nil {
		return nil, mcp.WrapRuntimeError(mcp.ErrCodeInternal, "invocation failed", err)
	}
	sessInvResult, ok := sessInvResp.(*runtime.SessionInvokeResult)
	if !ok {
		return nil, mcp.NewRuntimeError(mcp.ErrCodeInternal, "invalid session response")
	}
	if sessInvResult.Err != nil {
		var rErr *mcp.RuntimeError
		if errors.As(sessInvResult.Err, &rErr) {
			return nil, sessInvResult.Err
		}
		return nil, mcp.WrapRuntimeError(mcp.ErrCodeInternal, "session error", sessInvResult.Err)
	}
	return sessInvResult.Result, nil
}

// recordAuditEvent sends an audit event to the JournalActor via Tell (fire-and-forget).
// No-op when the journal is not available.
func (x *router) recordAuditEvent(event *audit.Event) {
	if x.journaler == nil || !x.journaler.IsRunning() {
		return
	}
	_ = goaktactor.Tell(context.Background(), x.journaler, &runtime.RecordAuditEvent{Event: event})
}

// invocationEvent constructs an audit Event from invocation context. Returns nil
// when the invocation itself is nil (defensive guard for early-validation failures).
func invocationEvent(inv *mcp.Invocation, evType audit.EventType, outcome, errorCode, message string) *audit.Event {
	if inv == nil {
		return nil
	}
	return &audit.Event{
		Type:      evType,
		Timestamp: time.Now(),
		TenantID:  string(inv.Correlation.TenantID),
		ClientID:  string(inv.Correlation.ClientID),
		ToolID:    string(inv.ToolID),
		RequestID: string(inv.Correlation.RequestID),
		TraceID:   string(inv.Correlation.TraceID),
		Outcome:   outcome,
		ErrorCode: errorCode,
		Message:   message,
	}
}

// outcomeFromError maps a RuntimeError to an audit outcome string. Policy denials
// produce "deny", rate/quota/concurrency limits produce "throttle", and all other
// errors produce "error".
func outcomeFromError(err error) string {
	if err == nil {
		return ""
	}
	var re *mcp.RuntimeError
	if errors.As(err, &re) {
		switch re.Code {
		case mcp.ErrCodePolicyDenied:
			return "deny"
		case mcp.ErrCodeRateLimited, mcp.ErrCodeQuotaExceeded, mcp.ErrCodeConcurrencyLimitReached:
			return "throttle"
		}
	}
	return "error"
}
