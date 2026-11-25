package encryption

import (
	"bytes"
	"crypto/rsa"
	"errors"
	"io"
	"net/http"
)

const maxBodyBytes = 4 << 20

func Middleware(priv *rsa.PrivateKey) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			if len(req.Header.Values(Header)) > 0 {
				if decryptRequest(res, req, priv) {
					next.ServeHTTP(res, req)
				}
				return
			}

			if priv != nil && !bodyIsEmpty(req) {
				http.Error(res, "encrypted request body required", http.StatusBadRequest)
				return
			}

			next.ServeHTTP(res, req)
		})
	}
}

func decryptRequest(res http.ResponseWriter, req *http.Request, priv *rsa.PrivateKey) bool {
	if req.Header.Get(Header) != Scheme {
		http.Error(res, "unsupported encryption scheme", http.StatusBadRequest)
		return false
	}

	if priv == nil {
		http.Error(res, "encryption is not configured", http.StatusBadRequest)
		return false
	}

	data, err := io.ReadAll(http.MaxBytesReader(res, req.Body, maxBodyBytes))
	if err != nil {
		http.Error(res, "cannot read request body", http.StatusBadRequest)
		return false
	}

	plain, err := Decrypt(priv, data)
	if err != nil {
		http.Error(res, "cannot decrypt request body", http.StatusBadRequest)
		return false
	}

	req.Body = io.NopCloser(bytes.NewReader(plain))
	req.ContentLength = int64(len(plain))
	req.Header.Del(Header)

	return true
}

func bodyIsEmpty(req *http.Request) bool {
	if req.Body == nil || req.Body == http.NoBody || req.ContentLength == 0 {
		return true
	}

	var probe [1]byte
	if _, err := io.ReadFull(req.Body, probe[:]); !errors.Is(err, io.EOF) {
		return false
	}

	req.Body = http.NoBody
	return true
}
