package main

import (
	"context"
	"database/sql"
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
	"practice/internal/db"
	"practice/internal/dbstore"
	"practice/internal/filestore"
	handlers "practice/internal/handler"
	"practice/internal/logger"
	"practice/internal/storage"
	"practice/migrations"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

const dbStartupTimeout = 2 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.GetServerConfig(os.Args[1:])
	if err != nil {
		return fmt.Errorf("error configuration: %w", err)
	}

	// Уровень зафиксирован: по заданию все сообщения логгера — Info.
	if err := logger.Initialize("info"); err != nil {
		return fmt.Errorf("error logger initialization: %w", err)
	}
	defer logger.Sync()

	// Контекст, отменяемый по SIGINT/SIGTERM, — сигнал к остановке.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var pinger handlers.Pinger
	conn, err := db.New(cfg.DatabaseDSN)
	if err != nil {
		logger.Log.Info("database open failed", zap.Error(err))
	} else if conn != nil {
		defer conn.Close()
		pinger = conn
	}

	metricStore, onShutdown := buildStorage(ctx, cfg, conn)

	h := handlers.NewHandler(metricStore)

	r := chi.NewRouter()
	r.Use(logger.RequestLogger)
	r.Use(compress.Middleware)
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

	ph := handlers.NewPingHandler(pinger)
	r.Get("/ping", ph.Ping)
	r.Get("/ping/", ph.Ping)

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

	var runErr error
	select {
	case err := <-errCh:
		runErr = err
		logger.Log.Info("server stopped", zap.Error(err))
	case <-ctx.Done():
		// Получен сигнал остановки — переходим к штатному завершению.
	}

	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Log.Info("graceful shutdown error", zap.Error(err))
	}

	onShutdown()

	return runErr
}

func buildStorage(ctx context.Context, cfg *config.ServerConfig, conn *sql.DB) (storage.MetricStorage, func()) {
	noop := func() {}

	if conn != nil {
		pingCtx, cancelPing := context.WithTimeout(context.Background(), dbStartupTimeout)
		if err := conn.PingContext(pingCtx); err != nil {
			logger.Log.Info("database ping failed on startup", zap.Error(err))
		} else {
			logger.Log.Info("database connected")
		}
		cancelPing()

		if err := migrations.Up(conn); err != nil {
			logger.Log.Info("database migrations failed", zap.Error(err))
		}

		logger.Log.Info("storage: database")
		return dbstore.New(conn), noop
	}

	store := storage.NewMemStorage()

	if cfg.FileStoragePath == "" {
		logger.Log.Info("storage: memory")
		return store, noop
	}

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

	onShutdown := func() {
		if err := fs.SaveFrom(context.Background(), store); err != nil {
			logger.Log.Info("final dump failed", zap.Error(err))
		} else {
			logger.Log.Info("metrics saved on shutdown", zap.String("file", cfg.FileStoragePath))
		}
	}

	// Выбор режима записи на диск.
	if cfg.StoreInterval <= 0 {
		// Синхронный режим: пишем на диск при каждом обновлении.
		logger.Log.Info("disk persistence: synchronous",
			zap.String("file", cfg.FileStoragePath))
		return filestore.NewSyncStorage(store, fs), onShutdown
	}

	// Асинхронный режим: периодический сброс снимка на диск.
	go periodicSave(ctx, fs, store, cfg.StoreInterval)
	logger.Log.Info("disk persistence: periodic",
		zap.Duration("interval", cfg.StoreInterval),
		zap.String("file", cfg.FileStoragePath))

	return store, onShutdown
}

func periodicSave(ctx context.Context, fs *filestore.FileStore, src filestore.Snapshotter, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := fs.SaveFrom(ctx, src); err != nil {
				logger.Log.Info("periodic dump failed", zap.Error(err))
			}
		}
	}
}
