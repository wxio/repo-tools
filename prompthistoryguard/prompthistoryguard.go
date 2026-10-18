// Package prompthistoryguard is the prompt-history-guard subcommand. It keeps
// the prompt records — everything under .prompts/, and the transcripts under
// .prompt-history/ — in commits of their own.
//
// It runs as a local prek hook, before the commit is made, and does two things
// in one pass over the index:
//
//   - it takes the records back out of the index when the commit being made
//     holds anything else, so an ordinary `git add -A` followed by a commit of
//     code records the code and leaves the records in the working tree, still
//     there to stage afterwards;
//   - it refuses the commit when that mix is still there, which happens only
//     when the first step failed.
//
// A commit that stages ONLY records is the genuine article and is left alone,
// as is one that stages none. The two record directories may share a commit.
//
// The transcripts are written by the Claude Code Stop hook,
// prompt-history-record; .prompts/output/ holds a copy of each final reply.
// They are a record of how the work was done, not part of it, so they belong
// in commits of their own: `git log -- <path>` over the real tree then stays
// readable, and a record commit can be dropped or rewritten without disturbing
// any code change.
package prompthistoryguard

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/wxio/repo-tools/internal/exitcode"
)

// recordPrefixes are the directories that have to be committed alone.
// .prompt-history/ is where prompt-history-record writes today; it stays
// guarded until the transcripts move under .prompts/.
var recordPrefixes = []string{
	".prompts/",
	".prompt-history/",
}

const (
	// listLimit caps how many paths each side of the report prints, so that a
	// large commit does not bury the explanation under its own file list.
	listLimit = 10
)

// PromptHistoryGuardCmd is the command line of prompt-history-guard. It has
// no flags: it asks git what the commit stages.
type PromptHistoryGuardCmd struct{}

// NewPromptHistoryGuardCmd returns the command line of prompt-history-guard.
func NewPromptHistoryGuardCmd() *PromptHistoryGuardCmd {
	return &PromptHistoryGuardCmd{}
}

func (g *PromptHistoryGuardCmd) Run() error {
	var (
		paths   []string
		records []string
		other   []string
		report  string
		err     error
	)
	if paths, err = stagedPaths(); err != nil {
		return guardFailed(err)
	}
	if records, other = partition(paths); len(records) == 0 || len(other) == 0 {
		return nil
	}

	// The commit mixes the two. Take the records out of the index and look
	// again: the unstage is what makes the commit legal, and the report below
	// is for the case where it did not.
	if err = unstage(records, other); err != nil {
		return guardFailed(err)
	}
	if paths, err = stagedPaths(); err != nil {
		return guardFailed(err)
	}
	if report = check(paths); report != "" {
		fmt.Fprint(os.Stderr, report)
		return exitcode.Code(1)
	}
	return nil
}

// guardFailed reports a failure of the hook itself, rather than of the commit.
func guardFailed(err error) error {
	fmt.Fprintf(os.Stderr, "prompt-history-guard: %v\n", err)
	return exitcode.Code(2)
}

// partition splits the staged paths into the records and everything else.
// The commit mixes the two when both sides come back non-empty.
func partition(paths []string) (records, other []string) {
	for _, p := range paths {
		if recordDir(p) != "" {
			records = append(records, p)
			continue
		}
		other = append(other, p)
	}
	return records, other
}

// recordDir returns the record directory that holds p, without its trailing
// slash, and "" when p is not a record.
func recordDir(p string) string {
	for _, prefix := range recordPrefixes {
		if strings.HasPrefix(p, prefix) {
			return strings.TrimSuffix(prefix, "/")
		}
	}
	return ""
}

// recordDirs returns each record directory that holds one of records, once,
// in the order of recordPrefixes.
func recordDirs(records []string) []string {
	var (
		seen = map[string]bool{}
		dirs []string
	)
	for _, p := range records {
		seen[recordDir(p)] = true
	}
	for _, prefix := range recordPrefixes {
		var dir = strings.TrimSuffix(prefix, "/")
		if seen[dir] {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// unstage drops the records from the index. The working tree is untouched —
// only the index entry goes — so the records are still there to be staged
// again afterwards.
//
// It is `git reset -q --` rather than the newer `git restore --staged`, which
// refuses a repository whose HEAD is unborn: the first commit of a fresh clone
// is exactly where a hook must not fall over.
func unstage(records, other []string) error {
	var (
		dirs = recordDirs(records)
		cmd  = exec.Command("git", append([]string{"reset", "-q", "--"}, dirs...)...)
		out  []byte
		err  error
	)
	if out, err = cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git reset -- %s: %w: %s",
			strings.Join(dirs, " "), err, strings.TrimSpace(string(out)))
	}
	fmt.Fprintf(os.Stderr, "unstaged %d file(s) under %s, leaving %d other staged file(s) to commit\n",
		len(records), strings.Join(dirs, " "), len(other))
	return nil
}

// check returns the message to show when paths mixes the records with
// anything else, and "" when the commit is fine. A commit that is all
// records, or holds none at all, passes.
func check(paths []string) string {
	var (
		records []string
		other   []string
		dirs    []string
		b       strings.Builder
	)
	if records, other = partition(paths); len(records) == 0 || len(other) == 0 {
		return ""
	}

	dirs = recordDirs(records)
	b.WriteString("The files under " + strings.Join(dirs, " and ") + " have to be committed on their own.\n\n")
	writeList(&b, "record(s)", records)
	writeList(&b, "other file(s)", other)
	b.WriteString("\nStage one side at a time, for example:\n\n")
	b.WriteString("    git restore --staged " + strings.Join(dirs, " ") + "\n")
	return b.String()
}

// writeList renders one side of the report, truncated to listLimit paths.
func writeList(b *strings.Builder, label string, paths []string) {
	fmt.Fprintf(b, "  %d staged %s:\n", len(paths), label)
	for _, p := range paths[:min(len(paths), listLimit)] {
		fmt.Fprintf(b, "    %s\n", p)
	}
	if len(paths) > listLimit {
		fmt.Fprintf(b, "    ... and %d more\n", len(paths)-listLimit)
	}
}

// stagedPaths lists what the commit about to be made will contain.
//
// It asks git rather than reading the file list prek hands the hook: prek
// splits a long list over several invocations, and a batch that happened to
// hold only records would look clean on its own. --no-renames keeps every
// entry a single path, so the -z stream needs no further parsing.
func stagedPaths() ([]string, error) {
	var (
		cmd    = exec.Command("git", "diff", "--cached", "--name-only", "--no-renames", "-z")
		stderr bytes.Buffer
		out    []byte
		err    error
	)
	cmd.Stderr = &stderr
	if out, err = cmd.Output(); err != nil {
		return nil, fmt.Errorf("git diff --cached: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return splitNUL(out), nil
}

// splitNUL splits git's -z output, which terminates — rather than separates —
// each path with a NUL.
func splitNUL(out []byte) []string {
	var paths []string
	for _, p := range bytes.Split(out, []byte{0}) {
		if len(p) == 0 {
			continue
		}
		paths = append(paths, string(p))
	}
	return paths
}
