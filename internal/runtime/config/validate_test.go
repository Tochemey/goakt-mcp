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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tochemey/goakt-mcp/internal/runtime"
)

func TestValidate(t *testing.T) {
	validStdioTool := ToolConfig{
		ID:        "fs",
		Transport: runtime.TransportStdio,
		Command:   "echo",
	}
	validHTTPTool := ToolConfig{
		ID:        "remote",
		Transport: runtime.TransportHTTP,
		URL:       "https://example.com",
	}

	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "valid minimal config",
			cfg: Config{
				HTTP:    HTTPConfig{ListenAddress: ":8080"},
				Tools:   []ToolConfig{validStdioTool},
				Tenants: []TenantConfig{},
			},
		},
		{
			name: "valid config with tenants and tools",
			cfg: Config{
				HTTP:    HTTPConfig{ListenAddress: ":8080"},
				Tenants: []TenantConfig{{ID: "t1"}},
				Tools:   []ToolConfig{validStdioTool, validHTTPTool},
			},
		},
		{
			name: "ingress TLS missing cert_file",
			cfg: Config{
				HTTP: HTTPConfig{
					ListenAddress: ":8443",
					TLS:           &IngressTLSConfig{KeyFile: "/key.pem"},
				},
			},
			wantErr: "http.tls: cert_file and key_file are required",
		},
		{
			name: "ingress TLS missing key_file",
			cfg: Config{
				HTTP: HTTPConfig{
					ListenAddress: ":8443",
					TLS:           &IngressTLSConfig{CertFile: "/cert.pem"},
				},
			},
			wantErr: "http.tls: cert_file and key_file are required",
		},
		{
			name: "cluster enabled without discovery",
			cfg: Config{
				Cluster: ClusterConfig{Enabled: true},
			},
			wantErr: "cluster.discovery is required",
		},
		{
			name: "tenant with empty id",
			cfg: Config{
				Tenants: []TenantConfig{{ID: ""}},
			},
			wantErr: "tenants[0]: id is required",
		},
		{
			name: "duplicate tenant ids",
			cfg: Config{
				Tenants: []TenantConfig{{ID: "dup"}, {ID: "dup"}},
			},
			wantErr: "duplicate tenant id",
		},
		{
			name: "tool with empty id",
			cfg: Config{
				Tools: []ToolConfig{{Transport: runtime.TransportStdio, Command: "echo"}},
			},
			wantErr: "tools[0]: id is required",
		},
		{
			name: "duplicate tool ids",
			cfg: Config{
				Tools: []ToolConfig{
					{ID: "dup", Transport: runtime.TransportStdio, Command: "echo"},
					{ID: "dup", Transport: runtime.TransportStdio, Command: "cat"},
				},
			},
			wantErr: "duplicate tool id",
		},
		{
			name: "stdio tool missing command",
			cfg: Config{
				Tools: []ToolConfig{{ID: "bad", Transport: runtime.TransportStdio}},
			},
			wantErr: "command is required for stdio",
		},
		{
			name: "http tool missing url",
			cfg: Config{
				Tools: []ToolConfig{{ID: "bad", Transport: runtime.TransportHTTP}},
			},
			wantErr: "url is required for http",
		},
		{
			name: "egress TLS cert without key",
			cfg: Config{
				Tools: []ToolConfig{{
					ID:        "bad",
					Transport: runtime.TransportHTTP,
					URL:       "https://example.com",
					HTTPTLS:   &runtime.EgressTLSConfig{ClientCertFile: "/cert.pem"},
				}},
			},
			wantErr: "client_cert_file and client_key_file must both be set",
		},
		{
			name: "egress TLS key without cert",
			cfg: Config{
				Tools: []ToolConfig{{
					ID:        "bad",
					Transport: runtime.TransportHTTP,
					URL:       "https://example.com",
					HTTPTLS:   &runtime.EgressTLSConfig{ClientKeyFile: "/key.pem"},
				}},
			},
			wantErr: "client_cert_file and client_key_file must both be set",
		},
		{
			name: "egress TLS with both cert and key is valid",
			cfg: Config{
				Tools: []ToolConfig{{
					ID:        "ok",
					Transport: runtime.TransportHTTP,
					URL:       "https://example.com",
					HTTPTLS: &runtime.EgressTLSConfig{
						ClientCertFile: "/cert.pem",
						ClientKeyFile:  "/key.pem",
					},
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(&tt.cfg)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
