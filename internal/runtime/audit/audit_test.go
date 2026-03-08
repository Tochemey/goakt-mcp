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

package audit

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		et       EventType
		expected string
	}{
		{"PolicyDecision", EventTypePolicyDecision, "policy_decision"},
		{"InvocationStart", EventTypeInvocationStart, "invocation_start"},
		{"InvocationComplete", EventTypeInvocationComplete, "invocation_complete"},
		{"InvocationFailed", EventTypeInvocationFailed, "invocation_failed"},
		{"HealthTransition", EventTypeHealthTransition, "health_transition"},
		{"CircuitStateChange", EventTypeCircuitStateChange, "circuit_state_change"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, string(tc.et))
		})
	}
}

func TestEventConstruction(t *testing.T) {
	now := time.Now()
	event := &Event{
		Type:      EventTypeInvocationComplete,
		Timestamp: now,
		TenantID:  "tenant-1",
		ClientID:  "client-1",
		ToolID:    "tool-1",
		RequestID: "req-1",
		TraceID:   "trace-1",
		Outcome:   "success",
		ErrorCode: "",
		Message:   "completed",
		Metadata:  map[string]string{"key": "value"},
	}

	assert.Equal(t, EventTypeInvocationComplete, event.Type)
	assert.Equal(t, now, event.Timestamp)
	assert.Equal(t, "tenant-1", event.TenantID)
	assert.Equal(t, "client-1", event.ClientID)
	assert.Equal(t, "tool-1", event.ToolID)
	assert.Equal(t, "req-1", event.RequestID)
	assert.Equal(t, "trace-1", event.TraceID)
	assert.Equal(t, "success", event.Outcome)
	assert.Empty(t, event.ErrorCode)
	assert.Equal(t, "completed", event.Message)
	assert.Equal(t, "value", event.Metadata["key"])
}

func TestEventZeroValue(t *testing.T) {
	var event Event
	assert.Equal(t, EventType(""), event.Type)
	assert.True(t, event.Timestamp.IsZero())
	assert.Empty(t, event.TenantID)
	assert.Empty(t, event.ToolID)
	assert.Nil(t, event.Metadata)
}

func TestMemorySinkImplementsSink(t *testing.T) {
	var _ Sink = (*MemorySink)(nil)
}

func TestMemorySinkNewEmpty(t *testing.T) {
	sink := NewMemorySink()
	require.NotNil(t, sink)
	assert.Empty(t, sink.Events())
}

func TestMemorySinkWriteAndRetrieve(t *testing.T) {
	sink := NewMemorySink()

	event := &Event{
		Type:      EventTypePolicyDecision,
		Timestamp: time.Now(),
		TenantID:  "t1",
		ToolID:    "tool-a",
		Outcome:   "allow",
	}

	err := sink.Write(event)
	require.NoError(t, err)

	events := sink.Events()
	require.Len(t, events, 1)
	assert.Equal(t, EventTypePolicyDecision, events[0].Type)
	assert.Equal(t, "t1", events[0].TenantID)
	assert.Equal(t, "tool-a", events[0].ToolID)
	assert.Equal(t, "allow", events[0].Outcome)
}

func TestMemorySinkWriteMultiple(t *testing.T) {
	sink := NewMemorySink()

	for i := 0; i < 5; i++ {
		err := sink.Write(&Event{
			Type:    EventTypeInvocationComplete,
			Outcome: "success",
		})
		require.NoError(t, err)
	}

	assert.Len(t, sink.Events(), 5)
}

func TestMemorySinkDefensiveCopyOnWrite(t *testing.T) {
	sink := NewMemorySink()

	meta := map[string]string{"key": "original"}
	event := &Event{
		Type:     EventTypeInvocationFailed,
		Metadata: meta,
	}

	err := sink.Write(event)
	require.NoError(t, err)

	meta["key"] = "mutated"

	events := sink.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "original", events[0].Metadata["key"])
}

func TestMemorySinkDefensiveCopyOnRead(t *testing.T) {
	sink := NewMemorySink()

	err := sink.Write(&Event{
		Type:     EventTypeHealthTransition,
		Metadata: map[string]string{"state": "open"},
	})
	require.NoError(t, err)

	events := sink.Events()
	require.Len(t, events, 1)
	events[0].Metadata["state"] = "mutated"

	fresh := sink.Events()
	assert.Equal(t, "open", fresh[0].Metadata["state"])
}

func TestMemorySinkWriteNilMetadata(t *testing.T) {
	sink := NewMemorySink()

	err := sink.Write(&Event{
		Type:   EventTypeInvocationStart,
		ToolID: "tool-x",
	})
	require.NoError(t, err)

	events := sink.Events()
	require.Len(t, events, 1)
	assert.Nil(t, events[0].Metadata)
}

func TestMemorySinkCloseStopsWrites(t *testing.T) {
	sink := NewMemorySink()

	err := sink.Write(&Event{Type: EventTypeInvocationComplete})
	require.NoError(t, err)

	err = sink.Close()
	require.NoError(t, err)

	err = sink.Write(&Event{Type: EventTypeInvocationFailed})
	require.NoError(t, err)

	assert.Len(t, sink.Events(), 1)
}

func TestMemorySinkCloseIdempotent(t *testing.T) {
	sink := NewMemorySink()
	require.NoError(t, sink.Close())
	require.NoError(t, sink.Close())
}

func TestMemorySinkConcurrentWrites(t *testing.T) {
	sink := NewMemorySink()
	const writers = 10
	const eventsPerWriter = 50

	var wg sync.WaitGroup
	wg.Add(writers)

	for w := 0; w < writers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < eventsPerWriter; i++ {
				_ = sink.Write(&Event{
					Type:    EventTypeInvocationComplete,
					Outcome: "success",
				})
			}
		}()
	}

	wg.Wait()
	assert.Len(t, sink.Events(), writers*eventsPerWriter)
}

func TestMemorySinkConcurrentWriteAndClose(t *testing.T) {
	sink := NewMemorySink()
	const writers = 5

	var wg sync.WaitGroup
	wg.Add(writers + 1)

	for w := 0; w < writers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				_ = sink.Write(&Event{Type: EventTypeInvocationComplete})
			}
		}()
	}

	go func() {
		defer wg.Done()
		time.Sleep(time.Millisecond)
		_ = sink.Close()
	}()

	wg.Wait()

	events := sink.Events()
	assert.LessOrEqual(t, len(events), writers*100)
}
