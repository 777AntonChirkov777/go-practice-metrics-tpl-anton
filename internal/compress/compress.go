package compress

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

func isCompressible(contentType string) bool {
	if i := strings.IndexByte(contentType, ';'); i >= 0 {
		contentType = contentType[:i]
	}
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "application/json", "text/html":
		return true
	default:
		return false
	}
}

// supportsGzip — клиент готов принять gzip-ответ.
func supportsGzip(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
}

// hasGzipBody — тело запроса пришло сжатым.
func hasGzipBody(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Content-Encoding"), "gzip")
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Входящее сжатое тело: подменяем r.Body распакованным потоком.
		if hasGzipBody(r) {
			cr, err := newCompressReader(r.Body)
			if err != nil {
				// Заголовок обещал gzip, а поток не разжимается — это 400.
				http.Error(w, "invalid gzip body", http.StatusBadRequest)
				return
			}
			r.Body = cr
			defer cr.Close()
			// Заголовок больше не соответствует телу; снимаем, чтобы ниже
			// по стеку никто не попытался распаковать поток повторно.
			r.Header.Del("Content-Encoding")
		}

		// Исходящее сжатие подключаем только когда клиент его поддерживает.
		// Иначе отдаём w без обёртки — поведение байт-в-байт как раньше.
		ow := w
		if supportsGzip(r) {
			cw := newCompressWriter(w)
			ow = cw
			// Close дописывает gzip-трейлер уже после хендлера.
			defer cw.Close()
		}

		next.ServeHTTP(ow, r)
	})
}

// compressWriter откладывает решение о сжатии до момента, когда становится
// известен Content-Type ответа: хендлер выставляет его перед WriteHeader.
type compressWriter struct {
	http.ResponseWriter
	zw          *gzip.Writer
	compress    bool
	wroteHeader bool
}

func newCompressWriter(w http.ResponseWriter) *compressWriter {
	return &compressWriter{ResponseWriter: w}
}

func (c *compressWriter) WriteHeader(statusCode int) {
	if c.wroteHeader {
		// net/http учитывает только первый вызов; не искажаем поведение.
		c.ResponseWriter.WriteHeader(statusCode)
		return
	}
	c.wroteHeader = true

	// Сжимаем только успешные ответы поддерживаемых типов. http.Error отдаёт
	// text/plain со статусом 4xx/5xx — такой ответ сюда не попадёт.
	if statusCode < http.StatusMultipleChoices &&
		isCompressible(c.Header().Get("Content-Type")) {
		c.compress = true
		c.Header().Set("Content-Encoding", "gzip")
		// После сжатия длина тела другая. Устаревший Content-Length
		// обрезал бы ответ у клиента — снимаем, пусть Go выберет chunked.
		c.Header().Del("Content-Length")
		c.zw = gzip.NewWriter(c.ResponseWriter)
	}

	c.ResponseWriter.WriteHeader(statusCode)
}

func (c *compressWriter) Write(b []byte) (int, error) {
	if !c.wroteHeader {
		// Write без WriteHeader — это неявный 200.
		c.WriteHeader(http.StatusOK)
	}
	if c.compress {
		return c.zw.Write(b)
	}
	return c.ResponseWriter.Write(b)
}

// Close дописывает gzip-трейлер. Вызывается из middleware после хендлера.
// Если сжатие не включалось, zw == nil и делать нечего.
func (c *compressWriter) Close() error {
	if c.zw != nil {
		return c.zw.Close()
	}
	return nil
}

// compressReader подменяет r.Body: наружу отдаёт распакованный поток,
// а Close закрывает и gzip-обёртку, и исходное тело.
type compressReader struct {
	body io.ReadCloser
	zr   *gzip.Reader
}

func newCompressReader(body io.ReadCloser) (*compressReader, error) {
	zr, err := gzip.NewReader(body)
	if err != nil {
		return nil, err
	}
	return &compressReader{body: body, zr: zr}, nil
}

func (c *compressReader) Read(p []byte) (int, error) {
	return c.zr.Read(p)
}

func (c *compressReader) Close() error {
	// Закрываем обе стороны; исходное тело — всегда, даже если zr дал ошибку.
	zerr := c.zr.Close()
	berr := c.body.Close()
	if zerr != nil {
		return zerr
	}
	return berr
}
