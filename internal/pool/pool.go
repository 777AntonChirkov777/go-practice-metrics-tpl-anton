package pool

import "sync"

type Resetter interface {
	Reset()
}

type Pool[T Resetter] struct {
	pool sync.Pool
}

func New[T Resetter](newFn func() T) *Pool[T] {
	p := &Pool[T]{}
	if newFn != nil {
		p.pool.New = func() any {
			return newFn()
		}
	}

	return p
}

func (p *Pool[T]) Get() T {
	x, _ := p.pool.Get().(T)

	return x
}

func (p *Pool[T]) Put(x T) {
	if any(x) == nil {
		return
	}

	x.Reset()
	p.pool.Put(x)
}
