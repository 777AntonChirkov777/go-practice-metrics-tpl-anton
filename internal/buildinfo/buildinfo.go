package buildinfo

import (
	"cmp"
	"fmt"
	"io"
)

const notAvailable = "N/A"

func Print(w io.Writer, version, date, commit string) {
	fmt.Fprintf(w, "Build version: %s\nBuild date: %s\nBuild commit: %s\n",
		cmp.Or(version, notAvailable), cmp.Or(date, notAvailable), cmp.Or(commit, notAvailable))
}
