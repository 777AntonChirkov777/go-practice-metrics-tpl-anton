package main

import stdos "os"

func useAlias() {
	stdos.Exit(1) // want `use of os\.Exit outside function main of package main`
}
