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
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goaktlog "github.com/tochemey/goakt/v4/log"
)

// spyLogger records the last method called, the message, and any structured
// fields passed to it. It implements Logger and is used to verify that
// loggerAdapter routes each GoAkt method to the correct inner method with the
// correct message and fields.
type spyLogger struct {
	lastMethod string
	lastMsg    string
	lastFields []any
}

func (s *spyLogger) Debug(msg string, args ...any) {
	s.lastMethod = "debug"
	s.lastMsg = msg
	s.lastFields = args
}
func (s *spyLogger) Info(msg string, args ...any) {
	s.lastMethod = "info"
	s.lastMsg = msg
	s.lastFields = args
}
func (s *spyLogger) Warn(msg string, args ...any) {
	s.lastMethod = "warn"
	s.lastMsg = msg
	s.lastFields = args
}
func (s *spyLogger) Error(msg string, args ...any) {
	s.lastMethod = "error"
	s.lastMsg = msg
	s.lastFields = args
}

func newAdapter(spy *spyLogger) *loggerAdapter {
	return &loggerAdapter{inner: spy}
}

// -----------------------------------------------------------------------------
// Debug family
// -----------------------------------------------------------------------------

func TestLoggerAdapterDebug(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.Debug("debug msg")
	assert.Equal(t, "debug", spy.lastMethod)
	assert.Equal(t, "debug msg", spy.lastMsg)
}

func TestLoggerAdapterDebugf(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.Debugf("hello %s", "world")
	assert.Equal(t, "debug", spy.lastMethod)
	assert.Equal(t, "hello world", spy.lastMsg)
}

func TestLoggerAdapterDebugContext(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.DebugContext(context.Background(), "ctx debug")
	assert.Equal(t, "debug", spy.lastMethod)
	assert.Equal(t, "ctx debug", spy.lastMsg)
}

func TestLoggerAdapterDebugfContext(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.DebugfContext(context.Background(), "ctx %s", "debugf")
	assert.Equal(t, "debug", spy.lastMethod)
	assert.Equal(t, "ctx debugf", spy.lastMsg)
}

// -----------------------------------------------------------------------------
// Info family
// -----------------------------------------------------------------------------

func TestLoggerAdapterInfo(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.Info("info msg")
	assert.Equal(t, "info", spy.lastMethod)
	assert.Equal(t, "info msg", spy.lastMsg)
}

func TestLoggerAdapterInfof(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.Infof("hello %s", "world")
	assert.Equal(t, "info", spy.lastMethod)
	assert.Equal(t, "hello world", spy.lastMsg)
}

func TestLoggerAdapterInfoContext(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.InfoContext(context.Background(), "ctx info")
	assert.Equal(t, "info", spy.lastMethod)
	assert.Equal(t, "ctx info", spy.lastMsg)
}

func TestLoggerAdapterInfofContext(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.InfofContext(context.Background(), "ctx %s", "infof")
	assert.Equal(t, "info", spy.lastMethod)
	assert.Equal(t, "ctx infof", spy.lastMsg)
}

// -----------------------------------------------------------------------------
// Warn family
// -----------------------------------------------------------------------------

func TestLoggerAdapterWarn(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.Warn("warn msg")
	assert.Equal(t, "warn", spy.lastMethod)
	assert.Equal(t, "warn msg", spy.lastMsg)
}

func TestLoggerAdapterWarnf(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.Warnf("hello %s", "world")
	assert.Equal(t, "warn", spy.lastMethod)
	assert.Equal(t, "hello world", spy.lastMsg)
}

func TestLoggerAdapterWarnContext(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.WarnContext(context.Background(), "ctx warn")
	assert.Equal(t, "warn", spy.lastMethod)
	assert.Equal(t, "ctx warn", spy.lastMsg)
}

func TestLoggerAdapterWarnfContext(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.WarnfContext(context.Background(), "ctx %s", "warnf")
	assert.Equal(t, "warn", spy.lastMethod)
	assert.Equal(t, "ctx warnf", spy.lastMsg)
}

// -----------------------------------------------------------------------------
// Error family
// -----------------------------------------------------------------------------

func TestLoggerAdapterError(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.Error("error msg")
	assert.Equal(t, "error", spy.lastMethod)
	assert.Equal(t, "error msg", spy.lastMsg)
}

func TestLoggerAdapterErrorf(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.Errorf("hello %s", "world")
	assert.Equal(t, "error", spy.lastMethod)
	assert.Equal(t, "hello world", spy.lastMsg)
}

func TestLoggerAdapterErrorContext(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.ErrorContext(context.Background(), "ctx error")
	assert.Equal(t, "error", spy.lastMethod)
	assert.Equal(t, "ctx error", spy.lastMsg)
}

func TestLoggerAdapterErrorfContext(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	a.ErrorfContext(context.Background(), "ctx %s", "errorf")
	assert.Equal(t, "error", spy.lastMethod)
	assert.Equal(t, "ctx errorf", spy.lastMsg)
}

// -----------------------------------------------------------------------------
// Panic family
// -----------------------------------------------------------------------------

func TestLoggerAdapterPanic(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	assert.Panics(t, func() {
		a.Panic("panic msg")
	})
	assert.Equal(t, "error", spy.lastMethod)
	assert.Equal(t, "panic msg", spy.lastMsg)
}

func TestLoggerAdapterPanicf(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	assert.Panics(t, func() {
		a.Panicf("panic %s", "formatted")
	})
	assert.Equal(t, "error", spy.lastMethod)
	assert.Equal(t, "panic formatted", spy.lastMsg)
}

// -----------------------------------------------------------------------------
// Fatal family — subprocess tests (os.Exit(1) cannot be tested in-process)
// -----------------------------------------------------------------------------

func TestLoggerAdapterFatal(t *testing.T) {
	if os.Getenv("GO_TEST_ADAPTER_FATAL") == "1" {
		a := newAdapter(&spyLogger{})
		a.Fatal("fatal msg")
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestLoggerAdapterFatalHelperProcess$", "-test.v") // #nosec G204 G702
	cmd.Env = append(os.Environ(), "GO_TEST_ADAPTER_FATAL=1")
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode())
	assert.NotEmpty(t, out)
}

func TestLoggerAdapterFatalHelperProcess(t *testing.T) {
	if os.Getenv("GO_TEST_ADAPTER_FATAL") != "1" {
		t.Skip("helper process")
	}
	a := newAdapter(&spyLogger{})
	a.Fatal("fatal msg")
}

func TestLoggerAdapterFatalf(t *testing.T) {
	if os.Getenv("GO_TEST_ADAPTER_FATALF") == "1" {
		a := newAdapter(&spyLogger{})
		a.Fatalf("fatal %s", "formatted")
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestLoggerAdapterFatalfHelperProcess$", "-test.v") // #nosec G204 G702
	cmd.Env = append(os.Environ(), "GO_TEST_ADAPTER_FATALF=1")
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode())
	assert.NotEmpty(t, out)
}

func TestLoggerAdapterFatalfHelperProcess(t *testing.T) {
	if os.Getenv("GO_TEST_ADAPTER_FATALF") != "1" {
		t.Skip("helper process")
	}
	a := newAdapter(&spyLogger{})
	a.Fatalf("fatal %s", "formatted")
}

// -----------------------------------------------------------------------------
// Utility methods
// -----------------------------------------------------------------------------

func TestLoggerAdapterLogLevel(t *testing.T) {
	a := newAdapter(&spyLogger{})
	assert.Equal(t, goaktlog.DebugLevel, a.LogLevel())
}

func TestLoggerAdapterEnabled(t *testing.T) {
	a := newAdapter(&spyLogger{})
	assert.True(t, a.Enabled(goaktlog.DebugLevel))
	assert.True(t, a.Enabled(goaktlog.InfoLevel))
	assert.True(t, a.Enabled(goaktlog.WarningLevel))
	assert.True(t, a.Enabled(goaktlog.ErrorLevel))
	assert.True(t, a.Enabled(goaktlog.FatalLevel))
	assert.True(t, a.Enabled(goaktlog.PanicLevel))
}

func TestLoggerAdapterWith(t *testing.T) {
	a := newAdapter(&spyLogger{})
	result := a.With("key", "value")
	assert.Equal(t, a, result, "With should return the same adapter")
}

func TestLoggerAdapterLogOutput(t *testing.T) {
	a := newAdapter(&spyLogger{})
	assert.Nil(t, a.LogOutput())
}

func TestLoggerAdapterFlush(t *testing.T) {
	a := newAdapter(&spyLogger{})
	assert.NoError(t, a.Flush())
}

func TestLoggerAdapterStdLogger(t *testing.T) {
	spy := &spyLogger{}
	a := newAdapter(spy)
	std := a.StdLogger()
	require.NotNil(t, std)
	std.Print("std message")
	assert.Equal(t, "info", spy.lastMethod)
	assert.Contains(t, spy.lastMsg, "std message")
}

// -----------------------------------------------------------------------------
// goaktArgsToMsg helper
// -----------------------------------------------------------------------------

func TestGoaktArgsToMsg(t *testing.T) {
	t.Run("empty args returns empty message and nil fields", func(t *testing.T) {
		msg, fields := goaktArgsToMsg(nil)
		assert.Equal(t, "", msg)
		assert.Nil(t, fields)
	})

	t.Run("single arg returns it as message with nil fields", func(t *testing.T) {
		msg, fields := goaktArgsToMsg([]any{"hello"})
		assert.Equal(t, "hello", msg)
		assert.Nil(t, fields)
	})

	t.Run("multiple args splits message from key-value fields", func(t *testing.T) {
		msg, fields := goaktArgsToMsg([]any{"hello", "key", "value"})
		assert.Equal(t, "hello", msg)
		assert.Equal(t, []any{"key", "value"}, fields)
	})
}

// -----------------------------------------------------------------------------
// Multi-arg routing — verifies fields are passed through, not swallowed
// -----------------------------------------------------------------------------

func TestLoggerAdapterMultiArgRouting(t *testing.T) {
	t.Run("Debug passes fields to inner logger", func(t *testing.T) {
		spy := &spyLogger{}
		a := newAdapter(spy)
		a.Debug("msg", "k", "v")
		assert.Equal(t, "msg", spy.lastMsg)
		assert.Equal(t, []any{"k", "v"}, spy.lastFields)
	})

	t.Run("Info passes fields to inner logger", func(t *testing.T) {
		spy := &spyLogger{}
		a := newAdapter(spy)
		a.Info("msg", "k", "v")
		assert.Equal(t, "msg", spy.lastMsg)
		assert.Equal(t, []any{"k", "v"}, spy.lastFields)
	})

	t.Run("Warn passes fields to inner logger", func(t *testing.T) {
		spy := &spyLogger{}
		a := newAdapter(spy)
		a.Warn("msg", "k", "v")
		assert.Equal(t, "msg", spy.lastMsg)
		assert.Equal(t, []any{"k", "v"}, spy.lastFields)
	})

	t.Run("Error passes fields to inner logger", func(t *testing.T) {
		spy := &spyLogger{}
		a := newAdapter(spy)
		a.Error("msg", "k", "v")
		assert.Equal(t, "msg", spy.lastMsg)
		assert.Equal(t, []any{"k", "v"}, spy.lastFields)
	})

	t.Run("Panic passes fields to inner logger before panicking", func(t *testing.T) {
		spy := &spyLogger{}
		a := newAdapter(spy)
		assert.Panics(t, func() { a.Panic("msg", "k", "v") })
		assert.Equal(t, "msg", spy.lastMsg)
		assert.Equal(t, []any{"k", "v"}, spy.lastFields)
	})

	t.Run("empty args produces empty message", func(t *testing.T) {
		spy := &spyLogger{}
		a := newAdapter(spy)
		a.Info()
		assert.Equal(t, "", spy.lastMsg)
		assert.Nil(t, spy.lastFields)
	})
}
