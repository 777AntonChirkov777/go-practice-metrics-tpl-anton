package config

import (
	"flag"
	"fmt"
)

type ServerConfigFlag struct {
	Address         string
	StoreInterval   int
	FileStoragePath string
	Restore         bool
	DatabaseDSN     string
	Key             string
}

func ParseServerFlags(args []string) (*ServerConfigFlag, error) {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)

	var cfg ServerConfigFlag
	fs.StringVar(&cfg.Address, "a", "localhost:8080", "address and port to run server")
	fs.IntVar(&cfg.StoreInterval, "i", 300, "интервал записи метрик на диск в секундах (0 — синхронно)")
	fs.StringVar(&cfg.FileStoragePath, "f", "metrics-db.json", "путь до файла, куда сохраняются метрики")
	fs.BoolVar(&cfg.Restore, "r", true, "загружать ли сохранённые метрики при старте")
	fs.StringVar(&cfg.DatabaseDSN, "d", "", "строка подключения к базе данных")
	fs.StringVar(&cfg.Key, "k", "", "ключ подписи HashSHA256")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("не удалось разобрать флаги: %w", err)
	}

	if len(fs.Args()) > 0 {
		return nil, fmt.Errorf("неизвестные позиционные аргументы: %v", fs.Args())
	}

	return &cfg, nil
}
