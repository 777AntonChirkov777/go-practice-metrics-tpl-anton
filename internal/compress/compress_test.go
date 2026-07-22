package compress

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jsonHandler пишет фиксированный JSON — самый частый сжимаемый ответ.
func jsonHandler(res http.ResponseWriter, _ *http.Request) {
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	res.Write([]byte(`{"id":"x","type":"gauge","value":1}`))
}

func TestMiddlewareCompressesJSONWhenClientAcceptsGzip(t *testing.T) {
	h := Middleware(http.HandlerFunc(jsonHandler))

	req := httptest.NewRequest(http.MethodPost, "/value/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	assert.Equal(t, "gzip", res.Header.Get("Content-Encoding"))
	// Content-Length от несжатого тела не должен утечь в ответ.
	assert.Empty(t, res.Header.Get("Content-Length"))

	zr, err := gzip.NewReader(res.Body)
	require.NoError(t, err)
	decoded, err := io.ReadAll(zr)
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":"x","type":"gauge","value":1}`, string(decoded))
}

func TestMiddlewareCompressesHTML(t *testing.T) {
	h := Middleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
		res.Header().Set("Content-Type", "text/html; charset=utf-8")
		res.WriteHeader(http.StatusOK)
		res.Write([]byte("<html><body>metrics</body></html>"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	assert.Equal(t, "gzip", res.Header.Get("Content-Encoding"))

	zr, err := gzip.NewReader(res.Body)
	require.NoError(t, err)
	decoded, err := io.ReadAll(zr)
	require.NoError(t, err)
	assert.Contains(t, string(decoded), "<body>metrics</body>")
}

func TestMiddlewareLeavesBodyPlainWhenClientLacksGzip(t *testing.T) {
	h := Middleware(http.HandlerFunc(jsonHandler))

	// Заголовка Accept-Encoding нет — сжимать нельзя.
	req := httptest.NewRequest(http.MethodPost, "/value/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	assert.Empty(t, res.Header.Get("Content-Encoding"))
	body, _ := io.ReadAll(res.Body)
	assert.JSONEq(t, `{"id":"x","type":"gauge","value":1}`, string(body))
}

func TestMiddlewareSkipsNonCompressibleContentType(t *testing.T) {
	// text/plain (его ставит http.Error) сжимать по заданию не нужно.
	h := Middleware(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
		http.Error(res, "metric not found", http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodPost, "/value/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	assert.Empty(t, res.Header.Get("Content-Encoding"))
	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	body, _ := io.ReadAll(res.Body)
	assert.Contains(t, string(body), "metric not found")
}

func TestMiddlewareDecompressesRequestBody(t *testing.T) {
	const payload = `{"id":"PollCount","type":"counter","delta":5}`

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write([]byte(payload))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	var seen string
	h := Middleware(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Хендлер обязан увидеть уже распакованный поток.
		b, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		seen = string(b)
		res.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/update/", &buf)
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, payload, seen)
}

func TestMiddlewareRejectsInvalidGzipBody(t *testing.T) {
	// Заголовок обещает gzip, а тело — не gzip: должен быть 400,
	// хендлер вызываться не должен.
	called := false
	h := Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodPost, "/update/", strings.NewReader("not gzip at all"))
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.False(t, called, "handler must not run on undecodable body")
}

func TestMiddlewareRoundTripThroughServer(t *testing.T) {
	// Полный круг: сжатый запрос -> распаковка -> эхо -> сжатый ответ,
	// прочитанный через реальный net/http.
	srv := httptest.NewServer(Middleware(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		res.Write(body)
	})))
	defer srv.Close()

	const payload = `{"id":"Alloc","type":"gauge","value":42}`
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write([]byte(payload))
	require.NoError(t, zw.Close())

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/update/", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	// Accept-Encoding: gzip Transport добавит сам и прозрачно распакует ответ.

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.JSONEq(t, payload, string(got))
}

func TestIsCompressible(t *testing.T) {
	cases := map[string]bool{
		"application/json":            true,
		"application/json; charset=1": true,
		"text/html":                   true,
		"text/html; charset=utf-8":    true,
		"TEXT/HTML":                   true,
		"text/plain; charset=utf-8":   false,
		"application/octet-stream":    false,
		"":                            false,
	}
	for ct, want := range cases {
		assert.Equalf(t, want, isCompressible(ct), "content-type %q", ct)
	}
}
