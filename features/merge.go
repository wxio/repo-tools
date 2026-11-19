package features

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// mergeReference is what a call without the branch prints.
const mergeReference = `repo-tools features merge <branch> [--no-scenario "<reason>"]
                          [--message-file <path>] [--dry-run]

The first finish stage: reset the merge tree to the local trunk, and
merge the branch there, --no-ff, with the merge message file. It
records the merge in the state file that verify and land read. It does
not fetch. A conflict is fixed on the branch, never here.

  <branch>              the branch to merge; it must hold the trunk.
  --no-scenario         let a todo scenario pass; the reason goes into
                        the merge message as "No scenario: <reason>".
  --message-file        the merge message. The default is
                        <git common dir>/feature-merge/<branch>.msg.
  --dry-run             name the conflicts and the steps prefixed
                        would:; change nothing.

It refuses: the trunk as the branch; a missing branch; a branch behind
the trunk, or with nothing new; a todo scenario with no reason; a merge
tag that exists; a missing merge message; a conflict.
`

// MergeCmd is the command line of features merge.
type MergeCmd struct {
	NoScenario  string   `help:"let a todo scenario pass, for this reason"`
	MessageFile string   `help:"the merge message file (default: <git common dir>/feature-merge/<branch>.msg)"`
	DryRun      bool     `help:"name the conflicts and the steps prefixed would:; change nothing"`
	Branches    []string `opts:"mode=arg,name=branch,max=1" help:"the branch to merge"`

	out io.Writer
	dir string
}

// NewMergeCmd returns the command line of features merge.
func NewMergeCmd() *MergeCmd {
	return &MergeCmd{
		out: os.Stdout,
		dir: ".",
	}
}

// Run merges the branch in the merge tree.
func (c *MergeCmd) Run() error {
	const cmd = "merge"
	var (
		cfg       *Config
		branch    string
		tag       string
		msg       string
		m         string
		head      string
		short     string
		revlist   string
		content   []byte
		conflicts []string
		todo      bool
		info      os.FileInfo
		err       error
	)
	if len(c.Branches) == 0 {
		return reference(c.out, mergeReference)
	}
	branch = c.Branches[0]
	if cfg, err = Load(c.dir); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if branch == cfg.Trunk {
		return refuse(c.out, cmd, "%s is not a branch to merge", cfg.Trunk)
	}
	if !branchExists(cfg.Root, branch) {
		return refuse(c.out, cmd, "there is no branch %s", branch)
	}

	// The refusals: a branch behind the trunk, and a scenario that is
	// still a todo, with no reason. The trunk is the local one.
	if _, err = gitOut(cfg.Root, "merge-base", "--is-ancestor", cfg.Trunk, branch); err != nil {
		return refuse(c.out, cmd, "%s does not hold %s; run repo-tools features refresh in the branch first", branch, cfg.Trunk)
	}
	if revlist, err = gitOut(cfg.Root, "rev-list", cfg.Trunk+".."+branch); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if revlist == "" {
		return refuse(c.out, cmd, "%s has nothing that %s lacks", branch, cfg.Trunk)
	}
	if todo, err = scenarioIsTodo(cfg, branch, branch); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if todo && c.NoScenario == "" {
		return refuse(c.out, cmd, "scenario: todo %s; write its steps, or give --no-scenario \"<reason>\"", branch)
	}
	tag = mergeTagName(cfg, branch)
	if tagExists(cfg.Root, tag) {
		return refuse(c.out, cmd, "the tag %s exists", tag)
	}
	if msg = c.MessageFile; msg == "" {
		var sd string
		if sd, err = stateDir(cfg.Root); err != nil {
			return refuse(c.out, cmd, "%v", err)
		}
		msg = filepath.Join(sd, branch+".msg")
	}
	if info, err = os.Stat(msg); err != nil || info.Size() == 0 {
		return refuse(c.out, cmd, "no merge message: write it to %s, or pass --message-file <path>", msg)
	}
	if m, err = mergeTreePath(cfg, cfg.Root); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}

	if c.DryRun {
		if conflicts, err = mergeConflicts(cfg.Root, cfg.Trunk, branch); err != nil {
			return refuse(c.out, cmd, "%v", err)
		}
		if len(conflicts) > 0 {
			fmt.Fprintln(c.out, strings.Join(conflicts, "\n"))
			return refuse(c.out, cmd, "the merge would conflict in the files above; run repo-tools features refresh in the branch")
		}
		short, _ = gitOut(cfg.Root, "rev-parse", "--short", cfg.Trunk)
		wouldf(c.out, "reset %s to %s %s", m, cfg.Trunk, short)
		wouldf(c.out, "git -C %s merge --no-ff %s, with the message of %s", m, branch, msg)
		wouldf(c.out, "record the merge for features verify and features land")
		return nil
	}

	// The merge worktree. The path must be in `git worktree list`
	// before the first git -C, so a stale entry is pruned first.
	if _, err = gitOut(cfg.Root, "worktree", "prune"); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if isWorktree(cfg.Root, m) {
		gitRun(io.Discard, m, "merge", "--abort")
		if err = gitRun(c.out, m, "reset", "-q", "--hard", cfg.Trunk); err != nil {
			return refuse(c.out, cmd, "the reset of %s to %s failed", m, cfg.Trunk)
		}
	} else if _, err = os.Stat(m); err == nil {
		return refuse(c.out, cmd, "%s exists, and it is not a worktree of this repository", m)
	} else if err = gitRun(c.out, cfg.Root, "worktree", "add", "-q", "--detach", m, cfg.Trunk); err != nil {
		return refuse(c.out, cmd, "git worktree add %s failed", m)
	}
	if err = removeState(cfg.Root, ""); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	short, _ = gitOut(cfg.Root, "rev-parse", "--short", cfg.Trunk)
	fmt.Fprintf(c.out, "features merge: merging %s in %s, at %s %s\n", branch, m, cfg.Trunk, short)
	if err = gitRun(c.out, m, "merge", "-q", "--no-ff", "--no-commit", branch); err != nil {
		var files, _ = gitOut(m, "diff", "--name-only", "--diff-filter=U")
		fmt.Fprintln(c.out, files)
		gitRun(io.Discard, m, "merge", "--abort")
		return refuse(c.out, cmd, "the merge conflicts in the files above; run repo-tools features refresh in the branch")
	}

	// The branch's commits kept the transcripts in commits of their
	// own, so the merge commit holds both, and the guard hook is
	// skipped.
	if content, err = os.ReadFile(msg); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if c.NoScenario != "" {
		content = append(content, []byte("\nNo scenario: "+c.NoScenario+"\n")...)
	}
	if err = gitRunInput(c.out, m, string(content),
		[]string{"SKIP=prompt-history-guard"}, "commit", "-q", "-F", "-"); err != nil {
		gitRun(io.Discard, m, "merge", "--abort")
		return refuse(c.out, cmd, "a commit hook refused the merge commit; fix the file it names on the branch, then merge again")
	}
	if head, err = gitOut(m, "rev-parse", "HEAD"); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if err = writeState(cfg.Root, "branch", branch, "merged", head); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	short, _ = gitOut(m, "rev-parse", "--short", "HEAD")
	fmt.Fprintf(c.out, "features merge: %s is merged in %s as %s\n", branch, m, short)
	fmt.Fprintf(c.out, "features merge: the next command is: repo-tools features verify %s\n", branch)
	return nil
}

// isWorktree reports whether path is a worktree of the repository that
// holds dir.
func isWorktree(dir, path string) bool {
	var trees, err = worktrees(dir)
	if err != nil {
		return false
	}
	for _, t := range trees {
		if t.path == path {
			return true
		}
	}
	return false
}
