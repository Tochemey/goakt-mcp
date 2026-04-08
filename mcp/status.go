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

package mcp

// ToolStatus describes the current operational state of a registered tool.
type ToolStatus struct {
	// ToolID identifies the tool.
	ToolID ToolID
	// State is the current operational state of the tool.
	State ToolState
	// Circuit is the current circuit breaker state of the tool supervisor.
	Circuit CircuitState
	// SessionCount is the number of active sessions for this tool.
	SessionCount int
	// Draining is true when the tool has been administratively drained.
	// When draining, the supervisor rejects new sessions but existing sessions
	// continue until they passivate or complete.
	Draining bool
	// Schemas holds the MCP tool schemas discovered from the backend server.
	Schemas []ToolSchema
}

// GatewayStatus describes the current operational state of the gateway.
type GatewayStatus struct {
	// Running is true when the gateway actor system is started and operational.
	Running bool
	// ToolCount is the number of tools currently registered in the registry.
	ToolCount int
	// SessionCount is the total number of active sessions across all tools.
	SessionCount int
}

// SessionInfo describes a single active session.
type SessionInfo struct {
	// Name is the deterministic actor name of the session.
	Name string
	// ToolID is the identifier of the tool the session is bound to.
	ToolID ToolID
	// TenantID is the tenant the session was created for.
	TenantID TenantID
	// ClientID is the client the session was created for.
	ClientID ClientID
}
