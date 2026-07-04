package config

import (
	"flag"
	"fmt"
)

// ServerConfig содержит настройки HTTP-сервера.
type ServerConfig struct {
	Address string
}

// ParseServerFlags обрабатывает аргументы командной строки для сервера.
func ParseServerFlags(args []string) (*ServerConfig, error) {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)

	var cfg ServerConfig
	fs.StringVar(&cfg.Address, "a", "localhost:8080", "address and port to run server")

	// Parse автоматически упадет с ошибкой, если встретит неизвестный флаг
	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	// Проверка на лишние позиционные аргументы (если требуется по условию)
	if len(fs.Args()) > 0 {
		return nil, fmt.Errorf("unknown positional arguments: %v", fs.Args())
	}

	return &cfg, nil
}
