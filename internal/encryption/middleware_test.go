package encryption

import (
	"bytes"
	"crypto/rsa"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type seenRequest struct {
	called        bool
	body          []byte
	contentLength int64
	marker        []string
}

func recordingHandler(seen *seenRequest) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		seen.called = true
		seen.contentLength = req.ContentLength
		seen.marker = req.Header.Values(Header)

		body, err := io.ReadAll(req.Body)
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		seen.body = body
		res.WriteHeader(http.StatusOK)
	})
}

func serve(priv *rsa.PrivateKey, req *http.Request) (*httptest.ResponseRecorder, *seenRequest) {
	seen := &seenRequest{}
	rec := httptest.NewRecorder()
	Middleware(priv)(recordingHandler(seen)).ServeHTTP(rec, req)
	return rec, seen
}

func encryptedRequest(t *testing.T, pub *rsa.PublicKey, plain []byte) *http.Request {
	t.Helper()
	data, err := Encrypt(pub, plain)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(data))
	req.Header.Set(Header, Scheme)
	return req
}

func TestMiddlewareDecryptsMarkedRequest(t *testing.T) {
	priv := testPrivateKey(t)

	rec, seen := serve(priv, encryptedRequest(t, &priv.PublicKey, testPlaintext))

	assert.Equal(t, http.StatusOK, rec.Code)
	require.True(t, seen.called)
	assert.Equal(t, testPlaintext, seen.body)
	assert.Equal(t, int64(len(testPlaintext)), seen.contentLength)
	assert.Empty(t, seen.marker)
}

func TestMiddlewareRejectsUndecryptableBody(t *testing.T) {
	priv := testPrivateKey(t)

	tests := []struct {
		name string
		req  *http.Request
	}{
		{name: "foreign key", req: encryptedRequest(t, &testOtherKey(t).PublicKey, testPlaintext)},
		{name: "garbage", req: func() *http.Request {
			req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(bytes.Repeat([]byte{7}, 512)))
			req.Header.Set(Header, Scheme)
			return req
		}()},
		{name: "empty body", req: func() *http.Request {
			req := httptest.NewRequest(http.MethodPost, "/updates/", nil)
			req.Header.Set(Header, Scheme)
			return req
		}()},
		{name: "too large", req: func() *http.Request {
			req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(make([]byte, maxBodyBytes+1)))
			req.Header.Set(Header, Scheme)
			return req
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, seen := serve(priv, tt.req)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.False(t, seen.called)
		})
	}
}

func TestMiddlewareWithKeyRequiresEncryption(t *testing.T) {
	priv := testPrivateKey(t)

	req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader(testPlaintext))
	rec, seen := serve(priv, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.False(t, seen.called)

	chunked := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader(testPlaintext))
	chunked.ContentLength = -1
	rec, seen = serve(priv, chunked)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.False(t, seen.called)
}

func TestMiddlewareWithKeyPassesEmptyBody(t *testing.T) {
	priv := testPrivateKey(t)

	tests := []struct {
		name string
		req  *http.Request
	}{
		{name: "no body", req: httptest.NewRequest(http.MethodGet, "/", nil)},
		{name: "text route", req: httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/1", http.NoBody)},
		{name: "unknown length, empty", req: func() *http.Request {
			req := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/1", io.NopCloser(bytes.NewReader(nil)))
			req.ContentLength = -1
			return req
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, seen := serve(priv, tt.req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.True(t, seen.called)
			assert.Empty(t, seen.body)
		})
	}
}

func TestMiddlewareWithoutKey(t *testing.T) {
	priv := testPrivateKey(t)

	rec, seen := serve(nil, encryptedRequest(t, &priv.PublicKey, testPlaintext))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.False(t, seen.called)

	req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader(testPlaintext))
	rec, seen = serve(nil, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	require.True(t, seen.called)
	assert.Equal(t, testPlaintext, seen.body)
	assert.Equal(t, int64(len(testPlaintext)), seen.contentLength)
}

func TestMiddlewareRejectsUnknownScheme(t *testing.T) {
	priv := testPrivateKey(t)

	for _, key := range []*rsa.PrivateKey{priv, nil} {
		req := encryptedRequest(t, &priv.PublicKey, testPlaintext)
		req.Header.Set(Header, "rsa-pkcs1")

		rec, seen := serve(key, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.False(t, seen.called)
	}
}
