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

package actor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goaktactor "github.com/tochemey/goakt/v4/actor"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	"github.com/tochemey/goakt-mcp/internal/runtime/audit"
	"github.com/tochemey/goakt-mcp/mcp"
)

func TestJournalActor(t *testing.T) {
	ctx := context.Background()

	t.Run("starts and stops cleanly", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		sink := audit.NewMemorySink()
		pid, err := system.Spawn(ctx, mcp.ActorNameJournal, newJournaler(sink))
		require.NoError(t, err)
		require.NotNil(t, pid)
		assert.Equal(t, mcp.ActorNameJournal, pid.Name())

		waitForActors()
	})

	t.Run("records audit events", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		sink := audit.NewMemorySink()
		pid, err := system.Spawn(ctx, mcp.ActorNameJournal, newJournaler(sink))
		require.NoError(t, err)
		waitForActors()

		ev := &audit.Event{
			Type:     audit.EventTypeInvocationComplete,
			TenantID: "tenant-1",
			ClientID: "client-1",
			ToolID:   "tool-1",
			Outcome:  "success",
		}
		err = goaktactor.Tell(ctx, pid, &runtime.RecordAuditEvent{Event: ev})
		require.NoError(t, err)
		waitForActors()

		events := sink.Events()
		require.Len(t, events, 1)
		assert.Equal(t, audit.EventTypeInvocationComplete, events[0].Type)
		assert.Equal(t, "tenant-1", events[0].TenantID)
		assert.Equal(t, "success", events[0].Outcome)
	})

	t.Run("ignores nil event", func(t *testing.T) {
		kit, ctx := newTestKit(t)

		sink := audit.NewMemorySink()
		kit.Spawn(ctx, "journal-nil-event", newJournaler(sink))
		waitForActors()

		probe := kit.NewProbe(ctx)
		probe.Send("journal-nil-event", &runtime.RecordAuditEvent{Event: nil})
		probe.ExpectNoMessage()
		probe.Stop()
	})

	t.Run("ignores when sink is nil", func(t *testing.T) {
		kit, ctx := newTestKit(t)

		kit.Spawn(ctx, "journal-nil-sink", newJournaler(nil))
		waitForActors()

		probe := kit.NewProbe(ctx)
		probe.Send("journal-nil-sink", &runtime.RecordAuditEvent{
			Event: &audit.Event{Type: audit.EventTypeInvocationComplete},
		})
		probe.ExpectNoMessage()
		probe.Stop()
	})

	t.Run("handles write failure gracefully", func(t *testing.T) {
		kit, ctx := newTestKit(t)

		failingSink := &failingAuditSink{}
		kit.Spawn(ctx, "journal-fail", newJournaler(failingSink))
		waitForActors()

		probe := kit.NewProbe(ctx)
		probe.Send("journal-fail", &runtime.RecordAuditEvent{
			Event: &audit.Event{Type: audit.EventTypeInvocationComplete, TenantID: "t1"},
		})
		probe.ExpectNoMessage()
		probe.Stop()
	})

	t.Run("unhandles unknown message", func(t *testing.T) {
		kit, ctx := newTestKit(t)

		sink := audit.NewMemorySink()
		pid, err := kit.ActorSystem().Spawn(ctx, "journal-unknown", newJournaler(sink))
		require.NoError(t, err)
		require.NoError(t, pid.Tell(ctx, pid, "unknown"))
		waitForActors()
	})
}
