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

	if flagCfg := flag.ParseAgentFlags(args); flagCfg != nil {
		cfg.Address = flagCfg.Address
		cfg.PollInterval = flagCfg.PollInterval
		cfg.ReportInterval = flagCfg.ReportInterval
	}

	if evnCfg := env.GetAgentConfigEnv(); evnCfg != nil {
		if evnCfg.Address != "" {
			cfg.Address = evnCfg.Address
		}
		if evnCfg.ReportInterval != 0 {
			cfg.ReportInterval = time.Duration(evnCfg.ReportInterval) * time.Second
		}
		if evnCfg.PollInterval != 0 {
			cfg.PollInterval = time.Duration(evnCfg.PollInterval) * time.Second
		}
	}

	return cfg, nil
}
