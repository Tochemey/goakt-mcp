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
	goaktactor "github.com/tochemey/goakt/v4/actor"
	goaktlog "github.com/tochemey/goakt/v4/log"

	"github.com/tochemey/goakt-mcp/mcp"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	"github.com/tochemey/goakt-mcp/internal/runtime/audit"
)

// journaler is the Journal Actor.
//
// Journaler receives asynchronous audit events off the critical request path
// and writes them to a durable audit sink. It ensures that request outcomes,
// policy decisions, and lifecycle transitions are recorded without blocking
// the actors that produce those events.
//
// Spawn: GatewayManager spawns Journaler in spawnFoundationalActors via
// ctx.Spawn(ActorNameJournal, newJournalActor(auditSink)) as a child of GatewayManager.
// The audit sink is created from config (createAuditSink).
//
// Relocation: No. Journaler runs on the local node as a child of GatewayManager
// and does not relocate in cluster mode.
//
// All fields are unexported to enforce actor immutability rules.
type journaler struct {
	sink   audit.Sink
	logger goaktlog.Logger
}

// enforce that journaler satisfies the GoAkt Actor interface at compile time.
var _ goaktactor.Actor = (*journaler)(nil)

// newJournaler creates a new Journaler instance with the given sink.
// Pass nil for sink to create a no-op journal (events are discarded).
func newJournaler(sink audit.Sink) *journaler {
	return &journaler{sink: sink}
}

// PreStart initializes Journaler before message processing begins.
func (x *journaler) PreStart(ctx *goaktactor.Context) error {
	x.logger = ctx.Logger()
	x.logger.Infof("actor=%s starting", mcp.ActorNameJournal)
	return nil
}

// Receive handles messages delivered to Journaler.
func (x *journaler) Receive(ctx *goaktactor.ReceiveContext) {
	switch msg := ctx.Message().(type) {
	case *goaktactor.PostStart:
		x.logger.Infof("actor=%s started", mcp.ActorNameJournal)
	case *runtime.RecordAuditEvent:
		x.handleRecordAuditEvent(msg)
	default:
		ctx.Unhandled()
	}
}

// PostStop performs cleanup after Journaler has stopped.
func (x *journaler) PostStop(ctx *goaktactor.Context) error {
	if x.sink != nil {
		_ = x.sink.Close()
	}
	x.logger.Infof("actor=%s stopped", mcp.ActorNameJournal)
	return nil
}

// handleRecordAuditEvent writes the audit event to the configured sink. Nil
// events and nil sinks are silently ignored. Write failures are logged but do
// not affect the calling actor (journal is off the critical path).
func (x *journaler) handleRecordAuditEvent(msg *runtime.RecordAuditEvent) {
	if msg.Event == nil || x.sink == nil {
		return
	}
	if err := x.sink.Write(msg.Event); err != nil {
		x.logger.Warnf("actor=%s audit write failed: %v", mcp.ActorNameJournal, err)
	}
}
