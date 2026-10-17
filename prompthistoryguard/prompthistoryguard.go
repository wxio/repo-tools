// Package prompthistoryguard is the prompt-history-guard subcommand. It keeps
// the .prompt-history/ transcripts in commits of their own.
//
// It runs as a local prek hook, before the commit is made, and does two things
// in one pass over the index:
//
//   - it takes the transcripts back out of the index when the commit being made
//     holds anything else, so an ordinary `git add -A` followed by a commit of
//     code records the code and leaves the transcripts in the working tree,
//     still there to stage afterwards;
//   - it refuses the commit when that mix is still there, which happens only
//     when the first step failed.
//
// A commit that stages ONLY transcripts is the genuine article and is left
// alone, as is one that stages none.
//
// The transcripts are written by the Claude Code Stop hook,
// prompt-history-record. They are a record of how the work was done,
// not part of it, so they belong in commits of their own: `git log -- <path>`
// over the real tree then stays readable, and a transcript commit can be
// dropped or rewritten without disturbing any code change.
package prompthistoryguard

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/wxio/repo-tools/internal/exitcode"
)

const (
	// promptHistoryPrefix is the one directory that has to be committed alone.
	promptHistoryPrefix = ".prompt-history/"

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
		history []string
		other   []string
		report  string
		err     error
	)
	if paths, err = stagedPaths(); err != nil {
		return guardFailed(err)
	}
	if history, other = partition(paths); len(history) == 0 || len(other) == 0 {
		return nil
	}

	// The commit mixes the two. Take the transcripts out of the index and look
	// again: the unstage is what makes the commit legal, and the report below
	// is for the case where it did not.
	if err = unstage(history, other); err != nil {
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

// partition splits the staged paths into the transcripts and everything else.
// The commit mixes the two when both sides come back non-empty.
func partition(paths []string) (history, other []string) {
	for _, p := range paths {
		if strings.HasPrefix(p, promptHistoryPrefix) {
			history = append(history, p)
			continue
		}
		other = append(other, p)
	}
	return history, other
}

// unstage drops the transcripts from the index. The working tree is untouched —
// only the index entry goes — so the transcripts are still there to be staged
// again afterwards.
//
// It is `git reset -q --` rather than the newer `git restore --staged`, which
// refuses a repository whose HEAD is unborn: the first commit of a fresh clone
// is exactly where a hook must not fall over.
func unstage(history, other []string) error {
	var (
		cmd = exec.Command("git", "reset", "-q", "--", strings.TrimSuffix(promptHistoryPrefix, "/"))
		out []byte
		err error
	)
	if out, err = cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git reset -- %s: %w: %s",
			promptHistoryPrefix, err, strings.TrimSpace(string(out)))
	}
	fmt.Fprintf(os.Stderr, "unstaged %d file(s) under %s, leaving %d other staged file(s) to commit\n",
		len(history), promptHistoryPrefix, len(other))
	return nil
}

// check returns the message to show when paths mixes .prompt-history/ with
// anything else, and "" when the commit is fine. A commit that is all
// transcripts, or holds none at all, passes.
func check(paths []string) string {
	var (
		history []string
		other   []string
		b       strings.Builder
	)
	if history, other = partition(paths); len(history) == 0 || len(other) == 0 {
		return ""
	}

	b.WriteString(promptHistoryPrefix + " has to be committed on its own.\n\n")
	writeList(&b, "transcript(s)", history)
	writeList(&b, "other file(s)", other)
	b.WriteString("\nStage one side at a time, for example:\n\n")
	b.WriteString("    git restore --staged " + strings.TrimSuffix(promptHistoryPrefix, "/") + "\n")
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
// hold only transcripts would look clean on its own. --no-renames keeps every
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
