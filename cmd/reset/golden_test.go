package main

import (
	"bytes"
	"flag"
	"go/format"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const goldenExt = ".golden"

var update = flag.Bool("update", false, "rewrite testdata/golden/**/reset.gen.go.golden from the generator output")

func TestGolden(t *testing.T) {
	dir := filepath.Join("testdata", "golden")
	for _, name := range fixtureCases(t, dir) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src := filepath.Join(dir, name)
			root := copyFixture(t, src)
			before := readTree(t, root, genFile)

			changes, errs := plan(root)
			if len(errs) > 0 {
				t.Fatalf("plan reported errors:\n%s", formatErrors(errs, root))
			}
			for _, c := range changes {
				if rel := relSlash(t, root, c.path); !c.remove && bytes.Equal(before[rel], c.content) {
					t.Errorf("plan rewrites %s with unchanged content", rel)
				}
			}
			if err := apply(changes); err != nil {
				t.Fatal(err)
			}
			got := readTree(t, root, genFile)

			if *update {
				writeGolden(t, src, got)
				return
			}

			want := readGolden(t, src)
			for _, rel := range slices.Sorted(maps.Keys(want)) {
				content, ok := got[rel]
				switch {
				case !ok:
					t.Errorf("%s was not generated", rel)
				case !bytes.Equal(content, want[rel]):
					t.Errorf("%s differs from %s%s\n--- got ---\n%s\n--- want ---\n%s", rel, rel, goldenExt, content, want[rel])
				}
			}
			for _, rel := range slices.Sorted(maps.Keys(got)) {
				if _, ok := want[rel]; !ok {
					t.Errorf("unexpected generated file %s", rel)
				}
				if formatted, err := format.Source(got[rel]); err != nil || !bytes.Equal(formatted, got[rel]) {
					t.Errorf("%s is not gofmt-formatted (err %v)", rel, err)
				}
			}

			again, errs := plan(root)
			if len(errs) > 0 {
				t.Fatalf("second plan reported errors:\n%s", formatErrors(errs, root))
			}
			for _, c := range again {
				t.Errorf("second run changes %s", relSlash(t, root, c.path))
			}
		})
	}
}

func TestRunFromSubdir(t *testing.T) {
	if *update {
		t.Skip("golden files are being rewritten")
	}
	t.Parallel()
	src := filepath.Join("testdata", "golden", "markers")
	root := copyFixture(t, src)
	var stderr bytes.Buffer
	if code := run(filepath.Join(root, "internal", "x", "y"), &stderr); code != 0 || stderr.Len() > 0 {
		t.Fatalf("run exit code = %d, want 0 and no output; output:\n%s", code, stderr.String())
	}
	got, want := readTree(t, root, genFile), readGolden(t, src)
	if !maps.EqualFunc(got, want, bytes.Equal) {
		t.Errorf("run from a subdirectory generated %v, want %v", slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(want)))
	}
}

func TestVendorSkipped(t *testing.T) {
	if *update {
		t.Skip("golden files are being rewritten")
	}
	t.Parallel()
	src := filepath.Join("testdata", "golden", "markers")
	root := copyFixture(t, src)
	vendored := filepath.Join(root, "vendor", "v")
	if err := os.MkdirAll(vendored, 0o755); err != nil {
		t.Fatal(err)
	}
	marked := "package v\n\n// generate:reset\ntype V struct {\n\tn int\n}\n"
	if err := os.WriteFile(filepath.Join(vendored, "v.go"), []byte(marked), 0o644); err != nil {
		t.Fatal(err)
	}
	changes, errs := plan(root)
	if len(errs) > 0 {
		t.Fatalf("plan reported errors:\n%s", formatErrors(errs, root))
	}
	if err := apply(changes); err != nil {
		t.Fatal(err)
	}
	got, want := readTree(t, root, genFile), readGolden(t, src)
	if _, ok := got["vendor/v/"+genFile]; ok {
		t.Errorf("vendor/v/%s was generated, want vendor/ to be skipped", genFile)
	}
	if !maps.EqualFunc(got, want, bytes.Equal) {
		t.Errorf("generated %v, want %v", slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(want)))
	}
}

func fixtureCases(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		t.Fatalf("no fixtures in %s", dir)
	}
	return names
}

func copyFixture(t *testing.T, src string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(root, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if strings.HasSuffix(d.Name(), goldenExt) || (d.Name() == "want.txt" && rel == d.Name()) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func readTree(t *testing.T, root, base string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || (base != "" && d.Name() != base) {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[relSlash(t, root, path)] = data
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func readGolden(t *testing.T, src string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for rel, data := range readTree(t, src, genFile+goldenExt) {
		out[strings.TrimSuffix(rel, goldenExt)] = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	}
	return out
}

func writeGolden(t *testing.T, src string, got map[string][]byte) {
	t.Helper()
	for rel := range readTree(t, src, genFile+goldenExt) {
		if err := os.Remove(filepath.Join(src, filepath.FromSlash(rel))); err != nil {
			t.Fatal(err)
		}
	}
	for rel, data := range got {
		if err := os.WriteFile(filepath.Join(src, filepath.FromSlash(rel)+goldenExt), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func relSlash(t *testing.T, root, path string) string {
	t.Helper()
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(rel)
}

func formatErrors(errs []error, base string) string {
	return strings.Join(errorLines(errs, base), "\n")
}

func errorLines(errs []error, base string) []string {
	lines := make([]string, len(errs))
	for i, err := range errs {
		lines[i] = filepath.ToSlash(relativeError(err, base))
	}
	return lines
}
