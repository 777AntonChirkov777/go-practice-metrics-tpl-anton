package main

import (
	. "log"
	. "os"
)

func main() {
	if len(Args) > 1 {
		Fatal("unexpected arguments")
	}
	Exit(0)
}

func stop() {
	Exit(1) // want `use of os\.Exit outside function main of package main`
}

func stopf() {
	Fatalf("stop: %d", 1) // want `use of log\.Fatalf outside function main of package main`
}

func panic(v any) {}

func shadowedByPackageFunc() {
	panic("x")
}
