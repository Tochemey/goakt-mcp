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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	goaktactor "github.com/tochemey/goakt/v4/actor"
	goaktlog "github.com/tochemey/goakt/v4/log"
	"github.com/tochemey/goakt/v4/testkit"

	"github.com/tochemey/goakt-mcp/mcp"

	actorextension "github.com/tochemey/goakt-mcp/internal/runtime/actor/extension"
	"github.com/tochemey/goakt-mcp/internal/runtime/audit"
	"github.com/tochemey/goakt-mcp/internal/runtime/config"
)

// askTimeout is the default timeout for Ask calls in tests.
const askTimeout = 5 * time.Second

// validStdioTool returns a valid stdio tool for use in tests.
func validStdioTool(id mcp.ToolID) mcp.Tool {
	return mcp.Tool{
		ID:        id,
		Transport: mcp.TransportStdio,
		Stdio:     &mcp.StdioTransportConfig{Command: "npx"},
		State:     mcp.ToolStateEnabled,
	}
}

// validHTTPTool returns a valid HTTP tool for use in tests.
func validHTTPTool(id mcp.ToolID) mcp.Tool {
	return mcp.Tool{
		ID:        id,
		Transport: mcp.TransportHTTP,
		HTTP:      &mcp.HTTPTransportConfig{URL: "http://localhost:8080"},
		State:     mcp.ToolStateEnabled,
	}
}

// testActorSystem creates and starts a minimal GoAkt actor system for use in tests.
// The returned stop function must be called to clean up the system after the test.
// Uses the discard logger to suppress all log output during tests.
func testActorSystem(t *testing.T, extras ...goaktactor.Option) (goaktactor.ActorSystem, func()) {
	t.Helper()
	ctx := context.Background()
	opts := []goaktactor.Option{
		goaktactor.WithLogger(goaktlog.DiscardLogger),
	}
	opts = append(opts, extras...)
	system, err := goaktactor.NewActorSystem("test-goakt-mcp", opts...)
	require.NoError(t, err)
	require.NoError(t, system.Start(ctx))

	stop := func() {
		require.NoError(t, system.Stop(ctx))
	}
	return system, stop
}

// testActorSystemWithTools creates and starts an actor system pre-loaded with a
// ToolConfigExtension containing the given tools and ConfigExtension for journal/health.
// Convenience wrapper over testActorSystem for supervisor tests.
func testActorSystemWithTools(t *testing.T, tools ...mcp.Tool) (goaktactor.ActorSystem, func()) {
	t.Helper()
	toolCfgExt := actorextension.NewToolConfigExtension()
	for _, tool := range tools {
		toolCfgExt.Register(tool)
	}
	cfg := testConfig()
	cfg.Audit.Sink = audit.NewMemorySink()
	return testActorSystem(t, goaktactor.WithExtensions(toolCfgExt, actorextension.NewConfigExtension(cfg)))
}

// testConfig returns a minimal Config suitable for use in tests.
func testConfig() config.Config {
	return config.Config{
		Runtime: config.RuntimeConfig{
			SessionIdleTimeout: config.DefaultSessionIdleTimeout,
			RequestTimeout:     config.DefaultRequestTimeout,
			StartupTimeout:     config.DefaultStartupTimeout,
		},
		Credentials: config.CredentialsConfig{
			Providers: []mcp.CredentialsProvider{&mockCredentialProvider{creds: map[string]string{"api-key": "test-secret"}}},
			CacheTTL:  mcp.DefaultCredentialTTL,
		},
	}
}

// testConfigWithTenants returns a Config with tenant allowlist for policy tests.
func testConfigWithTenants(tenantIDs ...mcp.TenantID) config.Config {
	cfg := testConfig()
	for _, id := range tenantIDs {
		cfg.Tenants = append(cfg.Tenants, config.TenantConfig{ID: id, Quotas: config.TenantQuotaConfig{}})
	}
	return cfg
}

// waitForActors pauses briefly to allow asynchronous PostStart messages to be
// delivered and processed by newly spawned actors. This is necessary in tests
// that assert on actor state that is set during PostStart handling.
func waitForActors() {
	time.Sleep(100 * time.Millisecond)
}

// spawnFoundationalActorsForTest spawns Registrar, Journal, Policy, CredentialBroker,
// and Router with their canonical names. The system must have ConfigExtension and
// optionally ToolConfigExtension registered. Use for router/registrar tests that
// need the full actor graph.
func spawnFoundationalActorsForTest(ctx context.Context, system goaktactor.ActorSystem, cfg config.Config) {
	system.Spawn(ctx, mcp.ActorNameJournal, newJournaler())
	waitForActors()
	system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
	waitForActors()
	system.Spawn(ctx, mcp.ActorNamePolicy, newPolicyMaker(cfg))
	waitForActors()
	system.Spawn(ctx, mcp.ActorNameCredentialBroker, newCredentialBroker())
	waitForActors()
	system.Spawn(ctx, mcp.ActorNameRouter, newRouterActor())
	waitForActors()
}

// sessionInvocation returns a minimal Invocation for session tests.
func sessionInvocation(toolID mcp.ToolID, tenantID, clientID string) *mcp.Invocation {
	return &mcp.Invocation{
		Correlation: mcp.CorrelationMeta{
			TenantID:  mcp.TenantID(tenantID),
			ClientID:  mcp.ClientID(clientID),
			RequestID: "req-1",
		},
		ToolID: toolID,
		Method: "tools/call",
		Params: map[string]any{},
	}
}

// newTestKit creates a testkit for use in tests. The returned kit is cleaned up via t.Cleanup.
// Optional testkit options (e.g. testkit.WithExtensions) can be passed to configure the system.
func newTestKit(t *testing.T, opts ...testkit.Option) (*testkit.TestKit, context.Context) {
	t.Helper()
	ctx := context.Background()
	kit := testkit.New(ctx, t, opts...)
	t.Cleanup(func() {
		kit.Shutdown(ctx)
	})
	return kit, ctx
}

// failingAuditSink is a test sink that returns an error on Write.
type failingAuditSink struct{}

func (f *failingAuditSink) Write(_ *mcp.AuditEvent) error {
	return errors.New("write failed")
}

func (f *failingAuditSink) Close() error {
	return nil
}

// mockCredentialProvider is a test credential provider for use in tests.
type mockCredentialProvider struct {
	creds map[string]string
	err   error
}

var _ mcp.CredentialsProvider = (*mockCredentialProvider)(nil)

func (m *mockCredentialProvider) ID() string {
	return "mock"
}

func (m *mockCredentialProvider) ResolveCredentials(_ context.Context, _ mcp.TenantID, _ mcp.ToolID) (*mcp.Credentials, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &mcp.Credentials{Values: m.creds}, nil
}
