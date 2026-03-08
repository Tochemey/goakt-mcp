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

package http

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"time"

	goaktactor "github.com/tochemey/goakt/v4/actor"
	goaktlog "github.com/tochemey/goakt/v4/log"

	"github.com/tochemey/goakt-mcp/internal/runtime"
	"github.com/tochemey/goakt-mcp/internal/runtime/config"
)

const (
	routeInvoke = "POST /v1/tools/{tool}/invoke"
	routeTools  = "GET /v1/tools"

	defaultReadTimeout       = 30 * time.Second
	defaultReadHeaderTimeout = 10 * time.Second
	defaultWriteTimeout      = 60 * time.Second
	defaultIdleTimeout       = 120 * time.Second
	defaultMaxHeaderBytes    = 1 << 20 // 1 MiB
)

// Server is the HTTP data-plane server for the goakt-mcp gateway.
//
// It exposes POST /v1/tools/{tool}/invoke and GET /v1/tools, mapping requests
// to runtime actor messages and responses back to HTTP.
type Server struct {
	cfg    config.HTTPConfig
	system goaktactor.ActorSystem
	logger goaktlog.Logger
	server *http.Server
}

// NewServer creates an HTTP server that uses the given actor system for routing.
// The server is configured with production-ready timeouts and header limits.
func NewServer(cfg config.HTTPConfig, system goaktactor.ActorSystem, logger goaktlog.Logger) *Server {
	if logger == nil {
		logger = goaktlog.DiscardLogger
	}

	addr := cfg.ListenAddress
	if addr == "" {
		addr = config.DefaultHTTPListenAddress
	}

	s := &Server{cfg: cfg, system: system, logger: logger}

	mux := http.NewServeMux()
	mux.HandleFunc(routeInvoke, s.handleInvoke)
	mux.HandleFunc(routeTools, s.handleListTools)

	s.server = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadTimeout:       defaultReadTimeout,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
		MaxHeaderBytes:    defaultMaxHeaderBytes,
	}

	return s
}

// Handler returns the HTTP handler for use in tests or custom server wiring.
func (s *Server) Handler() http.Handler {
	return s.server.Handler
}

// Start starts the HTTP server. It blocks until the server is stopped or an error
// occurs. The provided context is monitored: when ctx is cancelled, the server
// is shut down gracefully. When HTTPConfig.TLS is set, the server uses TLS
// (and optionally mTLS).
func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		if s.isTLSEnabled() {
			errCh <- s.serveTLS()
		} else {
			s.logger.Infof("ingress http listening on %s", s.server.Addr)
			errCh <- s.server.ListenAndServe()
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return ctx.Err()
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// Stop gracefully shuts down the HTTP server within the given context deadline.
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

// isTLSEnabled reports whether ingress TLS configuration is present and complete.
func (s *Server) isTLSEnabled() bool {
	return s.cfg.TLS != nil && s.cfg.TLS.CertFile != "" && s.cfg.TLS.KeyFile != ""
}

// serveTLS starts the HTTPS listener with the configured certificate, key, and
// optional client CA for mutual TLS.
func (s *Server) serveTLS() error {
	tlsCfg, err := runtime.BuildServerTLSConfig(
		s.cfg.TLS.CertFile,
		s.cfg.TLS.KeyFile,
		s.cfg.TLS.ClientCAFile,
	)
	if err != nil {
		return err
	}

	listener, err := tls.Listen("tcp", s.server.Addr, tlsCfg)
	if err != nil {
		return err
	}

	s.logger.Infof("ingress https listening on %s (TLS)", s.server.Addr)
	return s.server.Serve(listener)
}
