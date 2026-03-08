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
	"net/http"
	"sync"

	goaktactor "github.com/tochemey/goakt/v4/actor"
	goaktlog "github.com/tochemey/goakt/v4/log"

	"github.com/tochemey/goakt-mcp/internal/egress"
	ingresshttp "github.com/tochemey/goakt-mcp/internal/ingress/http"
	"github.com/tochemey/goakt-mcp/internal/runtime"
	actorextension "github.com/tochemey/goakt-mcp/internal/runtime/actor/extension"
	"github.com/tochemey/goakt-mcp/internal/runtime/cluster"
	"github.com/tochemey/goakt-mcp/internal/runtime/config"
)

// gatewayActorSystemName is the fixed name of the GoAkt actor system for the gateway.
const gatewayActorSystemName = "goakt-mcp"

// Gateway is the top-level runtime handle for the goakt-mcp gateway.
//
// Gateway owns the GoAkt actor system and orchestrates the full lifecycle of all
// runtime actors. It is the process-level entry point: callers create a Gateway,
// call Start to make the runtime operational, and call Stop to shut it down cleanly.
//
// Gateway itself is not an actor. The actor composition root inside the system is
// GatewayManager, which is spawned by Gateway during Start.
//
// When HTTP.ListenAddress is configured, Gateway starts the ingress HTTP server
// and stops it during Stop.
type Gateway struct {
	config     config.Config
	logger     goaktlog.Logger
	system     goaktactor.ActorSystem
	httpServer *ingresshttp.Server
	httpDone   chan struct{}
	httpOnce   sync.Once
}

// New creates a new Gateway with the provided configuration and logger.
//
// The logger must satisfy the GoAkt log.Logger interface. Pass log.DiscardLogger
// to suppress all runtime log output (useful in tests). New does not start the
// actor system; call Start to make the gateway operational.
func New(config config.Config, logger goaktlog.Logger) (*Gateway, error) {
	if logger == nil {
		return nil, runtime.NewRuntimeError(runtime.ErrCodeInvalidRequest, "logger is required")
	}
	return &Gateway{config: config, logger: logger}, nil
}

// Start creates and starts the GoAkt actor system, then spawns GatewayManager
// as the runtime composition root.
//
// If the actor system cannot be created or started, Start returns a RuntimeError
// with ErrCodeInternal. If GatewayManager cannot be spawned, Start stops the
// actor system before returning the error, leaving the Gateway in a clean state.
//
// When Cluster.Enabled is true, Start validates that discovery is configured
// (kubernetes or dnssd with valid config). If not, Start returns ErrCodeInvalidRequest.
//
// Start must not be called more than once without an intervening Stop.
func (g *Gateway) Start(ctx context.Context) error {
	if g.config.Cluster.Enabled && !cluster.IsClusterConfigured(g.config) {
		return runtime.NewRuntimeError(runtime.ErrCodeInvalidRequest,
			"cluster is enabled but discovery is not configured: set Discovery to \"kubernetes\" or \"dnssd\" with valid config")
	}

	execFactory := egress.NewCompositeExecutorFactory(g.config.Runtime.StartupTimeout, nil)
	opts := []goaktactor.Option{
		goaktactor.WithLogger(g.logger),
		goaktactor.WithActorInitMaxRetries(3),
		goaktactor.WithExtensions(actorextension.NewExecutorFactoryExtension(execFactory)),
	}

	if clusterOpts := cluster.BuildOptions(g.config, NewRegistrar()); len(clusterOpts) > 0 {
		opts = append(opts, clusterOpts...)
	}

	system, err := goaktactor.NewActorSystem(gatewayActorSystemName, opts...)
	if err != nil {
		return runtime.WrapRuntimeError(runtime.ErrCodeInternal, "failed to create actor system", err)
	}

	if err := system.Start(ctx); err != nil {
		return runtime.WrapRuntimeError(runtime.ErrCodeInternal, "failed to start actor system", err)
	}

	if _, err := system.Spawn(ctx, runtime.ActorNameGatewayManager, newGatewayManager(g.config)); err != nil {
		_ = system.Stop(ctx)
		return runtime.WrapRuntimeError(runtime.ErrCodeInternal, "failed to spawn GatewayManager", err)
	}

	g.system = system

	if g.config.HTTP.ListenAddress != "" {
		g.httpServer = ingresshttp.NewServer(g.config.HTTP, system, g.logger)
		g.httpDone = make(chan struct{})
		go func() {
			defer close(g.httpDone)
			if err := g.httpServer.Start(ctx); err != nil && err != http.ErrServerClosed {
				g.logger.Warnf("ingress http server: %v", err)
			}
		}()
	}

	return nil
}

// Stop gracefully shuts down the HTTP server (if started) and the actor system.
//
// Stop blocks until both have completed or the provided context is cancelled.
// Calling Stop on a Gateway that was never started is a no-op. Stop must not
// be called concurrently with Start.
func (g *Gateway) Stop(ctx context.Context) error {
	if g.system == nil {
		return nil
	}

	g.httpOnce.Do(func() {
		if g.httpServer != nil {
			_ = g.httpServer.Stop(ctx)
			g.httpServer = nil
			<-g.httpDone
		}
	})

	if err := g.system.Stop(ctx); err != nil {
		return runtime.WrapRuntimeError(runtime.ErrCodeInternal, "failed to stop actor system", err)
	}
	return nil
}

// System returns the underlying GoAkt actor system.
//
// System returns nil if Start has not been called or has not yet succeeded.
// This method is primarily intended for introspection and testing; production
// code should interact with the runtime through actor messages rather than
// direct actor-system access.
func (g *Gateway) System() goaktactor.ActorSystem {
	return g.system
}
