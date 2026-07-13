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
}

// ParseAgentFlags обрабатывает аргументы командной строки для агента.
func ParseAgentFlags(args []string) *AgentConfigFlag {
	cfg := &AgentConfigFlag{}

	var reportSec, pollSec int

	fs := flag.NewFlagSet("agent", flag.ExitOnError)
	fs.StringVar(&cfg.Address, "a", "localhost:8080", "адрес HTTP-эндпоинта сервера")
	fs.IntVar(&reportSec, "r", 10, "частота отправки метрик на сервер (в секундах)")
	fs.IntVar(&pollSec, "p", 2, "частота опроса метрик runtime (в секундах)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Агент: %s -a=<адрес> -r=<сек> -p=<сек>\n", os.Args[0])
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		fmt.Printf("Не получилось спарсить конфиг из флагов\n")
		fmt.Printf("Причина: %s\n", err.Error())
		return nil
	}

	if reportSec <= 0 || pollSec <= 0 {
		fmt.Printf("Не получилось спарсить конфиг из флагов\n")
		fmt.Printf("Причина: интервалы должны быть больше нуля\n")
		return nil
	}

	cfg.ReportInterval = time.Duration(reportSec) * time.Second
	cfg.PollInterval = time.Duration(pollSec) * time.Second

	return cfg
}
