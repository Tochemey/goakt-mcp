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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tochemey/goakt/v4/remote"
)

func TestTracerDefaultIsNoop(t *testing.T) {
	UnregisterTracer()
	tracer := Tracer()
	require.NotNil(t, tracer)
	assert.False(t, TracingEnabled())
}

func TestRegisterTracer(t *testing.T) {
	t.Cleanup(UnregisterTracer)
	tracer := RegisterTracer()
	require.NotNil(t, tracer)
	assert.True(t, TracingEnabled())
	assert.Equal(t, tracer, Tracer())
}

func TestUnregisterTracer(t *testing.T) {
	RegisterTracer()
	assert.True(t, TracingEnabled())
	UnregisterTracer()
	assert.False(t, TracingEnabled())
}

func TestOTelContextPropagator_ImplementsInterface(t *testing.T) {
	var _ remote.ContextPropagator = (*OTelContextPropagator)(nil)
}

func TestOTelContextPropagator_InjectExtract(t *testing.T) {
	prop := NewOTelContextPropagator()
	ctx := context.Background()
	headers := make(http.Header)

	err := prop.Inject(ctx, headers)
	require.NoError(t, err)

	got, err := prop.Extract(ctx, headers)
	require.NoError(t, err)
	require.NotNil(t, got)
}
