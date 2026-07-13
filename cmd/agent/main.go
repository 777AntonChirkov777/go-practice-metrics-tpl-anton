// main.go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	agent "practice/internal/agent"

	//configF "practice/internal/config/flag"
	config "practice/internal/config"
	"syscall"
)

func main() {

	acfg, err := config.GetAgentConfig(os.Args[1:])

	if err != nil {
		fmt.Fprintf(os.Stderr, "ошибка конфигурации: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Агент запущен:\n")
	fmt.Printf("  адрес сервера: %s\n", acfg.Address)
	fmt.Printf("  интервал отправки: %v\n", acfg.ReportInterval)
	fmt.Printf("  интервал опроса:   %v\n", acfg.PollInterval)

	agent := agent.NewAgent(
		acfg.PollInterval,
		acfg.ReportInterval,
		"http://"+acfg.Address,
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
	agent.Wait()

}
