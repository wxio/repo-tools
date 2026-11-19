package features

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// resolved is the feature an argument names: the number, the name, and
// the branch the new branch starts at. A fix branch has the number
// "fix" and starts at the trunk.
type resolved struct {
	number string
	name   string
	start  string
}

// resolveFeature resolves <name or nnn> on the trunk: the directory
// <featuresDir>/<nnn>-<name> is reserved there by new. A name with the
// fix prefix has no directory and no number.
func resolveFeature(cfg *Config, arg string) (resolved, error) {
	var (
		lines   string
		matches []string
		base    string
		err     error
	)
	if strings.HasPrefix(arg, cfg.FixPrefix) {
		if !kebab.MatchString(arg) {
			return resolved{}, fmt.Errorf("give a kebab-case name")
		}
		return resolved{
			number: "fix",
			name:   arg,
			start:  cfg.Trunk,
		}, nil
	}
	if lines, err = gitOut(cfg.Root, "ls-tree", "--name-only", cfg.Trunk, cfg.FeaturesDir+"/"); err != nil {
		return resolved{}, err
	}
	for _, line := range strings.Split(lines, "\n") {
		base = path.Base(line)
		if !featureDir.MatchString(base) {
			continue
		}
		if isNumber.MatchString(arg) {
			var n, _ = strconv.Atoi(arg)
			if strings.HasPrefix(base, fmt.Sprintf("%03d-", n)) {
				matches = append(matches, base)
			}
		} else if base[4:] == arg {
			matches = append(matches, base)
		}
	}
	if len(matches) == 0 {
		return resolved{}, fmt.Errorf("no feature directory matches %s on %s; a fix skips it as %s<descr>",
			arg, cfg.Trunk, cfg.FixPrefix)
	}
	if len(matches) > 1 {
		return resolved{}, fmt.Errorf("%s matches more than one feature: %s",
			arg, strings.Join(matches, " "))
	}
	if !branchExists(cfg.Root, cfg.Spec) {
		return resolved{}, fmt.Errorf("there is no %s branch; a feature branch starts at %s's tip",
			cfg.Spec, cfg.Spec)
	}
	return resolved{
		number: matches[0][:3],
		name:   matches[0][4:],
		start:  cfg.Spec,
	}, nil
}

// freeTree is the path of the implementation tree of one role, and
// whether it is free: parked on its namesake branch, with no change to
// a tracked file. An absent tree is not free.
func freeTree(cfg *Config, role string) (string, bool, error) {
	var (
		p     string
		b     string
		clean bool
		err   error
	)
	if p, err = treeOfRole(cfg.Root, role); err != nil || p == "" {
		return p, false, err
	}
	if b, err = currentBranch(p); err != nil || b != role {
		return p, false, err
	}
	if clean, err = isClean(p); err != nil {
		return p, false, err
	}
	return p, clean, nil
}

// pickFreeTree picks the first free implementation tree, in the
// configured order. The empty role means every tree is busy.
func pickFreeTree(cfg *Config) (role, path string, err error) {
	for _, r := range cfg.Implementation {
		var (
			p    string
			free bool
		)
		if p, free, err = freeTree(cfg, r); err != nil {
			return "", "", err
		}
		if free {
			return r, p, nil
		}
	}
	return "", "", nil
}

// scenarioPath is the path of one feature's scenario, relative to the
// tree's root.
func scenarioPath(cfg *Config, name string) string {
	return expand(cfg.ScenarioFile, "{name}", name)
}

// writeScenario writes the feature's scenario as a todo, under base. It
// refuses to overwrite one. It returns the relative path it wrote.
func writeScenario(cfg *Config, base, name string) (string, error) {
	var (
		rel = scenarioPath(cfg, name)
		f   = filepath.Join(base, rel)
		err error
	)
	if _, err = os.Stat(f); err == nil {
		return rel, fmt.Errorf("%s exists; it is not overwritten", rel)
	}
	if err = os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		return rel, err
	}
	return rel, os.WriteFile(f, []byte(expand(cfg.ScenarioTodo, "{name}", name)), 0o644)
}

// scenarioIsTodo reports whether ref changes the watched directory
// while the branch's scenario is absent, or still holds the todo
// marker. The diff is against the trunk.
func scenarioIsTodo(cfg *Config, branch, ref string) (bool, error) {
	var (
		changed string
		shown   string
		err     error
	)
	if changed, err = gitOut(cfg.Root, "diff", "--name-only",
		cfg.Trunk, ref, "--", cfg.ScenarioWatch); err != nil {
		return false, err
	}
	if changed == "" {
		return false, nil
	}
	if shown, err = gitOut(cfg.Root, "show", ref+":"+scenarioPath(cfg, branch)); err != nil {
		return true, nil
	}
	return strings.Contains(shown, `"todo"`), nil
}

// mergeTagName is the tag land writes, from the configured template.
func mergeTagName(cfg *Config, branch string) string {
	return expand(cfg.MergeTag,
		"{branch}", branch,
		"{date}", time.Now().Format("20060102"))
}
