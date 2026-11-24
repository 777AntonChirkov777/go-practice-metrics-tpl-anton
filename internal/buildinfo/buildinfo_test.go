package buildinfo

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrint(t *testing.T) {
	tests := []struct {
		name    string
		version string
		date    string
		commit  string
		want    string
	}{
		{
			name:    "all values set",
			version: "v1.0.0",
			date:    "2026-11-23T10:00:00Z",
			commit:  "1ffabbc",
			want:    "Build version: v1.0.0\nBuild date: 2026-11-23T10:00:00Z\nBuild commit: 1ffabbc\n",
		},
		{
			name: "no values set",
			want: "Build version: N/A\nBuild date: N/A\nBuild commit: N/A\n",
		},
		{
			name:    "only version set",
			version: "v1.0.0",
			want:    "Build version: v1.0.0\nBuild date: N/A\nBuild commit: N/A\n",
		},
		{
			name: "only date set",
			date: "2026-11-23T10:00:00Z",
			want: "Build version: N/A\nBuild date: 2026-11-23T10:00:00Z\nBuild commit: N/A\n",
		},
		{
			name:   "only commit set",
			commit: "1ffabbc",
			want:   "Build version: N/A\nBuild date: N/A\nBuild commit: 1ffabbc\n",
		},
		{
			name: "whitespace value printed as is",
			date: "   ",
			want: "Build version: N/A\nBuild date: " + "   " + "\nBuild commit: N/A\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			Print(&buf, tt.version, tt.date, tt.commit)
			assert.Equal(t, tt.want, buf.String())
		})
	}
}

type failingWriter struct {
	calls int
}

func (w *failingWriter) Write([]byte) (int, error) {
	w.calls++
	return 0, errors.New("write failed")
}

func TestPrintIgnoresWriteError(t *testing.T) {
	w := &failingWriter{}

	assert.NotPanics(t, func() {
		Print(w, "v1.0.0", "2026-11-23T10:00:00Z", "1ffabbc")
	})
	assert.Positive(t, w.calls)
}
