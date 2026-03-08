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

	runtimeConfig "github.com/tochemey/goakt-mcp/internal/runtime/config"
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
func IsClusterConfigured(config runtimeConfig.Config) bool {
	if !config.Cluster.Enabled {
		return false
	}
	switch config.Cluster.Discovery {
	case "kubernetes":
		return isKubernetesDiscoveryValid(config.Cluster.Kubernetes)
	case "dnssd":
		return isDNSSDDiscoveryValid(config.Cluster.DNSSD)
	default:
		return false
	}
}

func isKubernetesDiscoveryValid(kubernetesConfig *runtimeConfig.KubernetesDiscoveryConfig) bool {
	if kubernetesConfig == nil {
		return false
	}
	return kubernetesConfig.Namespace != "" &&
		kubernetesConfig.DiscoveryPortName != "" &&
		kubernetesConfig.RemotingPortName != "" &&
		kubernetesConfig.PeersPortName != "" &&
		len(kubernetesConfig.PodLabels) > 0
}

func isDNSSDDiscoveryValid(dnssdConfig *runtimeConfig.DNSSDDiscoveryConfig) bool {
	if dnssdConfig == nil {
		return false
	}
	return dnssdConfig.DomainName != ""
}

// BuildOptions returns actor system options for cluster mode when enabled.
//
// When cfg.Cluster.Enabled is false, returns nil (single-node, no cluster options).
// When enabled with valid kubernetes or dnssd discovery, builds ClusterConfig and
// RemoteConfig. remoteOpts are forwarded to [remote.NewConfig] (e.g., serializer
// registrations via [remote.WithSerializables]). kinds are the actor instances to
// register for cluster (e.g., RegistryActor).
func BuildOptions(config runtimeConfig.Config, remoteOpts []remote.Option, kinds ...goaktactor.Actor) []goaktactor.Option {
	if !config.Cluster.Enabled {
		return nil
	}

	clConfig := config.Cluster
	peersPort := clConfig.PeersPort
	if peersPort <= 0 {
		peersPort = defaultPeersPort
	}
	remotingPort := clConfig.RemotingPort
	if remotingPort <= 0 {
		remotingPort = defaultRemotingPort
	}

	var opts []goaktactor.Option

	remoteCfg := remote.NewConfig("0.0.0.0", remotingPort, remoteOpts...)
	opts = append(opts, goaktactor.WithRemote(remoteCfg))

	var discoveryProvider discovery.Provider
	switch clConfig.Discovery {
	case "kubernetes":
		discoveryProvider = buildKubernetesDiscovery(clConfig.Kubernetes)
	case "dnssd":
		discoveryProvider = buildDNSSDDiscovery(clConfig.DNSSD)
	}

	if discoveryProvider != nil {
		clusterConfig := goaktactor.NewClusterConfig().
			WithDiscovery(discoveryProvider).
			WithPeersPort(peersPort).
			WithDiscoveryPort(peersPort).
			WithBootstrapTimeout(10 * time.Second).
			WithClusterBalancerInterval(time.Second).
			WithPartitionCount(20).
			WithMinimumPeersQuorum(1).
			WithReplicaCount(1)

		if len(kinds) > 0 {
			clusterConfig = clusterConfig.WithKinds(kinds...)
		}

		if clConfig.SingletonRole != "" {
			clusterConfig = clusterConfig.WithRoles(clConfig.SingletonRole)
		}
		opts = append(opts, goaktactor.WithCluster(clusterConfig))
	}

	return opts
}

func buildKubernetesDiscovery(k *runtimeConfig.KubernetesDiscoveryConfig) *kubernetes.Discovery {
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

func buildDNSSDDiscovery(d *runtimeConfig.DNSSDDiscoveryConfig) *dnssd.Discovery {
	if d == nil || !isDNSSDDiscoveryValid(d) {
		return nil
	}
	ipv6 := d.IPv6
	return dnssd.NewDiscovery(&dnssd.Config{
		DomainName: d.DomainName,
		IPv6:       &ipv6,
	})
}
