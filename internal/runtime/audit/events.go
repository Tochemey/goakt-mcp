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

package audit

import (
	"time"

	"github.com/tochemey/goakt-mcp/mcp"
)

// HealthTransitionAuditEvent creates an audit event for a tool health state change.
func HealthTransitionAuditEvent(toolID, fromState, toState string) *mcp.AuditEvent {
	meta := map[string]string{"from": fromState, "to": toState}
	return &mcp.AuditEvent{
		Type:      mcp.AuditEventTypeHealthTransition,
		Timestamp: time.Now(),
		ToolID:    toolID,
		Outcome:   toState,
		Metadata:  meta,
	}
}

// CircuitStateChangeAuditEvent creates an audit event for a circuit breaker transition.
func CircuitStateChangeAuditEvent(toolID, state string, metadata map[string]string) *mcp.AuditEvent {
	return &mcp.AuditEvent{
		Type:      mcp.AuditEventTypeCircuitStateChange,
		Timestamp: time.Now(),
		ToolID:    toolID,
		Outcome:   state,
		Metadata:  metadata,
	}
}
