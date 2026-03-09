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

	goaktactor "github.com/tochemey/goakt/v4/actor"
	goaktlog "github.com/tochemey/goakt/v4/log"
	"github.com/tochemey/goakt/v4/remote"
	gtls "github.com/tochemey/goakt/v4/tls"

	"github.com/tochemey/goakt-mcp/internal/egress"
	"github.com/tochemey/goakt-mcp/internal/runtime"
	"github.com/tochemey/goakt-mcp/internal/runtime/actor"
	actorextension "github.com/tochemey/goakt-mcp/internal/runtime/actor/extension"
	"github.com/tochemey/goakt-mcp/internal/runtime/cluster"
	"github.com/tochemey/goakt-mcp/internal/runtime/config"
	"github.com/tochemey/goakt-mcp/internal/runtime/policy"
	"github.com/tochemey/goakt-mcp/internal/runtime/telemetry"
	"github.com/tochemey/goakt-mcp/mcp"
)

const gatewayActorSystemName = "goakt-mcp"

// Gateway is the top-level handle for the goakt-mcp gateway.
//
// Gateway owns the GoAkt actor system and orchestrates the full lifecycle of all
// runtime actors. It exposes a programmatic API for tool invocations, listing,
// and dynamic tool management.
//
// Create a Gateway with New, start it with Start, and stop it with Stop.
type Gateway struct {
	config  mcp.Config
	logger  goaktlog.Logger
	metrics bool
	tracing bool
	system  goaktactor.ActorSystem
}

// New creates a new Gateway with the provided configuration and options.
//
// New validates the configuration and applies defaults for any zero-valued
// runtime settings. Call Start to make the gateway operational.
func New(cfg mcp.Config, opts ...Option) (*Gateway, error) {
	config.ApplyDefaults(&cfg)

	gw := &Gateway{
		config: cfg,
		logger: goaktlog.DiscardLogger,
	}

	for _, opt := range opts {
		opt(gw)
	}

	if gw.config.LogLevel != "" {
		gw.logger = config.NewLogger(gw.config.LogLevel)
	}

	return gw, nil
}

// Start creates and starts the GoAkt actor system, then spawns GatewayManager
// as the runtime composition root.
//
// When Cluster.Enabled is true, Start validates that discovery is configured
// (kubernetes or dnssd with valid config). If not, Start returns an error.
//
// Start must not be called more than once without an intervening Stop.
func (g *Gateway) Start(ctx context.Context) error {
	if err := g.validateClusterConfig(); err != nil {
		return err
	}

	if g.metrics {
		if _, err := telemetry.RegisterMetrics(nil); err != nil {
			return mcp.WrapRuntimeError(mcp.ErrCodeInternal, "failed to register metrics", err)
		}
	}

	if g.tracing {
		telemetry.RegisterTracer()
	}

	var tlsInfo *gtls.Info
	if g.config.Cluster.Enabled && g.config.Cluster.TLS != nil {
		var err error
		tlsInfo, err = mcp.BuildRemotingTLSInfo(g.config.Cluster.TLS)
		if err != nil {
			return mcp.WrapRuntimeError(mcp.ErrCodeInternal, "cluster TLS config", err)
		}
	}

	system, err := goaktactor.NewActorSystem(gatewayActorSystemName, g.actorSystemOptions(tlsInfo)...)
	if err != nil {
		return mcp.WrapRuntimeError(mcp.ErrCodeInternal, "failed to create actor system", err)
	}

	if err := system.Start(ctx); err != nil {
		return mcp.WrapRuntimeError(mcp.ErrCodeInternal, "failed to start actor system", err)
	}

	if _, err := system.Spawn(ctx, mcp.ActorNameGatewayManager, actor.NewGatewayManager()); err != nil {
		_ = system.Stop(ctx)
		return mcp.WrapRuntimeError(mcp.ErrCodeInternal, "failed to spawn GatewayManager", err)
	}

	g.system = system
	return nil
}

// Stop gracefully shuts down the actor system.
//
// Stop blocks until shutdown has completed or the provided context is cancelled.
// Calling Stop on a Gateway that was never started is a no-op.
func (g *Gateway) Stop(ctx context.Context) error {
	if g.system == nil {
		return nil
	}

	if err := g.system.Stop(ctx); err != nil {
		return mcp.WrapRuntimeError(mcp.ErrCodeInternal, "failed to stop actor system", err)
	}
	return nil
}

// System returns the underlying GoAkt actor system.
//
// Returns nil if Start has not been called or has not yet succeeded.
// Intended for advanced use cases and testing.
func (g *Gateway) System() goaktactor.ActorSystem {
	return g.system
}

func (g *Gateway) validateClusterConfig() error {
	if g.config.Cluster.Enabled && !cluster.IsClusterConfigured(g.config) {
		return mcp.NewRuntimeError(mcp.ErrCodeInvalidRequest,
			"cluster is enabled but discovery is not configured: set Discovery to \"kubernetes\" or \"dnssd\" with valid config")
	}
	return nil
}

func (g *Gateway) remoteOptions() []remote.Option {
	opts := []remote.Option{
		remote.WithSerializables(
			(*runtime.CanAcceptWork)(nil),
			(*runtime.CanAcceptWorkResult)(nil),
			(*runtime.ReportFailure)(nil),
			(*runtime.ReportSuccess)(nil),

			(*runtime.RouteInvocation)(nil),
			(*runtime.RouteResult)(nil),

			(*runtime.RegisterTool)(nil),
			(*runtime.RegisterToolResult)(nil),
			(*runtime.UpdateTool)(nil),
			(*runtime.UpdateToolResult)(nil),
			(*runtime.DisableTool)(nil),
			(*runtime.DisableToolResult)(nil),
			(*runtime.RemoveTool)(nil),
			(*runtime.RemoveToolResult)(nil),
			(*runtime.QueryTool)(nil),
			(*runtime.QueryToolResult)(nil),
			(*runtime.UpdateToolHealth)(nil),
			(*runtime.UpdateToolHealthResult)(nil),
			(*runtime.BootstrapTools)(nil),
			(*runtime.GetSupervisor)(nil),
			(*runtime.GetSupervisorResult)(nil),
			(*runtime.ListTools)(nil),
			(*runtime.ListToolsResult)(nil),
			(*runtime.CountSessionsForTenant)(nil),
			(*runtime.CountSessionsForTenantResult)(nil),
			(*runtime.SupervisorCountSessionsForTenant)(nil),
			(*runtime.SupervisorCountSessionsForTenantResult)(nil),

			(*runtime.GetOrCreateSession)(nil),
			(*runtime.GetOrCreateSessionResult)(nil),
			(*runtime.SessionInvoke)(nil),
			(*runtime.SessionInvokeResult)(nil),

			(*runtime.RecordAuditEvent)(nil),

			(*policy.EvaluateRequest)(nil),
			(*policy.EvaluateResult)(nil),

			(*runtime.ResolveRequest)(nil),
			(*runtime.ResolveResult)(nil),
		),
	}
	if g.tracing {
		opts = append(opts, remote.WithContextPropagator(telemetry.NewOTelContextPropagator()))
	}
	return opts
}

func (g *Gateway) actorSystemOptions(tlsInfo *gtls.Info) []goaktactor.Option {
	execFactory := egress.NewCompositeExecutorFactory(g.config.Runtime.StartupTimeout, nil)
	opts := []goaktactor.Option{
		goaktactor.WithLogger(g.logger),
		goaktactor.WithActorInitMaxRetries(3),
		goaktactor.WithExtensions(
			actorextension.NewExecutorFactoryExtension(execFactory),
			actorextension.NewToolConfigExtension(),
			actorextension.NewConfigExtension(g.config),
		),
	}

	if tlsInfo != nil {
		opts = append(opts, goaktactor.WithTLS(tlsInfo))
	}

	if clusterOpts := cluster.BuildOptions(g.config, g.remoteOptions(), actor.NewRegistrar()); len(clusterOpts) > 0 {
		opts = append(opts, clusterOpts...)
	}
	return opts
}
