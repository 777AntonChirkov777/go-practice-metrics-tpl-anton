package fields

import (
	"bytes"
	"crypto/sha256"
	stdjson "encoding/json"
	goscanner "go/scanner"
	"io"
	"reflect"
	"slices"
	"testing"
	textscanner "text/scanner"
	"time"
	"unsafe"

	"example.com/kinds/other"
)

func TestNilReceiver(t *testing.T) {
	var (
		empty     *Empty
		basic     *Basic
		sl        *Slices
		maps      *Maps
		resetters *Resetters
		structs   *Structs
		embedded  *Embedded
		pointers  *Pointers
		arrays    *Arrays
		refs      *Refs
		box       *Box[int, *node]
		n         *node
		item      *other.Item
		g         *other.G[string]
	)
	empty.Reset()
	basic.Reset()
	sl.Reset()
	maps.Reset()
	resetters.Reset()
	structs.Reset()
	embedded.Reset()
	pointers.Reset()
	arrays.Reset()
	refs.Reset()
	box.Reset()
	n.Reset()
	item.Reset()
	g.Reset()
}

func TestEmpty(t *testing.T) {
	e := &Empty{}
	e.Reset()
	if *e != (Empty{}) {
		t.Fatalf("Empty after Reset = %+v", *e)
	}
}

func TestBasic(t *testing.T) {
	x := 1
	b := &Basic{i: 1, f: 2.5, s: "s", b: true, u: 3, c: 1 + 2i, up: unsafe.Pointer(&x), kind: 4, d: time.Second}
	b.Reset()
	if *b != (Basic{}) {
		t.Fatalf("Basic after Reset = %+v, want zero value", *b)
	}
}

func TestSlices(t *testing.T) {
	ints := make([]int, 3, 4)
	copy(ints, []int{1, 2, 3})
	names := []string{"a", "b"}
	n1 := &node{val: 1}
	ptrs := []*node{n1, n1}
	points := []point{{1, 2}}
	pairs := []pair{{key: "k", val: 1}}
	arrs := [][2]int{{1, 2}}
	strArrs := [][2]string{{"a", "b"}}
	writers := []io.Writer{&bytes.Buffer{}}
	x := 1
	uptrs := make([]unsafe.Pointer, 2, 3)
	uptrs[0], uptrs[1] = unsafe.Pointer(&x), unsafe.Pointer(&x)
	s := &Slices{
		ints:    ints,
		names:   names,
		ptrs:    ptrs,
		points:  points,
		pairs:   pairs,
		arrs:    arrs,
		strArrs: strArrs,
		writers: writers,
		named:   Names{"x", "y"},
		uptrs:   uptrs,
	}
	s.Reset()

	if len(s.ints) != 0 || cap(s.ints) != 4 {
		t.Errorf("ints: len %d cap %d, want 0 and 4", len(s.ints), cap(s.ints))
	}
	if got := s.ints[:3]; !slices.Equal(got, []int{1, 2, 3}) {
		t.Errorf("ints[:3] = %v, want [1 2 3]", got)
	}
	if &s.ints[:1][0] != &ints[0] {
		t.Error("ints: backing array changed")
	}
	if len(s.names) != 0 || cap(s.names) != 2 {
		t.Errorf("names: len %d cap %d, want 0 and 2", len(s.names), cap(s.names))
	}
	if got := s.names[:2]; !slices.Equal(got, []string{"", ""}) {
		t.Errorf("names[:2] = %q, want two empty strings", got)
	}
	if s.empty != nil {
		t.Errorf("nil slice became %v", s.empty)
	}
	if got := s.ptrs[:2]; got[0] != nil || got[1] != nil {
		t.Errorf("ptrs[:2] = %v, want nil elements", got)
	}
	if n1.val != 1 {
		t.Error("slice elements must not be reset")
	}
	if got := s.points[:1]; got[0] != (point{1, 2}) {
		t.Errorf("points[:1] = %v, want the old element kept", got)
	}
	if got := s.pairs[:1]; got[0] != (pair{}) {
		t.Errorf("pairs[:1] = %v, want zero element", got)
	}
	if got := s.arrs[:1]; got[0] != [2]int{1, 2} {
		t.Errorf("arrs[:1] = %v, want the old element kept", got)
	}
	if got := s.strArrs[:1]; got[0] != [2]string{} {
		t.Errorf("strArrs[:1] = %q, want zero element", got)
	}
	if got := s.writers[:1]; got[0] != nil {
		t.Errorf("writers[:1] = %v, want nil element", got)
	}
	if !slices.Equal(s.named, Names{"reset"}) {
		t.Errorf("named = %q, want Names.Reset to be called", s.named)
	}
	if len(s.uptrs) != 0 || cap(s.uptrs) != 3 || &s.uptrs[:1][0] != &uptrs[0] {
		t.Errorf("uptrs: len %d cap %d, want 0 and 3 over the same backing array", len(s.uptrs), cap(s.uptrs))
	}
	if uptrs[0] != nil || uptrs[1] != nil {
		t.Errorf("uptrs[:2] = %v, want nil elements", uptrs)
	}
}

func TestMaps(t *testing.T) {
	m := map[string]string{"a": "1", "b": "2"}
	alias := m
	x := &Maps{m: m, byPtr: map[*node]bool{{}: true}}
	x.Reset()
	if x.m == nil || len(x.m) != 0 {
		t.Fatalf("m = %v, want empty non-nil map", x.m)
	}
	if len(alias) != 0 {
		t.Errorf("alias = %v, want the same cleared map", alias)
	}
	alias["c"] = "3"
	if x.m["c"] != "3" {
		t.Error("m is not the same map object")
	}
	if x.empty != nil {
		t.Errorf("nil map became %v", x.empty)
	}
	if len(x.byPtr) != 0 {
		t.Errorf("byPtr = %v, want empty", x.byPtr)
	}
}

func TestResetters(t *testing.T) {
	var r Resetters
	r.buf.WriteString("hello, world")
	bufCap := r.buf.Cap()
	r.names = Names{"a"}
	r.rd.Reset([]byte("abc"))
	r.h = sha256.New()
	r.item.ID = 1
	r.item.Tags = make([]string, 1, 3)
	r.g.V = 5
	r.g.List = make([]int, 2, 5)
	r.counter = 7
	r.set = Set{"a": true}
	r.er = ErrReset{n: 3}
	r.vr = 3
	r.fr.Reset = true
	r.fr.WriteString("data")
	r.ca = 7
	r.Reset()

	if r.buf.Len() != 0 || r.buf.Cap() != bufCap {
		t.Errorf("buf: len %d cap %d, want 0 and %d", r.buf.Len(), r.buf.Cap(), bufCap)
	}
	if !slices.Equal(r.names, Names{"reset"}) {
		t.Errorf("names = %q, want Names.Reset to be called", r.names)
	}
	if !reflect.ValueOf(r.rd).IsZero() {
		t.Errorf("rd: size %d len %d, want bytes.Reader{}", r.rd.Size(), r.rd.Len())
	}
	if r.h != nil {
		t.Errorf("h = %v, want nil", r.h)
	}
	if r.item.ID != 0 || len(r.item.Tags) != 0 || cap(r.item.Tags) != 3 {
		t.Errorf("item = %+v cap %d, want other.Item.Reset to be called", r.item, cap(r.item.Tags))
	}
	if r.g.V != 0 || len(r.g.List) != 0 || cap(r.g.List) != 5 {
		t.Errorf("g = %+v cap %d, want other.G.Reset to be called", r.g, cap(r.g.List))
	}
	if r.counter != -1 {
		t.Errorf("counter = %d, want Counter.Reset to be called", r.counter)
	}
	if r.set != nil {
		t.Errorf("set = %v, want Set.Reset to be called", r.set)
	}
	if r.er != (ErrReset{}) {
		t.Errorf("er = %+v, want ErrReset{}: Reset() error must not be called", r.er)
	}
	if r.vr != 0 {
		t.Errorf("vr = %d, want 0: Reset(...int) must not be called", r.vr)
	}
	if r.fr.Reset || r.fr.Len() != 0 || r.fr.Cap() != 0 {
		t.Errorf("fr: Reset %t, len %d, cap %d, want FieldReset{}: the field Reset hides the promoted Buffer.Reset", r.fr.Reset, r.fr.Len(), r.fr.Cap())
	}
	if r.ca != -1 {
		t.Errorf("ca = %d, want Counter.Reset to be called through the alias", r.ca)
	}
}

func TestStructs(t *testing.T) {
	var s Structs
	s.at = time.Now()
	s.mu.Lock()
	s.cfg.n = 1
	s.cfg.tags = []string{"a"}
	s.pl = other.Plain{N: 1, Name: "n"}
	s.pt = point{1, 2}
	s.se = goscanner.Error{Msg: "m"}
	s.pos = textscanner.Position{Line: 3}
	s.Reset()

	if !s.at.IsZero() {
		t.Errorf("at = %v, want zero time", s.at)
	}
	if !s.mu.TryLock() {
		t.Error("mu is still locked, want sync.Mutex{}")
	} else {
		s.mu.Unlock()
	}
	if s.cfg.n != 0 || s.cfg.tags != nil {
		t.Errorf("cfg = %+v, want zero value", s.cfg)
	}
	if s.pl != (other.Plain{}) {
		t.Errorf("pl = %+v, want zero value", s.pl)
	}
	if s.pt != (point{}) {
		t.Errorf("pt = %+v, want zero value", s.pt)
	}
	if s.se != (goscanner.Error{}) {
		t.Errorf("se = %+v, want zero value", s.se)
	}
	if s.pos != (textscanner.Position{}) {
		t.Errorf("pos = %+v, want zero value", s.pos)
	}
}

func TestEmbedded(t *testing.T) {
	n := &node{val: 1, name: "n"}
	e := &Embedded{point: point{1, 2}, node: n, n: 3}
	e.WriteString("data")
	bufCap := e.Cap()
	e.Reset()

	if e.Len() != 0 || e.Cap() != bufCap {
		t.Errorf("Buffer: len %d cap %d, want 0 and %d", e.Len(), e.Cap(), bufCap)
	}
	if e.point != (point{}) {
		t.Errorf("point = %+v, want zero value", e.point)
	}
	if e.node != n || *n != (node{}) {
		t.Errorf("node = %p %+v, want the same pointer to a reset node", e.node, *n)
	}
	if e.n != 0 {
		t.Errorf("n = %d, want 0", e.n)
	}
}

func TestPointers(t *testing.T) {
	str := "x"
	childStr := "child"
	child := &Pointers{strP: &childStr}
	var out bytes.Buffer
	enc := stdjson.NewEncoder(&out)
	five := 5
	p5 := &five
	nd := &node{val: 1, name: "n"}
	sl := make([]string, 1, 2)
	sl[0] = "a"
	m := map[string]int{"a": 1}
	arr := [2][]byte{make([]byte, 2, 3), make([]byte, 1, 4)}
	var w io.Writer = &out
	item := &other.Item{ID: 7, Tags: make([]string, 1, 2)}
	pt := &point{1, 2}
	up := unsafe.Pointer(&five)
	npNode := &node{val: 2, name: "np"}
	p := &Pointers{strP: &str, child: child, enc: enc, pp: &p5, ppNode: &nd, sp: &sl, mp: &m, ap: &arr, wp: &w, ip: item, pt: pt, upp: &up, np: npNode}
	p.Reset()

	if p.strP != &str || str != "" {
		t.Errorf("strP: %p %q, want the same address holding an empty string", p.strP, str)
	}
	if p.child != child || childStr != "" {
		t.Errorf("child: %p, child string %q, want child.Reset to be called", p.child, childStr)
	}
	if p.enc != enc || !reflect.ValueOf(enc).Elem().IsZero() {
		t.Error("enc: want the same pointer to json.Encoder{}")
	}
	if p.pp != &p5 || *p.pp != p5 || five != 0 {
		t.Errorf("pp: **pp = %d, want 0 behind the same pointers", five)
	}
	if p.ppNode != &nd || *nd != (node{}) {
		t.Errorf("ppNode: %+v, want node.Reset behind the same pointers", *nd)
	}
	if p.sp != &sl || len(sl) != 0 || cap(sl) != 2 || sl[:1][0] != "" {
		t.Errorf("sp: %q len %d cap %d, want truncated and cleared", sl[:1], len(sl), cap(sl))
	}
	if p.mp != &m || len(m) != 0 || m == nil {
		t.Errorf("mp: %v, want the same empty map", m)
	}
	if p.ap != &arr || len(arr[0]) != 0 || cap(arr[0]) != 3 || len(arr[1]) != 0 || cap(arr[1]) != 4 {
		t.Errorf("ap: %v, want each element truncated with its capacity", arr)
	}
	if p.wp != &w || w != nil {
		t.Errorf("wp: %v, want nil behind the same pointer", w)
	}
	if p.ip != item || item.ID != 0 || len(item.Tags) != 0 || cap(item.Tags) != 2 {
		t.Errorf("ip: %+v, want other.Item.Reset behind the same pointer", *item)
	}
	if p.pt != pt || *pt != (point{}) {
		t.Errorf("pt: %+v, want point{} behind the same pointer", *pt)
	}
	if p.nilP != nil {
		t.Errorf("nilP = %v, want nil", p.nilP)
	}
	if p.upp != &up || up != nil {
		t.Errorf("upp: %v, want nil behind the same pointer", up)
	}
	if p.np != NP(npNode) || *npNode != (node{}) {
		t.Errorf("np: %+v, want node.Reset behind the same named pointer", *npNode)
	}
}

func TestPointersNilLevels(t *testing.T) {
	var inner *int
	var innerNode *node
	p := &Pointers{pp: &inner, ppNode: &innerNode}
	p.Reset()
	if p.pp != &inner || inner != nil {
		t.Errorf("pp: inner = %v, want nil", inner)
	}
	if p.ppNode != &innerNode || innerNode != nil {
		t.Errorf("ppNode: inner = %v, want nil", innerNode)
	}
	if p.strP != nil || p.child != nil || p.enc != nil || p.sp != nil || p.mp != nil || p.ap != nil || p.wp != nil || p.ip != nil || p.pt != nil || p.pwb != nil || p.upp != nil || p.np != nil {
		t.Error("nil pointers must stay nil")
	}
}

func TestArrays(t *testing.T) {
	var a Arrays
	a.digest = [32]byte{1, 2, 3}
	a.bufs[0] = make([]byte, 2, 4)
	a.bufs[3] = make([]byte, 1, 2)
	n0 := &node{val: 1, name: "n"}
	a.nodes = [2]*node{n0, nil}
	a.grid[1][2] = make([]int, 2, 3)
	a.strs = [2]string{"a", "b"}
	a.vals[0] = node{val: 1, name: "a"}
	a.maps[0] = map[string]int{"a": 1}
	a.named = Digest{1, 2}
	a.Reset()

	if a.digest != [32]byte{} {
		t.Errorf("digest = %v, want zero", a.digest)
	}
	if len(a.bufs[0]) != 0 || cap(a.bufs[0]) != 4 || len(a.bufs[3]) != 0 || cap(a.bufs[3]) != 2 || a.bufs[1] != nil {
		t.Errorf("bufs = %v, want each element truncated with its capacity", a.bufs)
	}
	if a.nodes[0] != n0 || *n0 != (node{}) || a.nodes[1] != nil {
		t.Errorf("nodes = %v, want node.Reset on the non-nil element", a.nodes)
	}
	if len(a.grid[1][2]) != 0 || cap(a.grid[1][2]) != 3 {
		t.Errorf("grid[1][2] = %v cap %d, want truncated", a.grid[1][2], cap(a.grid[1][2]))
	}
	if a.strs != [2]string{} {
		t.Errorf("strs = %q, want zero", a.strs)
	}
	if a.vals != [2]node{} {
		t.Errorf("vals = %+v, want reset elements", a.vals)
	}
	if a.maps[0] == nil || len(a.maps[0]) != 0 || a.maps[1] != nil {
		t.Errorf("maps = %v, want cleared elements", a.maps)
	}
	if a.named != (Digest{}) {
		t.Errorf("named = %v, want zero", a.named)
	}
}

func TestRefs(t *testing.T) {
	r := &Refs{w: &bytes.Buffer{}, done: make(chan struct{}), onClose: func() {}, err: io.EOF, v: 1}
	r.Reset()
	if r.w != nil || r.done != nil || r.onClose != nil || r.err != nil || r.v != nil {
		t.Errorf("Refs after Reset = %+v, want nil fields", *r)
	}
}

func TestBoxInt(t *testing.T) {
	items := make([]int, 2, 3)
	copy(items, []int{4, 5})
	v := 7
	rn := &node{val: 1}
	pn := &node{val: 2}
	pr := pn
	an := &node{val: 3}
	en := &node{val: 4}
	ra := [2]*node{en, nil}
	b := &Box[int, *node]{
		v:     1,
		items: items,
		r:     rn,
		pv:    &v,
		pr:    &pr,
		arr:   [2]int{1, 2},
		rs:    [2]*node{an, nil},
		m:     map[string]int{"a": 1},
		pair:  Pair[int]{1, 2},
		inner: other.G[int]{V: 1, List: make([]int, 1, 2)},
		pra:   &ra,
	}
	b.Reset()

	if b.v != 0 {
		t.Errorf("v = %d, want 0", b.v)
	}
	if len(b.items) != 0 || cap(b.items) != 3 || !slices.Equal(items, []int{0, 0}) {
		t.Errorf("items = %v cap %d, old elements %v; want cleared and truncated", b.items, cap(b.items), items)
	}
	if b.r != rn || rn.val != 0 {
		t.Errorf("r: val %d, want r.Reset to be called", rn.val)
	}
	if b.pv != &v || v != 0 {
		t.Errorf("pv: %d, want zero behind the same pointer", v)
	}
	if b.pr != &pr || pr != pn || pn.val != 0 {
		t.Errorf("pr: val %d, want Reset behind the same pointer", pn.val)
	}
	if b.arr != [2]int{} {
		t.Errorf("arr = %v, want zero", b.arr)
	}
	if b.rs[0] != an || an.val != 0 || b.rs[1] != nil {
		t.Errorf("rs = %v, want Reset on each element", b.rs)
	}
	if b.m == nil || len(b.m) != 0 {
		t.Errorf("m = %v, want cleared", b.m)
	}
	if b.pair != (Pair[int]{}) {
		t.Errorf("pair = %+v, want zero", b.pair)
	}
	if b.inner.V != 0 || len(b.inner.List) != 0 || cap(b.inner.List) != 2 {
		t.Errorf("inner = %+v, want other.G.Reset to be called", b.inner)
	}
	if b.pra != &ra || ra[0] != en || en.val != 0 || ra[1] != nil {
		t.Errorf("pra: %v, val %d, want Reset on each element behind the same pointer", ra, en.val)
	}
}

func TestBoxString(t *testing.T) {
	items := []string{"a", "b"}
	b := &Box[string, *node]{v: "v", items: items}
	b.Reset()
	if b.v != "" {
		t.Errorf("v = %q, want empty", b.v)
	}
	if len(b.items) != 0 || cap(b.items) != 2 || !slices.Equal(items, []string{"", ""}) {
		t.Errorf("items: old elements %q, want cleared and truncated", items)
	}
}

func TestOtherItem(t *testing.T) {
	tags := make([]string, 2, 4)
	tags[0], tags[1] = "a", "b"
	it := &other.Item{ID: 1, Tags: tags}
	it.Reset()
	if it.ID != 0 || len(it.Tags) != 0 || cap(it.Tags) != 4 || !slices.Equal(tags, []string{"", ""}) {
		t.Errorf("Item after Reset = %+v, old tags %q", *it, tags)
	}
}

func TestPromotedReset(t *testing.T) {
	r := &Resetters{}
	r.w.WriteString("data")
	r.w.n = 5
	r.Reset()
	if r.w.Len() != 0 {
		t.Errorf("w.Len() = %d, want 0", r.w.Len())
	}
	if r.w.n != 5 {
		t.Errorf("w.n = %d, want 5: Wrapper gets its promoted Buffer.Reset()", r.w.n)
	}

	s := &Structs{}
	s.anon.WriteString("data")
	s.anon.n = 7
	s.Reset()
	if s.anon.Len() != 0 || s.anon.Cap() != 0 || s.anon.n != 0 {
		t.Errorf("anon = len %d, cap %d, n %d, want the zero value", s.anon.Len(), s.anon.Cap(), s.anon.n)
	}

	pa := &struct {
		bytes.Buffer
		n int
	}{}
	pa.WriteString("data")
	pa.n = 9
	p := &Pointers{pa: pa}
	p.Reset()
	if p.pa != pa {
		t.Error("pa points to a different address after Reset")
	}
	if pa.Len() != 0 || pa.Cap() != 0 || pa.n != 0 {
		t.Errorf("*pa = len %d, cap %d, n %d, want the zero value", pa.Len(), pa.Cap(), pa.n)
	}
}

func TestPromotedResetNilEmbedded(t *testing.T) {
	r := &Resetters{}
	r.Reset()
	if r.wb.Buffer != nil || r.wh.Hash != nil {
		t.Errorf("wb.Buffer = %v, wh.Hash = %v, want both to stay nil", r.wb.Buffer, r.wh.Hash)
	}

	buf := bytes.NewBufferString("data")
	bufCap := buf.Cap()
	h := sha256.New()
	h.Write([]byte("data"))
	r.wb.Buffer = buf
	r.wh.Hash = h
	r.Reset()
	if r.wb.Buffer != buf || buf.Len() != 0 || buf.Cap() != bufCap {
		t.Errorf("wb: len %d cap %d, want Buffer.Reset on the same buffer with cap %d", buf.Len(), buf.Cap(), bufCap)
	}
	if r.wh.Hash != h || !bytes.Equal(h.Sum(nil), sha256.New().Sum(nil)) {
		t.Error("wh: want Hash.Reset on the same hash")
	}

	p := &Pointers{pwb: &WB{}}
	p.Reset()
	if p.pwb.Buffer != nil {
		t.Errorf("pwb.Buffer = %v, want nil", p.pwb.Buffer)
	}
	pbuf := bytes.NewBufferString("data")
	p.pwb.Buffer = pbuf
	p.Reset()
	if p.pwb.Buffer != pbuf || pbuf.Len() != 0 {
		t.Errorf("pwb: len %d, want Buffer.Reset behind the pointer", pbuf.Len())
	}

	a := &Arrays{}
	abuf := bytes.NewBufferString("data")
	a.wbs[1].Buffer = abuf
	a.Reset()
	if a.wbs[0].Buffer != nil || a.wbs[1].Buffer != abuf || abuf.Len() != 0 {
		t.Errorf("wbs: len %d, want the nil element unchanged and Buffer.Reset on the other", abuf.Len())
	}
}
