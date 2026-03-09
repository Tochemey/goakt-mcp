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
	"net/http"
	"sync/atomic"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

var (
	tracerPtr  atomic.Pointer[trace.Tracer]
	noopTracer = noop.NewTracerProvider().Tracer(instrumentationName)
)

// RegisterTracer creates an OpenTelemetry Tracer from the global
// TracerProvider and stores it as the process-wide default. Call this at
// gateway Start when WithTracing() is enabled. The TracerProvider and
// exporter must be configured before Start, similar to GoAkt's WithTracing().
func RegisterTracer() trace.Tracer {
	tracer := otel.GetTracerProvider().Tracer(instrumentationName)
	tracerPtr.Store(&tracer)
	return tracer
}

// UnregisterTracer clears the default tracer. Use in tests to reset state.
func UnregisterTracer() {
	tracerPtr.Store(nil)
}

// Tracer returns the registered tracer, or a package-level no-op tracer
// when tracing is not enabled. Lock-free via atomic load.
func Tracer() trace.Tracer {
	if p := tracerPtr.Load(); p != nil {
		return *p
	}
	return noopTracer
}

// TracingEnabled reports whether a tracer has been registered. Lock-free.
func TracingEnabled() bool {
	return tracerPtr.Load() != nil
}

// OTelContextPropagator adapts the OpenTelemetry propagation.TextMapPropagator
// to GoAkt's remote.ContextPropagator interface. This allows OTel trace context
// (W3C traceparent/tracestate) to travel across GoAkt remote actor calls.
type OTelContextPropagator struct {
	propagator propagation.TextMapPropagator
}

// NewOTelContextPropagator creates a propagator that delegates to the global
// OTel TextMapPropagator (otel.GetTextMapPropagator()).
func NewOTelContextPropagator() *OTelContextPropagator {
	return &OTelContextPropagator{
		propagator: otel.GetTextMapPropagator(),
	}
}

// Inject writes OTel trace context from ctx into the outgoing http.Header.
func (x *OTelContextPropagator) Inject(ctx context.Context, headers http.Header) error {
	x.propagator.Inject(ctx, propagation.HeaderCarrier(headers))
	return nil
}

// Extract reads OTel trace context from the incoming http.Header and returns
// a derived context carrying the propagated span context.
func (x *OTelContextPropagator) Extract(ctx context.Context, headers http.Header) (context.Context, error) {
	return x.propagator.Extract(ctx, propagation.HeaderCarrier(headers)), nil
}
