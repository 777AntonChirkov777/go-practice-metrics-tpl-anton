package pool

import (
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"practice/internal/audit"
	"practice/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	_ Resetter = (*audit.Event)(nil)
	_ Resetter = (*model.Metric)(nil)
)

type buffer struct {
	data   []byte
	resets int
}

func (b *buffer) Reset() {
	b.data = b.data[:0]
	b.resets++
}

type token int

func (token) Reset() {}

type probe struct {
	pool *Pool[*probe]
	seen *probe
}

func (pr *probe) Reset() {
	pr.seen = pr.pool.Get()
}

var nilResets atomic.Int32

type nilSafe struct{}

func (n *nilSafe) Reset() {
	if n == nil {
		nilResets.Add(1)
	}
}

type factory struct {
	mu      sync.Mutex
	seed    string
	created []*buffer
}

func (f *factory) create() *buffer {
	b := &buffer{data: append(make([]byte, 0, 64), f.seed...)}

	f.mu.Lock()
	f.created = append(f.created, b)
	f.mu.Unlock()

	return b
}

func (f *factory) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.created)
}

func (f *factory) resets() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	total := 0
	for _, b := range f.created {
		total += b.resets
	}

	return total
}

func TestResetter_Constraint(t *testing.T) {
	resetter := reflect.TypeFor[Resetter]()

	tests := []struct {
		name string
		typ  reflect.Type
		want bool
	}{
		{name: "pointer to audit event", typ: reflect.TypeFor[*audit.Event](), want: true},
		{name: "pointer to model metric", typ: reflect.TypeFor[*model.Metric](), want: true},
		{name: "audit event value", typ: reflect.TypeFor[audit.Event](), want: false},
		{name: "metrics dto without reset", typ: reflect.TypeFor[*model.Metrics](), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.typ.Implements(resetter))
		})
	}
}

func TestNew_ProjectType(t *testing.T) {
	p := New(func() *audit.Event { return &audit.Event{} })

	require.NotNil(t, p)
	assert.IsType(t, (*Pool[*audit.Event])(nil), p)
	assert.NotNil(t, p.Get())
}

func TestNew_DoesNotCallFactory(t *testing.T) {
	f := &factory{}

	p := New(f.create)

	require.NotNil(t, p)
	assert.Zero(t, f.calls())
}

func TestNew_NilFactory(t *testing.T) {
	var p *Pool[*buffer]

	require.NotPanics(t, func() { p = New[*buffer](nil) })
	assert.NotNil(t, p)
}

func TestGet_EmptyPoolCallsFactoryOnce(t *testing.T) {
	f := &factory{}
	p := New(f.create)

	got := p.Get()

	require.Equal(t, 1, f.calls())
	assert.Same(t, f.created[0], got)
}

func TestGet_KeepsFactoryPreparation(t *testing.T) {
	f := &factory{seed: "seed"}
	p := New(f.create)

	got := p.Get()

	assert.Equal(t, "seed", string(got.data))
	assert.Equal(t, 64, cap(got.data))
	assert.Zero(t, got.resets)
}

func TestGet_EmptyPoolWithoutFactory(t *testing.T) {
	t.Run("pointer", func(t *testing.T) {
		p := New[*buffer](nil)

		var got *buffer
		require.NotPanics(t, func() { got = p.Get() })
		assert.Nil(t, got)
	})

	t.Run("interface", func(t *testing.T) {
		p := New[Resetter](nil)

		var got Resetter
		require.NotPanics(t, func() { got = p.Get() })
		assert.Nil(t, got)
	})

	t.Run("value", func(t *testing.T) {
		p := New[token](nil)

		got := token(1)
		require.NotPanics(t, func() { got = p.Get() })
		assert.Equal(t, token(0), got)
	})
}

func TestPut_ResetsObject(t *testing.T) {
	f := &factory{}
	p := New(f.create)
	b := p.Get()
	b.data = append(b.data, "payload"...)

	p.Put(b)

	assert.Equal(t, 1, b.resets)
	assert.Empty(t, b.data)
}

func TestPut_ThenGet(t *testing.T) {
	f := &factory{}
	p := New(f.create)
	b := p.Get()
	b.data = append(b.data, "payload"...)
	p.Put(b)

	got := p.Get()

	require.NotNil(t, got)
	assert.Empty(t, got.data)
	if got == b {
		assert.Equal(t, 1, got.resets)
		assert.Equal(t, 1, f.calls())
	} else {
		assert.Zero(t, got.resets)
		assert.Equal(t, 2, f.calls())
	}
}

func TestPut_InterfaceType(t *testing.T) {
	t.Run("nil is ignored", func(t *testing.T) {
		p := New[Resetter](nil)

		require.NotPanics(t, func() { p.Put(nil) })
		assert.Nil(t, p.Get())
	})

	t.Run("value is reset", func(t *testing.T) {
		p := New[Resetter](nil)
		b := &buffer{data: []byte("payload")}

		p.Put(b)

		assert.Equal(t, 1, b.resets)
		assert.Empty(t, b.data)
	})
}

func TestPut_TypedNilWithGeneratedReset(t *testing.T) {
	p := New(func() *audit.Event { return &audit.Event{} })

	assert.NotPanics(t, func() { p.Put((*audit.Event)(nil)) })
}

func TestPut_TypedNilCallsReset(t *testing.T) {
	nilResets.Store(0)
	p := New[*nilSafe](nil)

	require.NotPanics(t, func() { p.Put((*nilSafe)(nil)) })
	assert.Equal(t, int32(1), nilResets.Load())
}

func TestPut_ResetBeforeReturnToPool(t *testing.T) {
	for range 100 {
		p := New[*probe](nil)
		pr := &probe{pool: p}

		p.Put(pr)

		require.Nil(t, pr.seen)
	}
}

func TestPool_ConcurrentGetPut(t *testing.T) {
	const (
		workers    = 8
		iterations = 1000
	)

	f := &factory{}
	p := New(f.create)

	var dirty atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range iterations {
				b := p.Get()
				if len(b.data) != 0 {
					dirty.Add(1)
				}
				b.data = append(b.data, 'x')
				p.Put(b)
			}
		})
	}
	wg.Wait()

	assert.Zero(t, dirty.Load())
	assert.Positive(t, f.calls())
	assert.Equal(t, workers*iterations, f.resets())
}
