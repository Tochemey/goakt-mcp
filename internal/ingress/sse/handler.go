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

package sse

import (
	"errors"
	"net/http"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tochemey/goakt-mcp/internal/ingress/shared"
	"github.com/tochemey/goakt-mcp/mcp"
)

// New returns an [http.Handler] that serves MCP SSE sessions (2024-11-05 spec)
// and routes each tool call through gw.
//
// Identity (tenantID + clientID) is resolved once per new MCP session via
// cfg.IdentityResolver. On resolution failure, getServer returns nil which
// causes the SDK to reply with HTTP 400 Bad Request. The Stateless and
// SessionIdleTimeout fields of cfg are not used by the SSE transport; session
// lifetime is controlled by the client's GET connection.
//
// New validates that cfg.IdentityResolver is non-nil and returns an error
// if it is not.
func New(gw shared.Invoker, cfg mcp.IngressConfig) (http.Handler, error) {
	if cfg.IdentityResolver == nil {
		return nil, errors.New("ingress/sse: IdentityResolver must not be nil")
	}

	getServer := shared.BuildGetServer(gw, cfg.IdentityResolver)
	return sdkmcp.NewSSEHandler(getServer, nil), nil
}
