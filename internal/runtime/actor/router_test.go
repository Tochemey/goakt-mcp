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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goaktactor "github.com/tochemey/goakt/v4/actor"
	"github.com/tochemey/goakt/v4/testkit"
	noopmetric "go.opentelemetry.io/otel/metric/noop"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	actorextension "github.com/tochemey/goakt-mcp/internal/runtime/actor/extension"
	"github.com/tochemey/goakt-mcp/internal/runtime/audit"
	"github.com/tochemey/goakt-mcp/internal/runtime/config"
	"github.com/tochemey/goakt-mcp/internal/runtime/credentials"
	"github.com/tochemey/goakt-mcp/internal/runtime/telemetry"
	"github.com/tochemey/goakt-mcp/mcp"
)

func TestRouterActor(t *testing.T) {
	ctx := context.Background()

	t.Run("successful route and execute", func(t *testing.T) {
		system, stop := testActorSystem(t,
			goaktactor.WithExtensions(actorextension.NewToolConfigExtension()),
		)
		defer stop()

		_, err := system.Spawn(ctx, mcp.ActorNameJournal, newJournaler(audit.NewMemorySink()))
		require.NoError(t, err)

		registryPID, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		tool := validStdioTool("route-tool")
		_, err = goaktactor.Ask(ctx, registryPID, &runtime.RegisterTool{Tool: tool}, askTimeout)
		require.NoError(t, err)
		waitForActors()

		routerPID, err := system.Spawn(ctx, mcp.ActorNameRouter, newRouterActor(registryPID, nil, nil, nil))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation("route-tool", "tenant1", "client1")
		resp, err := goaktactor.Ask(ctx, routerPID, &runtime.RouteInvocation{Invocation: inv}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.RouteResult)
		require.True(t, ok)
		require.NoError(t, result.Err)
		require.NotNil(t, result.Result)
		assert.Equal(t, mcp.ExecutionStatusSuccess, result.Result.Status)
		assert.Equal(t, "tenant1", string(result.Result.Correlation.TenantID))
		assert.Equal(t, "client1", string(result.Result.Correlation.ClientID))
	})

	t.Run("tool not found", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		registryPID, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		routerPID, err := system.Spawn(ctx, mcp.ActorNameRouter, newRouterActor(registryPID, nil, nil, nil))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation("nonexistent-tool", "default", "default")
		resp, err := goaktactor.Ask(ctx, routerPID, &runtime.RouteInvocation{Invocation: inv}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.RouteResult)
		require.True(t, ok)
		require.Error(t, result.Err)
		assert.ErrorIs(t, result.Err, mcp.ErrToolNotFound)
	})

	t.Run("circuit open rejects work", func(t *testing.T) {
		system, stop := testActorSystem(t,
			goaktactor.WithExtensions(actorextension.NewToolConfigExtension()),
		)
		defer stop()

		_, err := system.Spawn(ctx, mcp.ActorNameJournal, newJournaler(audit.NewMemorySink()))
		require.NoError(t, err)

		tool := validStdioTool("circuit-tool")
		registryPID, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		_, err = goaktactor.Ask(ctx, registryPID, &runtime.RegisterTool{Tool: tool}, askTimeout)
		require.NoError(t, err)
		waitForActors()

		// Open the circuit by reporting failures
		resp, err := goaktactor.Ask(ctx, registryPID, &runtime.GetSupervisor{ToolID: "circuit-tool"}, askTimeout)
		require.NoError(t, err)
		gsResult, ok := resp.(*runtime.GetSupervisorResult)
		require.True(t, ok)
		require.True(t, gsResult.Found)
		supervisorPID, ok := gsResult.Supervisor.(*goaktactor.PID)
		require.True(t, ok)

		for i := 0; i < mcp.DefaultCircuitFailureThreshold; i++ {
			_ = goaktactor.Tell(ctx, supervisorPID, &runtime.ReportFailure{ToolID: tool.ID})
		}
		waitForActors()

		routerPID, err := system.Spawn(ctx, mcp.ActorNameRouter, newRouterActor(registryPID, nil, nil, nil))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation("circuit-tool", "default", "default")
		resp, err = goaktactor.Ask(ctx, routerPID, &runtime.RouteInvocation{Invocation: inv}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.RouteResult)
		require.True(t, ok)
		require.Error(t, result.Err)
		var rErr *mcp.RuntimeError
		require.True(t, assert.ErrorAs(t, result.Err, &rErr))
		assert.Equal(t, mcp.ErrCodeToolUnavailable, rErr.Code)
	})

	t.Run("tool disabled", func(t *testing.T) {
		system, stop := testActorSystem(t,
			goaktactor.WithExtensions(actorextension.NewToolConfigExtension()),
		)
		defer stop()

		_, err := system.Spawn(ctx, mcp.ActorNameJournal, newJournaler(audit.NewMemorySink()))
		require.NoError(t, err)

		tool := validStdioTool("disabled-tool")
		tool.State = mcp.ToolStateDisabled
		registryPID, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		_, err = goaktactor.Ask(ctx, registryPID, &runtime.RegisterTool{Tool: tool}, askTimeout)
		require.NoError(t, err)
		waitForActors()

		routerPID, err := system.Spawn(ctx, mcp.ActorNameRouter, newRouterActor(registryPID, nil, nil, nil))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation("disabled-tool", "default", "default")
		resp, err := goaktactor.Ask(ctx, routerPID, &runtime.RouteInvocation{Invocation: inv}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.RouteResult)
		require.True(t, ok)
		require.Error(t, result.Err)
		var rErr *mcp.RuntimeError
		require.True(t, assert.ErrorAs(t, result.Err, &rErr))
		assert.Equal(t, mcp.ErrCodeToolDisabled, rErr.Code)
	})

	t.Run("invalid invocation nil", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		registryPID, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		routerPID, err := system.Spawn(ctx, mcp.ActorNameRouter, newRouterActor(registryPID, nil, nil, nil))
		require.NoError(t, err)
		waitForActors()

		resp, err := goaktactor.Ask(ctx, routerPID, &runtime.RouteInvocation{Invocation: nil}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.RouteResult)
		require.True(t, ok)
		require.Error(t, result.Err)
		var rErr *mcp.RuntimeError
		require.True(t, assert.ErrorAs(t, result.Err, &rErr))
		assert.Equal(t, mcp.ErrCodeInvalidRequest, rErr.Code)
	})

	t.Run("invalid invocation missing tool ID", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		registryPID, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		routerPID, err := system.Spawn(ctx, mcp.ActorNameRouter, newRouterActor(registryPID, nil, nil, nil))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation("", "default", "default")
		resp, err := goaktactor.Ask(ctx, routerPID, &runtime.RouteInvocation{Invocation: inv}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.RouteResult)
		require.True(t, ok)
		require.Error(t, result.Err)
		var rErr *mcp.RuntimeError
		require.True(t, assert.ErrorAs(t, result.Err, &rErr))
		assert.Equal(t, mcp.ErrCodeInvalidRequest, rErr.Code)
	})

	t.Run("policy denies tenant not in allowlist", func(t *testing.T) {
		system, stop := testActorSystem(t,
			goaktactor.WithExtensions(actorextension.NewToolConfigExtension()),
		)
		defer stop()

		_, err := system.Spawn(ctx, mcp.ActorNameJournal, newJournaler(audit.NewMemorySink()))
		require.NoError(t, err)

		cfg := testConfigWithTenants("allowed-tenant")
		policyPID, err := system.Spawn(ctx, mcp.ActorNamePolicy, newPolicyActor(cfg))
		require.NoError(t, err)

		registryPID, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		tool := validStdioTool("policy-tool")
		tool.AuthorizationPolicy = mcp.AuthorizationPolicyTenantAllowlist
		_, err = goaktactor.Ask(ctx, registryPID, &runtime.RegisterTool{Tool: tool}, askTimeout)
		require.NoError(t, err)
		waitForActors()

		routerPID, err := system.Spawn(ctx, mcp.ActorNameRouter, newRouterActor(registryPID, policyPID, nil, nil))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation("policy-tool", "denied-tenant", "client-1")
		resp, err := goaktactor.Ask(ctx, routerPID, &runtime.RouteInvocation{Invocation: inv}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.RouteResult)
		require.True(t, ok)
		require.Error(t, result.Err)
		var rErr *mcp.RuntimeError
		require.True(t, assert.ErrorAs(t, result.Err, &rErr))
		assert.Equal(t, mcp.ErrCodePolicyDenied, rErr.Code)
	})

	t.Run("successful route with journal records audit event", func(t *testing.T) {
		kit, ctx := newTestKit(t,
			testkit.WithExtensions(actorextension.NewToolConfigExtension()),
		)

		sink := audit.NewMemorySink()
		_, err := kit.ActorSystem().Spawn(ctx, mcp.ActorNameJournal, newJournaler(sink))
		require.NoError(t, err)

		journalPID, err := kit.ActorSystem().ActorOf(ctx, mcp.ActorNameJournal)
		require.NoError(t, err)

		registryPID, err := kit.ActorSystem().Spawn(ctx, "registry-journal", newRegistrar())
		require.NoError(t, err)
		waitForActors()

		tool := validStdioTool("route-journal-tool")
		probe := kit.NewProbe(ctx)
		probe.SendSync("registry-journal", &runtime.RegisterTool{Tool: tool}, askTimeout)
		probe.ExpectAnyMessage()
		waitForActors()

		_, err = kit.ActorSystem().Spawn(ctx, "router-journal", newRouterActor(registryPID, nil, nil, journalPID))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation("route-journal-tool", "tenant1", "client1")
		probe.SendSync("router-journal", &runtime.RouteInvocation{Invocation: inv}, askTimeout)
		resp := probe.ExpectAnyMessage()
		result, ok := resp.(*runtime.RouteResult)
		require.True(t, ok)
		require.NoError(t, result.Err)
		require.NotNil(t, result.Result)
		assert.Equal(t, mcp.ExecutionStatusSuccess, result.Result.Status)

		waitForActors()
		events := sink.Events()
		require.NotEmpty(t, events)
		assert.Equal(t, audit.EventTypeInvocationComplete, events[len(events)-1].Type)
		probe.Stop()
	})

	t.Run("route with CredentialPolicyRequired resolves credentials", func(t *testing.T) {
		kit, ctx := newTestKit(t,
			testkit.WithExtensions(actorextension.NewToolConfigExtension()),
		)

		_, err := kit.ActorSystem().Spawn(ctx, mcp.ActorNameJournal, newJournaler(audit.NewMemorySink()))
		require.NoError(t, err)

		provider := &mockCredentialProvider{creds: map[string]string{"api_key": "secret123"}}
		broker := newCredentialBroker([]credentials.Provider{provider}, time.Minute)
		brokerPID, err := kit.ActorSystem().Spawn(ctx, "broker-route", broker)
		require.NoError(t, err)

		registryPID, err := kit.ActorSystem().Spawn(ctx, "registry-cred", newRegistrar())
		require.NoError(t, err)
		waitForActors()

		tool := validStdioTool("cred-tool")
		tool.CredentialPolicy = mcp.CredentialPolicyRequired
		probe := kit.NewProbe(ctx)
		probe.SendSync("registry-cred", &runtime.RegisterTool{Tool: tool}, askTimeout)
		probe.ExpectAnyMessage()
		waitForActors()

		_, err = kit.ActorSystem().Spawn(ctx, "router-cred", newRouterActor(registryPID, nil, brokerPID, nil))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation("cred-tool", "tenant1", "client1")
		probe.SendSync("router-cred", &runtime.RouteInvocation{Invocation: inv}, askTimeout)
		resp := probe.ExpectAnyMessage()
		result, ok := resp.(*runtime.RouteResult)
		require.True(t, ok)
		require.NoError(t, result.Err)
		require.NotNil(t, result.Result)
		probe.Stop()
	})

	t.Run("route with CredentialPolicyRequired and unavailable credentials fails", func(t *testing.T) {
		kit, ctx := newTestKit(t,
			testkit.WithExtensions(actorextension.NewToolConfigExtension()),
		)

		_, err := kit.ActorSystem().Spawn(ctx, mcp.ActorNameJournal, newJournaler(audit.NewMemorySink()))
		require.NoError(t, err)

		provider := &mockCredentialProvider{creds: nil}
		broker := newCredentialBroker([]credentials.Provider{provider}, time.Minute)
		brokerPID, err := kit.ActorSystem().Spawn(ctx, "broker-unavail", broker)
		require.NoError(t, err)

		registryPID, err := kit.ActorSystem().Spawn(ctx, "registry-unavail", newRegistrar())
		require.NoError(t, err)
		waitForActors()

		tool := validStdioTool("cred-req-tool")
		tool.CredentialPolicy = mcp.CredentialPolicyRequired
		probe := kit.NewProbe(ctx)
		probe.SendSync("registry-unavail", &runtime.RegisterTool{Tool: tool}, askTimeout)
		probe.ExpectAnyMessage()
		waitForActors()

		_, err = kit.ActorSystem().Spawn(ctx, "router-unavail", newRouterActor(registryPID, nil, brokerPID, nil))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation("cred-req-tool", "tenant1", "client1")
		probe.SendSync("router-unavail", &runtime.RouteInvocation{Invocation: inv}, askTimeout)
		resp := probe.ExpectAnyMessage()
		result, ok := resp.(*runtime.RouteResult)
		require.True(t, ok)
		require.Error(t, result.Err)
		var rErr *mcp.RuntimeError
		require.True(t, assert.ErrorAs(t, result.Err, &rErr))
		assert.Equal(t, mcp.ErrCodeCredentialUnavailable, rErr.Code)
		probe.Stop()
	})

	t.Run("policy rate limit produces throttle outcome", func(t *testing.T) {
		kit, ctx := newTestKit(t,
			testkit.WithExtensions(actorextension.NewToolConfigExtension()),
		)

		_, err := kit.ActorSystem().Spawn(ctx, mcp.ActorNameJournal, newJournaler(audit.NewMemorySink()))
		require.NoError(t, err)

		cfg := testConfig()
		cfg.Tenants = []config.TenantConfig{{
			ID: "rate-tenant",
			Quotas: config.TenantQuotaConfig{
				RequestsPerMinute: 2,
			},
		}}
		policyPID, err := kit.ActorSystem().Spawn(ctx, "policy-rate", newPolicyActor(cfg))
		require.NoError(t, err)

		registryPID, err := kit.ActorSystem().Spawn(ctx, "registry-rate", newRegistrar())
		require.NoError(t, err)
		waitForActors()

		tool := validStdioTool("rate-tool")
		probe := kit.NewProbe(ctx)
		probe.SendSync("registry-rate", &runtime.RegisterTool{Tool: tool}, askTimeout)
		probe.ExpectAnyMessage()
		waitForActors()

		_, err = kit.ActorSystem().Spawn(ctx, "router-rate", newRouterActor(registryPID, policyPID, nil, nil))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation("rate-tool", "rate-tenant", "client1")
		probe.SendSync("router-rate", &runtime.RouteInvocation{Invocation: inv}, askTimeout)
		resp := probe.ExpectAnyMessage()
		result, ok := resp.(*runtime.RouteResult)
		require.True(t, ok)
		require.NoError(t, result.Err)

		inv2 := sessionInvocation("rate-tool", "rate-tenant", "client1")
		inv2.Correlation.RequestID = "req-2"
		probe.SendSync("router-rate", &runtime.RouteInvocation{Invocation: inv2}, askTimeout)
		resp2 := probe.ExpectAnyMessage()
		result2, ok := resp2.(*runtime.RouteResult)
		require.True(t, ok)
		require.Error(t, result2.Err)
		var rErr *mcp.RuntimeError
		require.True(t, assert.ErrorAs(t, result2.Err, &rErr))
		assert.Equal(t, mcp.ErrCodeRateLimited, rErr.Code)
		probe.Stop()
	})

	t.Run("records InvocationLatency metric on success when metrics are registered", func(t *testing.T) {
		meter := noopmetric.NewMeterProvider().Meter("test")
		_, err := telemetry.RegisterMetrics(meter)
		require.NoError(t, err)
		t.Cleanup(telemetry.UnregisterMetrics)

		system, stop := testActorSystem(t, goaktactor.WithExtensions(actorextension.NewToolConfigExtension()))
		defer stop()

		_, err = system.Spawn(ctx, mcp.ActorNameJournal, newJournaler(audit.NewMemorySink()))
		require.NoError(t, err)

		registryPID, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		tool := validStdioTool("metrics-route-tool")
		_, err = goaktactor.Ask(ctx, registryPID, &runtime.RegisterTool{Tool: tool}, askTimeout)
		require.NoError(t, err)
		waitForActors()

		routerPID, err := system.Spawn(ctx, mcp.ActorNameRouter, newRouterActor(registryPID, nil, nil, nil))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation("metrics-route-tool", "tenant1", "client1")
		resp, err := goaktactor.Ask(ctx, routerPID, &runtime.RouteInvocation{Invocation: inv}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.RouteResult)
		require.True(t, ok)
		require.NoError(t, result.Err)
	})

	t.Run("records InvocationFailure metric on tool-not-found when metrics are registered", func(t *testing.T) {
		meter := noopmetric.NewMeterProvider().Meter("test")
		_, err := telemetry.RegisterMetrics(meter)
		require.NoError(t, err)
		t.Cleanup(telemetry.UnregisterMetrics)

		system, stop := testActorSystem(t, goaktactor.WithExtensions(actorextension.NewToolConfigExtension()))
		defer stop()

		_, err = system.Spawn(ctx, mcp.ActorNameJournal, newJournaler(audit.NewMemorySink()))
		require.NoError(t, err)

		registryPID, err := system.Spawn(ctx, mcp.ActorNameRegistrar, newRegistrar())
		require.NoError(t, err)
		waitForActors()

		routerPID, err := system.Spawn(ctx, mcp.ActorNameRouter, newRouterActor(registryPID, nil, nil, nil))
		require.NoError(t, err)
		waitForActors()

		inv := sessionInvocation("nonexistent-tool", "tenant1", "client1")
		resp, err := goaktactor.Ask(ctx, routerPID, &runtime.RouteInvocation{Invocation: inv}, askTimeout)
		require.NoError(t, err)
		result, ok := resp.(*runtime.RouteResult)
		require.True(t, ok)
		require.Error(t, result.Err)
	})
}
