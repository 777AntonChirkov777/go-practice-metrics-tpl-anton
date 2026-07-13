package config

import (
	env "practice/internal/config/env"
	flag "practice/internal/config/flag"

	"github.com/creasty/defaults"
)

type ServerConfig struct {
	Address string `default:"localhost:8080"`
}

func GetServerConfig(args []string) (*ServerConfig, error) {
	cfg := &ServerConfig{}

	if err := defaults.Set(cfg); err != nil {
		return nil, err
	}

	evnCfg := env.GetServerConfigEnv()

	if evnCfg != nil {
		cfg.Address = evnCfg.Address
		return cfg, nil
	}

	flagCfg := flag.ParseServerFlags(args)

	if flagCfg != nil {
		cfg.Address = flagCfg.Address
		return cfg, nil
	}

	return cfg, nil
}
