package markers

// Described — структура с описанием и маркером.
// generate:reset
type Described struct {
	n int
}

// DescribedDirective — структура с описанием и маркером без пробела.
//
//generate:reset
type DescribedDirective struct {
	n int
}

//generate:reset
type NoSpace struct {
	n int
}

// generate:reset

type Detached struct {
	n int
}

// generate.reset
type Typo struct {
	n int
}

// generate: reset
type Spaced struct {
	n int
}

// generate:reset
type (
	groupA struct {
		n int
	}
	groupB struct {
		n int
	}
)

type (
	// generate:reset
	gaugeRow struct {
		value float64
	}

	counterRow struct {
		delta int64
	}
)

// generate:reset
type Empty struct{}
