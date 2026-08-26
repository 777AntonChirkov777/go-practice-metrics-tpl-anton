package config

import (
	"flag"
	"fmt"
	"os"
	"time"
)

// AgentConfigFlag содержит настройки агента сбора метрик.
type AgentConfigFlag struct {
	Address        string
	ReportInterval time.Duration
	PollInterval   time.Duration
	Key            string
	RateLimit      int
}

// ParseAgentFlags обрабатывает аргументы командной строки для агента.
func ParseAgentFlags(args []string) (*AgentConfigFlag, error) {
	cfg := &AgentConfigFlag{}

	var reportSec, pollSec int

	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	fs.StringVar(&cfg.Address, "a", "localhost:8080", "адрес HTTP-эндпоинта сервера")
	fs.IntVar(&reportSec, "r", 10, "частота отправки метрик на сервер (в секундах)")
	fs.IntVar(&pollSec, "p", 2, "частота опроса метрик runtime (в секундах)")
	fs.StringVar(&cfg.Key, "k", "", "ключ подписи HashSHA256")
	fs.IntVar(&cfg.RateLimit, "l", 1, "предел одновременно исходящих запросов к серверу")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Агент: %s -a=<адрес> -r=<сек> -p=<сек> -k=<ключ> -l=<число>\n", os.Args[0])
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("не удалось разобрать флаги: %w", err)
	}

	if reportSec <= 0 || pollSec <= 0 {
		return nil, fmt.Errorf("интервалы должны быть больше нуля: -r=%d, -p=%d", reportSec, pollSec)
	}

	if cfg.RateLimit <= 0 {
		return nil, fmt.Errorf("предел одновременных запросов должен быть больше нуля: -l=%d", cfg.RateLimit)
	}

	cfg.ReportInterval = time.Duration(reportSec) * time.Second
	cfg.PollInterval = time.Duration(pollSec) * time.Second

	return cfg, nil
}
