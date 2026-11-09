package compress

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

var benchPlainBody = []byte(`[{"id":"Alloc","type":"gauge","value":123456.75},` +
	`{"id":"HeapInuse","type":"gauge","value":98765.5},` +
	`{"id":"PollCount","type":"counter","delta":17}]`)

var benchGzipBody = benchGzip(benchPlainBody)

func benchGzip(plain []byte) []byte {
	var buf bytes.Buffer

	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(plain); err != nil {
		panic(err)
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}

	return buf.Bytes()
}

func benchBodyReader() http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		_, _ = io.Copy(io.Discard, req.Body)
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		_, _ = res.Write(benchPlainBody)
	})
}

func BenchmarkMiddlewareCompressedResponse(b *testing.B) {
	h := Middleware(http.HandlerFunc(jsonHandler))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/value/", nil)
		req.Header.Set("Accept-Encoding", "gzip")

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			b.Fatalf("iteration %d answered %d", i, rec.Code)
		}
	}
}

func BenchmarkMiddlewareCompressedRequest(b *testing.B) {
	h := Middleware(benchBodyReader())

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(benchGzipBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Accept-Encoding", "gzip")

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			b.Fatalf("iteration %d answered %d", i, rec.Code)
		}
	}
}

func BenchmarkMiddlewarePassThrough(b *testing.B) {
	h := Middleware(benchBodyReader())

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(benchPlainBody))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			b.Fatalf("iteration %d answered %d", i, rec.Code)
		}
	}
}
