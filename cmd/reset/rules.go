package main

import (
	"go/scanner"
	"go/token"
	"go/types"
)

type rule int

const (
	ruleCall rule = iota
	ruleBasic
	ruleNil
	ruleComposite
	ruleNew
	ruleSlice
	ruleMap
	rulePointer
	ruleEach
)

type expr struct {
	text  string
	deref bool
}

type stmt struct {
	line  string
	conds []string
	index string
	over  string
	body  []stmt
}

type body struct {
	qual  types.Qualifier
	index func(depth int) string
	used  map[string]bool
}

func newBody(qual types.Qualifier, index func(int) string) *body {
	return &body{qual: qual, index: index, used: map[string]bool{}}
}

func (e expr) operand() string {
	if e.deref {
		return "(" + e.text + ")"
	}
	return e.text
}

func (e expr) dot(name string) expr {
	return expr{text: e.operand() + "." + name}
}

func (e expr) at(index string) expr {
	return expr{text: e.operand() + "[" + index + "]"}
}

func (e expr) star() expr {
	return expr{text: "*" + e.text, deref: true}
}

func classify(t types.Type) rule {
	t = types.Unalias(t)
	if tp, ok := t.(*types.TypeParam); ok {
		if hasReset(tp) {
			return ruleCall
		}
		return ruleNew
	}
	if types.IsInterface(t) {
		return ruleNil
	}
	if _, ok := t.(*types.Named); ok && hasReset(t) {
		return ruleCall
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return ruleBasic
	case *types.Slice:
		return ruleSlice
	case *types.Map:
		return ruleMap
	case *types.Pointer:
		return rulePointer
	case *types.Struct:
		return ruleComposite
	case *types.Array:
		if isZero(classify(u.Elem())) {
			return ruleComposite
		}
		return ruleEach
	}
	return ruleNil
}

func isZero(r rule) bool {
	return r == ruleBasic || r == ruleNil || r == ruleComposite || r == ruleNew
}

func hasReset(t types.Type) bool {
	obj, _, _ := types.LookupFieldOrMethod(t, true, nil, "Reset")
	fn, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	sig := fn.Signature()
	return sig.Params().Len() == 0 && sig.Results().Len() == 0 && !sig.Variadic()
}

func directReset(t types.Type) bool {
	if _, ok := types.Unalias(t).(*types.Named); !ok {
		return false
	}
	return !types.IsInterface(t) && hasReset(t)
}

func hasPointers(t types.Type) bool {
	if _, ok := types.Unalias(t).(*types.TypeParam); ok {
		return true
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return u.Info()&types.IsString != 0 || u.Kind() == types.UnsafePointer
	case *types.Struct:
		for i := range u.NumFields() {
			if hasPointers(u.Field(i).Type()) {
				return true
			}
		}
		return false
	case *types.Array:
		return u.Len() > 0 && hasPointers(u.Elem())
	}
	return true
}

func (b *body) fields(recv expr, st *types.Struct) []stmt {
	var out []stmt
	for i := range st.NumFields() {
		v := st.Field(i)
		if v.Name() == "_" {
			continue
		}
		out = append(out, b.reset(recv.dot(v.Name()), v.Type(), 0)...)
	}
	return out
}

func (b *body) reset(e expr, t types.Type, depth int) []stmt {
	switch classify(t) {
	case ruleCall:
		return b.call(e, t)
	case ruleBasic:
		return []stmt{assign(e, b.basicZero(t))}
	case ruleNil:
		return []stmt{assign(e, b.builtin("nil"))}
	case ruleComposite:
		return []stmt{assign(e, b.typeString(t)+"{}")}
	case ruleNew:
		return []stmt{assign(e, "*"+b.builtin("new")+"("+b.typeString(t)+")")}
	case ruleSlice:
		var out []stmt
		if hasPointers(t.Underlying().(*types.Slice).Elem()) {
			out = append(out, stmt{line: b.builtin("clear") + "(" + e.text + ")"})
		}
		return append(out, assign(e, e.operand()+"[:0]"))
	case ruleMap:
		return []stmt{{line: b.builtin("clear") + "(" + e.text + ")"}}
	case rulePointer:
		return b.pointer(e, t, depth)
	case ruleEach:
		index := b.index(depth)
		elem := t.Underlying().(*types.Array).Elem()
		return []stmt{{index: index, over: e.text, body: b.reset(e.at(index), elem, depth+1)}}
	}
	return nil
}

func (b *body) pointer(e expr, t types.Type, depth int) []stmt {
	elem := t.Underlying().(*types.Pointer).Elem()
	var inner []stmt
	if _, unnamed := types.Unalias(t).(*types.Pointer); unnamed && directReset(elem) {
		inner = b.call(e, elem)
	} else {
		inner = b.reset(e.star(), elem, depth)
	}
	cond := e.text + " != " + b.builtin("nil")
	if len(inner) == 1 && inner[0].conds != nil {
		return []stmt{{conds: append([]string{cond}, inner[0].conds...), body: inner[0].body}}
	}
	return []stmt{{conds: []string{cond}, body: inner}}
}

func (b *body) call(e expr, t types.Type) []stmt {
	call := stmt{line: e.operand() + ".Reset()"}
	conds := b.embedded(e, t)
	if len(conds) == 0 {
		return []stmt{call}
	}
	return []stmt{{conds: conds, body: []stmt{call}}}
}

func (b *body) embedded(e expr, t types.Type) []string {
	t = types.Unalias(t)
	if _, ok := t.(*types.TypeParam); ok {
		return nil
	}
	_, index, _ := types.LookupFieldOrMethod(t, true, nil, "Reset")
	if len(index) < 2 {
		return nil
	}
	var conds []string
	for _, i := range index[:len(index)-1] {
		st, ok := t.Underlying().(*types.Struct)
		if !ok {
			break
		}
		field := st.Field(i)
		e = e.dot(field.Name())
		t = types.Unalias(field.Type())
		if p, ok := t.(*types.Pointer); ok {
			conds = append(conds, e.text+" != "+b.builtin("nil"))
			t = types.Unalias(p.Elem())
		} else if types.IsInterface(t) {
			conds = append(conds, e.text+" != "+b.builtin("nil"))
		}
	}
	return conds
}

func assign(e expr, value string) stmt {
	return stmt{line: e.text + " = " + value}
}

func (b *body) basicZero(t types.Type) string {
	u := t.Underlying().(*types.Basic)
	switch {
	case u.Info()&types.IsBoolean != 0:
		return b.builtin("false")
	case u.Info()&types.IsString != 0:
		return `""`
	case u.Kind() == types.UnsafePointer:
		return b.builtin("nil")
	}
	return "0"
}

func (b *body) builtin(name string) string {
	b.used[name] = true
	return name
}

func (b *body) typeString(t types.Type) string {
	s := types.TypeString(t, b.qual)
	for _, id := range identifiers(s) {
		b.used[id] = true
	}
	return s
}

func identifiers(src string) []string {
	fset := token.NewFileSet()
	file := fset.AddFile("", -1, len(src))
	var s scanner.Scanner
	s.Init(file, []byte(src), nil, 0)
	var out []string
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			return out
		}
		if tok == token.IDENT {
			out = append(out, lit)
		}
	}
}
