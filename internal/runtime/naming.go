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

package runtime

import "fmt"

// Actor name constants for well-known singleton actors.
//
// These constants define the fixed names under which singleton actors are registered
// in the GoAkt actor system. Fixed names allow other actors and runtime components to
// address them by name without holding a direct actor reference.
const (
	// ActorNameGatewayManager is the fixed name for the GatewayManager actor.
	// GatewayManager is the composition root of the runtime.
	ActorNameGatewayManager = "gateway-manager"

	// ActorNameRegistrar is the fixed name for the Registrar.
	// Registrar is a cluster singleton and must use a consistent name across nodes.
	ActorNameRegistrar = "registrar"

	// ActorNameHealth is the fixed name for the HealthActor.
	// HealthActor runs scheduled probes and reports health state transitions.
	ActorNameHealth = "health"

	// ActorNameJournal is the fixed name for the JournalActor.
	// JournalActor writes durable audit records asynchronously off the request path.
	ActorNameJournal = "journal"

	// ActorNameCredentialBroker is the fixed name for the CredentialBrokerActor.
	// CredentialBrokerActor resolves credentials just in time for tool invocations.
	ActorNameCredentialBroker = "credential-broker" //nolint:gosec // not a credential, it is an actor name constant

	// ActorNameRouter is the fixed name for the RouterActor.
	// RouterActor is the runtime entry point for tool invocations and performs
	// routing from tool lookup to session execution.
	ActorNameRouter = "router"

	// ActorNamePolicy is the fixed name for the PolicyActor.
	// PolicyActor evaluates authorization, quotas, rate limits, and concurrency
	// limits before execution.
	ActorNamePolicy = "policy"
)

// ToolSupervisorName returns the deterministic actor name for the tool supervisor
// responsible for the given tool.
//
// Supervisor names must be stable across restarts and consistent across the cluster
// so that registry lookups and routing decisions always resolve to the same actor name.
// Uses hyphen (not colon) to comply with GoAkt actor name pattern [a-zA-Z0-9][a-zA-Z0-9-_]*.
func ToolSupervisorName(toolID ToolID) string {
	return fmt.Sprintf("supervisor-%s", toolID)
}

// SessionName returns the deterministic actor name for the session owned by the
// given tenant, client, and tool combination.
//
// This name enforces the one-active-session-per-tuple invariant: because the actor
// name is derived from the identity triple, the GoAkt actor system will reject
// attempts to spawn a duplicate actor under the same name.
// Uses hyphens to comply with GoAkt actor name pattern [a-zA-Z0-9][a-zA-Z0-9-_]*.
func SessionName(tenantID TenantID, clientID ClientID, toolID ToolID) string {
	return fmt.Sprintf("session-%s-%s-%s", tenantID, clientID, toolID)
}
