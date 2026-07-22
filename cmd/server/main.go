package main

import (
	"fmt"
	"net"
	"net/http"
	"os"

	//configF "practice/internal/config/flag"
	"practice/internal/compress"
	config "practice/internal/config"
	handlers "practice/internal/handler"
	"practice/internal/logger"
	"practice/internal/storage"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.GetServerConfig(os.Args[1:])

	if err != nil {
		fmt.Fprintf(os.Stderr, "error configuration: %v\n", err)
		os.Exit(1)
	}

	// Уровень зафиксирован: по заданию все сообщения логгера — Info.
	if err := logger.Initialize("info"); err != nil {
		fmt.Fprintf(os.Stderr, "error logger initialization: %v\n", err)
		os.Exit(1)
	}

	store := storage.NewMemStorage()
	h := handlers.NewHandler(store)

	r := chi.NewRouter()
	// Use обязан идти до регистрации маршрутов, иначе chi паникует:
	// "all middlewares must be defined before routes on a mux".
	//
	// Порядок важен: RequestLogger снаружи, compress внутри. Тогда логгер
	// видит ответ уже сжатым и пишет в size реальный объём байт «на проводе».
	r.Use(logger.RequestLogger)
	r.Use(compress.Middleware)
	r.Get("/", h.ListHandler)

	// JSON-эндпоинты инкремента 7. Регистрируем обе формы, со слэшем и без:
	// для chi "/update" и "/update/" — РАЗНЫЕ маршруты, редиректа между ними нет.
	// Конфликта с "/update/{type}/{name}/{value}" не возникает: статический узел
	// и param-потомок сосуществуют в дереве chi.
	r.Post("/update", h.UpdateJSONHandler)
	r.Post("/update/", h.UpdateJSONHandler)
	r.Post("/value", h.ValueJSONHandler)
	r.Post("/value/", h.ValueJSONHandler)

	// Текстовые эндпоинты инкрементов 1-5 остаются нетронутыми.
	r.Post("/update/{type}/{name}/{value}", h.UpdateHandler)
	r.Get("/value/{type}/{name}", h.ValueHandler)

	_, port, err := net.SplitHostPort(cfg.Address)
	if err != nil {
		// Фоллбэк, если адрес передан без хоста (например, ":8080") или нестандартно.
		port = strings.TrimPrefix(cfg.Address, ":")
	}
	listenAddr := ":" + port

	logger.Log.Info("server started", zap.String("address", cfg.Address))

	if err := http.ListenAndServe(listenAddr, r); err != nil {
		// zap.Error — это поле, уровень сообщения остаётся Info.
		logger.Log.Info("server stopped", zap.Error(err))
		logger.Sync()
		os.Exit(1)
	}
}
