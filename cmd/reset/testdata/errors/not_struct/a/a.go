package a

type S struct {
	n int
}

// generate:reset
type ID int

// generate:reset
type List []int

// generate:reset
type Dict map[string]int

// generate:reset
type Iface interface {
	M()
}

// generate:reset
type Fn func()

// generate:reset
type Defined S

// generate:reset
type Alias = S
