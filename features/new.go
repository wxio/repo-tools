package features

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/wxio/repo-tools/internal/mdtable"
)

// newReference is what a call without the name prints.
const newReference = `repo-tools features new <name> [--dry-run]

Reserve the next feature number on the trunk, from any tree. It works
in the trunk's checkout: it pulls the trunk, writes the prd.md stub,
the README row and the todo scenario, then commits and pushes.

  <name>     the feature, in kebab-case. The number is the highest in
             the features directory plus one, padded to three digits.
  --dry-run  print each step prefixed would:; change nothing. It does
             not pull, so the number can be one behind origin.

It refuses: a name that is not kebab-case; no worktree on the trunk; a
trunk checkout with changes; a feature or a branch that exists; a
refused push, which undoes the commit.
`

// NewCmd is the command line of features new.
type NewCmd struct {
	DryRun bool     `help:"print each mutating step prefixed would:; run the refusal checks; change nothing"`
	Names  []string `opts:"mode=arg,name=name,max=1" help:"the feature, in kebab-case"`

	out io.Writer
	dir string
}

// NewNewCmd returns the command line of features new.
func NewNewCmd() *NewCmd {
	return &NewCmd{
		out: os.Stdout,
		dir: ".",
	}
}

// Run reserves the next feature number on the trunk.
func (c *NewCmd) Run() error {
	const cmd = "new"
	var (
		cfg      *Config
		name     string
		mt       string
		feature  string
		row      string
		scenario string
		msg      string
		clean    bool
		err      error
	)
	if len(c.Names) == 0 {
		return reference(c.out, newReference)
	}
	name = c.Names[0]
	if !kebab.MatchString(name) {
		return refuse(c.out, cmd, "give a kebab-case name: repo-tools features new <name>")
	}
	if cfg, err = Load(c.dir); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if mt, err = trunkTree(cfg, cfg.Root); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if clean, err = isClean(mt); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if !clean {
		return refuse(c.out, cmd, "the %s checkout %s has changes; commit or stash them first", cfg.Trunk, mt)
	}
	if c.DryRun {
		wouldGit(c.out, mt, "pull", "-q", "--ff-only", "origin", cfg.Trunk)
	} else if err = gitRun(c.out, mt, "pull", "-q", "--ff-only", "origin", cfg.Trunk); err != nil {
		return refuse(c.out, cmd, "the pull of %s from origin failed", cfg.Trunk)
	}
	if feature, err = nextFeature(cfg, mt, name); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if branchExists(cfg.Root, name) {
		return refuse(c.out, cmd, "the branch %s exists", name)
	}
	row = expand(cfg.ReadmeRow, "{feature}", feature)
	scenario = scenarioPath(cfg, name)
	msg = "features/" + feature + ": reserve the number"

	if c.DryRun {
		wouldf(c.out, "mkdir %s, and write its prd.md stub", filepath.Join(mt, cfg.FeaturesDir, feature))
		wouldf(c.out, "add the row %s to %s", row, cfg.FeaturesReadme)
		wouldf(c.out, "write %s as a todo scenario", scenario)
		wouldf(c.out, "git -C %s commit -m %q", mt, msg)
		wouldGit(c.out, mt, "push", "origin", cfg.Trunk)
		fmt.Fprintf(c.out, "features new: then the next command is: repo-tools features attach %s\n", name)
		return nil
	}

	if err = os.MkdirAll(filepath.Join(mt, cfg.FeaturesDir, feature), 0o755); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if err = os.WriteFile(filepath.Join(mt, cfg.FeaturesDir, feature, "prd.md"),
		fmt.Appendf(nil, "# %s\n\nStatus: the number is reserved. Nothing is written yet.\n", name),
		0o644); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if err = addReadmeRow(cfg, mt, row); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if _, err = writeScenario(cfg, mt, name); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	if err = gitRun(c.out, mt, "add",
		slashPath(cfg.FeaturesDir, feature, "prd.md"), cfg.FeaturesReadme, scenario); err != nil {
		return refuse(c.out, cmd, "git add failed")
	}
	if err = gitRun(c.out, mt, "commit", "-q", "-m", msg); err != nil {
		return refuse(c.out, cmd, "a commit hook refused the commit; fix the file it names")
	}
	if err = gitRun(c.out, mt, "push", "-q", "origin", cfg.Trunk); err != nil {
		gitRun(c.out, mt, "reset", "-q", "--keep", "HEAD~1")
		return refuse(c.out, cmd, "the push was refused, and the commit is undone; run features new again for the next number")
	}
	fmt.Fprintf(c.out, "features new: the number %s is reserved on %s and pushed\n", feature, cfg.Trunk)
	fmt.Fprintf(c.out, "features new: the next command is: repo-tools features attach %s\n", name)
	return nil
}

// slashPath joins path elements with slashes, for a path git takes
// to the tree.
func slashPath(elem ...string) string {
	return strings.Join(elem, "/")
}

// nextFeature names the new feature directory, <nnn>-<name>: the
// highest number in the features directory plus one. It refuses a name
// that has a directory already.
func nextFeature(cfg *Config, mt, name string) (string, error) {
	var (
		exists  = regexp.MustCompile(`^[0-9]{3}-` + regexp.QuoteMeta(name) + `$`)
		entries []os.DirEntry
		max     int
		err     error
	)
	if entries, err = os.ReadDir(filepath.Join(mt, cfg.FeaturesDir)); err != nil {
		return "", err
	}
	for _, e := range entries {
		if !featureDir.MatchString(e.Name()) {
			continue
		}
		if exists.MatchString(e.Name()) {
			return "", fmt.Errorf("the feature %s exists", name)
		}
		if n, _ := strconv.Atoi(e.Name()[:3]); n > max {
			max = n
		}
	}
	return fmt.Sprintf("%03d-%s", max+1, name), nil
}

// addReadmeRow puts the new row after the last feature row of the
// features README, and re-aligns the tables. The anchor is the row
// template's text before {feature}.
func addReadmeRow(cfg *Config, mt, row string) error {
	var (
		file   = filepath.Join(mt, cfg.FeaturesReadme)
		anchor = regexp.MustCompile("^" +
			regexp.QuoteMeta(strings.SplitN(cfg.ReadmeRow, "{feature}", 2)[0]) + `[0-9]{3}-`)
		src   []byte
		lines []string
		last  = -1
		err   error
	)
	if src, err = os.ReadFile(file); err != nil {
		return err
	}
	lines = strings.Split(string(src), "\n")
	for i, l := range lines {
		if anchor.MatchString(l) {
			last = i
		}
	}
	if last < 0 {
		return fmt.Errorf("no feature row in %s to put the new row after", cfg.FeaturesReadme)
	}
	lines = append(lines[:last+1], append([]string{row}, lines[last+1:]...)...)
	return os.WriteFile(file,
		[]byte(mdtable.Formatter{}.Format(strings.Join(lines, "\n")).Text), 0o644)
}
