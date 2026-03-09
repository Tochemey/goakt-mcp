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

package telemetry

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/tochemey/goakt-mcp/mcp"
)

const instrumentationName = "github.com/tochemey/goakt-mcp/telemetry"

// Metrics holds OpenTelemetry instruments for goakt-mcp runtime observability.
// Created at gateway level via RegisterMetrics and used by actors via the
// Record* functions. Mirrors the GoAkt pattern of registering instruments at
// actor system creation.
type Metrics struct {
	toolAvailability  metric.Int64Counter
	invocationLatency metric.Float64Histogram
	invocationFailure metric.Int64Counter
	circuitState      metric.Int64Counter
}

var (
	defaultMetrics *Metrics
	metricsMu      sync.RWMutex
)

// RegisterMetrics creates OpenTelemetry instruments from the given Meter and
// registers them as the default metrics for the process. Call this at gateway
// Start when WithMetrics() is enabled. Returns an error if any instrument
// cannot be created.
//
// Pass nil to use otel.GetMeterProvider().Meter(instrumentationName).
func RegisterMetrics(m metric.Meter) (*Metrics, error) {
	if m == nil {
		m = otel.GetMeterProvider().Meter(instrumentationName)
	}

	metrics := &Metrics{}
	var err error

	if metrics.toolAvailability, err = m.Int64Counter(
		"goaktmcp.tool.availability",
		metric.WithDescription("Tool availability state transitions"),
	); err != nil {
		return nil, err
	}

	if metrics.invocationLatency, err = m.Float64Histogram(
		"goaktmcp.invocation.latency",
		metric.WithDescription("Tool invocation latency in milliseconds"),
		metric.WithUnit("ms"),
	); err != nil {
		return nil, err
	}

	if metrics.invocationFailure, err = m.Int64Counter(
		"goaktmcp.invocation.failure",
		metric.WithDescription("Failed tool invocations"),
	); err != nil {
		return nil, err
	}

	if metrics.circuitState, err = m.Int64Counter(
		"goaktmcp.circuit.state",
		metric.WithDescription("Circuit breaker state transitions"),
	); err != nil {
		return nil, err
	}

	metricsMu.Lock()
	defaultMetrics = metrics
	metricsMu.Unlock()
	return metrics, nil
}

// UnregisterMetrics clears the default metrics. Use in tests to reset state.
func UnregisterMetrics() {
	metricsMu.Lock()
	defaultMetrics = nil
	metricsMu.Unlock()
}

// RecordToolAvailability records a tool availability state transition.
// No-op when metrics are not registered.
func RecordToolAvailability(ctx context.Context, toolID mcp.ToolID, available bool) {
	metricsMu.RLock()
	m := defaultMetrics
	metricsMu.RUnlock()
	if m == nil {
		return
	}
	m.toolAvailability.Add(ctx, 1, metric.WithAttributes(
		attribute.String("tool_id", string(toolID)),
		attribute.Bool("available", available),
	))
}

// RecordInvocationLatency records the latency of a successful tool invocation.
// No-op when metrics are not registered.
func RecordInvocationLatency(ctx context.Context, toolID mcp.ToolID, tenantID mcp.TenantID, latencyMs float64) {
	metricsMu.RLock()
	m := defaultMetrics
	metricsMu.RUnlock()
	if m == nil {
		return
	}
	m.invocationLatency.Record(ctx, latencyMs, metric.WithAttributes(
		attribute.String("tool_id", string(toolID)),
		attribute.String("tenant_id", string(tenantID)),
	))
}

// RecordInvocationFailure records a failed tool invocation.
// No-op when metrics are not registered.
func RecordInvocationFailure(ctx context.Context, toolID mcp.ToolID, tenantID mcp.TenantID, reason string) {
	metricsMu.RLock()
	m := defaultMetrics
	metricsMu.RUnlock()
	if m == nil {
		return
	}
	m.invocationFailure.Add(ctx, 1, metric.WithAttributes(
		attribute.String("tool_id", string(toolID)),
		attribute.String("tenant_id", string(tenantID)),
		attribute.String("reason", reason),
	))
}

// RecordCircuitState records a circuit breaker state transition.
// No-op when metrics are not registered.
func RecordCircuitState(ctx context.Context, toolID mcp.ToolID, state string) {
	metricsMu.RLock()
	m := defaultMetrics
	metricsMu.RUnlock()
	if m == nil {
		return
	}
	m.circuitState.Add(ctx, 1, metric.WithAttributes(
		attribute.String("tool_id", string(toolID)),
		attribute.String("state", state),
	))
}
