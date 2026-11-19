package features

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// attachReference is what a call without the feature prints.
const attachReference = `repo-tools features attach <name or nnn> [tree] [--dry-run]

Place a feature on a free implementation tree, from any tree: the
branch, its checkout, the pair in the tree's env file, the tmux line.

  <name or nnn>  the feature's name, or its number, resolved on the
                 trunk. A name with the fix prefix needs no feature
                 directory; its branch starts at the trunk. A feature's
                 branch starts at the spec branch's tip.
  [tree]         one of the implementation trees. The default is the
                 first free one, in the configured order. Free means
                 parked on its namesake branch, with no change to a
                 tracked file.
  --dry-run      print each step prefixed would:; change nothing.

It refuses: an argument that matches no feature, or more than one; a
branch checked out outside the implementation trees; a named tree that
is busy; every tree busy. A branch already on an implementation tree is
not a refusal: it says where, and exits 0.
`

// AttachCmd is the command line of features attach.
type AttachCmd struct {
	DryRun bool     `help:"print each mutating step prefixed would:; run the refusal checks; change nothing"`
	Args   []string `opts:"mode=arg,name=arg,max=2" help:"the feature, as a name or a number, then an optional tree"`

	out io.Writer
	dir string
}

// NewAttachCmd returns the command line of features attach.
func NewAttachCmd() *AttachCmd {
	return &AttachCmd{
		out: os.Stdout,
		dir: ".",
	}
}

// Run places the feature on an implementation tree.
func (c *AttachCmd) Run() error {
	const cmd = "attach"
	var (
		cfg    *Config
		r      resolved
		tree   string
		treeP  string
		holder string
		made   string
		pair   string
		free   bool
		err    error
	)
	if len(c.Args) == 0 {
		return reference(c.out, attachReference)
	}
	if cfg, err = Load(c.dir); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if r, err = resolveFeature(cfg, c.Args[0]); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}

	// The branch may be attached already: then say where, and go no
	// further. Checked out outside the trees, it is not ours to move.
	if holder, err = treeOfBranch(cfg.Root, r.name); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if holder != "" {
		if roleOf(cfg.Implementation, holder) == "" {
			return refuse(c.out, cmd, "%s is checked out in %s; detach it there first", r.name, holder)
		}
		fmt.Fprintf(c.out, "features attach: %s is attached to %s already\n", r.name, holder)
		fmt.Fprintf(c.out, "  tmux new -s %s -c %s\n", r.name, holder)
		return nil
	}

	// Pick a free tree, or take the one named.
	if len(c.Args) > 1 {
		tree = c.Args[1]
		if roleOf(cfg.Implementation, tree) == "" {
			return refuse(c.out, cmd, "the tree is %s, not %s", joinOr(cfg.Implementation), tree)
		}
		if treeP, free, err = freeTree(cfg, tree); err != nil {
			return refuse(c.out, cmd, "%v", err)
		}
		if treeP == "" {
			return refuse(c.out, cmd, "no worktree is the %s tree; git worktree list names them", tree)
		}
		if !free {
			var on, _ = currentBranch(treeP)
			return refuse(c.out, cmd, "%s is busy, on %s; park it or name another tree", treeP, on)
		}
	} else {
		if tree, treeP, err = pickFreeTree(cfg); err != nil {
			return refuse(c.out, cmd, "%v", err)
		}
		if tree == "" {
			return refuse(c.out, cmd, "all the implementation trees are busy; land or park one first")
		}
	}

	// The branch, its checkout, and the assignment record.
	if !branchExists(cfg.Root, r.name) {
		if c.DryRun {
			wouldGit(c.out, cfg.Root, "branch", r.name, r.start)
		} else if err = gitRun(c.out, cfg.Root, "branch", r.name, r.start); err != nil {
			return refuse(c.out, cmd, "git branch %s %s failed", r.name, r.start)
		}
		made = fmt.Sprintf(" (the branch is new, at %s's tip)", r.start)
	}
	if c.DryRun {
		wouldGit(c.out, treeP, "switch", "-q", r.name)
	} else if err = gitRun(c.out, treeP, "switch", "-q", r.name); err != nil {
		return refuse(c.out, cmd, "git switch %s failed in %s", r.name, treeP)
	}
	pair = filepath.Join(treeP, cfg.EnvFile)
	if c.DryRun {
		wouldf(c.out, "write export %s=%s and export %s=%s into %s",
			cfg.EnvNumber, r.number, cfg.EnvName, r.name, pair)
	} else if err = writeEnvPair(pair, cfg, r.number, r.name); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	fmt.Fprintf(c.out, "features attach: %s is on %s%s; work in a tmux there:\n", r.name, treeP, made)
	fmt.Fprintf(c.out, "  tmux new -s %s -c %s\n", r.name, treeP)
	return nil
}
