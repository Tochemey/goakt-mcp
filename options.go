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
