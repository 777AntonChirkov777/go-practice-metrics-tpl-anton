package handlers

import (
	"context"
	"net/http"
	"time"
)

const pingTimeout = 2 * time.Second

type Pinger interface {
	PingContext(ctx context.Context) error
}

type PingHandler struct {
	pinger Pinger
}

func NewPingHandler(p Pinger) *PingHandler {
	return &PingHandler{pinger: p}
}

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
