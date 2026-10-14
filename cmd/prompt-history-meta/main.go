// Command prompt-history-meta rebuilds .prompt-history/meta-info.csv from the
// entries already on disk.
//
// The CSV is the index over the history: one row per turn and per agent, every
// numeric column a bare integer, so it can be summed or plotted as it stands. It
// is derived and not a second source of truth — every column it holds is also
// written into the markdown — so this command reconstructs it, which is also how
// a history written before the CSV existed is backfilled.
//
// It reads nothing from stdin.
//
//	-root  the repository to work in (default: the git top level of the working
//	       directory)
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/wxio/sheafdb-tools/internal/prompthistory"
)

func main() {
	var (
		root = flag.String("root", "", "repository to work in (default: the git top level of the working directory)")
		path string
		rows int
		err  error
	)
	flag.Parse()

	if path, rows, err = prompthistory.Rebuild(*root); err != nil {
		fmt.Fprintln(os.Stderr, "rebuild failed:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s with %d rows\n", path, rows)
}
