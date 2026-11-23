package main

import (
	"context"
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

func TestBuildInfoPrintedFirst(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "server.exe")
	out, err := exec.Command("go", "build", "-buildvcs=false", "-ldflags", testLDFlags, "-o", bin, ".").CombinedOutput()
	require.NoError(t, err, string(out))

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	stdout, err := exec.CommandContext(ctx, bin, "-unknown").Output()

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode())
	assert.Equal(t, wantBuildInfo, string(stdout))
}
