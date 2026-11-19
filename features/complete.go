package features

import (
	"path"
	"strings"
)

// Branch is the branch argument of merge, verify, land and finish. Its
// Complete lists the open branches: every local branch minus the trunk,
// the spec branch and the standing trees' own branches. opts completes
// an argument as a file name and does not call it, so main answers the
// shell first; see internal/argcomplete.
type Branch string

// Complete lists the open branches. It lists nothing when no
// configuration or no repository is found, as the shell then has
// nothing to offer.
func (b *Branch) Complete(user string) []string {
	var cfg, err = Load(".")
	if err != nil {
		return nil
	}
	return branchCandidates(cfg)
}

// AttachArgs completes the arguments of attach: the feature first, then
// the implementation tree, the free trees first. It implements
// argcomplete.ArgsCompleter, so the second argument completes too.
type AttachArgs struct{}

// Complete lists the candidates for the first argument.
func (a *AttachArgs) Complete(user string) []string {
	return a.CompleteArgs(nil, user)
}

// CompleteArgs lists the features for the first argument and the trees
// for the second, the free trees first.
func (a *AttachArgs) CompleteArgs(prior []string, user string) []string {
	var cfg, err = Load(".")
	if err != nil {
		return nil
	}
	switch len(prior) {
	case 0:
		return featureCandidates(cfg)
	case 1:
		return treeCandidates(cfg)
	}
	return nil
}

// featureCandidates lists what attach's first argument takes: each
// feature directory's name and number, then the open branches. The
// directories come from the trunk, through ls-tree, as attach resolves
// them, so a stale checkout hides nothing.
func featureCandidates(cfg *Config) []string {
	var (
		seen  = map[string]bool{}
		found []string
		out   string
		base  string
		err   error
	)
	var add = func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			found = append(found, s)
		}
	}
	if out, err = gitOut(cfg.Root, "ls-tree", "--name-only", cfg.Trunk, cfg.FeaturesDir+"/"); err == nil {
		for _, line := range strings.Split(out, "\n") {
			if base = path.Base(line); featureDir.MatchString(base) {
				add(base[4:])
				add(base[:3])
			}
		}
	}
	for _, b := range branchCandidates(cfg) {
		add(b)
	}
	return found
}

// treeCandidates lists the implementation trees that exist, the free
// ones first, each in the configured order.
func treeCandidates(cfg *Config) []string {
	var free, busy []string
	for _, r := range cfg.Implementation {
		var p, ok, err = freeTree(cfg, r)
		if err != nil || p == "" {
			continue
		}
		if ok {
			free = append(free, r)
		} else {
			busy = append(busy, r)
		}
	}
	return append(free, busy...)
}

// branchCandidates lists the local branches minus the trunk, the spec
// branch and the standing trees' own branches.
func branchCandidates(cfg *Config) []string {
	var (
		skip = map[string]bool{
			cfg.Trunk: true,
			cfg.Spec:  true,
			cfg.Merge: true,
		}
		out   string
		found []string
		err   error
	)
	for _, r := range cfg.Implementation {
		skip[r] = true
	}
	if out, err = gitOut(cfg.Root, "for-each-ref", "--format=%(refname:short)", "refs/heads"); err != nil || out == "" {
		return nil
	}
	for _, b := range strings.Split(out, "\n") {
		if !skip[b] {
			found = append(found, b)
		}
	}
	return found
}
