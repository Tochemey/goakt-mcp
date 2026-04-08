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

import (
	"time"
)

// AuditEventType identifies the category of an audit event.
type AuditEventType string

const (
	// AuditEventTypePolicyDecision records a policy allow/deny/throttle decision.
	AuditEventTypePolicyDecision AuditEventType = "policy_decision"

	// AuditEventTypeInvocationStart records the start of a tool invocation.
	AuditEventTypeInvocationStart AuditEventType = "invocation_start"

	// AuditEventTypeInvocationComplete records the completion of a tool invocation.
	AuditEventTypeInvocationComplete AuditEventType = "invocation_complete"

	// AuditEventTypeInvocationFailed records a failed invocation (routing, policy, credential, etc.).
	AuditEventTypeInvocationFailed AuditEventType = "invocation_failed"

	// AuditEventTypeHealthTransition records a tool health state change.
	AuditEventTypeHealthTransition AuditEventType = "health_transition"

	// AuditEventTypeCircuitStateChange records a circuit breaker state transition.
	AuditEventTypeCircuitStateChange AuditEventType = "circuit_state_change"
)

// AuditEvent is the canonical audit event schema.
//
// All runtime actors that produce auditable outcomes should emit events
// conforming to this schema. The JournalActor writes events to the configured sink.
type AuditEvent struct {
	// Type identifies the event category.
	Type AuditEventType

	// Timestamp is when the event occurred.
	Timestamp time.Time

	// TenantID is the tenant context when applicable.
	TenantID string

	// ClientID is the client context when applicable.
	ClientID string

	// ToolID is the tool context when applicable.
	ToolID string

	// RequestID correlates the event to a specific invocation when applicable.
	RequestID string

	// TraceID carries distributed trace context when available.
	TraceID string

	// Outcome describes the result: "allow", "deny", "throttle", "success",
	// "failure", "timeout", "denied", "throttled", "error_code", etc.
	Outcome string

	// ErrorCode is the runtime error code when the outcome is an error.
	ErrorCode string

	// Message is a human-readable description.
	Message string

	// Metadata holds additional context (e.g., circuit state, session count).
	Metadata map[string]string
}

// AuditSink is the abstraction for durable audit storage.
//
// Implementations must be safe for concurrent use. Write may be asynchronous;
// the JournalActor buffers and batches as needed.
type AuditSink interface {
	// Write persists an audit event. Returns an error if the event cannot be stored.
	Write(event *AuditEvent) error

	// Close releases resources. No further writes should be attempted after Close.
	Close() error
}
