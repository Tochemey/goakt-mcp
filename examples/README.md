# goakt-mcp Examples

Concrete, runnable examples demonstrating the goakt-mcp gateway API.

## filesystem

Starts the gateway with the MCP filesystem server, lists registered tools, and invokes `list_directory` to show directory contents. See **[examples/filesystem/README.md](filesystem/README.md)** for a full walkthrough, code flow, and environment variables.

## audit-http

Demonstrates **durable auditing** (FileSink) and **HTTP-based egress** (MCP tool over HTTP). Uses both stdio (filesystem) and HTTP (server-everything) tools, then prints the audit log. See **[examples/audit-http/README.md](audit-http/README.md)** for setup and environment variables.
