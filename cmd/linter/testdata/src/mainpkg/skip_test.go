package main

import "os"

func panicInTest() {
	panic("boom")
}

func exitInTest() {
	os.Exit(1)
}
