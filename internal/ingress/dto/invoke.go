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

package dto

import "fmt"

// InvokeRequest is the HTTP request body for POST /v1/tools/{tool}/invoke.
//
// All correlation fields are required. Method and Params define the MCP call.
type InvokeRequest struct {
	// TenantID identifies the ownership boundary for this request.
	TenantID string `json:"tenant_id"`

	// ClientID identifies the caller within the tenant.
	ClientID string `json:"client_id"`

	// RequestID is a unique identifier for this invocation.
	RequestID string `json:"request_id"`

	// TraceID carries the distributed-trace context from the originating caller.
	TraceID string `json:"trace_id"`

	// Method is the MCP method name (e.g., "tools/call").
	Method string `json:"method"`

	// Params is the MCP params payload. Structure is method-specific.
	Params map[string]any `json:"params"`

	// Metadata holds additional caller-provided context.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Validate returns an error if required fields are missing or invalid.
func (r *InvokeRequest) Validate() error {
	if r.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if r.ClientID == "" {
		return fmt.Errorf("client_id is required")
	}
	if r.RequestID == "" {
		return fmt.Errorf("request_id is required")
	}
	if r.TraceID == "" {
		return fmt.Errorf("trace_id is required")
	}
	if r.Method == "" {
		return fmt.Errorf("method is required")
	}
	if r.Params == nil {
		return fmt.Errorf("params is required")
	}
	return nil
}

// InvokeResponse is the HTTP response body for POST /v1/tools/{tool}/invoke.
type InvokeResponse struct {
	// Status describes the outcome: "success", "failure", "timeout", "denied", "throttled".
	Status string `json:"status"`

	// Output holds the MCP response payload on success.
	Output map[string]any `json:"output,omitempty"`

	// Error holds the error details when status is not success.
	Error *ErrorDetail `json:"error,omitempty"`

	// DurationMs is the total wall-clock time in milliseconds.
	DurationMs int64 `json:"duration_ms"`

	// Correlation echoes the request correlation metadata.
	Correlation *CorrelationMeta `json:"correlation,omitempty"`
}

// ErrorDetail holds structured error information for HTTP responses.
type ErrorDetail struct {
	// Code is the stable machine-readable error category.
	Code string `json:"code"`

	// Message is a human-readable description.
	Message string `json:"message"`
}

// CorrelationMeta holds request correlation metadata in responses.
type CorrelationMeta struct {
	TenantID  string `json:"tenant_id"`
	ClientID  string `json:"client_id"`
	RequestID string `json:"request_id"`
	TraceID   string `json:"trace_id"`
}
