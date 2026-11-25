package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testLDFlags = "-X main.buildVersion=v1.2.3 -X main.buildDate=2026-01-02T03:04:05Z -X main.buildCommit=abc1234"

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "agent.exe")
	out, err := exec.Command("go", "build", "-buildvcs=false", "-ldflags", testLDFlags, "-o", bin, ".").CombinedOutput()
	require.NoError(t, err, string(out))
	return bin
}

func TestBuildInfoPrintedFirst(t *testing.T) {
	bin := buildBinary(t)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin)
	cmd.Env = append(os.Environ(), "ADDRESS=127.0.0.1:1")
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})

	var lines []string
	scanner := bufio.NewScanner(stdout)
	for len(lines) < 4 && scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	require.Len(t, lines, 4)
	assert.Equal(t, []string{
		"Build version: v1.2.3",
		"Build date: 2026-01-02T03:04:05Z",
		"Build commit: abc1234",
	}, lines[:3])
	assert.True(t, json.Valid([]byte(lines[3])), lines[3])
}

func TestMissingCryptoKeyStopsAgent(t *testing.T) {
	bin := buildBinary(t)
	missing := filepath.Join(t.TempDir(), "missing.pem")

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "-crypto-key="+missing)
	cmd.Env = append(os.Environ(), "ADDRESS=127.0.0.1:1", "CRYPTO_KEY=")
	cmd.Stderr = &stderr

	err := cmd.Run()

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode())
	assert.Contains(t, stderr.String(), "ключа шифрования")
	assert.Contains(t, stderr.String(), missing)
}
