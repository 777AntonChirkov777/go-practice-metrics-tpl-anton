package main

import (
	"context"
	"crypto/rsa"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	agent "practice/internal/agent"
	"practice/internal/buildinfo"
	config "practice/internal/config"
	"practice/internal/encryption"
	"practice/internal/logger"

	"go.uber.org/zap"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func main() {
	buildinfo.Print(os.Stdout, buildVersion, buildDate, buildCommit)

	// Уровень зафиксирован: по заданию все сообщения логгера — Info.
	if err := logger.Initialize("info"); err != nil {
		fmt.Fprintf(os.Stderr, "ошибка инициализации логгера: %v\n", err)
		os.Exit(1)
	}

	acfg, err := config.GetAgentConfig(os.Args[1:])

	if err != nil {
		fmt.Fprintf(os.Stderr, "ошибка конфигурации: %v\n", err)
		os.Exit(1)
	}

	var publicKey *rsa.PublicKey
	if acfg.CryptoKey != "" {
		publicKey, err = encryption.LoadPublicKey(acfg.CryptoKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ошибка загрузки ключа шифрования: %v\n", err)
			os.Exit(1)
		}
	}

	logger.Log.Info("agent started",
		zap.String("server_address", acfg.Address),
		zap.Duration("report_interval", acfg.ReportInterval),
		zap.Duration("poll_interval", acfg.PollInterval),
		zap.Bool("signing", acfg.Key != ""),
		zap.Bool("encryption", publicKey != nil),
		zap.Int("rate_limit", acfg.RateLimit),
	)

	agent := agent.NewAgent(
		acfg.PollInterval,
		acfg.ReportInterval,
		"http://"+acfg.Address,
		acfg.Key,
		acfg.RateLimit,
		publicKey,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agent.Start(ctx)

	// Ожидаем сигнала завершения.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logger.Log.Info("agent shutting down")
	cancel()
	agent.Wait()
	logger.Sync()

}
