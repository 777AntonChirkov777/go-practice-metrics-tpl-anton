package main

import log "fakelog"

func useFakeLog() {
	log.Fatal("not the standard log")
}
