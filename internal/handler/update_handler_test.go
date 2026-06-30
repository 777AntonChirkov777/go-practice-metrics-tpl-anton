package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	handlers "practice/internal/handler"
	models "practice/internal/model"
	repository "practice/internal/repository"
)

// setupTempDir создаёт временную директорию, переходит в неё и возвращает
// функцию для восстановления исходной рабочей директории.
func setupTempDir(t *testing.T) func() {
	t.Helper()
	tmpDir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	return func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("failed to restore working dir: %v", err)
		}
	}
}

// executeRequest выполняет запрос к UpdateHandler и возвращает рекордер.
func executeRequest(method, target, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	handlers.UpdateHandler(rec, req)
	return rec
}

// ---------- Ошибки HTTP ----------

func TestUpdateHandler_MethodNotAllowed(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	rec := executeRequest(http.MethodGet, "/update/gauge/cpu/1.0", "text/plain")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
	expectedBody := "Only POST requests are allowed!\n"
	if rec.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, rec.Body.String())
	}
}

func TestUpdateHandler_WrongContentType(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	rec := executeRequest(http.MethodPost, "/update/gauge/cpu/1.0", "application/json")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
	expectedBody := "Only contentType text/plain\n"
	if rec.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, rec.Body.String())
	}
}

// ---------- Ошибки URL ----------

func TestUpdateHandler_EmptyPath(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	// Пустой путь после /update/
	rec := executeRequest(http.MethodPost, "/update/", "text/plain")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateHandler_NoMeaningPart(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	rec := executeRequest(http.MethodPost, "/update/gauge/cpu", "text/plain")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateHandler_MeaningEmptyAfterTrim(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	// Значение – три пробела, закодированные как %20
	rec := executeRequest(http.MethodPost, "/update/gauge/cpu/%20%20%20", "text/plain")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateHandler_InvalidMetricType(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	rec := executeRequest(http.MethodPost, "/update/histogram/cpu/1", "text/plain")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateHandler_InvalidMetricValue(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	// Gauge с нечисловым значением
	rec := executeRequest(http.MethodPost, "/update/gauge/cpu/abc", "text/plain")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// ---------- Успешные сценарии ----------

func TestUpdateHandler_SuccessGauge(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	rec := executeRequest(http.MethodPost, "/update/gauge/temperature/36.6", "text/plain")
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	// Дополнительно проверим, что метрика сохранилась
	// (опционально, для уверенности)
	m, err := repository.FindMetric("temperature", int8(models.Gauge))
	if err != nil {
		t.Fatalf("unexpected error finding metric: %v", err)
	}
	if m == nil || m.Value != 36.6 {
		t.Errorf("metric not saved correctly, got %+v", m)
	}
}

func TestUpdateHandler_SuccessCounter(t *testing.T) {
	restore := setupTempDir(t)
	defer restore()

	rec := executeRequest(http.MethodPost, "/update/counter/requests/10", "text/plain")
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	m, err := repository.FindMetric("requests", int8(models.Counter))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil || m.Delta != 10 {
		t.Errorf("expected delta 10, got %+v", m)
	}
}
