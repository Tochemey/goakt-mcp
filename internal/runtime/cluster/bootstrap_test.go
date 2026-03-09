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

package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tochemey/goakt-mcp/internal/runtime/config"
)

func TestIsClusterConfigured(t *testing.T) {
	t.Run("returns false when Cluster.Enabled is false", func(t *testing.T) {
		cfg := config.Config{}
		cfg.Cluster.Enabled = false
		cfg.Cluster.Discovery = "dnssd"
		cfg.Cluster.DNSSD = config.DNSSDDiscoveryConfig{DomainName: "goakt-mcp.local"}
		assert.False(t, IsClusterConfigured(cfg))
	})

	t.Run("returns false when Discovery is unsupported", func(t *testing.T) {
		cfg := config.Config{}
		cfg.Cluster.Enabled = true
		cfg.Cluster.Discovery = "consul"
		assert.False(t, IsClusterConfigured(cfg))
	})

	t.Run("returns false when kubernetes config is nil", func(t *testing.T) {
		cfg := config.Config{}
		cfg.Cluster.Enabled = true
		cfg.Cluster.Discovery = "kubernetes"
		cfg.Cluster.Kubernetes = config.KubernetesDiscoveryConfig{}
		assert.False(t, IsClusterConfigured(cfg))
	})

	t.Run("returns false when kubernetes config is incomplete", func(t *testing.T) {
		cfg := config.Config{}
		cfg.Cluster.Enabled = true
		cfg.Cluster.Discovery = "kubernetes"
		cfg.Cluster.Kubernetes = config.KubernetesDiscoveryConfig{
			Namespace: "default",
			// missing DiscoveryPortName, RemotingPortName, PeersPortName, PodLabels
		}
		assert.False(t, IsClusterConfigured(cfg))
	})

	t.Run("returns true when enabled with valid kubernetes discovery", func(t *testing.T) {
		cfg := config.Config{}
		cfg.Cluster.Enabled = true
		cfg.Cluster.Discovery = "kubernetes"
		cfg.Cluster.Kubernetes = config.KubernetesDiscoveryConfig{
			Namespace:         "default",
			DiscoveryPortName: "gossip",
			RemotingPortName:  "remoting",
			PeersPortName:     "cluster",
			PodLabels:         map[string]string{"app": "goakt-mcp"},
		}
		assert.True(t, IsClusterConfigured(cfg))
	})

	t.Run("returns false when dnssd config is nil", func(t *testing.T) {
		cfg := config.Config{}
		cfg.Cluster.Enabled = true
		cfg.Cluster.Discovery = "dnssd"
		cfg.Cluster.DNSSD = config.DNSSDDiscoveryConfig{}
		assert.False(t, IsClusterConfigured(cfg))
	})

	t.Run("returns false when dnssd DomainName is empty", func(t *testing.T) {
		cfg := config.Config{}
		cfg.Cluster.Enabled = true
		cfg.Cluster.Discovery = "dnssd"
		cfg.Cluster.DNSSD = config.DNSSDDiscoveryConfig{DomainName: ""}
		assert.False(t, IsClusterConfigured(cfg))
	})

	t.Run("returns true when enabled with valid dnssd discovery", func(t *testing.T) {
		cfg := config.Config{}
		cfg.Cluster.Enabled = true
		cfg.Cluster.Discovery = "dnssd"
		cfg.Cluster.DNSSD = config.DNSSDDiscoveryConfig{DomainName: "goakt-mcp.default.svc.cluster.local"}
		assert.True(t, IsClusterConfigured(cfg))
	})
}

func TestBuildOptions(t *testing.T) {
	t.Run("returns nil when Cluster.Enabled is false", func(t *testing.T) {
		cfg := config.Config{}
		cfg.Cluster.Enabled = false
		opts := BuildOptions(cfg, nil)
		assert.Nil(t, opts)
	})

	t.Run("returns only WithRemote when enabled but discovery invalid", func(t *testing.T) {
		cfg := config.Config{}
		cfg.Cluster.Enabled = true
		cfg.Cluster.Discovery = "kubernetes"
		cfg.Cluster.Kubernetes = config.KubernetesDiscoveryConfig{}
		opts := BuildOptions(cfg, nil)
		require.NotNil(t, opts)
		// Should have WithRemote but not WithCluster (cluster requires valid discovery)
		assert.GreaterOrEqual(t, len(opts), 1)
	})

	t.Run("returns WithRemote and WithCluster when kubernetes discovery configured", func(t *testing.T) {
		cfg := config.Config{}
		cfg.Cluster.Enabled = true
		cfg.Cluster.Discovery = "kubernetes"
		cfg.Cluster.Kubernetes = config.KubernetesDiscoveryConfig{
			Namespace:         "default",
			DiscoveryPortName: "gossip",
			RemotingPortName:  "remoting",
			PeersPortName:     "cluster",
			PodLabels:         map[string]string{"app": "goakt-mcp"},
		}
		opts := BuildOptions(cfg, nil)
		require.NotNil(t, opts)
		assert.GreaterOrEqual(t, len(opts), 2, "expect WithRemote and WithCluster")
	})

	t.Run("returns WithRemote and WithCluster when dnssd discovery configured", func(t *testing.T) {
		cfg := config.Config{}
		cfg.Cluster.Enabled = true
		cfg.Cluster.Discovery = "dnssd"
		cfg.Cluster.DNSSD = config.DNSSDDiscoveryConfig{DomainName: "goakt-mcp.local"}
		opts := BuildOptions(cfg, nil)
		require.NotNil(t, opts)
		assert.GreaterOrEqual(t, len(opts), 2, "expect WithRemote and WithCluster")
	})
}
