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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// staticProvider is a minimal DiscoveryProvider implementation for testing.
type staticProvider struct {
	peers []string
}

var _ DiscoveryProvider = (*staticProvider)(nil)

func (s *staticProvider) ID() string                                        { return "static" }
func (s *staticProvider) Start(_ context.Context) error                     { return nil }
func (s *staticProvider) DiscoverPeers(_ context.Context) ([]string, error) { return s.peers, nil }
func (s *staticProvider) Stop(_ context.Context) error                      { return nil }

func TestDiscoveryProvider_InterfaceCompilability(t *testing.T) {
	provider := &staticProvider{peers: []string{"host1:8080", "host2:8080"}}
	ctx := context.Background()

	assert.Equal(t, "static", provider.ID())

	require.NoError(t, provider.Start(ctx))

	peers, err := provider.DiscoverPeers(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"host1:8080", "host2:8080"}, peers)

	require.NoError(t, provider.Stop(ctx))
}

func TestDiscoveryProvider_EmptyPeers(t *testing.T) {
	provider := &staticProvider{}
	ctx := context.Background()

	peers, err := provider.DiscoverPeers(ctx)
	require.NoError(t, err)
	assert.Nil(t, peers)
}
