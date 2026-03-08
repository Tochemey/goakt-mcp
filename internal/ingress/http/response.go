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

package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/tochemey/goakt-mcp/internal/ingress/dto"
	"github.com/tochemey/goakt-mcp/internal/runtime"
)

// writeError writes a JSON error response with the given status and error code.
func writeError(w http.ResponseWriter, status int, code runtime.ErrorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(dto.InvokeResponse{
		Status: statusToStatus(code),
		Error: &dto.ErrorDetail{
			Code:    string(code),
			Message: message,
		},
	})
}

// writeRuntimeError maps a RuntimeError to HTTP status and writes the response.
func writeRuntimeError(w http.ResponseWriter, err error) {
	var re *runtime.RuntimeError
	if errors.As(err, &re) {
		status := runtimeErrorToHTTPStatus(re.Code)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(dto.InvokeResponse{
			Status: statusToStatus(re.Code),
			Error: &dto.ErrorDetail{
				Code:    string(re.Code),
				Message: re.Message,
			},
		})
		return
	}
	writeError(w, http.StatusInternalServerError, runtime.ErrCodeInternal, err.Error())
}

// writeInvokeResponse writes a successful or failed invocation result as JSON.
func writeInvokeResponse(w http.ResponseWriter, result *runtime.ExecutionResult) {
	if result == nil {
		writeError(w, http.StatusInternalServerError, runtime.ErrCodeInternal, "no result")
		return
	}

	status := http.StatusOK
	if !result.Succeeded() {
		status = executionResultToHTTPStatus(result)
	}

	resp := dto.InvokeResponse{
		Status:     string(result.Status),
		Output:     result.Output,
		DurationMs: result.Duration.Milliseconds(),
		Correlation: &dto.CorrelationMeta{
			TenantID:  string(result.Correlation.TenantID),
			ClientID:  string(result.Correlation.ClientID),
			RequestID: string(result.Correlation.RequestID),
			TraceID:   string(result.Correlation.TraceID),
		},
	}
	if result.Err != nil {
		resp.Error = &dto.ErrorDetail{
			Code:    string(result.Err.Code),
			Message: result.Err.Message,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

// runtimeErrorToHTTPStatus maps a runtime error code to the appropriate HTTP
// status code for the JSON error response.
func runtimeErrorToHTTPStatus(code runtime.ErrorCode) int {
	switch code {
	case runtime.ErrCodeToolNotFound:
		return http.StatusNotFound
	case runtime.ErrCodeToolUnavailable, runtime.ErrCodeToolDisabled:
		return http.StatusServiceUnavailable
	case runtime.ErrCodePolicyDenied:
		return http.StatusForbidden
	case runtime.ErrCodeQuotaExceeded, runtime.ErrCodeRateLimited, runtime.ErrCodeConcurrencyLimitReached:
		return http.StatusTooManyRequests
	case runtime.ErrCodeInvalidRequest:
		return http.StatusBadRequest
	case runtime.ErrCodeInvocationTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

// executionResultToHTTPStatus derives the HTTP status code from an execution
// result, considering both its error code and execution status.
func executionResultToHTTPStatus(r *runtime.ExecutionResult) int {
	if r.Err != nil {
		return runtimeErrorToHTTPStatus(r.Err.Code)
	}
	switch r.Status {
	case runtime.ExecutionStatusTimeout:
		return http.StatusGatewayTimeout
	case runtime.ExecutionStatusDenied:
		return http.StatusForbidden
	case runtime.ExecutionStatusThrottled:
		return http.StatusTooManyRequests
	case runtime.ExecutionStatusFailure:
		return http.StatusInternalServerError
	default:
		return http.StatusOK
	}
}

// statusToStatus maps a runtime error code to the status label used in the
// InvokeResponse JSON body (e.g. "failure", "denied", "throttled").
func statusToStatus(code runtime.ErrorCode) string {
	switch code {
	case runtime.ErrCodeToolNotFound:
		return "failure"
	case runtime.ErrCodePolicyDenied:
		return "denied"
	case runtime.ErrCodeQuotaExceeded, runtime.ErrCodeRateLimited, runtime.ErrCodeConcurrencyLimitReached:
		return "throttled"
	case runtime.ErrCodeInvocationTimeout:
		return "timeout"
	default:
		return "failure"
	}
}
