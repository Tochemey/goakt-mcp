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
	"context"

	"github.com/tochemey/goakt/v4/discovery"

	"github.com/tochemey/goakt-mcp/mcp"
)

// discoveryAdapter wraps an mcp.DiscoveryProvider to satisfy GoAkt's
// discovery.Provider interface. The caller-supplied context is derived into a
// cancellable child and passed to the provider on every call.
type discoveryAdapter struct {
	provider mcp.DiscoveryProvider
	ctx      context.Context
	cancel   context.CancelFunc
}

// Compile-time interface check.
var _ discovery.Provider = (*discoveryAdapter)(nil)

// newDiscoveryAdapter creates an adapter that bridges the simplified
// mcp.DiscoveryProvider to GoAkt's discovery.Provider. The provided context
// controls the adapter's lifetime — when it is cancelled, in-flight provider
// calls are signaled to abort.
func newDiscoveryAdapter(ctx context.Context, provider mcp.DiscoveryProvider) *discoveryAdapter {
	ctx, cancel := context.WithCancel(ctx)
	return &discoveryAdapter{
		provider: provider,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// ID returns the provider name.
func (a *discoveryAdapter) ID() string {
	return a.provider.ID()
}

// Initialize delegates to the provider's Start method.
// GoAkt calls Initialize once during actor system startup.
func (a *discoveryAdapter) Initialize() error {
	return a.provider.Start(a.ctx)
}

// Register is a no-op. The simplified DiscoveryProvider interface merges
// registration into Start.
func (a *discoveryAdapter) Register() error {
	return nil
}

// Deregister is a no-op. The simplified DiscoveryProvider interface merges
// deregistration into Stop.
func (a *discoveryAdapter) Deregister() error {
	return nil
}

// DiscoverPeers delegates to the provider's DiscoverPeers method,
// passing the adapter's context for cancellation and timeout support.
func (a *discoveryAdapter) DiscoverPeers() ([]string, error) {
	return a.provider.DiscoverPeers(a.ctx)
}

// Close cancels the adapter context (aborting any in-flight DiscoverPeers
// calls) and delegates to the provider's Stop method with a fresh
// background context so cleanup work is not prematurely cancelled.
func (a *discoveryAdapter) Close() error {
	a.cancel()
	return a.provider.Stop(context.Background())
}
