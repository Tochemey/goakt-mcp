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
	"strings"
	"time"

	goaktactor "github.com/tochemey/goakt/v4/actor"
	goaktlog "github.com/tochemey/goakt/v4/log"

	"github.com/tochemey/goakt-mcp/mcp"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	"github.com/tochemey/goakt-mcp/internal/runtime/actor/extension"
	"github.com/tochemey/goakt-mcp/internal/runtime/telemetry"
)

// runProbes is an internal message the HealthActor sends to itself to trigger a probe run.
type runProbes struct{}

// healthChecker is the HealthActor.
//
// HealthActor runs scheduled liveness probes against registered tools and reports
// health-state transitions back into the routing and registry layers. It probes
// each tool's supervisor via CanAcceptWork and maps the result to ToolState.
//
// Spawn: GatewayManager spawns HealthChecker in spawnFoundationalActors via
// ctx.Spawn(ActorNameHealth, newHealthChecker(registryPID, interval)) as a child
// of GatewayManager. Dependencies are passed in the constructor because HealthActor
// does not relocate.
//
// Relocation: No. HealthActor runs on the local node and does not relocate.
type healthChecker struct {
	registrar *goaktactor.PID
	journal   *goaktactor.PID
	interval  time.Duration
	logger    goaktlog.Logger
}

var _ goaktactor.Actor = (*healthChecker)(nil)

// newHealthChecker creates a new HealthChecker with the given registry PID,
// optional journal PID for audit events, and probe interval. When interval is
// zero, config.DefaultHealthProbeInterval is used.
func newHealthChecker() *healthChecker {
	return &healthChecker{}
}

// PreStart initializes the logger.
func (x *healthChecker) PreStart(ctx *goaktactor.Context) error {
	x.logger = ctx.Logger()
	config := ctx.Extension(extension.ConfigExtensionID).(*extension.ConfigExtension).Config()
	x.interval = config.HealthProbe.Interval
	x.logger.Infof("actor=%s starting interval=%s", mcp.ActorNameHealth, x.interval)
	return nil
}

// Receive handles messages delivered to HealthChecker.
func (x *healthChecker) Receive(ctx *goaktactor.ReceiveContext) {
	switch ctx.Message().(type) {
	case *goaktactor.PostStart:
		x.logger.Infof("actor=%s started", mcp.ActorNameHealth)
		x.resolveActors(ctx)
		x.scheduleNext(ctx)
	case *runProbes:
		x.runProbes(ctx)
	default:
		ctx.Unhandled()
	}
}

// PostStop performs cleanup after HealthChecker has stopped.
func (x *healthChecker) PostStop(ctx *goaktactor.Context) error {
	x.logger.Infof("actor=%s stopped", mcp.ActorNameHealth)
	return nil
}

// resolveActors resolves the journal and registrar actors.
// It is called in PreStart to ensure the actors are available when the health checker starts.
func (x *healthChecker) resolveActors(ctx *goaktactor.ReceiveContext) {
	actorSystem := ctx.ActorSystem()
	goCtx := ctx.Context()

	// resolve the journal actor
	journal, err := actorSystem.ActorOf(goCtx, mcp.ActorNameJournal)
	if err != nil {
		ctx.Err(err)
		return
	}

	// resolve the registrar actor
	registrar, err := actorSystem.ActorOf(goCtx, mcp.ActorNameRegistrar)
	if err != nil {
		ctx.Err(err)
		return
	}

	// let us set the various required actors
	x.journal = journal
	x.registrar = registrar
}

// runProbes queries the registry for all tools, probes each via CanAcceptWork,
// and sends UpdateToolHealth to the registry. Then schedules the next run.
func (x *healthChecker) runProbes(ctx *goaktactor.ReceiveContext) {
	if x.registrar == nil || !x.registrar.IsRunning() {
		x.scheduleNext(ctx)
		return
	}

	probeCtx, cancel := context.WithTimeout(ctx.Context(), 10*time.Second)
	defer cancel()

	listResp, err := goaktactor.Ask(probeCtx, x.registrar, &runtime.ListTools{}, 5*time.Second)
	if err != nil {
		x.logger.Warnf("actor=%s list tools failed: %v", mcp.ActorNameHealth, err)
		x.scheduleNext(ctx)
		return
	}

	listResult, ok := listResp.(*runtime.ListToolsResult)
	if !ok || listResult == nil {
		x.scheduleNext(ctx)
		return
	}

	for _, tool := range listResult.Tools {
		if tool.State == mcp.ToolStateDisabled {
			continue
		}
		state := x.probeTool(probeCtx, tool)
		if state != tool.State {
			_ = goaktactor.Tell(ctx.Context(), x.registrar, &runtime.UpdateToolHealth{ToolID: tool.ID, State: state})
			telemetry.RecordToolAvailability(ctx.Context(), tool.ID, state == mcp.ToolStateEnabled)
			x.recordHealthTransition(string(tool.ID), string(tool.State), string(state))
		}
	}

	x.scheduleNext(ctx)
}

// probeTool asks the tool supervisor CanAcceptWork and maps the result to ToolState.
func (x *healthChecker) probeTool(ctx context.Context, tool mcp.Tool) mcp.ToolState {
	supResp, err := goaktactor.Ask(ctx, x.registrar, &runtime.GetSupervisor{ToolID: tool.ID}, 2*time.Second)
	if err != nil {
		return mcp.ToolStateUnavailable
	}

	gsResult, ok := supResp.(*runtime.GetSupervisorResult)
	if !ok || !gsResult.Found || gsResult.Supervisor == nil {
		return mcp.ToolStateUnavailable
	}

	supervisor, ok := gsResult.Supervisor.(*goaktactor.PID)
	if !ok || !supervisor.IsRunning() {
		return mcp.ToolStateUnavailable
	}

	acceptResp, err := goaktactor.Ask(ctx, supervisor, &runtime.CanAcceptWork{ToolID: tool.ID}, 2*time.Second)
	if err != nil {
		return mcp.ToolStateUnavailable
	}

	acceptResult, ok := acceptResp.(*runtime.CanAcceptWorkResult)
	if !ok {
		return mcp.ToolStateUnavailable
	}

	if acceptResult.Accept {
		return mcp.ToolStateEnabled
	}

	reason := strings.ToLower(acceptResult.Reason)
	if strings.Contains(reason, "circuit") || strings.Contains(reason, "open") {
		return mcp.ToolStateUnavailable
	}

	if strings.Contains(reason, "half-open") || strings.Contains(reason, "degraded") {
		return mcp.ToolStateDegraded
	}
	return tool.State
}

// scheduleNext uses the GoAkt scheduler to deliver runProbes to self after the configured interval.
func (x *healthChecker) scheduleNext(ctx *goaktactor.ReceiveContext) {
	if x.interval <= 0 {
		return
	}

	sys := ctx.ActorSystem()
	if err := sys.ScheduleOnce(ctx.Context(), &runProbes{}, ctx.Self(), x.interval); err != nil {
		x.logger.Warnf("actor=%s schedule next probe failed: %v", mcp.ActorNameHealth, err)
		ctx.Err(err)
	}
}

// recordHealthTransition sends a health transition audit event to the JournalActor.
func (x *healthChecker) recordHealthTransition(toolID, fromState, toState string) {
	if x.journal == nil || !x.journal.IsRunning() {
		return
	}
	ev := mcp.HealthTransitionAuditEvent(toolID, fromState, toState)
	_ = goaktactor.Tell(context.Background(), x.journal, &runtime.RecordAuditEvent{Event: ev})
}
