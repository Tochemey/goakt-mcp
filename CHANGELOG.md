# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

#### Sessions Migrated to GoAkt Grains (virtual actors)

- The per-`(tenant, client, tool)` `SessionActor` is replaced by a `sessionGrain` that implements the `goaktactor.Grain` interface. Sessions are now activated on demand by goakt's grain engine via `GrainIdentity` and addressed through `ActorSystem.AskGrain` / `TellGrain` rather than actor PIDs.
- Idle passivation is delegated to `WithGrainDeactivateAfter(idleTimeout)`; the hand-rolled `PausePassivation`/`ResumePassivation` dance inside session invocations is gone because the grain mailbox already serializes with in-flight work.
- Executor construction moved into the grain's `OnActivate` (from the supervisor). The supervisor previously pre-created an executor on every `GetOrCreateSession` call, but goakt's grain engine always invokes the factory/dependency options even when the grain is already active; that executor was silently orphaned — a resource leak under load. Reading the `ExecutorFactoryExtension` inside the grain itself eliminates the leak because the construction only happens on first activation. `SessionDependency.Executor` and the `executor` parameter of `NewSessionDependency` are removed accordingly.
- In-flight streaming invocations are now tracked by a `sync.WaitGroup` on the grain. `OnDeactivate` waits for all streaming goroutines to finish before closing the executor, preventing a mid-stream passivation tear-down when a stream runs longer than the idle timeout.
- `forwardStream` now uses a bounded `select` with `streamForwardSendTimeout` (30s) when forwarding progress events to the caller. A disconnected consumer that never drains the channel no longer leaks the forwarding goroutine; it times out, cancels the executor context, and exits cleanly.
- `ToolSupervisor.GetOrCreateSession` now calls `ActorSystem.GrainIdentity` and returns the resulting `*goaktactor.GrainIdentity` in `runtime.GetOrCreateSessionResult.Session`. Callers (the Router) dispatch via `ActorSystem.AskGrain`.
- Supervisor session accounting (backpressure + admin enumeration) moved from `ctx.Self().ChildrenCount()` / `Children()` to an in-memory map of `sessionRegistration` entries. The grain sends `runtime.SessionActivated` on `OnActivate` and `runtime.SessionDeactivated` on `OnDeactivate`; the supervisor maintains the authoritative per-tool map from those messages.
- Executor recovery stays intact: on transport failure the grain rebuilds its executor via `ExecutorFactoryExtension` and retries once.
- Streaming invocations (`SessionInvokeStream`) work the same — the grain forwards progress and final events through a `StreamingResult` while the handler returns immediately so the mailbox stays free for the next message.
- The `GatewayManager.PreStart` registers the grain kind on the actor system so remote activation / recreation works in cluster mode.
- Legacy files deleted: `internal/runtime/actor/session.go` (499 LOC), `internal/runtime/actor/session_test.go` (821 LOC). New files: `internal/runtime/actor/session_grain.go` and `internal/runtime/actor/session_grain_test.go` (7 focused tests covering invoke, stream, fallback, identity, failure reporting).

#### Registrar Tool Catalog Extraction

- The in-memory `map[ToolID]Tool` + three parallel metadata maps inside the RegistryActor are replaced by a dedicated `toolCatalog` type (`internal/runtime/actor/registrar_catalog.go`) that bundles each tool with its schemas and MCP resource metadata in a single entry.
- `toolCatalog` exposes a narrow API (`Put`, `UpdateTool`, `Remove`, `Get`, `Has`, `List`, `SetSchemas`, `SetResources`, etc.) so a future CRDT-backed implementation can slot in behind the same contract without changing the actor's message flow.
- Registrar's per-handler logic is shorter and the storage invariants (stale schemas cleared on re-registration, metadata cleared on Remove) now live in one place.
- Tested with 10 new unit tests covering every catalog mutation path.

### Added

#### Dead-Letter Observability

- `Gateway` now consumes `*goaktactor.Deadletter` events from the actor system event stream in addition to passivation events. Every undelivered message is logged at Warn level with sender, receiver, and reason fields.
- New OpenTelemetry counter `goaktmcp.actor.dead_letter` tagged by `reason` surfaces dead-letter rates for alerting; exposed via `telemetry.RecordDeadLetter`.
- Event consumer renamed from `consumePassivationEvents` to `consumeSystemEvents` to reflect the wider scope.

### Changed

#### Router Pipeline Refactor

- The Router actor's synchronous and streaming handlers now share a single pre-execution pipeline (`runPreExecutionPipeline`): tool lookup, policy evaluation, supervisor lookup, accept-work check, credential resolution, and session resolution happen in one place with uniform telemetry and audit emission.
- Stage-specific failure handling collapsed into `emitRouteFailure`, `emitValidationFailure`, `emitExecutionFailure`, `emitInvocationComplete`. Scattered `errors.As` for error-code extraction centralised in `errorCodeFrom`.
- Audit-outcome strings (`invalid`, `error`, `unavailable`, `credential_unavailable`, `session_error`, `execution_error`, `streaming`) are now package-level constants in `router_pipeline.go` rather than inline literals at each call site.
- Metrics attribute keys (`tool_id`, `tenant_id`, `reason`) and credential-cache result values (`hit`, `miss`) in `internal/runtime/telemetry/metrics.go` are now named constants.

#### Audit Event Stream

- `Gateway.SubscribeAudit()` returns a `eventstream.Subscriber` that receives every `*mcp.AuditEvent` written by the Journal actor after `Gateway.Start`. Consumers poll via `Subscriber.Iterator()` for in-process fan-out of audit events alongside the existing `AuditSink` path.
- `Gateway.UnsubscribeAudit(sub)` cleanly removes a subscriber and tears down its buffered state.
- `AuditStreamExtension` system-level extension exposes the audit `eventstream.Stream` to the `Journaler` actor, which publishes on the `mcp.audit` topic (`extension.AuditStreamTopic`) in addition to writing to the configured `AuditSink`.
- `audit.MetadataKeyReason`, `audit.MetadataKeyFailureCount`, `audit.MetadataKeyFromState`, `audit.MetadataKeyToState` constants replace inline metadata keys in audit event emitters.

#### Enterprise Token Exchange (RFC 8693)

- `TokenExchanger` interface and `TokenExchangeRequest`/`TokenExchangeResult` domain types for performing RFC 8693 token exchange on behalf of authenticated users (MCP Enterprise Managed Authorization, SEP-990).
- `NewOAuthTokenExchanger` constructs a `TokenExchanger` backed by the MCP SDK's `oauthex.ExchangeToken`; defaults to `oauthex.TokenTypeIDJAG` for `RequestedTokenType` when unset.
- `EnterpriseAuthConfig.TokenExchanger` wires a `TokenExchanger` into the gateway so custom `CredentialsProvider` and `PolicyEvaluator` implementations can mint downstream tokens from the inbound user token without knowing IdP topology.
- `ErrTokenExchangerConfigRequired` sentinel error for missing required configuration (endpoint or client credentials).

### Changed

#### Circuit Breaker Internals

- `mcp.CircuitBreaker` is now a first-class state machine type with explicit `Acquire`, `OnSuccess`, `OnFailure`, `Reset`, and `Peek` methods, replacing the fields-on-actor approach in `toolSupervisor`.
- `mcp.CircuitAcquireOutcome` enum and `mcp.CircuitTransition` struct describe `Acquire` results and state transitions explicitly, with stable `CircuitTransitionReason` constants (`failure_threshold_exceeded`, `half_open_probe_failed`, `half_open_probe_success`, `open_duration_elapsed`, `manual_reset`).
- `NewCircuitBreakerWithClock` accepts an injectable clock for deterministic tests.
- `toolSupervisor` delegates all circuit logic to `mcp.CircuitBreaker`; audit and telemetry emission go through a single `emitTransition` helper.

#### MCP Resources (List + Read)

- `ResourceSchema` and `ResourceTemplateSchema` domain types for resource metadata discovered from backend MCP servers via `resources/list` and `resources/templates/list`
- `ResourceFetcher` interface for discovering resources from backend MCP servers at registration time
- `ResourceExecutor` optional interface on `ToolExecutor` for handling `resources/read` invocations
- `Resources` and `ResourceTemplates` fields on `Tool` struct, populated at registration time and returned by `ListTools`
- HTTP, stdio, and gRPC transport-specific `FetchResources` functions mirroring the existing schema fetch pattern
- `CompositeResourceFetcher` that routes to the appropriate transport-specific fetcher based on the tool's transport type
- `ResourceFetcherExtension` actor system extension for dependency injection into the registrar
- Registrar actor caches resource metadata alongside tool schemas, clears on tool removal, and attaches to `ListTools` results
- Session actor method-based dispatch : `resources/read` invocations are routed to `ResourceExecutor.ReadResource` on the executor
- `ReadResource` method on `HTTPExecutor` and `StdioExecutor` : proxies `resources/read` to backend MCP servers via the SDK's `ClientSession.ReadResource`
- Ingress `RegisterResources` function : registers resources and resource templates on the per-session SDK server with handler closures that dispatch through the gateway
- `DispatchResourceRead` ingress dispatch function : translates SDK `ReadResourceRequest` into a gateway `Invocation` with method `resources/read`, including OAuth scope propagation
- `ExecutionResultToReadResourceResult` and `OutputToReadResourceResult` conversion functions for mapping gateway results back to SDK types
- `ReadResourceResultToOutput` and `ResourceParamsFromInvocation` egress conversion helpers in `mcpconv`
- `SDKResourcesToSchemas` and `SDKResourceTemplatesToSchemas` conversion helpers in `schemaconv`
- Resources capability is automatically advertised by the SDK server when resources are registered on the per-session server
- Resource reads flow through the full actor pipeline (Router → Registrar → ToolSupervisor → Session → Executor) and benefit from circuit breaking, passivation, executor recovery, policy evaluation, credential brokering, and audit journaling
- gRPC egress returns empty resource slices (gRPC tools use protobuf service definitions, not MCP resources)

#### gRPC Ingress Transport

- `MCPToolService` gRPC service definition (`protos/mcp/v1/mcp_tool_service.proto`) with `ListTools`, `CallTool`, and `CallToolStream` RPCs
- `GRPCIdentityResolver` interface for extracting tenant and client identity from gRPC metadata via `metadata.FromIncomingContext`
- `GRPCIngressConfig` configuration type for the gRPC ingress handler
- `Gateway.RegisterGRPCService(srv, cfg)` method to register the MCP gRPC service on a user-provided `grpc.Server`
- `StreamInvoker` interface extending `Invoker` with `InvokeStream` for streaming progress support in the gRPC ingress
- Tool name resolution supporting both single-schema tools (tool name matches tool ID) and multi-schema tools (schema name maps to parent tool ID)
- Full server-streaming support via `CallToolStream` RPC delivering progress events followed by the final result
- Proto messages use JSON-encoded bytes for arguments and schemas, enabling transport-agnostic forwarding of arbitrary tool parameters
- Enterprise-managed authorization support via gRPC interceptors: `GRPCAuthInterceptors` returns unary and stream interceptors that validate Bearer tokens from gRPC metadata, enforce required scopes, and store validated `TokenInfo` in context
- `NewGRPCTokenIdentityResolver` maps validated bearer token claims to `TenantID`/`ClientID` for gRPC contexts
- `GRPCContextWithTokenInfo` / `GRPCTokenInfoFromContext` for storing and retrieving validated token info in gRPC contexts
- Auto-install of token-based `GRPCIdentityResolver` when `EnterpriseAuth` is set and `IdentityResolver` is nil (same pattern as HTTP ingress)
- OAuth scope propagation from validated gRPC bearer tokens through to `Invocation.Scopes` for scope-aware policy evaluation
- Tool name TTL cache (`ToolCacheTTL` on `GRPCIngressConfig`) avoids per-request `ListTools` actor Ask with configurable TTL (default 5s, negative disables)
- `ingress-grpc` example demonstrating metadata-based identity resolution and all three RPCs
- Comprehensive test suite (47 tests) using `google.golang.org/grpc/test/bufconn` for in-process gRPC testing
- Earthfile `protogen-ingress` target for generating Go code from the ingress proto definition

#### gRPC Egress Transport

- `TransportGRPC` transport type for tools backed by gRPC services
- `GRPCTransportConfig` with target, service, method, TLS, and static metadata support
- User-provided proto descriptor set (`.binpb`) for production environments where server reflection is unavailable
- Optional gRPC server reflection mode for development and staging environments
- Dynamic protobuf message construction via `dynamicpb` — no compiled `.pb.go` types required at gateway build time
- Automatic JSON Schema derivation from proto message descriptors for MCP tool schema discovery
- Server-streaming RPC support via `ToolStreamExecutor` interface
- Connectivity verification during schema fetch (mirrors HTTP/stdio live-fetch pattern)
- Reuses existing `TLSClientConfig` and `security.BuildClientTLSConfig` for mTLS support
- Integrated into `CompositeExecutorFactory` and `CompositeSchemaFetcher` — no gateway wiring changes needed

#### Enterprise-Managed Authorization (MCP Extension)

- MCP enterprise-managed authorization extension (`io.modelcontextprotocol/enterprise-managed-authorization`) support for centralized access control through enterprise identity providers
- `EnterpriseAuthConfig` on `IngressConfig` enables Bearer token enforcement per RFC 9728 on all ingress transports (Streamable HTTP, SSE, WebSocket)
- `IdentityMapper` interface for mapping validated token claims to the gateway's `TenantID` and `ClientID` identity model
- `IdentityMapperFunc` adapter for using plain functions as `IdentityMapper`
- `DefaultIdentityMapper()` maps `TokenInfo.UserID` to `ClientID` with `TenantID` defaulting to "default"
- `NewTokenIdentityResolver()` creates an `IdentityResolver` backed by validated bearer tokens from request context
- `ProtectedResourceMetadataHandler()` serves OAuth 2.0 Protected Resource Metadata (RFC 9728) for client discovery of authorization servers
- OAuth scope propagation from bearer tokens through to `PolicyInput.Scopes`, enabling scope-aware custom `PolicyEvaluator` implementations
- `Scopes` field on `Invocation` carries granted OAuth scopes through the full request lifecycle
- `Scopes` field on `PolicyInput` exposes granted scopes to custom policy evaluators
- Automatic `IdentityResolver` installation when `EnterpriseAuth` is configured without an explicit resolver
- `RequiredScopes` enforcement at the middleware level with HTTP 403 Forbidden for insufficient scopes
- Validation of `EnterpriseAuthConfig` (requires `TokenVerifier`, `ResourceMetadata` with non-empty `Resource`)
- `OAuthHandler` field on `HTTPTransportConfig` enables enterprise-managed authorization on outbound connections to tool backends, supporting the full 3-step flow (OIDC login, RFC 8693 token exchange, RFC 7523 JWT bearer grant) via the SDK's `extauth.EnterpriseHandler`
- Built on the MCP Go SDK's `auth` and `oauthex` packages for standards-compliant OAuth 2.0 token verification, RFC 8693 token exchange, and RFC 7523 JWT bearer grants

## [v0.1.0] - 2026-03-15

Initial release of goakt-mcp -- a production-ready MCP gateway library for Go, built on the GoAkt actor framework.

### Added

#### Multi-Transport Ingress

- Streamable HTTP handler (`Gateway.Handler`) supporting the MCP 2025-11-25 spec
- Server-Sent Events handler (`Gateway.SSEHandler`) supporting the MCP 2024-11-05 spec
- WebSocket handler (`Gateway.WSHandler`) for full-duplex streaming
- Identity resolution and session management across all transports

#### Multi-Transport Egress

- Stdio executor for child-process tool backends communicating over stdin/stdout
- HTTP executor for remote MCP server connectivity with Streamable HTTP semantics
- Automatic schema discovery and caching from tool backends
- W3C trace-context header propagation on outbound calls

#### Multi-Tenancy and Authorization

- Per-request tenant and client identity resolution via the `IdentityResolver` interface
- Per-tenant quota enforcement with configurable rate limits and concurrency caps
- Two-layer policy evaluation: built-in checks (rate limits, concurrency, tool authorization) and custom `PolicyEvaluator` for context-aware decisions (OPA, ABAC, allowlists)
- Every policy decision recorded in the audit journal

#### Credential Brokering

- Per-tool, per-tenant secret resolution via the `CredentialsProvider` interface
- Multiple providers with ordered evaluation
- Configurable LRU cache with tunable TTL and max entries

#### Circuit Breaking and Resilience

- Per-tool circuit breakers with closed, open, and half-open states
- Configurable failure threshold, open duration, and half-open max requests
- Session executor recovery with transparent failover on transport failures
- Periodic health probing on every tool with automatic state transitions
- Per-tool session concurrency limits and bounded audit mailbox for backpressure

#### Session Management

- One session actor per tool + tenant + client combination for stateful operations
- Session affinity modes: `sticky` (session reuse) and `least_loaded` (load-balanced routing)
- Automatic session passivation after configurable idle timeout

#### Dynamic Tool Management

- Register, update, enable, disable, drain, and remove tools at runtime without restart
- Drain mode to stop accepting new sessions while existing ones complete
- Tool state management: enabled, degraded, unavailable, disabled

#### Streaming

- `InvokeStream()` API with a progress channel for intermediate notifications
- `StreamingResult` with `Collect()` convenience method

#### Cluster Mode

- Multi-node deployment with gossip-based membership via GoAkt clustering
- Distributed actor messaging via GoAkt remoting
- Cluster singleton registrar ensuring exactly one registry instance across the cluster
- Pluggable peer discovery via the `DiscoveryProvider` interface with built-in Kubernetes support
- TLS support for all remoting and cluster communication
- Graceful shutdown with ordered pod termination

#### Observability

- OpenTelemetry metrics: invocation latency (histogram), invocation failures (counter), tool availability state (counter), circuit state transitions (counter), active sessions (up-down counter), session lifecycle events (counters), credential cache results (counter), policy evaluation latency (histogram)
- Distributed tracing with end-to-end spans from ingress to egress and W3C trace propagation
- Pluggable structured logger interface with correlation fields (tenant ID, tool ID, request ID, trace ID)

#### Durable Audit Trail

- Structured `AuditEvent` capture for policy decisions, invocation lifecycle, health transitions, and circuit state changes
- Pluggable `AuditSink` interface with built-in `MemorySink` and `FileSink` implementations
- Bounded mailbox with configurable capacity for backpressure

#### Admin API

- `GetGatewayStatus()` for overall gateway health and tool count
- `GetToolStatus()` for per-tool status, circuit state, sessions, and schemas
- `ListSessions()` for all active sessions across all tools
- `DrainTool()` and `ResetCircuit()` for operational control

#### Error Handling

- Comprehensive error codes: `ErrCodeToolUnavailable`, `ErrCodePolicyDenied`, `ErrCodeTransportFailure`, `ErrCodeTimeout`, `ErrCodeThrottled`, `ErrCodeConcurrencyLimitReached`, `ErrCodeInvalidRequest`, `ErrCodeInternal`
- Rich execution results with status, output map, error detail, duration, and correlation metadata

#### Examples

- `filesystem` -- minimal gateway with a stdio filesystem tool
- `audit-http` -- durable file audit sink with HTTP egress
- `ingress` -- MCP Streamable HTTP ingress with header-based identity
- `admin-policy` -- full admin API and custom policy evaluator
- `quota-assess` -- per-tenant rate limiting and concurrency enforcement
- `full-config` -- complete configuration reference
- `ai-hub` -- production-grade multi-tenant hub with stdio + HTTP egress, policy, credentials, audit, and OpenTelemetry
- `cluster` -- three-node Kubernetes cluster with peer discovery, nginx session affinity, and Jaeger tracing

#### CI/CD

- GitHub Actions pipeline with linting (`golangci-lint`), testing with race detection, and coverage reporting via Codecov

[v0.1.0]: https://github.com/tochemey/goakt-mcp/releases/tag/v0.1.0
