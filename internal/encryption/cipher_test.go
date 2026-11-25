package encryption

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testPlaintext = []byte(`[{"id":"Alloc","type":"gauge","value":1.5},{"id":"PollCount","type":"counter","delta":3}]`)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	priv := testPrivateKey(t)

	tests := []struct {
		name  string
		plain []byte
	}{
		{name: "empty", plain: []byte{}},
		{name: "json", plain: testPlaintext},
		{name: "large", plain: bytes.Repeat([]byte("metrics"), 64<<10)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Encrypt(&priv.PublicKey, tt.plain)
			require.NoError(t, err)

			got, err := Decrypt(priv, data)
			require.NoError(t, err)
			assert.True(t, bytes.Equal(tt.plain, got))
		})
	}
}

func TestEncryptIsRandomized(t *testing.T) {
	priv := testPrivateKey(t)

	first, err := Encrypt(&priv.PublicKey, testPlaintext)
	require.NoError(t, err)
	second, err := Encrypt(&priv.PublicKey, testPlaintext)
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
}

func TestEncryptedLength(t *testing.T) {
	priv := testPrivateKey(t)

	for _, n := range []int{0, 1, 1000} {
		data, err := Encrypt(&priv.PublicKey, make([]byte, n))
		require.NoError(t, err)
		assert.Len(t, data, 256+12+n+16)
	}
}

func TestDecryptRejectsTamperedByte(t *testing.T) {
	priv := testPrivateKey(t)

	data, err := Encrypt(&priv.PublicKey, []byte("gauge"))
	require.NoError(t, err)

	for i := range data {
		tampered := bytes.Clone(data)
		tampered[i] ^= 0x01

		_, err := Decrypt(priv, tampered)
		require.ErrorIs(t, err, ErrDecrypt, "byte %d", i)
	}
}

func TestDecryptRejectsForeignKey(t *testing.T) {
	data, err := Encrypt(&testOtherKey(t).PublicKey, testPlaintext)
	require.NoError(t, err)

	_, err = Decrypt(testPrivateKey(t), data)
	assert.ErrorIs(t, err, ErrDecrypt)
}

func TestDecryptRejectsShortBody(t *testing.T) {
	priv := testPrivateKey(t)
	minLen := priv.Size() + nonceSize + tagSize

	for _, n := range []int{0, 1, priv.Size(), minLen - 1} {
		_, err := Decrypt(priv, make([]byte, n))
		assert.ErrorIs(t, err, ErrDecrypt, "length %d", n)
	}
}

func TestDecryptRejectsShortAESKey(t *testing.T) {
	priv := testPrivateKey(t)

	wrapped, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &priv.PublicKey, make([]byte, 16), nil)
	require.NoError(t, err)

	data := append(wrapped, make([]byte, nonceSize+tagSize)...)

	_, err = Decrypt(priv, data)
	assert.ErrorIs(t, err, ErrDecrypt)
}
