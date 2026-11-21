package a

// generate:reset
type A struct {
	n int
}

func f() {
	// generate:reset
	type L struct {
		n int
	}
}

var g = func() {
	type (
		// generate:reset
		M struct {
			n int
		}
	)
}
