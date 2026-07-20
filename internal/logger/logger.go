// Package logger содержит инициализацию общего логгера приложения
// и HTTP-middleware для логирования запросов и ответов.
package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Log — синглтон логгера. По умолчанию no-op, чтобы код, вызванный
// до Initialize (например, юнит-тесты middleware), не падал по nil.
var Log = zap.NewNop()

// Initialize настраивает синглтон логгера на указанный уровень.
func Initialize(level string) error {
	lvl, err := zap.ParseAtomicLevel(level)
	if err != nil {
		return err
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = lvl

	// NewProductionConfig включает сэмплинг (Initial:100, Thereafter:100)
	// по паре (уровень, message). У наших строк message одинаковый,
	// поэтому под нагрузкой zap молча выбросил бы большую их часть.
	cfg.Sampling = nil

	// Диагностика из internal/config/** идёт через fmt.Printf в stdout —
	// держим логи в том же потоке, чтобы порядок строк был осмысленным.
	cfg.OutputPaths = []string{"stdout"}
	cfg.ErrorOutputPaths = []string{"stderr"}

	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	// По умолчанию duration — float в секундах: 84 мкс печатается как 0.
	cfg.EncoderConfig.EncodeDuration = zapcore.StringDurationEncoder

	zl, err := cfg.Build()
	if err != nil {
		return err
	}

	Log = zl
	return nil
}

// Sync сбрасывает буферы логгера. Ошибка игнорируется сознательно:
// Sync() на консольном дескрипторе всегда возвращает ошибку
// (uber-go/zap#328, #991). Потери данных при этом нет.
func Sync() { _ = Log.Sync() }
