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

// Package credentials provides credential resolution for tool invocations.
//
// The CredentialBrokerActor resolves secrets just-in-time through configured
// providers (env, file, etc.). Credentials are not persisted in actor state
// longer than necessary.
package credentials

import (
	"context"
	"time"

	"github.com/tochemey/goakt-mcp/mcp"
)

// DefaultCredentialTTL is the default cache TTL for resolved credentials.
const DefaultCredentialTTL = 5 * time.Minute

// Provider resolves credentials for a tenant and tool.
//
// Implementations should avoid long-lived secret storage. The broker may cache
// results with bounded TTL; providers are not required to implement caching.
type Provider interface {
	// ID returns the provider identifier (e.g., "env", "file").
	ID() string

	// Resolve returns credentials for the given tenant and tool.
	// Returns (nil, nil) when the provider has no credentials for this combination.
	// Returns (nil, err) when resolution fails.
	Resolve(ctx context.Context, tenantID mcp.TenantID, toolID mcp.ToolID) (map[string]string, error)
}

// ResolveRequest is a request to resolve credentials for an invocation.
type ResolveRequest struct {
	TenantID mcp.TenantID
	ToolID   mcp.ToolID
}

// ResolveResult is the response to ResolveRequest.
type ResolveResult struct {
	Credentials map[string]string
	Err         error
}

// Resolved returns true when credentials were successfully resolved.
func (r *ResolveResult) Resolved() bool {
	return r.Err == nil && len(r.Credentials) > 0
}
