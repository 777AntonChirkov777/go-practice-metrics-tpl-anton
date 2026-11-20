package logger

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Снимаем до next.ServeHTTP: вложенные middleware и chi.Mount
		// могут мутировать r.URL / r.Method.
		uri := r.RequestURI
		if uri == "" { // запрос, собранный как клиентский (http.NewRequest)
			uri = r.URL.RequestURI()
		}
		method := r.Method

		lw := newLoggingResponseWriter(w)
		completed := false

		// Логируем в defer: строки появятся и когда хендлер паникует.
		defer func() {
			status := lw.status
			if !completed && !lw.wroteHeader {
				status = 0 // клиент получил оборванное соединение, а не 200
			}

			reqFields := []zap.Field{
				zap.String("uri", uri),
				zap.String("method", method),
				zap.Duration("duration", time.Since(start)),
			}
			if !completed {
				// panic — это поле, а не уровень: по заданию всё на Info.
				reqFields = append(reqFields, zap.Bool("panic", true))
			}

			Log.Info("request", reqFields...)
			Log.Info("response",
				zap.Int("status", status),
				zap.Int("size", lw.size),
			)
		}()

		next.ServeHTTP(lw, r) // именно lw, а не w
		completed = true
	})
}
