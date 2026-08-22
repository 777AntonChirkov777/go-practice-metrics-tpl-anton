package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"practice/internal/hash"
	models "practice/internal/model"
	"sync"
	"testing"
	"time"
)

func TestCollectMetrics(t *testing.T) {
	agent := NewAgent(time.Second, time.Second, "http://localhost", "")
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

	agent := NewAgent(time.Second, time.Second, srv.URL, "")
	agent.CollectMetrics()
	agent.report(context.Background())

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

	agent := NewAgent(time.Second, time.Second, srv.URL, "")
	agent.report(context.Background())

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

	agent := NewAgent(time.Second, time.Second, srv.URL, "")
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
			agent.report(context.Background())
		}
	}()
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if batches != rounds {
		t.Errorf("batches = %d, want %d", batches, rounds)
	}
}

type stubTransport struct {
	failures int
	statuses []int
	onCall   func()

	mu      sync.Mutex
	calls   int
	bodies  [][]byte
	headers []http.Header
}

func (t *stubTransport) statusFor(call int) int {
	if len(t.statuses) == 0 {
		return http.StatusOK
	}
	if call > len(t.statuses) {
		return t.statuses[len(t.statuses)-1]
	}
	return t.statuses[call-1]
}

func (t *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body.Close()

	t.mu.Lock()
	t.calls++
	call := t.calls
	t.bodies = append(t.bodies, body)
	t.headers = append(t.headers, req.Header.Clone())
	t.mu.Unlock()

	if t.onCall != nil {
		t.onCall()
	}

	if call <= t.failures {
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
	}

	status := t.statusFor(call)

	return &http.Response{
		Status:     http.StatusText(status),
		StatusCode: status,
		Proto:      "HTTP/1.1",
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(nil)),
		Request:    req,
	}, nil
}

func TestSendBatchRetriesServerError(t *testing.T) {
	tr := &stubTransport{statuses: []int{
		http.StatusInternalServerError,
		http.StatusInternalServerError,
		http.StatusOK,
	}}

	agent := NewAgent(time.Second, time.Second, "http://metrics.invalid", "")
	agent.client = &http.Client{Transport: tr}
	agent.retryDelays = []time.Duration{0, 0, 0}

	agent.CollectMetrics()
	agent.report(context.Background())

	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.calls != 3 {
		t.Fatalf("attempts = %d, want 3: две пятисотки и успешная третья попытка", tr.calls)
	}
	if !bytes.Equal(tr.bodies[0], tr.bodies[2]) {
		t.Error("повтор обязан отправлять тот же снимок метрик, что и первая попытка")
	}
}

func TestSendBatchDoesNotRetryClientError(t *testing.T) {
	tr := &stubTransport{statuses: []int{http.StatusBadRequest}}

	agent := NewAgent(time.Second, time.Second, "http://metrics.invalid", "")
	agent.client = &http.Client{Transport: tr}
	agent.retryDelays = []time.Duration{0, 0, 0}

	agent.CollectMetrics()
	agent.report(context.Background())

	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.calls != 1 {
		t.Errorf("attempts = %d, want 1: ответ 400 повторять бессмысленно", tr.calls)
	}
}

func TestSendBatchDoesNotRetryUnbuildableRequest(t *testing.T) {
	tr := &stubTransport{}

	agent := NewAgent(time.Second, time.Second, "http://\x7f", "")
	agent.client = &http.Client{Transport: tr}
	agent.retryDelays = []time.Duration{time.Hour, time.Hour, time.Hour}

	agent.CollectMetrics()

	done := make(chan struct{})
	go func() {
		defer close(done)
		agent.report(context.Background())
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("отправка выдерживает паузу: ошибка построения запроса ушла в повтор")
	}

	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.calls != 0 {
		t.Errorf("transport calls = %d, want 0: запрос не был построен", tr.calls)
	}
}

func TestSendBatchRetriesTransportFailure(t *testing.T) {
	tr := &stubTransport{failures: 2}

	agent := NewAgent(time.Second, time.Second, "http://metrics.invalid", "")
	agent.client = &http.Client{Transport: tr}
	agent.retryDelays = []time.Duration{0, 0, 0}

	agent.CollectMetrics()
	agent.report(context.Background())

	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.calls != 3 {
		t.Fatalf("attempts = %d, want 3: две неудачные попытки и успешная третья", tr.calls)
	}
	if !bytes.Equal(tr.bodies[0], tr.bodies[2]) {
		t.Error("повтор обязан отправлять тот же снимок метрик, что и первая попытка")
	}

	zr, err := gzip.NewReader(bytes.NewReader(tr.bodies[2]))
	if err != nil {
		t.Fatalf("тело успешной попытки не является gzip: %v", err)
	}
	defer zr.Close()

	var batch []models.Metrics
	if err := json.NewDecoder(zr).Decode(&batch); err != nil {
		t.Fatalf("decode batch: %v", err)
	}

	var hasAlloc, hasPollCount bool
	for _, m := range batch {
		switch m.ID {
		case "Alloc":
			hasAlloc = m.Value != nil
		case "PollCount":
			hasPollCount = m.Delta != nil
		}
	}
	if !hasAlloc || !hasPollCount {
		t.Errorf("батч успешной попытки неполон: Alloc=%v, PollCount=%v", hasAlloc, hasPollCount)
	}
}

func TestSendBatchStopsWhenContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tr := &stubTransport{failures: 100, onCall: cancel}

	agent := NewAgent(time.Second, time.Second, "http://metrics.invalid", "")
	agent.client = &http.Client{Transport: tr}
	agent.retryDelays = []time.Duration{30 * time.Second, 30 * time.Second, 30 * time.Second}

	agent.CollectMetrics()
	agent.report(ctx)

	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.calls != 1 {
		t.Errorf("attempts = %d, want 1: отмена контекста обрывает серию повторов", tr.calls)
	}
}

func gunzipBody(t *testing.T, body []byte) []byte {
	t.Helper()

	zr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("тело не является gzip: %v", err)
	}
	defer zr.Close()

	plain, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("не удалось распаковать тело: %v", err)
	}

	return plain
}

func TestSendBatchSignsUncompressedBody(t *testing.T) {
	tr := &stubTransport{}

	agent := NewAgent(time.Second, time.Second, "http://metrics.invalid", "secret")
	agent.client = &http.Client{Transport: tr}

	agent.CollectMetrics()
	agent.report(context.Background())

	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.calls != 1 {
		t.Fatalf("calls = %d, want 1", tr.calls)
	}

	want := hash.Sum("secret", gunzipBody(t, tr.bodies[0]))
	if got := tr.headers[0].Get(hash.Header); got != want {
		t.Errorf("%s = %q, want %q: подпись считается от несжатого тела", hash.Header, got, want)
	}
}

func TestSendBatchWithoutKeyIsUnsigned(t *testing.T) {
	tr := &stubTransport{}

	agent := NewAgent(time.Second, time.Second, "http://metrics.invalid", "")
	agent.client = &http.Client{Transport: tr}

	agent.CollectMetrics()
	agent.report(context.Background())

	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.calls != 1 {
		t.Fatalf("calls = %d, want 1", tr.calls)
	}
	if got := tr.headers[0].Get(hash.Header); got != "" {
		t.Errorf("%s = %q, want empty: без ключа подписи нет", hash.Header, got)
	}
}

func TestSendBatchRetryKeepsSignature(t *testing.T) {
	tr := &stubTransport{statuses: []int{
		http.StatusInternalServerError,
		http.StatusOK,
	}}

	agent := NewAgent(time.Second, time.Second, "http://metrics.invalid", "secret")
	agent.client = &http.Client{Transport: tr}
	agent.retryDelays = []time.Duration{0, 0, 0}

	agent.CollectMetrics()
	agent.report(context.Background())

	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.calls != 2 {
		t.Fatalf("calls = %d, want 2", tr.calls)
	}
	if !bytes.Equal(tr.bodies[0], tr.bodies[1]) {
		t.Error("повтор обязан отправлять то же тело")
	}
	if tr.headers[0].Get(hash.Header) != tr.headers[1].Get(hash.Header) {
		t.Error("повтор обязан отправлять ту же подпись, что и первая попытка")
	}
}
