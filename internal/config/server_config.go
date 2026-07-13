package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

type ServerConfig struct {
	Address string `env:"ADDRESS" envDefault:"localhost:8080"`
}

func GetServerConfig() (*ServerConfig, error) {
	cfg := &ServerConfig{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse env: %w", err)
	}
	return cfg, nil
}
