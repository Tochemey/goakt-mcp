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
	goaktactor "github.com/tochemey/goakt/v4/actor"
	goaktlog "github.com/tochemey/goakt/v4/log"
	"github.com/tochemey/goakt/v4/supervisor"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	actorextension "github.com/tochemey/goakt-mcp/internal/runtime/actor/extension"
	"github.com/tochemey/goakt-mcp/mcp"
)

// registrar is the Registry actor.
//
// Registrar is the canonical source of truth for tool metadata inside the runtime.
// It owns which tools exist, their current state, and the routing information needed
// to reach the responsible tool supervisor. In clustered deployments, Registrar
// operates as a cluster singleton.
//
// For each registered tool, Registrar spawns and supervises a ToolSupervisor.
// The supervisor PID is stored for lookup via GetSupervisor.
//
// Spawn: GatewayManager spawns Registrar in spawnFoundationalActors.
//   - Single-node: ctx.Spawn(ActorNameRegistrar, newRegistrar()) as a child of GatewayManager.
//   - Cluster mode: system.SpawnSingleton(ctx, ActorNameRegistrar, newRegistrar(), opts...)
//     when cluster.IsClusterConfigured(cfg) is true. NewRegistrar() is registered
//     as a cluster kind in cluster.BuildOptions.
type registrar struct {
	tools       map[mcp.ToolID]mcp.Tool
	supervisors map[mcp.ToolID]*goaktactor.PID
	logger      goaktlog.Logger
}

// enforce that registrar satisfies the GoAkt Actor interface at compile time.
var _ goaktactor.Actor = (*registrar)(nil)

// newRegistrar creates a new Registrar instance.
func newRegistrar() *registrar {
	return &registrar{}
}

// NewRegistrar returns a Registrar instance for cluster kind registration.
// Used by the cluster bootstrap when Cluster.Enabled is true.
func NewRegistrar() goaktactor.Actor { return newRegistrar() }

// PreStart initializes Registrar before message processing begins.
func (x *registrar) PreStart(ctx *goaktactor.Context) error {
	x.logger = ctx.Logger()
	x.tools = make(map[mcp.ToolID]mcp.Tool)
	x.supervisors = make(map[mcp.ToolID]*goaktactor.PID)
	ctx.Logger().Infof("actor=%s starting", mcp.ActorNameRegistrar)
	return nil
}

// Receive handles messages delivered to RegistryActor.
func (x *registrar) Receive(ctx *goaktactor.ReceiveContext) {
	switch msg := ctx.Message().(type) {
	case *goaktactor.PostStart:
		x.logger.Infof("actor=%s started", mcp.ActorNameRegistrar)
	case *runtime.RegisterTool:
		x.handleRegisterTool(ctx, msg)
	case *runtime.UpdateTool:
		x.handleUpdateTool(ctx, msg)
	case *runtime.DisableTool:
		x.handleDisableTool(ctx, msg)
	case *runtime.RemoveTool:
		x.handleRemoveTool(ctx, msg)
	case *runtime.QueryTool:
		x.handleQueryTool(ctx, msg)
	case *runtime.UpdateToolHealth:
		x.handleUpdateToolHealth(ctx, msg)
	case *runtime.BootstrapTools:
		x.handleBootstrapTools(ctx, msg)
	case *runtime.GetSupervisor:
		x.handleGetSupervisor(ctx, msg)
	case *runtime.ListTools:
		x.handleListTools(ctx)
	default:
		ctx.Unhandled()
	}
}

// PostStop performs cleanup after Registrar has stopped.
func (x *registrar) PostStop(ctx *goaktactor.Context) error {
	x.logger.Infof("actor=%s stopped", mcp.ActorNameRegistrar)
	return nil
}

// handleRegisterTool validates and registers a tool. If a tool with the same ID
// was previously disabled, the disabled state is preserved. The previous supervisor
// (if any) is stopped and a new one is spawned for the registered tool.
func (x *registrar) handleRegisterTool(ctx *goaktactor.ReceiveContext, msg *runtime.RegisterTool) {
	if err := mcp.ValidateTool(msg.Tool); err != nil {
		x.respondIfAsk(ctx, &runtime.RegisterToolResult{Err: err})
		return
	}

	tool := msg.Tool
	if existing, ok := x.tools[tool.ID]; ok && existing.State == mcp.ToolStateDisabled {
		tool.State = mcp.ToolStateDisabled
	}

	x.stopSupervisorIfExists(ctx, tool.ID)
	x.tools[tool.ID] = tool
	pid := x.spawnSupervisor(ctx, tool)
	if pid != nil {
		x.supervisors[tool.ID] = pid
	}

	x.logger.Infof("actor=%s registered tool=%s", mcp.ActorNameRegistrar, tool.ID)
	x.respondIfAsk(ctx, &runtime.RegisterToolResult{})
}

// handleUpdateTool applies mutable field updates to an existing tool. Identity
// and transport configuration are preserved from the original registration.
// Returns ErrToolNotFound when the tool does not exist.
func (x *registrar) handleUpdateTool(ctx *goaktactor.ReceiveContext, msg *runtime.UpdateTool) {
	existing, ok := x.tools[msg.Tool.ID]
	if !ok {
		x.respondIfAsk(ctx, &runtime.UpdateToolResult{Err: mcp.ErrToolNotFound})
		return
	}

	updated := msg.Tool
	updated.ID = existing.ID
	updated.Transport = existing.Transport
	updated.Stdio = existing.Stdio
	updated.HTTP = existing.HTTP

	if err := mcp.ValidateTool(updated); err != nil {
		x.respondIfAsk(ctx, &runtime.UpdateToolResult{Err: err})
		return
	}

	updated.State = existing.State
	x.tools[updated.ID] = updated
	x.logger.Infof("actor=%s updated tool=%s", mcp.ActorNameRegistrar, updated.ID)
	x.respondIfAsk(ctx, &runtime.UpdateToolResult{})
}

// handleDisableTool sets the tool state to ToolStateDisabled. The tool remains
// in the registry but all new requests are rejected. Returns ErrToolNotFound
// when the tool does not exist.
func (x *registrar) handleDisableTool(ctx *goaktactor.ReceiveContext, msg *runtime.DisableTool) {
	existing, ok := x.tools[msg.ToolID]
	if !ok {
		x.respondIfAsk(ctx, &runtime.DisableToolResult{Err: mcp.ErrToolNotFound})
		return
	}

	existing.State = mcp.ToolStateDisabled
	x.tools[msg.ToolID] = existing
	x.logger.Infof("actor=%s disabled tool=%s", mcp.ActorNameRegistrar, msg.ToolID)
	x.respondIfAsk(ctx, &runtime.DisableToolResult{})
}

// handleRemoveTool removes a tool from the registry and stops its supervisor.
// Returns ErrToolNotFound when the tool does not exist.
func (x *registrar) handleRemoveTool(ctx *goaktactor.ReceiveContext, msg *runtime.RemoveTool) {
	if _, ok := x.tools[msg.ToolID]; !ok {
		x.respondIfAsk(ctx, &runtime.RemoveToolResult{Err: mcp.ErrToolNotFound})
		return
	}

	x.stopSupervisorIfExists(ctx, msg.ToolID)
	delete(x.tools, msg.ToolID)
	delete(x.supervisors, msg.ToolID)
	x.logger.Infof("actor=%s removed tool=%s", mcp.ActorNameRegistrar, msg.ToolID)
	x.respondIfAsk(ctx, &runtime.RemoveToolResult{})
}

// handleQueryTool looks up a tool by ID and returns the authoritative definition.
// Returns Found=false and ErrToolNotFound when the tool is not registered.
func (x *registrar) handleQueryTool(ctx *goaktactor.ReceiveContext, msg *runtime.QueryTool) {
	tool, ok := x.tools[msg.ToolID]
	if !ok {
		x.respondIfAsk(ctx, &runtime.QueryToolResult{Found: false, Err: mcp.ErrToolNotFound})
		return
	}
	x.respondIfAsk(ctx, &runtime.QueryToolResult{Tool: &tool, Found: true})
}

// handleUpdateToolHealth transitions a tool's operational state (e.g. enabled,
// degraded, unavailable). Used by health probes and discovery to reflect actual
// tool availability.
func (x *registrar) handleUpdateToolHealth(ctx *goaktactor.ReceiveContext, msg *runtime.UpdateToolHealth) {
	existing, ok := x.tools[msg.ToolID]
	if !ok {
		x.respondIfAsk(ctx, &runtime.UpdateToolHealthResult{Err: mcp.ErrToolNotFound})
		return
	}
	existing.State = msg.State
	x.tools[msg.ToolID] = existing
	x.logger.Debugf("actor=%s updated health tool=%s state=%s", mcp.ActorNameRegistrar, msg.ToolID, msg.State)
	x.respondIfAsk(ctx, &runtime.UpdateToolHealthResult{})
}

// handleBootstrapTools bulk-registers tools from static configuration during
// startup. Invalid tools are logged and skipped. Each valid tool gets a
// supervisor spawned as a child of the registry.
func (x *registrar) handleBootstrapTools(ctx *goaktactor.ReceiveContext, msg *runtime.BootstrapTools) {
	for _, tool := range msg.Tools {
		if err := mcp.ValidateTool(tool); err != nil {
			x.logger.Warnf("actor=%s bootstrap skip tool=%s: %v", mcp.ActorNameRegistrar, tool.ID, err)
			continue
		}
		x.stopSupervisorIfExists(ctx, tool.ID)
		x.tools[tool.ID] = tool
		if pid := x.spawnSupervisor(ctx, tool); pid != nil {
			x.supervisors[tool.ID] = pid
		}
		x.logger.Infof("actor=%s bootstrap registered tool=%s", mcp.ActorNameRegistrar, tool.ID)
	}
}

// handleGetSupervisor resolves the supervisor PID for the given tool. Returns
// Found=false and ErrToolNotFound when no supervisor exists.
func (x *registrar) handleGetSupervisor(ctx *goaktactor.ReceiveContext, msg *runtime.GetSupervisor) {
	pid, ok := x.supervisors[msg.ToolID]
	if !ok {
		x.respondIfAsk(ctx, &runtime.GetSupervisorResult{Found: false, Err: mcp.ErrToolNotFound})
		return
	}
	x.respondIfAsk(ctx, &runtime.GetSupervisorResult{Supervisor: pid, Found: true})
}

// handleListTools returns all registered tools. Used by HealthActor for probing.
func (x *registrar) handleListTools(ctx *goaktactor.ReceiveContext) {
	tools := make([]mcp.Tool, 0, len(x.tools))
	for _, t := range x.tools {
		tools = append(tools, t)
	}
	x.respondIfAsk(ctx, &runtime.ListToolsResult{Tools: tools})
}

// spawnSupervisor creates a ToolSupervisorActor as a child of the registry for
// the given tool. The tool is injected via WithDependencies. Returns nil on error.
// Uses a supervisor strategy that resumes (does not suspend) when child sessions fail,
// so the tool supervisor remains available for subsequent GetOrCreateSession requests.
func (x *registrar) spawnSupervisor(ctx *goaktactor.ReceiveContext, tool mcp.Tool) *goaktactor.PID {
	name := mcp.ToolSupervisorName(tool.ID)
	dep := actorextension.NewToolDependency(tool)
	toolSupervisor := supervisor.NewSupervisor(supervisor.WithAnyErrorDirective(supervisor.ResumeDirective))

	pid := ctx.Spawn(name, newToolSupervisor(),
		goaktactor.WithDependencies(dep),
		goaktactor.WithSupervisor(toolSupervisor))
	return pid
}

// stopSupervisorIfExists stops and removes the supervisor for the given tool
// if one is currently tracked. No-op when no supervisor exists.
func (x *registrar) stopSupervisorIfExists(ctx *goaktactor.ReceiveContext, toolID mcp.ToolID) {
	if pid, ok := x.supervisors[toolID]; ok {
		ctx.Stop(pid)
		delete(x.supervisors, toolID)
	}
}

// respondIfAsk sends the response when the message was delivered via Ask.
// When delivered via Tell, the response channel is nil and this is a no-op.
func (x *registrar) respondIfAsk(ctx *goaktactor.ReceiveContext, resp any) {
	ctx.Response(resp)
}
