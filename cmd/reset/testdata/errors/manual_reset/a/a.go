package a

// generate:reset
type T struct {
	n int
}

func (t *T) Reset() {
	t.n = 0
}

// generate:reset
type U struct {
	n int
}

func (u U) Reset(n int) error {
	return nil
}
