package features

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// scenarioReference is what a call without the name prints.
const scenarioReference = `repo-tools features scenario <name> [--dry-run]

Write the feature's scenario as a todo, in the current tree. new writes
one in the trunk's checkout; run this by hand for a feature older than
new, or for a second scenario.

  <name>     the feature, in kebab-case. The scenario's path comes from
             the configured template.
  --dry-run  print the write prefixed would:; change nothing.

It refuses: a name that is not kebab-case; a scenario that exists, which
it does not overwrite.
`

// ScenarioCmd is the command line of features scenario.
type ScenarioCmd struct {
	DryRun bool     `help:"print each mutating step prefixed would:; run the refusal checks; change nothing"`
	Names  []string `opts:"mode=arg,name=name,max=1" help:"the feature, in kebab-case"`

	out io.Writer
	dir string
}

// NewScenarioCmd returns the command line of features scenario.
func NewScenarioCmd() *ScenarioCmd {
	return &ScenarioCmd{
		out: os.Stdout,
		dir: ".",
	}
}

// Run writes the todo scenario in the current tree.
func (c *ScenarioCmd) Run() error {
	const cmd = "scenario"
	var (
		cfg *Config
		rel string
		err error
	)
	if len(c.Names) == 0 {
		return reference(c.out, scenarioReference)
	}
	if !kebab.MatchString(c.Names[0]) {
		return refuse(c.out, cmd, "give a kebab-case name: repo-tools features scenario <name>")
	}
	if cfg, err = Load(c.dir); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	rel = scenarioPath(cfg, c.Names[0])
	if c.DryRun {
		if _, err = os.Stat(filepath.Join(cfg.Root, rel)); err == nil {
			return refuse(c.out, cmd, "%s exists; it is not overwritten", rel)
		}
		wouldf(c.out, "write %s as a todo scenario", rel)
		return nil
	}
	if rel, err = writeScenario(cfg, cfg.Root, c.Names[0]); err != nil {
		return refuse(c.out, cmd, "%v", err)
	}
	fmt.Fprintf(c.out, "features scenario: %s is a todo; write its steps\n", rel)
	return nil
}
