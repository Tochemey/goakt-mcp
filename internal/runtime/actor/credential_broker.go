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

package actor

import (
	"maps"
	"time"

	goaktactor "github.com/tochemey/goakt/v4/actor"
	goaktlog "github.com/tochemey/goakt/v4/log"

	"github.com/tochemey/goakt-mcp/mcp"

	"github.com/tochemey/goakt-mcp/internal/runtime/credentials"
)

// credentialCacheEntry holds cached credentials with expiration.
type credentialCacheEntry struct {
	creds     map[string]string
	expiresAt time.Time
}

// credentialBroker is the CredentialBroker.
//
// Resolves credentials just-in-time through configured providers. Uses bounded
// in-memory cache with TTL to avoid repeated provider calls. Does not persist
// secrets longer than the cache TTL.
//
// Spawn: GatewayManager spawns CredentialBroker in spawnCredentialBroker when
// credential providers are configured. Uses ctx.Spawn(ActorNameCredentialBroker,
// newCredentialBroker(providers, ttl)) as a child of GatewayManager. Not spawned
// when buildCredentialProviders returns an empty list.
//
// Relocation: No. CredentialBroker runs on the local node as a child of
// GatewayManager and does not relocate in cluster mode.
//
// State is protected by the actor mailbox (one message at a time); no mutex
// is needed or allowed inside an actor.
type credentialBroker struct {
	providers []credentials.Provider
	cache     map[string]*credentialCacheEntry
	cacheTTL  time.Duration
	logger    goaktlog.Logger
}

var _ goaktactor.Actor = (*credentialBroker)(nil)

// newCredentialBroker creates a CredentialBroker with the given providers.
func newCredentialBroker(providers []credentials.Provider, cacheTTL time.Duration) *credentialBroker {
	if cacheTTL <= 0 {
		cacheTTL = credentials.DefaultCredentialTTL
	}
	return &credentialBroker{
		providers: providers,
		cache:     make(map[string]*credentialCacheEntry),
		cacheTTL:  cacheTTL,
	}
}

// PreStart initializes the CredentialBroker.
func (x *credentialBroker) PreStart(ctx *goaktactor.Context) error {
	x.logger = ctx.Logger()
	x.logger.Infof("actor=%s started", mcp.ActorNameCredentialBroker)
	return nil
}

// Receive handles messages delivered to CredentialBroker.
func (x *credentialBroker) Receive(ctx *goaktactor.ReceiveContext) {
	switch msg := ctx.Message().(type) {
	case *goaktactor.PostStart:
		x.logger.Debugf("actor=%s post-start", mcp.ActorNameCredentialBroker)
	case *credentials.ResolveRequest:
		x.handleResolve(ctx, msg)
	default:
		ctx.Unhandled()
	}
}

// PostStop clears the credential cache and releases resources.
// GoAkt guarantees PostStop runs after all message processing has completed,
// so no synchronization is needed.
func (x *credentialBroker) PostStop(ctx *goaktactor.Context) error {
	x.cache = make(map[string]*credentialCacheEntry)
	x.logger.Infof("actor=%s stopped", mcp.ActorNameCredentialBroker)
	return nil
}

// handleResolve resolves credentials for the requested tenant and tool.
// It checks the in-memory cache first; on miss or expiry it iterates through
// configured providers in preference order and caches the first successful result.
// Returns a defensive copy of the credential map so callers cannot mutate the cache.
func (x *credentialBroker) handleResolve(ctx *goaktactor.ReceiveContext, msg *credentials.ResolveRequest) {
	cacheKey := string(msg.TenantID) + ":" + string(msg.ToolID)

	if entry, ok := x.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		credsCopy := make(map[string]string, len(entry.creds))
		maps.Copy(credsCopy, entry.creds)
		ctx.Response(&credentials.ResolveResult{Credentials: credsCopy})
		return
	}

	goCtx := ctx.Context()
	var resolved map[string]string
	for _, provider := range x.providers {
		creds, err := provider.Resolve(goCtx, msg.TenantID, msg.ToolID)
		if err != nil {
			x.logger.Warnf("actor=%s provider=%s resolve failed: %v", mcp.ActorNameCredentialBroker, provider.ID(), err)
			continue
		}

		if len(creds) > 0 {
			resolved = creds
			break
		}
	}

	if len(resolved) == 0 {
		ctx.Response(&credentials.ResolveResult{
			Err: mcp.NewRuntimeError(mcp.ErrCodeCredentialUnavailable, "no credentials found for tenant and tool"),
		})
		return
	}

	x.cache[cacheKey] = &credentialCacheEntry{
		creds:     resolved,
		expiresAt: time.Now().Add(x.cacheTTL),
	}

	credsCopy := make(map[string]string, len(resolved))
	maps.Copy(credsCopy, resolved)
	ctx.Response(&credentials.ResolveResult{Credentials: credsCopy})
}
