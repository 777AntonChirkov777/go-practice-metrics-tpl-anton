package handlers

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"practice/internal/compress"
	model "practice/internal/model"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createBatchRouter(store *MockStorage) *chi.Mux {
	r := chi.NewRouter()
	h := NewHandler(store)
	r.Post("/updates", h.UpdatesJSONHandler)
	r.Post("/updates/", h.UpdatesJSONHandler)
	r.Post("/update/", h.UpdateJSONHandler)
	r.Post("/update/{type}/{name}/{value}", h.UpdateHandler)
	return r
}

func postBatch(t *testing.T, r *chi.Mux, path string, in []model.Metrics) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(in)
	require.NoError(t, err)

	return postRaw(t, r, path, body)
}

func postRaw(t *testing.T, r *chi.Mux, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func storedCount(t *testing.T, store *MockStorage) int {
	t.Helper()

	all, err := store.GetAll(context.Background())
	require.NoError(t, err)
	return len(all)
}

func TestUpdatesBatch_MixedTypes(t *testing.T) {
	store := NewMockStorage()
	r := createBatchRouter(store)

	w := postBatch(t, r, "/updates/", []model.Metrics{
		model.NewGaugeDTO("Alloc", 123.45),
		model.NewCounterDTO("PollCount", 7),
	})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.Bytes())

	gauge, err := store.Get(context.Background(), model.Gauge, "Alloc")
	require.NoError(t, err)
	assert.Equal(t, 123.45, gauge.Value)

	counter, err := store.Get(context.Background(), model.Counter, "PollCount")
	require.NoError(t, err)
	assert.Equal(t, int64(7), counter.Delta)
}

func TestUpdatesBatch_PathWithoutTrailingSlash(t *testing.T) {
	store := NewMockStorage()
	r := createBatchRouter(store)

	w := postBatch(t, r, "/updates", []model.Metrics{model.NewGaugeDTO("Alloc", 1)})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, storedCount(t, store))
}

func TestUpdatesBatch_CounterAccumulatesBetweenBatches(t *testing.T) {
	store := NewMockStorage()
	r := createBatchRouter(store)

	batch := []model.Metrics{model.NewCounterDTO("PollCount", 10)}
	postBatch(t, r, "/updates/", batch)
	w := postBatch(t, r, "/updates/", batch)

	assert.Equal(t, http.StatusOK, w.Code)

	counter, err := store.Get(context.Background(), model.Counter, "PollCount")
	require.NoError(t, err)
	assert.Equal(t, int64(20), counter.Delta)
}

func TestUpdatesBatch_DuplicateNamesInsideBatch(t *testing.T) {
	store := NewMockStorage()
	r := createBatchRouter(store)

	w := postBatch(t, r, "/updates/", []model.Metrics{
		model.NewCounterDTO("PollCount", 1),
		model.NewCounterDTO("PollCount", 2),
		model.NewGaugeDTO("Alloc", 1.5),
		model.NewGaugeDTO("Alloc", 2.5),
	})

	assert.Equal(t, http.StatusOK, w.Code)

	counter, err := store.Get(context.Background(), model.Counter, "PollCount")
	require.NoError(t, err)
	assert.Equal(t, int64(3), counter.Delta)

	gauge, err := store.Get(context.Background(), model.Gauge, "Alloc")
	require.NoError(t, err)
	assert.Equal(t, 2.5, gauge.Value)
}

func TestUpdatesBatch_UnknownTypeRejectsWholeBatch(t *testing.T) {
	store := NewMockStorage()
	r := createBatchRouter(store)

	w := postRaw(t, r, "/updates/", []byte(
		`[{"id":"Alloc","type":"gauge","value":1.5},{"id":"Latency","type":"histogram"}]`))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, 0, storedCount(t, store), "валидный элемент не должен сохраниться при браке в батче")
}

func TestUpdatesBatch_MissingValueRejectsWholeBatch(t *testing.T) {
	store := NewMockStorage()
	r := createBatchRouter(store)

	w := postRaw(t, r, "/updates/", []byte(
		`[{"id":"PollCount","type":"counter","delta":1},{"id":"Alloc","type":"gauge"}]`))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, 0, storedCount(t, store))
}

func TestUpdatesBatch_MissingDeltaRejectsWholeBatch(t *testing.T) {
	store := NewMockStorage()
	r := createBatchRouter(store)

	w := postRaw(t, r, "/updates/", []byte(`[{"id":"PollCount","type":"counter"}]`))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, 0, storedCount(t, store))
}

func TestUpdatesBatch_BlankIDRejectsWholeBatch(t *testing.T) {
	store := NewMockStorage()
	r := createBatchRouter(store)

	w := postRaw(t, r, "/updates/", []byte(
		`[{"id":"Alloc","type":"gauge","value":1.5},{"id":"   ","type":"gauge","value":2}]`))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, 0, storedCount(t, store))
}

func TestUpdatesBatch_MalformedBodies(t *testing.T) {
	cases := map[string]string{
		"битый json":       `[{"id":"Alloc",`,
		"не массив":        `{"id":"Alloc","type":"gauge","value":1.5}`,
		"мусор в хвосте":   `[{"id":"Alloc","type":"gauge","value":1.5}]{"junk":1}`,
		"пустое тело":      ``,
		"массив не метрик": `[1,2,3]`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			store := NewMockStorage()
			r := createBatchRouter(store)

			w := postRaw(t, r, "/updates/", []byte(body))

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Equal(t, 0, storedCount(t, store))
		})
	}
}

func TestUpdatesBatch_EmptyArrayIsNoop(t *testing.T) {
	store := NewMockStorage()
	r := createBatchRouter(store)

	w := postRaw(t, r, "/updates/", []byte(`[]`))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0, storedCount(t, store))
}

func TestUpdatesBatch_StorageErrorIs500(t *testing.T) {
	store := NewMockStorage()
	store.batchErr = errors.New("connection refused")
	r := createBatchRouter(store)

	w := postBatch(t, r, "/updates/", []model.Metrics{model.NewGaugeDTO("Alloc", 1.5)})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestUpdatesBatch_AcceptsGzippedBody(t *testing.T) {
	store := NewMockStorage()
	r := createBatchRouter(store)

	raw, err := json.Marshal([]model.Metrics{
		model.NewGaugeDTO("Alloc", 123.45),
		model.NewCounterDTO("PollCount", 7),
	})
	require.NoError(t, err)

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err = zw.Write(raw)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	w := httptest.NewRecorder()

	compress.Middleware(r).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 2, storedCount(t, store))
}

func TestUpdatesBatch_BrokenGzipBodyIs400(t *testing.T) {
	store := NewMockStorage()
	r := createBatchRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader([]byte("not gzip at all")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	w := httptest.NewRecorder()

	compress.Middleware(r).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, 0, storedCount(t, store))
}

func TestUpdatesBatch_DoesNotShadowTextUpdateRoute(t *testing.T) {
	store := NewMockStorage()
	r := createBatchRouter(store)

	req := httptest.NewRequest(http.MethodPost, "/update/counter/PollCount/5", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	counter, err := store.Get(context.Background(), model.Counter, "PollCount")
	require.NoError(t, err)
	assert.Equal(t, int64(5), counter.Delta)
}
