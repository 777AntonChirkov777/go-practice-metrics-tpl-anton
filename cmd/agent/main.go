// main.go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	agent "practice/internal/agent"
	"syscall"
	"time"
)

func main() {
	agent := agent.NewAgent(
		2*time.Second,
		10*time.Second,
		"http://localhost:8080",
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agent.Start(ctx)

	// Ожидаем сигнала завершения.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("Shutting down agent...")
	cancel()
	time.Sleep(time.Second) // даём время на завершение горутин
}
