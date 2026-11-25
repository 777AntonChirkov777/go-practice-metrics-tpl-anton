package server_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"practice/internal/encryption"
	"practice/internal/hash"
	"practice/internal/model"
	"practice/internal/server"
	"practice/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var routerKey = sync.OnceValues(func() (*rsa.PrivateKey, error) { return rsa.GenerateKey(rand.Reader, 2048) })

func routerPrivateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := routerKey()
	require.NoError(t, err)
	return key
}

func testBatch() []model.Metrics {
	return []model.Metrics{
		model.NewGaugeDTO("Alloc", 1.5),
		model.NewCounterDTO(pollCountName, 3),
	}
}

func encryptedRequest(t *testing.T, pub *rsa.PublicKey, path string, plain []byte, gzipped bool) *http.Request {
	t.Helper()

	payload := plain
	if gzipped {
		payload = mustGzip(plain)
	}

	data, err := encryption.Encrypt(pub, payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	if gzipped {
		req.Header.Set("Content-Encoding", "gzip")
	}
	req.Header.Set(encryption.Header, encryption.Scheme)
	return req
}

func serveRouter(r http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func storedCount(t *testing.T, store *storage.MemStorage) int {
	t.Helper()
	all, err := store.GetAll(context.Background())
	require.NoError(t, err)
	return len(all)
}

func TestRouterAcceptsEncryptedBatch(t *testing.T) {
	priv := routerPrivateKey(t)
	store := storage.NewMemStorage()
	r := server.NewRouter(store, nullAuditor{}, nil, "", priv)

	rec := serveRouter(r, encryptedRequest(t, &priv.PublicKey, "/updates/", mustJSON(testBatch()), true))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	alloc, err := store.Get(context.Background(), model.Gauge, "Alloc")
	require.NoError(t, err)
	assert.Equal(t, 1.5, alloc.Value)

	pollCount, err := store.Get(context.Background(), model.Counter, pollCountName)
	require.NoError(t, err)
	assert.Equal(t, int64(3), pollCount.Delta)
}

func TestRouterVerifiesSignatureOfDecryptedJSON(t *testing.T) {
	priv := routerPrivateKey(t)
	plain := mustJSON(testBatch())

	store := storage.NewMemStorage()
	r := server.NewRouter(store, nullAuditor{}, nil, routerTestKey, priv)

	req := encryptedRequest(t, &priv.PublicKey, "/updates/", plain, true)
	req.Header.Set(hash.Header, hash.Sum(routerTestKey, plain))
	rec := serveRouter(r, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 2, storedCount(t, store))

	other := storage.NewMemStorage()
	r = server.NewRouter(other, nullAuditor{}, nil, routerTestKey, priv)

	req = encryptedRequest(t, &priv.PublicKey, "/updates/", plain, true)
	req.Header.Set(hash.Header, hash.Sum("another-key", plain))
	rec = serveRouter(r, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Zero(t, storedCount(t, other))
}

func TestRouterRejectsUndecryptableBody(t *testing.T) {
	priv := routerPrivateKey(t)
	store := storage.NewMemStorage()
	r := server.NewRouter(store, nullAuditor{}, nil, "", priv)

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(bytes.Repeat([]byte{1}, 512)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set(encryption.Header, encryption.Scheme)

	rec := serveRouter(r, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Zero(t, storedCount(t, store))
}

func TestRouterWithKeyRejectsPlainJSON(t *testing.T) {
	store := storage.NewMemStorage()
	r := server.NewRouter(store, nullAuditor{}, nil, "", routerPrivateKey(t))

	req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader(mustJSON(model.NewGaugeDTO("Alloc", 1))))
	req.Header.Set("Content-Type", "application/json")

	rec := serveRouter(r, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Zero(t, storedCount(t, store))
}

func TestRouterWithKeyServesBodylessRequests(t *testing.T) {
	store := storage.NewMemStorage()
	withKey := server.NewRouter(store, nullAuditor{}, nil, "", routerPrivateKey(t))
	withoutKey := server.NewRouter(store, nullAuditor{}, nil, "", nil)

	rec := serveRouter(withKey, httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/2.5", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	rec = serveRouter(withKey, httptest.NewRequest(http.MethodGet, "/value/gauge/Alloc", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "2.5", rec.Body.String())

	got := serveRouter(withKey, httptest.NewRequest(http.MethodGet, "/", nil))
	want := serveRouter(withoutKey, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, want.Code, got.Code)
	assert.Equal(t, want.Body.String(), got.Body.String())
	assert.Equal(t, want.Header(), got.Header())
}

func TestRouterWithoutKeyRejectsEncryptedRequest(t *testing.T) {
	priv := routerPrivateKey(t)
	store := storage.NewMemStorage()
	r := server.NewRouter(store, nullAuditor{}, nil, "", nil)

	rec := serveRouter(r, encryptedRequest(t, &priv.PublicKey, "/updates/", mustJSON(testBatch()), true))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Zero(t, storedCount(t, store))

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(mustGzip(mustJSON(testBatch()))))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	rec = serveRouter(r, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 2, storedCount(t, store))
}

func TestRouterRejectsUnknownEncryptionScheme(t *testing.T) {
	priv := routerPrivateKey(t)
	store := storage.NewMemStorage()
	r := server.NewRouter(store, nullAuditor{}, nil, "", priv)

	req := encryptedRequest(t, &priv.PublicKey, "/updates/", mustJSON(testBatch()), true)
	req.Header.Set(encryption.Header, "rsa-pkcs1")

	rec := serveRouter(r, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Zero(t, storedCount(t, store))
}

func TestRouterResponseToEncryptedRequestIsPlain(t *testing.T) {
	priv := routerPrivateKey(t)
	r := server.NewRouter(storage.NewMemStorage(), nullAuditor{}, nil, "", priv)

	rec := serveRouter(r, encryptedRequest(t, &priv.PublicKey, "/update/", mustJSON(model.NewGaugeDTO("Alloc", 4.25)), false))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, rec.Header().Values(encryption.Header))

	var out model.Metrics
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, "Alloc", out.ID)
	require.NotNil(t, out.Value)
	assert.Equal(t, 4.25, *out.Value)
}
