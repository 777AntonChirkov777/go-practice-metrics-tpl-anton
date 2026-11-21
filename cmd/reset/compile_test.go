package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompile(t *testing.T) {
	dir := filepath.Join("testdata", "golden")
	for _, name := range fixtureCases(t, dir) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := copyFixture(t, filepath.Join(dir, name))
			changes, errs := plan(root)
			if len(errs) > 0 {
				t.Fatalf("plan reported errors:\n%s", formatErrors(errs, root))
			}
			if err := apply(changes); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{
				{"build", "./..."},
				{"vet", "./..."},
				{"test", "-count=1", "./..."},
			} {
				cmd := exec.Command("go", args...)
				cmd.Dir = root
				cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local")
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
				}
			}
		})
	}
}
