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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		et       AuditEventType
		expected string
	}{
		{"PolicyDecision", AuditEventTypePolicyDecision, "policy_decision"},
		{"InvocationStart", AuditEventTypeInvocationStart, "invocation_start"},
		{"InvocationComplete", AuditEventTypeInvocationComplete, "invocation_complete"},
		{"InvocationFailed", AuditEventTypeInvocationFailed, "invocation_failed"},
		{"HealthTransition", AuditEventTypeHealthTransition, "health_transition"},
		{"CircuitStateChange", AuditEventTypeCircuitStateChange, "circuit_state_change"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, string(tc.et))
		})
	}
}

func TestEventConstruction(t *testing.T) {
	now := time.Now()
	event := &AuditEvent{
		Type:      AuditEventTypeInvocationComplete,
		Timestamp: now,
		TenantID:  "tenant-1",
		ClientID:  "client-1",
		ToolID:    "tool-1",
		RequestID: "req-1",
		TraceID:   "trace-1",
		Outcome:   "success",
		ErrorCode: "",
		Message:   "completed",
		Metadata:  map[string]string{"key": "value"},
	}

	assert.Equal(t, AuditEventTypeInvocationComplete, event.Type)
	assert.Equal(t, now, event.Timestamp)
	assert.Equal(t, "tenant-1", event.TenantID)
	assert.Equal(t, "client-1", event.ClientID)
	assert.Equal(t, "tool-1", event.ToolID)
	assert.Equal(t, "req-1", event.RequestID)
	assert.Equal(t, "trace-1", event.TraceID)
	assert.Equal(t, "success", event.Outcome)
	assert.Empty(t, event.ErrorCode)
	assert.Equal(t, "completed", event.Message)
	assert.Equal(t, "value", event.Metadata["key"])
}

func TestEventZeroValue(t *testing.T) {
	var event AuditEvent
	assert.Equal(t, AuditEventType(""), event.Type)
	assert.True(t, event.Timestamp.IsZero())
	assert.Empty(t, event.TenantID)
	assert.Empty(t, event.ToolID)
	assert.Nil(t, event.Metadata)
}

func TestHealthTransitionEvent(t *testing.T) {
	ev := HealthTransitionAuditEvent("tool-1", "enabled", "degraded")
	require.NotNil(t, ev)
	assert.Equal(t, AuditEventTypeHealthTransition, ev.Type)
	assert.Equal(t, "tool-1", ev.ToolID)
	assert.Equal(t, "degraded", ev.Outcome)
	assert.NotZero(t, ev.Timestamp)
	require.NotNil(t, ev.Metadata)
	assert.Equal(t, "enabled", ev.Metadata["from"])
	assert.Equal(t, "degraded", ev.Metadata["to"])
}

func TestCircuitStateChangeEvent(t *testing.T) {
	meta := map[string]string{"reason": "failure_threshold", "count": "5"}
	ev := CircuitStateChangeAuditEvent("tool-1", "open", meta)
	require.NotNil(t, ev)
	assert.Equal(t, AuditEventTypeCircuitStateChange, ev.Type)
	assert.Equal(t, "tool-1", ev.ToolID)
	assert.Equal(t, "open", ev.Outcome)
	assert.NotZero(t, ev.Timestamp)
	assert.Equal(t, meta, ev.Metadata)

	ev = CircuitStateChangeAuditEvent("tool-2", "closed", nil)
	require.NotNil(t, ev)
	assert.Nil(t, ev.Metadata)
}
