package config

import (
	"github.com/caarlos0/env/v11"
)

type ServerConfigEnv struct {
	Address         *string `env:"ADDRESS"`
	StoreInterval   *int    `env:"STORE_INTERVAL"`
	FileStoragePath *string `env:"FILE_STORAGE_PATH"`
	Restore         *bool   `env:"RESTORE"`
	DatabaseDSN     *string `env:"DATABASE_DSN"`
}

func GetServerConfigEnv() (*ServerConfigEnv, error) {
	cfg := &ServerConfigEnv{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
