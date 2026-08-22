package hash

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"practice/internal/compress"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testKey = "secret"

var testBody = []byte(`{"id":"Alloc","type":"gauge","value":1}`)

func echoHandler(seen *[]byte, called *bool) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		*called = true
		body, err := io.ReadAll(req.Body)
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		*seen = body

		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		res.Write(body)
	})
}

func post(t *testing.T, h http.Handler, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestMiddleware_EmptyKeyIsTransparent(t *testing.T) {
	var seen []byte
	var called bool

	h := Middleware("")(echoHandler(&seen, &called))
	rec := post(t, h, testBody, map[string]string{Header: "not-a-hash"})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, called, "без ключа запрос обязан дойти до хендлера")
	assert.Empty(t, rec.Header().Get(Header), "без ключа ответ не подписывается")
}

func TestMiddleware_ValidSignaturePasses(t *testing.T) {
	var seen []byte
	var called bool

	h := Middleware(testKey)(echoHandler(&seen, &called))
	rec := post(t, h, testBody, map[string]string{Header: Sum(testKey, testBody)})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, testBody, seen, "хендлер обязан получить тело целиком")
}

func TestMiddleware_MissingHeaderPasses(t *testing.T) {
	var seen []byte
	var called bool

	h := Middleware(testKey)(echoHandler(&seen, &called))
	rec := post(t, h, testBody, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, called, "сверять нечего: запрос без подписи обрабатывается обычным порядком")
}

func TestMiddleware_RejectsBadSignature(t *testing.T) {
	cases := map[string]string{
		"чужой ключ":                  Sum("invalidkey", testBody),
		"подпись другого тела":        Sum(testKey, []byte("{}")),
		"не шестнадцатеричная строка": "not-a-hash",
	}

	for name, got := range cases {
		t.Run(name, func(t *testing.T) {
			var seen []byte
			var called bool

			h := Middleware(testKey)(echoHandler(&seen, &called))
			rec := post(t, h, testBody, map[string]string{Header: got})

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.False(t, called, "данные с негодной подписью не должны доходить до хендлера")
		})
	}
}

func TestMiddleware_SignsResponse(t *testing.T) {
	var seen []byte
	var called bool

	h := Middleware(testKey)(echoHandler(&seen, &called))
	rec := post(t, h, testBody, map[string]string{Header: Sum(testKey, testBody)})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, Sum(testKey, rec.Body.Bytes()), rec.Header().Get(Header))
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

func TestMiddleware_SignsErrorResponse(t *testing.T) {
	var seen []byte
	var called bool

	h := Middleware(testKey)(echoHandler(&seen, &called))
	rec := post(t, h, testBody, map[string]string{Header: "not-a-hash"})

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, Sum(testKey, rec.Body.Bytes()), rec.Header().Get(Header))
}

func TestMiddleware_SignsEmptyResponseBody(t *testing.T) {
	h := Middleware(testKey)(http.HandlerFunc(func(res http.ResponseWriter, _ *http.Request) {
		res.WriteHeader(http.StatusOK)
	}))

	rec := post(t, h, testBody, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Body.Bytes())
	assert.Equal(t, Sum(testKey, nil), rec.Header().Get(Header))
}

func TestMiddleware_WithCompress(t *testing.T) {
	var seen []byte
	var called bool

	h := compress.Middleware(Middleware(testKey)(echoHandler(&seen, &called)))

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write(testBody)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	rec := post(t, h, buf.Bytes(), map[string]string{
		Header:             Sum(testKey, testBody),
		"Content-Encoding": "gzip",
		"Accept-Encoding":  "gzip",
		"Content-Type":     "application/json",
	})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, testBody, seen, "подпись сверяется с распакованным телом")
	assert.Contains(t, rec.Header().Get("Content-Encoding"), "gzip", "сжатие ответа обязано сохраниться")

	zr, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	require.NoError(t, err)
	plain, err := io.ReadAll(zr)
	require.NoError(t, err)
	require.NoError(t, zr.Close())

	assert.Equal(t, testBody, plain)
	assert.Equal(t, Sum(testKey, plain), rec.Header().Get(Header), "ответ подписан по несжатому телу")
}
