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
	"github.com/tochemey/goakt/v4/testkit"

	"github.com/tochemey/goakt-mcp/mcp"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	"github.com/tochemey/goakt-mcp/internal/runtime/actor/extension"
	"github.com/tochemey/goakt-mcp/internal/runtime/audit"
)

func TestJournalActor(t *testing.T) {
	ctx := context.Background()

	t.Run("starts and stops cleanly", func(t *testing.T) {
		cfg := testConfig()
		cfg.Audit.Sink = audit.NewMemorySink()
		system, stop := testActorSystem(t, goaktactor.WithExtensions(extension.NewConfigExtension(cfg)))
		defer stop()

		pid, err := system.Spawn(ctx, mcp.ActorNameJournal, newJournaler())
		require.NoError(t, err)
		require.NotNil(t, pid)
		assert.Equal(t, mcp.ActorNameJournal, pid.Name())

		waitForActors()
	})

	t.Run("records audit events", func(t *testing.T) {
		sink := audit.NewMemorySink()
		cfg := testConfig()
		cfg.Audit.Sink = sink
		system, stop := testActorSystem(t, goaktactor.WithExtensions(extension.NewConfigExtension(cfg)))
		defer stop()

		pid, err := system.Spawn(ctx, mcp.ActorNameJournal, newJournaler())
		require.NoError(t, err)
		waitForActors()

		ev := &mcp.AuditEvent{
			Type:     mcp.AuditEventTypeInvocationComplete,
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
		assert.Equal(t, mcp.AuditEventTypeInvocationComplete, events[0].Type)
		assert.Equal(t, "tenant-1", events[0].TenantID)
		assert.Equal(t, "success", events[0].Outcome)
	})

	t.Run("ignores nil event", func(t *testing.T) {
		sink := audit.NewMemorySink()
		cfg := testConfig()
		cfg.Audit.Sink = sink
		kit, ctx := newTestKit(t, testkit.WithExtensions(extension.NewConfigExtension(cfg)))

		kit.Spawn(ctx, "journal-nil-event", newJournaler())
		waitForActors()

		probe := kit.NewProbe(ctx)
		probe.Send("journal-nil-event", &runtime.RecordAuditEvent{Event: nil})
		probe.ExpectNoMessage()
		probe.Stop()
	})

	t.Run("handles write failure gracefully", func(t *testing.T) {
		failingSink := &failingAuditSink{}
		cfg := testConfig()
		cfg.Audit.Sink = failingSink
		kit, ctx := newTestKit(t, testkit.WithExtensions(extension.NewConfigExtension(cfg)))

		kit.Spawn(ctx, "journal-fail", newJournaler())
		waitForActors()

		probe := kit.NewProbe(ctx)
		probe.Send("journal-fail", &runtime.RecordAuditEvent{
			Event: &mcp.AuditEvent{Type: mcp.AuditEventTypeInvocationComplete, TenantID: "t1"},
		})
		probe.ExpectNoMessage()
		probe.Stop()
	})

	t.Run("unhandles unknown message", func(t *testing.T) {
		sink := audit.NewMemorySink()
		cfg := testConfig()
		cfg.Audit.Sink = sink
		kit, ctx := newTestKit(t, testkit.WithExtensions(extension.NewConfigExtension(cfg)))

		pid, err := kit.ActorSystem().Spawn(ctx, "journal-unknown", newJournaler())
		require.NoError(t, err)
		require.NoError(t, pid.Tell(ctx, pid, "unknown"))
		waitForActors()
	})
}
