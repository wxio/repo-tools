package main

// prompt_history_meta.go is the prompt-history-meta subcommand. It rebuilds
// .prompt-history/meta-info.csv from the entries already on disk.
//
// The CSV is the index over the history: one row per turn and per agent, every
// numeric column a bare integer, so it can be summed or plotted as it stands. It
// is derived and not a second source of truth — every column it holds is also
// written into the markdown — so this command reconstructs it, which is also how
// a history written before the CSV existed is backfilled. It is also how a
// history recorded with `prompt-history-record -meta=false` gets its index.
//
// It reads nothing from stdin.

import (
	"fmt"

	"github.com/wxio/repo-tools/internal/prompthistory"
)

// promptHistoryMeta is the command line of prompt-history-meta.
type promptHistoryMeta struct {
	Root string `help:"the repository to work in (default: the git top level of the working directory)"`
}

func (c *promptHistoryMeta) Run() error {
	var (
		path string
		rows int
		err  error
	)
	if path, rows, err = prompthistory.Rebuild(c.Root); err != nil {
		return fmt.Errorf("rebuild failed: %w", err)
	}
	fmt.Printf("wrote %s with %d rows\n", path, rows)
	return nil
}
