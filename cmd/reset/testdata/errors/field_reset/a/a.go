package a

// generate:reset
type T struct {
	Reset bool
}

type Reset struct{}

// generate:reset
type U struct {
	Reset
	n int
}

// generate:reset
type V struct {
	*Reset
}
