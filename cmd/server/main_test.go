package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testLDFlags   = "-X main.buildVersion=v1.2.3 -X main.buildDate=2026-01-02T03:04:05Z -X main.buildCommit=abc1234"
	wantBuildInfo = "Build version: v1.2.3\nBuild date: 2026-01-02T03:04:05Z\nBuild commit: abc1234\n"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "server.exe")
	out, err := exec.Command("go", "build", "-buildvcs=false", "-ldflags", testLDFlags, "-o", bin, ".").CombinedOutput()
	require.NoError(t, err, string(out))
	return bin
}

func TestBuildInfoPrintedFirst(t *testing.T) {
	bin := buildBinary(t)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	stdout, err := exec.CommandContext(ctx, bin, "-unknown").Output()

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode())
	assert.Equal(t, wantBuildInfo, string(stdout))
}

func TestInvalidCryptoKeyStopsServer(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()

	missing := filepath.Join(dir, "missing.pem")
	broken := filepath.Join(dir, "broken.pem")
	require.NoError(t, os.WriteFile(broken, []byte("not a key"), 0o600))

	tests := []struct {
		name string
		args []string
		env  string
		path string
	}{
		{name: "missing file via flag", args: []string{"-crypto-key=" + missing}, path: missing},
		{name: "not PEM via env", env: broken, path: broken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()

			var stderr bytes.Buffer
			cmd := exec.CommandContext(ctx, bin, tt.args...)
			cmd.Env = append(os.Environ(), "CRYPTO_KEY="+tt.env, "DATABASE_DSN=", "AUDIT_FILE=", "AUDIT_URL=")
			cmd.Stderr = &stderr

			err := cmd.Run()

			var exitErr *exec.ExitError
			require.ErrorAs(t, err, &exitErr)
			assert.Equal(t, 1, exitErr.ExitCode())
			assert.Contains(t, stderr.String(), "crypto key")
			assert.Contains(t, stderr.String(), tt.path)
		})
	}
}
