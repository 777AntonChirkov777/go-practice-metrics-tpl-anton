// agent.go
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"practice/internal/logger"
	models "practice/internal/model"
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
func (a *Agent) report() {
	a.mu.Lock()
	// Копируем данные, чтобы не держать лок при HTTP-запросах.
	gaugesCopy := make(map[string]float64, len(a.gauges))
	for k, v := range a.gauges {
		gaugesCopy[k] = v
	}
	pollCount := a.pollCount
	a.mu.Unlock()

	for name, val := range gaugesCopy {
		a.sendMetric(models.NewGaugeDTO(name, val))
	}

	a.sendMetric(models.NewCounterDTO("PollCount", pollCount))
}

// sendMetric отправляет одну метрику на POST /update/ телом-JSON
// с заголовком Content-Type: application/json.
func (a *Agent) sendMetric(m models.Metrics) {
	// Слэш на конце — форма, которую использует сервер и автотесты; сервер
	// принимает обе.
	url := a.serverURL + "/update/"

	body, err := json.Marshal(m)
	if err != nil {
		logger.Log.Info("marshal metric failed",
			zap.String("id", m.ID), zap.Error(err))
		return
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		// zap.Error — это поле; уровень сообщения остаётся Info.
		logger.Log.Info("build metric request failed",
			zap.String("url", url), zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		logger.Log.Info("send metric failed",
			zap.String("url", url), zap.Error(err))
		return
	}
	// Тело обязательно дочитать, а не только закрыть: раньше ответы были
	// пустые, теперь /update/ возвращает JSON, и непрочитанный хвост не даёт
	// вернуть соединение в keep-alive пул — новый сокет на каждую метрику.
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Log.Info("unexpected response status",
			zap.String("id", m.ID), zap.Int("status", resp.StatusCode))
	}
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
				a.report()
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
