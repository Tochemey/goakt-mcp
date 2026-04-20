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

import "time"

// CorrelationMeta carries the request-scoped identity and tracing metadata
// that must propagate through every runtime actor message and log line.
type CorrelationMeta struct {
	TenantID  TenantID
	ClientID  ClientID
	RequestID RequestID
	TraceID   TraceID
}

// Invocation represents one logical tool execution request.
//
// It carries the full identity context, correlation metadata, MCP payload, and
// timing information needed to execute a tool call and produce an auditable record.
type Invocation struct {
	// Correlation carries the tenant, client, request, and trace identifiers
	// that propagate through every runtime actor and log line.
	Correlation CorrelationMeta
	// ToolID is the identifier of the tool to invoke.
	ToolID ToolID
	// SessionID, when non-empty, pins the invocation to an existing tool session.
	// The router creates a new session when this field is empty.
	SessionID SessionID
	// Method is the MCP JSON-RPC method name (e.g. "tools/call").
	Method string
	// Params holds the raw method parameters forwarded to the backend MCP server.
	// For tool calls, the map contains "name" (string) and "arguments" (map[string]any).
	Params map[string]any
	// Metadata holds arbitrary key-value pairs injected by middleware layers
	// (e.g. ingress headers, tracing baggage).
	Metadata map[string]string
	// Credentials holds resolved secrets injected by the credential broker
	// before the invocation reaches the egress layer.
	Credentials map[string]string
	// Scopes holds the OAuth scopes granted by the validated bearer token.
	// Populated by the enterprise-managed authorization middleware when
	// [EnterpriseAuthConfig] is active on the ingress. Empty when enterprise
	// auth is not configured or the token carries no scopes.
	//
	// Scopes are propagated to [PolicyInput] so that custom [PolicyEvaluator]
	// implementations can make scope-aware authorization decisions.
	Scopes []string
	// ReceivedAt is the wall-clock time the invocation was accepted by the
	// ingress layer, used to compute end-to-end latency in audit records.
	ReceivedAt time.Time
}

// ExecutionStatus describes the outcome category of a completed tool invocation.
type ExecutionStatus string

// ExecutionResult is the normalized outcome returned by the runtime after
// a tool invocation completes.
type ExecutionResult struct {
	Status      ExecutionStatus
	Output      map[string]any
	Err         *RuntimeError
	Duration    time.Duration
	Correlation CorrelationMeta
}

const (
	// ExecutionStatusSuccess indicates the tool call completed and returned output.
	ExecutionStatusSuccess ExecutionStatus = "success"
	// ExecutionStatusFailure indicates the tool call completed but the backend
	// reported an error (transport failure, non-zero exit, protocol error).
	ExecutionStatusFailure ExecutionStatus = "failure"
	// ExecutionStatusTimeout indicates the invocation exceeded its deadline
	// before the backend returned a response.
	ExecutionStatusTimeout ExecutionStatus = "timeout"
	// ExecutionStatusDenied indicates the policy layer rejected the invocation
	// (tool disabled, tenant not authorized, authorization policy denied).
	ExecutionStatusDenied ExecutionStatus = "denied"
	// ExecutionStatusThrottled indicates the invocation was rejected because the
	// tenant exceeded its rate or concurrency quota.
	ExecutionStatusThrottled ExecutionStatus = "throttled"
)

// IsZero reports whether the CorrelationMeta is entirely empty.
func (c CorrelationMeta) IsZero() bool {
	return c.TenantID.IsZero() && c.ClientID.IsZero() &&
		c.RequestID.IsZero() && c.TraceID.IsZero()
}

// Succeeded reports whether the invocation completed with a successful result.
func (r ExecutionResult) Succeeded() bool { return r.Status == ExecutionStatusSuccess }

// Failed reports whether the invocation ended in a runtime or tool-level failure.
func (r ExecutionResult) Failed() bool { return r.Status == ExecutionStatusFailure }

// TimedOut reports whether the invocation exceeded its deadline.
func (r ExecutionResult) TimedOut() bool { return r.Status == ExecutionStatusTimeout }

// Denied reports whether the invocation was rejected by the policy layer.
func (r ExecutionResult) Denied() bool { return r.Status == ExecutionStatusDenied }

// Throttled reports whether the invocation was rejected by quota or rate limiting.
func (r ExecutionResult) Throttled() bool { return r.Status == ExecutionStatusThrottled }
