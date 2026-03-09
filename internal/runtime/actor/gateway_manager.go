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
	"errors"

	goaktactor "github.com/tochemey/goakt/v4/actor"
	goakterrors "github.com/tochemey/goakt/v4/errors"
	goaktlog "github.com/tochemey/goakt/v4/log"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	"github.com/tochemey/goakt-mcp/internal/runtime/audit"
	"github.com/tochemey/goakt-mcp/internal/runtime/cluster"
	"github.com/tochemey/goakt-mcp/internal/runtime/config"
	"github.com/tochemey/goakt-mcp/internal/runtime/credentials"
	"github.com/tochemey/goakt-mcp/mcp"
)

// GatewayManager is the runtime composition root inside the GoAkt actor system.
// It is responsible for spawning and supervising the foundational runtime actors:
// RegistryActor, HealthActor, JournalActor, PolicyActor, CredentialBrokerActor,
// and RouterActor. It is always the first actor spawned after the actor system
// starts and the last to stop during shutdown.
//
// Spawn: Gateway.Start calls system.Spawn(ctx, ActorNameGatewayManager, NewGatewayManager(cfg)).
// GatewayManager is a top-level actor with no parent.
//
// Relocation: No. GatewayManager runs on the local node and does not relocate in cluster mode.
//
// All fields are unexported to enforce actor immutability rules. State is
// initialized in PreStart or on receipt of PostStart.
type GatewayManager struct {
	config config.Config
	logger goaktlog.Logger
}

// enforce that GatewayManager satisfies the GoAkt Actor interface at compile time.
var _ goaktactor.Actor = (*GatewayManager)(nil)

// NewGatewayManager creates a GatewayManager actor primed with the provided configuration.
// The logger is captured from the actor context on PostStart, not injected here.
func NewGatewayManager(config config.Config) *GatewayManager {
	return &GatewayManager{config: config}
}

// PreStart initializes the GatewayManager before message processing begins.
// At this stage the actor context is available but the actor system has not yet
// delivered the PostStart signal. Heavy initialization belongs in the PostStart handler.
func (g *GatewayManager) PreStart(ctx *goaktactor.Context) error {
	g.logger = ctx.Logger()
	return nil
}

// Receive handles messages delivered to GatewayManager.
//
// On PostStart: spawns RegistryActor, HealthActor, and JournalActor as supervised
// children. These actors are always present for the lifetime of the runtime.
// Unknown messages are marked as unhandled so the runtime can dead-letter them.
func (g *GatewayManager) Receive(ctx *goaktactor.ReceiveContext) {
	switch ctx.Message().(type) {
	case *goaktactor.PostStart:
		g.spawnFoundationalActors(ctx)
	default:
		ctx.Unhandled()
	}
}

// PostStop performs cleanup after GatewayManager has stopped.
func (g *GatewayManager) PostStop(ctx *goaktactor.Context) error {
	ctx.Logger().Infof("actor=%s stopped", mcp.ActorNameGatewayManager)
	return nil
}

// spawnFoundationalActors spawns RegistryActor, HealthActor, and JournalActor as
// children of GatewayManager. They are spawned in dependency order so that the
// journal is running before BootstrapTools triggers ToolSupervisorActor spawns.
// Each supervisor resolves the journal by name from the actor system in PreStart.
//
// If any child cannot be spawned, the error is recorded on the ReceiveContext and
// the GatewayManager will surface it to the supervision layer.
func (g *GatewayManager) spawnFoundationalActors(ctx *goaktactor.ReceiveContext) {
	g.logger.Infof("actor=%s spawning foundational actors", mcp.ActorNameGatewayManager)

	// spawn the registrar
	registrar := g.spawnRegistrar(ctx)

	// spawn the journal actor (before health and router so they can use it)
	auditSink := createAuditSink(g.config.Audit)
	journaler := ctx.Spawn(mcp.ActorNameJournal, newJournaler(auditSink))

	// spawn the health actor with journal for health transition audit events
	ctx.Spawn(mcp.ActorNameHealth, newHealthChecker(registrar, journaler, g.config.Runtime.HealthProbeInterval))

	// spawn the policy actor
	policyPID := ctx.Spawn(mcp.ActorNamePolicy, newPolicyActor(g.config))

	// spawn the credential broker actor
	credentialBroker := g.spawnCredentialBroker(ctx)

	// spawn the router actor
	if registrar != nil {
		ctx.Spawn(mcp.ActorNameRouter, newRouterActor(registrar, policyPID, credentialBroker, journaler))
	}

	// bootstrap the tools after journal is running so supervisors can resolve it
	if registrar != nil && len(g.config.Tools) > 0 {
		ctx.Tell(registrar, &runtime.BootstrapTools{Tools: g.config.Tools})
	}

	g.logger.Infof("actor=%s foundational actors spawned", mcp.ActorNameGatewayManager)
}

// spawnCredentialBroker spawns CredentialBrokerActor when providers are configured.
// Returns nil when no providers are configured.
func (g *GatewayManager) spawnCredentialBroker(ctx *goaktactor.ReceiveContext) *goaktactor.PID {
	providers := buildCredentialProviders(g.config.Credentials.Providers)
	if len(providers) == 0 {
		return nil
	}
	return ctx.Spawn(mcp.ActorNameCredentialBroker, newCredentialBroker(providers, credentials.DefaultCredentialTTL))
}

// spawnRegistrar spawns the RegistryActor. When cluster is configured (enabled with
// valid kubernetes or dnssd discovery), uses SpawnSingleton so the registry is a cluster singleton.
func (g *GatewayManager) spawnRegistrar(ctx *goaktactor.ReceiveContext) *goaktactor.PID {
	if cluster.IsClusterConfigured(g.config) {
		sys := ctx.Self().ActorSystem()

		var opts []goaktactor.ClusterSingletonOption
		if g.config.Cluster.SingletonRole != "" {
			opts = append(opts, goaktactor.WithSingletonRole(g.config.Cluster.SingletonRole))
		}

		pid, err := sys.SpawnSingleton(ctx.Context(), mcp.ActorNameRegistrar, newRegistrar(), opts...)
		if err != nil {
			if errors.Is(err, goakterrors.ErrSingletonAlreadyExists) {
				// let us fetch the existing singleton
				pid, err := sys.ActorOf(ctx.Context(), mcp.ActorNameRegistrar)
				if err != nil {
					g.logger.Warnf("actor=%s failed to fetch existing singleton registry: %v", mcp.ActorNameGatewayManager, err)
					return nil
				}

				return pid
			}

			g.logger.Warnf("actor=%s spawn singleton registry failed: %v", mcp.ActorNameGatewayManager, err)
			return nil
		}
		return pid
	}
	return ctx.Spawn(mcp.ActorNameRegistrar, newRegistrar())
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
