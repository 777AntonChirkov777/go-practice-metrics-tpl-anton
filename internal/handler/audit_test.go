package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"practice/internal/hash"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type discardAuditor struct{}

func (discardAuditor) Record(_ []string, _ string) {}

func firstAuditor(audit []Auditor) Auditor {
	if len(audit) == 0 {
		return discardAuditor{}
	}

	return audit[0]
}

type auditCall struct {
	metrics []string
	ip      string
}

type auditRecorder struct {
	calls []auditCall
}

func (a *auditRecorder) Record(metrics []string, ip string) {
	a.calls = append(a.calls, auditCall{metrics: metrics, ip: ip})
}

func postFrom(t *testing.T, r *chi.Mux, path, body, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func getFrom(t *testing.T, r *chi.Mux, path, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = remoteAddr

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestClientIP(t *testing.T) {
	cases := map[string]string{
		"192.168.0.42:54321": "192.168.0.42",
		"192.168.0.42":       "192.168.0.42",
		"[::1]:8080":         "::1",
		"":                   "",
	}

	for remoteAddr, want := range cases {
		got := clientIP(&http.Request{RemoteAddr: remoteAddr})
		assert.Equal(t, want, got, "RemoteAddr = %q", remoteAddr)
	}
}

func TestAudit_TextRouteRecordsEvent(t *testing.T) {
	audit := &auditRecorder{}
	r := createTestRouter(NewMockStorage(), audit)

	w := postFrom(t, r, "/update/gauge/Alloc/123.45", "", "192.168.0.42:54321")

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, audit.calls, 1)
	assert.Equal(t, []string{"Alloc"}, audit.calls[0].metrics)
	assert.Equal(t, "192.168.0.42", audit.calls[0].ip)
}

func TestAudit_JSONUpdateRecordsEvent(t *testing.T) {
	audit := &auditRecorder{}
	r := createJSONRouter(NewMockStorage(), audit)

	w := postFrom(t, r, "/update/", `{"id":"Alloc","type":"gauge","value":123.45}`, "10.0.0.7:30000")

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, audit.calls, 1)
	assert.Equal(t, []string{"Alloc"}, audit.calls[0].metrics)
	assert.Equal(t, "10.0.0.7", audit.calls[0].ip)
}

func TestAudit_BatchRecordsOneEventWithEveryName(t *testing.T) {
	audit := &auditRecorder{}
	r := createBatchRouter(NewMockStorage(), audit)

	body := `[{"id":"Alloc","type":"gauge","value":1},` +
		`{"id":"PollCount","type":"counter","delta":2},` +
		`{"id":"Alloc","type":"gauge","value":3}]`

	w := postFrom(t, r, "/updates/", body, "192.168.0.42:54321")

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, audit.calls, 1)
	assert.Equal(t, []string{"Alloc", "PollCount", "Alloc"}, audit.calls[0].metrics)
	assert.Equal(t, "192.168.0.42", audit.calls[0].ip)
}

func TestAudit_ProxyHeadersAreIgnored(t *testing.T) {
	audit := &auditRecorder{}
	r := createJSONRouter(NewMockStorage(), audit)

	req := httptest.NewRequest(http.MethodPost, "/update/",
		strings.NewReader(`{"id":"Alloc","type":"gauge","value":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Real-IP", "203.0.113.9")
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	req.RemoteAddr = "10.0.0.7:30000"

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, audit.calls, 1)
	assert.Equal(t, "10.0.0.7", audit.calls[0].ip)
}

func TestAudit_EmptyBatchRecordsNothing(t *testing.T) {
	audit := &auditRecorder{}
	r := createBatchRouter(NewMockStorage(), audit)

	w := postFrom(t, r, "/updates/", `[]`, "192.168.0.42:54321")

	require.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, audit.calls)
}

func TestAudit_RejectedRequestRecordsNothing(t *testing.T) {
	audit := &auditRecorder{}
	jsonRouter := createJSONRouter(NewMockStorage(), audit)

	w := postFrom(t, jsonRouter, "/update/", `{"id":"x","type":"histogram","value":1}`, "192.168.0.42:54321")
	require.Equal(t, http.StatusBadRequest, w.Code)

	textRouter := createTestRouter(NewMockStorage(), audit)

	w = postFrom(t, textRouter, "/update/gauge/Alloc/not-a-number", "", "192.168.0.42:54321")
	require.Equal(t, http.StatusBadRequest, w.Code)

	assert.Empty(t, audit.calls)
}

func TestAudit_StorageFailureRecordsNothing(t *testing.T) {
	audit := &auditRecorder{}

	store := NewMockStorage()
	store.saveErr = errors.New("storage is down")
	jsonRouter := createJSONRouter(store, audit)

	w := postFrom(t, jsonRouter, "/update/", `{"id":"Alloc","type":"gauge","value":1}`, "192.168.0.42:54321")
	require.Equal(t, http.StatusInternalServerError, w.Code)

	batchStore := NewMockStorage()
	batchStore.batchErr = errors.New("storage is down")
	batchRouter := createBatchRouter(batchStore, audit)

	w = postFrom(t, batchRouter, "/updates/", `[{"id":"Alloc","type":"gauge","value":1}]`, "192.168.0.42:54321")
	require.Equal(t, http.StatusInternalServerError, w.Code)

	assert.Empty(t, audit.calls)
}

func TestAudit_InvalidSignatureRecordsNothing(t *testing.T) {
	audit := &auditRecorder{}

	r := chi.NewRouter()
	r.Use(hash.Middleware("secret"))
	h := NewHandler(NewMockStorage(), audit)
	r.Post("/update/", h.UpdateJSONHandler)

	req := httptest.NewRequest(http.MethodPost, "/update/",
		strings.NewReader(`{"id":"Alloc","type":"gauge","value":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(hash.Header, "deadbeef")
	req.RemoteAddr = "192.168.0.42:54321"

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, audit.calls)
}

func TestAudit_ReadsRecordNothing(t *testing.T) {
	audit := &auditRecorder{}
	r := createTestRouter(NewMockStorage(), audit)

	require.Equal(t, http.StatusOK, postFrom(t, r, "/update/gauge/Alloc/1", "", "192.168.0.42:54321").Code)
	require.Len(t, audit.calls, 1)

	require.Equal(t, http.StatusOK, getFrom(t, r, "/value/gauge/Alloc", "192.168.0.42:54321").Code)
	require.Equal(t, http.StatusOK, getFrom(t, r, "/", "192.168.0.42:54321").Code)

	assert.Len(t, audit.calls, 1, "чтения не аудируются")
}
