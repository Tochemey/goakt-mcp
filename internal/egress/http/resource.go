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
	gohttp "net/http"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tochemey/goakt-mcp/internal/egress/schemaconv"
	"github.com/tochemey/goakt-mcp/mcp"
)

// FetchResources connects to the HTTP backend, calls resources/list and
// resources/templates/list, and returns the discovered resource metadata.
// The connection is closed before returning.
func FetchResources(ctx context.Context, cfg *mcp.HTTPTransportConfig, fallbackClient *gohttp.Client, startupTimeout time.Duration) ([]mcp.ResourceSchema, []mcp.ResourceTemplateSchema, error) {
	if cfg == nil || cfg.URL == "" {
		return nil, nil, mcp.NewRuntimeError(mcp.ErrCodeInvalidRequest, "http config required")
	}

	httpClient, err := buildHTTPClient(cfg, fallbackClient)
	if err != nil {
		return nil, nil, err
	}
	base := httpClient.Transport
	if base == nil {
		base = gohttp.DefaultTransport
	}
	httpClient.Transport = wrapTransport(base)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "goakt-mcp-resource", Version: mcp.Version()}, nil)
	transport := &sdkmcp.StreamableClientTransport{
		Endpoint:   cfg.URL,
		HTTPClient: httpClient,
	}

	fetchCtx := ctx
	if startupTimeout > 0 {
		var cancel context.CancelFunc
		fetchCtx, cancel = context.WithTimeout(fetchCtx, startupTimeout)
		defer cancel()
	}

	sess, err := client.Connect(fetchCtx, transport, nil)
	if err != nil {
		return nil, nil, mcp.WrapRuntimeError(mcp.ErrCodeTransportFailure, "http resource connect failed", err)
	}
	defer sess.Close()

	// Fetch resources. If the server does not support resources, ListResources
	// returns an error; treat that as empty resources rather than a fatal error.
	var resources []mcp.ResourceSchema
	resResult, err := sess.ListResources(fetchCtx, nil)
	if err == nil && resResult != nil {
		resources = schemaconv.SDKResourcesToSchemas(resResult.Resources)
	}

	// Fetch resource templates.
	var templates []mcp.ResourceTemplateSchema
	tmplResult, err := sess.ListResourceTemplates(fetchCtx, nil)
	if err == nil && tmplResult != nil {
		templates = schemaconv.SDKResourceTemplatesToSchemas(tmplResult.ResourceTemplates)
	}

	return resources, templates, nil
}
