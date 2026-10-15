// Command sheafdb-tools holds the small programs the sheafdb repositories run
// on themselves. Each program is a subcommand:
//
//	sheafdb-tools md-table-fmt -n $(git ls-files '*.md')
//	sheafdb-tools prompt-history-record < hook.json
//	sheafdb-tools prompt-history-meta -root /path/to/repo
//	sheafdb-tools prompt-history-guard
//
// `sheafdb-tools <subcommand> --help` lists the flags of one subcommand. A
// flag takes one dash or two: `-n` and `--n` are the same flag.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/jpillora/opts"
)

// root is the command line above the subcommands. It has no flags of its own.
type root struct{}

// exitCode is an error that sets the exit code of the process. A subcommand
// returns it when the code means something to its caller, as the code of a
// prek hook does. The subcommand reports the problem itself, so main prints
// nothing more.
type exitCode int

func (c exitCode) Error() string {
	return fmt.Sprintf("exit status %d", int(c))
}

func main() {
	var (
		cli  opts.ParsedOpts
		code exitCode
		err  error
	)
	cli = opts.New(&root{}).
		Name("sheafdb-tools").
		Summary("The small programs the sheafdb repositories run on themselves.").
		AddCommand(opts.New(&mdTableFmt{
			Wide: 70,
		}).
			Name("md-table-fmt").
			Summary("Align the tables in Markdown files.")).
		AddCommand(opts.New(&promptHistoryRecord{
			Meta: true,
		}).
			Name("prompt-history-record").
			Summary("The Claude Code Stop hook: write a prompt-history entry for the last turn.")).
		AddCommand(opts.New(&promptHistoryMeta{}).
			Name("prompt-history-meta").
			Summary("Rebuild .prompt-history/meta-info.csv from the entries on disk.")).
		AddCommand(opts.New(&promptHistoryGuard{}).
			Name("prompt-history-guard").
			Summary("The prek hook: keep .prompt-history/ in commits of its own.")).
		Parse()

	// No subcommand was given. Show what there is to choose from.
	if !cli.IsRunnable() {
		fmt.Fprint(os.Stderr, cli.Selected().Help())
		os.Exit(2)
	}
	if err = cli.Run(); err == nil {
		return
	}
	if errors.As(err, &code) {
		os.Exit(int(code))
	}
	fmt.Fprintf(os.Stderr, "sheafdb-tools: %v\n", err)
	os.Exit(1)
}
