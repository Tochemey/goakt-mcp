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

package extension

import (
	"bytes"
	"encoding/gob"

	goaktextension "github.com/tochemey/goakt/v4/extension"

	"github.com/tochemey/goakt-mcp/internal/runtime"
)

// SessionDependencyID is the fixed identifier for the session dependency.
const SessionDependencyID = "session"

// SessionDependency wraps session identity, tool config, and optional executor
// for injection into SessionActor. When Executor is non-nil, the session uses
// it for real MCP execution; otherwise it returns stub results.
type SessionDependency struct {
	tenantID runtime.TenantID
	clientID runtime.ClientID
	toolID   runtime.ToolID
	tool     runtime.Tool
	executor runtime.ToolExecutor
}

var _ goaktextension.Dependency = (*SessionDependency)(nil)

// sessionDependencyPayload is used for gob encoding.
type sessionDependencyPayload struct {
	TenantID string
	ClientID string
	ToolID   string
	Tool     runtime.Tool
}

// NewSessionDependency creates a dependency for the given session identity and
// tool. Pass nil for executor to use stub execution.
func NewSessionDependency(tenantID runtime.TenantID, clientID runtime.ClientID, toolID runtime.ToolID, tool runtime.Tool, executor runtime.ToolExecutor) *SessionDependency {
	return &SessionDependency{
		tenantID: tenantID,
		clientID: clientID,
		toolID:   toolID,
		tool:     tool,
		executor: executor,
	}
}

// ID returns the unique identifier for this dependency.
func (s *SessionDependency) ID() string {
	return SessionDependencyID
}

// MarshalBinary encodes the session dependency for transport or persistence.
func (s *SessionDependency) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	payload := sessionDependencyPayload{
		TenantID: string(s.tenantID),
		ClientID: string(s.clientID),
		ToolID:   string(s.toolID),
		Tool:     s.tool,
	}
	if err := enc.Encode(&payload); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary decodes the session dependency from binary form.
func (s *SessionDependency) UnmarshalBinary(data []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(data))
	var payload sessionDependencyPayload
	if err := dec.Decode(&payload); err != nil {
		return err
	}
	s.tenantID = runtime.TenantID(payload.TenantID)
	s.clientID = runtime.ClientID(payload.ClientID)
	s.toolID = runtime.ToolID(payload.ToolID)
	s.tool = payload.Tool
	return nil
}

// TenantID returns the tenant identifier.
func (s *SessionDependency) TenantID() runtime.TenantID { return s.tenantID }

// ClientID returns the client identifier.
func (s *SessionDependency) ClientID() runtime.ClientID { return s.clientID }

// ToolID returns the tool identifier.
func (s *SessionDependency) ToolID() runtime.ToolID { return s.toolID }

// Tool returns the tool definition (for idle timeout and transport config).
func (s *SessionDependency) Tool() runtime.Tool { return s.tool }

// Executor returns the optional tool executor for real MCP execution.
// Nil means stub mode.
func (s *SessionDependency) Executor() runtime.ToolExecutor { return s.executor }
