package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

type ServerConfigEnv struct {
	Address string `env:"ADDRESS"`
}

func GetServerConfigEnv() *ServerConfigEnv {
	cfg := &ServerConfigEnv{}
	if err := env.Parse(cfg); err != nil {
		fmt.Printf("Не получилось спарсить конфиг из переменных окружения\n")
		fmt.Printf("Причина: %s\n", err.Error())
		return nil
	}
	return cfg
}
