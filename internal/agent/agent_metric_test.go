package agent

import (
	"context"
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
	var mu sync.Mutex
	var received []models.Metrics

	// Тестовый HTTP-сервер, имитирующий сервер сбора метрик (JSON API).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/update/" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var m models.Metrics
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
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

	// Агент с маленькими интервалами для быстрого теста.
	agent := NewAgent(10*time.Millisecond, 20*time.Millisecond, srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	agent.Start(ctx)

	// Даём отработать нескольким циклам отправки.
	time.Sleep(80 * time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond) // ожидаем завершения горутин

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
