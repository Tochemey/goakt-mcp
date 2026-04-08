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

import "context"

// PolicyEvaluator is an extension point for custom authorization logic.
//
// PolicyEvaluator is invoked after all built-in authorization and quota checks
// pass. It provides a hook for use cases such as OPA integration, RBAC, or
// any domain-specific access control rules.
//
// Returning a non-nil *RuntimeError causes the invocation to be denied. The
// error is propagated as-is, so the evaluator controls both the error code and
// message. Returning nil allows the invocation to proceed.
//
// Implementations must be safe for concurrent use from multiple goroutines.
type PolicyEvaluator interface {
	Evaluate(ctx context.Context, input PolicyInput) *RuntimeError
}

// PolicyInput carries the context available to a custom PolicyEvaluator.
//
// It exposes the resolved tenant and tool identities plus the runtime usage
// counters that were used in the built-in quota checks, allowing custom
// evaluators to implement quota-aware or usage-sensitive policies.
type PolicyInput struct {
	// TenantID is the resolved tenant identity for this invocation.
	TenantID TenantID
	// ToolID is the identifier of the tool being invoked.
	ToolID ToolID
	// ActiveSessionCount is the number of live sessions for this tenant across
	// all tools at the time the invocation is being evaluated.
	ActiveSessionCount int
	// RequestsInCurrentMinute is the number of requests from this tenant in
	// the current minute window at the time the invocation is being evaluated.
	RequestsInCurrentMinute int
	// Scopes holds the OAuth scopes granted by the validated bearer token
	// when the enterprise-managed authorization extension is active. Empty
	// when enterprise auth is not configured or the token carries no scopes.
	//
	// Custom [PolicyEvaluator] implementations can use this field to enforce
	// scope-based access control per tool or operation.
	Scopes []string
}
