package audit

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type receivedRequest struct {
	method      string
	path        string
	contentType string
	body        []byte
}

func TestHTTPSink_PostsEventAsJSON(t *testing.T) {
	got := make(chan receivedRequest, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		got <- receivedRequest{
			method:      req.Method,
			path:        req.URL.Path,
			contentType: req.Header.Get("Content-Type"),
			body:        body,
		}
		res.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sink, err := NewHTTPSink(srv.URL + "/audit")
	require.NoError(t, err)

	require.NoError(t, sink.Notify(context.Background(), Event{
		Ts: 12345678, Metrics: []string{"Alloc", "Frees"}, IPAddress: "192.168.0.42",
	}))

	received := <-got
	assert.Equal(t, http.MethodPost, received.method)
	assert.Equal(t, "/audit", received.path)
	assert.Equal(t, "application/json", received.contentType)
	assert.JSONEq(t,
		`{"ts":12345678,"metrics":["Alloc","Frees"],"ip_address":"192.168.0.42"}`,
		string(received.body))
}

func TestHTTPSink_NonSuccessStatusIsFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
		res.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	sink, err := NewHTTPSink(srv.URL)
	require.NoError(t, err)

	assert.Error(t, sink.Notify(context.Background(), Event{Ts: 1, Metrics: []string{"Alloc"}}))
}

func TestHTTPSink_RedirectIsFailure(t *testing.T) {
	landed := make(chan string, 1)

	final := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		landed <- req.Method
		res.WriteHeader(http.StatusOK)
	}))
	defer final.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		http.Redirect(res, req, final.URL+"/audit", http.StatusFound)
	}))
	defer srv.Close()

	sink, err := NewHTTPSink(srv.URL + "/audit")
	require.NoError(t, err)

	require.Error(t, sink.Notify(context.Background(), Event{Ts: 1, Metrics: []string{"Alloc"}}))

	select {
	case method := <-landed:
		t.Fatalf("событие ушло по редиректу запросом %s вместо POST на заданный URL", method)
	default:
	}
}

func TestHTTPSink_UnreachableReceiverIsFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
		res.WriteHeader(http.StatusOK)
	}))
	target := srv.URL
	srv.Close()

	sink, err := NewHTTPSink(target)
	require.NoError(t, err)

	assert.Error(t, sink.Notify(context.Background(), Event{Ts: 1, Metrics: []string{"Alloc"}}))
}

func TestHTTPSink_RejectsUnusableURL(t *testing.T) {
	for _, raw := range []string{"", "collector", "collector:9000/audit", "ftp://collector/audit", "http://"} {
		sink, err := NewHTTPSink(raw)
		assert.Error(t, err, "URL %q обязан быть отвергнут", raw)
		assert.Nil(t, sink, "URL %q обязан быть отвергнут", raw)
	}
}
