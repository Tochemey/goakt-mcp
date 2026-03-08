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

package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	goaktactor "github.com/tochemey/goakt/v4/actor"

	"github.com/tochemey/goakt-mcp/internal/ingress/dto"
	"github.com/tochemey/goakt-mcp/internal/runtime"
)

// routingTimeout is the default deadline for actor-system lookups when routing
// an inbound request to the appropriate actor.
const routingTimeout = 30 * time.Second

// handleInvoke handles POST /v1/tools/{tool}/invoke.
func (s *Server) handleInvoke(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, runtime.ErrCodeInvalidRequest, "method not allowed")
		return
	}

	toolID := req.PathValue("tool")
	if toolID == "" {
		writeError(w, http.StatusBadRequest, runtime.ErrCodeInvalidRequest, "tool ID is required")
		return
	}

	var body dto.InvokeRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, runtime.ErrCodeInvalidRequest, "invalid JSON body")
		return
	}

	if err := body.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, runtime.ErrCodeInvalidRequest, err.Error())
		return
	}

	inv := &runtime.Invocation{
		Correlation: runtime.CorrelationMeta{
			TenantID:  runtime.TenantID(body.TenantID),
			ClientID:  runtime.ClientID(body.ClientID),
			RequestID: runtime.RequestID(body.RequestID),
			TraceID:   runtime.TraceID(body.TraceID),
		},
		ToolID:     runtime.ToolID(toolID),
		Method:     body.Method,
		Params:     body.Params,
		Metadata:   body.Metadata,
		ReceivedAt: time.Now(),
	}

	routerPID, err := s.resolveRouter(req.Context())
	if err != nil {
		s.logger.Warnf("ingress resolve router: %v", err)
		writeError(w, http.StatusServiceUnavailable, runtime.ErrCodeInternal, "runtime unavailable")
		return
	}

	resp, err := goaktactor.Ask(req.Context(), routerPID, &runtime.RouteInvocation{Invocation: inv}, routingTimeout)
	if err != nil {
		s.logger.Warnf("ingress route invocation: %v", err)
		writeError(w, http.StatusServiceUnavailable, runtime.ErrCodeInternal, "routing failed")
		return
	}

	routeResult, ok := resp.(*runtime.RouteResult)
	if !ok || routeResult == nil {
		writeError(w, http.StatusInternalServerError, runtime.ErrCodeInternal, "invalid route response")
		return
	}

	if routeResult.Err != nil {
		writeRuntimeError(w, routeResult.Err)
		return
	}

	writeInvokeResponse(w, routeResult.Result)
}

// handleListTools handles GET /v1/tools.
func (s *Server) handleListTools(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, runtime.ErrCodeInvalidRequest, "method not allowed")
		return
	}

	registrarPID, err := s.resolveRegistrar(req.Context())
	if err != nil {
		s.logger.Warnf("ingress resolve registrar: %v", err)
		writeError(w, http.StatusServiceUnavailable, runtime.ErrCodeInternal, "runtime unavailable")
		return
	}

	resp, err := goaktactor.Ask(req.Context(), registrarPID, &runtime.ListTools{}, routingTimeout)
	if err != nil {
		s.logger.Warnf("ingress list tools: %v", err)
		writeError(w, http.StatusServiceUnavailable, runtime.ErrCodeInternal, "list tools failed")
		return
	}

	listResult, ok := resp.(*runtime.ListToolsResult)
	if !ok || listResult == nil {
		writeError(w, http.StatusInternalServerError, runtime.ErrCodeInternal, "invalid list response")
		return
	}

	items := make([]dto.ToolItem, 0, len(listResult.Tools))
	for _, t := range listResult.Tools {
		items = append(items, dto.ToolItem{
			ID:        string(t.ID),
			Transport: string(t.Transport),
			State:     string(t.State),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dto.ToolsListResponse{Tools: items})
}

// resolveRouter returns the RouterActor PID. Router is a child of GatewayManager.
func (s *Server) resolveRouter(ctx context.Context) (*goaktactor.PID, error) {
	managerPID, err := s.system.ActorOf(ctx, runtime.ActorNameGatewayManager)
	if err != nil || managerPID == nil {
		return nil, err
	}
	return managerPID.Child(runtime.ActorNameRouter)
}

// resolveRegistrar returns the Registrar PID. In single-node it is a child of GatewayManager;
// in cluster mode it is a singleton.
func (s *Server) resolveRegistrar(ctx context.Context) (*goaktactor.PID, error) {
	managerPID, err := s.system.ActorOf(ctx, runtime.ActorNameGatewayManager)
	if err != nil || managerPID == nil {
		return nil, err
	}
	pid, err := managerPID.Child(runtime.ActorNameRegistrar)
	if err == nil && pid != nil {
		return pid, nil
	}
	// Cluster mode: Registrar may be a system-level singleton
	return s.system.ActorOf(ctx, runtime.ActorNameRegistrar)
}
