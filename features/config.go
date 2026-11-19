package features

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConfigName is the file that describes the host repository. It sits at
// the repository root, checked in, so every worktree carries one copy.
// Load finds it by walking up from the working directory.
const ConfigName = ".repo-tools-features.json"

// Config is the one description of the host repository. Everything
// repository-specific comes from here; the code names no consumer. The
// trees themselves are not here: the commands discover them through
// `git worktree list` and map each role by name, so no path lives in
// the file.
type Config struct {
	// Trunk is the branch the features land on, such as "main".
	Trunk string `json:"trunk"`

	// Spec is the branch a feature branch starts at.
	Spec string `json:"spec"`

	// Implementation names the standing implementation trees, in the
	// order attach picks a free one. A worktree is the tree of a role
	// when its directory name is the role, or ends in "-<role>".
	Implementation []string `json:"implementation"`

	// Merge names the merge tree, by the same directory-name rule.
	Merge string `json:"merge"`

	// FeaturesDir holds one directory per feature, named <nnn>-<name>.
	FeaturesDir string `json:"featuresDir"`

	// FeaturesReadme is the page that lists the features, one row per
	// feature.
	FeaturesReadme string `json:"featuresReadme"`

	// ReadmeRow is the row that new adds to FeaturesReadme, with
	// {feature} for <nnn>-<name>. The text before {feature} also
	// anchors the insert: the new row goes after the last row that
	// starts with it.
	ReadmeRow string `json:"readmeRow"`

	// ScenarioFile is the path of a feature's scenario, with {name}.
	ScenarioFile string `json:"scenarioFile"`

	// ScenarioWatch is the directory whose changes need a scenario.
	ScenarioWatch string `json:"scenarioWatch"`

	// ScenarioTodo is the content that new and scenario write, with
	// {name}. It holds the marker "todo", in quotes; a scenario counts
	// as a todo while its content does.
	ScenarioTodo string `json:"scenarioTodo"`

	// BranchGate is the commands refresh runs on the branch, each one
	// an argv, run verbatim and in order.
	BranchGate [][]string `json:"branchGate"`

	// Verify is the commands verify runs in the merge tree, each one
	// an argv, run verbatim and in order.
	Verify [][]string `json:"verify"`

	// MergeTag is the tag land writes, with {branch} and {date}; the
	// date is YYYYMMDD.
	MergeTag string `json:"mergeTag"`

	// FixPrefix starts the name of a branch that has no feature
	// directory and no number, such as "fix-".
	FixPrefix string `json:"fixPrefix"`

	// EnvFile is the untracked per-tree file that carries the pair,
	// such as ".envrc.local".
	EnvFile string `json:"envFile"`

	// EnvNumber and EnvName are the export names of the pair that
	// attach writes into EnvFile: the feature's number and its name.
	EnvNumber string `json:"envNumber"`
	EnvName   string `json:"envName"`

	// Root is the directory that holds the file: the root of the
	// worktree the command runs in. Load sets it.
	Root string `json:"-"`

	// Path is the file itself, for the refusals that name it.
	Path string `json:"-"`
}

// Load finds ConfigName in dir or above it, reads it and validates it.
// A missing file, or a missing key, is an error that names the file and
// the key.
func Load(dir string) (*Config, error) {
	var (
		cfg  Config
		abs  string
		path string
		src  []byte
		err  error
	)
	if abs, err = filepath.Abs(dir); err != nil {
		return nil, err
	}
	for d := abs; ; d = filepath.Dir(d) {
		path = filepath.Join(d, ConfigName)
		if _, err = os.Stat(path); err == nil {
			break
		}
		if filepath.Dir(d) == d {
			return nil, fmt.Errorf("no %s in %s or above it", ConfigName, abs)
		}
	}
	if src, err = os.ReadFile(path); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(src, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	cfg.Path = path
	cfg.Root = filepath.Dir(path)
	if err = cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// validate refuses a configuration with a missing key, and a template
// without its placeholder. The error names the file and the key.
func (c *Config) validate() error {
	var (
		texts = []struct {
			key string
			val string
		}{
			{key: "trunk", val: c.Trunk},
			{key: "spec", val: c.Spec},
			{key: "merge", val: c.Merge},
			{key: "featuresDir", val: c.FeaturesDir},
			{key: "featuresReadme", val: c.FeaturesReadme},
			{key: "readmeRow", val: c.ReadmeRow},
			{key: "scenarioFile", val: c.ScenarioFile},
			{key: "scenarioWatch", val: c.ScenarioWatch},
			{key: "scenarioTodo", val: c.ScenarioTodo},
			{key: "mergeTag", val: c.MergeTag},
			{key: "fixPrefix", val: c.FixPrefix},
			{key: "envFile", val: c.EnvFile},
			{key: "envNumber", val: c.EnvNumber},
			{key: "envName", val: c.EnvName},
		}
		holds = []struct {
			key  string
			val  string
			mark string
		}{
			{key: "readmeRow", val: c.ReadmeRow, mark: "{feature}"},
			{key: "scenarioFile", val: c.ScenarioFile, mark: "{name}"},
			{key: "scenarioTodo", val: c.ScenarioTodo, mark: `"todo"`},
			{key: "mergeTag", val: c.MergeTag, mark: "{branch}"},
			{key: "mergeTag", val: c.MergeTag, mark: "{date}"},
		}
		argvs = []struct {
			key string
			val [][]string
		}{
			{key: "branchGate", val: c.BranchGate},
			{key: "verify", val: c.Verify},
		}
	)
	for _, s := range texts {
		if strings.TrimSpace(s.val) == "" {
			return fmt.Errorf("%s: the key %q is missing or empty", c.Path, s.key)
		}
	}
	if len(c.Implementation) == 0 {
		return fmt.Errorf("%s: the key %q is missing or empty", c.Path, "implementation")
	}
	for _, r := range c.Implementation {
		if strings.TrimSpace(r) == "" {
			return fmt.Errorf("%s: the key %q holds an empty name", c.Path, "implementation")
		}
	}
	for _, a := range argvs {
		if len(a.val) == 0 {
			return fmt.Errorf("%s: the key %q is missing or empty", c.Path, a.key)
		}
		for _, argv := range a.val {
			if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
				return fmt.Errorf("%s: the key %q holds an empty command", c.Path, a.key)
			}
		}
	}
	for _, h := range holds {
		if !strings.Contains(h.val, h.mark) {
			return fmt.Errorf("%s: the key %q must hold %s", c.Path, h.key, h.mark)
		}
	}
	return nil
}
