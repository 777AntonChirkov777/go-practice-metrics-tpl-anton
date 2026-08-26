package config

import (
	"testing"
	"time"
)

func TestParseAgentFlags_Valid(t *testing.T) {
	cfg, err := ParseAgentFlags([]string{"-a=localhost:9999", "-r=7", "-p=4", "-k=secret", "-l=3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Address != "localhost:9999" {
		t.Errorf("Address = %q, want localhost:9999", cfg.Address)
	}
	if cfg.ReportInterval != 7*time.Second {
		t.Errorf("ReportInterval = %v, want 7s", cfg.ReportInterval)
	}
	if cfg.PollInterval != 4*time.Second {
		t.Errorf("PollInterval = %v, want 4s", cfg.PollInterval)
	}
	if cfg.Key != "secret" {
		t.Errorf("Key = %q, want secret", cfg.Key)
	}
	if cfg.RateLimit != 3 {
		t.Errorf("RateLimit = %d, want 3", cfg.RateLimit)
	}
}

func TestParseAgentFlags_Defaults(t *testing.T) {
	cfg, err := ParseAgentFlags(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RateLimit != 1 {
		t.Errorf("RateLimit = %d, want 1", cfg.RateLimit)
	}
	if cfg.ReportInterval != 10*time.Second {
		t.Errorf("ReportInterval = %v, want 10s", cfg.ReportInterval)
	}
	if cfg.PollInterval != 2*time.Second {
		t.Errorf("PollInterval = %v, want 2s", cfg.PollInterval)
	}
}

func TestParseAgentFlags_RejectsNonPositive(t *testing.T) {
	for _, args := range [][]string{{"-l=0"}, {"-l=-1"}, {"-r=0"}, {"-r=-1"}, {"-p=0"}, {"-p=-1"}} {
		cfg, err := ParseAgentFlags(args)
		if err == nil {
			t.Errorf("%v: ошибка не возвращена", args)
		}
		if cfg != nil {
			t.Errorf("%v: cfg = %+v, want nil", args, cfg)
		}
	}
}

func TestParseAgentFlags_RejectsUnknownFlag(t *testing.T) {
	cfg, err := ParseAgentFlags([]string{"-nosuchflag=1"})
	if err == nil {
		t.Error("ошибка не возвращена на неизвестном флаге")
	}
	if cfg != nil {
		t.Errorf("cfg = %+v, want nil", cfg)
	}
}
