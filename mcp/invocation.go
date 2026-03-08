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

// IsZero reports whether the CorrelationMeta is entirely empty.
func (c CorrelationMeta) IsZero() bool {
	return c.TenantID.IsZero() && c.ClientID.IsZero() &&
		c.RequestID.IsZero() && c.TraceID.IsZero()
}

// Invocation represents one logical tool execution request.
//
// It carries the full identity context, correlation metadata, MCP payload, and
// timing information needed to execute a tool call and produce an auditable record.
type Invocation struct {
	Correlation CorrelationMeta
	ToolID      ToolID
	SessionID   SessionID
	Method      string
	Params      map[string]any
	Metadata    map[string]string
	Credentials map[string]string
	ReceivedAt  time.Time
}

// ExecutionStatus describes the outcome category of a completed tool invocation.
type ExecutionStatus string

const (
	ExecutionStatusSuccess   ExecutionStatus = "success"
	ExecutionStatusFailure   ExecutionStatus = "failure"
	ExecutionStatusTimeout   ExecutionStatus = "timeout"
	ExecutionStatusDenied    ExecutionStatus = "denied"
	ExecutionStatusThrottled ExecutionStatus = "throttled"
)

// ExecutionResult is the normalized outcome returned by the runtime after
// a tool invocation completes.
type ExecutionResult struct {
	Status      ExecutionStatus
	Output      map[string]any
	Err         *RuntimeError
	Duration    time.Duration
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
