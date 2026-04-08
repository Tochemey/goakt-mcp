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

package pkg

import (
	"errors"

	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/tochemey/goakt-mcp/mcp"
)

// ResolveAuthDefaults validates [mcp.EnterpriseAuthConfig] when present on cfg
// and fills in defaults. When EnterpriseAuth is set and IdentityResolver is nil,
// a [mcp.NewTokenIdentityResolver] is installed automatically using the
// configured IdentityMapper (or [mcp.DefaultIdentityMapper] when nil).
//
// Returns an error when EnterpriseAuth is set but required fields are missing.
func ResolveAuthDefaults(cfg *mcp.IngressConfig) error {
	ea := cfg.EnterpriseAuth
	if ea == nil {
		return nil
	}

	if ea.TokenVerifier == nil {
		return errors.New("enterprise auth: TokenVerifier must not be nil")
	}

	if ea.ResourceMetadata == nil {
		return errors.New("enterprise auth: ResourceMetadata must not be nil")
	}

	if ea.ResourceMetadata.Resource == "" {
		return errors.New("enterprise auth: ResourceMetadata.Resource must not be empty")
	}

	// When enterprise auth is active and no explicit IdentityResolver is
	// provided, install one that reads identity from the validated token.
	if cfg.IdentityResolver == nil {
		cfg.IdentityResolver = mcp.NewTokenIdentityResolver(ea.IdentityMapper)
	}

	return nil
}

// ResourceMetadataURL returns the well-known URL for the Protected Resource
// Metadata document derived from the resource identifier. When metadata is nil
// or the resource field is empty, an empty string is returned.
//
// Per RFC 9728 Section 3.1, the metadata URL is constructed by appending
// /.well-known/oauth-protected-resource to the resource origin.
func ResourceMetadataURL(metadata *oauthex.ProtectedResourceMetadata) string {
	if metadata == nil || metadata.Resource == "" {
		return ""
	}
	resource := metadata.Resource
	// Strip trailing slash before appending well-known path.
	if resource[len(resource)-1] == '/' {
		resource = resource[:len(resource)-1]
	}
	return resource + "/.well-known/oauth-protected-resource"
}
