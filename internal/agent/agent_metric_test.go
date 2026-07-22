package agent

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	models "practice/internal/model"
	"sync"
	"testing"
	"time"
)

func TestCollectMetrics(t *testing.T) {
	agent := NewAgent(time.Second, time.Second, "http://localhost")
	agent.CollectMetrics()

	gauges, pollCount := agent.GetMetrics()
	if pollCount != 1 {
		t.Errorf("pollCount = %d, want 1", pollCount)
	}
	if _, ok := gauges["Alloc"]; !ok {
		t.Error("Alloc metric missing")
	}
	if _, ok := gauges["RandomValue"]; !ok {
		t.Error("RandomValue metric missing")
	}
	if v := gauges["RandomValue"]; v < 0 || v >= 1 {
		t.Errorf("RandomValue out of range: %f", v)
	}

	// Проверяем наличие всех обязательных gauge‑метрик.
	requiredGauges := []string{
		"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys",
		"HeapAlloc", "HeapIdle", "HeapInuse", "HeapObjects", "HeapReleased",
		"HeapSys", "LastGC", "Lookups", "MCacheInuse", "MCacheSys",
		"MSpanInuse", "MSpanSys", "Mallocs", "NextGC", "NumForcedGC",
		"NumGC", "OtherSys", "PauseTotalNs", "StackInuse", "StackSys",
		"Sys", "TotalAlloc", "RandomValue",
	}
	for _, name := range requiredGauges {
		if _, ok := gauges[name]; !ok {
			t.Errorf("missing gauge metric: %s", name)
		}
	}
}

// TestSendMetricGzipsBody проверяет, что одиночная отправка уходит с
// Content-Encoding: gzip и корректно распаковывается на стороне сервера.
func TestSendMetricGzipsBody(t *testing.T) {
	// Хендлер httptest.Server выполняется в отдельной горутине; общие
	// переменные защищаем мьютексом, чтобы тест был чист под go test -race.
	var mu sync.Mutex
	var got models.Metrics
	var gotEncoding string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enc := r.Header.Get("Content-Encoding")

		zr, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Errorf("body is not gzip: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		raw, _ := io.ReadAll(zr)
		var m models.Metrics
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Errorf("decode: %v", err)
		}
		mu.Lock()
		gotEncoding = enc
		got = m
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(raw)
	}))
	defer srv.Close()

	// sendMetric синхронен: после его возврата хендлер уже отработал.
	a := NewAgent(time.Second, time.Second, srv.URL)
	a.sendMetric(models.NewGaugeDTO("Alloc", 123.5))

	mu.Lock()
	defer mu.Unlock()
	if gotEncoding != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", gotEncoding)
	}
	if got.ID != "Alloc" || got.MType != "gauge" {
		t.Fatalf("decoded metric = %+v, want Alloc/gauge", got)
	}
	if got.Value == nil || *got.Value != 123.5 {
		t.Fatalf("decoded value = %v, want 123.5", got.Value)
	}
}

func TestReport(t *testing.T) {
	var mu sync.Mutex
	var received []models.Metrics

	// Тестовый HTTP-сервер, имитирующий сервер сбора метрик (JSON API).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/update/" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// Агент обязан слать тело сжатым.
		if enc := r.Header.Get("Content-Encoding"); enc != "gzip" {
			t.Errorf("Content-Encoding = %q, want gzip", enc)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		body, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Errorf("body is not gzip: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer body.Close()

		var m models.Metrics
		if err := json.NewDecoder(body).Decode(&m); err != nil {
			t.Errorf("decode metric: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		received = append(received, m)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(m)
	}))
	defer srv.Close()

	agent := NewAgent(time.Second, time.Second, srv.URL)
	agent.CollectMetrics()
	agent.report()

	mu.Lock()
	got := make([]models.Metrics, len(received))
	copy(got, received)
	mu.Unlock()

	if len(got) == 0 {
		t.Fatal("no metrics were reported")
	}

	// Ровно одно из delta/value должно быть заполнено — ради этого DTO
	// использует указатели.
	hasGauge := map[string]bool{}
	hasCounter := false
	for _, m := range got {
		switch m.MType {
		case "gauge":
			if m.Value == nil {
				t.Errorf("gauge %s reported without value", m.ID)
				continue
			}
			if m.Delta != nil {
				t.Errorf("gauge %s reported with delta", m.ID)
			}
			hasGauge[m.ID] = true
		case "counter":
			if m.ID != "PollCount" {
				continue
			}
			if m.Delta == nil {
				t.Error("PollCount reported without delta")
				continue
			}
			if *m.Delta < 1 {
				t.Errorf("PollCount delta = %d, want >= 1", *m.Delta)
			}
			hasCounter = true
		default:
			t.Errorf("unexpected metric type %q", m.MType)
		}
	}

	requiredGauges := []string{
		"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys",
		"HeapAlloc", "HeapIdle", "HeapInuse", "HeapObjects", "HeapReleased",
		"HeapSys", "LastGC", "Lookups", "MCacheInuse", "MCacheSys",
		"MSpanInuse", "MSpanSys", "Mallocs", "NextGC", "NumForcedGC",
		"NumGC", "OtherSys", "PauseTotalNs", "StackInuse", "StackSys",
		"Sys", "TotalAlloc", "RandomValue",
	}
	for _, name := range requiredGauges {
		if !hasGauge[name] {
			t.Errorf("expected gauge %s to be reported", name)
		}
	}
	if !hasCounter {
		t.Error("expected PollCount counter to be reported")
	}
}
