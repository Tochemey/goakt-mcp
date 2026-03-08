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

// CircuitState represents the circuit breaker state for a tool supervisor.
type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

// CircuitConfig holds the parameters for circuit breaker behavior.
type CircuitConfig struct {
	FailureThreshold    int
	OpenDuration        time.Duration
	HalfOpenMaxRequests int
}

const (
	DefaultCircuitFailureThreshold    = 5
	DefaultCircuitOpenDuration        = 30 * time.Second
	DefaultCircuitHalfOpenMaxRequests = 1
)

// CanAccept returns true when the circuit allows new requests.
func (s CircuitState) CanAccept() bool {
	return s == CircuitClosed || s == CircuitHalfOpen
}

// IsOpen returns true when the circuit is open (fail fast).
func (s CircuitState) IsOpen() bool {
	return s == CircuitOpen
}
