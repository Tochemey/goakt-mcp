<h2 align="center">
  <img src="assets/logo.png" alt="Distributed MCP gateway library" width="800"/><br />
  Distributed MCP gateway library
</h2>

[![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/Tochemey/goakt-mcp/gha-pipeline.yml)](https://github.com/Tochemey/goakt-mcp/actions/workflows/gha-pipeline.yml)
[![codecov](https://codecov.io/gh/Tochemey/goakt-mcp/graph/badge.svg?token=EkuaJqCDZr)](https://codecov.io/gh/Tochemey/goakt-mcp)
[![GitHub go.mod Go version](https://badges.chse.dev/github/go-mod/go-version/Tochemey/goakt-mcp)](https://go.dev/doc/install)


goakt-mcp is an MCP gateway library built in Go. It provides a resilient, observable, and scalable execution layer for tool calls through a programmatic Go API.

Instead of acting as a thin JSON-RPC proxy, goakt-mcp is designed as an operational control plane for MCP workloads. It manages tool lifecycle, session affinity, supervision, routing, credential brokering, policy enforcement, and auditability behind a single `Gateway` API.

## Status

The project is in active development. The core runtime (Gateway, Router, Registrar, ToolSupervisor, CredentialBroker, Policy, Health, Journal) is implemented. The MCP Streamable HTTP ingress handler (`Gateway.Handler`) is implemented and tested. The admin operational API (`GetToolStatus`, `GetGatewayStatus`, `ListSessions`, `DrainTool`, `ResetCircuit`) and the pluggable policy engine (`mcp.PolicyEvaluator`) are implemented. Cluster mode, telemetry, and audit are supported. The API and configuration model may still evolve before a stable release.

## Why goakt-mcp

MCP workloads are stateful, concurrent, and failure-prone:

- Some MCP servers are local child processes while others are remote services
- Tools may be slow, bursty, or unreliable
- Session state often needs sticky ownership
- Observability, restart behavior, and auditability are operational requirements, not optional extras

The actor model is a strong fit for this problem space. goakt-mcp is built on [GoAkt](https://github.com/Tochemey/goakt) to model tools, sessions, and control-plane workflows with clear supervision and lifecycle boundaries.

## Goals

goakt-mcp is intended to provide:

- A programmatic Go API for MCP tool invocation and management
- Support for both stdio and HTTP-based MCP transports
- Per-tool supervision, circuit breakers, passivation, and backpressure
- Sticky session ownership per tenant + client + tool
- Cluster-aware routing and failover
- Dynamic tool registration and discovery
- Credential broker integration (pluggable providers)
- Tenant-aware policy enforcement, quotas, and auditing
- OpenTelemetry traces, metrics, structured logs, and durable audit records

## Quick Start

For a minimal runnable example, see [examples/filesystem](examples/filesystem):

```bash
go run ./examples/filesystem
```

For the most comprehensive example — a real-world multi-tenant AI tool hub covering egress (stdio + HTTP), ingress (MCP Streamable HTTP), pluggable policy evaluation, credential broker, durable audit, runtime config, telemetry, and the complete admin API — see [examples/ai-hub](examples/ai-hub):

```bash
# Prerequisites: Node.js and npx (for the filesystem MCP server)
go run ./examples/ai-hub
```

Key environment variables for `ai-hub`:

| Variable                      | Default                     | Description                                  |
|-------------------------------|-----------------------------|----------------------------------------------|
| `MCP_FS_ROOT`                 | `.`                         | Root directory served by the filesystem tool |
| `MCP_HTTP_URL`                | `http://localhost:3001/mcp` | URL of an optional HTTP MCP tool             |
| `MCP_AUDIT_DIR`               | `$TMPDIR/goakt-mcp-ai-hub`  | Directory for the NDJSON audit log           |
| `MCP_ADDR`                    | `127.0.0.1:0`               | Listen address for the MCP HTTP ingress      |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | *(unset)*                   | OTLP endpoint for traces and metrics         |

Other focused examples are in [examples/](examples/):

| Example                               | Demonstrates                                      |
|---------------------------------------|---------------------------------------------------|
| [filesystem](examples/filesystem)     | Minimal egress (stdio tool)                       |
| [audit-http](examples/audit-http)     | Durable audit + HTTP egress                       |
| [ingress](examples/ingress)           | MCP Streamable HTTP ingress + identity resolution |
| [admin-policy](examples/admin-policy) | Admin API + pluggable policy evaluator            |
| [quota-assess](examples/quota-assess) | Tenant quota enforcement                          |
| [full-config](examples/full-config)   | Full configuration reference                      |

## API

The `Gateway` is the sole public entry point. Construct it with a `mcp.Config` and optional options, then call methods:

| Method                       | Description                                       |
|------------------------------|---------------------------------------------------|
| `New(cfg, ...opts)`          | Create a new Gateway                              |
| `Start(ctx)`                 | Start the actor system and bootstrap tools        |
| `Stop(ctx)`                  | Gracefully shut down                              |
| `Invoke(ctx, inv)`           | Execute a tool invocation                         |
| `ListTools(ctx)`             | List all registered tools                         |
| `RegisterTool(ctx, tool)`    | Dynamically register a tool                       |
| `UpdateTool(ctx, tool)`      | Update a tool's metadata                          |
| `DisableTool(ctx, toolID)`   | Disable a tool                                    |
| `EnableTool(ctx, toolID)`    | Re-enable a previously disabled tool              |
| `RemoveTool(ctx, toolID)`    | Remove a tool                                     |
| `Handler(cfg)`               | Return an HTTP handler for MCP clients            |
| `GetToolStatus(ctx, toolID)` | Return circuit state, session count, drain status |
| `GetToolSchema(ctx, toolID)` | Return cached MCP tool schemas for a tool         |
| `GetGatewayStatus(ctx)`      | Return overall gateway health and counts          |
| `ListSessions(ctx)`          | List all active sessions across all tools         |
| `DrainTool(ctx, toolID)`     | Stop accepting new sessions for a tool            |
| `ResetCircuit(ctx, toolID)`  | Manually reset a tripped circuit breaker          |

### Admin / Operational API

The admin methods provide operational visibility and control over running tools. They are safe to call while the gateway is serving traffic.

```go
// Inspect the current state of a tool: circuit state, session count, drain status.
status, err := gw.GetToolStatus(ctx, "my-tool")
fmt.Println(status.Circuit, status.SessionCount, status.Draining)

// Overall gateway health: whether it is running, tool and session counts.
gwStatus, err := gw.GetGatewayStatus(ctx)
fmt.Println(gwStatus.Running, gwStatus.ToolCount, gwStatus.SessionCount)

// List every active session across all tools.
sessions, err := gw.ListSessions(ctx)
for _, s := range sessions {
    fmt.Println(s.Name, s.ToolID, s.TenantID, s.ClientID)
}

// Stop a tool from accepting new sessions while existing sessions drain.
err = gw.DrainTool(ctx, "my-tool")

// Manually close a circuit breaker that tripped after transient failures.
err = gw.ResetCircuit(ctx, "my-tool")
```

**`mcp.ToolStatus` fields**

| Field          | Type               | Description                                                       |
|----------------|--------------------|-------------------------------------------------------------------|
| `ToolID`       | `mcp.ToolID`       | Identifier of the tool.                                           |
| `State`        | `mcp.ToolState`    | Current operational state (`enabled`, `disabled`, `degraded`, …). |
| `Circuit`      | `mcp.CircuitState` | Circuit breaker state (`closed`, `open`, `half_open`).            |
| `SessionCount` | `int`              | Number of active sessions for this tool.                          |
| `Draining`     | `bool`             | `true` when the tool is draining (no new sessions accepted).      |
| `Schemas`      | `[]mcp.ToolSchema` | MCP tool schemas discovered from the backend via `tools/list`.    |

**`mcp.GatewayStatus` fields**

| Field          | Type   | Description                                          |
|----------------|--------|------------------------------------------------------|
| `Running`      | `bool` | `true` when the gateway actor system is operational. |
| `ToolCount`    | `int`  | Number of tools currently registered.                |
| `SessionCount` | `int`  | Total active sessions across all tools.              |

**`mcp.SessionInfo` fields**

| Field      | Type           | Description                              |
|------------|----------------|------------------------------------------|
| `Name`     | `string`       | Deterministic actor name of the session. |
| `ToolID`   | `mcp.ToolID`   | Tool the session is bound to.            |
| `TenantID` | `mcp.TenantID` | Tenant the session was created for.      |
| `ClientID` | `mcp.ClientID` | Client the session was created for.      |

### Tool Schema Discovery

When a tool is registered (via `RegisterTool` or config bootstrap), the gateway connects to the backend MCP server, calls `tools/list`, and caches the returned schemas. These schemas contain the tool's name, description, and JSON Schema for input parameters.

Cached schemas are:
- Returned by `GetToolSchema(ctx, toolID)` for programmatic access
- Included in `ToolStatus` via `GetToolStatus`
- Attached to tools returned by `ListTools`
- Used by the ingress handler to register tools with their actual schema instead of a permissive fallback

If schema fetching fails (backend unreachable, timeout), the tool is still registered and operates with a pass-through `{"type":"object"}` schema.

```go
schemas, err := gw.GetToolSchema(ctx, "my-tool")
for _, s := range schemas {
    fmt.Println(s.Name, s.Description, string(s.InputSchema))
}
```

### Session Executor Recovery

When a tool invocation fails due to a transport error, the session actor transparently attempts recovery. Recovery triggers when either:

- `ToolExecutor.Execute` returns a non-nil Go error (unexpected crash), or
- The result contains `ErrCodeTransportFailure` (connection drop, stdio process crash) — this is how the built-in stdio and HTTP executors surface transport failures.

Steps:

1. The failed executor is closed
2. A fresh executor is created via the `ExecutorFactory`
3. The invocation is retried once with a fresh timeout context
4. If recovery fails, the original error is returned and reported to the circuit breaker

This eliminates the need to wait for session passivation and recreation on transient failures.

### MCP Streamable HTTP ingress

`Gateway.Handler` returns an `http.Handler` that serves the [MCP Streamable HTTP transport](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports#streamable-http). Mount it in your own HTTP server or router to expose the gateway to MCP clients (Claude Desktop, LLM agents, etc.).

```go
import (
    "net/http"
    "github.com/tochemey/goakt-mcp/mcp"
)

// 1. Implement mcp.IdentityResolver to extract tenant + client identity
//    from each incoming MCP session request (JWT, API key, mTLS cert, etc.).
type headerResolver struct{}

func (r *headerResolver) ResolveIdentity(req *http.Request) (mcp.TenantID, mcp.ClientID, error) {
    tenant := mcp.TenantID(req.Header.Get("X-Tenant-ID"))
    client := mcp.ClientID(req.Header.Get("X-Client-ID"))
    if tenant.IsZero() {
        return "", "", errors.New("missing X-Tenant-ID header")
    }
    return tenant, client, nil
}

// 2. Build and start the gateway (tools and tenants configured via mcp.Config).
gw, _ := goaktmcp.New(cfg)
_ = gw.Start(ctx)

// 3. Get the ingress handler and mount it.
h, err := gw.Handler(mcp.IngressConfig{
    IdentityResolver:   &headerResolver{},
    SessionIdleTimeout: 10 * time.Minute,
})
if err != nil {
    log.Fatal(err)
}

http.Handle("/mcp", h)
log.Fatal(http.ListenAndServe(":8080", nil))
```

**How it works**

- `IdentityResolver.ResolveIdentity` is called once per new MCP session. A non-nil error rejects the session with HTTP 400.
- All tools currently registered in the gateway are registered with the MCP session at connection time. Tool additions after a session is established take effect on the next new session.
- In **stateful** mode (default), each client gets a unique `Mcp-Session-Id`; identity resolution runs once per connection.
- In **stateless** mode (`Stateless: true`), a new session is created for every HTTP request. Useful for load-balanced deployments without sticky sessions.

**`IngressConfig` fields**

| Field                | Type                   | Description                                                                |
|----------------------|------------------------|----------------------------------------------------------------------------|
| `IdentityResolver`   | `mcp.IdentityResolver` | Required. Extracts tenant + client identity from each new session request. |
| `SessionIdleTimeout` | `time.Duration`        | Close idle sessions after this duration. Zero means never (default).       |
| `Stateless`          | `bool`                 | When true, every HTTP request is an independent session. Default: `false`. |

### Pluggable Policy Engine

By default, goakt-mcp enforces authorization (tenant allowlist) and quota limits (rate and concurrency). You can extend policy evaluation per tenant by implementing `mcp.PolicyEvaluator`:

```go
type PolicyEvaluator interface {
    Evaluate(ctx context.Context, input mcp.PolicyInput) *mcp.RuntimeError
}
```

The custom evaluator is called **after** all built-in checks pass. Return `nil` to allow the invocation, or a `*mcp.RuntimeError` to deny it.

```go
// Example: OPA-backed or RBAC evaluator.
type myPolicyEvaluator struct{ opaClient *opa.Client }

func (e *myPolicyEvaluator) Evaluate(ctx context.Context, in mcp.PolicyInput) *mcp.RuntimeError {
    allowed, err := e.opaClient.Check(ctx, in.TenantID, in.ToolID)
    if err != nil || !allowed {
        return mcp.NewRuntimeError(mcp.ErrCodePolicyDenied, "OPA denied the request")
    }
    return nil
}

cfg := mcp.Config{
    Tenants: []mcp.TenantConfig{
        {
            ID:        "enterprise-tenant",
            Evaluator: &myPolicyEvaluator{opaClient: opaClient},
            Quotas: mcp.TenantQuotaConfig{
                RequestsPerMinute:  500,
                ConcurrentSessions: 50,
            },
        },
    },
}
```

**`mcp.PolicyInput` fields** (passed to every `Evaluate` call)

| Field                     | Type           | Description                                                        |
|---------------------------|----------------|--------------------------------------------------------------------|
| `TenantID`                | `mcp.TenantID` | Resolved tenant identity for this invocation.                      |
| `ToolID`                  | `mcp.ToolID`   | Identifier of the tool being invoked.                              |
| `ActiveSessionCount`      | `int`          | Live sessions for this tenant across all tools at evaluation time. |
| `RequestsInCurrentMinute` | `int`          | Requests from this tenant in the current minute window.            |

### Options

- `WithLogger(logger Logger)` — Plug in a custom logging backend (zap, zerolog, slog, logrus, etc.) by implementing the four-method `Logger` interface (`Debug`, `Info`, `Warn`, `Error`). The gateway wraps it to satisfy the engine's internal logger. If the `Logger` also implements the optional `LeveledLogger` interface (`Level() string`), the adapter uses its level for engine-side log gating; otherwise it defaults to `InfoLevel`. Passing `nil` is a no-op — the logger set via `mcp.Config.LogLevel` (if any) is preserved.
- `WithDebug()` — Enable verbose debug logging from the underlying engine to stdout. Useful for diagnosing actor lifecycle, message routing, and cluster events.
- `WithMetrics()` — Enable OpenTelemetry metrics (invocation latency, failures, tool availability, circuit state).
- `WithTracing()` — Enable OpenTelemetry tracing and W3C trace-context propagation on egress.

Log level can also be set declaratively via `mcp.Config.LogLevel` (e.g. `"info"`, `"debug"`, `"error"`). When both `LogLevel` and `WithLogger` are set, the programmatic option takes precedence.

## Configuration

Build a `mcp.Config` with your tools, tenants, and runtime settings. Zero-valued fields are filled with defaults by `New`.

```go
import (
 "time"
 "github.com/tochemey/goakt-mcp/mcp"
)

cfg := mcp.Config{
 LogLevel: "info",
 Runtime: mcp.RuntimeConfig{
  SessionIdleTimeout:  5 * time.Minute,
  RequestTimeout:     30 * time.Second,
  StartupTimeout:     10 * time.Second,
  HealthProbeInterval: 30 * time.Second,
  ShutdownTimeout:    30 * time.Second,
 },
 Tools: []mcp.Tool{
  {
   ID:        "filesystem",
   Transport: mcp.TransportStdio,
   Stdio:     &mcp.StdioTransportConfig{Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"}},
   State:     mcp.ToolStateEnabled,
  },
 },
}
```

### Config sections

- **Runtime** — Session idle timeout, request timeout, startup timeout, health probe interval, shutdown timeout. All fields default when zero (see `mcp` package constants).
- **Cluster** — Multi-node mode. When `Enabled` is true, set `Discovery` to `"kubernetes"` or `"dnssd"` and supply the corresponding config (`Kubernetes` or `DNSSD`). Set `Cluster.TLS` to a `RemotingTLSConfig` to enable TLS for remoting and cluster communication (all nodes must share the same root CA). Set `SingletonRole`, `PeersPort`, and `RemotingPort` to match your network topology.
- **Telemetry** — OpenTelemetry export (e.g. `OTLPEndpoint`).
- **Audit** — Set `Audit.Sink` to an implementation of `mcp.AuditSink` for durable audit events. `Audit.MailboxSize` caps the journal actor's in-flight event queue (defaults to `DefaultAuditMailboxSize = 1024`); senders block when full, providing backpressure. Omit `Sink` for discard-only (testing).
- **Credentials** — Set `Credentials.Providers` to a slice of `mcp.CredentialsProvider` and optionally `Credentials.CacheTTL` and `Credentials.MaxCacheEntries`. Used by the credential broker when tools require secrets.
- **Tenants** — Per-tenant config with quota limits: `RequestsPerMinute` (rate limiting) and `ConcurrentSessions` (concurrency cap). Requests from tenants not listed here are accepted without quota enforcement. Set `TenantConfig.Evaluator` to a `mcp.PolicyEvaluator` implementation to add custom authorization logic (OPA, RBAC, etc.) that runs after built-in checks pass. See [Pluggable Policy Engine](#pluggable-policy-engine).
- **HealthProbe** — `HealthProbe.Interval` controls how often the health actor probes tool supervisors. `HealthProbe.Timeout` caps each probe cycle.
- **Tools** — Tool definitions to register at startup. Each tool has: transport (`stdio` or `http`), transport config (`Stdio` or `HTTP`), `State` (`enabled`/`disabled`), `RequestTimeout`, `IdleTimeout`, `MaxSessionsPerTool` (concurrency backpressure), `Circuit` (circuit breaker tuning via `CircuitConfig`), `CredentialPolicy`, `AuthorizationPolicy`, and `Routing` mode.

See `mcp.Config` and related types in the `mcp` package for the full configuration model.

### Cluster discovery

When `Cluster.Enabled` is true, discovery can be:

- **Kubernetes** — In-cluster pod discovery via `KubernetesDiscoveryConfig` (namespace, port names, pod labels).
- **DNS-SD** — DNS-based service discovery via `DNSSDDiscoveryConfig` (domain name, optional IPv6) for local development.

## License

This project is licensed under the terms in `LICENSE`.
