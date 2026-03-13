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

package mcp

import (
	"context"
	"encoding/json"
)

// ToolSchema describes a single MCP tool capability as reported by a backend
// MCP server via the tools/list method.
type ToolSchema struct {
	// Name is the MCP tool name as reported by the backend server.
	Name string
	// Description is the human-readable description of the tool.
	Description string
	// InputSchema is the JSON Schema describing the tool's input parameters.
	InputSchema json.RawMessage
}

// SchemaFetcher connects to a backend MCP server and retrieves the tool schemas
// it exposes. Implementations create a temporary connection, call tools/list,
// and close the connection. The returned schemas are cached by the registrar.
type SchemaFetcher interface {
	FetchSchemas(ctx context.Context, tool Tool) ([]ToolSchema, error)
}
