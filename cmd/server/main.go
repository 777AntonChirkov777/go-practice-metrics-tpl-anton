package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"practice/internal/compress"
	config "practice/internal/config"
	"practice/internal/filestore"
	handlers "practice/internal/handler"
	"practice/internal/logger"
	"practice/internal/storage"

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
	fs := filestore.New(cfg.FileStoragePath)

	// Восстановление ранее сохранённых значений.
	if cfg.Restore {
		metrics, err := fs.Load()
		if err != nil {
			logger.Log.Info("restore skipped", zap.Error(err))
		} else {
			store.LoadAll(metrics)
			logger.Log.Info("metrics restored",
				zap.Int("count", len(metrics)),
				zap.String("file", cfg.FileStoragePath))
		}
	}

	// Контекст, отменяемый по SIGINT/SIGTERM, — сигнал к остановке.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Выбор режима записи на диск.
	var metricStore storage.MetricStorage = store
	if cfg.StoreInterval <= 0 {
		// Синхронный режим: пишем на диск при каждом обновлении.
		metricStore = filestore.NewSyncStorage(store, fs)
		logger.Log.Info("disk persistence: synchronous",
			zap.String("file", cfg.FileStoragePath))
	} else {
		// Асинхронный режим: периодический сброс снимка на диск.
		go periodicSave(ctx, fs, store, cfg.StoreInterval)
		logger.Log.Info("disk persistence: periodic",
			zap.Duration("interval", cfg.StoreInterval),
			zap.String("file", cfg.FileStoragePath))
	}

	h := handlers.NewHandler(metricStore)

	r := chi.NewRouter()
	r.Use(logger.RequestLogger)
	r.Use(compress.Middleware)
	r.Get("/", h.ListHandler)

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
	srv := &http.Server{Addr: ":" + port, Handler: r}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	logger.Log.Info("server started", zap.String("address", cfg.Address))

	select {
	case err := <-errCh:
		logger.Log.Info("server stopped", zap.Error(err))
		// Пытаемся сохранить накопленное перед аварийным выходом.
		_ = fs.Save(store.GetAll())
		logger.Sync()
		os.Exit(1)
	case <-ctx.Done():
		// Получен сигнал остановки — переходим к штатному завершению.
	}

	// Штатное завершение: закрываем HTTP-сервер и сохраняем финальный снимок.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Log.Info("graceful shutdown error", zap.Error(err))
	}

	if err := fs.Save(store.GetAll()); err != nil {
		logger.Log.Info("final dump failed", zap.Error(err))
	} else {
		logger.Log.Info("metrics saved on shutdown", zap.String("file", cfg.FileStoragePath))
	}

	logger.Sync()
}

// periodicSave раз в interval сбрасывает текущий снимок метрик на диск, пока
// не будет отменён контекст.
func periodicSave(ctx context.Context, fs *filestore.FileStore, store *storage.MemStorage, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := fs.Save(store.GetAll()); err != nil {
				logger.Log.Info("periodic dump failed", zap.Error(err))
			}
		}
	}
}
