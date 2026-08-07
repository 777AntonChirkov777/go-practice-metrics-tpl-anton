package config

import (
	"os"
	"testing"
	"time"
)

var serverEnvKeys = []string{"ADDRESS", "STORE_INTERVAL", "FILE_STORAGE_PATH", "RESTORE", "DATABASE_DSN"}

func resetServerEnv(t *testing.T) {
	t.Helper()
	saved := make(map[string]*string, len(serverEnvKeys))
	for _, k := range serverEnvKeys {
		if v, ok := os.LookupEnv(k); ok {
			vv := v
			saved[k] = &vv
		}
		_ = os.Unsetenv(k)
	}
	t.Cleanup(func() {
		for _, k := range serverEnvKeys {
			if v, ok := saved[k]; ok {
				_ = os.Setenv(k, *v)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	})
}

func TestGetServerConfig_Defaults(t *testing.T) {
	resetServerEnv(t)

	cfg, err := GetServerConfig(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Address != "localhost:8080" {
		t.Errorf("Address = %q, want localhost:8080", cfg.Address)
	}
	if cfg.StoreInterval != 300*time.Second {
		t.Errorf("StoreInterval = %v, want 300s", cfg.StoreInterval)
	}
	if cfg.FileStoragePath != "metrics-db.json" {
		t.Errorf("FileStoragePath = %q, want metrics-db.json", cfg.FileStoragePath)
	}
	if !cfg.Restore {
		t.Errorf("Restore = %v, want true", cfg.Restore)
	}
	if cfg.DatabaseDSN != "" {
		t.Errorf("DatabaseDSN = %q, want empty", cfg.DatabaseDSN)
	}
}

func TestGetServerConfig_FlagsOnly(t *testing.T) {
	resetServerEnv(t)

	cfg, err := GetServerConfig([]string{
		"-a=localhost:9999", "-i=10", "-f=/tmp/x.json", "-r=false",
		"-d=postgres://flag:flag@localhost:5432/praktikum?sslmode=disable",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Address != "localhost:9999" {
		t.Errorf("Address = %q, want localhost:9999", cfg.Address)
	}
	if cfg.StoreInterval != 10*time.Second {
		t.Errorf("StoreInterval = %v, want 10s", cfg.StoreInterval)
	}
	if cfg.FileStoragePath != "/tmp/x.json" {
		t.Errorf("FileStoragePath = %q, want /tmp/x.json", cfg.FileStoragePath)
	}
	if cfg.Restore {
		t.Errorf("Restore = %v, want false (from flag)", cfg.Restore)
	}
	if cfg.DatabaseDSN != "postgres://flag:flag@localhost:5432/praktikum?sslmode=disable" {
		t.Errorf("DatabaseDSN = %q, want value from -d flag", cfg.DatabaseDSN)
	}
}

// Флаг -i=0 должен давать синхронный режим (0), а не подменяться дефолтом.
func TestGetServerConfig_FlagZeroInterval(t *testing.T) {
	resetServerEnv(t)

	cfg, err := GetServerConfig([]string{"-i=0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.StoreInterval != 0 {
		t.Errorf("StoreInterval = %v, want 0 (synchronous)", cfg.StoreInterval)
	}
}

// Приоритет env над флагами, но только для заданных в env полей.
func TestGetServerConfig_EnvOverridesFlagsPerField(t *testing.T) {
	resetServerEnv(t)
	_ = os.Setenv("ADDRESS", "localhost:7000")
	_ = os.Setenv("STORE_INTERVAL", "5")
	// FILE_STORAGE_PATH и RESTORE в env не заданы — должны прийти из флагов.

	cfg, err := GetServerConfig([]string{
		"-a=localhost:1111", "-i=99", "-f=/flag/path.json", "-r=false",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Address != "localhost:7000" {
		t.Errorf("Address = %q, want localhost:7000 (env wins)", cfg.Address)
	}
	if cfg.StoreInterval != 5*time.Second {
		t.Errorf("StoreInterval = %v, want 5s (env wins)", cfg.StoreInterval)
	}
	if cfg.FileStoragePath != "/flag/path.json" {
		t.Errorf("FileStoragePath = %q, want /flag/path.json (from flag)", cfg.FileStoragePath)
	}
	if cfg.Restore {
		t.Errorf("Restore = %v, want false (from flag)", cfg.Restore)
	}
}

// STORE_INTERVAL=0 из окружения — это осмысленный ноль (синхронно), а не «не задано».
func TestGetServerConfig_EnvZeroInterval(t *testing.T) {
	resetServerEnv(t)
	_ = os.Setenv("STORE_INTERVAL", "0")

	// Флаг задаёт ненулевой интервал — env=0 обязан победить.
	cfg, err := GetServerConfig([]string{"-i=300"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.StoreInterval != 0 {
		t.Errorf("StoreInterval = %v, want 0 (env=0 must win)", cfg.StoreInterval)
	}
}

// RESTORE=false из окружения — осмысленное false, а не «не задано».
func TestGetServerConfig_EnvRestoreFalse(t *testing.T) {
	resetServerEnv(t)
	_ = os.Setenv("RESTORE", "false")

	cfg, err := GetServerConfig(nil) // дефолт Restore = true
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Restore {
		t.Errorf("Restore = %v, want false (env=false must win over default true)", cfg.Restore)
	}
}

func TestGetServerConfig_EnvDatabaseDSNOverridesFlag(t *testing.T) {
	resetServerEnv(t)
	_ = os.Setenv("DATABASE_DSN", "postgres://env:env@localhost:5432/praktikum?sslmode=disable")

	cfg, err := GetServerConfig([]string{"-d=postgres://flag:flag@localhost:5432/praktikum?sslmode=disable"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DatabaseDSN != "postgres://env:env@localhost:5432/praktikum?sslmode=disable" {
		t.Errorf("DatabaseDSN = %q, want value from env (env must win over -d)", cfg.DatabaseDSN)
	}
}

func TestGetServerConfig_EmptyEnvDatabaseDSNKeepsFlag(t *testing.T) {
	resetServerEnv(t)
	_ = os.Setenv("DATABASE_DSN", "")

	cfg, err := GetServerConfig([]string{"-d=postgres://flag:flag@localhost:5432/praktikum?sslmode=disable"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DatabaseDSN != "postgres://flag:flag@localhost:5432/praktikum?sslmode=disable" {
		t.Errorf("DatabaseDSN = %q, want value from -d flag (empty env is ignored)", cfg.DatabaseDSN)
	}
}

// Полный «автотестовый» сценарий: конфигурация только из env, без флагов.
func TestGetServerConfig_EnvOnlyLikeAutotest(t *testing.T) {
	resetServerEnv(t)
	_ = os.Setenv("ADDRESS", "localhost:33807")
	_ = os.Setenv("RESTORE", "true")
	_ = os.Setenv("STORE_INTERVAL", "2")
	_ = os.Setenv("FILE_STORAGE_PATH", "/tmp/metrics-db.json")
	_ = os.Setenv("DATABASE_DSN", "postgres://postgres:postgres@postgres:5432/praktikum?sslmode=disable")

	cfg, err := GetServerConfig(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Address != "localhost:33807" {
		t.Errorf("Address = %q, want localhost:33807", cfg.Address)
	}
	if cfg.StoreInterval != 2*time.Second {
		t.Errorf("StoreInterval = %v, want 2s", cfg.StoreInterval)
	}
	if cfg.FileStoragePath != "/tmp/metrics-db.json" {
		t.Errorf("FileStoragePath = %q, want /tmp/metrics-db.json", cfg.FileStoragePath)
	}
	if !cfg.Restore {
		t.Errorf("Restore = %v, want true", cfg.Restore)
	}
	if cfg.DatabaseDSN != "postgres://postgres:postgres@postgres:5432/praktikum?sslmode=disable" {
		t.Errorf("DatabaseDSN = %q, want value from env", cfg.DatabaseDSN)
	}
}
