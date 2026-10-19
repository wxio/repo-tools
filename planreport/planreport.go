// Package planreport is the plan-report subcommand. It counts the open and
// the done tasks of the Markdown files under a directory tree, and prints
// one table row per feature:
//
//	repo-tools plan-report --dir docs/features --file-match '^plan\.md$'
//
// A task is a list item that starts with a checkbox: `- [ ]` is open and
// `- [x]` is done. A checkbox inside a fenced code block is an example,
// not a task, and is not counted.
//
// A feature is a directory. The files that match -file-match in one
// directory are summed into one row, named by the path of the directory
// relative to -dir. A file in -dir itself gets a row of its own, named
// by the file.
//
// The table is Markdown, aligned the way md-table-fmt aligns it, so it
// pastes into a page as it is.
package planreport

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/wxio/repo-tools/internal/exitcode"
	"github.com/wxio/repo-tools/internal/mdtable"
)

// exitRefused says the program could not read what it was given.
const exitRefused exitcode.Code = 2

// task matches a list item that starts with a checkbox. Group 1 is the
// mark inside the brackets: a space is open, an x is done. A space or
// the end of the line must follow the checkbox, as GitHub requires.
var task = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+\[( |[xX])\](?:\s|$)`)

// PlanReportCmd is the command line of plan-report.
type PlanReportCmd struct {
	Dir       string `help:"the directory tree to read"`
	FileMatch string `help:"a regexp; a file whose name matches it is read"`
}

// NewPlanReportCmd returns the command line of plan-report, with the
// defaults of its flags set.
func NewPlanReportCmd() *PlanReportCmd {
	return &PlanReportCmd{
		Dir:       ".",
		FileMatch: `\.md$`,
	}
}

func (c *PlanReportCmd) Run() error {
	var (
		match *regexp.Regexp
		rows  []Row
		err   error
	)
	if match, err = regexp.Compile(c.FileMatch); err != nil {
		fmt.Fprintf(os.Stderr, "plan-report: -file-match: %v\n", err)
		return exitRefused
	}
	if rows, err = Report(c.Dir, match); err != nil {
		fmt.Fprintf(os.Stderr, "plan-report: %v\n", err)
		return exitRefused
	}
	fmt.Print(Table(rows))
	return nil
}

// Row is one line of the report: the tasks of one feature.
type Row struct {
	Feature string
	Open    int
	Done    int
}

// Total is the number of tasks, open and done.
func (r Row) Total() int {
	return r.Open + r.Done
}

// Percent is the share of the tasks that are done, rounded to a whole
// number. A feature with no task is 0.
func (r Row) Percent() int {
	var total = r.Total()
	if total == 0 {
		return 0
	}
	return (r.Done*200 + total) / (2 * total)
}

// Report walks dir, reads every file whose name matches match, and sums
// the tasks per directory. The rows come back sorted by feature.
func Report(dir string, match *regexp.Regexp) ([]Row, error) {
	var (
		byFeature = map[string]*Row{}
		rows      []Row
		err       error
	)
	if err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		var (
			feature string
			open    int
			done    int
			src     []byte
		)
		if err != nil {
			return err
		}
		if d.IsDir() || !match.MatchString(d.Name()) {
			return nil
		}
		if feature, err = featureOf(dir, path); err != nil {
			return err
		}
		if src, err = os.ReadFile(path); err != nil {
			return err
		}
		open, done = Count(string(src))
		if byFeature[feature] == nil {
			byFeature[feature] = &Row{
				Feature: feature,
			}
		}
		byFeature[feature].Open += open
		byFeature[feature].Done += done
		return nil
	}); err != nil {
		return nil, err
	}
	for _, r := range byFeature {
		rows = append(rows, *r)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Feature < rows[j].Feature
	})
	return rows, nil
}

// featureOf names the feature a file belongs to: the directory of the
// file, relative to dir. A file in dir itself is named by the file.
func featureOf(dir, path string) (string, error) {
	var (
		rel string
		err error
	)
	if rel, err = filepath.Rel(dir, path); err != nil {
		return "", err
	}
	if filepath.Dir(rel) == "." {
		return filepath.ToSlash(rel), nil
	}
	return filepath.ToSlash(filepath.Dir(rel)), nil
}

// Count counts the open and the done tasks in a Markdown source. A
// checkbox inside a fenced code block is not counted.
func Count(src string) (open, done int) {
	var (
		sc    = bufio.NewScanner(strings.NewReader(src))
		fence string
		m     []string
	)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		var line = sc.Text()
		if f := fenceOf(line); f != "" {
			if fence == "" {
				fence = f
			} else if strings.HasPrefix(f, fence) {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		if m = task.FindStringSubmatch(line); m == nil {
			continue
		}
		if m[1] == " " {
			open++
		} else {
			done++
		}
	}
	return open, done
}

// fenceOf returns the fence a line opens or closes: three or more
// backticks or tildes, after up to three spaces. It returns "" for a
// line that is not a fence.
func fenceOf(line string) string {
	var (
		s = strings.TrimLeft(line, " ")
		n int
		c byte
	)
	if len(line)-len(s) > 3 || s == "" {
		return ""
	}
	c = s[0]
	if c != '`' && c != '~' {
		return ""
	}
	for n = 0; n < len(s) && s[n] == c; n++ {
	}
	if n < 3 {
		return ""
	}
	return s[:n]
}

// Table renders the rows as an aligned Markdown table.
func Table(rows []Row) string {
	var b strings.Builder
	b.WriteString("| feature | open | done | total | percentage complete |\n")
	b.WriteString("|---|---|---|---|---|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d%% |\n",
			r.Feature, r.Open, r.Done, r.Total(), r.Percent())
	}
	return mdtable.Formatter{}.Format(b.String()).Text
}
