// Package mdtablefmt is the md-table-fmt subcommand. It aligns the tables in
// Markdown files.
//
//	repo-tools md-table-fmt docs/architecture/lowering.md
//	repo-tools md-table-fmt -n $(git ls-files '*.md')
//
// It is the "Markdown style" rule of this repo's AGENTS.md, made a program:
// every cell is padded to the width of the widest cell in its column, and
// the separator row is filled to match, so every row of one table is the
// same length. A page then reads in the raw file and not only in a
// viewer. Nothing formats Markdown the way gofmt formats Go, so this is
// written down rather than assumed.
//
// It also runs as a local prek hook, over the Markdown files a commit
// stages. A fixer hook rewrites the files and fails, which is what makes
// the commit stop and the author look at what changed; `git add` and
// commit again.
//
// Exit codes:
//
//	0  every file was already aligned
//	1  a file was rewritten, or -n found one that needs it
//	2  a file holds `\|` in a cell, or could not be read or written
//
// `go run` reports a non-zero code as "exit status N" and exits 1
// itself, so the hook sees pass or fail and the message on stderr says
// which of the two it was. Run the built binary where the code matters.
//
// The difference between 1 and 2 is whether the program can fix it. A
// pipe inside a cell is two characters in the file and one on screen, so
// no padding lines the column up in both; rewriting the cell is a
// decision about words, which the author makes. A cell past -wide is
// reported and left alone for the same reason, but does not fail: the
// repo has tables that are over the limit and are not wrong enough to
// block a commit over.
package mdtablefmt

import (
	"fmt"
	"os"

	"github.com/wxio/repo-tools/internal/exitcode"
	"github.com/wxio/repo-tools/internal/mdtable"
)

const (
	// exitChanged says a file was rewritten. prek reads it as a failed
	// hook, which is how a fixer tells the author to look.
	exitChanged exitcode.Code = 1

	// exitRefused says the program cannot fix what it found.
	exitRefused exitcode.Code = 2
)

// MdTableFmtCmd is the command line of md-table-fmt.
type MdTableFmtCmd struct {
	DryRun bool     `opts:"name=n" help:"report what would change; write nothing"`
	Quiet  bool     `opts:"name=q" help:"report nothing but refusals"`
	Wide   int      `help:"report a cell wider than this many runes; 0 turns the report off"`
	Paths  []string `opts:"mode=arg,name=file.md"`
}

// NewMdTableFmtCmd returns the command line of md-table-fmt, with the
// defaults of its flags set.
func NewMdTableFmtCmd() *MdTableFmtCmd {
	return &MdTableFmtCmd{
		Wide: 70,
	}
}

func (c *MdTableFmtCmd) Run() error {
	var (
		changed int
		refused int
		err     error
	)
	if len(c.Paths) == 0 {
		fmt.Fprintln(os.Stderr, "md-table-fmt: no file given; see --help")
		return exitRefused
	}
	for _, path := range c.Paths {
		var n, r int
		if n, r, err = formatFile(path, mdtable.Formatter{
			Wide: c.Wide,
		}, c.DryRun, c.Quiet); err != nil {
			fmt.Fprintf(os.Stderr, "md-table-fmt: %v\n", err)
			refused++
			continue
		}
		changed += n
		refused += r
	}
	if refused > 0 {
		return exitRefused
	}
	if changed > 0 {
		return exitChanged
	}
	return nil
}

// formatFile formats a single file. It returns whether the file changed and
// whether it was refused, so that every file is looked at before the
// program decides its exit code -- a run over the whole repo reports
// every offending table, not the first.
func formatFile(path string, f mdtable.Formatter, dry, quiet bool) (int, int, error) {
	var (
		src  []byte
		res  mdtable.Result
		info os.FileInfo
		err  error
	)
	if src, err = os.ReadFile(path); err != nil {
		return 0, 0, err
	}
	res = f.Format(string(src))
	for _, p := range res.Pipes {
		fmt.Fprintf(os.Stderr, "%s:%d: a cell holds `\\|`, which cannot align in the file and on screen: %s\n",
			path, p.Line, p.Cell)
	}
	if len(res.Pipes) > 0 {
		return 0, 1, nil
	}
	if !quiet {
		for _, w := range res.Wides {
			fmt.Fprintf(os.Stderr, "%s:%d: a cell of %d runes makes every row of its table long: %.40s...\n",
				path, w.Line, len([]rune(w.Cell)), w.Cell)
		}
	}
	if !res.Changed {
		return 0, 0, nil
	}
	if dry {
		fmt.Printf("%s: %d tables, needs aligning\n", path, res.Tables)
		return 1, 0, nil
	}
	if info, err = os.Stat(path); err != nil {
		return 0, 0, err
	}
	if err = os.WriteFile(path, []byte(res.Text), info.Mode().Perm()); err != nil {
		return 0, 0, err
	}
	if !quiet {
		fmt.Printf("%s: %d tables aligned\n", path, res.Tables)
	}
	return 1, 0, nil
}
