package config

import (
	"time"

	env "practice/internal/config/env"
	flag "practice/internal/config/flag"

	"github.com/creasty/defaults"
)

type ServerConfig struct {
	Address         string        `default:"localhost:8080"`
	StoreInterval   time.Duration `default:"300s"`
	FileStoragePath string        `default:"metrics-db.json"`
	Restore         bool          `default:"true"`
}

func GetServerConfig(args []string) (*ServerConfig, error) {
	cfg := &ServerConfig{}

	if err := defaults.Set(cfg); err != nil {
		return nil, err
	}

	// Слой флагов. Значения по умолчанию флагов совпадают с дефолтами cfg,
	// поэтому их применение при отсутствии флагов не меняет дефолты.
	flagCfg, err := flag.ParseServerFlags(args)
	if err != nil {
		return nil, err
	}
	cfg.Address = flagCfg.Address
	cfg.StoreInterval = time.Duration(flagCfg.StoreInterval) * time.Second
	cfg.FileStoragePath = flagCfg.FileStoragePath
	cfg.Restore = flagCfg.Restore

	// Слой окружения. Переопределяем только присутствующие (не-nil) поля.
	envCfg, err := env.GetServerConfigEnv()
	if err != nil {
		return nil, err
	}
	if envCfg.Address != nil && *envCfg.Address != "" {
		cfg.Address = *envCfg.Address
	}
	if envCfg.StoreInterval != nil {
		cfg.StoreInterval = time.Duration(*envCfg.StoreInterval) * time.Second
	}
	if envCfg.FileStoragePath != nil && *envCfg.FileStoragePath != "" {
		cfg.FileStoragePath = *envCfg.FileStoragePath
	}
	if envCfg.Restore != nil {
		cfg.Restore = *envCfg.Restore
	}

	return cfg, nil
}
