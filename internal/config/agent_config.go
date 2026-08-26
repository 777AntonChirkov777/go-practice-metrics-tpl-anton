package config

import (
	env "practice/internal/config/env"
	flag "practice/internal/config/flag"
	"practice/internal/logger"
	"time"

	"github.com/creasty/defaults"
	"go.uber.org/zap"
)

type AgentConfig struct {
	Address        string        `default:"localhost:8080"`
	ReportInterval time.Duration `default:"30s"`
	PollInterval   time.Duration `default:"5s"`
	Key            string
	RateLimit      int `default:"1"`
}

func GetAgentConfig(args []string) (*AgentConfig, error) {
	cfg := &AgentConfig{}

	if err := defaults.Set(cfg); err != nil {
		return nil, err
	}

	flagCfg, err := flag.ParseAgentFlags(args)
	if err != nil {
		logger.Log.Info("флаги агента не применены, действуют значения по умолчанию", zap.Error(err))
	} else {
		cfg.Address = flagCfg.Address
		cfg.PollInterval = flagCfg.PollInterval
		cfg.ReportInterval = flagCfg.ReportInterval
		cfg.Key = flagCfg.Key
		cfg.RateLimit = flagCfg.RateLimit
	}

	envCfg, err := env.GetAgentConfigEnv()
	if err != nil {
		logger.Log.Info("переменные окружения агента не применены", zap.Error(err))
		return cfg, nil
	}

	if envCfg.Address != "" {
		cfg.Address = envCfg.Address
	}
	if envCfg.ReportInterval > 0 {
		cfg.ReportInterval = time.Duration(envCfg.ReportInterval) * time.Second
	} else if envCfg.ReportInterval < 0 {
		logger.Log.Info("REPORT_INTERVAL не применён: значение должно быть больше нуля",
			zap.Int("value", envCfg.ReportInterval))
	}
	if envCfg.PollInterval > 0 {
		cfg.PollInterval = time.Duration(envCfg.PollInterval) * time.Second
	} else if envCfg.PollInterval < 0 {
		logger.Log.Info("POLL_INTERVAL не применён: значение должно быть больше нуля",
			zap.Int("value", envCfg.PollInterval))
	}
	if envCfg.Key != "" {
		cfg.Key = envCfg.Key
	}
	if envCfg.RateLimit > 0 {
		cfg.RateLimit = envCfg.RateLimit
	} else if envCfg.RateLimit < 0 {
		logger.Log.Info("RATE_LIMIT не применён: значение должно быть больше нуля",
			zap.Int("value", envCfg.RateLimit))
	}

	return cfg, nil
}
