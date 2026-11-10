package server_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"practice/internal/hash"
	model "practice/internal/model"
	"practice/internal/server"
	"practice/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const routerTestKey = "router-test-key"

func TestRouterTextEndpointsRoundTrip(t *testing.T) {
	r := server.NewRouter(storage.NewMemStorage(), nullAuditor{}, nil, "")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/1.5", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/value/gauge/Alloc", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "1.5", rec.Body.String())
}

func TestRouterSignedGzipJSONRoundTrip(t *testing.T) {
	r := server.NewRouter(storage.NewMemStorage(), nullAuditor{}, nil, routerTestKey)

	body := mustJSON(model.NewCounterDTO(pollCountName, 7))

	req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader(mustGzip(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set(hash.Header, hash.Sum(routerTestKey, body))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))

	zr, err := gzip.NewReader(rec.Body)
	require.NoError(t, err)
	plain, err := io.ReadAll(zr)
	require.NoError(t, err)
	require.NoError(t, zr.Close())

	assert.Equal(t, hash.Sum(routerTestKey, plain), rec.Header().Get(hash.Header))

	var out model.Metrics
	require.NoError(t, json.Unmarshal(plain, &out))
	require.NotNil(t, out.Delta)
	assert.Equal(t, int64(7), *out.Delta)
}

func TestRouterRejectsInvalidSignature(t *testing.T) {
	r := server.NewRouter(storage.NewMemStorage(), nullAuditor{}, nil, routerTestKey)

	req := httptest.NewRequest(http.MethodPost, "/update/",
		bytes.NewReader(mustJSON(model.NewGaugeDTO("Alloc", 1))))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(hash.Header, "deadbeef")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRouterBatchThenList(t *testing.T) {
	r := server.NewRouter(storage.NewMemStorage(), nullAuditor{}, nil, "")

	batch := []model.Metrics{
		model.NewGaugeDTO("Alloc", 2.5),
		model.NewCounterDTO(pollCountName, 3),
	}

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(mustJSON(batch)))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, rec.Body.String(), "Alloc")
	assert.Contains(t, rec.Body.String(), pollCountName)
}

func TestRouterPingWithoutDatabase(t *testing.T) {
	r := server.NewRouter(storage.NewMemStorage(), nullAuditor{}, nil, "")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping", nil))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
