package hash

import (
	"bytes"
	"net/http"
	"sync"
)

const (
	maxBodyBytes  = 4 << 20
	maxPooledBody = 64 << 10
)

var statePool = sync.Pool{
	New: func() any { return new(requestState) },
}

type bodyReader struct {
	reader bytes.Reader
}

func (b *bodyReader) Read(p []byte) (int, error) { return b.reader.Read(p) }

func (b *bodyReader) Close() error { return nil }

type requestState struct {
	writer signingWriter
	buf    bytes.Buffer
	body   bodyReader
}

func (st *requestState) reset() {
	st.writer.ResponseWriter = nil
	st.writer.macs = nil

	if st.buf.Cap() > maxPooledBody {
		st.buf = bytes.Buffer{}
	} else {
		st.buf.Reset()
	}

	if st.writer.body.Cap() > maxPooledBody {
		st.writer.body = bytes.Buffer{}
	} else {
		st.writer.body.Reset()
	}

	st.body.reader.Reset(nil)
}

func Middleware(key string) func(http.Handler) http.Handler {
	if key == "" {
		return func(next http.Handler) http.Handler { return next }
	}

	macs := &sync.Pool{
		New: func() any { return newMAC(key) },
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			st := statePool.Get().(*requestState)
			defer func() {
				st.reset()
				statePool.Put(st)
			}()

			sw := &st.writer
			sw.init(res, macs)

			st.buf.Reset()
			if _, err := st.buf.ReadFrom(http.MaxBytesReader(res, req.Body, maxBodyBytes)); err != nil {
				http.Error(sw, "cannot read request body", http.StatusBadRequest)
				sw.flush()
				return
			}

			body := st.buf.Bytes()

			st.body.reader.Reset(body)
			req.Body = &st.body

			if got := req.Header.Get(Header); got != "" && !equalWith(macs, body, got) {
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
	macs        *sync.Pool
	status      int
	body        bytes.Buffer
	wroteHeader bool
	flushed     bool
}

func (w *signingWriter) init(res http.ResponseWriter, macs *sync.Pool) {
	w.ResponseWriter = res
	w.macs = macs
	w.status = http.StatusOK
	w.body.Reset()
	w.wroteHeader = false
	w.flushed = false
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

	w.Header().Set(Header, sumWith(w.macs, w.body.Bytes()))
	w.ResponseWriter.WriteHeader(w.status)

	if w.body.Len() > 0 {
		w.ResponseWriter.Write(w.body.Bytes())
	}
}
