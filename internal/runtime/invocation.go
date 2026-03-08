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

import "time"

// CorrelationMeta carries the request-scoped identity and tracing metadata
// that must propagate through every runtime actor message and log line.
//
// All runtime actors receiving an Invocation or producing an ExecutionResult
// should preserve and forward the CorrelationMeta unchanged.
type CorrelationMeta struct {
	// TenantID identifies the ownership boundary for this request.
	TenantID TenantID

	// ClientID identifies the caller within the tenant.
	ClientID ClientID

	// RequestID is a unique identifier for this individual invocation.
	RequestID RequestID

	// TraceID carries the distributed-trace context from the originating caller.
	TraceID TraceID
}

// IsZero reports whether the CorrelationMeta is entirely empty.
func (c CorrelationMeta) IsZero() bool {
	return c.TenantID.IsZero() && c.ClientID.IsZero() &&
		c.RequestID.IsZero() && c.TraceID.IsZero()
}

// Invocation represents one logical tool execution request.
//
// It carries the full identity context, correlation metadata, MCP payload, and
// timing information needed to execute a tool call and produce an auditable record.
// An Invocation is immutable once constructed; the runtime never mutates its fields.
type Invocation struct {
	// Correlation carries the request's tracing and tenant identity.
	Correlation CorrelationMeta

	// ToolID identifies the target tool.
	ToolID ToolID

	// SessionID identifies an existing session, if the caller has one.
	// A zero value indicates no existing session; the runtime will resolve or create one.
	SessionID SessionID

	// Method is the MCP method name for this invocation (e.g., "tools/call").
	Method string

	// Params is the MCP params payload. The structure is method-specific and is
	// passed through without runtime-level interpretation.
	Params map[string]any

	// Metadata holds additional caller-provided context (e.g., source system, tags).
	// This metadata is forwarded to audit and telemetry sinks but not interpreted
	// by the runtime routing or policy layers.
	Metadata map[string]string

	// Credentials holds resolved secrets for the tool invocation when
	// CredentialPolicyRequired. Populated by the router after credential resolution.
	// Nil when credentials are optional or not yet resolved.
	Credentials map[string]string

	// ReceivedAt is the wall-clock time when the invocation entered the runtime.
	ReceivedAt time.Time
}

// ExecutionStatus describes the outcome category of a completed tool invocation.
type ExecutionStatus string

const (
	// ExecutionStatusSuccess means the tool executed and returned a result.
	ExecutionStatusSuccess ExecutionStatus = "success"

	// ExecutionStatusFailure means the tool executed but returned an error.
	ExecutionStatusFailure ExecutionStatus = "failure"

	// ExecutionStatusTimeout means the invocation exceeded its deadline.
	ExecutionStatusTimeout ExecutionStatus = "timeout"

	// ExecutionStatusDenied means the request was rejected by the policy layer.
	ExecutionStatusDenied ExecutionStatus = "denied"

	// ExecutionStatusThrottled means the request was rejected due to quota or
	// rate-limit enforcement.
	ExecutionStatusThrottled ExecutionStatus = "throttled"
)

// ExecutionResult is the normalized outcome returned by the runtime after
// a tool invocation completes. Ingress adapters map ExecutionResult values
// into protocol-specific responses (e.g., HTTP status codes and bodies).
//
// Exactly one of Output or Err is meaningful depending on Status.
type ExecutionResult struct {
	// Status describes the outcome category of the invocation.
	Status ExecutionStatus

	// Output holds the raw MCP response payload on a successful invocation.
	// This field is nil when Status is not ExecutionStatusSuccess.
	Output map[string]any

	// Err holds the runtime error when the invocation did not succeed.
	// This field is nil when Status is ExecutionStatusSuccess.
	Err *RuntimeError

	// Duration is the total wall-clock time from invocation receipt to result.
	Duration time.Duration

	// Correlation echoes the originating invocation's correlation metadata so
	// results can be matched back to their requests without additional context.
	Correlation CorrelationMeta
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
