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

package runtime

import "github.com/tochemey/goakt-mcp/mcp"

// Session command and response types for SessionActor and ToolSupervisorActor.
//
// These messages define the contract for session lifecycle, invocation routing,
// and passivation. Session creation is owned by the supervisor; sessions are
// children of the tool supervisor.

// GetOrCreateSession is a request to resolve or create a session for the given
// tenant, client, and tool combination.
//
// The supervisor returns the session PID when found or newly spawned.
// Credentials are passed when creating a new session for executor setup (e.g. HTTP auth).
// Must be used with Ask. Response is GetOrCreateSessionResult.
type GetOrCreateSession struct {
	TenantID    mcp.TenantID
	ClientID    mcp.ClientID
	ToolID      mcp.ToolID
	Credentials map[string]string
}

// GetOrCreateSessionResult is the response to GetOrCreateSession.
//
// When Found is true, Session holds the *actor.PID of the SessionActor.
type GetOrCreateSessionResult struct {
	Session any
	Found   bool
	Err     error
}

// SessionInvoke is a request to execute an invocation through the session.
//
// The session serializes invocations through its mailbox and tracks in-flight
// work. Must be used with Ask. Response is SessionInvokeResult.
type SessionInvoke struct {
	Invocation *mcp.Invocation
}

// SessionInvokeResult is the response to SessionInvoke.
type SessionInvokeResult struct {
	Result *mcp.ExecutionResult
	Err    error
}
