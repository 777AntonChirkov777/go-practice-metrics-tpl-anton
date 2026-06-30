package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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
	var receivedPaths []string

	// Тестовый HTTP-сервер, имитирующий сервер сбора метрик.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if ct := r.Header.Get("Content-Type"); ct != "text/plain" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		receivedPaths = append(receivedPaths, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
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
	paths := make([]string, len(receivedPaths))
	copy(paths, receivedPaths)
	mu.Unlock()

	if len(paths) == 0 {
		t.Fatal("no metrics were reported")
	}

	// Анализируем полученные пути на наличие всех gauge и counter.
	hasGauge := map[string]bool{}
	hasCounter := false
	for _, p := range paths {
		if strings.HasPrefix(p, "/update/gauge/") {
			parts := strings.SplitN(p, "/", 5)
			if len(parts) == 5 {
				hasGauge[parts[3]] = true
			}
		} else if strings.HasPrefix(p, "/update/counter/") {
			if strings.Contains(p, "PollCount") {
				hasCounter = true
			}
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
