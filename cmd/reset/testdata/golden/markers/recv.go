package markers

// generate:reset
type HTTPSink struct {
	addr string
}

// generate:reset
type Metric struct {
	id string
}

func (mm *Metric) ID() string {
	return mm.id
}

// generate:reset
type Blank struct {
	n int
}

func (Blank) Name() string {
	return "blank"
}

// generate:reset
type Under struct {
	n int
}

func (_ Under) Name() string {
	return "under"
}

// generate:reset
type Rule struct {
	val r
}

type r struct {
	n int
}

// generate:reset
type IfFlag struct {
	on bool
}

// generate:reset
type Index struct {
	grid [2][]int
}
