package main

import (
	"os"
	"testing"
)

func TestUpToDate(t *testing.T) {
	t.Parallel()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := findModuleRoot(wd)
	if err != nil {
		t.Fatal(err)
	}
	changes, errs := plan(root)
	if len(errs) > 0 {
		t.Fatalf("generation over the module fails:\n%s", formatErrors(errs, root))
	}
	for _, c := range changes {
		action := "regenerate"
		if c.remove {
			action = "remove"
		}
		t.Errorf("%s is out of date: run go run ./cmd/reset to %s it", relSlash(t, root, c.path), action)
	}
}
