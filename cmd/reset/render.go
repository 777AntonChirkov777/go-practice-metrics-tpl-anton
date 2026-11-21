package main

import (
	"bytes"
	"cmp"
	"fmt"
	"go/format"
	"go/token"
	"go/types"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

type unit struct {
	pkg     *types.Package
	modPath string
	names   map[string]string
	refs    map[string]*types.Package
}

type target struct {
	mt     *markedType
	named  *types.Named
	strukt *types.Struct
}

func render(fset *token.FileSet, modPath string, p *pkgInfo, tpkg *types.Package) ([]byte, []genError) {
	file := filepath.Join(p.dir, genFile)
	marked := slices.Clone(p.types)
	slices.SortFunc(marked, func(a, b *markedType) int {
		return cmp.Or(cmp.Compare(a.file, b.file), cmp.Compare(a.offset, b.offset))
	})
	var (
		targets []target
		errs    []genError
	)
	for _, mt := range marked {
		tn, _ := tpkg.Scope().Lookup(mt.name).(*types.TypeName)
		var named *types.Named
		if tn != nil {
			named, _ = tn.Type().(*types.Named)
		}
		var st *types.Struct
		if named != nil {
			st, _ = named.Underlying().(*types.Struct)
		}
		if st == nil {
			errs = append(errs, genError{file: mt.file, msg: fmt.Sprintf("%s is not a struct type after type checking", mt.name)})
			continue
		}
		targets = append(targets, target{mt: mt, named: named, strukt: st})
	}
	if len(errs) > 0 {
		return nil, errs
	}
	for _, t := range targets {
		tps := t.named.TypeParams()
		for i := range tps.Len() {
			if obj := tps.At(i).Obj(); types.Universe.Lookup(obj.Name()) != nil {
				errs = append(errs, posError(fset, obj.Pos(), "cannot generate Reset for %s: type parameter %s has the name of a predeclared identifier", t.mt.name, obj.Name()))
			}
		}
	}

	u := &unit{pkg: tpkg, modPath: modPath, names: map[string]string{}, refs: map[string]*types.Package{}}
	avoid := map[string]bool{}
	for _, t := range targets {
		for _, name := range typeParams(t.named) {
			avoid[name] = true
		}
		newBody(u.qualify, placeholderIndex).fields(expr{text: "_"}, t.strukt)
	}
	u.assignNames(avoid)

	used := map[string]bool{"nil": true}
	methods := make([]string, 0, len(targets))
	for _, t := range targets {
		text, names := u.method(t)
		methods = append(methods, text)
		for name := range names {
			used[name] = true
		}
	}
	for _, name := range slices.Sorted(maps.Keys(used)) {
		if types.Universe.Lookup(name) == nil {
			continue
		}
		if obj := tpkg.Scope().Lookup(name); obj != nil {
			errs = append(errs, posError(fset, obj.Pos(), "package-level %s shadows the predeclared %s used by the generated Reset methods", name, name))
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}

	var src bytes.Buffer
	fmt.Fprintf(&src, "%s\n\npackage %s\n\n", genHeader, tpkg.Name())
	if imports := u.imports(); imports != "" {
		src.WriteString(imports + "\n")
	}
	src.WriteString(strings.Join(methods, "\n"))
	out, err := format.Source(src.Bytes())
	if err != nil {
		return nil, []genError{{file: file, msg: "formatting generated code: " + err.Error()}}
	}
	return out, nil
}

func placeholderIndex(int) string {
	return "_"
}

func typeParams(n *types.Named) []string {
	tps := n.TypeParams()
	out := make([]string, tps.Len())
	for i := range tps.Len() {
		out[i] = tps.At(i).Obj().Name()
	}
	return out
}

func (u *unit) qualify(p *types.Package) string {
	if p.Path() == u.pkg.Path() {
		return ""
	}
	u.refs[p.Path()] = p
	if name, ok := u.names[p.Path()]; ok {
		return name
	}
	return p.Name()
}

func (u *unit) assignNames(avoid map[string]bool) {
	taken := map[string]bool{}
	for _, importPath := range slices.Sorted(maps.Keys(u.refs)) {
		base := u.refs[importPath].Name()
		name := base
		for n := 2; u.conflicts(name, taken, avoid); n++ {
			name = base + strconv.Itoa(n)
		}
		taken[name] = true
		u.names[importPath] = name
	}
}

func (u *unit) conflicts(name string, taken, avoid map[string]bool) bool {
	return taken[name] || avoid[name] || u.pkg.Scope().Lookup(name) != nil || types.Universe.Lookup(name) != nil
}

func (u *unit) imports() string {
	var groups [3][]string
	for _, importPath := range slices.Sorted(maps.Keys(u.refs)) {
		spec := strconv.Quote(importPath)
		if name := u.names[importPath]; name != path.Base(importPath) {
			spec = name + " " + spec
		}
		g := importGroup(u.modPath, importPath)
		groups[g] = append(groups[g], spec)
	}
	var lines []string
	for _, g := range groups {
		if len(g) == 0 {
			continue
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, g...)
	}
	switch len(lines) {
	case 0:
		return ""
	case 1:
		return "import " + lines[0] + "\n"
	}
	var b strings.Builder
	b.WriteString("import (\n")
	for _, line := range lines {
		if line != "" {
			b.WriteString("\t" + line)
		}
		b.WriteString("\n")
	}
	b.WriteString(")\n")
	return b.String()
}

func importGroup(modPath, importPath string) int {
	if modPath != "" && (importPath == modPath || strings.HasPrefix(importPath, modPath+"/")) {
		return 2
	}
	first, _, _ := strings.Cut(importPath, "/")
	if strings.Contains(first, ".") {
		return 1
	}
	return 0
}

func (u *unit) method(t target) (string, map[string]bool) {
	params := typeParams(t.named)
	probe := newBody(u.qualify, placeholderIndex)
	probe.fields(expr{text: "_"}, t.strukt)
	reserved := maps.Clone(probe.used)
	reserved["nil"] = true
	for _, name := range params {
		reserved[name] = true
	}
	recv := receiverName(t.mt, reserved)
	reserved[recv] = true

	b := newBody(u.qualify, indexNames(reserved))
	stmts := b.fields(expr{text: recv}, t.strukt)

	typeArgs := ""
	if len(params) > 0 {
		typeArgs = "[" + strings.Join(params, ", ") + "]"
	}
	var w strings.Builder
	fmt.Fprintf(&w, "func (%s *%s%s) Reset() {\n\tif %s == nil {\n\t\treturn\n\t}\n", recv, t.mt.name, typeArgs, recv)
	if len(stmts) > 0 {
		w.WriteString("\n")
		writeStmts(&w, stmts, 1)
	}
	w.WriteString("}\n")
	return w.String(), b.used
}

func writeStmts(w *strings.Builder, stmts []stmt, depth int) {
	tabs := strings.Repeat("\t", depth)
	for _, s := range stmts {
		switch {
		case s.conds != nil:
			fmt.Fprintf(w, "%sif %s {\n", tabs, strings.Join(s.conds, " && "))
			writeStmts(w, s.body, depth+1)
			fmt.Fprintf(w, "%s}\n", tabs)
		case s.index != "":
			fmt.Fprintf(w, "%sfor %s := range %s {\n", tabs, s.index, s.over)
			writeStmts(w, s.body, depth+1)
			fmt.Fprintf(w, "%s}\n", tabs)
		default:
			fmt.Fprintf(w, "%s%s\n", tabs, s.line)
		}
	}
}

func receiverName(mt *markedType, reserved map[string]bool) string {
	primary := mt.recv
	if primary == "" || primary == "_" {
		primary = initials(mt.name)
	}
	for _, name := range []string{primary, "r", "x", "v", "rcv"} {
		if usable(name, reserved) {
			return name
		}
	}
	for n := 2; ; n++ {
		if name := "rcv" + strconv.Itoa(n); usable(name, reserved) {
			return name
		}
	}
}

func usable(name string, reserved map[string]bool) bool {
	return name != "" && name != "_" && !token.IsKeyword(name) && !reserved[name]
}

func initials(name string) string {
	runes := []rune(name)
	var out []rune
	for i, r := range runes {
		if !unicode.IsLetter(r) {
			continue
		}
		start := i == 0 || runes[i-1] == '_'
		if !start && unicode.IsUpper(r) {
			prev := runes[i-1]
			next := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			start = unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && next)
		}
		if start {
			out = append(out, unicode.ToLower(r))
		}
	}
	return string(out)
}

func indexNames(reserved map[string]bool) func(int) string {
	var names []string
	next := 0
	return func(depth int) string {
		for len(names) <= depth {
			name := indexCandidate(next)
			next++
			if !reserved[name] {
				names = append(names, name)
			}
		}
		return names[depth]
	}
}

func indexCandidate(n int) string {
	const letters = "ijklmnopqrstuvwxyz"
	if n < len(letters) {
		return letters[n : n+1]
	}
	return "i" + strconv.Itoa(n-len(letters)+2)
}
