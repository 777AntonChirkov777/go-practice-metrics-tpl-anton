package other

// generate:reset
type Item struct {
	ID   int
	Tags []string
}

type Plain struct {
	N    int
	Name string
}

// generate:reset
type G[T any] struct {
	V    T
	List []T
}
