package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	model "practice/internal/model"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createJSONRouter — отдельный от createTestRouter хелпер: текстовые маршруты
// в JSON-тестах не участвуют.
func createJSONRouter(store *MockStorage) *chi.Mux {
	r := chi.NewRouter()
	h := NewHandler(store)
	r.Post("/update", h.UpdateJSONHandler)
	r.Post("/update/", h.UpdateJSONHandler)
	r.Post("/value", h.ValueJSONHandler)
	r.Post("/value/", h.ValueJSONHandler)
	return r
}

func postJSON(t *testing.T, r *chi.Mux, path string, in model.Metrics) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(in)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestUpdateJSON_Gauge(t *testing.T) {
	r := createJSONRouter(NewMockStorage())

	w := postJSON(t, r, "/update/", model.NewGaugeDTO("Alloc", 123.45))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var out model.Metrics
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, "gauge", out.MType)
	require.NotNil(t, out.Value)
	assert.Equal(t, 123.45, *out.Value)
	assert.Nil(t, out.Delta)
}

// Ноль обязан присутствовать в JSON — ради этого DTO использует указатели.
func TestUpdateJSON_ZeroGaugeIsSerialized(t *testing.T) {
	r := createJSONRouter(NewMockStorage())

	w := postJSON(t, r, "/update/", model.NewGaugeDTO("Lookups", 0))

	assert.Equal(t, http.StatusOK, w.Code)

	var generic map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &generic))
	assert.Contains(t, generic, "value", "нулевой value обязан присутствовать в ответе")
	assert.NotContains(t, generic, "delta")
}

// Ответ должен содержать накопленную дельту, а не присланную.
func TestUpdateJSON_CounterAccumulates(t *testing.T) {
	r := createJSONRouter(NewMockStorage())

	postJSON(t, r, "/update/", model.NewCounterDTO("PollCount", 5))
	w := postJSON(t, r, "/update/", model.NewCounterDTO("PollCount", 7))

	assert.Equal(t, http.StatusOK, w.Code)

	var out model.Metrics
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.NotNil(t, out.Delta)
	assert.Equal(t, int64(12), *out.Delta)
	assert.Nil(t, out.Value)
}

func TestValueJSON_RoundTrip(t *testing.T) {
	r := createJSONRouter(NewMockStorage())

	postJSON(t, r, "/update/", model.NewCounterDTO("c", 3))
	postJSON(t, r, "/update/", model.NewCounterDTO("c", 4))

	w := postJSON(t, r, "/value/", model.Metrics{ID: "c", MType: "counter"})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var out model.Metrics
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.NotNil(t, out.Delta)
	assert.Equal(t, int64(7), *out.Delta)
}

// Обе формы пути — со слэшем и без — должны работать одинаково.
func TestJSON_BothPathForms(t *testing.T) {
	r := createJSONRouter(NewMockStorage())

	assert.Equal(t, http.StatusOK, postJSON(t, r, "/update", model.NewGaugeDTO("g", 1)).Code)
	assert.Equal(t, http.StatusOK, postJSON(t, r, "/update/", model.NewGaugeDTO("g", 2)).Code)
	assert.Equal(t, http.StatusOK, postJSON(t, r, "/value", model.Metrics{ID: "g", MType: "gauge"}).Code)
	assert.Equal(t, http.StatusOK, postJSON(t, r, "/value/", model.Metrics{ID: "g", MType: "gauge"}).Code)
}

func TestValueJSON_NotFound(t *testing.T) {
	r := createJSONRouter(NewMockStorage())

	w := postJSON(t, r, "/value/", model.Metrics{ID: "nope", MType: "gauge"})

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestValueJSON_BadType(t *testing.T) {
	r := createJSONRouter(NewMockStorage())

	w := postJSON(t, r, "/value/", model.Metrics{ID: "x", MType: "histogram"})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateJSON_GaugeWithoutValue(t *testing.T) {
	r := createJSONRouter(NewMockStorage())

	w := postJSON(t, r, "/update/", model.Metrics{ID: "x", MType: "gauge"})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateJSON_EmptyID(t *testing.T) {
	r := createJSONRouter(NewMockStorage())

	w := postJSON(t, r, "/update/", model.NewGaugeDTO("  ", 1))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateJSON_MalformedBody(t *testing.T) {
	r := createJSONRouter(NewMockStorage())

	req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader([]byte("{oops")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateJSON_TrailingGarbage(t *testing.T) {
	r := createJSONRouter(NewMockStorage())

	body := []byte(`{"id":"x","type":"gauge","value":1} {"id":"y"}`)
	req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
