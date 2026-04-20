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

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// TokenExchanger exchanges a subject token for a new issued token per RFC 8693
// Token Exchange (https://www.rfc-editor.org/rfc/rfc8693). Implementations
// typically mint downstream access tokens on behalf of an authenticated user
// when the gateway calls a backend MCP server or tool that requires its own
// authorization context (MCP Enterprise Managed Authorization, SEP-990).
//
// Implementations must be safe for concurrent use.
type TokenExchanger interface {
	// ExchangeToken performs a single exchange. The returned result carries the
	// issued token, its type, and expiry. Implementations must return an error
	// rather than a zero-valued result when the exchange fails.
	ExchangeToken(ctx context.Context, req *TokenExchangeRequest) (*TokenExchangeResult, error)
}

// TokenExchangeRequest describes one RFC 8693 token exchange attempt.
//
// The zero value is not a valid request: SubjectToken, SubjectTokenType,
// Audience, and Resource are all mandatory. RequestedTokenType defaults to
// oauthex.TokenTypeIDJAG when empty, matching SEP-990 guidance.
type TokenExchangeRequest struct {
	// SubjectToken is the security token that represents the identity of the
	// party on behalf of whom the request is being made. For SEP-990 flows
	// this is typically an OpenID Connect ID Token.
	SubjectToken string

	// SubjectTokenType is the type URN for SubjectToken. Common values:
	// oauthex.TokenTypeIDToken, oauthex.TokenTypeSAML2.
	SubjectTokenType string

	// Audience is the logical name of the target service where the issued
	// token will be used. For SEP-990, set this to the issuer URL of the
	// downstream MCP server's authorization server.
	Audience string

	// Resource is the RFC 9728 resource identifier of the target resource.
	Resource string

	// Scope lists the OAuth scopes requested for the issued token. Optional.
	Scope []string

	// RequestedTokenType is the URN describing the desired issued-token type.
	// When empty, the exchanger uses oauthex.TokenTypeIDJAG.
	RequestedTokenType string
}

// TokenExchangeResult is the outcome of a successful TokenExchanger call.
type TokenExchangeResult struct {
	// AccessToken is the issued token string. For SEP-990 flows, this is the
	// ID-JAG JWT even though the underlying transport field is named
	// "access_token" in the RFC 8693 response.
	AccessToken string

	// TokenType is the token_type reported by the IdP (often "N_A" for SEP-990).
	TokenType string

	// IssuedTokenType is the URN describing the type of AccessToken. Callers
	// may use this to route the token (e.g. ID-JAG vs access token).
	IssuedTokenType string

	// Expiry is the absolute time at which the issued token expires. A zero
	// value means the IdP did not supply an expiry.
	Expiry time.Time

	// Scope carries the scope granted by the IdP when present.
	Scope []string
}

// OAuthTokenExchangerConfig configures an OAuth-backed TokenExchanger.
//
// TokenEndpoint and ClientCredentials are required; all other fields have
// sensible zero-value defaults.
type OAuthTokenExchangerConfig struct {
	// TokenEndpoint is the IdP token endpoint used for the exchange. The URL
	// must use the https scheme (the oauthex helper validates this).
	TokenEndpoint string

	// ClientCredentials identifies this gateway to the IdP.
	ClientCredentials *oauthex.ClientCredentials

	// HTTPClient is an optional HTTP client for the exchange request. When
	// nil, http.DefaultClient is used.
	HTTPClient *http.Client
}

// ErrTokenExchangerConfigRequired is returned by NewOAuthTokenExchanger when
// the supplied configuration is nil or missing required fields.
var ErrTokenExchangerConfigRequired = errors.New("token exchanger configuration is required")

// oauthTokenExchanger is the concrete OAuth-backed TokenExchanger. It wraps
// oauthex.ExchangeToken with the configured endpoint, client credentials, and
// HTTP client so call sites can issue exchanges without knowing those details.
type oauthTokenExchanger struct {
	tokenEndpoint string
	clientCreds   *oauthex.ClientCredentials
	httpClient    *http.Client
}

// Compile-time proof that oauthTokenExchanger implements TokenExchanger.
var _ TokenExchanger = (*oauthTokenExchanger)(nil)

// NewOAuthTokenExchanger constructs a TokenExchanger backed by the MCP SDK's
// oauthex.ExchangeToken helper. It performs no network I/O until
// TokenExchanger.ExchangeToken is called.
//
// Returns ErrTokenExchangerConfigRequired when cfg, cfg.TokenEndpoint, or
// cfg.ClientCredentials is missing.
func NewOAuthTokenExchanger(cfg *OAuthTokenExchangerConfig) (TokenExchanger, error) {
	if cfg == nil || cfg.TokenEndpoint == "" || cfg.ClientCredentials == nil {
		return nil, ErrTokenExchangerConfigRequired
	}

	return &oauthTokenExchanger{
		tokenEndpoint: cfg.TokenEndpoint,
		clientCreds:   cfg.ClientCredentials,
		httpClient:    cfg.HTTPClient,
	}, nil
}

// ExchangeToken performs the RFC 8693 exchange. It validates the request,
// applies the SEP-990 default RequestedTokenType when empty, and delegates to
// oauthex.ExchangeToken. The returned TokenExchangeResult reflects the IdP
// response with best-effort parsing of the scope and issued_token_type extras.
func (x *oauthTokenExchanger) ExchangeToken(ctx context.Context, req *TokenExchangeRequest) (*TokenExchangeResult, error) {
	if req == nil {
		return nil, WrapRuntimeError(ErrCodeInvalidRequest, "token exchange request is required", nil)
	}

	requestedType := req.RequestedTokenType
	if requestedType == "" {
		requestedType = oauthex.TokenTypeIDJAG
	}

	exchangeReq := &oauthex.TokenExchangeRequest{
		RequestedTokenType: requestedType,
		Audience:           req.Audience,
		Resource:           req.Resource,
		Scope:              req.Scope,
		SubjectToken:       req.SubjectToken,
		SubjectTokenType:   req.SubjectTokenType,
	}

	token, err := oauthex.ExchangeToken(ctx, x.tokenEndpoint, exchangeReq, x.clientCreds, x.httpClient)
	if err != nil {
		return nil, WrapRuntimeError(ErrCodeInternal, "token exchange failed", err)
	}

	issuedTokenType, _ := token.Extra("issued_token_type").(string)

	return &TokenExchangeResult{
		AccessToken:     token.AccessToken,
		TokenType:       token.TokenType,
		IssuedTokenType: issuedTokenType,
		Expiry:          token.Expiry,
		Scope:           scopeFromToken(token.Extra("scope")),
	}, nil
}

// scopeFromToken parses the optional scope field from an RFC 8693 response.
// The SDK returns extras as an any, so we tolerate both string and []string
// shapes and gracefully return nil when absent or malformed.
func scopeFromToken(raw any) []string {
	switch value := raw.(type) {
	case string:
		if value == "" {
			return nil
		}

		scopes := splitSpace(value)
		if len(scopes) == 0 {
			return nil
		}
		return scopes
	case []string:
		if len(value) == 0 {
			return nil
		}
		return value
	default:
		return nil
	}
}

// splitSpace splits a space-delimited scope string into a slice, skipping
// empty segments. It avoids the strings.Fields allocation pattern for the
// common single-scope path.
func splitSpace(value string) []string {
	var scopes []string
	start := 0
	for i := 0; i < len(value); i++ {
		if value[i] != ' ' {
			continue
		}

		if i > start {
			scopes = append(scopes, value[start:i])
		}
		start = i + 1
	}

	if start < len(value) {
		scopes = append(scopes, value[start:])
	}
	return scopes
}
