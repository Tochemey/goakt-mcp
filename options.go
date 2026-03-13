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

package goaktmcp

import (
	"os"

	goaktlog "github.com/tochemey/goakt/v4/log"
)

// LogLevel wraps goaktlog.Level for gateway logging configuration.
// Use with WithLogger to configure the gateway's log level.
type LogLevel = goaktlog.Level

// Option configures a Gateway.
type Option func(*Gateway)

// WithLogger sets the log level for the gateway. When set, the gateway uses
// GoAkt's slog-based logger at the specified level. Pass goaktlog.InvalidLevel
// to suppress all output (e.g. in tests).
func WithLogger(level LogLevel) Option {
	return func(g *Gateway) {
		if level == goaktlog.InvalidLevel {
			g.logger = goaktlog.DiscardLogger
		} else {
			g.logger = goaktlog.NewSlog(level, os.Stdout)
		}
	}
}

// WithMetrics enables OpenTelemetry metrics for the gateway. When set, the
// gateway registers goakt-mcp instruments (tool availability, invocation
// latency, failures, circuit state) at Start. Metrics are exported only when
// the OpenTelemetry SDK is initialized and a MeterProvider/exporter is
// configured before Start, similar to GoAkt's WithMetrics().
func WithMetrics() Option {
	return func(g *Gateway) {
		g.metrics = true
	}
}

// WithTracing enables OpenTelemetry tracing for the gateway. When set, the
// gateway registers a Tracer at Start and instruments egress HTTP calls with
// otelhttp for automatic W3C trace-context propagation and span creation.
// Traces are exported only when the OpenTelemetry SDK is initialized and a
// TracerProvider/exporter is configured before Start, similar to GoAkt's
// WithTracing().
func WithTracing() Option {
	return func(g *Gateway) {
		g.tracing = true
	}
}
