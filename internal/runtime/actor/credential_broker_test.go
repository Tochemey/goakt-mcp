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
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goaktactor "github.com/tochemey/goakt/v4/actor"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	"github.com/tochemey/goakt-mcp/internal/runtime/credentials"
)

func TestCredentialBrokerActor(t *testing.T) {
	ctx := context.Background()

	t.Run("resolves credentials from env provider", func(t *testing.T) {
		os.Setenv("MCP_CRED_CRED_TOOL_API_KEY", "test-secret")
		defer os.Unsetenv("MCP_CRED_CRED_TOOL_API_KEY")

		system, stop := testActorSystem(t)
		defer stop()

		providers := []credentials.Provider{credentials.NewEnvProvider()}
		pid, err := system.Spawn(ctx, runtime.ActorNameCredentialBroker, newCredentialBroker(providers, credentials.DefaultCredentialTTL))
		require.NoError(t, err)
		waitForActors()

		resp, err := goaktactor.Ask(ctx, pid, &credentials.ResolveRequest{
			TenantID: "default",
			ToolID:   "cred-tool",
		}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*credentials.ResolveResult)
		require.True(t, ok)
		require.True(t, result.Resolved())
		assert.Equal(t, "test-secret", result.Credentials["api-key"])
	})

	t.Run("returns ErrCredentialUnavailable when no credentials", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		providers := []credentials.Provider{credentials.NewEnvProvider()}
		pid, err := system.Spawn(ctx, runtime.ActorNameCredentialBroker, newCredentialBroker(providers, credentials.DefaultCredentialTTL))
		require.NoError(t, err)
		waitForActors()

		resp, err := goaktactor.Ask(ctx, pid, &credentials.ResolveRequest{
			TenantID: "default",
			ToolID:   "no-creds-tool",
		}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*credentials.ResolveResult)
		require.True(t, ok)
		assert.False(t, result.Resolved())
		require.Error(t, result.Err)
		var rErr *runtime.RuntimeError
		require.True(t, assert.ErrorAs(t, result.Err, &rErr))
		assert.Equal(t, runtime.ErrCodeCredentialUnavailable, rErr.Code)
	})

	t.Run("resolves from provider and caches", func(t *testing.T) {
		kit, ctx := newTestKit(t)

		provider := &mockCredentialProvider{creds: map[string]string{"api_key": "secret"}}
		broker := newCredentialBroker([]credentials.Provider{provider}, time.Minute)
		kit.ActorSystem().Spawn(ctx, "broker-cache", broker)
		waitForActors()

		probe := kit.NewProbe(ctx)
		probe.SendSync("broker-cache", &credentials.ResolveRequest{
			TenantID: "tenant-1",
			ToolID:   "tool-1",
		}, askTimeout)
		resp := probe.ExpectAnyMessage()
		result, ok := resp.(*credentials.ResolveResult)
		require.True(t, ok)
		require.NoError(t, result.Err)
		assert.Equal(t, "secret", result.Credentials["api_key"])

		probe.SendSync("broker-cache", &credentials.ResolveRequest{
			TenantID: "tenant-1",
			ToolID:   "tool-1",
		}, askTimeout)
		resp2 := probe.ExpectAnyMessage()
		result2, ok := resp2.(*credentials.ResolveResult)
		require.True(t, ok)
		require.NoError(t, result2.Err)
		assert.Equal(t, "secret", result2.Credentials["api_key"])
		probe.Stop()
	})

	t.Run("returns ErrCredentialUnavailable when no provider has credentials", func(t *testing.T) {
		kit, ctx := newTestKit(t)

		provider := &mockCredentialProvider{creds: nil}
		broker := newCredentialBroker([]credentials.Provider{provider}, time.Minute)
		kit.ActorSystem().Spawn(ctx, "broker-empty", broker)
		waitForActors()

		probe := kit.NewProbe(ctx)
		probe.SendSync("broker-empty", &credentials.ResolveRequest{
			TenantID: "tenant-1",
			ToolID:   "tool-1",
		}, askTimeout)
		resp := probe.ExpectAnyMessage()
		result, ok := resp.(*credentials.ResolveResult)
		require.True(t, ok)
		require.Error(t, result.Err)
		assert.Equal(t, runtime.ErrCodeCredentialUnavailable, result.Err.(*runtime.RuntimeError).Code)
		probe.Stop()
	})

	t.Run("uses default cache TTL when zero", func(t *testing.T) {
		kit, ctx := newTestKit(t)

		provider := &mockCredentialProvider{creds: map[string]string{"k": "v"}}
		broker := newCredentialBroker([]credentials.Provider{provider}, 0)
		kit.ActorSystem().Spawn(ctx, "broker-default-ttl", broker)
		waitForActors()

		probe := kit.NewProbe(ctx)
		probe.SendSync("broker-default-ttl", &credentials.ResolveRequest{
			TenantID: "t1",
			ToolID:   "tool1",
		}, askTimeout)
		resp := probe.ExpectAnyMessage()
		result, ok := resp.(*credentials.ResolveResult)
		require.True(t, ok)
		require.NoError(t, result.Err)
		assert.Equal(t, "v", result.Credentials["k"])
		probe.Stop()
	})

	t.Run("skips provider that returns error and uses next provider", func(t *testing.T) {
		kit, ctx := newTestKit(t)

		failingProvider := &mockCredentialProvider{err: errors.New("provider failed")}
		workingProvider := &mockCredentialProvider{creds: map[string]string{"key": "val"}}
		broker := newCredentialBroker([]credentials.Provider{failingProvider, workingProvider}, time.Minute)
		kit.ActorSystem().Spawn(ctx, "broker-fail-then-ok", broker)
		waitForActors()

		probe := kit.NewProbe(ctx)
		probe.SendSync("broker-fail-then-ok", &credentials.ResolveRequest{
			TenantID: "tenant-1",
			ToolID:   "tool-1",
		}, askTimeout)
		resp := probe.ExpectAnyMessage()
		result, ok := resp.(*credentials.ResolveResult)
		require.True(t, ok)
		require.NoError(t, result.Err)
		assert.Equal(t, "val", result.Credentials["key"])
		probe.Stop()
	})

	t.Run("unhandles unknown message", func(t *testing.T) {
		kit, ctx := newTestKit(t)

		provider := &mockCredentialProvider{creds: map[string]string{"k": "v"}}
		broker := newCredentialBroker([]credentials.Provider{provider}, time.Minute)
		pid, err := kit.ActorSystem().Spawn(ctx, "broker-unknown", broker)
		require.NoError(t, err)
		require.NoError(t, pid.Tell(ctx, pid, "unknown"))
		waitForActors()
	})
}
