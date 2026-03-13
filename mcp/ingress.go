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
	"net/http"
	"time"
)

// IdentityResolver extracts the tenant and client identity from an incoming
// HTTP request. Implementations may read from JWT claims, API key headers,
// mTLS certificates, or any other source appropriate for the deployment.
//
// ResolveIdentity is called once per new MCP session. A non-nil error rejects
// the session; the gateway returns HTTP 400 Bad Request to the client.
// Empty identity values are accepted; no default identity is injected.
type IdentityResolver interface {
	ResolveIdentity(r *http.Request) (tenantID TenantID, clientID ClientID, err error)
}

// IngressConfig configures the MCP HTTP ingress handler returned by
// [Gateway.Handler]. Users mount the resulting [http.Handler] inside their
// own HTTP server or router framework.
type IngressConfig struct {
	// IdentityResolver extracts caller identity from each incoming MCP session
	// initialization request. Required; [Gateway.Handler] returns an error when
	// this field is nil.
	IdentityResolver IdentityResolver

	// SessionIdleTimeout is how long the server waits before closing an idle
	// MCP session. Zero means sessions are never closed due to inactivity.
	SessionIdleTimeout time.Duration

	// Stateless disables per-client session tracking. When true, each HTTP
	// request is treated as an independent session with no Mcp-Session-Id
	// header. Suitable for deployments behind a load balancer that does not
	// support sticky sessions. Default: false (stateful sessions).
	Stateless bool
}
