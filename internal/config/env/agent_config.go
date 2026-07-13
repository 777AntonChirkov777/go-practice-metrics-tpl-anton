package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type AgentConfigEnv struct {
	Address        string        `env:"ADDRESS"`
	ReportInterval time.Duration `env:"REPORT_INTERVAL"`
	PollInterval   time.Duration `env:"POLL_INTERVAL"`
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
