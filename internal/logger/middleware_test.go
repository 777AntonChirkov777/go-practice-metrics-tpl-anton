package logger

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// withObservedLog подменяет синглтон на observer и возвращает его вместе
// с функцией восстановления.
func withObservedLog(t *testing.T) *observer.ObservedLogs {
	t.Helper()

	core, logs := observer.New(zapcore.InfoLevel)
	prev := Log
	Log = zap.New(core)
	t.Cleanup(func() { Log = prev })

	return logs
}

func TestRequestLoggerWritesTwoInfoEntries(t *testing.T) {
	logs := withObservedLog(t)

	h := RequestLogger(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
		res.WriteHeader(http.StatusCreated)
		res.Write([]byte("hel"))
		res.Write([]byte("lo"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/x/1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// middleware не должна искажать ответ
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "hello", rec.Body.String())

	entries := logs.All()
	require.Len(t, entries, 2)

	assert.Equal(t, zapcore.InfoLevel, entries[0].Level)
	assert.Equal(t, "request", entries[0].Message)
	reqFields := entries[0].ContextMap()
	assert.Equal(t, "/update/gauge/x/1", reqFields["uri"])
	assert.Equal(t, "POST", reqFields["method"])
	assert.Contains(t, reqFields, "duration")

	assert.Equal(t, zapcore.InfoLevel, entries[1].Level)
	assert.Equal(t, "response", entries[1].Message)
	resFields := entries[1].ContextMap()
	assert.EqualValues(t, http.StatusCreated, resFields["status"])
	// размер суммируется по обоим вызовам Write
	assert.EqualValues(t, 5, resFields["size"])
}

func TestRequestLoggerDefaultsToStatus200(t *testing.T) {
	logs := withObservedLog(t)

	// Хендлер не вызывает WriteHeader — net/http подставит 200.
	h := RequestLogger(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
		res.Write([]byte("ok"))
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	entries := logs.All()
	require.Len(t, entries, 2)
	assert.EqualValues(t, http.StatusOK, entries[1].ContextMap()["status"])
	assert.EqualValues(t, 2, entries[1].ContextMap()["size"])
}

func TestRequestLoggerLogsPanicAndRepanics(t *testing.T) {
	logs := withObservedLog(t)

	h := RequestLogger(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	assert.PanicsWithValue(t, "boom", func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	})

	entries := logs.All()
	require.Len(t, entries, 2)
	assert.Equal(t, zapcore.InfoLevel, entries[0].Level)
	assert.Equal(t, "boom", entries[0].ContextMap()["panic"])
	// заголовок не отправлен — 200 подставлять нельзя
	assert.EqualValues(t, 0, entries[1].ContextMap()["status"])
}

// loggingResponseWriter должен запоминать первый статус: net/http учитывает
// только его. httptest.ResponseRecorder сам защищается от повторного вызова,
// поэтому используем собственную заглушку.
type nopResponseWriter struct{ header http.Header }

func (n nopResponseWriter) Header() http.Header         { return n.header }
func (n nopResponseWriter) Write(b []byte) (int, error) { return len(b), nil }
func (n nopResponseWriter) WriteHeader(int)             {}

func TestLoggingResponseWriterKeepsFirstStatus(t *testing.T) {
	lw := newLoggingResponseWriter(nopResponseWriter{header: http.Header{}})

	lw.WriteHeader(http.StatusNotFound)
	lw.WriteHeader(http.StatusInternalServerError)

	assert.Equal(t, http.StatusNotFound, lw.status)
}

func TestInitializeRejectsUnknownLevel(t *testing.T) {
	assert.Error(t, Initialize("verbose"))
	require.NoError(t, Initialize("info"))
}
