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

// TenantID is the opaque identifier for a tenant.
//
// A tenant identifies an ownership boundary for policy enforcement, audit records,
// quota accounting, and routing rules. All runtime operations are scoped to a tenant.
type TenantID string

// ClientID is the opaque identifier for a caller within a tenant.
//
// A client identifies the session owner. Together with TenantID and ToolID it
// forms the unique key for a live session inside the runtime.
type ClientID string

// ToolID is the opaque identifier for a registered MCP tool.
//
// ToolID is used throughout the runtime to look up tool metadata, resolve the
// responsible supervisor, and construct deterministic actor names.
type ToolID string

// SessionID is the opaque identifier for a stateful session.
//
// A session is the live execution binding between one client and one tool.
// SessionID uniquely names that binding within the runtime.
type SessionID string

// RequestID is the opaque identifier for a single invocation request.
//
// RequestID correlates all log lines, trace spans, and audit records produced
// during the handling of one logical tool call.
type RequestID string

// TraceID carries the distributed-trace context from an inbound request into
// the runtime and through all actor messages.
//
// It is expected to follow the W3C trace-context format when supplied by callers.
type TraceID string

// IsZero reports whether the TenantID is the zero value (empty).
func (t TenantID) IsZero() bool { return t == "" }

// String returns the string representation of the TenantID.
func (t TenantID) String() string { return string(t) }

// IsZero reports whether the ClientID is the zero value (empty).
func (c ClientID) IsZero() bool { return c == "" }

// String returns the string representation of the ClientID.
func (c ClientID) String() string { return string(c) }

// IsZero reports whether the ToolID is the zero value (empty).
func (t ToolID) IsZero() bool { return t == "" }

// String returns the string representation of the ToolID.
func (t ToolID) String() string { return string(t) }

// IsZero reports whether the SessionID is the zero value (empty).
func (s SessionID) IsZero() bool { return s == "" }

// String returns the string representation of the SessionID.
func (s SessionID) String() string { return string(s) }

// IsZero reports whether the RequestID is the zero value (empty).
func (r RequestID) IsZero() bool { return r == "" }

// String returns the string representation of the RequestID.
func (r RequestID) String() string { return string(r) }

// IsZero reports whether the TraceID is the zero value (empty).
func (t TraceID) IsZero() bool { return t == "" }

// String returns the string representation of the TraceID.
func (t TraceID) String() string { return string(t) }
