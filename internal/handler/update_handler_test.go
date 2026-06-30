package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// resetStorage очищает хранилище метрик для изоляции тестов
func resetStorage() {
	mu.Lock()
	defer mu.Unlock()
	storage = make(map[string]map[string]float64)
}

// withChiParams добавляет URL-параметры chi в контекст запроса
func withChiParams(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestUpdateHandler_Success(t *testing.T) {
	resetStorage()

	req, _ := http.NewRequest(http.MethodPost, "/update/gauge/cpu/3.14", nil)
	req = withChiParams(req, map[string]string{
		"metricType":  "gauge",
		"metricName":  "cpu",
		"metricValue": "3.14",
	})
	rec := httptest.NewRecorder()

	UpdateHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	mu.RLock()
	val := storage["gauge"]["cpu"]
	mu.RUnlock()
	if val != 3.14 {
		t.Errorf("expected metric value 3.14, got %v", val)
	}
}

func TestUpdateHandler_InvalidValue(t *testing.T) {
	resetStorage()

	req, _ := http.NewRequest(http.MethodPost, "/update/gauge/cpu/abc", nil)
	req = withChiParams(req, map[string]string{
		"metricType":  "gauge",
		"metricName":  "cpu",
		"metricValue": "abc",
	})
	rec := httptest.NewRecorder()

	UpdateHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestValueHandler_ExistingMetric(t *testing.T) {
	resetStorage()

	// предварительно записываем метрику
	mu.Lock()
	storage["gauge"] = map[string]float64{"memory": 1024.5}
	mu.Unlock()

	req, _ := http.NewRequest(http.MethodGet, "/value/gauge/memory", nil)
	req = withChiParams(req, map[string]string{
		"metricType": "gauge",
		"metricName": "memory",
	})
	rec := httptest.NewRecorder()

	ValueHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/plain" {
		t.Errorf("expected Content-Type text/plain, got %q", contentType)
	}

	body := rec.Body.String()
	if body != "1024.5" {
		t.Errorf("expected body '1024.5', got %q", body)
	}
}

func TestValueHandler_NonExistentType(t *testing.T) {
	resetStorage()

	req, _ := http.NewRequest(http.MethodGet, "/value/gauge/missing", nil)
	req = withChiParams(req, map[string]string{
		"metricType": "gauge",
		"metricName": "missing",
	})
	rec := httptest.NewRecorder()

	ValueHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestValueHandler_NonExistentName(t *testing.T) {
	resetStorage()

	// тип существует, но метрика с таким именем отсутствует
	mu.Lock()
	storage["counter"] = map[string]float64{"hits": 42}
	mu.Unlock()

	req, _ := http.NewRequest(http.MethodGet, "/value/counter/views", nil)
	req = withChiParams(req, map[string]string{
		"metricType": "counter",
		"metricName": "views",
	})
	rec := httptest.NewRecorder()

	ValueHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestIndexHandler_Empty(t *testing.T) {
	resetStorage()

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	IndexHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "<th>Name</th>") {
		t.Error("HTML table header not found")
	}
	// проверяем, что строк данных нет (только заголовок)
	if strings.Count(body, "<tr>") != 1 {
		t.Error("expected only one table row (header), found more")
	}
}

func TestIndexHandler_WithMetrics(t *testing.T) {
	resetStorage()

	mu.Lock()
	storage["gauge"] = map[string]float64{"cpu": 12.3}
	storage["counter"] = map[string]float64{"requests": 100}
	mu.Unlock()

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	IndexHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	// проверяем наличие данных в таблице
	if !strings.Contains(body, "gauge") || !strings.Contains(body, "cpu") || !strings.Contains(body, "12.3") {
		t.Error("HTML does not contain expected gauge/cpu/12.3")
	}
	if !strings.Contains(body, "counter") || !strings.Contains(body, "requests") || !strings.Contains(body, "100") {
		t.Error("HTML does not contain expected counter/requests/100")
	}
}
