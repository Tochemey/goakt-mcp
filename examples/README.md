# goakt-mcp Examples

Concrete, runnable examples demonstrating the goakt-mcp gateway API.

## filesystem

Starts the gateway with the MCP filesystem server, lists registered tools, and invokes `list_directory` to show directory contents. See **[examples/filesystem/README.md](filesystem/README.md)** for a full walkthrough, code flow, and environment variables.

## audit-http

Demonstrates **durable auditing** (FileSink) and **HTTP-based egress** (MCP tool over HTTP). Uses both stdio (filesystem) and HTTP (server-everything) tools, then prints the audit log. See **[examples/audit-http/README.md](audit-http/README.md)** for setup and environment variables.

## full-config

Demonstrates the **majority of the gateway configuration** in one example: Runtime, Telemetry, Audit, Credentials, Tenants, HealthProbe, gateway options (WithMetrics, WithTracing), and mixed stdio + HTTP tools. Use as a reference for production-ready setups. See **[examples/full-config/README.md](full-config/README.md)** for a full walkthrough.

## quota-assess

Efficiently assesses **tenant quota enforcement** in a real-world scenario: tight limits (RPM=5, ConcurrentSessions=2), concurrent invocations to hit session limit, sequential invocations to hit rate limit. See **[examples/quota-assess/README.md](quota-assess/README.md)** for setup and usage.
