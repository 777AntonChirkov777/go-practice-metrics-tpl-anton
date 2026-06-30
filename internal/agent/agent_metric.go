// agent.go
package agent

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"time"
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
		url := fmt.Sprintf("%s/update/gauge/%s/%s",
			a.serverURL, name, strconv.FormatFloat(val, 'g', -1, 64))
		a.sendMetric(url)
	}

	counterURL := fmt.Sprintf("%s/update/counter/PollCount/%d",
		a.serverURL, pollCount)
	a.sendMetric(counterURL)
}

// sendMetric выполняет POST-запрос к серверу с заголовком Content-Type: text/plain.
func (a *Agent) sendMetric(url string) {
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		// В реальном приложении здесь должно быть логирование.
		return
	}
	req.Header.Set("Content-Type", "text/plain")
	resp, err := a.client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// Start запускает периодический сбор и отправку метрик.
// Работает до тех пор, пока ctx не будет отменён.
func (a *Agent) Start(ctx context.Context) {
	// Немедленный первый сбор метрик.
	a.CollectMetrics()

	pollTicker := time.NewTicker(a.pollInterval)
	reportTicker := time.NewTicker(a.reportInterval)

	go func() {
		for {
			select {
			case <-pollTicker.C:
				a.CollectMetrics()
			case <-ctx.Done():
				pollTicker.Stop()
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-reportTicker.C:
				a.report()
			case <-ctx.Done():
				reportTicker.Stop()
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
