// Command md-table-fmt aligns the tables in Markdown files.
//
//	md-table-fmt docs/architecture/lowering.md
//	md-table-fmt -n $(git ls-files '*.md')
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
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/wxio/sheafdb-tools/internal/mdtable"
)

const (
	// exitChanged says a file was rewritten. prek reads it as a failed
	// hook, which is how a fixer tells the author to look.
	exitChanged = 1

	// exitRefused says the program cannot fix what it found.
	exitRefused = 2
)

func main() {
	var (
		dry     = flag.Bool("n", false, "report what would change; write nothing")
		wide    = flag.Int("wide", 70, "report a cell wider than this many runes; 0 turns the report off")
		quiet   = flag.Bool("q", false, "report nothing but refusals")
		paths   []string
		changed int
		refused int
		err     error
	)
	flag.Usage = usage
	flag.Parse()
	if paths = flag.Args(); len(paths) == 0 {
		usage()
		os.Exit(exitRefused)
	}
	for _, path := range paths {
		var n, r int
		if n, r, err = one(path, mdtable.Formatter{
			Wide: *wide,
		}, *dry, *quiet); err != nil {
			fmt.Fprintf(os.Stderr, "md-table-fmt: %v\n", err)
			refused++
			continue
		}
		changed += n
		refused += r
	}
	if refused > 0 {
		os.Exit(exitRefused)
	}
	if changed > 0 {
		os.Exit(exitChanged)
	}
}

// one formats a single file. It returns whether the file changed and
// whether it was refused, so that every file is looked at before the
// program decides its exit code -- a run over the whole repo reports
// every offending table, not the first.
func one(path string, f mdtable.Formatter, dry, quiet bool) (int, int, error) {
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

func usage() {
	fmt.Fprint(os.Stderr, `md-table-fmt aligns the tables in Markdown files.

usage: md-table-fmt [-n] [-q] [-wide N] file.md...

`)
	flag.PrintDefaults()
}
