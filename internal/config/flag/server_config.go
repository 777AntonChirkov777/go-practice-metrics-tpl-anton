package config

import (
	"flag"
	"fmt"
)

// ServerConfigFlag содержит настройки HTTP-сервера.
type ServerConfigFlag struct {
	Address string
}

// ParseServerFlags обрабатывает аргументы командной строки для сервера.
func ParseServerFlags(args []string) *ServerConfigFlag {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)

	var cfg ServerConfigFlag
	fs.StringVar(&cfg.Address, "a", "localhost:8080", "address and port to run server")

	// Parse автоматически упадет с ошибкой, если встретит неизвестный флаг
	if err := fs.Parse(args); err != nil {
		fmt.Printf("Не получилось спарсить конфиг из флагов\n")
		fmt.Printf("Причина: %s\n", err.Error())
		return nil
	}

	// Проверка на лишние позиционные аргументы (если требуется по условию)
	if len(fs.Args()) > 0 {
		fmt.Printf("Не получилось спарсить конфиг из флагов\n")
		fmt.Printf("Причина: unknown positional arguments: %v", fs.Args())
		return nil
	}

	return &cfg
}
