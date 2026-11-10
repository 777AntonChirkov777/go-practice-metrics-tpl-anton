package server_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"practice/internal/hash"
	model "practice/internal/model"
	"practice/internal/server"
	"practice/internal/storage"
)

const (
	benchKey      = "benchmark-signing-key"
	pollCountName = "PollCount"
)

const (
	kindUpdateText = iota
	kindUpdateJSON
	kindValueJSON
	kindUpdatesBatch
	kindList
	kindCount
)

var gaugeNames = []string{
	"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys",
	"HeapAlloc", "HeapIdle", "HeapInuse", "HeapObjects", "HeapReleased",
	"HeapSys", "LastGC", "Lookups", "MCacheInuse", "MCacheSys",
	"MSpanInuse", "MSpanSys", "Mallocs", "NextGC", "NumForcedGC",
	"NumGC", "OtherSys", "PauseTotalNs", "StackInuse", "StackSys",
	"Sys", "TotalAlloc", "RandomValue",
}

type nullAuditor struct{}

func (nullAuditor) Record([]string, string) {}

func (nullAuditor) Enabled() bool { return false }

type nullResponseWriter struct {
	headers http.Header
	status  int
	written int
}

func newNullResponseWriter() *nullResponseWriter {
	return &nullResponseWriter{headers: make(http.Header, 8)}
}

func (w *nullResponseWriter) Header() http.Header { return w.headers }

func (w *nullResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *nullResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.written += len(p)
	return len(p), nil
}

func (w *nullResponseWriter) reset() {
	for name := range w.headers {
		delete(w.headers, name)
	}
	w.status = 0
	w.written = 0
}

type load struct {
	handler    http.Handler
	acceptGzip bool
	paths      []string
	updates    [][]byte
	updateSigs []string
	values     [][]byte
	valueSigs  []string
	batchGzip  []byte
	batchSig   string
	sink       *nullResponseWriter
}

func newLoad(key string, acceptGzip bool) *load {
	store := storage.NewMemStorage()
	preset := make([]*model.Metric, 0, len(gaugeNames)+1)
	for i, name := range gaugeNames {
		preset = append(preset, model.NewGaugeMetric(name, float64(i)+0.5))
	}
	preset = append(preset, model.NewCountMetric(pollCountName, int64(len(gaugeNames))))
	store.LoadAll(preset)

	l := &load{
		handler:    server.NewRouter(store, nullAuditor{}, nil, key),
		acceptGzip: acceptGzip,
		paths:      make([]string, len(gaugeNames)),
		updates:    make([][]byte, len(gaugeNames)),
		updateSigs: make([]string, len(gaugeNames)),
		values:     make([][]byte, len(gaugeNames)),
		valueSigs:  make([]string, len(gaugeNames)),
		sink:       newNullResponseWriter(),
	}

	batch := make([]model.Metrics, 0, len(gaugeNames)+1)
	for i, name := range gaugeNames {
		value := float64(i) + 0.25

		l.paths[i] = fmt.Sprintf("/update/gauge/%s/%g", name, value)
		l.updates[i] = mustJSON(model.NewGaugeDTO(name, value))
		l.values[i] = mustJSON(model.Metrics{ID: name, MType: "gauge"})

		batch = append(batch, model.NewGaugeDTO(name, value))
	}
	batch = append(batch, model.NewCounterDTO(pollCountName, 1))

	batchRaw := mustJSON(batch)
	l.batchGzip = mustGzip(batchRaw)

	if key != "" {
		for i := range gaugeNames {
			l.updateSigs[i] = hash.Sum(key, l.updates[i])
			l.valueSigs[i] = hash.Sum(key, l.values[i])
		}
		l.batchSig = hash.Sum(key, batchRaw)
	}

	return l
}

func (l *load) do(b *testing.B, i int) {
	idx := i % len(gaugeNames)

	var req *http.Request
	switch i % kindCount {
	case kindUpdateText:
		req = httptest.NewRequest(http.MethodPost, l.paths[idx], nil)
	case kindUpdateJSON:
		req = jsonRequest("/update", l.updates[idx], l.updateSigs[idx], false)
	case kindValueJSON:
		req = jsonRequest("/value", l.values[idx], l.valueSigs[idx], false)
	case kindUpdatesBatch:
		req = jsonRequest("/updates/", l.batchGzip, l.batchSig, true)
	default:
		req = httptest.NewRequest(http.MethodGet, "/", nil)
	}

	if l.acceptGzip {
		req.Header.Set("Accept-Encoding", "gzip")
	}

	l.sink.reset()
	l.handler.ServeHTTP(l.sink, req)

	if l.sink.status != http.StatusOK {
		b.Fatalf("iteration %d: %s %s answered %d", i, req.Method, req.URL.Path, l.sink.status)
	}
}

func jsonRequest(target string, body []byte, sig string, gzipped bool) *http.Request {
	req := httptest.NewRequest(http.MethodPost, target, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if gzipped {
		req.Header.Set("Content-Encoding", "gzip")
	}
	if sig != "" {
		req.Header.Set(hash.Header, sig)
	}
	return req
}

func mustJSON(v any) []byte {
	out, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return out
}

func mustGzip(plain []byte) []byte {
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

func BenchmarkRouterMixed(b *testing.B) {
	l := newLoad(benchKey, true)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.do(b, i)
	}
}

func BenchmarkRouterMixedPlain(b *testing.B) {
	l := newLoad("", false)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.do(b, i)
	}
}

func BenchmarkRouterUpdateText(b *testing.B) { benchmarkKind(b, kindUpdateText) }

func BenchmarkRouterUpdateJSON(b *testing.B) { benchmarkKind(b, kindUpdateJSON) }

func BenchmarkRouterValueJSON(b *testing.B) { benchmarkKind(b, kindValueJSON) }

func BenchmarkRouterUpdatesBatch(b *testing.B) { benchmarkKind(b, kindUpdatesBatch) }

func BenchmarkRouterList(b *testing.B) { benchmarkKind(b, kindList) }

func benchmarkKind(b *testing.B, kind int) {
	l := newLoad(benchKey, true)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.do(b, i*kindCount+kind)
	}
}
