package encryption

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

const (
	Header     = "X-Encryption"
	Scheme     = "rsa-oaep-aes256gcm"
	MinKeyBits = 2048
)

var (
	ErrNoPEM          = errors.New("в файле ключа нет PEM-блока")
	ErrUnsupportedPEM = errors.New("неподдерживаемый тип PEM-блока")
	ErrNotRSA         = errors.New("ключ не является ключом RSA")
	ErrWeakKey        = fmt.Errorf("ключ RSA короче %d бит", MinKeyBits)
)

func ParsePublicKey(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, ErrNoPEM
	}

	var pub *rsa.PublicKey
	switch block.Type {
	case "PUBLIC KEY":
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, ErrNotRSA
		}
		pub = rsaKey
	case "RSA PUBLIC KEY":
		key, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		pub = key
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedPEM, block.Type)
	}

	if pub.N.BitLen() < MinKeyBits {
		return nil, ErrWeakKey
	}

	return pub, nil
}

func ParsePrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, ErrNoPEM
	}

	var priv *rsa.PrivateKey
	switch block.Type {
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, ErrNotRSA
		}
		priv = rsaKey
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		priv = key
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedPEM, block.Type)
	}

	if priv.N.BitLen() < MinKeyBits {
		return nil, ErrWeakKey
	}

	return priv, nil
}

func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("чтение публичного ключа: %w", err)
	}

	key, err := ParsePublicKey(data)
	if err != nil {
		return nil, fmt.Errorf("публичный ключ %s: %w", path, err)
	}

	return key, nil
}

func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("чтение приватного ключа: %w", err)
	}

	key, err := ParsePrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("приватный ключ %s: %w", path, err)
	}

	return key, nil
}
