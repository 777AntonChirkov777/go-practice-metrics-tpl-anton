package a

type I interface {
	M()
}

type T struct{}

func (T) M(int) {}

var _ I = T{}

// generate:reset
type A struct {
	n int
}
