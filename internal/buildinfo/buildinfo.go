package buildinfo

import (
	"cmp"
	"fmt"
	"io"
)

const notAvailable = "N/A"

// Print writes the build version, date and commit to w, one per
// line, printing N/A in place of empty values.
func Print(w io.Writer, version, date, commit string) {
	fmt.Fprintf(w, "Build version: %s\nBuild date: %s\nBuild commit: %s\n",
		cmp.Or(version, notAvailable), cmp.Or(date, notAvailable), cmp.Or(commit, notAvailable))
}
