package features

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/wxio/repo-tools/internal/exitcode"
)

// TestBareCommandPrintsTheWorkflow holds the bare command to the whole
// workflow, not the subcommand list alone. The list says what to run;
// the rest says which tree to stand in, and what the user does between
// the commands. A reader who gets only the list still cannot start.
func TestBareCommandPrintsTheWorkflow(t *testing.T) {
	var (
		out bytes.Buffer
		c   = &FeaturesCmd{
			out: &out,
		}
		got string
		err error
	)
	if err = c.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got = out.String()

	// Every subcommand, so the printed list cannot drift from the family.
	for _, name := range []string{
		"new", "attach", "scenario", "refresh",
		"merge", "verify", "land", "finish",
	} {
		if !strings.Contains(got, "\n  "+name) {
			t.Errorf("the loop does not name the subcommand %s", name)
		}
	}

	// The workflow around the commands: the roles of the trees, where
	// each command runs, the steps that stay the user's, and the fix lane.
	for _, want := range []string{
		"The trees stand",
		"an implementation tree",
		"the merge tree",
		"Run each command where it belongs",
		"The steps between the commands are yours",
		"A fix skips the spec",
		ConfigName,
		"--dry-run",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the loop does not carry %q", want)
		}
	}

	// It is terminal help, and two blocks are aligned by hand. A line
	// that wraps breaks the alignment it was padded for.
	for _, line := range strings.Split(got, "\n") {
		if len(line) > 80 {
			t.Errorf("a line of %d columns: %q", len(line), line)
		}
	}
}

func TestLoadConfigWalksUp(t *testing.T) {
	var (
		f        = newFixture(t)
		cfg, err = Load(filepath.Join(f.root, "docs", "features"))
	)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Root != f.root {
		t.Fatalf("Root = %q, want %q", cfg.Root, f.root)
	}
	if cfg.Trunk != "main" || cfg.Spec != "plan" {
		t.Fatalf("trunk %q, spec %q", cfg.Trunk, cfg.Spec)
	}
}

func TestLoadConfigRefusals(t *testing.T) {
	var cases = []struct {
		name string
		edit func(c *Config)
		want string
	}{
		{
			name: "missing trunk",
			edit: func(c *Config) { c.Trunk = "" },
			want: `the key "trunk" is missing or empty`,
		},
		{
			name: "missing implementation",
			edit: func(c *Config) { c.Implementation = nil },
			want: `the key "implementation" is missing or empty`,
		},
		{
			name: "missing branch gate",
			edit: func(c *Config) { c.BranchGate = nil },
			want: `the key "branchGate" is missing or empty`,
		},
		{
			name: "merge tag without the branch",
			edit: func(c *Config) { c.MergeTag = "merged-{date}" },
			want: `the key "mergeTag" must hold {branch}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				dir  = t.TempDir()
				cfg  = testConfig()
				src  []byte
				err  error
				file = filepath.Join(dir, ConfigName)
			)
			tc.edit(cfg)
			if src, err = json.Marshal(cfg); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(file, src, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err = Load(dir); err == nil {
				t.Fatal("Load accepted the broken configuration")
			}
			if !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), file) {
				t.Fatalf("error %q; want it to name %q and %q", err, tc.want, file)
			}
		})
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	var _, err = Load(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), ConfigName) {
		t.Fatalf("error %v; want it to name %s", err, ConfigName)
	}
}

func TestResolveFeature(t *testing.T) {
	var (
		f     = newFixture(t)
		cases = []struct {
			arg    string
			number string
			name   string
			start  string
		}{
			{arg: "64", number: "064", name: "two", start: "plan"},
			{arg: "064", number: "064", name: "two", start: "plan"},
			{arg: "1", number: "001", name: "one", start: "plan"},
			{arg: "two", number: "064", name: "two", start: "plan"},
			{arg: "fix-typo", number: "fix", name: "fix-typo", start: "main"},
		}
	)
	for _, tc := range cases {
		t.Run(tc.arg, func(t *testing.T) {
			var r, err = resolveFeature(f.cfg, tc.arg)
			if err != nil {
				t.Fatal(err)
			}
			if r.number != tc.number || r.name != tc.name || r.start != tc.start {
				t.Fatalf("resolveFeature(%q) = %+v, want %s %s %s", tc.arg, r, tc.number, tc.name, tc.start)
			}
		})
	}
}

func TestResolveFeatureRefusals(t *testing.T) {
	var (
		f      = newFixture(t)
		_, err = resolveFeature(f.cfg, "seven")
	)
	if err == nil || !strings.Contains(err.Error(), "no feature directory matches seven") {
		t.Fatalf("error %v; want the no-match refusal", err)
	}
	// Two directories with one number: the number is ambiguous.
	writeT(t, f.root, "docs/features/077-a/prd.md", "# a\n")
	writeT(t, f.root, "docs/features/077-b/prd.md", "# b\n")
	gitT(t, f.root, "add", "docs/features")
	gitT(t, f.root, "commit", "-q", "-m", "two 077")
	if _, err = resolveFeature(f.cfg, "77"); err == nil ||
		!strings.Contains(err.Error(), "matches more than one feature") {
		t.Fatalf("error %v; want the ambiguity refusal", err)
	}
}

func TestPickFreeTreeOrder(t *testing.T) {
	var (
		f    = newFixture(t)
		role string
		path string
		err  error
	)
	// Both trees are parked: alpha, the first in the order, wins.
	if role, path, err = pickFreeTree(f.cfg); err != nil {
		t.Fatal(err)
	}
	if role != "alpha" || path != f.alpha {
		t.Fatalf("picked %s at %s, want alpha at %s", role, path, f.alpha)
	}
	// A busy alpha, on a feature branch, loses to beta.
	gitT(t, f.alpha, "switch", "-q", "-c", "two", "plan")
	if role, _, err = pickFreeTree(f.cfg); err != nil {
		t.Fatal(err)
	}
	if role != "beta" {
		t.Fatalf("picked %s, want beta", role)
	}
	// A parked alpha with a changed tracked file is busy too.
	gitT(t, f.alpha, "switch", "-q", "alpha")
	writeT(t, f.alpha, "docs/features/README.md", "changed\n")
	if role, _, err = pickFreeTree(f.cfg); err != nil {
		t.Fatal(err)
	}
	if role != "beta" {
		t.Fatalf("picked %s, want beta when alpha is dirty", role)
	}
}

func TestAttachDryRun(t *testing.T) {
	var (
		f   = newFixture(t)
		out strings.Builder
		cmd = &AttachCmd{
			DryRun: true,
			Args: []string{
				"64",
			},
			out: &out,
			dir: f.root,
		}
		err  = cmd.Run()
		want = []string{
			"would: git -C " + f.root + " branch two plan",
			"would: git -C " + f.alpha + " switch -q two",
			"would: write export FEATURE_NO=064 and export FEATURE_NAME=two into " +
				filepath.Join(f.alpha, ".envrc.local"),
			"features attach: two is on " + f.alpha + " (the branch is new, at plan's tip); work in a tmux there:",
		}
	)
	if err != nil {
		t.Fatalf("attach --dry-run: %v\n%s", err, out.String())
	}
	for _, w := range want {
		if !strings.Contains(out.String(), w) {
			t.Fatalf("output misses %q:\n%s", w, out.String())
		}
	}
	// The dry run changed nothing: no branch, and alpha still parked.
	if branchExists(f.root, "two") {
		t.Fatal("the dry run created the branch")
	}
	if b, _ := currentBranch(f.alpha); b != "alpha" {
		t.Fatalf("alpha is on %q after the dry run", b)
	}
}

func TestAttachWritesThePair(t *testing.T) {
	var (
		f   = newFixture(t)
		out strings.Builder
		cmd = &AttachCmd{
			Args: []string{
				"two",
				"beta",
			},
			out: &out,
			dir: f.root,
		}
		err = cmd.Run()
	)
	if err != nil {
		t.Fatalf("attach: %v\n%s", err, out.String())
	}
	if b, _ := currentBranch(f.beta); b != "two" {
		t.Fatalf("beta is on %q, want two", b)
	}
	if v := envPairValue(filepath.Join(f.beta, ".envrc.local"), "FEATURE_NAME"); v != "two" {
		t.Fatalf("FEATURE_NAME = %q, want two", v)
	}
	if v := envPairValue(filepath.Join(f.beta, ".envrc.local"), "FEATURE_NO"); v != "064" {
		t.Fatalf("FEATURE_NO = %q, want 064", v)
	}
	// A second attach is not a refusal: it says where the branch is.
	out.Reset()
	if err = (&AttachCmd{
		Args: []string{
			"two",
		},
		out: &out,
		dir: f.root,
	}).Run(); err != nil {
		t.Fatalf("second attach: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "two is attached to "+f.beta+" already") {
		t.Fatalf("second attach did not say where:\n%s", out.String())
	}
}

func TestLandRefusesBeforeVerify(t *testing.T) {
	var (
		f   = newFixture(t)
		out strings.Builder
		err error
	)
	// A merged branch that verify has not marked.
	gitT(t, f.alpha, "switch", "-q", "-c", "two", "plan")
	writeT(t, f.alpha, "docs/features/064-two/notes.md", "work\n")
	gitT(t, f.alpha, "add", ".")
	gitT(t, f.alpha, "commit", "-q", "-m", "features/064-two: work")
	if err = writeState(f.root, "branch", "two", "merged", gitT(t, f.alpha, "rev-parse", "HEAD")); err != nil {
		t.Fatal(err)
	}
	err = (&LandCmd{
		DryRun: true,
		Branches: []string{
			"two",
		},
		out: &out,
		dir: f.root,
	}).Run()
	if code, ok := err.(exitcode.Code); !ok || int(code) != 1 {
		t.Fatalf("land returned %v, want exit code 1", err)
	}
	if !strings.Contains(out.String(), "the merge of two is not verified; run repo-tools features verify first") {
		t.Fatalf("land's refusal:\n%s", out.String())
	}
}

func TestVerifyRefusesAnotherBranch(t *testing.T) {
	var (
		f   = newFixture(t)
		out strings.Builder
		err error
	)
	if err = writeState(f.root, "branch", "two", "merged", "0000"); err != nil {
		t.Fatal(err)
	}
	err = (&VerifyCmd{
		Branches: []string{
			"one",
		},
		out: &out,
		dir: f.root,
	}).Run()
	if code, ok := err.(exitcode.Code); !ok || int(code) != 1 {
		t.Fatalf("verify returned %v, want exit code 1", err)
	}
	if !strings.Contains(out.String(), "holds a merge of two, not of one") {
		t.Fatalf("verify's refusal:\n%s", out.String())
	}
}

func TestFeatureCandidates(t *testing.T) {
	var f = newFixture(t)
	gitT(t, f.root, "branch", "fix-typo")
	var got = featureCandidates(f.cfg)
	for _, w := range []string{"one", "001", "two", "064", "fix-typo"} {
		if !slices.Contains(got, w) {
			t.Fatalf("candidates %q miss %q", got, w)
		}
	}
	for _, skip := range []string{"main", "plan", "alpha", "beta"} {
		if slices.Contains(got, skip) {
			t.Fatalf("candidates %q hold %q", got, skip)
		}
	}
}

func TestTreeCandidatesFreeFirst(t *testing.T) {
	var f = newFixture(t)
	// alpha busy, beta free: beta comes first.
	gitT(t, f.alpha, "switch", "-q", "-c", "two", "plan")
	var got = treeCandidates(f.cfg)
	if !slices.Equal(got, []string{"beta", "alpha"}) {
		t.Fatalf("candidates = %q, want [beta alpha]", got)
	}
}

func TestBranchCandidates(t *testing.T) {
	var f = newFixture(t)
	gitT(t, f.root, "branch", "two", "plan")
	gitT(t, f.root, "branch", "fix-typo")
	var got = branchCandidates(f.cfg)
	if !slices.Equal(got, []string{"fix-typo", "two"}) {
		t.Fatalf("candidates = %q, want [fix-typo two]", got)
	}
}

func TestRefreshPairCheckOnlyInImplementationTree(t *testing.T) {
	var (
		f   = newFixture(t)
		out strings.Builder
		err error
	)
	// An implementation tree whose pair disagrees with its branch.
	gitT(t, f.alpha, "switch", "-q", "-c", "two", "plan")
	writeT(t, f.alpha, ".envrc.local", "export FEATURE_NO=001\nexport FEATURE_NAME=one\n")
	err = (&RefreshCmd{
		DryRun: true,
		out:    &out,
		dir:    f.alpha,
	}).Run()
	if err == nil || !strings.Contains(out.String(), "this tree is attached to another feature") {
		t.Fatalf("refresh in alpha: err %v\n%s", err, out.String())
	}
	// The same stale pair in the spec tree is no refusal: the check
	// runs in an implementation tree only.
	var spec = filepath.Join(filepath.Dir(f.root), "spec")
	gitT(t, f.root, "worktree", "add", "-q", spec, "plan")
	writeT(t, spec, ".envrc.local", "export FEATURE_NO=001\nexport FEATURE_NAME=one\n")
	out.Reset()
	if err = (&RefreshCmd{
		DryRun: true,
		out:    &out,
		dir:    spec,
	}).Run(); err != nil {
		t.Fatalf("refresh in the spec tree: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "would: git -C "+spec+" merge --no-edit main") {
		t.Fatalf("refresh's dry run:\n%s", out.String())
	}
}
