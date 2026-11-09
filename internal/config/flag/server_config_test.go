package config

import "testing"

func TestParseServerFlags_Audit(t *testing.T) {
	cfg, err := ParseServerFlags([]string{"-audit-file=/tmp/audit.jsonl", "-audit-url=http://collector:9000/audit"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuditFile != "/tmp/audit.jsonl" {
		t.Errorf("AuditFile = %q, want /tmp/audit.jsonl", cfg.AuditFile)
	}
	if cfg.AuditURL != "http://collector:9000/audit" {
		t.Errorf("AuditURL = %q, want http://collector:9000/audit", cfg.AuditURL)
	}
}

func TestParseServerFlags_AuditDefaultsEmpty(t *testing.T) {
	cfg, err := ParseServerFlags(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuditFile != "" {
		t.Errorf("AuditFile = %q, want empty", cfg.AuditFile)
	}
	if cfg.AuditURL != "" {
		t.Errorf("AuditURL = %q, want empty", cfg.AuditURL)
	}
}
