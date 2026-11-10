package hash

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

const benchSigningKey = "benchmark-signing-key"

var (
	benchBatchBody = []byte(`[{"id":"Alloc","type":"gauge","value":123456.75},` +
		`{"id":"HeapInuse","type":"gauge","value":98765.5},` +
		`{"id":"PollCount","type":"counter","delta":17}]`)
	benchSumSink   string
	benchEqualSink bool
)

func benchJSONHandler() http.Handler {
	body := []byte(`{"id":"PollCount","type":"counter","delta":17}`)

	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		_, _ = io.Copy(io.Discard, req.Body)
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		_, _ = res.Write(body)
	})
}

func BenchmarkSum(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchSumSink = Sum(benchSigningKey, benchBatchBody)
	}
}

func BenchmarkEqual(b *testing.B) {
	want := Sum(benchSigningKey, benchBatchBody)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchEqualSink = Equal(benchSigningKey, benchBatchBody, want)
	}
}

func BenchmarkMiddleware(b *testing.B) {
	h := Middleware(benchSigningKey)(benchJSONHandler())
	sig := Sum(benchSigningKey, benchBatchBody)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(benchBatchBody))
		req.Header.Set(Header, sig)

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			b.Fatalf("iteration %d answered %d", i, rec.Code)
		}
	}
}
