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

package main

import (
	"context"
	"fmt"
	"net"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// KubernetesDiscovery implements [mcp.DiscoveryProvider] directly against the
// Kubernetes API using k8s.io/client-go.
//
// It deliberately does NOT wrap GoAkt's kubernetes.Discovery. GoAkt's type owns
// its own Initialize/Register/Deregister/Close state machine; driving that
// machine from inside mcp.DiscoveryProvider.Start() causes lifecycle conflicts
// with the discoveryAdapter layer (which maps adapter.Initialize → provider.Start
// and treats adapter.Register as a no-op). Implementing the pod-listing logic
// directly avoids any double-invocation or state-machine interference.
//
// Requirements:
//   - Must run inside a Kubernetes pod (uses rest.InClusterConfig).
//   - The pod's ServiceAccount must have RBAC permission to list/get/watch pods
//     in the target namespace (see k8s/gateway.yaml for the ClusterRole definition).
type KubernetesDiscovery struct {
	namespace         string
	discoveryPortName string
	podLabels         map[string]string
	client            kubernetes.Interface
}

// NewKubernetesDiscovery creates a KubernetesDiscovery. The discoveryPortName
// must match the named port declared on the StatefulSet pods (discovery-port).
// podLabels is used as a label selector to filter gateway peers.
func NewKubernetesDiscovery(namespace, discoveryPortName, _, _ string, podLabels map[string]string) *KubernetesDiscovery {
	return &KubernetesDiscovery{
		namespace:         namespace,
		discoveryPortName: discoveryPortName,
		podLabels:         podLabels,
	}
}

// ID returns the provider identifier.
func (d *KubernetesDiscovery) ID() string { return "kubernetes" }

// Start creates the in-cluster Kubernetes client. Returns an error when called
// outside a Kubernetes pod (rest.InClusterConfig fails).
func (d *KubernetesDiscovery) Start(_ context.Context) error {
	if d.namespace == "" {
		return fmt.Errorf("kubernetes discovery: namespace is required")
	}
	if d.discoveryPortName == "" {
		return fmt.Errorf("kubernetes discovery: discoveryPortName is required")
	}
	if len(d.podLabels) == 0 {
		return fmt.Errorf("kubernetes discovery: podLabels are required")
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("kubernetes discovery: get in-cluster config: %w", err)
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("kubernetes discovery: create client: %w", err)
	}

	d.client = client
	return nil
}

// DiscoverPeers queries the Kubernetes API for Running and Ready pods matching
// the configured label selector and returns the discovery-port address of each
// pod as "ip:port" strings.
func (d *KubernetesDiscovery) DiscoverPeers(ctx context.Context) ([]string, error) {
	pods, err := d.client.CoreV1().Pods(d.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(d.podLabels).String(),
	})
	if err != nil {
		return nil, fmt.Errorf("kubernetes discovery: list pods: %w", err)
	}

	var addresses []string

MainLoop:
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		// Skip pods that have a Ready condition set to false.
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status != corev1.ConditionTrue {
				continue MainLoop
			}
		}

		for _, container := range pod.Spec.Containers {
			for _, port := range container.Ports {
				if port.Name == d.discoveryPortName {
					addresses = append(addresses, net.JoinHostPort(
						pod.Status.PodIP,
						strconv.Itoa(int(port.ContainerPort)),
					))
				}
			}
		}
	}

	return addresses, nil
}

// Stop is a no-op; the k8s client has no persistent connections to release.
func (d *KubernetesDiscovery) Stop(_ context.Context) error { return nil }
