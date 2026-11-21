package main

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

type change struct {
	path    string
	content []byte
	remove  bool
}

func main() {
	if code := run(".", os.Stderr); code != 0 {
		os.Exit(code)
	}
}

func run(dir string, stderr io.Writer) int {
	wd, err := filepath.Abs(dir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	root, err := findModuleRoot(wd)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	changes, errs := plan(root)
	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintln(stderr, relativeError(err, wd))
		}
		return 1
	}
	if err := apply(changes); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func findModuleRoot(dir string) (string, error) {
	for d := dir; ; {
		if info, err := os.Stat(filepath.Join(d, "go.mod")); err == nil && !info.IsDir() {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", fmt.Errorf("go.mod not found in %s or any parent directory", dir)
		}
		d = parent
	}
}

func plan(root string) ([]change, []error) {
	changes, errs := build(root)
	if len(errs) > 0 {
		slices.SortStableFunc(errs, compareErrors)
		errs = slices.Compact(errs)
		out := make([]error, len(errs))
		for i, e := range errs {
			out[i] = e
		}
		return nil, out
	}
	slices.SortFunc(changes, func(a, b change) int { return cmp.Compare(a.path, b.path) })
	return changes, nil
}

func build(root string) ([]change, []genError) {
	mod, errs := scan(root)
	if mod == nil || len(errs) > 0 {
		return nil, errs
	}
	fset, typed, errs := typecheck(root, stubs(mod.pkgs))
	if len(errs) > 0 {
		return nil, errs
	}
	var (
		changes   []change
		overlay   = map[string][]byte{}
		generated bool
	)
	for _, p := range mod.pkgs {
		file := filepath.Join(p.dir, genFile)
		if len(p.types) == 0 {
			if p.genOurs {
				changes = append(changes, change{path: file, remove: true})
				overlay[file] = []byte("package " + p.name + "\n")
			}
			continue
		}
		tpkg := typed[p.path]
		if tpkg == nil {
			errs = append(errs, genError{file: p.dir, msg: fmt.Sprintf("package %s was not type-checked", p.path)})
			continue
		}
		content, rerrs := render(fset, mod.path, p, tpkg)
		if len(rerrs) > 0 {
			errs = append(errs, rerrs...)
			continue
		}
		overlay[file] = content
		generated = true
		old, err := os.ReadFile(file)
		switch {
		case err == nil && bytes.Equal(old, content):
			continue
		case err != nil && !errors.Is(err, fs.ErrNotExist):
			errs = append(errs, genError{file: file, msg: err.Error()})
			continue
		}
		changes = append(changes, change{path: file, content: content})
	}
	if len(errs) > 0 {
		return nil, errs
	}
	if generated {
		if errs := verify(root, overlay); len(errs) > 0 {
			return nil, errs
		}
	}
	return changes, nil
}

func apply(changes []change) error {
	for _, c := range changes {
		if c.remove {
			if err := os.Remove(c.path); err != nil {
				return err
			}
			continue
		}
		if err := os.WriteFile(c.path, c.content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func relativeError(err error, base string) string {
	var ge genError
	if errors.As(err, &ge) {
		return ge.relative(base)
	}
	return err.Error()
}
