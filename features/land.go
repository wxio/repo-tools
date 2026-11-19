package features

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// landReference is what a call without the branch prints.
const landReference = `repo-tools features land <branch> [--dry-run]

The third finish stage, from any tree. It fast-forwards the trunk to
the verified merge commit, tags it, and pushes the trunk and the tag in
one atomic push. Then it parks the implementation tree that held the
branch, removes the pair from that tree's env file, and deletes the
branch. The tag is the durable record.

  <branch>   the branch the merge tree holds, verified.
  --dry-run  print each step prefixed would:; change nothing.

It refuses: a missing branch; a merge tree without a verified merge of
the branch; a branch or a trunk that moved since the merge; a tag that
exists; a trunk checkout with changes. A refused push is not a refusal:
the trunk and the tag stay here, and the last line gives the later push
command.
`

// LandCmd is the command line of features land.
type LandCmd struct {
	DryRun   bool     `help:"print each mutating step prefixed would:; run the refusal checks; change nothing"`
	Branches []string `opts:"mode=arg,name=branch,max=1" help:"the branch to land"`

	out io.Writer
	dir string
}

// NewLandCmd returns the command line of features land.
func NewLandCmd() *LandCmd {
	return &LandCmd{
		out: os.Stdout,
		dir: ".",
	}
}

// Run lands the verified merge on the trunk.
func (c *LandCmd) Run() error {
	const cmd = "land"
	var (
		cfg    *Config
		state  map[string]string
		branch string
		m      string
		mt     string
		tag    string
		holder string
		parked string
		short  string
		out    string
		pushed = true
		clean  bool
		err    error
	)
	if len(c.Branches) == 0 {
		return reference(c.out, landReference)
	}
	branch = c.Branches[0]
	if cfg, err = Load(c.dir); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if !branchExists(cfg.Root, branch) {
		return refuse(c.out, cmd, "there is no branch %s", branch)
	}

	// The verified merge. merge wrote the state; verify marked it. The
	// branch and the trunk must still be what was merged.
	if state, err = readState(cfg.Root); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if state["branch"] != branch {
		return refuse(c.out, cmd, "the merge tree holds no merge of %s; run repo-tools features merge %s", branch, branch)
	}
	if state["verified"] == "" || state["verified"] != state["merged"] {
		return refuse(c.out, cmd, "the merge of %s is not verified; run repo-tools features verify first", branch)
	}
	if m, err = mergeTreePath(cfg, cfg.Root); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if out, err = gitOut(m, "rev-parse", "HEAD"); err != nil || out != state["merged"] {
		return refuse(c.out, cmd, "%s is not at the verified merge commit; run repo-tools features merge %s again", m, branch)
	}
	if out, err = gitOut(m, "rev-parse", "HEAD^2"); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if head, _ := gitOut(cfg.Root, "rev-parse", "refs/heads/"+branch); head != out {
		return refuse(c.out, cmd, "%s moved since the merge; run repo-tools features merge %s again", branch, branch)
	}
	if out, err = gitOut(m, "rev-parse", "HEAD^1"); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if head, _ := gitOut(cfg.Root, "rev-parse", "refs/heads/"+cfg.Trunk); head != out {
		return refuse(c.out, cmd, "%s moved since the merge; run repo-tools features merge %s again", cfg.Trunk, branch)
	}
	tag = mergeTagName(cfg, branch)
	if tagExists(cfg.Root, tag) {
		return refuse(c.out, cmd, "the tag %s exists", tag)
	}
	if mt, err = trunkTree(cfg, cfg.Root); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if clean, err = isClean(mt); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if !clean {
		return refuse(c.out, cmd, "the %s checkout %s has changes; commit or stash them first, since %s moves there",
			cfg.Trunk, mt, cfg.Trunk)
	}

	if c.DryRun {
		wouldGit(c.out, mt, "merge", "-q", "--ff-only", state["merged"])
		wouldGit(c.out, mt, "tag", tag, state["merged"])
		wouldGit(c.out, mt, "push", "--atomic", "origin", cfg.Trunk, "refs/tags/"+tag)
	} else {
		if err = gitRun(c.out, mt, "merge", "-q", "--ff-only", state["merged"]); err != nil {
			return refuse(c.out, cmd, "the fast-forward of %s failed", cfg.Trunk)
		}
		if err = gitRun(c.out, mt, "tag", tag, state["merged"]); err != nil {
			return refuse(c.out, cmd, "the tag %s failed", tag)
		}
		// The push sends what the trunk here already holds. A plain
		// push never forces: when origin has commits the trunk lacks,
		// origin refuses it and stays as it was.
		if err = gitRun(c.out, mt, "push", "-q", "--atomic", "origin", cfg.Trunk, "refs/tags/"+tag); err != nil {
			pushed = false
		}
	}

	// Park the implementation tree that held the branch, and delete
	// the branch. The spec branch, and a branch outside the trees,
	// stays: it is not parked, and a checked-out branch cannot be
	// deleted.
	if holder, err = treeOfBranch(cfg.Root, branch); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if role := roleOf(cfg.Implementation, holder); holder != "" && role != "" {
		if c.DryRun {
			wouldGit(c.out, holder, "switch", "-q", role)
			wouldf(c.out, "remove the %s and %s lines from %s",
				cfg.EnvNumber, cfg.EnvName, filepath.Join(holder, cfg.EnvFile))
		} else {
			if err = gitRun(c.out, holder, "switch", "-q", role); err != nil {
				return refuse(c.out, cmd, "the park of %s on %s failed", holder, role)
			}
			if err = stripEnvPair(filepath.Join(holder, cfg.EnvFile), cfg); err != nil {
				return refuse(c.out, cmd, "%v", err)
			}
		}
		parked = fmt.Sprintf("%s is parked on %s; ", holder, role)
		holder = ""
	}
	if holder != "" {
		fmt.Fprintf(c.out, "features land: the branch %s stays: it is checked out in %s\n", branch, holder)
	} else {
		if c.DryRun {
			wouldGit(c.out, mt, "branch", "-q", "-d", branch)
		} else if err = gitRun(c.out, mt, "branch", "-q", "-d", branch); err != nil {
			return refuse(c.out, cmd, "the delete of %s failed", branch)
		}
		parked = parked + branch + " is deleted; the tag is the record"
	}
	if c.DryRun {
		wouldf(c.out, "remove the state file and the merge message file")
	} else if err = removeState(cfg.Root, branch); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}

	short, _ = gitOut(cfg.Root, "rev-parse", "--short", state["merged"])
	switch {
	case c.DryRun:
		fmt.Fprintf(c.out, "features land: %s would land as %s, tagged %s\n", branch, short, tag)
	case pushed:
		fmt.Fprintf(c.out, "features land: %s is merged as %s, tagged %s, and pushed\n", branch, short, tag)
	default:
		fmt.Fprintf(c.out, "features land: %s is merged as %s and tagged %s on %s here; the push was refused, so origin is unchanged; push later with: git push origin %s %s\n",
			branch, short, tag, cfg.Trunk, cfg.Trunk, tag)
	}
	if parked != "" {
		fmt.Fprintf(c.out, "features land: %s\n", parked)
	}
	return nil
}
