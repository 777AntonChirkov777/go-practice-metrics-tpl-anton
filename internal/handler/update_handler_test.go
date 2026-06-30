package handlers

import (
	"net/http"
	"net/http/httptest"
	model "practice/internal/model"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

// MockStorage — тестовая реализация MetricStorage
type MockStorage struct {
	metrics map[string]*model.Metric
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		metrics: make(map[string]*model.Metric),
	}
}

func (m *MockStorage) Save(metric *model.Metric) error {
	key := metric.ID + "_" + string(rune(metric.MType))

	// Реализуем логику накопления для counter
	if existing, ok := m.metrics[key]; ok {
		if model.MetricType(metric.MType) == model.Counter {
			existing.Delta += metric.Delta
			existing.Hash = metric.Hash
			return nil
		}
	}

	m.metrics[key] = metric
	return nil
}

func (m *MockStorage) Get(mtype model.MetricType, name string) (*model.Metric, bool) {
	key := name + "_" + string(rune(mtype))
	metric, ok := m.metrics[key]
	return metric, ok
}

func (m *MockStorage) GetAll() []*model.Metric {
	result := make([]*model.Metric, 0, len(m.metrics))
	for _, m := range m.metrics {
		result = append(result, m)
	}
	return result
}

// createTestRouter создаёт тестовый роутер с хендлерами
func createTestRouter(store *MockStorage) *chi.Mux {
	r := chi.NewRouter()
	h := NewHandler(store)
	r.Get("/", h.ListHandler)
	r.Post("/update/{type}/{name}/{value}", h.UpdateHandler)
	r.Get("/value/{type}/{name}", h.ValueHandler)
	return r
}

// TestUpdateHandler_Success_Gauge тестирует успешное обновление gauge
func TestUpdateHandler_Success_Gauge(t *testing.T) {
	store := NewMockStorage()
	router := createTestRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/test_metric/123.45", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Проверяем, что метрика сохранилась
	metric, ok := store.Get(model.Gauge, "test_metric")
	assert.True(t, ok)
	assert.Equal(t, "test_metric", metric.ID)
	assert.Equal(t, float64(123.45), metric.Value)
}

// TestUpdateHandler_Success_Counter тестирует успешное обновление counter
func TestUpdateHandler_Success_Counter(t *testing.T) {
	store := NewMockStorage()
	router := createTestRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/update/counter/test_counter/10", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	metric, ok := store.Get(model.Counter, "test_counter")
	assert.True(t, ok)
	assert.Equal(t, "test_counter", metric.ID)
	assert.Equal(t, int64(10), metric.Delta)
}

func TestUpdateHandler_WrongMethod(t *testing.T) {
	store := NewMockStorage()
	router := createTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/update/gauge/test/123", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	// chi сам возвращает 405, тело ответа может быть пустым
}

// TestUpdateHandler_InvalidMetricType тестирует обработку неверного типа метрики
func TestUpdateHandler_InvalidMetricType(t *testing.T) {
	store := NewMockStorage()
	router := createTestRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/update/invalid/test/123", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid metric type")
}

// TestUpdateHandler_InvalidGaugeValue тестирует обработку неверного значения gauge
func TestUpdateHandler_InvalidGaugeValue(t *testing.T) {
	store := NewMockStorage()
	router := createTestRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/test/not_a_number", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid metric value")
}

// TestUpdateHandler_InvalidCounterValue тестирует обработку неверного значения counter
func TestUpdateHandler_InvalidCounterValue(t *testing.T) {
	store := NewMockStorage()
	router := createTestRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/update/counter/test/12.34", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid metric value")
}

// TestValueHandler_Success_Gauge тестирует успешное получение gauge
func TestValueHandler_Success_Gauge(t *testing.T) {
	store := NewMockStorage()

	// Добавляем тестовую метрику
	gauge := model.NewGaugeMetric("test_metric", 123.45)
	_ = gauge.CalculateHash()
	_ = store.Save(gauge)

	router := createTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/value/gauge/test_metric", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "123.45", w.Body.String())
}

// TestValueHandler_Success_Counter тестирует успешное получение counter
func TestValueHandler_Success_Counter(t *testing.T) {
	store := NewMockStorage()

	counter := model.NewCountMetric("test_counter", 42)
	_ = counter.CalculateHash()
	_ = store.Save(counter)

	router := createTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/value/counter/test_counter", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "42", w.Body.String())
}

// TestValueHandler_MetricNotFound тестирует получение несуществующей метрики
func TestValueHandler_MetricNotFound(t *testing.T) {
	store := NewMockStorage()
	router := createTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/value/gauge/nonexistent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "metric not found")
}

// TestValueHandler_InvalidType тестирует обработку неверного типа метрики
func TestValueHandler_InvalidType(t *testing.T) {
	store := NewMockStorage()
	router := createTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/value/invalid/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid metric type")
}

// TestListHandler_Empty тестирует получение пустого списка метрик
func TestListHandler_Empty(t *testing.T) {
	store := NewMockStorage()
	router := createTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "no metrics yet")
}

// TestListHandler_WithMetrics тестирует получение списка с метриками
func TestListHandler_WithMetrics(t *testing.T) {
	store := NewMockStorage()

	gauge := model.NewGaugeMetric("metric1", 100.5)
	_ = gauge.CalculateHash()
	_ = store.Save(gauge)

	counter := model.NewCountMetric("metric2", 50)
	_ = counter.CalculateHash()
	_ = store.Save(counter)

	router := createTestRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))

	body := w.Body.String()
	assert.Contains(t, body, "metric1")
	assert.Contains(t, body, "100.5")
	assert.Contains(t, body, "metric2")
	assert.Contains(t, body, "50")
}

func TestUpdateHandler_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		contentType    string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Valid gauge update",
			method:         http.MethodPost,
			path:           "/update/gauge/cpu_usage/75.5",
			contentType:    "text/plain",
			expectedStatus: http.StatusOK,
			expectedBody:   "",
		},
		{
			name:           "Valid counter update",
			method:         http.MethodPost,
			path:           "/update/counter/requests_total/1",
			contentType:    "text/plain",
			expectedStatus: http.StatusOK,
			expectedBody:   "",
		},
		{
			name:           "Wrong HTTP method",
			method:         http.MethodGet,
			path:           "/update/gauge/test/123",
			contentType:    "text/plain",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "", // chi сам возвращает 405 с пустым телом
		},
		{
			name:           "Invalid metric type",
			method:         http.MethodPost,
			path:           "/update/unknown/test/123",
			contentType:    "text/plain",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid metric type",
		},
		{
			name:           "Invalid gauge value",
			method:         http.MethodPost,
			path:           "/update/gauge/test/abc",
			contentType:    "text/plain",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid metric value",
		},
		{
			name:           "Invalid counter value (float)",
			method:         http.MethodPost,
			path:           "/update/counter/test/12.5",
			contentType:    "text/plain",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid metric value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMockStorage()
			router := createTestRouter(store)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, w.Body.String(), tt.expectedBody)
			}
		})
	}
}

// TestCounterAccumulation тестирует накопление значений counter
func TestCounterAccumulation(t *testing.T) {
	store := NewMockStorage()
	router := createTestRouter(store)

	// Первый запрос: counter = 10
	req1 := httptest.NewRequest(http.MethodPost, "/update/counter/test_counter/10", nil)
	req1.Header.Set("Content-Type", "text/plain")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Второй запрос: counter += 5
	req2 := httptest.NewRequest(http.MethodPost, "/update/counter/test_counter/5", nil)
	req2.Header.Set("Content-Type", "text/plain")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// Проверяем, что значение накопилось
	req3 := httptest.NewRequest(http.MethodGet, "/value/counter/test_counter", nil)
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)

	assert.Equal(t, http.StatusOK, w3.Code)
	assert.Equal(t, "15", w3.Body.String())
}

// TestGaugeOverwrite тестирует перезапись значения gauge
func TestGaugeOverwrite(t *testing.T) {
	store := NewMockStorage()
	router := createTestRouter(store)

	// Первый запрос: gauge = 100
	req1 := httptest.NewRequest(http.MethodPost, "/update/gauge/test_gauge/100", nil)
	req1.Header.Set("Content-Type", "text/plain")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Второй запрос: gauge = 200 (перезапись)
	req2 := httptest.NewRequest(http.MethodPost, "/update/gauge/test_gauge/200", nil)
	req2.Header.Set("Content-Type", "text/plain")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// Проверяем, что значение перезаписалось
	req3 := httptest.NewRequest(http.MethodGet, "/value/gauge/test_gauge", nil)
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)

	assert.Equal(t, http.StatusOK, w3.Code)
	assert.Equal(t, "200", w3.Body.String())
}
