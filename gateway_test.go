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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goaktlog "github.com/tochemey/goakt/v4/log"

	"github.com/tochemey/goakt-mcp/mcp"
)

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
		gw, err := New(testConfig(), WithLogger(goaktlog.InvalidLevel))
		require.NoError(t, err)
		require.NotNil(t, gw)
		assert.Nil(t, gw.System(), "system must be nil before Start")
	})

	t.Run("WithMetrics enables metrics", func(t *testing.T) {
		gw, err := New(testConfig(), WithLogger(goaktlog.InvalidLevel), WithMetrics())
		require.NoError(t, err)
		require.NotNil(t, gw)
		assert.True(t, gw.metrics)
	})

	t.Run("returns Gateway with default logger when none provided", func(t *testing.T) {
		gw, err := New(testConfig())
		require.NoError(t, err)
		require.NotNil(t, gw)
	})
}

func TestGatewayStartStop(t *testing.T) {
	ctx := context.Background()

	t.Run("starts cleanly and exposes the actor system", func(t *testing.T) {
		gw, err := New(testConfig(), WithLogger(goaktlog.InvalidLevel))
		require.NoError(t, err)

		require.NoError(t, gw.Start(ctx))
		assert.NotNil(t, gw.System())

		waitForActors()

		require.NoError(t, gw.Stop(ctx))
	})

	t.Run("stop on unstarted gateway is a no-op", func(t *testing.T) {
		gw, err := New(testConfig(), WithLogger(goaktlog.InvalidLevel))
		require.NoError(t, err)
		assert.NoError(t, gw.Stop(ctx))
	})

	t.Run("GatewayManager is present after start", func(t *testing.T) {
		gw, err := New(testConfig(), WithLogger(goaktlog.InvalidLevel))
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))

		waitForActors()

		pid, err := gw.System().ActorOf(ctx, mcp.ActorNameGatewayManager)
		require.NoError(t, err)
		require.NotNil(t, pid)

		require.NoError(t, gw.Stop(ctx))
	})

	t.Run("foundational actors are children of GatewayManager after start", func(t *testing.T) {
		gw, err := New(testConfig(), WithLogger(goaktlog.InvalidLevel))
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

		gw, err := New(cfg, WithLogger(goaktlog.InvalidLevel))
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

		gw, err := New(cfg, WithLogger(goaktlog.InvalidLevel))
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

		gw, err := New(cfg, WithLogger(goaktlog.InvalidLevel))
		require.NoError(t, err)

		err = gw.Start(ctx)
		require.Error(t, err)
		var rErr *mcp.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, mcp.ErrCodeInvalidRequest, rErr.Code)
	})

	t.Run("Start with WithMetrics succeeds", func(t *testing.T) {
		gw, err := New(testConfig(), WithLogger(goaktlog.InvalidLevel), WithMetrics())
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))
		waitForActors()
		assert.NotNil(t, gw.System())
		require.NoError(t, gw.Stop(ctx))
	})

	t.Run("Start with WithTracing succeeds", func(t *testing.T) {
		gw, err := New(testConfig(), WithLogger(goaktlog.InvalidLevel), WithTracing())
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
		gw, err := New(testConfig(), WithLogger(goaktlog.InvalidLevel))
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
		gw, err := New(cfg, WithLogger(goaktlog.InvalidLevel))
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
		gw, err := New(cfg, WithLogger(goaktlog.InvalidLevel))
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
		gw, err := New(testConfig(), WithLogger(goaktlog.InvalidLevel))
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
		gw, err := New(testConfig(), WithLogger(goaktlog.InvalidLevel))
		require.NoError(t, err)

		_, err = gw.ListTools(ctx)
		require.Error(t, err)

		_, err = gw.Invoke(ctx, &mcp.Invocation{})
		require.Error(t, err)

		err = gw.RegisterTool(ctx, mcp.Tool{ID: "x", Transport: mcp.TransportStdio, Stdio: &mcp.StdioTransportConfig{Command: "y"}})
		require.Error(t, err)
	})

	t.Run("Invoke returns error for non-existent tool", func(t *testing.T) {
		gw, err := New(testConfig(), WithLogger(goaktlog.InvalidLevel))
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
		gw, err := New(cfg, WithLogger(goaktlog.InvalidLevel))
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
		gw, err := New(testConfig(), WithLogger(goaktlog.InvalidLevel))
		require.NoError(t, err)
		err = gw.UpdateTool(ctx, mcp.Tool{ID: "x"})
		require.Error(t, err)
	})
}
