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

package extension_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tochemey/goakt-mcp/mcp"

	"github.com/tochemey/goakt-mcp/internal/runtime/actor/extension"
)

// ---- ExecutorFactoryExtension ----

func TestExecutorFactoryExtension(t *testing.T) {
	t.Run("ID returns fixed identifier", func(t *testing.T) {
		ext := extension.NewExecutorFactoryExtension(nil)
		assert.Equal(t, extension.ExecutorFactoryExtensionID, ext.ID())
	})

	t.Run("Factory returns nil when nil factory provided", func(t *testing.T) {
		ext := extension.NewExecutorFactoryExtension(nil)
		assert.Nil(t, ext.Factory())
	})
}

// ---- ToolConfigExtension ----

func stdioTool(id mcp.ToolID) mcp.Tool {
	return mcp.Tool{
		ID:        id,
		Transport: mcp.TransportStdio,
		Stdio:     &mcp.StdioTransportConfig{Command: "npx"},
		State:     mcp.ToolStateEnabled,
	}
}

func TestToolConfigExtension(t *testing.T) {
	t.Run("ID returns ToolConfigExtensionID", func(t *testing.T) {
		ext := extension.NewToolConfigExtension()
		assert.Equal(t, extension.ToolConfigExtensionID, ext.ID())
	})

	t.Run("Get returns false for unknown tool", func(t *testing.T) {
		ext := extension.NewToolConfigExtension()
		_, found := ext.Get("missing")
		assert.False(t, found)
	})

	t.Run("Register and Get round-trip", func(t *testing.T) {
		ext := extension.NewToolConfigExtension()
		tool := stdioTool("my-tool")
		ext.Register(tool)
		got, found := ext.Get(tool.ID)
		require.True(t, found)
		assert.Equal(t, tool.ID, got.ID)
		assert.Equal(t, tool.Transport, got.Transport)
	})

	t.Run("Register replaces existing tool", func(t *testing.T) {
		ext := extension.NewToolConfigExtension()
		original := stdioTool("replace-tool")
		updated := original
		updated.State = mcp.ToolStateDisabled
		ext.Register(original)
		ext.Register(updated)
		got, found := ext.Get(original.ID)
		require.True(t, found)
		assert.Equal(t, mcp.ToolStateDisabled, got.State)
	})

	t.Run("Remove deletes tool", func(t *testing.T) {
		ext := extension.NewToolConfigExtension()
		tool := stdioTool("remove-tool")
		ext.Register(tool)
		ext.Remove(tool.ID)
		_, found := ext.Get(tool.ID)
		assert.False(t, found)
	})

	t.Run("Remove is no-op for unknown tool", func(t *testing.T) {
		ext := extension.NewToolConfigExtension()
		assert.NotPanics(t, func() { ext.Remove("nonexistent") })
	})
}

// ---- CircuitConfigExtension ----

func TestCircuitConfigExtension(t *testing.T) {
	cfg := mcp.CircuitConfig{
		FailureThreshold:    3,
		OpenDuration:        5 * time.Second,
		HalfOpenMaxRequests: 1,
	}

	t.Run("ID returns CircuitConfigExtensionID", func(t *testing.T) {
		ext := extension.NewCircuitConfigExtension(cfg)
		assert.Equal(t, extension.CircuitConfigExtensionID, ext.ID())
	})

	t.Run("Config returns the wrapped config", func(t *testing.T) {
		ext := extension.NewCircuitConfigExtension(cfg)
		assert.Equal(t, cfg, ext.Config())
	})
}

// ---- SessionDependency ----

func TestSessionDependency(t *testing.T) {
	tenantID := mcp.TenantID("tenant-1")
	clientID := mcp.ClientID("client-1")
	toolID := mcp.ToolID("tool-1")
	tool := stdioTool(toolID)

	dep := extension.NewSessionDependency(tenantID, clientID, toolID, tool, nil)

	t.Run("ID returns SessionDependencyID", func(t *testing.T) {
		assert.Equal(t, extension.SessionDependencyID, dep.ID())
	})

	t.Run("accessors return correct values", func(t *testing.T) {
		assert.Equal(t, tenantID, dep.TenantID())
		assert.Equal(t, clientID, dep.ClientID())
		assert.Equal(t, toolID, dep.ToolID())
		assert.Equal(t, tool, dep.Tool())
		assert.Nil(t, dep.Executor())
	})

	t.Run("MarshalBinary and UnmarshalBinary round-trip", func(t *testing.T) {
		data, err := dep.MarshalBinary()
		require.NoError(t, err)
		require.NotEmpty(t, data)

		restored := &extension.SessionDependency{}
		require.NoError(t, restored.UnmarshalBinary(data))
		assert.Equal(t, tenantID, restored.TenantID())
		assert.Equal(t, clientID, restored.ClientID())
		assert.Equal(t, toolID, restored.ToolID())
		assert.Equal(t, tool.ID, restored.Tool().ID)
	})

	t.Run("UnmarshalBinary with invalid data returns error", func(t *testing.T) {
		bad := &extension.SessionDependency{}
		err := bad.UnmarshalBinary([]byte("not-valid-gob"))
		require.Error(t, err)
	})
}
