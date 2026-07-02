package config

import (
	"flag"
	"fmt"
	"os"
)

// ServerConfig содержит настройки HTTP-сервера.
type ServerConfig struct {
	Address string
}

// ParseServerFlags обрабатывает аргументы командной строки для сервера.
func ParseServerFlags(args []string) (*ServerConfig, error) {
	cfg := &ServerConfig{}

	fs := flag.NewFlagSet("server", flag.ExitOnError)
	fs.StringVar(&cfg.Address, "a", "localhost:8080", "адрес HTTP-эндпоинта")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Сервер: %s -a=<адрес>\n", os.Args[0])
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return cfg, nil
}
