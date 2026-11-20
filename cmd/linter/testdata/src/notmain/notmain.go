package notmain

import "os"

func main() {
	if len(os.Args) > 1 {
		panic("unexpected arguments") // want `use of builtin panic`
	}
	os.Exit(1) // want `use of os\.Exit outside function main of package main`
}
