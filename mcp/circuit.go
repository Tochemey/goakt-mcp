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

// CircuitConfig holds the parameters for circuit breaker behavior on a tool
// supervisor. When any field is zero, the corresponding default constant is used.
type CircuitConfig struct {
	// FailureThreshold is the number of consecutive execution failures that trip
	// the circuit from closed to open. Zero means use DefaultCircuitFailureThreshold.
	FailureThreshold int
	// OpenDuration is how long the circuit stays open (fail-fast) before
	// transitioning to half-open to probe recovery.
	// Zero means use DefaultCircuitOpenDuration.
	OpenDuration time.Duration
	// HalfOpenMaxRequests is the number of probe requests permitted while the
	// circuit is half-open. A successful probe closes the circuit; a failure
	// re-opens it. Zero means use DefaultCircuitHalfOpenMaxRequests.
	HalfOpenMaxRequests int
}

const (
	// DefaultCircuitFailureThreshold is the consecutive-failure count that trips
	// a circuit breaker from closed to open when CircuitConfig.FailureThreshold is zero.
	DefaultCircuitFailureThreshold = 5
	// DefaultCircuitOpenDuration is the time a tripped circuit stays open before
	// transitioning to half-open when CircuitConfig.OpenDuration is zero.
	DefaultCircuitOpenDuration = 30 * time.Second
	// DefaultCircuitHalfOpenMaxRequests is the number of probe requests allowed
	// while the circuit is half-open when CircuitConfig.HalfOpenMaxRequests is zero.
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
