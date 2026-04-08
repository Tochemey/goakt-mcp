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

// Package mcpconv provides shared MCP-to-runtime conversion helpers used by
// both the stdio and HTTP egress adapters.
package mcpconv

import (
	"encoding/base64"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tochemey/goakt-mcp/mcp"
)

// ParamsFromInvocation extracts the tool name and arguments from an Invocation.
// Params is expected to contain "name" and "arguments" keys per the MCP tools/call
// convention. When "name" is absent, the ToolID is used as a fallback.
func ParamsFromInvocation(inv *mcp.Invocation) (string, any) {
	if inv == nil || inv.Params == nil {
		return "", nil
	}
	name, _ := inv.Params["name"].(string)
	if name == "" {
		name = string(inv.ToolID)
	}
	return name, inv.Params["arguments"]
}

// CallResultToOutput converts an MCP CallToolResult into the runtime output map.
// Returns nil when res is nil.
func CallResultToOutput(res *sdkmcp.CallToolResult) map[string]any {
	if res == nil {
		return nil
	}
	out := make(map[string]any, 2)
	if len(res.Content) > 0 {
		out["content"] = contentToSlice(res.Content)
	}
	if res.StructuredContent != nil {
		out["structuredContent"] = res.StructuredContent
	}
	return out
}

// ContentErrorText extracts a human-readable error message from a CallToolResult.
// Falls back to "tool error" when no text content is present.
func ContentErrorText(res *sdkmcp.CallToolResult) string {
	if res == nil || len(res.Content) == 0 {
		return "tool error"
	}
	if t, ok := res.Content[0].(*sdkmcp.TextContent); ok {
		return t.Text
	}
	return "tool error"
}

// ResourceParamsFromInvocation extracts the resource URI from an Invocation.
// Params is expected to contain a "uri" key per the MCP resources/read convention.
func ResourceParamsFromInvocation(inv *mcp.Invocation) string {
	if inv == nil || inv.Params == nil {
		return ""
	}
	uri, _ := inv.Params["uri"].(string)
	return uri
}

// ReadResourceResultToOutput converts an MCP ReadResourceResult into the runtime
// output map. Returns nil when res is nil.
func ReadResourceResultToOutput(res *sdkmcp.ReadResourceResult) map[string]any {
	if res == nil {
		return nil
	}
	out := make(map[string]any, 1)
	if len(res.Contents) > 0 {
		contents := make([]map[string]any, 0, len(res.Contents))
		for _, c := range res.Contents {
			if c == nil {
				continue
			}
			item := make(map[string]any, 4)
			item["uri"] = c.URI
			if c.MIMEType != "" {
				item["mimeType"] = c.MIMEType
			}
			if c.Text != "" {
				item["text"] = c.Text
			}
			if len(c.Blob) > 0 {
				item["blob"] = c.Blob
			}
			contents = append(contents, item)
		}
		out["contents"] = contents
	}
	return out
}

func contentToSlice(c []sdkmcp.Content) []map[string]any {
	s := make([]map[string]any, 0, len(c))
	for _, item := range c {
		switch t := item.(type) {
		case *sdkmcp.TextContent:
			s = append(s, map[string]any{
				"type": "text",
				"text": t.Text,
			})
		case *sdkmcp.ImageContent:
			s = append(s, map[string]any{
				"type":     "image",
				"data":     base64.StdEncoding.EncodeToString(t.Data),
				"mimeType": t.MIMEType,
			})
		case *sdkmcp.AudioContent:
			s = append(s, map[string]any{
				"type":     "audio",
				"data":     base64.StdEncoding.EncodeToString(t.Data),
				"mimeType": t.MIMEType,
			})
		case *sdkmcp.EmbeddedResource:
			m := map[string]any{"type": "resource"}
			if t.Resource != nil {
				m["uri"] = t.Resource.URI
				if t.Resource.MIMEType != "" {
					m["mimeType"] = t.Resource.MIMEType
				}
				if t.Resource.Text != "" {
					m["text"] = t.Resource.Text
				}
			}
			s = append(s, m)
		case *sdkmcp.ResourceLink:
			m := map[string]any{
				"type": "resource_link",
				"uri":  t.URI,
				"name": t.Name,
			}
			if t.MIMEType != "" {
				m["mimeType"] = t.MIMEType
			}
			s = append(s, m)
		default:
			s = append(s, map[string]any{"type": "unknown"})
		}
	}
	return s
}
