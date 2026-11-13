package handlers

import (
	"context"
	"net/http"
	"time"
)

const pingTimeout = 2 * time.Second

// Pinger описывает источник, доступность которого проверяет [PingHandler].
// В сервисе это подключение к базе данных.
type Pinger interface {
	// PingContext проверяет доступность источника и возвращает ошибку,
	// если проверка не удалась или контекст отменён.
	PingContext(ctx context.Context) error
}

// PingHandler обрабатывает проверку доступности базы данных через [Pinger].
type PingHandler struct {
	pinger Pinger
}

// NewPingHandler создаёт [PingHandler] поверх p. Допустим nil: тогда Ping
// отвечает 500 с текстом "database is not configured", а не паникует.
func NewPingHandler(p Pinger) *PingHandler {
	return &PingHandler{pinger: p}
}

// Ping обрабатывает GET /ping и GET /ping/: вызывает PingContext у [Pinger]
// с таймаутом 2 секунды от контекста запроса. Тело запроса не используется.
//
// Коды ответа:
//   - 200 — источник доступен, тело ответа пустое;
//   - 500 — [Pinger] не задан ("database is not configured") или проверка
//     не прошла ("database is unavailable").
func (h *PingHandler) Ping(res http.ResponseWriter, req *http.Request) {
	if h.pinger == nil {
		http.Error(res, "database is not configured", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), pingTimeout)
	defer cancel()

	if err := h.pinger.PingContext(ctx); err != nil {
		http.Error(res, "database is unavailable", http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusOK)
}
