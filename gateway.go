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
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	goaktactor "github.com/tochemey/goakt/v4/actor"
	"github.com/tochemey/goakt/v4/eventstream"
	goaktlog "github.com/tochemey/goakt/v4/log"
	"github.com/tochemey/goakt/v4/remote"
	gtls "github.com/tochemey/goakt/v4/tls"

	"github.com/tochemey/goakt-mcp/internal/discovery"
	"github.com/tochemey/goakt-mcp/internal/egress"
	ingresshttp "github.com/tochemey/goakt-mcp/internal/ingress/http"
	ingresssse "github.com/tochemey/goakt-mcp/internal/ingress/sse"
	ingressws "github.com/tochemey/goakt-mcp/internal/ingress/ws"
	"github.com/tochemey/goakt-mcp/internal/naming"
	"github.com/tochemey/goakt-mcp/internal/runtime"
	"github.com/tochemey/goakt-mcp/internal/runtime/actor"
	actorextension "github.com/tochemey/goakt-mcp/internal/runtime/actor/extension"
	"github.com/tochemey/goakt-mcp/internal/runtime/cluster"
	"github.com/tochemey/goakt-mcp/internal/runtime/config"
	"github.com/tochemey/goakt-mcp/internal/runtime/policy"
	"github.com/tochemey/goakt-mcp/internal/runtime/telemetry"
	"github.com/tochemey/goakt-mcp/internal/security"
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

	mu       sync.RWMutex
	system   goaktactor.ActorSystem
	draining bool

	// eventSub receives actor system events (e.g. ActorPassivated) for metrics.
	eventSub    eventstream.Subscriber
	eventStopCh chan struct{}

	// managerName is the actor name used for GatewayManager. In single-node mode it
	// is always naming.ActorNameGatewayManager. In cluster mode it is suffixed with the
	// pod hostname so that each node can spawn its own local GatewayManager without
	// conflicting with GoAkt's cluster-wide actor name uniqueness check.
	managerName string

	// this is only set for testing and is used to inject a pre-started actor system, so Start doesn't create a new one
	testSystem goaktactor.ActorSystem
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

	// Config-level logger is applied first so that explicit options take precedence.
	if cfg.LogLevel != "" {
		gw.logger = config.NewLogger(cfg.LogLevel)
	}

	for _, opt := range opts {
		opt(gw)
	}

	if err := validateTenants(gw.config.Tenants); err != nil {
		return nil, err
	}

	return gw, nil
}

// Start creates and starts the GoAkt actor system, then spawns GatewayManager
// as the runtime composition root.
//
// When Cluster.Enabled is true, Start validates that a DiscoveryProvider is
// configured. If not, Start returns an error.
//
// Start must not be called more than once without an intervening Stop.
func (g *Gateway) Start(ctx context.Context) error {
	if err := g.validateClusterConfig(); err != nil {
		return err
	}

	if g.testSystem != nil {
		g.mu.Lock()
		g.system = g.testSystem
		g.managerName = naming.ActorNameGatewayManager
		g.mu.Unlock()
		return nil
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
		tlsInfo, err = security.BuildRemotingTLSInfo(g.config.Cluster.TLS)
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

	managerName := g.localManagerName()
	if _, err := system.Spawn(ctx, managerName, actor.NewGatewayManager(), goaktactor.WithLongLived()); err != nil {
		_ = system.Stop(ctx)
		return mcp.WrapRuntimeError(mcp.ErrCodeInternal, "failed to spawn GatewayManager", err)
	}
	g.managerName = managerName

	g.mu.Lock()
	g.system = system
	g.mu.Unlock()

	if g.metrics {
		g.startEventConsumer(system)
	}

	return nil
}

// Stop gracefully shuts down the actor system.
//
// Stop blocks until shutdown has completed or the provided context is cancelled.
// Calling Stop on a Gateway that was never started or already stopped is a no-op.
func (g *Gateway) Stop(ctx context.Context) error {
	g.mu.Lock()
	if g.system == nil {
		g.mu.Unlock()
		return nil
	}

	g.draining = true
	system := g.system
	g.mu.Unlock()

	g.stopEventConsumer(system)

	if err := system.Stop(ctx); err != nil {
		g.mu.Lock()
		g.draining = false
		g.mu.Unlock()
		return mcp.WrapRuntimeError(mcp.ErrCodeInternal, "failed to stop actor system", err)
	}

	g.mu.Lock()
	g.system = nil
	g.draining = false
	g.mu.Unlock()
	return nil
}

// System returns the underlying GoAkt actor system.
//
// Returns nil if Start has not been called or has not yet succeeded.
// Intended for advanced use cases and testing.
func (g *Gateway) System() goaktactor.ActorSystem {
	g.mu.RLock()
	s := g.system
	g.mu.RUnlock()
	return s
}

// requireRunning returns the actor system if the gateway is running and not
// draining. The returned system is safe to use for the duration of a single
// API call. Callers must not cache the returned system across calls.
func (g *Gateway) requireRunning() (goaktactor.ActorSystem, error) {
	g.mu.RLock()
	system := g.system
	draining := g.draining
	g.mu.RUnlock()
	if draining {
		return nil, mcp.NewRuntimeError(mcp.ErrCodeInternal, "gateway is shutting down")
	}
	if system == nil {
		return nil, mcp.NewRuntimeError(mcp.ErrCodeInternal, "gateway not started")
	}
	return system, nil
}

func (g *Gateway) validateClusterConfig() error {
	if g.config.Cluster.Enabled && discovery.IsNilDiscoveryProvider(g.config.Cluster.DiscoveryProvider) {
		return mcp.NewRuntimeError(mcp.ErrCodeInvalidRequest,
			"cluster is enabled but no DiscoveryProvider is configured")
	}
	return nil
}

// validateTenants checks that tenant IDs are non-empty and unique.
func validateTenants(tenants []mcp.TenantConfig) error {
	seen := make(map[mcp.TenantID]struct{}, len(tenants))
	for _, tenant := range tenants {
		if tenant.ID.IsZero() {
			return mcp.NewRuntimeError(mcp.ErrCodeInvalidRequest, "tenant ID must not be empty")
		}

		if _, dup := seen[tenant.ID]; dup {
			return mcp.NewRuntimeError(mcp.ErrCodeInvalidRequest, "duplicate tenant ID: "+string(tenant.ID))
		}
		seen[tenant.ID] = struct{}{}
	}
	return nil
}

// Handler returns an [http.Handler] that serves MCP Streamable HTTP sessions,
// routing each tool call through this gateway.
//
// Handler validates that cfg.IdentityResolver is set and delegates to
// [ingresshttp.New]. The gateway does not need to be running at the time
// Handler is called; tool discovery happens lazily per session via ListTools.
func (g *Gateway) Handler(cfg mcp.IngressConfig) (http.Handler, error) {
	return ingresshttp.New(g, cfg)
}

// SSEHandler returns an [http.Handler] that serves MCP SSE sessions
// (2024-11-05 spec version), routing each tool call through this gateway.
//
// SSEHandler validates that cfg.IdentityResolver is set and delegates to
// [ingresssse.New]. The gateway does not need to be running at the time
// SSEHandler is called; tool discovery happens lazily per session.
func (g *Gateway) SSEHandler(cfg mcp.IngressConfig) (http.Handler, error) {
	return ingresssse.New(g, cfg)
}

// WSHandler returns an [http.Handler] that upgrades HTTP connections to
// WebSocket and serves MCP sessions over the WebSocket transport.
//
// WSHandler validates that cfg.IdentityResolver is set and delegates to
// [ingressws.New]. The wsCfg parameter is optional; nil uses defaults.
// The gateway does not need to be running at the time WSHandler is called;
// tool discovery happens lazily per session.
func (g *Gateway) WSHandler(cfg mcp.IngressConfig, wsCfg *mcp.WSConfig) (http.Handler, error) {
	return ingressws.New(g, cfg, wsCfg)
}

// startEventConsumer subscribes to the actor system event stream and starts a
// background goroutine that polls for ActorPassivated events. This is the only
// reliable way to detect session passivation: GoAkt publishes ActorPassivated
// exclusively when the passivation strategy fires, not on explicit stops.
func (g *Gateway) startEventConsumer(system goaktactor.ActorSystem) {
	sub, err := system.Subscribe()
	if err != nil {
		g.logger.Warnf("failed to subscribe to event stream for passivation metrics: %v", err)
		return
	}

	stopCh := make(chan struct{})

	g.mu.Lock()
	g.eventSub = sub
	g.eventStopCh = stopCh
	g.mu.Unlock()

	go g.consumePassivationEvents(sub, stopCh)
}

// stopEventConsumer signals the background event consumer to stop and
// unsubscribes from the actor system event stream.
func (g *Gateway) stopEventConsumer(system goaktactor.ActorSystem) {
	g.mu.Lock()
	sub := g.eventSub
	stopCh := g.eventStopCh
	g.eventSub = nil
	g.eventStopCh = nil
	g.mu.Unlock()

	if stopCh != nil {
		close(stopCh)
	}
	if sub != nil {
		_ = system.Unsubscribe(sub)
	}
}

// consumePassivationEvents periodically drains the event stream subscriber and
// records a passivation metric for each ActorPassivated event whose address
// identifies a session actor. The tool ID is extracted from the supervisor
// parent component of the actor address.
func (g *Gateway) consumePassivationEvents(sub eventstream.Subscriber, stopCh <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			for msg := range sub.Iterator() {
				ev, ok := msg.Payload().(*goaktactor.ActorPassivated)
				if !ok {
					continue
				}
				toolID := toolIDFromPath(ev.ActorPath())
				if !toolID.IsZero() {
					telemetry.RecordSessionPassivated(context.Background(), toolID)
				}
			}
		}
	}
}

// toolIDFromPath extracts the tool ID from a passivated session's
// Path. Session actors are named "session-..." and are children of a supervisor
// actor named "supervisor-{toolID}". Returns a zero ToolID when the path does
// not match a session actor.
func toolIDFromPath(path goaktactor.Path) mcp.ToolID {
	if path == nil {
		return ""
	}
	if !strings.HasPrefix(path.Name(), "session-") {
		return ""
	}
	parent := path.Parent()
	if parent == nil {
		return ""
	}

	supervisorName := parent.Name()
	if !strings.HasPrefix(supervisorName, "supervisor-") {
		return ""
	}
	return naming.ToolIDFromSupervisorName(supervisorName)
}

// localManagerName returns the actor name to use for GatewayManager on this node.
//
// In single-node mode the name is always naming.ActorNameGatewayManager.
// In cluster mode GoAkt's system.Spawn checks actor name uniqueness across the
// entire cluster DMap, so a fixed name would prevent every node after the first
// from spawning its own GatewayManager. Suffixing with the pod hostname makes
// the name cluster-globally unique while remaining stable across pod restarts.
func (g *Gateway) localManagerName() string {
	if g.config.Cluster.Enabled {
		if hostname, err := os.Hostname(); err == nil && hostname != "" {
			return naming.ActorNameGatewayManager + "-" + hostname
		}
	}
	return naming.ActorNameGatewayManager
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
			(*runtime.SessionInvokeStream)(nil),
			(*runtime.SessionInvokeStreamResult)(nil),

			(*runtime.RecordAuditEvent)(nil),

			(*policy.EvaluateRequest)(nil),
			(*policy.EvaluateResult)(nil),

			(*runtime.ResolveRequest)(nil),
			(*runtime.ResolveResult)(nil),
			(*runtime.EnableTool)(nil),
			(*runtime.EnableToolResult)(nil),
			(*runtime.RefreshToolConfig)(nil),

			(*runtime.GetToolStatus)(nil),
			(*runtime.GetToolStatusResult)(nil),
			(*runtime.ResetCircuit)(nil),
			(*runtime.ResetCircuitResult)(nil),
			(*runtime.DrainTool)(nil),
			(*runtime.DrainToolResult)(nil),
			(*runtime.ListAllSessions)(nil),
			(*runtime.ListAllSessionsResult)(nil),
			(*runtime.ListSupervisorSessions)(nil),
			(*runtime.ListSupervisorSessionsResult)(nil),
			(*runtime.GetSessionIdentity)(nil),
			(*runtime.GetSessionIdentityResult)(nil),
			(*runtime.GetToolSchema)(nil),
			(*runtime.GetToolSchemaResult)(nil),
		),
	}
	if g.tracing {
		opts = append(opts, remote.WithContextPropagator(telemetry.NewOTelContextPropagator()))
	}
	return opts
}

func (g *Gateway) actorSystemOptions(tlsInfo *gtls.Info) []goaktactor.Option {
	execFactory := egress.NewCompositeExecutorFactory(g.config.Runtime.StartupTimeout, nil)
	schemaFetcher := egress.NewCompositeSchemaFetcher(g.config.Runtime.StartupTimeout, nil)
	opts := []goaktactor.Option{
		goaktactor.WithLogger(g.logger),
		goaktactor.WithActorInitMaxRetries(3),
		goaktactor.WithExtensions(
			actorextension.NewExecutorFactoryExtension(execFactory),
			actorextension.NewSchemaFetcherExtension(schemaFetcher),
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
