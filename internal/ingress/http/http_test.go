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

package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goaktlog "github.com/tochemey/goakt/v4/log"

	"github.com/tochemey/goakt-mcp/internal/ingress/dto"
	ingresshttp "github.com/tochemey/goakt-mcp/internal/ingress/http"
	"github.com/tochemey/goakt-mcp/internal/runtime"
	"github.com/tochemey/goakt-mcp/internal/runtime/actor"
	"github.com/tochemey/goakt-mcp/internal/runtime/config"
)

func TestInvokeHandler(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	gw, err := actor.New(cfg, goaktlog.DiscardLogger)
	require.NoError(t, err)
	require.NoError(t, gw.Start(ctx))
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = gw.Stop(stopCtx)
	})

	time.Sleep(200 * time.Millisecond) // allow actors to start

	server := ingresshttp.NewServer(cfg.HTTP, gw.System(), goaktlog.DiscardLogger)

	t.Run("returns 404 for unknown tool", func(t *testing.T) {
		body := dto.InvokeRequest{
			TenantID:  "acme",
			ClientID:  "client-1",
			RequestID: "req-1",
			TraceID:   "trace-1",
			Method:    "tools/call",
			Params:    map[string]any{"name": "test", "arguments": map[string]any{}},
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/v1/tools/unknown-tool/invoke", bytes.NewReader(b))
		req.SetPathValue("tool", "unknown-tool")

		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		var resp dto.InvokeResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
		assert.Equal(t, "failure", resp.Status)
		assert.NotNil(t, resp.Error)
		assert.Equal(t, string(runtime.ErrCodeToolNotFound), resp.Error.Code)
	})

	t.Run("returns 400 for invalid request body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/tools/filesystem/invoke", bytes.NewReader([]byte("invalid")))
		req.SetPathValue("tool", "filesystem")

		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("returns 400 when tenant_id missing", func(t *testing.T) {
		body := dto.InvokeRequest{
			ClientID:  "client-1",
			RequestID: "req-1",
			TraceID:   "trace-1",
			Method:    "tools/call",
			Params:    map[string]any{"name": "test"},
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/v1/tools/filesystem/invoke", bytes.NewReader(b))
		req.SetPathValue("tool", "filesystem")

		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestListToolsHandler(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	gw, err := actor.New(cfg, goaktlog.DiscardLogger)
	require.NoError(t, err)
	require.NoError(t, gw.Start(ctx))
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = gw.Stop(stopCtx)
	})

	time.Sleep(200 * time.Millisecond)

	server := ingresshttp.NewServer(cfg.HTTP, gw.System(), goaktlog.DiscardLogger)

	t.Run("returns tool list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/tools", nil)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp dto.ToolsListResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
		assert.NotNil(t, resp.Tools)
		assert.GreaterOrEqual(t, len(resp.Tools), 1)
		found := false
		for _, t := range resp.Tools {
			if t.ID == "filesystem" {
				found = true
				break
			}
		}
		assert.True(t, found, "expected filesystem tool in list")
	})
}

func TestServerPatterns(t *testing.T) {
	cfg := config.HTTPConfig{ListenAddress: ""}
	s := ingresshttp.NewServer(cfg, nil, goaktlog.DiscardLogger)
	require.NotNil(t, s)
	require.NotNil(t, s.Handler())
}

func testConfig() config.Config {
	return config.Config{
		HTTP: config.HTTPConfig{ListenAddress: ":0"},
		Runtime: config.RuntimeConfig{
			SessionIdleTimeout: 5 * time.Minute,
			RequestTimeout:     30 * time.Second,
			StartupTimeout:     10 * time.Second,
		},
		Tools: []config.ToolConfig{
			{
				ID:        runtime.ToolID("filesystem"),
				Transport: runtime.TransportStdio,
				Command:   "echo",
				Args:      []string{"hello"},
			},
		},
	}
}
