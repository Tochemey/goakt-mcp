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

package runtime

import "github.com/tochemey/goakt-mcp/mcp"

// Supervisor command and response types for ToolSupervisorActor.
//
// These messages define the contract between the supervisor and its callers
// (routing, sessions, health probes). Use Ask for request-response, Tell for
// fire-and-forget notifications.

// CanAcceptWork is a request to determine whether the supervisor can accept new work.
//
// The supervisor considers circuit state, tool availability, and administrative
// disable status. Must be used with Ask. Response is CanAcceptWorkResult.
type CanAcceptWork struct {
	ToolID mcp.ToolID
}

// CanAcceptWorkResult is the response to CanAcceptWork.
// SessionCount is populated for callers that need backpressure info without
// a separate round-trip.
type CanAcceptWorkResult struct {
	Accept       bool
	Reason       string
	SessionCount int
}

// ReportFailure notifies the supervisor that an invocation or session failed.
//
// The supervisor increments its failure count and may open the circuit.
// Typically sent via Tell from sessions or transport adapters.
type ReportFailure struct {
	ToolID mcp.ToolID
}

// ReportSuccess notifies the supervisor that an invocation succeeded.
//
// In closed state, this resets the failure counter. In half-open state,
// this closes the circuit. Typically sent via Tell.
type ReportSuccess struct {
	ToolID mcp.ToolID
}

// SupervisorCountSessionsForTenant is a request to count this supervisor's
// sessions that belong to the given tenant. Session names follow
// SessionName(tenantID, clientID, toolID). Must be used with Ask.
// Response is SupervisorCountSessionsForTenantResult.
type SupervisorCountSessionsForTenant struct {
	TenantID mcp.TenantID
}

// SupervisorCountSessionsForTenantResult is the response.
type SupervisorCountSessionsForTenantResult struct {
	Count int
}

// RefreshToolConfig notifies a ToolSupervisor that its tool configuration
// has changed in the ToolConfigExtension. The supervisor re-reads the
// updated config from the extension. Typically sent via Tell from the
// Registrar after an UpdateTool or EnableTool.
type RefreshToolConfig struct {
	ToolID mcp.ToolID
}
