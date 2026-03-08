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

package goaktmcp

import (
	"context"
	"fmt"

	goaktactor "github.com/tochemey/goakt/v4/actor"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	"github.com/tochemey/goakt-mcp/mcp"
)

// Invoke sends a tool invocation through the gateway and returns the execution result.
//
// The invocation is routed to the appropriate tool session via the RouterActor.
// Policy evaluation, credential resolution, and session management are handled
// internally by the runtime.
func (g *Gateway) Invoke(ctx context.Context, inv *mcp.Invocation) (*mcp.ExecutionResult, error) {
	if g.system == nil {
		return nil, mcp.NewRuntimeError(mcp.ErrCodeInternal, "gateway not started")
	}

	routerPID, err := g.resolveRouter(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := goaktactor.Ask(ctx, routerPID, &runtime.RouteInvocation{Invocation: inv}, g.config.Runtime.RequestTimeout)
	if err != nil {
		return nil, mcp.WrapRuntimeError(mcp.ErrCodeInternal, "router ask failed", err)
	}

	result, ok := resp.(*runtime.RouteResult)
	if !ok {
		return nil, mcp.NewRuntimeError(mcp.ErrCodeInternal, fmt.Sprintf("unexpected response type %T", resp))
	}

	if result.Err != nil {
		return result.Result, result.Err
	}
	return result.Result, nil
}

// ListTools returns all tools currently registered in the gateway.
func (g *Gateway) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	if g.system == nil {
		return nil, mcp.NewRuntimeError(mcp.ErrCodeInternal, "gateway not started")
	}

	registrarPID, err := g.resolveRegistrar(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := goaktactor.Ask(ctx, registrarPID, &runtime.ListTools{}, g.config.Runtime.RequestTimeout)
	if err != nil {
		return nil, mcp.WrapRuntimeError(mcp.ErrCodeInternal, "registrar ask failed", err)
	}

	result, ok := resp.(*runtime.ListToolsResult)
	if !ok {
		return nil, mcp.NewRuntimeError(mcp.ErrCodeInternal, fmt.Sprintf("unexpected response type %T", resp))
	}
	return result.Tools, nil
}

// RegisterTool adds or replaces a tool in the gateway registry.
//
// The tool is validated before registration. If a tool with the same ID exists,
// it is replaced.
func (g *Gateway) RegisterTool(ctx context.Context, tool mcp.Tool) error {
	if g.system == nil {
		return mcp.NewRuntimeError(mcp.ErrCodeInternal, "gateway not started")
	}

	if err := mcp.ValidateTool(tool); err != nil {
		return err
	}

	registrarPID, err := g.resolveRegistrar(ctx)
	if err != nil {
		return err
	}

	resp, err := goaktactor.Ask(ctx, registrarPID, &runtime.RegisterTool{Tool: tool}, g.config.Runtime.RequestTimeout)
	if err != nil {
		return mcp.WrapRuntimeError(mcp.ErrCodeInternal, "registrar ask failed", err)
	}

	result, ok := resp.(*runtime.RegisterToolResult)
	if !ok {
		return mcp.NewRuntimeError(mcp.ErrCodeInternal, fmt.Sprintf("unexpected response type %T", resp))
	}
	return result.Err
}

// UpdateTool updates an existing tool's metadata in the registry.
//
// The tool must already exist. Only updatable fields are applied; the tool ID
// cannot be changed.
func (g *Gateway) UpdateTool(ctx context.Context, tool mcp.Tool) error {
	if g.system == nil {
		return mcp.NewRuntimeError(mcp.ErrCodeInternal, "gateway not started")
	}

	registrarPID, err := g.resolveRegistrar(ctx)
	if err != nil {
		return err
	}

	resp, err := goaktactor.Ask(ctx, registrarPID, &runtime.UpdateTool{Tool: tool}, g.config.Runtime.RequestTimeout)
	if err != nil {
		return mcp.WrapRuntimeError(mcp.ErrCodeInternal, "registrar ask failed", err)
	}

	result, ok := resp.(*runtime.UpdateToolResult)
	if !ok {
		return mcp.NewRuntimeError(mcp.ErrCodeInternal, fmt.Sprintf("unexpected response type %T", resp))
	}
	return result.Err
}

// DisableTool administratively disables a tool. The tool remains in the registry
// but its state is set to Disabled. Requests to disabled tools are rejected.
func (g *Gateway) DisableTool(ctx context.Context, toolID mcp.ToolID) error {
	if g.system == nil {
		return mcp.NewRuntimeError(mcp.ErrCodeInternal, "gateway not started")
	}

	registrarPID, err := g.resolveRegistrar(ctx)
	if err != nil {
		return err
	}

	resp, err := goaktactor.Ask(ctx, registrarPID, &runtime.DisableTool{ToolID: toolID}, g.config.Runtime.RequestTimeout)
	if err != nil {
		return mcp.WrapRuntimeError(mcp.ErrCodeInternal, "registrar ask failed", err)
	}

	result, ok := resp.(*runtime.DisableToolResult)
	if !ok {
		return mcp.NewRuntimeError(mcp.ErrCodeInternal, fmt.Sprintf("unexpected response type %T", resp))
	}
	return result.Err
}

// RemoveTool removes a tool from the registry entirely.
func (g *Gateway) RemoveTool(ctx context.Context, toolID mcp.ToolID) error {
	if g.system == nil {
		return mcp.NewRuntimeError(mcp.ErrCodeInternal, "gateway not started")
	}

	registrarPID, err := g.resolveRegistrar(ctx)
	if err != nil {
		return err
	}

	resp, err := goaktactor.Ask(ctx, registrarPID, &runtime.RemoveTool{ToolID: toolID}, g.config.Runtime.RequestTimeout)
	if err != nil {
		return mcp.WrapRuntimeError(mcp.ErrCodeInternal, "registrar ask failed", err)
	}

	result, ok := resp.(*runtime.RemoveToolResult)
	if !ok {
		return mcp.NewRuntimeError(mcp.ErrCodeInternal, fmt.Sprintf("unexpected response type %T", resp))
	}
	return result.Err
}

// resolveRegistrar finds the RegistrarActor PID, trying child lookup first and
// falling back to system-wide lookup for cluster singletons.
func (g *Gateway) resolveRegistrar(ctx context.Context) (*goaktactor.PID, error) {
	managerPID, err := g.system.ActorOf(ctx, mcp.ActorNameGatewayManager)
	if err != nil {
		return nil, mcp.WrapRuntimeError(mcp.ErrCodeInternal, "GatewayManager not found", err)
	}

	pid, err := managerPID.Child(mcp.ActorNameRegistrar)
	if err == nil && pid != nil {
		return pid, nil
	}

	pid, err = g.system.ActorOf(ctx, mcp.ActorNameRegistrar)
	if err != nil {
		return nil, mcp.WrapRuntimeError(mcp.ErrCodeInternal, "registrar not found", err)
	}
	return pid, nil
}

// resolveRouter finds the RouterActor PID.
func (g *Gateway) resolveRouter(ctx context.Context) (*goaktactor.PID, error) {
	managerPID, err := g.system.ActorOf(ctx, mcp.ActorNameGatewayManager)
	if err != nil {
		return nil, mcp.WrapRuntimeError(mcp.ErrCodeInternal, "GatewayManager not found", err)
	}

	pid, err := managerPID.Child(mcp.ActorNameRouter)
	if err == nil && pid != nil {
		return pid, nil
	}

	pid, err = g.system.ActorOf(ctx, mcp.ActorNameRouter)
	if err != nil {
		return nil, mcp.WrapRuntimeError(mcp.ErrCodeInternal, "router not found", err)
	}
	return pid, nil
}
