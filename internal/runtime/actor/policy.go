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
	"time"

	goaktactor "github.com/tochemey/goakt/v4/actor"
	goaktlog "github.com/tochemey/goakt/v4/log"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	"github.com/tochemey/goakt-mcp/internal/runtime/config"
	"github.com/tochemey/goakt-mcp/internal/runtime/policy"
)

// policyActor is the PolicyActor.
//
// PolicyActor evaluates authorization, quotas, rate limits, and concurrency
// limits before execution. It holds per-tenant usage counters for rate limiting
// and delegates to the policy Evaluator for authorization and quota checks.
//
// Spawn: GatewayManager spawns PolicyActor in spawnFoundationalActors via
// ctx.Spawn(ActorNamePolicy, newPolicyActor(cfg)) as a child of GatewayManager.
// The config is passed from GatewayManager's config.
//
// Relocation: No. PolicyActor runs on the local node as a child of GatewayManager
// and does not relocate in cluster mode.
//
// State is protected by the actor mailbox (one message at a time); no mutex
// is needed or allowed inside an actor.
//
// All fields are unexported to enforce actor immutability rules.
type policyActor struct {
	evaluator     *policy.Evaluator
	config        config.Config
	requestCounts map[runtime.TenantID]int
	currentMinute int64
	logger        goaktlog.Logger
}

// enforce that policyActor satisfies the GoAkt Actor interface at compile time.
var _ goaktactor.Actor = (*policyActor)(nil)

// newPolicyActor creates a PolicyActor instance.
// The config is passed at construction; the actor receives it from the spawner.
func newPolicyActor(config config.Config) *policyActor {
	return &policyActor{
		evaluator:     policy.NewEvaluator(config),
		config:        config,
		requestCounts: make(map[runtime.TenantID]int),
	}
}

// PreStart initializes the PolicyActor before message processing begins.
func (x *policyActor) PreStart(ctx *goaktactor.Context) error {
	x.logger = ctx.Logger()
	x.logger.Infof("actor=%s started", runtime.ActorNamePolicy)
	return nil
}

// Receive handles messages delivered to PolicyActor.
func (x *policyActor) Receive(ctx *goaktactor.ReceiveContext) {
	switch msg := ctx.Message().(type) {
	case *goaktactor.PostStart:
		x.logger.Debugf("actor=%s post-start", runtime.ActorNamePolicy)
	case *policy.EvaluateRequest:
		x.handleEvaluate(ctx, msg)
	default:
		ctx.Unhandled()
	}
}

// PostStop performs cleanup after PolicyActor has stopped.
func (x *policyActor) PostStop(ctx *goaktactor.Context) error {
	x.logger.Infof("actor=%s stopped", runtime.ActorNamePolicy)
	return nil
}

// handleEvaluate processes a policy evaluation request. It tracks per-tenant
// request counts for rate limiting (resetting each calendar minute) and
// delegates authorization and quota checks to the Evaluator.
func (x *policyActor) handleEvaluate(ctx *goaktactor.ReceiveContext, msg *policy.EvaluateRequest) {
	if msg.Input == nil {
		ctx.Response(&policy.EvaluateResult{
			Result: policy.Result{
				Decision: policy.DecisionDeny,
				Reason:   "missing policy input",
				Err:      runtime.NewRuntimeError(runtime.ErrCodeInvalidRequest, "missing policy input"),
			},
		})
		return
	}

	in := msg.Input

	now := time.Now()
	minute := now.Unix() / 60
	if minute != x.currentMinute {
		x.requestCounts = make(map[runtime.TenantID]int)
		x.currentMinute = minute
	}

	x.requestCounts[in.TenantID]++
	in.RequestsInCurrentMinute = x.requestCounts[in.TenantID]

	in.TenantConfig = x.lookupTenantConfig(in.TenantID)

	result := x.evaluator.Evaluate(in)
	ctx.Response(&policy.EvaluateResult{Result: result})
}

// lookupTenantConfig returns the TenantConfig for the given tenant, or nil
// when the tenant is not explicitly configured. Used to resolve quota limits.
func (x *policyActor) lookupTenantConfig(tenantID runtime.TenantID) *config.TenantConfig {
	for i := range x.config.Tenants {
		tc := &x.config.Tenants[i]
		if tc.ID == tenantID {
			return tc
		}
	}
	return nil
}
