package agent

import (
	"compress/gzip"
	"encoding/json"
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

func TestReport(t *testing.T) {
	// Хендлер httptest.Server выполняется в отдельной горутине; общие
	// переменные защищаем мьютексом, чтобы тест был чист под go test -race.
	var mu sync.Mutex
	var requests int
	var received []models.Metrics

	// Тестовый HTTP-сервер, имитирующий сервер сбора метрик (JSON API).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()

		if r.Method != http.MethodPost || r.URL.Path != "/updates/" {
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

		var batch []models.Metrics
		if err := json.NewDecoder(body).Decode(&batch); err != nil {
			t.Errorf("decode batch: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		received = append(received, batch...)
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	agent := NewAgent(time.Second, time.Second, srv.URL)
	agent.CollectMetrics()
	agent.report()

	mu.Lock()
	gotRequests := requests
	got := make([]models.Metrics, len(received))
	copy(got, received)
	mu.Unlock()

	if gotRequests != 1 {
		t.Fatalf("requests = %d, want exactly 1 batch request", gotRequests)
	}
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

func TestReportSkipsEmptyBatch(t *testing.T) {
	var mu sync.Mutex
	var requests int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	agent := NewAgent(time.Second, time.Second, srv.URL)
	agent.report()

	mu.Lock()
	defer mu.Unlock()
	if requests != 0 {
		t.Fatalf("requests = %d, want 0 when there is nothing collected", requests)
	}
}

func TestReportConcurrentWithCollect(t *testing.T) {
	var mu sync.Mutex
	var batches int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Errorf("body is not gzip: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer body.Close()

		var batch []models.Metrics
		if err := json.NewDecoder(body).Decode(&batch); err != nil {
			t.Errorf("decode batch: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Снимок обязан быть согласованным: у каждой метрики заполнено ровно
		// одно из delta/value, даже когда сбор идёт параллельно с отправкой.
		for _, m := range batch {
			switch m.MType {
			case "gauge":
				if m.Value == nil || m.Delta != nil {
					t.Errorf("inconsistent gauge in batch: %+v", m)
				}
			case "counter":
				if m.Delta == nil || m.Value != nil {
					t.Errorf("inconsistent counter in batch: %+v", m)
				}
			default:
				t.Errorf("unexpected metric type %q", m.MType)
			}
		}

		mu.Lock()
		batches++
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	agent := NewAgent(time.Second, time.Second, srv.URL)
	agent.CollectMetrics()

	const rounds = 50

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			agent.CollectMetrics()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			agent.report()
		}
	}()
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if batches != rounds {
		t.Errorf("batches = %d, want %d", batches, rounds)
	}
}
