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

// Package audit provides the audit event schema and sink abstractions for the
// runtime. Policy decisions, tool invocations, failures, and health transitions
// are recorded for observability and compliance.
package audit

import "time"

// EventType identifies the category of an audit event.
type EventType string

const (
	// EventTypePolicyDecision records a policy allow/deny/throttle decision.
	EventTypePolicyDecision EventType = "policy_decision"

	// EventTypeInvocationStart records the start of a tool invocation.
	EventTypeInvocationStart EventType = "invocation_start"

	// EventTypeInvocationComplete records the completion of a tool invocation.
	EventTypeInvocationComplete EventType = "invocation_complete"

	// EventTypeInvocationFailed records a failed invocation (routing, policy, credential, etc.).
	EventTypeInvocationFailed EventType = "invocation_failed"

	// EventTypeHealthTransition records a tool health state change.
	EventTypeHealthTransition EventType = "health_transition"

	// EventTypeCircuitStateChange records a circuit breaker state transition.
	EventTypeCircuitStateChange EventType = "circuit_state_change"
)

// Event is the canonical audit event schema.
//
// All runtime actors that produce auditable outcomes should emit events
// conforming to this schema. The JournalActor writes events to the configured sink.
type Event struct {
	// Type identifies the event category.
	Type EventType

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

// Sink is the abstraction for durable audit storage.
//
// Implementations must be safe for concurrent use. Write may be asynchronous;
// the JournalActor buffers and batches as needed.
type Sink interface {
	// Write persists an audit event. Returns an error if the event cannot be stored.
	Write(event *Event) error

	// Close releases resources. No further writes should be attempted after Close.
	Close() error
}

// HealthTransitionEvent creates an audit event for a tool health state change.
func HealthTransitionEvent(toolID, fromState, toState string) *Event {
	meta := map[string]string{"from": fromState, "to": toState}
	return &Event{
		Type:      EventTypeHealthTransition,
		Timestamp: time.Now(),
		ToolID:    toolID,
		Outcome:   toState,
		Metadata:  meta,
	}
}

// CircuitStateChangeEvent creates an audit event for a circuit breaker transition.
func CircuitStateChangeEvent(toolID, state string, metadata map[string]string) *Event {
	return &Event{
		Type:      EventTypeCircuitStateChange,
		Timestamp: time.Now(),
		ToolID:    toolID,
		Outcome:   state,
		Metadata:  metadata,
	}
}
