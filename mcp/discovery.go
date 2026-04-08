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

package mcp

import (
	"context"
)

// DiscoveryProvider is the abstraction for cluster peer discovery.
//
// Implementations must be safe for concurrent use. The gateway calls Start once
// before any calls to DiscoverPeers, and Stop once during graceful shutdown.
type DiscoveryProvider interface {
	// ID returns a unique, human-readable name for this provider (e.g., "consul", "static").
	ID() string

	// Start initializes the provider and registers the current node with the
	// discovery backend. Called once when the gateway enters cluster mode.
	// The provided context is cancelled on shutdown.
	Start(ctx context.Context) error

	// DiscoverPeers returns the current list of peer addresses.
	// Each entry must be a "host:port" string where host is an IP address or
	// hostname and port is the discovery port (e.g., "192.168.1.10:7946",
	// "[::1]:7946"). Use [net.JoinHostPort] to build entries correctly,
	// especially for IPv6 addresses.
	// Called periodically by the cluster subsystem to refresh the peer set.
	DiscoverPeers(ctx context.Context) ([]string, error)

	// Stop deregisters the current node and releases resources.
	// Called once during gateway shutdown. No further calls to DiscoverPeers
	// will be made after Stop returns.
	Stop(ctx context.Context) error
}
