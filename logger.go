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
	"context"
	"fmt"
	"io"
	golog "log"
	"os"

	goaktlog "github.com/tochemey/goakt/v4/log"
)

// Logger is the logging interface that developers can implement to plug in
// their own logging backend (e.g., zap, zerolog, slog, logrus).
//
// Methods follow the slog convention: msg is the log message and args are
// optional key-value pairs (alternating string keys and arbitrary values)
// for structured logging. Implementations that do not support structured
// fields may ignore args.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// loggerAdapter wraps a Logger and satisfies the goaktlog.Logger interface
// used internally by the underlying engine.
type loggerAdapter struct {
	inner Logger
}

// compile-time check
var _ goaktlog.Logger = (*loggerAdapter)(nil)

// goaktArgsToMsg splits GoAkt variadic log arguments into a message string
// and optional key-value fields. GoAkt follows the slog convention where the
// first argument is the log message and subsequent arguments are alternating
// key-value pairs for structured logging.
func goaktArgsToMsg(args []any) (string, []any) {
	if len(args) == 0 {
		return "", nil
	}
	if len(args) == 1 {
		return fmt.Sprint(args[0]), nil
	}
	return fmt.Sprint(args[0]), args[1:]
}

func (a *loggerAdapter) Debug(args ...any) {
	msg, fields := goaktArgsToMsg(args)
	a.inner.Debug(msg, fields...)
}
func (a *loggerAdapter) Debugf(format string, args ...any) {
	a.inner.Debug(fmt.Sprintf(format, args...))
}
func (a *loggerAdapter) DebugContext(_ context.Context, args ...any) {
	msg, fields := goaktArgsToMsg(args)
	a.inner.Debug(msg, fields...)
}
func (a *loggerAdapter) DebugfContext(_ context.Context, format string, args ...any) {
	a.inner.Debug(fmt.Sprintf(format, args...))
}
func (a *loggerAdapter) Info(args ...any) {
	msg, fields := goaktArgsToMsg(args)
	a.inner.Info(msg, fields...)
}
func (a *loggerAdapter) Infof(format string, args ...any) {
	a.inner.Info(fmt.Sprintf(format, args...))
}
func (a *loggerAdapter) InfoContext(_ context.Context, args ...any) {
	msg, fields := goaktArgsToMsg(args)
	a.inner.Info(msg, fields...)
}
func (a *loggerAdapter) InfofContext(_ context.Context, format string, args ...any) {
	a.inner.Info(fmt.Sprintf(format, args...))
}
func (a *loggerAdapter) Warn(args ...any) {
	msg, fields := goaktArgsToMsg(args)
	a.inner.Warn(msg, fields...)
}
func (a *loggerAdapter) Warnf(format string, args ...any) {
	a.inner.Warn(fmt.Sprintf(format, args...))
}
func (a *loggerAdapter) WarnContext(_ context.Context, args ...any) {
	msg, fields := goaktArgsToMsg(args)
	a.inner.Warn(msg, fields...)
}
func (a *loggerAdapter) WarnfContext(_ context.Context, format string, args ...any) {
	a.inner.Warn(fmt.Sprintf(format, args...))
}
func (a *loggerAdapter) Error(args ...any) {
	msg, fields := goaktArgsToMsg(args)
	a.inner.Error(msg, fields...)
}
func (a *loggerAdapter) Errorf(format string, args ...any) {
	a.inner.Error(fmt.Sprintf(format, args...))
}
func (a *loggerAdapter) ErrorContext(_ context.Context, args ...any) {
	msg, fields := goaktArgsToMsg(args)
	a.inner.Error(msg, fields...)
}
func (a *loggerAdapter) ErrorfContext(_ context.Context, format string, args ...any) {
	a.inner.Error(fmt.Sprintf(format, args...))
}

func (a *loggerAdapter) Fatal(args ...any) {
	msg, fields := goaktArgsToMsg(args)
	a.inner.Error(msg, fields...)
	os.Exit(1)
}

func (a *loggerAdapter) Fatalf(format string, args ...any) {
	a.inner.Error(fmt.Sprintf(format, args...))
	os.Exit(1)
}

func (a *loggerAdapter) Panic(args ...any) {
	msg, fields := goaktArgsToMsg(args)
	a.inner.Error(msg, fields...)
	panic(msg)
}

func (a *loggerAdapter) Panicf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	a.inner.Error(msg)
	panic(msg)
}

func (a *loggerAdapter) LogLevel() goaktlog.Level      { return goaktlog.DebugLevel }
func (a *loggerAdapter) Enabled(_ goaktlog.Level) bool { return true }
func (a *loggerAdapter) With(_ ...any) goaktlog.Logger { return a }
func (a *loggerAdapter) LogOutput() []io.Writer        { return nil }
func (a *loggerAdapter) Flush() error                  { return nil }

func (a *loggerAdapter) StdLogger() *golog.Logger {
	return golog.New(&loggerWriter{inner: a.inner}, "", 0)
}

// loggerWriter adapts Logger.Info to io.Writer for use with *log.Logger.
type loggerWriter struct {
	inner Logger
}

func (w *loggerWriter) Write(p []byte) (int, error) {
	w.inner.Info(string(p))
	return len(p), nil
}
