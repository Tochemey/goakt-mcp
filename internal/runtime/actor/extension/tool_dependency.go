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

func init() {
	gob.Register(&runtime.StdioTransportConfig{})
	gob.Register(&runtime.HTTPTransportConfig{})
}

// ToolDependencyID is the fixed identifier for the tool dependency injected
// into ToolSupervisorActor.
const ToolDependencyID = "tool"

// ToolDependency wraps a Tool for injection into ToolSupervisorActor via
// WithDependencies. The supervisor resolves this dependency in PreStart.
type ToolDependency struct {
	tool runtime.Tool
}

var _ goaktextension.Dependency = (*ToolDependency)(nil)

// NewToolDependency creates a dependency wrapping the given tool.
func NewToolDependency(tool runtime.Tool) *ToolDependency {
	return &ToolDependency{tool: tool}
}

// ID returns the unique identifier for this dependency.
func (x *ToolDependency) ID() string {
	return ToolDependencyID
}

// MarshalBinary encodes the tool for transport or persistence.
func (x *ToolDependency) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(&x.tool); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary decodes the tool from binary form.
func (x *ToolDependency) UnmarshalBinary(data []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(data))
	return dec.Decode(&x.tool)
}

// Tool returns the wrapped tool definition.
func (x *ToolDependency) Tool() runtime.Tool {
	return x.tool
}

// CircuitConfigDependencyID is the fixed identifier for the optional circuit
// config dependency.
const CircuitConfigDependencyID = "circuit_config"

// CircuitConfigDependency optionally overrides circuit breaker parameters.
// Used in tests to avoid long OpenDuration waits.
type CircuitConfigDependency struct {
	cfg runtime.CircuitConfig
}

var _ goaktextension.Dependency = (*CircuitConfigDependency)(nil)

// NewCircuitConfigDependency creates a circuit config dependency.
func NewCircuitConfigDependency(cfg runtime.CircuitConfig) *CircuitConfigDependency {
	return &CircuitConfigDependency{cfg: cfg}
}

// ID returns the unique identifier for this dependency.
func (c *CircuitConfigDependency) ID() string {
	return CircuitConfigDependencyID
}

// Config returns the circuit breaker configuration.
func (c *CircuitConfigDependency) Config() runtime.CircuitConfig {
	return c.cfg
}

// MarshalBinary is a no-op; circuit config is not serialized.
func (c *CircuitConfigDependency) MarshalBinary() ([]byte, error) {
	return nil, nil
}

// UnmarshalBinary is a no-op; circuit config is not deserialized.
func (c *CircuitConfigDependency) UnmarshalBinary(_ []byte) error {
	return nil
}
