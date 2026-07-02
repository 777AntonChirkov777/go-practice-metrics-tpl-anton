// main.go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	agent "practice/internal/agent"
	"practice/internal/config"
	"syscall"
	"time"
)

func main() {

	cfg, err := config.ParseAgentFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "ошибка конфигурации: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Агент запущен:\n")
	fmt.Printf("  адрес сервера: %s\n", cfg.ServerAddress)
	fmt.Printf("  интервал отправки: %v\n", cfg.ReportInterval)
	fmt.Printf("  интервал опроса:   %v\n", cfg.PollInterval)

	agent := agent.NewAgent(
		cfg.PollInterval,
		cfg.ReportInterval,
		"http://"+cfg.ServerAddress,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agent.Start(ctx)

	// Ожидаем сигнала завершения.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Println("Shutting down agent...")
	cancel()
	time.Sleep(time.Second) // даём время на завершение горутин

}
