package hash

import (
	"bytes"
	"io"
	"net/http"
)

const maxBodyBytes = 4 << 20

func Middleware(key string) func(http.Handler) http.Handler {
	if key == "" {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			sw := &signingWriter{ResponseWriter: res, key: key, status: http.StatusOK}

			body, err := io.ReadAll(http.MaxBytesReader(res, req.Body, maxBodyBytes))
			if err != nil {
				http.Error(sw, "cannot read request body", http.StatusBadRequest)
				sw.flush()
				return
			}
			req.Body = io.NopCloser(bytes.NewReader(body))

			if got := req.Header.Get(Header); got != "" && !Equal(key, body, got) {
				http.Error(sw, "invalid hash", http.StatusBadRequest)
				sw.flush()
				return
			}

			next.ServeHTTP(sw, req)
			sw.flush()
		})
	}
}

type signingWriter struct {
	http.ResponseWriter
	key         string
	status      int
	body        bytes.Buffer
	wroteHeader bool
	flushed     bool
}

func (w *signingWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = statusCode
}

func (w *signingWriter) Write(b []byte) (int, error) {
	w.wroteHeader = true
	return w.body.Write(b)
}

func (w *signingWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *signingWriter) flush() {
	if w.flushed {
		return
	}
	w.flushed = true

	w.Header().Set(Header, Sum(w.key, w.body.Bytes()))
	w.ResponseWriter.WriteHeader(w.status)

	if w.body.Len() > 0 {
		w.ResponseWriter.Write(w.body.Bytes())
	}
}
