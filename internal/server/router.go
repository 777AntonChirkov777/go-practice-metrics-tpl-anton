package server

import (
	"net/http"

	"practice/internal/compress"
	"practice/internal/handler"
	"practice/internal/hash"
	"practice/internal/logger"
	"practice/internal/storage"

	"github.com/go-chi/chi/v5"
)

func NewRouter(store storage.MetricStorage, auditor handler.Auditor, pinger handler.Pinger, key string) http.Handler {
	h := handler.NewHandler(store, auditor)

	r := chi.NewRouter()
	r.Use(logger.RequestLogger)
	r.Use(compress.Middleware)
	r.Use(hash.Middleware(key))
	r.Get("/", h.ListHandler)

	r.Post("/update", h.UpdateJSONHandler)
	r.Post("/update/", h.UpdateJSONHandler)
	r.Post("/value", h.ValueJSONHandler)
	r.Post("/value/", h.ValueJSONHandler)

	r.Post("/updates", h.UpdatesJSONHandler)
	r.Post("/updates/", h.UpdatesJSONHandler)

	// Текстовые эндпоинты инкрементов 1-5 остаются нетронутыми.
	r.Post("/update/{type}/{name}/{value}", h.UpdateHandler)
	r.Get("/value/{type}/{name}", h.ValueHandler)

	ph := handler.NewPingHandler(pinger)
	r.Get("/ping", ph.Ping)
	r.Get("/ping/", ph.Ping)

	return r
}
