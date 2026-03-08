# GoAkt MCP Gateway Ideation

## Project Vision
`goakt-mcp` is a Go-based MCP gateway built on top of `GoAkt`. Its purpose is to sit between AI agents and MCP servers, providing a resilient, observable, and scalable execution layer for tool calls.

The vision is not simply to proxy JSON-RPC messages. The gateway should act as an operational control plane for MCP workloads:

- manage many concurrent MCP sessions safely
- isolate failing or slow tools from healthy traffic
- support local and remote MCP servers through a unified runtime
- provide tenancy, tracing, security, and governance by default
- scale from a single-node developer setup to a clustered production deployment

GoAkt is a strong fit because the actor model maps naturally to MCP connection management. Each tool session is stateful, failure-prone, and concurrent, which is exactly the kind of workload actors, supervision, mailboxes, and passivation are designed to handle.

## Problem Statement
AI agents increasingly rely on external tools for retrieval, automation, data access, and execution. In practice, that creates a few recurring operational problems:

- MCP servers are stateful and can be expensive to start or reconnect
- tool runtimes may be local processes, containers, or remote services
- some tools are fast and reliable, while others are slow, flaky, or bursty
- a simple request proxy offers little protection against cascading failures
- observability, tenant isolation, and auditability are often bolted on later

The gateway should solve these problems without forcing clients to understand the complexity of the underlying tool topology.

## Product Goal
Expose a single, well-defined gateway that agents can call while the runtime handles:

- MCP server lifecycle management
- connection pooling or session ownership where applicable
- supervision and restart behavior
- request routing and backpressure
- security boundaries and credential brokering
- tracing, metrics, dead-letter capture, and audit logs

For the first implementation, this means a single deployable Go service that accepts HTTP requests, routes them into a GoAkt actor system, and executes MCP calls against primarily local `stdio` servers.

## Design Principles
- `Actor-first runtime`: all long-lived, stateful, concurrent workflows are modeled as actors.
- `Transport-agnostic core`: the runtime should not care whether a tool is reached via `stdio`, HTTP, or another MCP-compatible transport.
- `Fail small`: a single bad tool or bad session must not degrade the entire gateway.
- `Operational by default`: telemetry, retries, admission control, and auditing are core features, not afterthoughts.
- `Cluster-ready`: local development should be simple, but the architecture must support multi-node deployment from the start.
- `GoAkt-native`: prefer GoAkt features directly instead of rebuilding actor runtime behavior in application code.

## Why GoAkt
GoAkt already provides many of the runtime capabilities this gateway needs:

- supervision strategies for failure handling and controlled restarts
- remoting and clustering for distributed deployments
- cluster singletons for globally unique coordination actors
- passivation for reclaiming idle actors and sessions
- scheduling for health checks, retries, and maintenance jobs
- context propagation for trace and tenant metadata
- OpenTelemetry support, dead letters, and event-stream observability
- customizable mailboxes for bounded queues and backpressure

This allows `goakt-mcp` to focus on MCP semantics and product concerns rather than reinventing a concurrency platform.

## Proposed Architecture
The system can be organized into three layers: ingress, actor runtime, and egress.

### 1. Ingress Layer
This layer accepts requests from AI agents, SDKs, or upstream orchestrators.

Responsibilities:

- expose a stable API for tool invocation and session management
- normalize incoming requests into internal command messages
- attach metadata such as `tenant_id`, `request_id`, `trace_id`, and auth context
- enforce admission control, authn/authz, quotas, and request size limits

Possible interfaces:

- a gateway-native HTTP/JSON API is the primary v1 interface
- an admin and control-plane API is available for dynamic tool registration, policy inspection, and operational workflows
- an MCP-compatible server facade remains optional and can be layered on later if ecosystem demand justifies it
- streaming endpoints are supported when required by the target MCP server workflow, but synchronous request/response remains the default path

The ingress layer should remain thin. Its job is validation, metadata injection, and dispatch into the actor system.

### 2. Actor Runtime Layer
This is the core of the system and should be implemented with GoAkt actors.

#### Suggested actor topology

`GatewayManager`

- top-level coordinator for gateway lifecycle
- wires the actor system, registries, dependencies, and bootstrap flow

`RegistryActor` as a cluster singleton

- keeps the canonical mapping between tool identity and the responsible supervisor
- tracks capability metadata such as transport, version, health, and tenancy scope
- answers lookups for routing decisions

`ToolSupervisorActor` per tool definition

- owns the lifecycle policy for a given MCP server or tool family
- spawns and monitors one or more session or connection actors
- enforces restart rules, retry windows, and circuit-breaker behavior

`SessionActor` or `ConnectionActor` per active MCP session

- owns the actual stateful interaction with a tool endpoint
- handles protocol sequencing, in-flight request bookkeeping, and response correlation
- isolates failures to a single session whenever possible

`RouterActor`

- selects the appropriate tool supervisor or session actor
- can apply routing policies such as sticky sessions, round-robin, or least-loaded

`CredentialBrokerActor`

- resolves secrets or short-lived credentials on demand
- prevents long-lived secrets from being copied broadly across the system

`JournalActor`

- receives asynchronous audit events for request and response activity
- writes to durable storage without blocking the request path

`HealthActor`

- performs scheduled liveness checks and recovery probes
- reports health state transitions for alerting and routing decisions

### 3. Egress Layer
This layer speaks to actual MCP servers.

Supported execution models should include:

- `stdio` for local processes launched with `os/exec`
- HTTP-based MCP connectivity for remote services
- optional compatibility support for legacy transports if required by the target environment

The actor model should hide transport differences behind a consistent internal message contract.

## Core Runtime Flows
### Tool invocation
1. A client submits a request to invoke a tool.
2. The ingress layer validates the request and adds metadata.
3. The `RegistryActor` resolves the target tool owner.
4. A `RouterActor` or `ToolSupervisorActor` selects the correct `SessionActor`.
5. The session executes the MCP call and returns the response.
6. Telemetry and audit events are emitted asynchronously.

### Session recovery
1. A session or transport fails.
2. The owning supervisor applies GoAkt supervision rules.
3. If failure thresholds are exceeded, the circuit opens and new requests fail fast.
4. A scheduled health probe attempts recovery.
5. On success, the circuit closes and traffic resumes.

### Idle cleanup
1. Low-traffic or one-off sessions become idle.
2. GoAkt passivation stops them automatically after the configured timeout.
3. A future request recreates the actor when needed.

## Resilience Model
Resilience should be designed explicitly rather than treated as a side effect of restarts.

### Supervision
Use GoAkt supervision for actor failures with policies tuned by actor type:

- registry and coordination actors should favor stability and controlled restart behavior
- session actors can be disposable and aggressively restarted
- journal and telemetry actors should fail independently from request-serving actors

### Circuit breaker
Each `ToolSupervisorActor` should maintain a simple state machine:

- `closed`: requests flow normally
- `open`: requests are rejected quickly after repeated failures
- `half_open`: limited recovery probes are allowed before the tool is trusted again

This avoids slow hangs and reduces failure amplification across the system.

### Backpressure
Bounded mailboxes should be used where request surges are expected. When overloaded, the gateway should prefer predictable rejection or queuing limits over unbounded memory growth.

### Timeouts and cancellation
Every request path should carry deadline and cancellation context. A slow or disconnected MCP server must not hold resources indefinitely.

## Security Model
Security should be part of the architecture, especially when launching local tools.

### Credential handling
- session actors should request credentials just in time from a dedicated broker
- secrets should not be embedded in actor state unless strictly necessary
- short-lived tokens are preferred over static credentials

### Process isolation
- local MCP servers should run with minimal environment exposure
- filesystem, process, and network permissions should be constrained where possible
- per-tool configuration should define what capabilities are allowed

### Multi-tenancy
- tenant metadata should flow through actor messages and telemetry
- tenant isolation rules should prevent accidental data leakage across sessions
- quotas and policy checks should be enforced before work is admitted

For production deployments, soft multi-tenancy should be supported in a single cluster, while deployment-level isolation remains available for regulated or high-trust environments. The internal message model must always carry `tenant_id` and `client_id`, and all routing, policy, and audit behavior must remain tenant-aware.

## Observability And Governance
This project should be production-friendly from the beginning.

### Telemetry
- emit traces for each request lifecycle
- expose metrics for tool latency, failure rate, restarts, open circuits, and queue depth
- capture dead letters and abnormal actor exits

### Auditability
- asynchronously journal request and response metadata
- persist governance events such as auth decisions, retries, and restarts
- make it easy to answer "who invoked what tool, when, and under which tenant?"

### Operations
- provide health and readiness endpoints
- expose actor-system level insights needed for diagnosing stuck tools or hot partitions
- support structured logging with correlation identifiers

## Deployment Model
The project should support two major modes.

### Single-node mode
Best for local development, CI, and low-scale deployments.

- one GoAkt actor system
- local process execution via `stdio`
- minimal dependencies

### Cluster mode
Best for high availability and horizontal scale.

- GoAkt clustering for multi-node runtime coordination
- cluster singleton for the global registry
- remote actors for distributed routing and execution
- service discovery backend selected to fit the target environment

Both modes are first-class. Single-node exists for developer ergonomics; clustered deployment is the default production topology.

## Production-Ready v1 Scope
The first release should ship with the capabilities required for real production use, not as a thin MVP.

### Core runtime
- gateway-native HTTP/JSON API for invocation, health, readiness, and catalog discovery
- `stdio` and HTTP-based MCP transports available out of the box
- per-tool supervision with bounded mailboxes, passivation, and circuit breakers
- sticky session ownership per `tenant_id + client_id + tool_id`
- request deadlines, cancellation propagation, and structured error mapping
- OpenTelemetry tracing, metrics, structured logs, and dead-letter visibility
- asynchronous audit journaling with durable persistence

### Scale and high availability
- GoAkt cluster mode supported in `v1`
- cluster singleton for registry and control-plane coordination
- remote actor routing across nodes
- health-aware failover between eligible session or transport targets
- service discovery integration suitable for production environments

### Dynamic operations
- dynamic tool discovery and registration
- admin workflows for register, update, disable, and remove tool definitions
- change propagation across the cluster without full process restart
- health state reflected back into routing and registration visibility

### Security and governance
- credential broker integrations for secret resolution and short-lived token retrieval
- policy enforcement for per-tenant and per-tool authorization
- quotas, concurrency limits, and rate limiting
- full tenant-aware audit records and policy decision logging
- deployment-level isolation option for teams that require harder boundaries

### Routing
- sticky routing for stateful sessions
- least-loaded and health-aware routing for stateless or pooled workloads
- configurable routing policy per tool definition
- routing decisions based on tenant, tool capabilities, node health, and session affinity

## Non-Goals For The First Iteration
- becoming a full workflow engine for multi-step agent orchestration
- supporting every MCP transport or extension on day one
- providing a general-purpose API gateway unrelated to MCP
- acting as a full secret-management product instead of integrating with established providers

## Concrete Decisions
The following decisions define a production-ready `v1` rather than a narrow prototype.

### 1. Transport priority
`v1` supports both `stdio` and HTTP-based MCP transports as first-class execution paths.

- local MCP servers launched as child processes remain a primary execution target
- each active session owns exactly one child process and one `stdio` transport connection
- remote HTTP MCP servers are supported behind the same internal actor contract
- all transport implementations must satisfy the same internal message model so routing, policy, observability, and supervision stay transport-agnostic

This keeps the architecture coherent while supporting both local and remote tool execution in production.

### 2. External API shape
`v1` exposes a gateway-native HTTP/JSON data plane and a small control plane.

- the data plane exposes at least these endpoints:
- `POST /v1/tools/{tool}/invoke`
- `GET /v1/tools`
- `GET /healthz`
- `GET /readyz`
- the control plane exposes administrative endpoints for tool registration, update, disablement, and removal
- requests are normalized into internal command messages before entering the actor system
- the request body for `invoke` contains `client_id`, `request_id`, `method`, `params`, and optional metadata
- streaming remains optional and transport-specific, but the runtime must not block future streaming support
- an MCP-compatible facade is not required for `v1`

This keeps the public surface small while still enabling dynamic operations and production management.

### 3. Tenancy model
`v1` supports soft multi-tenancy with deployment-level isolation as an option.

- one cluster may serve multiple tenants when allowed by policy
- deployment-level single-tenant mode remains available for stricter isolation requirements
- `tenant_id` is required in the request model and actor metadata
- `client_id` is mandatory because it defines session affinity
- `trace_id` and `request_id` are mandatory for observability and auditability

This makes the platform useful in shared environments without forcing every deployment into the same trust model.

### 4. Default observability and audit backends
The default telemetry stack should be production-ready:

- traces and metrics are emitted through OpenTelemetry
- logs are structured JSON logs with correlation identifiers
- audit events are written asynchronously through the `JournalActor` to a pluggable durable sink
- the default production sink should be object storage, a database, or another centralized durable backend
- local JSONL output remains available for development and fallback scenarios
- OTLP export is enabled by configuration and expected in production environments
- each audit record contains `timestamp`, `tenant_id`, `client_id`, `tool_id`, `request_id`, `trace_id`, `method`, `status`, `latency_ms`, and error details when present

The audit sink must be abstracted behind an interface so deployment environments can select the backend that fits their compliance and retention requirements.

### 5. Session ownership model
Session ownership is sticky per client and tool.

- each active `client_id + tool_id` pair is mapped to one `SessionActor`
- the actor name should be derived from a stable key such as `session/{tenant_id}/{client_id}/{tool_id}`
- the `SessionActor` owns the live MCP connection, the child process, and any protocol state associated with it
- idle sessions are passivated after a configured timeout, with a default of `5m`
- when a session is passivated or crashes, the child process is terminated and fully recreated on the next request
- session state is not shared across clients, even when they call the same tool
- a new request for the same `client_id + tool_id` recreates or reactivates the session as needed

This model preserves isolation, avoids state leakage across clients, and still gives the system warm-session reuse. For stateless tools, pooled session mode may be enabled explicitly by configuration.

### 6. Tool registration model
`v1` uses hybrid registration: static bootstrap plus dynamic registration and discovery.

- tools may be declared in configuration for bootstrap
- tools may also be registered and updated dynamically through the control plane
- external discovery sources may publish tool availability into the registry
- each tool definition includes `id`, `transport`, `command` or `url`, `args`, `env`, `working_directory`, startup timeout, request timeout, idle passivation timeout, routing policy, credential policy, and authorization policy
- the `RegistryActor` remains the canonical in-memory registry, backed by a durable control-plane store and cluster propagation
- registration changes are applied without full process restart

This gives operators a stable bootstrap path while still supporting real operational change in production.

### 7. Routing and concurrency model
Routing is policy-driven, but it keeps actor ownership rules simple.

- `POST /v1/tools/{tool}/invoke` always routes to the `ToolSupervisorActor` responsible for `{tool}`
- the supervisor resolves the target `SessionActor` using `tenant_id + client_id + tool_id`
- there is at most one live `SessionActor` per session key
- in-flight requests for the same session are serialized through that actor's mailbox
- concurrency comes from many independent session actors, not from parallel use of the same MCP session
- the default routing mode for stateful tools is sticky and affinity-aware
- the default routing mode for stateless tools is least-loaded among healthy eligible targets
- routing decisions may incorporate tenant policy, node locality, health, and transport availability

This matches actor semantics, avoids protocol races, and gives production operators the routing controls they need.

### 8. Failure policy
`v1` uses explicit, configurable, and production-oriented failure handling.

- process startup failure returns a retryable `503 Service Unavailable`
- session-level timeout returns `504 Gateway Timeout`
- malformed request returns `400 Bad Request`
- unknown tool returns `404 Not Found`
- tool circuit open returns `503 Service Unavailable`
- unexpected runtime failure returns `500 Internal Server Error`
- each `ToolSupervisorActor` opens its circuit after `5` consecutive failures and schedules a half-open probe after `30s`

These defaults should be configurable, but they provide a concrete baseline for implementation and testing.

### 9. Credential management model
`v1` includes credential broker integrations out of the box.

- actors never persist long-lived credentials in session state
- the `CredentialBrokerActor` resolves credentials just in time for the target tool and tenant
- the broker supports a provider chain such as environment variables, local files, Vault, and cloud secret managers
- short-lived tokens are preferred whenever the upstream system supports them
- broker responses may be cached briefly with TTL-aware invalidation

This keeps secrets out of broad process state while making production integrations practical.

### 10. Governance and policy model
`v1` includes richer governance and policy enforcement as a runtime capability.

- each tool definition may include an authorization policy, quota policy, and concurrency policy
- requests are evaluated before session resolution and again before tool execution when needed
- policy decisions are logged to the audit stream
- policy failure returns explicit client-visible errors such as `403 Forbidden` or `429 Too Many Requests`
- tenant-level and tool-level limits are enforced consistently across cluster nodes

This prevents governance from becoming an afterthought once the system is under real load.

## Initial Implementation Shape
To make the implementation path concrete, the repository should evolve toward something close to the following structure:

- `cmd/goakt-mcp`: process entrypoint and config loading
- `internal/api`: data-plane HTTP handlers, request validation, and DTOs
- `internal/admin`: control-plane handlers for tool registration, policy inspection, and operational actions
- `internal/actors`: GoAkt actors such as `RegistryActor`, `ToolSupervisorActor`, `SessionActor`, `JournalActor`, `HealthActor`, `CredentialBrokerActor`, and policy-related coordinators
- `internal/cluster`: cluster bootstrap, singleton wiring, and discovery integration
- `internal/mcp`: transport-agnostic MCP client abstractions and message contracts
- `internal/transports/stdio`: child-process launcher, stdio framing, and process lifecycle integration
- `internal/transports/http`: remote MCP HTTP client, connection management, and retry behavior
- `internal/telemetry`: OpenTelemetry setup, metrics, tracing, and structured logging helpers
- `internal/audit`: durable audit sink abstractions and the writer used by the `JournalActor`
- `internal/config`: tool definitions, timeouts, passivation settings, cluster options, and runtime policy configuration
- `internal/discovery`: dynamic tool discovery adapters and registration propagation
- `internal/policy`: authorization, quota, and rate-limit evaluation
- `internal/credentials`: secret provider integrations and credential caching

## Core Contracts
The first implementation should standardize on a small but production-ready internal message model so transports and APIs remain replaceable.

### HTTP request contract
`POST /v1/tools/{tool}/invoke`

```json
{
  "tenant_id": "acme-dev",
  "client_id": "cursor-user-123",
  "request_id": "req_01JXYZ",
  "trace_id": "trace_01JXYZ",
  "method": "tools/call",
  "params": {
    "name": "search_docs",
    "arguments": {
      "query": "actor supervision"
    }
  },
  "metadata": {
    "source": "cursor"
  }
}
```

### Internal actor messages
The actor system should begin with a clear set of command and event types:

- `InvokeTool`
- `InvokeToolResult`
- `GetToolCatalog`
- `RegisterTool`
- `UpdateTool`
- `DisableTool`
- `RemoveTool`
- `ResolveSession`
- `StartSession`
- `StopSession`
- `SessionPassivated`
- `ToolProcessExited`
- `ResolveCredentials`
- `AuthorizeRequest`
- `PolicyDecision`
- `ToolDiscovered`
- `ToolHealthChanged`
- `HealthCheckTick`
- `AuditRecord`

These message types are enough to support `v1` without prematurely over-modeling the protocol.

## Configuration Baseline
`v1` should support static bootstrap configuration with production features enabled from the start.

Example tool definition:

```yaml
tenants:
  - id: acme-dev
    quotas:
      requests_per_minute: 1000
      concurrent_sessions: 200
http:
  listen_address: ":8080"
cluster:
  enabled: true
  discovery: consul
  singleton_role: control-plane
telemetry:
  otlp_endpoint: "http://otel-collector:4318"
audit:
  backend: s3
  bucket: goakt-mcp-audit
credentials:
  providers: ["env", "vault"]
runtime:
  session_idle_timeout: 5m
  request_timeout: 30s
  startup_timeout: 10s
tools:
  - id: filesystem
    transport: stdio
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    env: {}
    working_directory: /
    startup_timeout: 10s
    request_timeout: 30s
    idle_timeout: 5m
    routing: sticky
    credential_policy: optional
    authorization_policy: tenant_allowlist
  - id: docs-search
    transport: http
    url: "https://mcp.internal.example.com"
    request_timeout: 15s
    idle_timeout: 2m
    routing: least_loaded
    credential_policy: required
    authorization_policy: tenant_allowlist
```

This gives the implementation a concrete source of truth for bootstrapping the `RegistryActor`, supervisors, and runtime policies.

## Production Build Plan
The first implementation should be built in this order:

1. Define the configuration model for tenants, tools, policies, clustering, telemetry, auditing, and credentials.
2. Implement the data-plane endpoints `POST /v1/tools/{tool}/invoke`, `GET /v1/tools`, `GET /healthz`, and `GET /readyz`.
3. Implement the control-plane endpoints for `register`, `update`, `disable`, and `remove` tool operations.
4. Build cluster bootstrap, service discovery wiring, and the `RegistryActor` as a cluster singleton.
5. Build one `ToolSupervisorActor` per registered tool and make registration updates propagate cluster-wide.
6. Implement the `SessionActor` for `stdio`-backed MCP processes, including child process lifecycle and passivation.
7. Implement the HTTP transport adapter behind the same internal actor contract.
8. Add credential resolution through `CredentialBrokerActor` and provider integrations.
9. Add authorization, quotas, and rate limiting before session resolution and tool execution.
10. Implement sticky, least-loaded, and health-aware routing policies.
11. Add timeout handling, cancellation propagation, retries, circuit breakers, and structured HTTP error mapping.
12. Add `JournalActor`, durable audit sinks, OpenTelemetry tracing, JSON logging, and core metrics.
13. Add recovery probes, health transitions, and failover behavior across nodes and transports.

## Recommended Direction
Build `goakt-mcp` as a GoAkt-native MCP gateway, not just a transport proxy. The main value of the project is operational control over MCP tool execution: supervision, isolation, routing, observability, and scale.

That framing preserves the original vision while making the implementation strategy more concrete, more realistic, and much better aligned with what GoAkt already does well.