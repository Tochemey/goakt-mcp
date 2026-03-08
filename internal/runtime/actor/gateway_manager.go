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

	"github.com/tochemey/goakt-mcp/internal/runtime"
	"github.com/tochemey/goakt-mcp/internal/runtime/audit"
	"github.com/tochemey/goakt-mcp/internal/runtime/cluster"
	"github.com/tochemey/goakt-mcp/internal/runtime/config"
	"github.com/tochemey/goakt-mcp/internal/runtime/credentials"
)

// gatewayManager is the GatewayManager actor.
//
// GatewayManager is the runtime composition root inside the GoAkt actor system.
// It is responsible for spawning and supervising the foundational runtime actors:
// RegistryActor, HealthActor, JournalActor, PolicyActor, CredentialBrokerActor,
// and RouterActor. It is always the first actor spawned after the actor system
// starts and the last to stop during shutdown.
//
// Spawn: Gateway.Start calls system.Spawn(ctx, ActorNameGatewayManager, newGatewayManager(cfg)).
// GatewayManager is a top-level actor with no parent.
//
// Relocation: No. GatewayManager runs on the local node and does not relocate in cluster mode.
//
// All fields are unexported to enforce actor immutability rules. State is
// initialized in PreStart or on receipt of PostStart.
type gatewayManager struct {
	cfg    config.Config
	logger goaktlog.Logger
}

// enforce that gatewayManager satisfies the GoAkt Actor interface at compile time.
var _ goaktactor.Actor = (*gatewayManager)(nil)

// newGatewayManager creates a GatewayManager actor primed with the provided configuration.
// The logger is captured from the actor context on PostStart, not injected here.
func newGatewayManager(cfg config.Config) *gatewayManager {
	return &gatewayManager{cfg: cfg}
}

// PreStart initializes the GatewayManager before message processing begins.
// At this stage the actor context is available but the actor system has not yet
// delivered the PostStart signal. Heavy initialization belongs in the PostStart handler.
func (g *gatewayManager) PreStart(ctx *goaktactor.Context) error {
	g.logger = ctx.Logger()
	return nil
}

// Receive handles messages delivered to GatewayManager.
//
// On PostStart: spawns RegistryActor, HealthActor, and JournalActor as supervised
// children. These actors are always present for the lifetime of the runtime.
// Unknown messages are marked as unhandled so the runtime can dead-letter them.
func (g *gatewayManager) Receive(ctx *goaktactor.ReceiveContext) {
	switch ctx.Message().(type) {
	case *goaktactor.PostStart:
		g.spawnFoundationalActors(ctx)
	default:
		ctx.Unhandled()
	}
}

// PostStop performs cleanup after GatewayManager has stopped.
func (g *gatewayManager) PostStop(ctx *goaktactor.Context) error {
	ctx.Logger().Infof("actor=%s stopped", runtime.ActorNameGatewayManager)
	return nil
}

// spawnFoundationalActors spawns RegistryActor, HealthActor, and JournalActor as
// children of GatewayManager. They are spawned in dependency order so that the
// registry is always available before health and journal actors start.
//
// After spawning RegistryActor, GatewayManager sends BootstrapTools with tools
// from the static configuration so the registry is populated before other
// actors depend on it.
//
// If any child cannot be spawned, the error is recorded on the ReceiveContext and
// the GatewayManager will surface it to the supervision layer.
func (g *gatewayManager) spawnFoundationalActors(ctx *goaktactor.ReceiveContext) {
	g.logger.Infof("actor=%s spawning foundational actors", runtime.ActorNameGatewayManager)

	registryPID := g.spawnRegistry(ctx)
	ctx.Spawn(runtime.ActorNameHealth, newHealthChecker(registryPID, g.cfg.Runtime.HealthProbeInterval))

	auditSink := createAuditSink(g.cfg.Audit)
	journalPID := ctx.Spawn(runtime.ActorNameJournal, newJournaler(auditSink))
	_ = journalPID

	policyPID := ctx.Spawn(runtime.ActorNamePolicy, newPolicyActor(g.cfg))

	credentialBrokerPID := g.spawnCredentialBroker(ctx)

	if registryPID != nil {
		ctx.Spawn(runtime.ActorNameRouter, newRouterActor(registryPID, policyPID, credentialBrokerPID, journalPID))
	}

	if registryPID != nil && len(g.cfg.Tools) > 0 {
		tools := make([]runtime.Tool, 0, len(g.cfg.Tools))
		for _, tc := range g.cfg.Tools {
			tools = append(tools, config.ToolConfigToTool(tc, g.cfg.Runtime))
		}
		ctx.Tell(registryPID, &runtime.BootstrapTools{Tools: tools})
	}

	g.logger.Infof("actor=%s foundational actors spawned", runtime.ActorNameGatewayManager)
}

// spawnCredentialBroker spawns CredentialBrokerActor when providers are configured.
// Returns nil when no providers are configured.
func (g *gatewayManager) spawnCredentialBroker(ctx *goaktactor.ReceiveContext) *goaktactor.PID {
	providers := buildCredentialProviders(g.cfg.Credentials.Providers)
	if len(providers) == 0 {
		return nil
	}
	return ctx.Spawn(runtime.ActorNameCredentialBroker, newCredentialBroker(providers, credentials.DefaultCredentialTTL))
}

// spawnRegistry spawns the RegistryActor. When cluster is configured (enabled with
// valid kubernetes or dnssd discovery), uses SpawnSingleton so the registry is a cluster singleton.
// Otherwise spawns locally.
func (g *gatewayManager) spawnRegistry(ctx *goaktactor.ReceiveContext) *goaktactor.PID {
	if cluster.IsClusterConfigured(g.cfg) {
		sys := ctx.Self().ActorSystem()
		opts := []goaktactor.ClusterSingletonOption{}
		if g.cfg.Cluster.SingletonRole != "" {
			opts = append(opts, goaktactor.WithSingletonRole(g.cfg.Cluster.SingletonRole))
		}
		pid, err := sys.SpawnSingleton(ctx.Context(), runtime.ActorNameRegistrar, newRegistrar(), opts...)
		if err != nil {
			g.logger.Warnf("actor=%s spawn singleton registry failed: %v", runtime.ActorNameGatewayManager, err)
			return nil
		}
		return pid
	}
	return ctx.Spawn(runtime.ActorNameRegistrar, newRegistrar())
}

// createAuditSink creates an audit sink from config.
// Returns MemorySink for "memory" or empty backend; nil otherwise.
func createAuditSink(cfg config.AuditConfig) audit.Sink {
	switch cfg.Backend {
	case "", "memory":
		return audit.NewMemorySink()
	default:
		return audit.NewMemorySink()
	}
}

// buildCredentialProviders builds provider instances from config provider names.
func buildCredentialProviders(names []string) []credentials.Provider {
	var out []credentials.Provider
	for _, n := range names {
		switch n {
		case "env":
			out = append(out, credentials.NewEnvProvider())
		}
	}
	return out
}
