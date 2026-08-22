package config

import (
	"testing"
	"time"
)

func clearAgentEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ADDRESS", "")
	t.Setenv("REPORT_INTERVAL", "")
	t.Setenv("POLL_INTERVAL", "")
	t.Setenv("KEY", "")
}

func TestGetAgentConfig_AddressFromEnvOnly(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("ADDRESS", "localhost:33907")

	cfg, err := GetAgentConfig(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Address != "localhost:33907" {
		t.Errorf("Address = %q, want localhost:33907", cfg.Address)
	}
	// Интервалы приходят из дефолтов флагов (10s / 2s).
	if cfg.ReportInterval != 10*time.Second {
		t.Errorf("ReportInterval = %v, want 10s", cfg.ReportInterval)
	}
	if cfg.PollInterval != 2*time.Second {
		t.Errorf("PollInterval = %v, want 2s", cfg.PollInterval)
	}
}

// Приоритет: env перекрывает флаги, но лишь для заданных полей.
func TestGetAgentConfig_EnvOverridesFlagsPerField(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("ADDRESS", "localhost:33907") // только адрес

	cfg, err := GetAgentConfig([]string{"-a=localhost:9999", "-r=3", "-p=1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Address != "localhost:33907" {
		t.Errorf("Address = %q, want localhost:33907 (env wins)", cfg.Address)
	}
	// REPORT_INTERVAL/POLL_INTERVAL в env нет — значения берутся из флагов.
	if cfg.ReportInterval != 3*time.Second {
		t.Errorf("ReportInterval = %v, want 3s (from flag)", cfg.ReportInterval)
	}
	if cfg.PollInterval != 1*time.Second {
		t.Errorf("PollInterval = %v, want 1s (from flag)", cfg.PollInterval)
	}
}

// Без окружения работают флаги — поведение прежних инкрементов не меняется.
func TestGetAgentConfig_FlagsOnly(t *testing.T) {
	clearAgentEnv(t)

	cfg, err := GetAgentConfig([]string{"-a=localhost:5555", "-r=7", "-p=4"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Address != "localhost:5555" {
		t.Errorf("Address = %q, want localhost:5555", cfg.Address)
	}
	if cfg.ReportInterval != 7*time.Second {
		t.Errorf("ReportInterval = %v, want 7s", cfg.ReportInterval)
	}
	if cfg.PollInterval != 4*time.Second {
		t.Errorf("PollInterval = %v, want 4s", cfg.PollInterval)
	}
}

// Все три переменные окружения заданы — берутся из env целиком.
func TestGetAgentConfig_FullEnv(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("ADDRESS", "localhost:7000")
	t.Setenv("REPORT_INTERVAL", "15")
	t.Setenv("POLL_INTERVAL", "6")

	cfg, err := GetAgentConfig(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Address != "localhost:7000" {
		t.Errorf("Address = %q, want localhost:7000", cfg.Address)
	}
	if cfg.ReportInterval != 15*time.Second {
		t.Errorf("ReportInterval = %v, want 15s", cfg.ReportInterval)
	}
	if cfg.PollInterval != 6*time.Second {
		t.Errorf("PollInterval = %v, want 6s", cfg.PollInterval)
	}
}

func TestGetAgentConfig_KeyFromFlag(t *testing.T) {
	clearAgentEnv(t)

	cfg, err := GetAgentConfig([]string{"-k=secret"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Key != "secret" {
		t.Errorf("Key = %q, want secret", cfg.Key)
	}
}

func TestGetAgentConfig_KeyFromEnvOnly(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("KEY", "secret")

	cfg, err := GetAgentConfig(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Key != "secret" {
		t.Errorf("Key = %q, want secret", cfg.Key)
	}
}

func TestGetAgentConfig_EnvKeyOverridesFlag(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("KEY", "secret")

	cfg, err := GetAgentConfig([]string{"-k=invalidkey"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Key != "secret" {
		t.Errorf("Key = %q, want secret: окружение обязано перекрывать флаг -k", cfg.Key)
	}
}

func TestGetAgentConfig_NoKey(t *testing.T) {
	clearAgentEnv(t)

	cfg, err := GetAgentConfig(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Key != "" {
		t.Errorf("Key = %q, want empty", cfg.Key)
	}
}
