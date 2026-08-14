// agent.go
package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"practice/internal/logger"
	models "practice/internal/model"
	"practice/internal/retry"
	"runtime"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Agent отвечает за сбор метрик и их отправку на сервер.
type Agent struct {
	pollInterval   time.Duration
	reportInterval time.Duration
	serverURL      string
	client         *http.Client
	retryDelays    []time.Duration
	mu             sync.Mutex
	gauges         map[string]float64
	pollCount      int64
	randomValue    float64
	wg             sync.WaitGroup
}

// NewAgent создает новый экземпляр агента.
func NewAgent(pollInterval, reportInterval time.Duration, serverURL string) *Agent {
	return &Agent{
		pollInterval:   pollInterval,
		reportInterval: reportInterval,
		serverURL:      serverURL,
		client:         &http.Client{Timeout: 5 * time.Second},
		retryDelays:    retry.DefaultDelays,
		gauges:         make(map[string]float64),
	}
}

// CollectMetrics обновляет все метрики (gauge и counter) из пакета runtime,
// а также пользовательские PollCount и RandomValue.
func (a *Agent) CollectMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	a.mu.Lock()
	defer a.mu.Unlock()

	// Метрики типа gauge из runtime.
	a.gauges["Alloc"] = float64(m.Alloc)
	a.gauges["BuckHashSys"] = float64(m.BuckHashSys)
	a.gauges["Frees"] = float64(m.Frees)
	a.gauges["GCCPUFraction"] = m.GCCPUFraction
	a.gauges["GCSys"] = float64(m.GCSys)
	a.gauges["HeapAlloc"] = float64(m.HeapAlloc)
	a.gauges["HeapIdle"] = float64(m.HeapIdle)
	a.gauges["HeapInuse"] = float64(m.HeapInuse)
	a.gauges["HeapObjects"] = float64(m.HeapObjects)
	a.gauges["HeapReleased"] = float64(m.HeapReleased)
	a.gauges["HeapSys"] = float64(m.HeapSys)
	a.gauges["LastGC"] = float64(m.LastGC)
	a.gauges["Lookups"] = float64(m.Lookups)
	a.gauges["MCacheInuse"] = float64(m.MCacheInuse)
	a.gauges["MCacheSys"] = float64(m.MCacheSys)
	a.gauges["MSpanInuse"] = float64(m.MSpanInuse)
	a.gauges["MSpanSys"] = float64(m.MSpanSys)
	a.gauges["Mallocs"] = float64(m.Mallocs)
	a.gauges["NextGC"] = float64(m.NextGC)
	a.gauges["NumForcedGC"] = float64(m.NumForcedGC)
	a.gauges["NumGC"] = float64(m.NumGC)
	a.gauges["OtherSys"] = float64(m.OtherSys)
	a.gauges["PauseTotalNs"] = float64(m.PauseTotalNs)
	a.gauges["StackInuse"] = float64(m.StackInuse)
	a.gauges["StackSys"] = float64(m.StackSys)
	a.gauges["Sys"] = float64(m.Sys)
	a.gauges["TotalAlloc"] = float64(m.TotalAlloc)

	// Пользовательские метрики.
	a.pollCount++
	a.randomValue = rand.Float64()
	a.gauges["RandomValue"] = a.randomValue
}

// report отправляет все собранные метрики на сервер.
func (a *Agent) report(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}

	batch := a.snapshot()
	if len(batch) == 0 {
		return
	}

	a.sendBatch(ctx, batch)
}

// snapshot копирует данные, чтобы не держать лок при HTTP-запросах.
func (a *Agent) snapshot() []models.Metrics {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.gauges) == 0 && a.pollCount == 0 {
		return nil
	}

	batch := make([]models.Metrics, 0, len(a.gauges)+1)
	for name, val := range a.gauges {
		batch = append(batch, models.NewGaugeDTO(name, val))
	}
	batch = append(batch, models.NewCounterDTO("PollCount", a.pollCount))

	return batch
}

// gzipJSON сжимает тело в gzip. Сжимаем в буфер целиком (тело одной метрики
// крохотное), а не потоком: так проще выставить корректный Content-Length
// и переиспользовать *bytes.Reader при возможных ретраях.
func gzipJSON(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		zw.Close()
		return nil, err
	}
	// Close обязателен ДО чтения buf: он дописывает контрольную сумму и хвост,
	// без него gzip.NewReader на сервере вернёт unexpected EOF.
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (a *Agent) sendBatch(ctx context.Context, batch []models.Metrics) {
	// Слэш на конце — форма, которую использует сервер и автотесты; сервер
	// принимает обе.
	url := a.serverURL + "/updates/"

	body, err := json.Marshal(batch)
	if err != nil {
		logger.Log.Info("marshal batch failed",
			zap.Int("metrics", len(batch)), zap.Error(err))
		return
	}

	body, err = gzipJSON(body)
	if err != nil {
		logger.Log.Info("gzip batch failed",
			zap.Int("metrics", len(batch)), zap.Error(err))
		return
	}

	err = retry.Do(ctx, "agent.sendBatch", a.retryDelays, isRetriable,
		func(ctx context.Context) error {
			return a.postBatch(ctx, url, body)
		})
	if err != nil {
		logger.Log.Info("send batch failed",
			zap.String("url", url), zap.Error(err))
	}
}

func (a *Agent) postBatch(ctx context.Context, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	// Content-Type описывает распакованное тело, Content-Encoding — как оно
	// упаковано на проводе. Оба заголовка обязательны.
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &statusError{code: resp.StatusCode}
	}

	return nil
}

// Start запускает периодический сбор и отправку метрик.
// Работает до тех пор, пока ctx не будет отменён.
func (a *Agent) Start(ctx context.Context) {
	// Немедленный первый сбор метрик.
	a.CollectMetrics()

	pollTicker := time.NewTicker(a.pollInterval)
	reportTicker := time.NewTicker(a.reportInterval)

	a.wg.Add(2)

	go func() {
		defer a.wg.Done()
		defer pollTicker.Stop()
		for {
			select {
			case <-pollTicker.C:
				a.CollectMetrics()
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		defer a.wg.Done()
		defer reportTicker.Stop()
		for {
			select {
			case <-reportTicker.C:
				a.report(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// GetMetrics возвращает копии текущих значений метрик (для тестирования).
func (a *Agent) GetMetrics() (gauges map[string]float64, pollCount int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	gauges = make(map[string]float64, len(a.gauges))
	for k, v := range a.gauges {
		gauges[k] = v
	}
	pollCount = a.pollCount
	return
}

func (a *Agent) Wait() {
	a.wg.Wait()
}
