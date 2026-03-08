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
	"fmt"

	"github.com/tochemey/goakt-mcp/internal/runtime"
)

// Validate checks cfg for critical errors. Returns nil when valid.
func Validate(cfg *Config) error {
	if err := validateHTTP(&cfg.HTTP); err != nil {
		return err
	}
	if err := validateCluster(&cfg.Cluster); err != nil {
		return err
	}
	if err := validateTenants(cfg.Tenants); err != nil {
		return err
	}
	if err := validateTools(cfg.Tools); err != nil {
		return err
	}
	return nil
}

// validateHTTP checks that ingress TLS configuration is complete when enabled.
func validateHTTP(h *HTTPConfig) error {
	if h.TLS != nil {
		if h.TLS.CertFile == "" || h.TLS.KeyFile == "" {
			return fmt.Errorf("http.tls: cert_file and key_file are required when tls is enabled")
		}
	}
	return nil
}

// validateCluster ensures a discovery method is specified when clustering is on.
func validateCluster(c *ClusterConfig) error {
	if c.Enabled && c.Discovery == "" {
		return fmt.Errorf("cluster.discovery is required when cluster is enabled")
	}
	return nil
}

// validateTenants verifies that every tenant has a non-empty, unique ID.
func validateTenants(tenants []TenantConfig) error {
	seen := make(map[runtime.TenantID]bool, len(tenants))
	for i, t := range tenants {
		if t.ID == "" {
			return fmt.Errorf("tenants[%d]: id is required", i)
		}
		if seen[t.ID] {
			return fmt.Errorf("tenants: duplicate tenant id %q", t.ID)
		}
		seen[t.ID] = true
	}
	return nil
}

// validateTools verifies that every tool has a non-empty, unique ID and passes
// transport-specific validation.
func validateTools(tools []ToolConfig) error {
	seen := make(map[runtime.ToolID]bool, len(tools))
	for i, t := range tools {
		if t.ID == "" {
			return fmt.Errorf("tools[%d]: id is required", i)
		}
		if seen[t.ID] {
			return fmt.Errorf("tools: duplicate tool id %q", t.ID)
		}
		seen[t.ID] = true
		if err := validateTool(&t); err != nil {
			return fmt.Errorf("tools[%d] %q: %w", i, t.ID, err)
		}
	}
	return nil
}

// validateTool checks transport-specific required fields and egress TLS
// pairing for a single tool configuration.
func validateTool(t *ToolConfig) error {
	switch t.Transport {
	case runtime.TransportStdio:
		if t.Command == "" {
			return fmt.Errorf("command is required for stdio transport")
		}
	case runtime.TransportHTTP:
		if t.URL == "" {
			return fmt.Errorf("url is required for http transport")
		}
	}
	if t.HTTPTLS != nil {
		if err := validateEgressTLS(t.HTTPTLS); err != nil {
			return fmt.Errorf("http_tls: %w", err)
		}
	}
	return nil
}

// validateEgressTLS ensures that client_cert_file and client_key_file are
// either both set or both empty for mutual TLS.
func validateEgressTLS(tls *runtime.EgressTLSConfig) error {
	hasCert := tls.ClientCertFile != ""
	hasKey := tls.ClientKeyFile != ""
	if hasCert != hasKey {
		return fmt.Errorf("client_cert_file and client_key_file must both be set or both be empty")
	}
	return nil
}
