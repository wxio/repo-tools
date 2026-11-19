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
// feature moves through them.
const loop = `The feature loop, in the order a feature moves:

  new <name>                reserve the next feature number on the trunk
  attach <name|nnn> [tree]  place the feature on a free implementation tree
  scenario <name>           write a todo scenario in the current tree
  refresh                   merge the trunk into the branch; run the branch gate
  merge <branch>            merge the branch in the merge tree; the first stage
  verify [branch]           run the verify checks there; the second stage
  land <branch>             fast-forward the trunk, tag, push, park, delete
  finish <branch>           merge, verify and land; stop at the first refusal

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
