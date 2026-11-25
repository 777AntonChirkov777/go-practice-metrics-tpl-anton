package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
)

const (
	aesKeySize = 32
	nonceSize  = 12
	tagSize    = 16
)

var ErrDecrypt = errors.New("cannot decrypt message")

func Encrypt(pub *rsa.PublicKey, plaintext []byte) ([]byte, error) {
	aesKey := make([]byte, aesKeySize)
	rand.Read(aesKey)

	wrapped, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, aesKey, nil)
	if err != nil {
		return nil, err
	}

	gcm, err := newGCM(aesKey)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, nonceSize)
	rand.Read(nonce)

	out := make([]byte, 0, len(wrapped)+nonceSize+len(plaintext)+gcm.Overhead())
	out = append(out, wrapped...)
	out = append(out, nonce...)

	return gcm.Seal(out, nonce, plaintext, nil), nil
}

func Decrypt(priv *rsa.PrivateKey, data []byte) ([]byte, error) {
	keyLen := priv.Size()
	if len(data) < keyLen+nonceSize+tagSize {
		return nil, ErrDecrypt
	}

	aesKey, err := rsa.DecryptOAEP(sha256.New(), nil, priv, data[:keyLen], nil)
	if err != nil || len(aesKey) != aesKeySize {
		return nil, ErrDecrypt
	}

	gcm, err := newGCM(aesKey)
	if err != nil {
		return nil, ErrDecrypt
	}

	plain, err := gcm.Open(nil, data[keyLen:keyLen+nonceSize], data[keyLen+nonceSize:], nil)
	if err != nil {
		return nil, ErrDecrypt
	}

	return plain, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	return cipher.NewGCM(block)
}
