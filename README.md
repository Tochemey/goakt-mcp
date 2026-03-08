<h2 align="center">
  <img src="assets/logo.png" alt="GoAkt - Distributed Actor framework for Go" width="800"/><br />
  Distributed MCP gateway library built in Go using the actor model
</h2>

---
[![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/Tochemey/goakt-mcp/gha-pipeline.yml)](https://github.com/Tochemey/goakt-mcp/actions/workflows/gha-pipeline.yml)
[![codecov](https://codecov.io/gh/Tochemey/goakt-mcp/graph/badge.svg?token=EkuaJqCDZr)](https://codecov.io/gh/Tochemey/goakt-mcp)


goakt-mcp is a production-oriented MCP gateway library built in Go. It provides a resilient, observable, and scalable execution layer for tool calls through a programmatic Go API.

Instead of behaving like a thin JSON-RPC proxy, goakt-mcp is designed as an operational control plane for MCP workloads. It manages tool lifecycle, session affinity, supervision, routing, credential brokering, policy enforcement, and auditability behind a single `Gateway` API.

## Status

The project is in the design and buildout phase. The architecture and product direction are defined; the runtime is intended to support production-ready v1 capabilities. This repository should be read as a project in progress, not as a finished library.

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
- Credential broker integration
- Tenant-aware policy enforcement, quotas, and auditing
- OpenTelemetry traces, metrics, structured logs, and durable audit records

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"

	goaktmcp "github.com/tochemey/goakt-mcp"
	"github.com/tochemey/goakt-mcp/mcp"
)

func main() {
	cfg := mcp.Config{
		Tools: []mcp.Tool{
			{
				ID:        "filesystem",
				Transport: mcp.TransportStdio,
				Stdio:     &mcp.StdioTransportConfig{Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"}},
				State:     mcp.ToolStateEnabled,
			},
		},
	}

	gw, err := goaktmcp.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	if err := gw.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer gw.Stop(ctx)

	// List registered tools
	tools, _ := gw.ListTools(ctx)
	fmt.Printf("Registered tools: %d\n", len(tools))

	// Invoke a tool
	result, err := gw.Invoke(ctx, &mcp.Invocation{
		ToolID: "filesystem",
		Method: "tools/call",
		Params: map[string]any{
			"name":      "list_directory",
			"arguments": map[string]any{"path": "/tmp"},
		},
		Correlation: mcp.CorrelationMeta{
			TenantID: "acme-dev",
			ClientID: "my-app",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Result: %v\n", result.Status)

	// Dynamically register a new tool
	_ = gw.RegisterTool(ctx, mcp.Tool{
		ID:        "new-tool",
		Transport: mcp.TransportHTTP,
		HTTP:      &mcp.HTTPTransportConfig{URL: "https://mcp.example.com"},
		State:     mcp.ToolStateEnabled,
	})
}
```

## API

The `Gateway` is the sole public entry point. Users construct it with a `Config` and options, then call methods:

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

## Configuration

Configuration is constructed programmatically. Build a `Config` struct with your tools, tenants, and runtime settings:

```go
import "github.com/tochemey/goakt-mcp/mcp"

cfg := mcp.Config{
    LogLevel: "info",
    Runtime: mcp.RuntimeConfig{
        SessionIdleTimeout: 5 * time.Minute,
        RequestTimeout:     30 * time.Second,
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

Defaults are applied for zero-valued runtime fields. See `mcp.Config` and related types for the full configuration model.

### Cluster Discovery

goakt-mcp supports multi-node clustering with:

- **Kubernetes**: In-cluster pod discovery using labels and port names
- **DNS-SD**: DNS-based service discovery for local development

## License

This project is licensed under the terms in `LICENSE`.
