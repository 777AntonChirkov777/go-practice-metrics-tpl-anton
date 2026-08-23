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

func GetAgentConfigEnv() *AgentConfigEnv {
	cfg := &AgentConfigEnv{}
	if err := env.Parse(cfg); err != nil {
		fmt.Printf("Не получилось спарсить конфиг из переменных окружения\n")
		fmt.Printf("Причина: %s\n", err.Error())
		return nil
	}

	return cfg
}
