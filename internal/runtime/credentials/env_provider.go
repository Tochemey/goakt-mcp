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

package credentials

import (
	"context"
	"os"
	"strings"

	"github.com/tochemey/goakt-mcp/internal/runtime"
)

const envProviderID = "env"

// EnvProvider resolves credentials from environment variables.
//
// Looks for MCP_CRED_{TOOL_ID}_{KEY} (e.g., MCP_CRED_my-tool_API_KEY).
// Keys are uppercased and sanitized. Tenant is not used for env lookup;
// use a tenant-specific prefix in the key name if needed.
type EnvProvider struct{}

// NewEnvProvider creates an EnvProvider.
func NewEnvProvider() *EnvProvider {
	return &EnvProvider{}
}

// ID returns the provider identifier.
func (e *EnvProvider) ID() string {
	return envProviderID
}

// Resolve returns credentials from environment variables.
func (e *EnvProvider) Resolve(ctx context.Context, tenantID runtime.TenantID, toolID runtime.ToolID) (map[string]string, error) {
	prefix := "MCP_CRED_" + strings.ToUpper(strings.ReplaceAll(string(toolID), "-", "_")) + "_"
	creds := make(map[string]string)
	for _, env := range os.Environ() {
		if idx := strings.Index(env, "="); idx > 0 && strings.HasPrefix(env, prefix) {
			key := env[len(prefix):idx]
			key = strings.ToLower(strings.ReplaceAll(key, "_", "-"))
			creds[key] = env[idx+1:]
		}
	}
	if len(creds) == 0 {
		return nil, nil
	}
	return creds, nil
}
