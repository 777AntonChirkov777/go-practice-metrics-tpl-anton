package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

type AgentConfigEnv struct {
	Address        string `env:"ADDRESS"`
	ReportInterval int    `env:"REPORT_INTERVAL"`
	PollInterval   int    `env:"POLL_INTERVAL"`
	Key            string `env:"KEY"`
	RateLimit      int    `env:"RATE_LIMIT"`
}

func GetAgentConfigEnv() (*AgentConfigEnv, error) {
	cfg := &AgentConfigEnv{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("не удалось разобрать переменные окружения: %w", err)
	}

	return cfg, nil
}
