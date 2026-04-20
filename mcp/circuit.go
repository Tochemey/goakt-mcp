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

// CircuitAcquireOutcome is the result of CircuitBreaker.Acquire.
type CircuitAcquireOutcome int

// CircuitTransitionReason identifies why a circuit transition occurred. The
// values are stable strings used in audit metadata and telemetry.
type CircuitTransitionReason string

// CircuitTransition captures a single state change produced by the breaker.
// A nil transition means no state change occurred.
type CircuitTransition struct {
	From         CircuitState
	To           CircuitState
	Reason       CircuitTransitionReason
	FailureCount int
}

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

// CircuitBreaker is a per-tool, rolling state machine that protects a backend
// from cascading calls after consecutive failures. It transitions through
// Closed → Open → HalfOpen → Closed based on reported outcomes and elapsed
// time in the Open state.
//
// CircuitBreaker is NOT safe for concurrent mutation. It is designed to be
// owned by a single actor goroutine; callers must serialize access externally
// (which the actor runtime does by virtue of its single-threaded Receive).
type CircuitBreaker struct {
	config           CircuitConfig
	state            CircuitState
	failureCount     int
	openedAt         time.Time
	halfOpenInFlight int
	now              func() time.Time
}

const (
	// CircuitClosed is the nominal state: calls flow through and failures are counted.
	CircuitClosed CircuitState = "closed"
	// CircuitOpen fails every call fast without contacting the backend.
	CircuitOpen CircuitState = "open"
	// CircuitHalfOpen permits a bounded number of probe calls to assess recovery.
	CircuitHalfOpen CircuitState = "half_open"
)

const (
	// CircuitAcquireAccepted means the caller may proceed with the backend call.
	CircuitAcquireAccepted CircuitAcquireOutcome = iota
	// CircuitAcquireRejectedOpen means the circuit is open and the call must fail fast.
	CircuitAcquireRejectedOpen
	// CircuitAcquireRejectedHalfOpenLimit means the circuit is half-open and the
	// concurrent probe limit has been reached.
	CircuitAcquireRejectedHalfOpenLimit
)

const (
	// CircuitReasonThresholdExceeded marks a Closed→Open transition caused by
	// reaching the consecutive-failure threshold.
	CircuitReasonThresholdExceeded CircuitTransitionReason = "failure_threshold_exceeded"
	// CircuitReasonHalfOpenProbeFailed marks a HalfOpen→Open transition caused
	// by a failed probe.
	CircuitReasonHalfOpenProbeFailed CircuitTransitionReason = "half_open_probe_failed"
	// CircuitReasonHalfOpenProbeSucceeded marks a HalfOpen→Closed transition
	// caused by a successful probe.
	CircuitReasonHalfOpenProbeSucceeded CircuitTransitionReason = "half_open_probe_success"
	// CircuitReasonOpenDurationElapsed marks an Open→HalfOpen transition caused
	// by the open duration elapsing.
	CircuitReasonOpenDurationElapsed CircuitTransitionReason = "open_duration_elapsed"
	// CircuitReasonManualReset marks a manual reset to Closed.
	CircuitReasonManualReset CircuitTransitionReason = "manual_reset"
)

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

// NewCircuitBreaker constructs a CircuitBreaker with the given configuration.
// Zero-valued fields are replaced with the corresponding Default* constants.
// The wall clock is used internally; use NewCircuitBreakerWithClock to inject
// a custom clock (typically for tests).
func NewCircuitBreaker(config CircuitConfig) *CircuitBreaker {
	return NewCircuitBreakerWithClock(config, time.Now)
}

// NewCircuitBreakerWithClock constructs a CircuitBreaker using the provided
// clock function to determine elapsed time. Passing a nil clock falls back to
// time.Now.
func NewCircuitBreakerWithClock(config CircuitConfig, clock func() time.Time) *CircuitBreaker {
	if clock == nil {
		clock = time.Now
	}
	return &CircuitBreaker{
		config: sanitizeCircuitConfig(config),
		state:  CircuitClosed,
		now:    clock,
	}
}

// State returns the current circuit state without side effects. Because Open→
// HalfOpen transitions are driven by elapsed time, callers that need the
// time-adjusted state should call Peek or Acquire instead.
func (b *CircuitBreaker) State() CircuitState { return b.state }

// FailureCount returns the current consecutive-failure count. It resets to
// zero on success in Closed, on HalfOpen→Closed, and on Reset.
func (b *CircuitBreaker) FailureCount() int { return b.failureCount }

// Config returns the effective (sanitized) configuration in use.
func (b *CircuitBreaker) Config() CircuitConfig { return b.config }

// Peek returns the current state after applying any pending time-driven
// transitions (Open → HalfOpen when OpenDuration has elapsed). It does not
// reserve a probe slot and is safe to call from status-reporting paths.
func (b *CircuitBreaker) Peek() (CircuitState, *CircuitTransition) {
	transition := b.maybeHalfOpen()
	return b.state, transition
}

// Acquire attempts to reserve a slot for a forthcoming backend call. The
// returned CircuitAcquireOutcome tells the caller whether to proceed. If a
// transition occurred (Open → HalfOpen when OpenDuration elapsed), it is
// returned so the caller can emit audit / telemetry events.
//
// Acquire does not block. When outcome is CircuitAcquireAccepted and the
// circuit is half-open, the caller is committed to reporting the outcome via
// OnSuccess or OnFailure so the probe slot is released via state transition.
func (b *CircuitBreaker) Acquire() (CircuitAcquireOutcome, *CircuitTransition) {
	transition := b.maybeHalfOpen()

	switch b.state {
	case CircuitClosed:
		return CircuitAcquireAccepted, transition
	case CircuitHalfOpen:
		if b.halfOpenInFlight >= b.config.HalfOpenMaxRequests {
			return CircuitAcquireRejectedHalfOpenLimit, transition
		}
		b.halfOpenInFlight++
		return CircuitAcquireAccepted, transition
	default:
		return CircuitAcquireRejectedOpen, transition
	}
}

// OnSuccess records a successful backend call. A HalfOpen probe success
// transitions the circuit to Closed; a Closed success resets the failure
// counter. Returns a non-nil transition when the state changed.
func (b *CircuitBreaker) OnSuccess() *CircuitTransition {
	switch b.state {
	case CircuitHalfOpen:
		return b.transitionTo(CircuitClosed, CircuitReasonHalfOpenProbeSucceeded)
	case CircuitClosed:
		b.failureCount = 0
	}
	return nil
}

// OnFailure records a failed backend call. In Closed, the failure counter is
// incremented and the circuit opens once FailureThreshold is reached. In
// HalfOpen, any failure immediately re-opens the circuit. Returns a non-nil
// transition when the state changed.
func (b *CircuitBreaker) OnFailure() *CircuitTransition {
	switch b.state {
	case CircuitHalfOpen:
		return b.transitionTo(CircuitOpen, CircuitReasonHalfOpenProbeFailed)
	case CircuitClosed:
		b.failureCount++
		if b.failureCount >= b.config.FailureThreshold {
			return b.transitionTo(CircuitOpen, CircuitReasonThresholdExceeded)
		}
	}
	return nil
}

// Reset forces the circuit into Closed state, clears the failure counter, and
// releases any in-flight half-open probe slots. Returns a non-nil transition
// when the state actually changed; the returned transition uses
// CircuitReasonManualReset.
func (b *CircuitBreaker) Reset() *CircuitTransition {
	if b.state == CircuitClosed {
		b.failureCount = 0
		b.halfOpenInFlight = 0
		return nil
	}
	return b.transitionTo(CircuitClosed, CircuitReasonManualReset)
}

// maybeHalfOpen transitions Open → HalfOpen when OpenDuration has elapsed.
// It is called from Peek and Acquire to implement the time-driven recovery
// probe.
func (b *CircuitBreaker) maybeHalfOpen() *CircuitTransition {
	if b.state != CircuitOpen {
		return nil
	}
	if b.now().Sub(b.openedAt) < b.config.OpenDuration {
		return nil
	}
	return b.transitionTo(CircuitHalfOpen, CircuitReasonOpenDurationElapsed)
}

// transitionTo performs the state change and resets the counters and timers
// that belong to the target state. It returns a CircuitTransition describing
// the change so the caller can audit it.
func (b *CircuitBreaker) transitionTo(target CircuitState, reason CircuitTransitionReason) *CircuitTransition {
	prev := b.state
	failureCount := b.failureCount

	b.state = target
	switch target {
	case CircuitOpen:
		b.openedAt = b.now()
		b.halfOpenInFlight = 0
	case CircuitHalfOpen:
		b.halfOpenInFlight = 0
	case CircuitClosed:
		b.failureCount = 0
		b.halfOpenInFlight = 0
	}

	return &CircuitTransition{
		From:         prev,
		To:           target,
		Reason:       reason,
		FailureCount: failureCount,
	}
}

// sanitizeCircuitConfig replaces zero fields with the corresponding defaults.
func sanitizeCircuitConfig(config CircuitConfig) CircuitConfig {
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = DefaultCircuitFailureThreshold
	}
	if config.OpenDuration <= 0 {
		config.OpenDuration = DefaultCircuitOpenDuration
	}
	if config.HalfOpenMaxRequests <= 0 {
		config.HalfOpenMaxRequests = DefaultCircuitHalfOpenMaxRequests
	}
	return config
}
