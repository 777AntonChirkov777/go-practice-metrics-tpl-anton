package agent

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"practice/internal/compress"
	"practice/internal/encryption"
	"practice/internal/hash"
	"practice/internal/model"
)

var agentTestKey = sync.OnceValues(func() (*rsa.PrivateKey, error) { return rsa.GenerateKey(rand.Reader, 2048) })

func testPrivateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := agentTestKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

func decryptBody(t *testing.T, priv *rsa.PrivateKey, body []byte) []byte {
	t.Helper()
	plain, err := encryption.Decrypt(priv, body)
	if err != nil {
		t.Fatalf("тело не расшифровывается: %v", err)
	}
	return plain
}

func TestSendBatchEncryptsReport(t *testing.T) {
	priv := testPrivateKey(t)

	var mu sync.Mutex
	var markers []string
	var received []model.Metrics

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var batch []model.Metrics
		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
			t.Errorf("decode batch: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		received = append(received, batch...)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	decrypting := encryption.Middleware(priv)(compress.Middleware(inner))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		markers = append(markers, r.Header.Get(encryption.Header))
		mu.Unlock()
		decrypting.ServeHTTP(w, r)
	}))
	defer srv.Close()

	agent := NewAgent(time.Second, time.Second, srv.URL, "", 1, &priv.PublicKey)
	agent.CollectMetrics()
	agent.sendBatch(context.Background(), agent.snapshot())

	mu.Lock()
	defer mu.Unlock()

	if len(markers) != 1 {
		t.Fatalf("requests = %d, want 1", len(markers))
	}
	if markers[0] != encryption.Scheme {
		t.Errorf("%s = %q, want %q", encryption.Header, markers[0], encryption.Scheme)
	}

	hasPollCount := false
	for _, m := range received {
		if m.ID == "PollCount" {
			hasPollCount = true
		}
	}
	if len(received) == 0 || !hasPollCount {
		t.Fatalf("сервер не получил батч после расшифровки: %d метрик", len(received))
	}
}

func TestSendBatchSignsPlainJSONWhenEncrypted(t *testing.T) {
	priv := testPrivateKey(t)
	tr := &stubTransport{}

	agent := NewAgent(time.Second, time.Second, "http://metrics.invalid", "secret", 1, &priv.PublicKey)
	agent.client = &http.Client{Transport: tr}

	agent.CollectMetrics()
	agent.sendBatch(context.Background(), agent.snapshot())

	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.calls != 1 {
		t.Fatalf("calls = %d, want 1", tr.calls)
	}

	plain := gunzipBody(t, decryptBody(t, priv, tr.bodies[0]))
	if !json.Valid(plain) {
		t.Fatalf("расшифрованное тело не JSON: %q", plain)
	}

	want := hash.Sum("secret", plain)
	if got := tr.headers[0].Get(hash.Header); got != want {
		t.Errorf("%s = %q, want %q: подпись считается от открытого JSON", hash.Header, got, want)
	}
	if got := tr.headers[0].Get("Content-Encoding"); got != "gzip" {
		t.Errorf("Content-Encoding = %q, want gzip", got)
	}
	if got := tr.headers[0].Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}

func TestSendBatchRetryKeepsCiphertext(t *testing.T) {
	priv := testPrivateKey(t)
	tr := &stubTransport{statuses: []int{
		http.StatusInternalServerError,
		http.StatusOK,
	}}

	agent := NewAgent(time.Second, time.Second, "http://metrics.invalid", "secret", 1, &priv.PublicKey)
	agent.client = &http.Client{Transport: tr}
	agent.retryDelays = []time.Duration{0, 0, 0}

	agent.CollectMetrics()
	agent.sendBatch(context.Background(), agent.snapshot())

	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.calls != 2 {
		t.Fatalf("calls = %d, want 2", tr.calls)
	}
	if !bytes.Equal(tr.bodies[0], tr.bodies[1]) {
		t.Error("повторная попытка отправила другой шифротекст")
	}
	if !reflect.DeepEqual(tr.headers[0], tr.headers[1]) {
		t.Errorf("заголовки попыток различаются: %v vs %v", tr.headers[0], tr.headers[1])
	}
}

func TestSendBatchWithoutPublicKeyIsPlain(t *testing.T) {
	tr := &stubTransport{}

	agent := NewAgent(time.Second, time.Second, "http://metrics.invalid", "", 1, nil)
	agent.client = &http.Client{Transport: tr}

	agent.CollectMetrics()
	agent.sendBatch(context.Background(), agent.snapshot())

	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.calls != 1 {
		t.Fatalf("calls = %d, want 1", tr.calls)
	}
	if got := tr.headers[0].Values(encryption.Header); len(got) != 0 {
		t.Errorf("%s = %q, want absent", encryption.Header, got)
	}
	if !json.Valid(gunzipBody(t, tr.bodies[0])) {
		t.Error("тело без шифрования должно быть JSON в gzip")
	}
}
