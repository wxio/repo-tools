package features

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain drops every GIT_* variable from the process environment
// before the tests run.
//
// The go-test hook runs these tests from a git hook, and the hook's
// GIT_DIR outranks cmd.Dir. Without the scrub, a fixture's `git init`
// re-initializes the hook's own repository and sets core.bare there,
// and every later git command of a test answers for that repository.
// Dropping the variables leaves the fixture directory as the only
// thing that decides. The production helpers inherit the process
// environment, so the scrub must happen here and not in gitT alone.
func TestMain(m *testing.M) {
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "GIT_") {
			var name, _, _ = strings.Cut(kv, "=")
			os.Unsetenv(name)
		}
	}
	os.Exit(m.Run())
}

// gitT runs git in dir with a pinned identity, so a test needs no
// global configuration. A failure fails the test.
func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	var (
		cmd = exec.Command("git", args...)
		out []byte
		err error
	)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@example.invalid",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@example.invalid",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
	if out, err = cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// testConfig is the configuration every fixture writes: generic names,
// and checks that always pass (`true`).
func testConfig() *Config {
	return &Config{
		Trunk: "main",
		Spec:  "plan",
		Implementation: []string{
			"alpha",
			"beta",
		},
		Merge:          "merge",
		FeaturesDir:    "docs/features",
		FeaturesReadme: "docs/features/README.md",
		ReadmeRow:      "| `{feature}` | prd | |",
		ScenarioFile:   "web/regression/scenarios/{name}.json",
		ScenarioWatch:  "web",
		ScenarioTodo:   "{\n  \"title\": \"{name}\",\n  \"todo\": \"write the steps\",\n  \"steps\": []\n}\n",
		BranchGate: [][]string{
			{"true"},
		},
		Verify: [][]string{
			{"true"},
		},
		MergeTag:  "merged/{branch}-{date}",
		FixPrefix: "fix-",
		EnvFile:   ".envrc.local",
		EnvNumber: "FEATURE_NO",
		EnvName:   "FEATURE_NAME",
	}
}

// fixture is one repository under t.TempDir(): the trunk's checkout at
// host/, the spec branch, two features on the trunk, and the standing
// worktrees host-alpha and host-beta.
type fixture struct {
	root  string // the trunk's checkout
	alpha string
	beta  string
	cfg   *Config
}

// newFixture builds the repository: git init, the configuration file,
// the feature directories, the features README, one scenario, the spec
// branch, and the two implementation worktrees.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	var (
		tmp = t.TempDir()
		f   = &fixture{
			root:  filepath.Join(tmp, "host"),
			alpha: filepath.Join(tmp, "host-alpha"),
			beta:  filepath.Join(tmp, "host-beta"),
		}
		src []byte
		err error
	)
	if err = os.MkdirAll(f.root, 0o755); err != nil {
		t.Fatal(err)
	}
	gitT(t, f.root, "init", "-q", "-b", "main")
	if src, err = json.MarshalIndent(testConfig(), "", "  "); err != nil {
		t.Fatal(err)
	}
	writeT(t, f.root, ConfigName, string(src)+"\n")
	writeT(t, f.root, "docs/features/001-one/prd.md", "# one\n")
	writeT(t, f.root, "docs/features/064-two/prd.md", "# two\n")
	writeT(t, f.root, "docs/features/README.md",
		"# features\n\n| feature | status | notes |\n|---|---|---|\n| `001-one` | prd | |\n| `064-two` | prd | |\n")
	writeT(t, f.root, "web/regression/scenarios/.keep", "")
	gitT(t, f.root, "add", ".")
	gitT(t, f.root, "commit", "-q", "-m", "init")
	gitT(t, f.root, "branch", "plan")
	gitT(t, f.root, "worktree", "add", "-q", "-b", "alpha", f.alpha, "main")
	gitT(t, f.root, "worktree", "add", "-q", "-b", "beta", f.beta, "main")
	if f.cfg, err = Load(f.root); err != nil {
		t.Fatal(err)
	}
	return f
}

// writeT writes one file under dir, making its directories.
func writeT(t *testing.T, dir, rel, content string) {
	t.Helper()
	var p = filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
