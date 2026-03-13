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

// ProgressEvent represents a single progress notification from a backend
// MCP server. Fields mirror the MCP notifications/progress schema.
type ProgressEvent struct {
	// ProgressToken is the opaque token that correlates this progress
	// notification with the originating request.
	ProgressToken any

	// Progress is the current progress value (e.g., bytes transferred).
	Progress float64

	// Total is the expected total value. Zero means total is unknown.
	Total float64

	// Message is an optional human-readable description of the current step.
	Message string
}

// StreamingResult wraps a tool invocation's progress stream and final result.
//
// Callers must read from Progress until it is closed, then read the final
// ExecutionResult from Final. Both channels are closed by the producer;
// callers must not close them.
type StreamingResult struct {
	// Progress delivers zero or more progress events before the final result.
	// The channel is closed when no more progress events will arrive.
	Progress <-chan ProgressEvent

	// Final delivers exactly one result and is then closed.
	Final <-chan *ExecutionResult
}

// Collect drains the progress channel and returns the final result.
// This is a convenience for callers that do not need to observe progress.
func (s *StreamingResult) Collect() *ExecutionResult {
	for range s.Progress { //nolint:revive // intentional channel drain
	}
	return <-s.Final
}
