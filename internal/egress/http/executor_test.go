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

package http

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tochemey/goakt-mcp/mcp"
)

func TestNewHTTPExecutor_Validation(t *testing.T) {
	t.Run("nil config returns error", func(t *testing.T) {
		exec, err := NewHTTPExecutor(nil, nil, time.Second)
		assert.Nil(t, exec)
		require.Error(t, err)
		var rErr *mcp.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, mcp.ErrCodeInvalidRequest, rErr.Code)
	})

	t.Run("empty URL returns error", func(t *testing.T) {
		cfg := &mcp.HTTPTransportConfig{URL: ""}
		exec, err := NewHTTPExecutor(cfg, nil, time.Second)
		assert.Nil(t, exec)
		require.Error(t, err)
		var rErr *mcp.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, mcp.ErrCodeInvalidRequest, rErr.Code)
	})

	t.Run("unreachable endpoint returns transport failure", func(t *testing.T) {
		cfg := &mcp.HTTPTransportConfig{URL: "http://127.0.0.1:1/unreachable"}
		exec, err := NewHTTPExecutor(cfg, nil, 500*time.Millisecond)
		assert.Nil(t, exec)
		require.Error(t, err)
		var rErr *mcp.RuntimeError
		require.ErrorAs(t, err, &rErr)
		assert.Equal(t, mcp.ErrCodeTransportFailure, rErr.Code)
	})
}

func TestHTTPExecutor_Execute_NilSession(t *testing.T) {
	e := &HTTPExecutor{}
	inv := &mcp.Invocation{
		ToolID: "test",
		Correlation: mcp.CorrelationMeta{
			RequestID: "req-1",
		},
	}
	result, err := e.Execute(context.Background(), inv)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, mcp.ExecutionStatusFailure, result.Status)
	assert.Equal(t, mcp.ErrCodeTransportFailure, result.Err.Code)
}

func TestHTTPExecutor_Close_Idempotent(t *testing.T) {
	e := &HTTPExecutor{}
	require.NoError(t, e.Close())
	require.NoError(t, e.Close())
}

// customRoundTripper is a non-*http.Transport RoundTripper used to test the
// fallback path in buildHTTPClient where base is not cloneable.
type customRoundTripper struct{}

func (customRoundTripper) RoundTrip(*http.Request) (*http.Response, error) { return nil, nil }

func writeSelfSignedCert(t *testing.T, certFile, keyFile string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	require.NoError(t, os.WriteFile(certFile, certPEM, 0o600))

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	require.NoError(t, os.WriteFile(keyFile, keyPEM, 0o600))
}

func TestBuildHTTPClient(t *testing.T) {
	t.Run("nil TLS uses fallback client", func(t *testing.T) {
		fallback := &http.Client{}
		cfg := &mcp.HTTPTransportConfig{URL: "http://example.com", TLS: nil}
		client, err := buildHTTPClient(cfg, fallback)
		require.NoError(t, err)
		assert.Equal(t, fallback, client)
	})

	t.Run("nil TLS nil fallback returns default client", func(t *testing.T) {
		cfg := &mcp.HTTPTransportConfig{URL: "http://example.com", TLS: nil}
		client, err := buildHTTPClient(cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, client)
	})

	t.Run("TLS with invalid CA returns error", func(t *testing.T) {
		dir := t.TempDir()
		badCA := filepath.Join(dir, "bad-ca.crt")
		require.NoError(t, os.WriteFile(badCA, []byte("not-a-cert"), 0o600))

		cfg := &mcp.HTTPTransportConfig{
			URL: "https://example.com",
			TLS: &mcp.EgressTLSConfig{CACertFile: badCA},
		}
		_, err := buildHTTPClient(cfg, nil)
		require.Error(t, err)
	})

	t.Run("TLS insecure skip verify with fallback non-http.Transport", func(t *testing.T) {
		fallback := &http.Client{Transport: customRoundTripper{}}
		cfg := &mcp.HTTPTransportConfig{
			URL: "https://example.com",
			TLS: &mcp.EgressTLSConfig{InsecureSkipVerify: true},
		}
		client, err := buildHTTPClient(cfg, fallback)
		require.NoError(t, err)
		require.NotNil(t, client)
	})

	t.Run("TLS insecure skip verify with default transport", func(t *testing.T) {
		cfg := &mcp.HTTPTransportConfig{
			URL: "https://example.com",
			TLS: &mcp.EgressTLSConfig{InsecureSkipVerify: true},
		}
		client, err := buildHTTPClient(cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, client)
		transport, ok := client.Transport.(*http.Transport)
		require.True(t, ok)
		require.NotNil(t, transport.TLSClientConfig)
		assert.True(t, transport.TLSClientConfig.InsecureSkipVerify) //nolint:gosec
	})

	t.Run("TLS with valid client cert and fallback http.Transport", func(t *testing.T) {
		dir := t.TempDir()
		certFile := filepath.Join(dir, "client.crt")
		keyFile := filepath.Join(dir, "client.key")
		writeSelfSignedCert(t, certFile, keyFile)

		fallback := &http.Client{Transport: &http.Transport{}}
		cfg := &mcp.HTTPTransportConfig{
			URL: "https://example.com",
			TLS: &mcp.EgressTLSConfig{
				ClientCertFile: certFile,
				ClientKeyFile:  keyFile,
			},
		}
		client, err := buildHTTPClient(cfg, fallback)
		require.NoError(t, err)
		require.NotNil(t, client)
	})
}
