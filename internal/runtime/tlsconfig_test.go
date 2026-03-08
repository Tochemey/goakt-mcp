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

package runtime

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tochemey/goakt-mcp/mcp"
)

func TestBuildClientTLSConfig(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		cfg, err := mcp.BuildClientTLSConfig(nil)
		require.NoError(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("insecure skip verify", func(t *testing.T) {
		cfg, err := mcp.BuildClientTLSConfig(&mcp.EgressTLSConfig{InsecureSkipVerify: true})
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.True(t, cfg.InsecureSkipVerify)
		assert.GreaterOrEqual(t, cfg.MinVersion, uint16(tls.VersionTLS12))
	})

	t.Run("empty config returns defaults", func(t *testing.T) {
		cfg, err := mcp.BuildClientTLSConfig(&mcp.EgressTLSConfig{})
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.False(t, cfg.InsecureSkipVerify)
		assert.Nil(t, cfg.RootCAs)
		assert.Empty(t, cfg.Certificates)
	})

	t.Run("missing CA cert file", func(t *testing.T) {
		_, err := mcp.BuildClientTLSConfig(&mcp.EgressTLSConfig{CACertFile: "/nonexistent-ca.crt"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read CA cert")
	})

	t.Run("invalid CA cert content", func(t *testing.T) {
		dir := t.TempDir()
		caFile := filepath.Join(dir, "bad-ca.crt")
		require.NoError(t, os.WriteFile(caFile, []byte("not a certificate"), 0o600))

		_, err := mcp.BuildClientTLSConfig(&mcp.EgressTLSConfig{CACertFile: caFile})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse CA cert")
	})

	t.Run("valid CA cert", func(t *testing.T) {
		dir := t.TempDir()
		caFile := filepath.Join(dir, "ca.crt")
		require.NoError(t, os.WriteFile(caFile, testCACert, 0o600))

		cfg, err := mcp.BuildClientTLSConfig(&mcp.EgressTLSConfig{CACertFile: caFile})
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.NotNil(t, cfg.RootCAs)
	})

	t.Run("missing client cert file", func(t *testing.T) {
		_, err := mcp.BuildClientTLSConfig(&mcp.EgressTLSConfig{
			ClientCertFile: "/nonexistent.crt",
			ClientKeyFile:  "/nonexistent.key",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "load client cert")
	})
}

func TestBuildServerTLSConfig(t *testing.T) {
	t.Run("missing cert file", func(t *testing.T) {
		_, err := mcp.BuildServerTLSConfig("/nonexistent.crt", "/nonexistent.key", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "load server cert")
	})

	t.Run("missing client CA file", func(t *testing.T) {
		dir := t.TempDir()
		certFile := filepath.Join(dir, "server.crt")
		keyFile := filepath.Join(dir, "server.key")
		writeSelfSignedCert(t, certFile, keyFile)

		_, err := mcp.BuildServerTLSConfig(certFile, keyFile, "/nonexistent-ca.crt")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read client CA")
	})

	t.Run("invalid client CA content", func(t *testing.T) {
		dir := t.TempDir()
		certFile := filepath.Join(dir, "server.crt")
		keyFile := filepath.Join(dir, "server.key")
		writeSelfSignedCert(t, certFile, keyFile)

		caFile := filepath.Join(dir, "bad-ca.crt")
		require.NoError(t, os.WriteFile(caFile, []byte("not a cert"), 0o600))

		_, err := mcp.BuildServerTLSConfig(certFile, keyFile, caFile)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse client CA")
	})

	t.Run("valid without mTLS", func(t *testing.T) {
		dir := t.TempDir()
		certFile := filepath.Join(dir, "server.crt")
		keyFile := filepath.Join(dir, "server.key")
		writeSelfSignedCert(t, certFile, keyFile)

		cfg, err := mcp.BuildServerTLSConfig(certFile, keyFile, "")
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Len(t, cfg.Certificates, 1)
		assert.GreaterOrEqual(t, cfg.MinVersion, uint16(tls.VersionTLS12))
		assert.Nil(t, cfg.ClientCAs)
		assert.Equal(t, tls.NoClientCert, cfg.ClientAuth)
	})

	t.Run("valid with mTLS", func(t *testing.T) {
		dir := t.TempDir()
		certFile := filepath.Join(dir, "server.crt")
		keyFile := filepath.Join(dir, "server.key")
		writeSelfSignedCert(t, certFile, keyFile)

		caFile := filepath.Join(dir, "client-ca.crt")
		require.NoError(t, os.WriteFile(caFile, testCACert, 0o600))

		cfg, err := mcp.BuildServerTLSConfig(certFile, keyFile, caFile)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.NotNil(t, cfg.ClientCAs)
		assert.Equal(t, tls.RequireAndVerifyClientCert, cfg.ClientAuth)
	})
}
