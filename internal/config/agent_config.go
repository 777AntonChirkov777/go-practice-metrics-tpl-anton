package config

import (
	env "practice/internal/config/env"
	flag "practice/internal/config/flag"
	"time"

	"github.com/creasty/defaults"
)

type AgentConfig struct {
	Address        string        `default:"localhost:8080"`
	ReportInterval time.Duration `default:"30s"`
	PollInterval   time.Duration `default:"5s"`
}

func GetAgentConfig(args []string) (*AgentConfig, error) {
	cfg := &AgentConfig{}

	if err := defaults.Set(cfg); err != nil {
		return nil, err
	}

	evnCfg := env.GetAgentConfigEnv()

	if evnCfg != nil {
		cfg.Address = evnCfg.Address
		cfg.PollInterval = time.Duration(evnCfg.PollInterval) * time.Second
		cfg.ReportInterval = time.Duration(evnCfg.ReportInterval) * time.Second
		return cfg, nil
	}

	flagCfg := flag.ParseAgentFlags(args)

	if flagCfg != nil {
		cfg.Address = flagCfg.Address
		cfg.PollInterval = flagCfg.PollInterval
		cfg.ReportInterval = flagCfg.ReportInterval
		return cfg, nil
	}

	return cfg, nil
}
