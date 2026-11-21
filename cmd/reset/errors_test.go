package main

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestErrors(t *testing.T) {
	dir := filepath.Join("testdata", "errors")
	for _, name := range fixtureCases(t, dir) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src := filepath.Join(dir, name)
			want := readLines(t, filepath.Join(src, "want.txt"))
			root := copyFixture(t, src)
			before := readTree(t, root, "")

			if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
				changes, errs := plan(root)
				if len(changes) > 0 {
					t.Errorf("plan has %d changes, want none", len(changes))
				}
				matchErrors(t, "plan", errorLines(errs, root), want)
			} else if _, err := findModuleRoot(root); err == nil {
				t.Skip("the temporary directory is inside a Go module")
			}

			var stderr bytes.Buffer
			if code := run(root, &stderr); code != 1 {
				t.Errorf("run exit code = %d, want 1", code)
			}
			matchErrors(t, "run", readOutput(stderr.String()), want)

			if after := readTree(t, root, ""); !maps.EqualFunc(before, after, bytes.Equal) {
				t.Error("files of the module changed after a failed run")
			}
		})
	}
}

func TestParseError(t *testing.T) {
	t.Parallel()
	root := copyFixture(t, filepath.Join("testdata", "golden", "tz"))
	if err := os.WriteFile(filepath.Join(root, "broken.go"), []byte("package tz\n\nfunc broken() {\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := readTree(t, root, "")
	want := []string{"broken.go:3:17: expected '}', found 'EOF'"}

	changes, errs := plan(root)
	if len(changes) > 0 {
		t.Errorf("plan has %d changes, want none", len(changes))
	}
	matchErrors(t, "plan", errorLines(errs, root), want)

	var stderr bytes.Buffer
	if code := run(root, &stderr); code != 1 {
		t.Errorf("run exit code = %d, want 1", code)
	}
	matchErrors(t, "run", readOutput(stderr.String()), want)

	if after := readTree(t, root, ""); !maps.EqualFunc(before, after, bytes.Equal) {
		t.Error("files of the module changed after a failed run")
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return readOutput(string(data))
}

func readOutput(s string) []string {
	s = strings.TrimRight(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = filepath.ToSlash(line)
	}
	return lines
}

func matchErrors(t *testing.T, what string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: got %d errors, want %d:\n%s", what, len(got), len(want), strings.Join(got, "\n"))
		return
	}
	for i := range want {
		if !strings.Contains(got[i], want[i]) {
			t.Errorf("%s: error %d = %q, want it to contain %q", what, i+1, got[i], want[i])
		}
	}
}
