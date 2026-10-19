// Command repo-tools holds the small programs the sheafdb repositories run
// on themselves. Each program is a subcommand, in a package of its own:
//
//	repo-tools md-table-fmt -n $(git ls-files '*.md')
//	repo-tools plan-report --dir docs/features --file-match '^plan\.md$'
//	repo-tools prompt-history-record < hook.json
//	repo-tools prompt-history-meta -root /path/to/repo
//	repo-tools prompt-history-guard
//	repo-tools tmux attach <session>
//
// `repo-tools <subcommand> --help` lists the flags of one subcommand. A
// flag takes one dash or two: `-n` and `--n` are the same flag.
//
// `repo-tools --install` installs the shell completion for the shell in
// $SHELL (bash, zsh or fish). `repo-tools --uninstall` removes it. The
// completion runs this binary by its absolute path, so install it again
// after the binary moves, as it does on each nix build.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/jpillora/opts"
	"github.com/wxio/repo-tools/internal/argcomplete"
	"github.com/wxio/repo-tools/internal/exitcode"
	"github.com/wxio/repo-tools/mdtablefmt"
	"github.com/wxio/repo-tools/planreport"
	"github.com/wxio/repo-tools/prompthistoryguard"
	"github.com/wxio/repo-tools/prompthistorymeta"
	"github.com/wxio/repo-tools/prompthistoryrecord"
	"github.com/wxio/repo-tools/tmux"
)

var (
	rootCmd = &struct{}{} // empty root — no top-level flags
	builder = opts.New(rootCmd).
		Name("repo-tools").
		Summary("The small programs the sheafdb repositories run on themselves.").
		EmbedGlobalFlagSet().
		Complete()

	// argCompleters completes the first argument of these subcommands.
	// opts completes an argument as a file name, so main answers first.
	argCompleters = map[string]opts.Completer{
		"tmux attach":       new(tmux.Session),
		"tmux kill-session": new(tmux.Session),
	}
)

func init() {
	builder.AddCommand(opts.New(mdtablefmt.NewMdTableFmtCmd()).Name("md-table-fmt").
		Summary("Align the tables in Markdown files."))

	builder.AddCommand(opts.New(planreport.NewPlanReportCmd()).Name("plan-report").
		Summary("Count the open and done tasks of the Markdown files under a directory, per feature."))

	builder.AddCommand(opts.New(prompthistoryrecord.NewPromptHistoryRecordCmd()).Name("prompt-history-record").
		Summary("The Claude Code Stop hook: write a prompt-history entry for the last turn."))

	builder.AddCommand(opts.New(prompthistorymeta.NewPromptHistoryMetaCmd()).Name("prompt-history-meta").
		Summary("Rebuild .prompt-history/meta-info.csv from the entries on disk."))

	builder.AddCommand(opts.New(prompthistoryguard.NewPromptHistoryGuardCmd()).Name("prompt-history-guard").
		Summary("The prek hook: keep .prompts/ and .prompt-history/ in commits of their own."))

	builder.AddCommand(opts.New(&struct{}{}).Name("tmux").
		Summary("Make, attach to or kill a tmux session; <tab> lists the sessions.").
		AddCommand(opts.New(tmux.NewNewCmd()).Name("new").
			Summary("Make a tmux session and attach to it, or switch to it inside tmux.")).
		AddCommand(opts.New(tmux.NewAttachCmd()).Name("attach").
			Summary("Attach to a tmux session, or switch to it inside tmux.")).
		AddCommand(opts.New(tmux.NewKillSessionCmd()).Name("kill-session").
			Summary("Kill a tmux session.")))
}

func main() {
	var (
		cli  opts.ParsedOpts
		code exitcode.Code
		err  error
	)
	if argcomplete.Run(argCompleters) {
		return
	}
	// Parse, not ParseArgsError: opts returns --help and --install as
	// errors of unexported types, and Parse exits 0 on them where main
	// cannot tell them apart.
	cli = builder.Parse()
	if err = cli.Run(); err == nil {
		return
	}
	// The subcommand reported the problem itself. The code is for prek.
	if errors.As(err, &code) {
		os.Exit(int(code))
	}
	fmt.Fprintf(os.Stderr, "%s\nError: %v\n\n", cli.Selected().Help(), err)
	os.Exit(2)
}
