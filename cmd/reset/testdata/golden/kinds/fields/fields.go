package fields

import (
	"bytes"
	stdjson "encoding/json"
	goscanner "go/scanner"
	"hash"
	"io"
	"sync"
	textscanner "text/scanner"
	"time"
	"unsafe"

	"example.com/kinds/other"
)

type Kind int8

type Digest [4]byte

type Counter int

func (c *Counter) Reset() {
	*c = -1
}

type Names []string

func (n *Names) Reset() {
	*n = append((*n)[:0], "reset")
}

type Set map[string]bool

func (s *Set) Reset() {
	*s = nil
}

type point struct {
	x, y int
}

type pair struct {
	key string
	val int
}

type Pair[T any] struct {
	a, b T
}

type Wrapper struct {
	bytes.Buffer
	n int
}

type WB struct {
	*bytes.Buffer
}

type WH struct {
	hash.Hash
}

type ErrReset struct {
	n int
}

func (e *ErrReset) Reset() error {
	e.n = -1
	return nil
}

type Variadic int

func (v *Variadic) Reset(...int) {
	*v = -1
}

type resetFlag struct {
	Reset bool
}

type deepBuffer struct {
	bytes.Buffer
}

type FieldReset struct {
	resetFlag
	deepBuffer
}

type CounterAlias = Counter

type NP *node

// generate:reset
type Empty struct{}

// generate:reset
type Basic struct {
	i    int
	f    float64
	s    string
	b    bool
	u    uintptr
	c    complex128
	up   unsafe.Pointer
	kind Kind
	d    time.Duration
	_    [8]byte
}

// generate:reset
type Slices struct {
	ints    []int
	names   []string
	empty   []int
	ptrs    []*node
	points  []point
	pairs   []pair
	arrs    [][2]int
	strArrs [][2]string
	zeroArr [][0]*int
	writers []io.Writer
	named   Names
	uptrs   []unsafe.Pointer
}

// generate:reset
type Maps struct {
	m     map[string]string
	empty map[int]int
	byPtr map[*node]bool
}

// generate:reset
type Resetters struct {
	buf     bytes.Buffer
	names   Names
	rd      bytes.Reader
	h       hash.Hash
	item    other.Item
	g       other.G[int]
	counter Counter
	set     Set
	w       Wrapper
	wb      WB
	wh      WH
	er      ErrReset
	vr      Variadic
	fr      FieldReset
	ca      CounterAlias
}

// generate:reset
type Structs struct {
	at  time.Time
	mu  sync.Mutex
	cfg struct {
		n    int
		tags []string
	}
	pl   other.Plain
	pt   point
	se   goscanner.Error
	pos  textscanner.Position
	anon struct {
		bytes.Buffer
		n int
	}
}

// generate:reset
type Embedded struct {
	bytes.Buffer
	point
	*node
	n int
}

// generate:reset
type Pointers struct {
	strP   *string
	child  *Pointers
	enc    *stdjson.Encoder
	pp     **int
	ppNode **node
	sp     *[]string
	mp     *map[string]int
	ap     *[2][]byte
	wp     *io.Writer
	ip     *other.Item
	pt     *point
	nilP   *int
	pa     *struct {
		bytes.Buffer
		n int
	}
	pwb *WB
	upp *unsafe.Pointer
	np  NP
}

// generate:reset
type Arrays struct {
	digest [32]byte
	bufs   [4][]byte
	nodes  [2]*node
	grid   [2][3][]int
	strs   [2]string
	vals   [2]node
	maps   [2]map[string]int
	named  Digest
	wbs    [2]WB
}

// generate:reset
type Refs struct {
	w       io.Writer
	done    chan struct{}
	onClose func()
	err     error
	v       any
}

// generate:reset
type Box[T any, R interface{ Reset() }] struct {
	v     T
	items []T
	r     R
	pv    *T
	pr    *R
	arr   [2]T
	rs    [2]R
	m     map[string]T
	pair  Pair[T]
	inner other.G[T]
	pra   *[2]R
}
