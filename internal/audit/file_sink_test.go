package audit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readAuditLines(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

func TestFileSink_OneLinePerEvent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")

	sink, err := NewFileSink(path)
	require.NoError(t, err)

	require.NoError(t, sink.Notify(context.Background(), Event{
		Ts: 1, Metrics: []string{"Alloc"}, IPAddress: "192.168.0.42",
	}))
	require.NoError(t, sink.Notify(context.Background(), Event{
		Ts: 2, Metrics: []string{"Frees", "PollCount"}, IPAddress: "10.0.0.7",
	}))
	require.NoError(t, sink.Close())

	lines := readAuditLines(t, path)
	require.Len(t, lines, 2)

	var first Event
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &first))
	assert.Equal(t, int64(1), first.Ts)
	assert.Equal(t, []string{"Alloc"}, first.Metrics)
	assert.Equal(t, "192.168.0.42", first.IPAddress)

	var second Event
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &second))
	assert.Equal(t, int64(2), second.Ts)
	assert.Equal(t, []string{"Frees", "PollCount"}, second.Metrics)
	assert.Equal(t, "10.0.0.7", second.IPAddress)
}

func TestFileSink_AppendsToExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	existing := `{"ts":1,"metrics":["Old"],"ip_address":"127.0.0.1"}` + "\n"
	require.NoError(t, os.WriteFile(path, []byte(existing), 0o644))

	sink, err := NewFileSink(path)
	require.NoError(t, err)
	require.NoError(t, sink.Notify(context.Background(), Event{
		Ts: 2, Metrics: []string{"New"}, IPAddress: "10.0.0.7",
	}))
	require.NoError(t, sink.Close())

	lines := readAuditLines(t, path)
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], `"Old"`)
	assert.Contains(t, lines[1], `"New"`)
}

func TestFileSink_RejectsUnopenablePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-dir", "audit.jsonl")

	sink, err := NewFileSink(path)
	require.Error(t, err)
	assert.Nil(t, sink)
}
