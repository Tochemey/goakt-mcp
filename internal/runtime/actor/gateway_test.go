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
	goaktlog "github.com/tochemey/goakt/v4/log"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	"github.com/tochemey/goakt-mcp/internal/runtime/config"
)

func TestNew(t *testing.T) {
	t.Run("returns Gateway with valid inputs", func(t *testing.T) {
		gw, err := New(testConfig(), goaktlog.DiscardLogger)
		require.NoError(t, err)
		require.NotNil(t, gw)
		assert.Nil(t, gw.System(), "system must be nil before Start")
	})

	t.Run("returns error when logger is nil", func(t *testing.T) {
		gw, err := New(testConfig(), nil)
		require.Error(t, err)
		assert.Nil(t, gw)
		var rErr *runtime.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, runtime.ErrCodeInvalidRequest, rErr.Code)
	})
}

func TestGatewayStartStop(t *testing.T) {
	ctx := context.Background()

	t.Run("starts cleanly and exposes the actor system", func(t *testing.T) {
		gw, err := New(testConfig(), goaktlog.DiscardLogger)
		require.NoError(t, err)

		require.NoError(t, gw.Start(ctx))
		assert.NotNil(t, gw.System())

		waitForActors()

		require.NoError(t, gw.Stop(ctx))
	})

	t.Run("stop on unstarted gateway is a no-op", func(t *testing.T) {
		gw, err := New(testConfig(), goaktlog.DiscardLogger)
		require.NoError(t, err)
		assert.NoError(t, gw.Stop(ctx))
	})

	t.Run("GatewayManager is present after start", func(t *testing.T) {
		gw, err := New(testConfig(), goaktlog.DiscardLogger)
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))

		waitForActors()

		pid, err := gw.System().ActorOf(ctx, runtime.ActorNameGatewayManager)
		require.NoError(t, err)
		require.NotNil(t, pid)

		require.NoError(t, gw.Stop(ctx))
	})

	t.Run("foundational actors are children of GatewayManager after start", func(t *testing.T) {
		gw, err := New(testConfig(), goaktlog.DiscardLogger)
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))

		waitForActors()

		managerPID, err := gw.System().ActorOf(ctx, runtime.ActorNameGatewayManager)
		require.NoError(t, err)
		require.NotNil(t, managerPID)

		children := managerPID.Children()
		childNames := make(map[string]bool, len(children))
		for _, c := range children {
			childNames[c.Name()] = true
		}

		assert.True(t, childNames[runtime.ActorNameRegistrar], "RegistryActor must be a child of GatewayManager")
		assert.True(t, childNames[runtime.ActorNameHealth], "HealthActor must be a child of GatewayManager")
		assert.True(t, childNames[runtime.ActorNameJournal], "JournalActor must be a child of GatewayManager")

		require.NoError(t, gw.Stop(ctx))
	})

	t.Run("bootstrap tools from config are loaded into registry", func(t *testing.T) {
		cfg := testConfig()
		cfg.Tools = []config.ToolConfig{
			{
				ID:        runtime.ToolID("bootstrap-tool"),
				Transport: runtime.TransportStdio,
				Command:   "npx",
			},
		}

		gw, err := New(cfg, goaktlog.DiscardLogger)
		require.NoError(t, err)
		require.NoError(t, gw.Start(ctx))

		waitForActors()

		managerPID, err := gw.System().ActorOf(ctx, runtime.ActorNameGatewayManager)
		require.NoError(t, err)
		require.NotNil(t, managerPID)

		registryPID, err := managerPID.Child(runtime.ActorNameRegistrar)
		require.NoError(t, err)
		require.NotNil(t, registryPID)

		resp, err := goaktactor.Ask(ctx, registryPID, &runtime.QueryTool{ToolID: "bootstrap-tool"}, askTimeout)
		require.NoError(t, err)
		qResult, ok := resp.(*runtime.QueryToolResult)
		require.True(t, ok)
		require.True(t, qResult.Found)
		require.NotNil(t, qResult.Tool)
		assert.Equal(t, runtime.ToolID("bootstrap-tool"), qResult.Tool.ID)
		assert.Equal(t, runtime.TransportStdio, qResult.Tool.Transport)

		require.NoError(t, gw.Stop(ctx))
	})

	t.Run("Start returns error when Cluster.Enabled but discovery not configured", func(t *testing.T) {
		cfg := testConfig()
		cfg.Cluster.Enabled = true
		cfg.Cluster.Discovery = "kubernetes"
		cfg.Cluster.Kubernetes = nil // invalid: no kubernetes config

		gw, err := New(cfg, goaktlog.DiscardLogger)
		require.NoError(t, err)

		err = gw.Start(ctx)
		require.Error(t, err)
		var rErr *runtime.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, runtime.ErrCodeInvalidRequest, rErr.Code)
		assert.Contains(t, err.Error(), "cluster is enabled but discovery is not configured")
	})

	t.Run("Start returns error when Cluster.Enabled with dnssd but empty DomainName", func(t *testing.T) {
		cfg := testConfig()
		cfg.Cluster.Enabled = true
		cfg.Cluster.Discovery = "dnssd"
		cfg.Cluster.DNSSD = &config.DNSSDDiscoveryConfig{DomainName: ""} // invalid: empty domain

		gw, err := New(cfg, goaktlog.DiscardLogger)
		require.NoError(t, err)

		err = gw.Start(ctx)
		require.Error(t, err)
		var rErr *runtime.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, runtime.ErrCodeInvalidRequest, rErr.Code)
	})
}
