package config

import (
	"flag"
	"fmt"
	"os"
	"time"
)

// AgentConfig содержит настройки агента сбора метрик.
type AgentConfig struct {
	ServerAddress  string
	ReportInterval time.Duration
	PollInterval   time.Duration
}

// ParseAgentFlags обрабатывает аргументы командной строки для агента.
func ParseAgentFlags(args []string) (*AgentConfig, error) {
	cfg := &AgentConfig{}

	var reportSec, pollSec int

	fs := flag.NewFlagSet("agent", flag.ExitOnError)
	fs.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "адрес HTTP-эндпоинта сервера")
	fs.IntVar(&reportSec, "r", 10, "частота отправки метрик на сервер (в секундах)")
	fs.IntVar(&pollSec, "p", 2, "частота опроса метрик runtime (в секундах)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Агент: %s -a=<адрес> -r=<сек> -p=<сек>\n", os.Args[0])
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if reportSec <= 0 || pollSec <= 0 {
		return nil, fmt.Errorf("интервалы должны быть больше нуля")
	}

	cfg.ReportInterval = time.Duration(reportSec) * time.Second
	cfg.PollInterval = time.Duration(pollSec) * time.Second

	return cfg, nil
}
