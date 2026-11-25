package encryption

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	serverKey = sync.OnceValues(func() (*rsa.PrivateKey, error) { return rsa.GenerateKey(rand.Reader, 2048) })
	otherKey  = sync.OnceValues(func() (*rsa.PrivateKey, error) { return rsa.GenerateKey(rand.Reader, 2048) })
)

func testPrivateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := serverKey()
	require.NoError(t, err)
	return key
}

func testOtherKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := otherKey()
	require.NoError(t, err)
	return key
}

func encodePEM(blockType string, der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
}

func pkixPEM(t *testing.T, pub any) []byte {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(pub)
	require.NoError(t, err)
	return encodePEM("PUBLIC KEY", der)
}

func pkcs8PEM(t *testing.T, priv any) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)
	return encodePEM("PRIVATE KEY", der)
}

func TestParsePublicKeyFormats(t *testing.T) {
	priv := testPrivateKey(t)

	tests := []struct {
		name string
		data []byte
	}{
		{name: "PKIX", data: pkixPEM(t, &priv.PublicKey)},
		{name: "PKCS1", data: encodePEM("RSA PUBLIC KEY", x509.MarshalPKCS1PublicKey(&priv.PublicKey))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pub, err := ParsePublicKey(tt.data)
			require.NoError(t, err)
			assert.True(t, pub.Equal(&priv.PublicKey))
		})
	}
}

func TestParsePrivateKeyFormats(t *testing.T) {
	priv := testPrivateKey(t)

	tests := []struct {
		name string
		data []byte
	}{
		{name: "PKCS8", data: pkcs8PEM(t, priv)},
		{name: "PKCS1", data: encodePEM("RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(priv))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePrivateKey(tt.data)
			require.NoError(t, err)
			assert.True(t, got.Equal(priv))
		})
	}
}

func TestParseKeyErrors(t *testing.T) {
	priv := testPrivateKey(t)

	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	weak, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)

	tests := []struct {
		name    string
		private bool
		data    []byte
		want    error
	}{
		{name: "public: not PEM", data: []byte("not a key"), want: ErrNoPEM},
		{name: "public: private key block", data: pkcs8PEM(t, priv), want: ErrUnsupportedPEM},
		{name: "public: certificate block", data: encodePEM("CERTIFICATE", []byte{1, 2, 3}), want: ErrUnsupportedPEM},
		{name: "public: ECDSA", data: pkixPEM(t, &ecKey.PublicKey), want: ErrNotRSA},
		{name: "public: RSA 1024", data: pkixPEM(t, &weak.PublicKey), want: ErrWeakKey},
		{name: "public: broken DER", data: encodePEM("PUBLIC KEY", []byte("garbage"))},
		{name: "public: broken PKCS1 DER", data: encodePEM("RSA PUBLIC KEY", []byte("garbage"))},
		{name: "private: not PEM", private: true, data: []byte("not a key"), want: ErrNoPEM},
		{name: "private: public key block", private: true, data: pkixPEM(t, &priv.PublicKey), want: ErrUnsupportedPEM},
		{name: "private: encrypted block", private: true, data: encodePEM("ENCRYPTED PRIVATE KEY", []byte{1, 2, 3}), want: ErrUnsupportedPEM},
		{name: "private: ECDSA", private: true, data: pkcs8PEM(t, ecKey), want: ErrNotRSA},
		{name: "private: RSA 1024", private: true, data: encodePEM("RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(weak)), want: ErrWeakKey},
		{name: "private: broken DER", private: true, data: encodePEM("RSA PRIVATE KEY", []byte("garbage"))},
		{name: "private: broken PKCS8 DER", private: true, data: encodePEM("PRIVATE KEY", []byte("garbage"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.private {
				_, err = ParsePrivateKey(tt.data)
			} else {
				_, err = ParsePublicKey(tt.data)
			}

			require.Error(t, err)
			if tt.want != nil {
				assert.ErrorIs(t, err, tt.want)
			}
		})
	}
}

func TestLoadKeys(t *testing.T) {
	priv := testPrivateKey(t)
	dir := t.TempDir()

	pubPath := filepath.Join(dir, "public.pem")
	privPath := filepath.Join(dir, "private.pem")
	brokenPath := filepath.Join(dir, "broken.pem")
	missingPath := filepath.Join(dir, "missing.pem")

	require.NoError(t, os.WriteFile(pubPath, pkixPEM(t, &priv.PublicKey), 0o600))
	require.NoError(t, os.WriteFile(privPath, pkcs8PEM(t, priv), 0o600))
	require.NoError(t, os.WriteFile(brokenPath, []byte("not a key"), 0o600))

	pub, err := LoadPublicKey(pubPath)
	require.NoError(t, err)
	assert.True(t, pub.Equal(&priv.PublicKey))

	got, err := LoadPrivateKey(privPath)
	require.NoError(t, err)
	assert.True(t, got.Equal(priv))

	_, err = LoadPublicKey(missingPath)
	assert.ErrorIs(t, err, fs.ErrNotExist)

	_, err = LoadPrivateKey(missingPath)
	assert.ErrorIs(t, err, fs.ErrNotExist)

	_, err = LoadPublicKey(brokenPath)
	assert.ErrorIs(t, err, ErrNoPEM)
	assert.Contains(t, err.Error(), brokenPath)

	_, err = LoadPrivateKey(brokenPath)
	assert.ErrorIs(t, err, ErrNoPEM)
	assert.Contains(t, err.Error(), brokenPath)
}
