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

package http

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tochemey/goakt-mcp/mcp"
)

// dispatchToolCall translates an SDK CallToolRequest into a gateway Invocation,
// forwards it through gw, and converts the result back to an SDK CallToolResult.
//
// All errors (invalid requests, policy denials, timeouts, execution failures)
// are returned as tool errors (IsError: true) so the LLM can observe and
// self-correct. The returned Go error is always nil.
func dispatchToolCall(
	ctx context.Context,
	gw Invoker,
	req *sdkmcp.CallToolRequest,
	tenantID mcp.TenantID,
	clientID mcp.ClientID,
) (*sdkmcp.CallToolResult, error) {
	inv, err := requestToInvocation(req, tenantID, clientID)
	if err != nil {
		r := new(sdkmcp.CallToolResult)
		r.SetError(err)
		return r, nil
	}

	result, gwErr := gw.Invoke(ctx, inv)
	return executionResultToCallToolResult(result, gwErr), nil
}

// requestToInvocation converts a low-level SDK CallToolRequest into the gateway's
// Invocation type. Arguments are unmarshaled from JSON into a map so the egress
// layer can forward them to the backend MCP server via [mcpconv.ParamsFromInvocation].
//
// Backend tool name resolution: req.Params.Name is the gateway tool ID (e.g.
// "filesystem"). The backend MCP server may expose many sub-tools under that
// gateway ID (e.g. "list_directory", "read_file"). MCP clients pass the
// backend tool name and its arguments inside the arguments map using the
// standard MCP tools/call shape:
//
//	{"name": "<backend-tool>", "arguments": {...}}
//
// When args["name"] is present it is used as the backend tool name and
// args["arguments"] as the backend arguments. This allows LLMs and MCP clients
// to route invocations to the correct sub-tool on the backend server.
//
// If args["name"] is absent (single-operation tools where the gateway ID
// equals the backend tool name), req.Params.Name is used as a fallback and the
// entire args map is forwarded as arguments.
func requestToInvocation(
	req *sdkmcp.CallToolRequest,
	tenantID mcp.TenantID,
	clientID mcp.ClientID,
) (*mcp.Invocation, error) {
	if req == nil || req.Params == nil {
		return nil, fmt.Errorf("invalid tool call request: request and params are required")
	}

	var args map[string]any
	if len(req.Params.Arguments) > 0 {
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid tool arguments: %w", err)
		}
	}

	// Determine the backend tool name and arguments.
	backendName, _ := args["name"].(string)
	var backendArgs any
	if backendName != "" {
		// Nested shape: client specified a backend sub-tool name explicitly.
		backendArgs = args["arguments"]
	} else {
		// Flat shape: no sub-tool name; use the gateway tool ID as the backend
		// tool name and forward the entire args map as arguments.
		backendName = req.Params.Name
		backendArgs = args
	}

	return &mcp.Invocation{
		ToolID: mcp.ToolID(req.Params.Name),
		Method: "tools/call",
		Params: map[string]any{
			"name":      backendName,
			"arguments": backendArgs,
		},
		Correlation: mcp.CorrelationMeta{
			TenantID:  tenantID,
			ClientID:  clientID,
			RequestID: newRequestID(),
		},
		ReceivedAt: time.Now(),
	}, nil
}

// executionResultToCallToolResult converts a gateway ExecutionResult (and any
// accompanying gateway error) into an SDK CallToolResult.
//
// Successful results have their output serialized back to SDK content types.
// Non-success statuses (denied, throttled, timeout, failure) are reported as
// tool errors rather than protocol errors, preserving the LLM's ability to
// observe and correct the failure.
func executionResultToCallToolResult(res *mcp.ExecutionResult, gwErr error) *sdkmcp.CallToolResult {
	if gwErr != nil && res == nil {
		r := new(sdkmcp.CallToolResult)
		r.SetError(gwErr)
		return r
	}

	if res == nil {
		r := new(sdkmcp.CallToolResult)
		r.SetError(fmt.Errorf("empty result from gateway"))
		return r
	}

	if res.Status != mcp.ExecutionStatusSuccess {
		errMsg := string(res.Status)
		if res.Err != nil {
			errMsg = res.Err.Error()
		}
		r := new(sdkmcp.CallToolResult)
		r.SetError(fmt.Errorf("%s", errMsg))
		return r
	}

	return outputToCallToolResult(res.Output)
}

// outputToCallToolResult converts the gateway's normalized output map back to
// an SDK CallToolResult. Text content items produced by the egress layer are
// reconstructed into typed [sdkmcp.TextContent] entries; all other content
// falls back to a single JSON-serialized text entry.
//
// The full output map is also attached as StructuredContent so consumers with
// schema-aware tooling can access the raw data without parsing text.
func outputToCallToolResult(output map[string]any) *sdkmcp.CallToolResult {
	if output == nil {
		return &sdkmcp.CallToolResult{}
	}

	result := &sdkmcp.CallToolResult{StructuredContent: output}

	// Attempt to reconstruct typed text content from the "content" key written
	// by mcpconv.CallResultToOutput. Only text items are reconstructed; image,
	// audio, and embedded resource items fall through to JSON serialization.
	if rawContent, ok := output["content"]; ok {
		// json.Unmarshal into interface{} produces []any for arrays, not
		// []map[string]any, so we must handle both the in-memory path (direct
		// []map[string]any from contentToSlice) and the JSON-decoded path ([]any).
		var items []map[string]any
		switch v := rawContent.(type) {
		case []map[string]any:
			items = v
		case []any:
			items = make([]map[string]any, 0, len(v))
			for _, raw := range v {
				if m, ok := raw.(map[string]any); ok {
					items = append(items, m)
				}
			}
		}
		if len(items) > 0 {
			content := make([]sdkmcp.Content, 0, len(items))
			for _, item := range items {
				if typ, _ := item["type"].(string); typ == "text" {
					if text, _ := item["text"].(string); text != "" {
						content = append(content, &sdkmcp.TextContent{Text: text})
					}
				}
			}
			if len(content) > 0 {
				result.Content = content
				return result
			}
		}
	}

	// Fallback: serialize the full output as a single JSON text entry. This
	// preserves all content (image, audio, structured) for clients that can
	// parse JSON in text content.
	data, err := json.Marshal(output)
	if err != nil {
		r := new(sdkmcp.CallToolResult)
		r.SetError(fmt.Errorf("failed to serialize output: %w", err))
		return r
	}
	result.Content = []sdkmcp.Content{&sdkmcp.TextContent{Text: string(data)}}
	return result
}

// requestIDCounter is the fallback counter used when crypto/rand fails.
var requestIDCounter atomic.Uint64

// newRequestID generates a cryptographically random 16-hex-character request
// identifier. If crypto/rand fails, it falls back to a time+counter based ID
// packed into 8 bytes to guarantee the same 16-hex-character length.
func newRequestID() mcp.RequestID {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		ts := uint32(time.Now().UnixNano())
		seq := uint32(requestIDCounter.Add(1))
		b[0] = byte(ts >> 24)
		b[1] = byte(ts >> 16)
		b[2] = byte(ts >> 8)
		b[3] = byte(ts)
		b[4] = byte(seq >> 24)
		b[5] = byte(seq >> 16)
		b[6] = byte(seq >> 8)
		b[7] = byte(seq)
	}
	return mcp.RequestID(fmt.Sprintf("%x", b))
}
