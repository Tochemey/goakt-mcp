<h2 align="center">
  <img src="assets/logo.png" alt="GoAkt - Distributed Actor framework for Go" width="800"/><br />
  Distributed MCP gateway built in Go
</h2>

---

goakt-mcp is a production-oriented MCP gateway built in Go. It sits between AI agents and MCP servers and provides a resilient, observable, and scalable execution layer for tool calls.

Instead of behaving like a thin JSON-RPC proxy, goakt-mcp is designed as an operational control plane for MCP workloads. It manages tool lifecycle, session affinity, supervision, routing, credential brokering, policy enforcement, and auditability behind a single gateway API.

## Status

The project is in the design and buildout phase. The architecture and product direction are defined; the runtime is intended to support production-ready v1 capabilities. This repository should be read as a project in progress, not as a finished gateway.

## Why goakt-mcp

MCP workloads are stateful, concurrent, and failure-prone:

- Some MCP servers are local child processes while others are remote services
- Tools may be slow, bursty, or unreliable
- Session state often needs sticky ownership
- Observability, restart behavior, and auditability are operational requirements, not optional extras

The actor model is a strong fit for this problem space. goakt-mcp is built on [GoAkt](https://github.com/Tochemey/goakt) to model tools, sessions, and control-plane workflows with clear supervision and lifecycle boundaries.

## Goals

goakt-mcp is intended to provide:

- A single HTTP gateway for MCP tool invocation
- Support for both stdio and HTTP-based MCP transports
- Per-tool supervision, circuit breakers, passivation, and backpressure
- Sticky session ownership per tenant + client + tool
- Cluster-aware routing and failover
- Dynamic tool registration and discovery
- Credential broker integration
- Tenant-aware policy enforcement, quotas, and auditing
- OpenTelemetry traces, metrics, structured logs, and durable audit records

## API Shape

The primary data-plane endpoints are expected to include:

- `POST /v1/tools/{tool}/invoke`
- `GET /v1/tools`
- `GET /healthz`
- `GET /readyz`

The control plane is expected to include endpoints for registering, updating, disabling, and removing tools.

## Example Invoke Request

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

## Example Configuration

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

## Design Document

For the full architecture, runtime decisions, and production scope, see `IDEATION.md`.

## License

This project is licensed under the terms in `LICENSE`.
