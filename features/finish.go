package features

import (
	"fmt"
	"io"
	"os"
)

// finishReference is what a call without the branch prints.
const finishReference = `repo-tools features finish <branch> [--no-scenario "<reason>"]
                           [--message-file <path>] [--dry-run]

The three finish stages in order, stopping at the first refusal: merge
resets the merge tree to the local trunk and merges the branch; verify
runs the configured checks there; land fast-forwards the trunk, pushes,
tags, parks the implementation tree and deletes the branch. Run the
stages apart to stop between them.

  <branch>        the branch to finish.
  --no-scenario   let a todo scenario pass; goes to the merge stage.
  --message-file  the merge message; goes to the merge stage.
  --dry-run       dry-run the merge stage, then name the other two: a
                  dry merge records nothing, so their own dry runs
                  would refuse.

Its refusals are the stages' own.
`

// FinishCmd is the command line of features finish.
type FinishCmd struct {
	NoScenario  string   `help:"let a todo scenario pass, for this reason"`
	MessageFile string   `help:"the merge message file (default: <git common dir>/feature-merge/<branch>.msg)"`
	DryRun      bool     `help:"dry-run the merge stage and name the other two; change nothing"`
	Branches    []string `opts:"mode=arg,name=branch,max=1" help:"the branch to finish"`

	out io.Writer
	dir string
}

// NewFinishCmd returns the command line of features finish.
func NewFinishCmd() *FinishCmd {
	return &FinishCmd{
		out: os.Stdout,
		dir: ".",
	}
}

// Run runs merge, verify and land in order, stopping at the first
// refusal.
func (c *FinishCmd) Run() error {
	var (
		branch string
		err    error
	)
	if len(c.Branches) == 0 {
		return reference(c.out, finishReference)
	}
	branch = c.Branches[0]
	if err = (&MergeCmd{
		NoScenario:  c.NoScenario,
		MessageFile: c.MessageFile,
		DryRun:      c.DryRun,
		Branches:    []string{branch},
		out:         c.out,
		dir:         c.dir,
	}).Run(); err != nil {
		return err
	}
	if c.DryRun {
		fmt.Fprintf(c.out, "would: repo-tools features verify %s\n", branch)
		fmt.Fprintf(c.out, "would: repo-tools features land %s\n", branch)
		return nil
	}
	if err = (&VerifyCmd{
		Branches: []string{branch},
		out:      c.out,
		dir:      c.dir,
	}).Run(); err != nil {
		return err
	}
	return (&LandCmd{
		Branches: []string{branch},
		out:      c.out,
		dir:      c.dir,
	}).Run()
}
