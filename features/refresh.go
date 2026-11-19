package features

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/wxio/repo-tools/internal/exitcode"
)

// RefreshCmd is the command line of features refresh. It takes no
// argument: the branch is the current branch.
type RefreshCmd struct {
	DryRun bool `help:"name the conflicts and the steps prefixed would:; change nothing"`

	out io.Writer
	dir string
}

// NewRefreshCmd returns the command line of features refresh.
func NewRefreshCmd() *RefreshCmd {
	return &RefreshCmd{
		out: os.Stdout,
		dir: ".",
	}
}

// Run merges the local trunk into the branch, then runs the branch
// gate. It does not fetch origin. It prints `scenario: todo <branch>`
// when the branch changes the watched directory and its scenario has no
// steps; that line is a warning here, and merge's refusal.
func (c *RefreshCmd) Run() error {
	const cmd = "refresh"
	var (
		cfg       *Config
		branch    string
		pairName  string
		conflicts []string
		clean     bool
		red       bool
		err       error
	)
	if cfg, err = Load(c.dir); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if branch, err = currentBranch(cfg.Root); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if branch == "" || branch == cfg.Trunk {
		return refuse(c.out, cmd, "run in a branch's worktree, not on %s", cfg.Trunk)
	}
	// The pair and the branch are two records of one fact. Only an
	// implementation tree carries a pair: the spec tree and the trunk's
	// checkout have none, so the check runs nowhere else.
	if roleOf(cfg.Implementation, cfg.Root) != "" {
		pairName = envPairValue(filepath.Join(cfg.Root, cfg.EnvFile), cfg.EnvName)
		if pairName != "" && pairName != branch {
			return refuse(c.out, cmd, "the branch is %s, and %s is %s; this tree is attached to another feature",
				branch, cfg.EnvName, pairName)
		}
	}
	if clean, err = isClean(cfg.Root); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if !clean {
		return refuse(c.out, cmd, "the tree has changes; commit or stash them first")
	}

	if c.DryRun {
		if conflicts, err = mergeConflicts(cfg.Root, cfg.Trunk, "HEAD"); err != nil {
			return refuse(c.out, cmd, "%v", err)
		}
		if len(conflicts) > 0 {
			fmt.Fprintln(c.out, strings.Join(conflicts, "\n"))
			return refuse(c.out, cmd, "the merge of %s would conflict in the files above; resolve them on the branch", cfg.Trunk)
		}
		wouldGit(c.out, cfg.Root, "merge", "--no-edit", cfg.Trunk)
		for _, argv := range cfg.BranchGate {
			wouldf(c.out, "%s, in %s", strings.Join(argv, " "), cfg.Root)
		}
		c.printScenarioTodo(cfg, branch)
		return nil
	}

	// The local trunk, not origin's: origin is not fetched, so a trunk
	// on the remote that differs from the local one never reaches a
	// branch.
	if err = gitRun(c.out, cfg.Root, "merge", "-q", "--no-edit", cfg.Trunk); err != nil {
		var files, _ = gitOut(cfg.Root, "diff", "--name-only", "--diff-filter=U")
		fmt.Fprintln(c.out, files)
		return refuse(c.out, cmd, "a conflict in the files above; resolve them, run the branch gate, SKIP=prompt-history-guard git merge --continue, then refresh again")
	}
	for _, argv := range cfg.BranchGate {
		if err = runArgv(c.out, cfg.Root, argv); err != nil {
			red = true
			break
		}
	}
	c.printScenarioTodo(cfg, branch)
	// The gate said why it is red, so nothing is added here.
	if red {
		return exitcode.Code(1)
	}
	return nil
}

// printScenarioTodo prints the scenario warning when the branch needs
// steps, and reports whether it did.
func (c *RefreshCmd) printScenarioTodo(cfg *Config, branch string) bool {
	var todo, err = scenarioIsTodo(cfg, branch, "HEAD")
	if err != nil || !todo {
		return false
	}
	fmt.Fprintf(c.out, "scenario: todo %s\n", branch)
	return true
}
