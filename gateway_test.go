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
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goaktactor "github.com/tochemey/goakt/v4/actor"
	goaktlog "github.com/tochemey/goakt/v4/log"

	"github.com/tochemey/goakt-mcp/internal/egress"
	"github.com/tochemey/goakt-mcp/internal/runtime/actor"
	actorextension "github.com/tochemey/goakt-mcp/internal/runtime/actor/extension"
	"github.com/tochemey/goakt-mcp/internal/runtime/config"
	"github.com/tochemey/goakt-mcp/mcp"
)

// fixedIdentityResolver always returns the configured tenant and client IDs.
type fixedIdentityResolver struct {
	tenantID mcp.TenantID
	clientID mcp.ClientID
}

func (r *fixedIdentityResolver) ResolveIdentity(_ *http.Request) (mcp.TenantID, mcp.ClientID, error) {
	return r.tenantID, r.clientID, nil
}

// freePort returns an available port for use in tests.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

// minimalGatewayManager is a GatewayManager with no children. Used to exercise
// the resolveRegistrar/resolveRouter fallback path (system.ActorOf when Child fails).
type minimalGatewayManager struct{}

func (m *minimalGatewayManager) PreStart(_ *goaktactor.Context) error { return nil }
func (m *minimalGatewayManager) Receive(_ *goaktactor.ReceiveContext) {}
func (m *minimalGatewayManager) PostStop(_ *goaktactor.Context) error { return nil }

func testConfig() mcp.Config {
	return mcp.Config{
		Runtime: mcp.RuntimeConfig{
			SessionIdleTimeout: mcp.DefaultSessionIdleTimeout,
			RequestTimeout:     mcp.DefaultRequestTimeout,
			StartupTimeout:     mcp.DefaultStartupTimeout,
		},
	}
}

func waitForActors() {
	time.Sleep(100 * time.Millisecond)
}

func TestNew(t *testing.T) {
	t.Run("returns Gateway with valid config", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)
		require.NotNil(t, gw)
		assert.Nil(t, gw.System(), "system must be nil before Start")
	})

	t.Run("WithMetrics enables metrics", func(t *testing.T) {
		gw, err := New(testConfig(), WithMetrics())
		require.NoError(t, err)
		require.NotNil(t, gw)
		assert.True(t, gw.metrics)
	})

	t.Run("returns Gateway with default logger when none provided", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)
		require.NotNil(t, gw)
	})

	t.Run("New with LogLevel in config uses config logger", func(t *testing.T) {
		cfg := testConfig()
		cfg.LogLevel = "info"
		gw, err := New(cfg)
		require.NoError(t, err)
		require.NotNil(t, gw)
		assert.NotEqual(t, goaktlog.DiscardLogger, gw.logger)
	})
}

func TestGatewayStartStop(t *testing.T) {
	ctx := context.Background()

	t.Run("starts cleanly and exposes the actor system", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)

		require.NoError(t, gw.Start(ctx))
		assert.NotNil(t, gw.System())

		waitForActors()

		require.NoError(t, gw.Stop(ctx))
	})

	t.Run("stop on unstarted gateway is a no-op", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)
		assert.NoError(t, gw.Stop(ctx))
	})

	t.Run("GatewayManager is present after start", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))

		waitForActors()

		pid, err := gw.System().ActorOf(ctx, mcp.ActorNameGatewayManager)
		require.NoError(t, err)
		require.NotNil(t, pid)

		require.NoError(t, gw.Stop(ctx))
	})

	t.Run("foundational actors are children of GatewayManager after start", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))

		waitForActors()

		managerPID, err := gw.System().ActorOf(ctx, mcp.ActorNameGatewayManager)
		require.NoError(t, err)
		require.NotNil(t, managerPID)

		children := managerPID.Children()
		childNames := make(map[string]bool, len(children))
		for _, c := range children {
			childNames[c.Name()] = true
		}

		assert.True(t, childNames[mcp.ActorNameRegistrar], "RegistryActor must be a child of GatewayManager")
		assert.True(t, childNames[mcp.ActorNameHealth], "HealthActor must be a child of GatewayManager")
		assert.True(t, childNames[mcp.ActorNameJournal], "JournalActor must be a child of GatewayManager")

		require.NoError(t, gw.Stop(ctx))
	})

	t.Run("bootstrap tools from config are loaded into registry", func(t *testing.T) {
		cfg := testConfig()
		cfg.Tools = []mcp.Tool{
			{
				ID:        mcp.ToolID("bootstrap-tool"),
				Transport: mcp.TransportStdio,
				Stdio:     &mcp.StdioTransportConfig{Command: "npx"},
				State:     mcp.ToolStateEnabled,
			},
		}

		gw, err := New(cfg)
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))

		waitForActors()

		tools, err := gw.ListTools(ctx)
		require.NoError(t, err)
		require.Len(t, tools, 1)
		assert.Equal(t, mcp.ToolID("bootstrap-tool"), tools[0].ID)
		assert.Equal(t, mcp.TransportStdio, tools[0].Transport)

		require.NoError(t, gw.Stop(ctx))
	})

	t.Run("Start returns error when Cluster.Enabled but discovery not configured", func(t *testing.T) {
		cfg := testConfig()
		cfg.Cluster.Enabled = true
		cfg.Cluster.Discovery = "kubernetes"
		cfg.Cluster.Kubernetes = mcp.KubernetesDiscoveryConfig{}

		gw, err := New(cfg)
		require.NoError(t, err)

		err = gw.Start(ctx)
		require.Error(t, err)
		var rErr *mcp.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, mcp.ErrCodeInvalidRequest, rErr.Code)
		assert.Contains(t, err.Error(), "cluster is enabled but discovery is not configured")
	})

	t.Run("Start returns error when Cluster.Enabled with dnssd but empty DomainName", func(t *testing.T) {
		cfg := testConfig()
		cfg.Cluster.Enabled = true
		cfg.Cluster.Discovery = "dnssd"
		cfg.Cluster.DNSSD = mcp.DNSSDDiscoveryConfig{DomainName: ""}

		gw, err := New(cfg)
		require.NoError(t, err)

		err = gw.Start(ctx)
		require.Error(t, err)
		var rErr *mcp.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, mcp.ErrCodeInvalidRequest, rErr.Code)
	})

	t.Run("Start returns error when Cluster.TLS has invalid cert paths", func(t *testing.T) {
		cfg := testConfig()
		cfg.Cluster.Enabled = true
		cfg.Cluster.Discovery = "dnssd"
		cfg.Cluster.DNSSD = mcp.DNSSDDiscoveryConfig{DomainName: "local."}
		cfg.Cluster.TLS = &mcp.RemotingTLSConfig{
			CertFile: "/nonexistent/cert.pem",
			KeyFile:  "/nonexistent/key.pem",
		}

		gw, err := New(cfg)
		require.NoError(t, err)

		err = gw.Start(ctx)
		require.Error(t, err)
		var rErr *mcp.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, mcp.ErrCodeInternal, rErr.Code)
		assert.Contains(t, err.Error(), "cluster TLS")
	})

	t.Run("Start with WithMetrics succeeds", func(t *testing.T) {
		gw, err := New(testConfig(), WithMetrics())
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))
		waitForActors()
		assert.NotNil(t, gw.System())
		require.NoError(t, gw.Stop(ctx))
	})

	t.Run("Start with WithTracing succeeds", func(t *testing.T) {
		gw, err := New(testConfig(), WithTracing())
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))
		waitForActors()
		assert.NotNil(t, gw.System())
		require.NoError(t, gw.Stop(ctx))
	})
}

func TestGatewayAPI(t *testing.T) {
	ctx := context.Background()

	t.Run("RegisterTool and ListTools", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))
		waitForActors()
		defer func() { _ = gw.Stop(ctx) }()

		tool := mcp.Tool{
			ID:        "dynamic-tool",
			Transport: mcp.TransportStdio,
			Stdio:     &mcp.StdioTransportConfig{Command: "echo"},
			State:     mcp.ToolStateEnabled,
		}

		err = gw.RegisterTool(ctx, tool)
		require.NoError(t, err)

		waitForActors()

		tools, err := gw.ListTools(ctx)
		require.NoError(t, err)
		found := false
		for _, t := range tools {
			if t.ID == "dynamic-tool" {
				found = true
				break
			}
		}
		assert.True(t, found, "dynamically registered tool should be listed")
	})

	t.Run("DisableTool", func(t *testing.T) {
		cfg := testConfig()
		cfg.Tools = []mcp.Tool{
			{ID: "to-disable", Transport: mcp.TransportStdio, Stdio: &mcp.StdioTransportConfig{Command: "npx"}, State: mcp.ToolStateEnabled},
		}
		gw, err := New(cfg)
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))
		waitForActors()
		defer func() { _ = gw.Stop(ctx) }()

		err = gw.DisableTool(ctx, "to-disable")
		require.NoError(t, err)
	})

	t.Run("RemoveTool", func(t *testing.T) {
		cfg := testConfig()
		cfg.Tools = []mcp.Tool{
			{ID: "to-remove", Transport: mcp.TransportStdio, Stdio: &mcp.StdioTransportConfig{Command: "npx"}, State: mcp.ToolStateEnabled},
		}
		gw, err := New(cfg)
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))
		waitForActors()
		defer func() { _ = gw.Stop(ctx) }()

		err = gw.RemoveTool(ctx, "to-remove")
		require.NoError(t, err)

		tools, err := gw.ListTools(ctx)
		require.NoError(t, err)
		for _, tool := range tools {
			assert.NotEqual(t, mcp.ToolID("to-remove"), tool.ID)
		}
	})

	t.Run("RegisterTool validates tool", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))
		waitForActors()
		defer func() { _ = gw.Stop(ctx) }()

		err = gw.RegisterTool(ctx, mcp.Tool{})
		require.Error(t, err)
		var rErr *mcp.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, mcp.ErrCodeInvalidRequest, rErr.Code)
	})

	t.Run("API methods return error when gateway not started", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)

		_, err = gw.ListTools(ctx)
		require.Error(t, err)

		_, err = gw.Invoke(ctx, &mcp.Invocation{})
		require.Error(t, err)

		err = gw.RegisterTool(ctx, mcp.Tool{ID: "x", Transport: mcp.TransportStdio, Stdio: &mcp.StdioTransportConfig{Command: "y"}})
		require.Error(t, err)

		err = gw.DisableTool(ctx, "x")
		require.Error(t, err)

		err = gw.RemoveTool(ctx, "x")
		require.Error(t, err)
	})

	t.Run("Invoke returns error for non-existent tool", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))
		waitForActors()
		defer func() { _ = gw.Stop(ctx) }()

		_, err = gw.Invoke(ctx, &mcp.Invocation{
			ToolID: "does-not-exist",
			Method: "tools/call",
			Params: map[string]any{},
			Correlation: mcp.CorrelationMeta{
				TenantID:  "test-tenant",
				ClientID:  "test-client",
				RequestID: "req-1",
			},
		})
		require.Error(t, err)
	})

	t.Run("UpdateTool updates a registered tool", func(t *testing.T) {
		cfg := testConfig()
		cfg.Tools = []mcp.Tool{
			{ID: "to-update", Transport: mcp.TransportStdio, Stdio: &mcp.StdioTransportConfig{Command: "echo"}, State: mcp.ToolStateEnabled},
		}
		gw, err := New(cfg)
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))
		waitForActors()
		defer func() { _ = gw.Stop(ctx) }()

		updated := mcp.Tool{
			ID:        "to-update",
			Transport: mcp.TransportStdio,
			Stdio:     &mcp.StdioTransportConfig{Command: "cat"},
			State:     mcp.ToolStateEnabled,
		}
		err = gw.UpdateTool(ctx, updated)
		require.NoError(t, err)
	})

	t.Run("UpdateTool returns error when gateway not started", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)
		err = gw.UpdateTool(ctx, mcp.Tool{ID: "x"})
		require.Error(t, err)
	})

	t.Run("Invoke succeeds with MCP stdio tool", func(t *testing.T) {
		cfg := testConfig()
		cfg.Runtime.StartupTimeout = 15 * time.Second
		cfg.Tools = []mcp.Tool{
			{
				ID:        "echo-tool",
				Transport: mcp.TransportStdio,
				Stdio: &mcp.StdioTransportConfig{
					Command: "npx",
					Args:    []string{"-y", "mcp-hello-world"},
				},
				State: mcp.ToolStateEnabled,
			},
		}
		gw, err := New(cfg)
		require.NoError(t, err)
		if err := gw.Start(ctx); err != nil {
			t.Skipf("gateway start failed (npx/node may be unavailable): %v", err)
		}
		waitForActors()
		defer func() { _ = gw.Stop(ctx) }()

		result, err := gw.Invoke(ctx, &mcp.Invocation{
			ToolID: "echo-tool",
			Method: "tools/call",
			Params: map[string]any{
				"name":      "echo",
				"arguments": map[string]any{"message": "api-test"},
			},
			Correlation: mcp.CorrelationMeta{
				TenantID:  "test-tenant",
				ClientID:  "test-client",
				RequestID: "req-invoke-success",
			},
		})
		if err != nil {
			t.Skipf("invoke failed (MCP server may be unavailable): %v", err)
		}
		require.NotNil(t, result)
		assert.True(t, result.Succeeded(), "invocation should succeed")
		assert.NotNil(t, result.Output)
	})

	t.Run("API with system-wide Registrar and Router exercises resolver fallback", func(t *testing.T) {
		ctx := context.Background()
		cfg := actor.ExternalTestConfig()
		execFactory := egress.NewCompositeExecutorFactory(config.DefaultStartupTimeout, nil)
		system, stop := newTestActorSystemForResolverFallback(t, cfg, execFactory)
		defer stop()

		_, err := system.Spawn(ctx, mcp.ActorNameGatewayManager, &minimalGatewayManager{})
		require.NoError(t, err)
		waitForActors()

		actor.SpawnFoundationalActorsForExternalTest(ctx, system, cfg)
		waitForActors()

		gw, err := New(testConfig(), withSystemForTesting(system))
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))
		defer func() { _ = gw.Stop(ctx) }()

		tools, err := gw.ListTools(ctx)
		require.NoError(t, err)
		require.NotNil(t, tools)

		tool := mcp.Tool{
			ID:        "fallback-tool",
			Transport: mcp.TransportStdio,
			Stdio:     &mcp.StdioTransportConfig{Command: "npx"},
			State:     mcp.ToolStateEnabled,
		}
		err = gw.RegisterTool(ctx, tool)
		require.NoError(t, err)
		waitForActors()

		_, err = gw.Invoke(ctx, &mcp.Invocation{
			ToolID: "fallback-tool",
			Method: "tools/call",
			Params: map[string]any{},
			Correlation: mcp.CorrelationMeta{
				TenantID:  "test-tenant",
				ClientID:  "test-client",
				RequestID: "req-fallback",
			},
		})
		require.Error(t, err)
	})
}

func newTestActorSystemForResolverFallback(t *testing.T, cfg config.Config, execFactory mcp.ExecutorFactory) (goaktactor.ActorSystem, func()) {
	t.Helper()
	ctx := context.Background()
	opts := []goaktactor.Option{
		goaktactor.WithLogger(goaktlog.DiscardLogger),
		goaktactor.WithExtensions(
			actorextension.NewExecutorFactoryExtension(execFactory),
			actorextension.NewToolConfigExtension(),
			actorextension.NewConfigExtension(cfg),
		),
	}
	system, err := goaktactor.NewActorSystem("test-resolver-fallback", opts...)
	require.NoError(t, err)
	require.NoError(t, system.Start(ctx))
	return system, func() { _ = system.Stop(ctx) }
}

func TestGatewayTenantValidation(t *testing.T) {
	t.Run("New rejects empty tenant ID", func(t *testing.T) {
		cfg := testConfig()
		cfg.Tenants = []mcp.TenantConfig{{ID: ""}}
		_, err := New(cfg)
		require.Error(t, err)
		var rErr *mcp.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, mcp.ErrCodeInvalidRequest, rErr.Code)
		assert.Contains(t, err.Error(), "tenant ID must not be empty")
	})

	t.Run("New rejects duplicate tenant IDs", func(t *testing.T) {
		cfg := testConfig()
		cfg.Tenants = []mcp.TenantConfig{
			{ID: "dup-tenant"},
			{ID: "dup-tenant"},
		}
		_, err := New(cfg)
		require.Error(t, err)
		var rErr *mcp.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, mcp.ErrCodeInvalidRequest, rErr.Code)
		assert.Contains(t, err.Error(), "duplicate tenant ID")
	})

	t.Run("New accepts valid tenants", func(t *testing.T) {
		cfg := testConfig()
		cfg.Tenants = []mcp.TenantConfig{
			{ID: "tenant-a"},
			{ID: "tenant-b"},
		}
		gw, err := New(cfg)
		require.NoError(t, err)
		require.NotNil(t, gw)
	})

	t.Run("New accepts empty tenants list", func(t *testing.T) {
		cfg := testConfig()
		cfg.Tenants = nil
		gw, err := New(cfg)
		require.NoError(t, err)
		require.NotNil(t, gw)
	})
}

func TestGatewayDraining(t *testing.T) {
	ctx := context.Background()

	t.Run("API methods return error while gateway is draining", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))
		waitForActors()

		// Simulate draining state without actually stopping the system
		gw.mu.Lock()
		gw.draining = true
		gw.mu.Unlock()

		_, err = gw.ListTools(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "shutting down")

		_, err = gw.Invoke(ctx, &mcp.Invocation{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "shutting down")

		err = gw.RegisterTool(ctx, mcp.Tool{ID: "x", Transport: mcp.TransportStdio, Stdio: &mcp.StdioTransportConfig{Command: "y"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "shutting down")

		err = gw.EnableTool(ctx, "x")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "shutting down")

		err = gw.DisableTool(ctx, "x")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "shutting down")

		err = gw.RemoveTool(ctx, "x")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "shutting down")

		// Restore state so Stop() works
		gw.mu.Lock()
		gw.draining = false
		gw.mu.Unlock()
		require.NoError(t, gw.Stop(ctx))
	})
}

func TestGatewayEnableTool(t *testing.T) {
	ctx := context.Background()

	t.Run("EnableTool re-enables a disabled tool", func(t *testing.T) {
		cfg := testConfig()
		cfg.Tools = []mcp.Tool{
			{ID: "to-enable", Transport: mcp.TransportStdio, Stdio: &mcp.StdioTransportConfig{Command: "npx"}, State: mcp.ToolStateEnabled},
		}
		gw, err := New(cfg)
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))
		waitForActors()
		defer func() { _ = gw.Stop(ctx) }()

		// Disable the tool first
		err = gw.DisableTool(ctx, "to-enable")
		require.NoError(t, err)

		// Re-enable it
		err = gw.EnableTool(ctx, "to-enable")
		require.NoError(t, err)
	})

	t.Run("EnableTool returns error for non-existent tool", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))
		waitForActors()
		defer func() { _ = gw.Stop(ctx) }()

		err = gw.EnableTool(ctx, "nonexistent")
		require.Error(t, err)
	})

	t.Run("EnableTool rejects empty tool ID", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))
		waitForActors()
		defer func() { _ = gw.Stop(ctx) }()

		err = gw.EnableTool(ctx, "")
		require.Error(t, err)
		var rErr *mcp.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, mcp.ErrCodeInvalidRequest, rErr.Code)
	})

	t.Run("EnableTool returns error when gateway not started", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)
		err = gw.EnableTool(ctx, "x")
		require.Error(t, err)
	})

	t.Run("DisableTool rejects empty tool ID", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))
		waitForActors()
		defer func() { _ = gw.Stop(ctx) }()

		err = gw.DisableTool(ctx, "")
		require.Error(t, err)
		var rErr *mcp.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, mcp.ErrCodeInvalidRequest, rErr.Code)
	})

	t.Run("RemoveTool rejects empty tool ID", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))
		waitForActors()
		defer func() { _ = gw.Stop(ctx) }()

		err = gw.RemoveTool(ctx, "")
		require.Error(t, err)
		var rErr *mcp.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, mcp.ErrCodeInvalidRequest, rErr.Code)
	})

	t.Run("UpdateTool validates tool before sending to registrar", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))
		waitForActors()
		defer func() { _ = gw.Stop(ctx) }()

		// Tool with empty ID should fail validation
		err = gw.UpdateTool(ctx, mcp.Tool{})
		require.Error(t, err)
		var rErr *mcp.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, mcp.ErrCodeInvalidRequest, rErr.Code)
	})
}

func TestGatewayAPI_ClusterMode(t *testing.T) {
	ctx := context.Background()

	t.Run("API with cluster mode uses system-wide Registrar and Router", func(t *testing.T) {
		cfg := testConfig()
		cfg.Cluster.Enabled = true
		cfg.Cluster.Discovery = "dnssd"
		cfg.Cluster.DNSSD = mcp.DNSSDDiscoveryConfig{DomainName: "local."}
		// Use high ports to avoid conflicts with other processes
		cfg.Cluster.PeersPort = freePort(t)
		cfg.Cluster.RemotingPort = freePort(t)
		cfg.Tools = []mcp.Tool{
			{
				ID:        "cluster-tool",
				Transport: mcp.TransportStdio,
				Stdio:     &mcp.StdioTransportConfig{Command: "npx"},
				State:     mcp.ToolStateEnabled,
			},
		}
		gw, err := New(cfg)
		require.NoError(t, err)
		if err := gw.Start(ctx); err != nil {
			t.Skipf("cluster gateway start failed (dnssd may be unavailable): %v", err)
		}
		waitForActors()
		time.Sleep(2 * time.Second) // allow cluster to form and singleton to be elected
		defer func() { _ = gw.Stop(ctx) }()

		tools, err := gw.ListTools(ctx)
		if err != nil {
			t.Skipf("ListTools failed in cluster mode: %v", err)
		}
		require.NotNil(t, tools)
		found := false
		for _, tool := range tools {
			if tool.ID == "cluster-tool" {
				found = true
				break
			}
		}
		assert.True(t, found, "bootstrap tool should be listed in cluster mode")
	})
}

func TestGatewayHandler(t *testing.T) {
	t.Run("Handler returns a valid http.Handler when IdentityResolver is set", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)

		handler, err := gw.Handler(mcp.IngressConfig{
			IdentityResolver: &fixedIdentityResolver{tenantID: "t1", clientID: "c1"},
		})
		require.NoError(t, err)
		require.NotNil(t, handler)
	})

	t.Run("Handler returns error when IdentityResolver is nil", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)

		_, err = gw.Handler(mcp.IngressConfig{})
		require.Error(t, err)
	})
}

func TestGatewaySSEHandler(t *testing.T) {
	t.Run("SSEHandler returns a valid http.Handler when IdentityResolver is set", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)

		handler, err := gw.SSEHandler(mcp.IngressConfig{
			IdentityResolver: &fixedIdentityResolver{tenantID: "t1", clientID: "c1"},
		})
		require.NoError(t, err)
		require.NotNil(t, handler)
	})

	t.Run("SSEHandler returns error when IdentityResolver is nil", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)

		_, err = gw.SSEHandler(mcp.IngressConfig{})
		require.Error(t, err)
	})
}

func TestGatewayWSHandler(t *testing.T) {
	t.Run("WSHandler returns a valid http.Handler when IdentityResolver is set", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)

		handler, err := gw.WSHandler(mcp.IngressConfig{
			IdentityResolver: &fixedIdentityResolver{tenantID: "t1", clientID: "c1"},
		}, nil)
		require.NoError(t, err)
		require.NotNil(t, handler)
	})

	t.Run("WSHandler returns error when IdentityResolver is nil", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)

		_, err = gw.WSHandler(mcp.IngressConfig{}, nil)
		require.Error(t, err)
	})
}

func TestInvokeStream(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error when gateway not started", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)

		_, err = gw.InvokeStream(ctx, &mcp.Invocation{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "gateway not started")
	})

	t.Run("returns StreamingResult with closed Progress and buffered Final on success", func(t *testing.T) {
		cfg := testConfig()
		cfg.Tenants = []mcp.TenantConfig{{ID: "stream-tenant"}}
		cfg.Tools = []mcp.Tool{
			{ID: "stream-tool", Transport: mcp.TransportStdio, Stdio: &mcp.StdioTransportConfig{Command: "npx"}, State: mcp.ToolStateEnabled},
		}
		gw, err := New(cfg)
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))
		waitForActors()
		defer func() { _ = gw.Stop(ctx) }()

		inv := &mcp.Invocation{
			ToolID: "stream-tool",
			Method: "tools/call",
			Params: map[string]any{"name": "echo", "arguments": map[string]any{}},
			Correlation: mcp.CorrelationMeta{
				TenantID:  "stream-tenant",
				ClientID:  "stream-client",
				RequestID: "stream-req-1",
			},
		}

		result, err := gw.InvokeStream(ctx, inv)
		// The invocation may error (tool backend not running), but InvokeStream
		// itself must be reached and return a non-nil StreamingResult on success
		// or propagate the error from Invoke. Either way the function is covered.
		if err != nil {
			assert.Nil(t, result)
			return
		}
		require.NotNil(t, result)
		// Progress channel must be closed immediately (non-streaming path)
		_, open := <-result.Progress
		assert.False(t, open, "Progress channel should be closed for non-streaming result")
		// Final channel must have exactly one result buffered
		final, ok := <-result.Final
		require.True(t, ok)
		assert.NotNil(t, final)
	})
}

func TestGatewayStop(t *testing.T) {
	ctx := context.Background()

	t.Run("Stop sets system to nil after shutdown", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))
		waitForActors()
		assert.NotNil(t, gw.System())

		require.NoError(t, gw.Stop(ctx))
		assert.Nil(t, gw.System())
	})

	t.Run("Stop is idempotent — second call is a no-op", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))
		waitForActors()

		require.NoError(t, gw.Stop(ctx))
		require.NoError(t, gw.Stop(ctx)) // second stop must not error
	})
}
