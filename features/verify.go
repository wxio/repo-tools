package features

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// VerifyCmd is the command line of features verify. The branch is
// optional: it defaults to the branch the state file holds, and asserts
// which merge is verified when given.
type VerifyCmd struct {
	DryRun   bool     `help:"name the checks prefixed would:; run none of them; change nothing"`
	Branches []string `opts:"mode=arg,name=branch,max=1" help:"the branch the merge tree holds (default: the recorded one)"`

	out io.Writer
	dir string
}

// NewVerifyCmd returns the command line of features verify.
func NewVerifyCmd() *VerifyCmd {
	return &VerifyCmd{
		out: os.Stdout,
		dir: ".",
	}
}

// Run runs the verify checks in the merge tree: the second finish
// stage. On green it marks the merge verified in the state file that
// land reads. A red is fixed on the branch: refresh there, then merge
// again, which resets the merge tree first.
func (c *VerifyCmd) Run() error {
	const cmd = "verify"
	var (
		cfg   *Config
		state map[string]string
		m     string
		head  string
		err   error
	)
	if cfg, err = Load(c.dir); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if state, err = readState(cfg.Root); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if state["merged"] == "" {
		return refuse(c.out, cmd, "the merge tree holds no merge; run repo-tools features merge <branch> first")
	}
	if len(c.Branches) > 0 && c.Branches[0] != state["branch"] {
		return refuse(c.out, cmd, "the merge tree holds a merge of %s, not of %s; run repo-tools features merge %s",
			state["branch"], c.Branches[0], c.Branches[0])
	}
	if m, err = mergeTreePath(cfg, cfg.Root); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if head, err = gitOut(m, "rev-parse", "HEAD"); err != nil || head != state["merged"] {
		return refuse(c.out, cmd, "%s is not at the recorded merge commit; run repo-tools features merge %s again",
			m, state["branch"])
	}

	if c.DryRun {
		for _, argv := range cfg.Verify {
			wouldf(c.out, "%s, in %s", strings.Join(argv, " "), m)
		}
		wouldf(c.out, "mark the merge of %s verified, for features land", state["branch"])
		return nil
	}

	for _, argv := range cfg.Verify {
		if err = runArgv(c.out, m, argv); err != nil {
			return refuse(c.out, cmd, "%s failed; fix %s on its branch, refresh there, then repo-tools features merge again",
				strings.Join(argv, " "), state["branch"])
		}
	}
	if err = writeState(cfg.Root,
		"branch", state["branch"],
		"merged", state["merged"],
		"verified", state["merged"]); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	fmt.Fprintf(c.out, "features verify: the merge of %s is verified\n", state["branch"])
	fmt.Fprintf(c.out, "features verify: the next command is: repo-tools features land %s\n", state["branch"])
	return nil
}
