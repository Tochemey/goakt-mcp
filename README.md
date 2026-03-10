<h2 align="center">
  <img src="assets/logo.png" alt="Distributed MCP gateway library" width="800"/><br />
  Distributed MCP gateway library
</h2>

[![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/Tochemey/goakt-mcp/gha-pipeline.yml)](https://github.com/Tochemey/goakt-mcp/actions/workflows/gha-pipeline.yml)
[![codecov](https://codecov.io/gh/Tochemey/goakt-mcp/graph/badge.svg?token=EkuaJqCDZr)](https://codecov.io/gh/Tochemey/goakt-mcp)

goakt-mcp is an MCP gateway library built in Go. It provides a resilient, observable, and scalable execution layer for tool calls through a programmatic Go API.

Instead of acting as a thin JSON-RPC proxy, goakt-mcp is designed as an operational control plane for MCP workloads. It manages tool lifecycle, session affinity, supervision, routing, credential brokering, policy enforcement, and auditability behind a single `Gateway` API.

## Status

The project is in active development. The core runtime (Gateway, Router, Registrar, ToolSupervisor, CredentialBroker, Policy, Health, Journal) is implemented. Cluster mode, telemetry, and audit are supported; the API and configuration model may still evolve before a stable release.

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

See [examples/filesystem](examples/filesystem) for a runnable example:

```bash
go run ./examples/filesystem
```

## API

The `Gateway` is the sole public entry point. Construct it with a `mcp.Config` and optional options, then call methods:

| Method                     | Description                                |
|----------------------------|--------------------------------------------|
| `New(cfg, ...opts)`        | Create a new Gateway                       |
| `Start(ctx)`               | Start the actor system and bootstrap tools |
| `Stop(ctx)`                | Gracefully shut down                       |
| `Invoke(ctx, inv)`         | Execute a tool invocation                  |
| `ListTools(ctx)`           | List all registered tools                  |
| `RegisterTool(ctx, tool)`  | Dynamically register a tool                |
| `UpdateTool(ctx, tool)`    | Update a tool's metadata                   |
| `DisableTool(ctx, toolID)` | Disable a tool                             |
| `RemoveTool(ctx, toolID)`  | Remove a tool                              |

### Options

- `WithLogger(level goaktlog.Level)` — Set gateway log level; use `goaktlog.InvalidLevel` to suppress output (e.g. in tests).
- `WithMetrics()` — Enable OpenTelemetry metrics (invocation latency, failures, tool availability, circuit state).
- `WithTracing()` — Enable OpenTelemetry tracing and W3C trace-context propagation on egress.

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

- **Runtime** — Session idle timeout, request timeout, startup timeout, health probe interval, shutdown timeout. Defaults are applied when zero.
- **Cluster** — Multi-node mode. When `Enabled` is true, set `Discovery` to `"kubernetes"` or `"dnssd"` and supply the corresponding config (`Kubernetes` or `DNSSD`). Set `Cluster.TLS` to a `RemotingTLSConfig` to enable TLS for remoting and cluster communication (all nodes must share the same root CA).
- **Telemetry** — OpenTelemetry export (e.g. `OTLPEndpoint`).
- **Audit** — Set `Audit.Sink` to an implementation of `mcp.AuditSink` for durable audit events. Omit for in-memory (testing).
- **Credentials** — Set `Credentials.Providers` to a slice of `mcp.CredentialsProvider` and optionally `Credentials.CacheTTL`. Used by the credential broker when tools require credentials.
- **Tenants** — Per-tenant config and quotas (e.g. `RequestsPerMinute`, `ConcurrentSessions`) for policy and rate limiting.
- **HealthProbe** — Health probe interval for the health actor (`HealthProbe.Interval`).
- **Tools** — Tool definitions to register at startup.

See `mcp.Config` and related types in the `mcp` package for the full configuration model.

### Cluster discovery

When `Cluster.Enabled` is true, discovery can be:

- **Kubernetes** — In-cluster pod discovery via `KubernetesDiscoveryConfig` (namespace, port names, pod labels).
- **DNS-SD** — DNS-based service discovery via `DNSSDDiscoveryConfig` (domain name, optional IPv6) for local development.

## License

This project is licensed under the terms in `LICENSE`.
