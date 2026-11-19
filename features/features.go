// Package features is the features subcommand: the lifecycle of a
// feature branch across the standing worktrees of a repository.
//
//	repo-tools features new <name>
//	repo-tools features attach <name or nnn> [tree]
//	repo-tools features scenario <name>
//	repo-tools features refresh
//	repo-tools features merge <branch>
//	repo-tools features verify [branch]
//	repo-tools features land <branch>
//	repo-tools features finish <branch>
//
// The host repository describes itself in one checked-in file at its
// root, .repo-tools-features.json; see Config. The commands find the
// standing trees through `git worktree list` and map each role by its
// directory name, so no path lives in the file.
//
// Every mutating subcommand takes --dry-run: it prints each mutating
// step prefixed "would:", runs the refusal checks for real, changes
// nothing, and exits non-zero on a refusal.
//
// The point of the family is the tab completion. `attach <tab>` lists
// the features and then the trees, the free trees first; `merge <tab>`,
// `verify`, `land` and `finish` list the open branches. opts completes
// an argument as a file name, so main answers the shell first; see
// internal/argcomplete.
package features

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/wxio/repo-tools/internal/exitcode"
)

// kebab is the shape of a feature name: lower-case words, dash-joined.
var kebab = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// isNumber matches a feature number of one to three digits.
var isNumber = regexp.MustCompile(`^[0-9]{1,3}$`)

// featureDir matches the directory of one feature, <nnn>-<name>.
var featureDir = regexp.MustCompile(`^[0-9]{3}-.+$`)

// FeaturesCmd is the bare features command. With no subcommand it
// prints the loop, one line per subcommand, in the order a feature
// moves, and where the configuration lives. It exits 0.
type FeaturesCmd struct {
	out io.Writer
}

// NewFeaturesCmd returns the bare features command.
func NewFeaturesCmd() *FeaturesCmd {
	return &FeaturesCmd{
		out: os.Stdout,
	}
}

// loop is what the bare command prints: the subcommands in the order a
// feature moves through them, the roles of the standing trees, where
// each command runs, and the steps between the commands that stay the
// user's own. The commands are half the workflow; a user who reads only
// the subcommand list still does not know which tree to stand in.
const loop = `The feature loop, in the order a feature moves:

  new <name>                reserve the next feature number on the trunk
  attach <name|nnn> [tree]  place the feature on a free implementation tree
  scenario <name>           write a todo scenario in the current tree
  refresh                   merge the trunk into the branch; run the branch gate
  merge <branch>            merge the branch in the merge tree; the first stage
  verify [branch]           run the verify checks there; the second stage
  land <branch>             fast-forward the trunk, tag, push, park, delete
  finish <branch>           merge, verify and land; stop at the first refusal

The trees stand; the branches move through them. One tree has one role,
and an implementation tree holds one feature at a time:

  the trunk's checkout    the trunk      reserve a number; land moves it
  the spec tree           the spec       the PRD, the plan, the docs
  an implementation tree  the feature's  one feature, attach to land
  the merge tree          detached       the finish only; do not edit

Run each command where it belongs:

  new, attach, land, finish  any tree: each finds the tree it needs
  refresh                    the attached tree, on the branch, committed
  scenario                   the current tree

The steps between the commands are yours:

  1. Run new, then write the PRD and the plan in the spec tree.
  2. Commit the spec on the spec branch. attach starts the branch there,
     so the spec travels with the feature and lands with it.
  3. Run attach, then implement in that tree. Run refresh after each
     chunk: it brings the trunk in early, so the merge holds no surprise.
  4. Run finish, then merge the trunk into the spec branch. The next
     feature branch then starts near the trunk's tip.

A fix skips the spec, not the merge. A branch under the fix prefix needs
no number and no feature directory, and starts at the trunk:

  attach fix-<descr>
  finish fix-<descr> --no-scenario "<reason>"

The repository describes itself in ` + ConfigName + ` at its root;
the commands find it by walking up from the working directory.

Every mutating subcommand takes --dry-run: it prints each mutating step
prefixed "would:", runs the refusal checks for real, and changes nothing.
`

// Run prints the loop.
func (c *FeaturesCmd) Run() error {
	fmt.Fprint(c.out, loop)
	return nil
}

// refuse prints one refusal, prefixed by the subcommand, and returns
// the error that sets the exit code to 1. The command has said why, so
// main adds nothing.
func refuse(w io.Writer, cmd, format string, a ...any) error {
	fmt.Fprintf(w, "features %s: %s\n", cmd, fmt.Sprintf(format, a...))
	return exitcode.Code(1)
}

// reference prints a subcommand's short reference, for a call that
// misses a required argument, and returns the exit-code error.
func reference(w io.Writer, text string) error {
	fmt.Fprint(w, text)
	return exitcode.Code(1)
}

// wouldf prints one dry-run line: the mutating step, prefixed "would:".
func wouldf(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "would: %s\n", fmt.Sprintf(format, a...))
}

// wouldGit prints the dry-run line of one git command.
func wouldGit(w io.Writer, dir string, args ...string) {
	wouldf(w, "git -C %s %s", dir, strings.Join(args, " "))
}

// expand replaces each placeholder of a configured template: expand(s,
// "{name}", name) and so on, pairwise.
func expand(s string, pairs ...string) string {
	return strings.NewReplacer(pairs...).Replace(s)
}

// joinOr writes a list for a refusal: "alpha, beta or gamma".
func joinOr(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}
