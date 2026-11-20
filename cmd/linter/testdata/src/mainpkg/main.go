package main

import (
	"log"
	"os"
)

type app struct{}

type service struct {
	*log.Logger
}

var exit = os.Exit // want `use of os\.Exit outside function main of package main`

func main() {
	code := 0
	defer func() {
		os.Exit(code)
	}()
	defer os.Exit(0)
	go func() {
		log.Fatalln("worker failed")
	}()
	logger := log.New(os.Stderr, "", 0)
	if len(os.Args) > 2 {
		logger.Fatalf("too many arguments: %d", len(os.Args))
	}
	if len(os.Args) > 1 {
		log.Fatal("unexpected arguments")
	}
	if code != 0 {
		panic("bad exit code") // want `use of builtin panic`
	}
	os.Exit(1)
}

func init() {
	log.Fatalf("bad config: %v", nil) // want `use of log\.Fatalf outside function main of package main`
}

func run() {
	os.Exit(1) // want `use of os\.Exit outside function main of package main`
}

func fail() {
	panic("boom") // want `use of builtin panic`
}

func (a app) main() {
	os.Exit(2) // want `use of os\.Exit outside function main of package main`
}

func closureOutsideMain() func() {
	return func() {
		os.Exit(4) // want `use of os\.Exit outside function main of package main`
	}
}

func deferredExit() {
	defer os.Exit(3) // want `use of os\.Exit outside function main of package main`
}

func asyncFatal() {
	go log.Fatal("async") // want `use of log\.Fatal outside function main of package main`
}

func lineFatal() {
	log.Fatalln("line") // want `use of log\.Fatalln outside function main of package main`
}

func defaultLoggerFatal() {
	log.Default().Fatal("default") // want `use of \(\*log\.Logger\)\.Fatal outside function main of package main`
}

func useLogger(logger *log.Logger) {
	logger.Fatalln("x") // want `use of \(\*log\.Logger\)\.Fatalln outside function main of package main`
}

func useMethodValue(logger *log.Logger) {
	fatal := logger.Fatal // want `use of \(\*log\.Logger\)\.Fatal outside function main of package main`
	_ = fatal
}

func useMethodExpression() {
	fatalf := (*log.Logger).Fatalf // want `use of \(\*log\.Logger\)\.Fatalf outside function main of package main`
	_ = fatalf
}

func useEmbedded(s service) {
	s.Fatal("x") // want `use of \(\*log\.Logger\)\.Fatal outside function main of package main`
}

func useFuncValue() {
	stop := os.Exit // want `use of os\.Exit outside function main of package main`
	_ = stop
}

func shadowedByLocal() {
	panic := func(string) {}
	panic("x")
}

func shadowedByParam(panic func(string)) {
	panic("x")
}

func Exit(code int) {}

func ownExit() {
	Exit(1)
}
