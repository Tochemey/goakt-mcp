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

// Package cluster provides cluster bootstrap and configuration for multi-node
// runtime deployment. When Cluster.Enabled is true, the runtime uses GoAkt
// clustering with the configured discovery backend.
//
// Supported discovery providers:
//   - kubernetes: for production and cloud platforms (in-cluster pod discovery)
//   - dnssd: for local development (DNS-based service discovery)
package cluster

import (
	"time"

	goaktactor "github.com/tochemey/goakt/v4/actor"
	"github.com/tochemey/goakt/v4/discovery"
	"github.com/tochemey/goakt/v4/discovery/dnssd"
	"github.com/tochemey/goakt/v4/discovery/kubernetes"
	"github.com/tochemey/goakt/v4/remote"

	"github.com/tochemey/goakt-mcp/internal/runtime/config"
)

const (
	defaultPeersPort    = 15000
	defaultRemotingPort = 15001
)

// IsClusterConfigured returns true when cluster mode is enabled and the discovery
// configuration is valid. Use this to decide whether to use SpawnSingleton (cluster)
// vs Spawn (single-node) for actors like the registry.
//
// Returns true only when Cluster.Enabled and discovery is properly configured
// (kubernetes with valid config, or dnssd with non-empty DomainName).
func IsClusterConfigured(cfg config.Config) bool {
	if !cfg.Cluster.Enabled {
		return false
	}
	switch cfg.Cluster.Discovery {
	case "kubernetes":
		return isKubernetesDiscoveryValid(cfg.Cluster.Kubernetes)
	case "dnssd":
		return isDNSSDDiscoveryValid(cfg.Cluster.DNSSD)
	default:
		return false
	}
}

func isKubernetesDiscoveryValid(k *config.KubernetesDiscoveryConfig) bool {
	if k == nil {
		return false
	}
	return k.Namespace != "" &&
		k.DiscoveryPortName != "" &&
		k.RemotingPortName != "" &&
		k.PeersPortName != "" &&
		len(k.PodLabels) > 0
}

func isDNSSDDiscoveryValid(d *config.DNSSDDiscoveryConfig) bool {
	if d == nil {
		return false
	}
	return d.DomainName != ""
}

// BuildOptions returns actor system options for cluster mode when enabled.
//
// When cfg.Cluster.Enabled is false, returns nil (single-node, no cluster options).
// When enabled with valid kubernetes or dnssd discovery, builds ClusterConfig and
// RemoteConfig. kinds are the actor instances to register for cluster (e.g., RegistryActor).
func BuildOptions(cfg config.Config, kinds ...goaktactor.Actor) []goaktactor.Option {
	if !cfg.Cluster.Enabled {
		return nil
	}
	cl := cfg.Cluster
	peersPort := cl.PeersPort
	if peersPort <= 0 {
		peersPort = defaultPeersPort
	}
	remotingPort := cl.RemotingPort
	if remotingPort <= 0 {
		remotingPort = defaultRemotingPort
	}

	var opts []goaktactor.Option

	remoteCfg := remote.NewConfig("0.0.0.0", remotingPort)
	opts = append(opts, goaktactor.WithRemote(remoteCfg))

	var disc discovery.Provider
	switch cl.Discovery {
	case "kubernetes":
		disc = buildKubernetesDiscovery(cl.Kubernetes)
	case "dnssd":
		disc = buildDNSSDDiscovery(cl.DNSSD)
	}

	if disc != nil {
		clusterCfg := goaktactor.NewClusterConfig().
			WithDiscovery(disc).
			WithPeersPort(peersPort).
			WithDiscoveryPort(peersPort).
			WithBootstrapTimeout(10 * time.Second)
		if len(kinds) > 0 {
			clusterCfg = clusterCfg.WithKinds(kinds...)
		}
		if cl.SingletonRole != "" {
			clusterCfg = clusterCfg.WithRoles(cl.SingletonRole)
		}
		opts = append(opts, goaktactor.WithCluster(clusterCfg))
	}

	return opts
}

func buildKubernetesDiscovery(k *config.KubernetesDiscoveryConfig) *kubernetes.Discovery {
	if k == nil || !isKubernetesDiscoveryValid(k) {
		return nil
	}
	return kubernetes.NewDiscovery(&kubernetes.Config{
		Namespace:         k.Namespace,
		DiscoveryPortName: k.DiscoveryPortName,
		RemotingPortName:  k.RemotingPortName,
		PeersPortName:     k.PeersPortName,
		PodLabels:         k.PodLabels,
	})
}

func buildDNSSDDiscovery(d *config.DNSSDDiscoveryConfig) *dnssd.Discovery {
	if d == nil || !isDNSSDDiscoveryValid(d) {
		return nil
	}
	ipv6 := d.IPv6
	return dnssd.NewDiscovery(&dnssd.Config{
		DomainName: d.DomainName,
		IPv6:       &ipv6,
	})
}
