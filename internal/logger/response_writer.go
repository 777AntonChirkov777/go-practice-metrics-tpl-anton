package logger

import "net/http"

// loggingResponseWriter — обёртка над http.ResponseWriter,
// перехватывающая код статуса и размер тела ответа.
type loggingResponseWriter struct {
	http.ResponseWriter
	status      int
	size        int
	wroteHeader bool
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	// Если хендлер не вызовет WriteHeader, net/http отправит 200,
	// но наш WriteHeader вызван не будет — нулевое значение дало бы status=0.
	return &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	// net/http учитывает только первый вызов; логируем то, что увидел клиент.
	if !r.wroteHeader {
		r.status = statusCode
		r.wroteHeader = true
	}
	// Проброс всегда: поведение должно быть байт-в-байт как без middleware.
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	r.wroteHeader = true // Write без WriteHeader => неявный 200

	n, err := r.ResponseWriter.Write(b)
	// += , а не = : Write может вызываться многократно.
	// n, а не len(b): при частичной записи n < len(b).
	r.size += n
	return n, err
}

func (r *loggingResponseWriter) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
