package features

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gitOut runs git -C dir and returns its trimmed output. The error of a
// failed command carries what git wrote to stderr.
func gitOut(dir string, args ...string) (string, error) {
	var (
		cmd  = exec.Command("git", append([]string{"-C", dir}, args...)...)
		errb bytes.Buffer
		out  []byte
		err  error
	)
	cmd.Stderr = &errb
	if out, err = cmd.Output(); err != nil {
		return "", fmt.Errorf("git %s: %v: %s",
			strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return strings.TrimSpace(string(out)), nil
}

// gitRun runs git -C dir and lets it talk to w, so git's own words reach
// the user. A failure comes back as the error alone: git has said why.
func gitRun(w io.Writer, dir string, args ...string) error {
	var cmd = exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

// gitRunInput is gitRun with input on stdin and extra environment
// entries, for `git commit -F -` under SKIP.
func gitRunInput(w io.Writer, dir, input string, env []string, args ...string) error {
	var cmd = exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Stdin = strings.NewReader(input)
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.Env = append(os.Environ(), env...)
	return cmd.Run()
}

// runArgv runs one configured check command, verbatim, in dir, with the
// terminal of this process.
func runArgv(w io.Writer, dir string, argv []string) error {
	var cmd = exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

// worktree is one row of `git worktree list --porcelain`: the directory,
// and the branch it has checked out, empty when the head is detached.
type worktree struct {
	path   string
	branch string
}

// worktrees lists every worktree of the repository that holds dir.
func worktrees(dir string) ([]worktree, error) {
	var (
		out   string
		trees []worktree
		err   error
	)
	if out, err = gitOut(dir, "worktree", "list", "--porcelain"); err != nil {
		return nil, err
	}
	for _, line := range strings.Split(out, "\n") {
		if p, ok := strings.CutPrefix(line, "worktree "); ok {
			trees = append(trees, worktree{
				path: p,
			})
		}
		if b, ok := strings.CutPrefix(line, "branch refs/heads/"); ok && len(trees) > 0 {
			trees[len(trees)-1].branch = b
		}
	}
	return trees, nil
}

// treeOfBranch is the path of the worktree that has branch checked out;
// empty when none has.
func treeOfBranch(dir, branch string) (string, error) {
	var (
		trees []worktree
		err   error
	)
	if trees, err = worktrees(dir); err != nil {
		return "", err
	}
	for _, t := range trees {
		if t.branch == branch {
			return t.path, nil
		}
	}
	return "", nil
}

// roleOf names the standing-tree role of a worktree path: the role in
// roles whose name is the directory's last element, or ends it after a
// dash. It returns "" when the path is no standing tree.
func roleOf(roles []string, path string) string {
	var base = filepath.Base(path)
	for _, r := range roles {
		if base == r || strings.HasSuffix(base, "-"+r) {
			return r
		}
	}
	return ""
}

// treeOfRole is the path of the standing tree of one role, by the
// directory-name rule of roleOf; empty when no worktree matches.
func treeOfRole(dir, role string) (string, error) {
	var (
		trees []worktree
		err   error
	)
	if trees, err = worktrees(dir); err != nil {
		return "", err
	}
	for _, t := range trees {
		if roleOf([]string{role}, t.path) == role {
			return t.path, nil
		}
	}
	return "", nil
}

// trunkTree is the working tree of the trunk's checkout. The recorded
// worktree path can be the git dir when the repository is a submodule,
// so the path resolves through rev-parse.
func trunkTree(cfg *Config, dir string) (string, error) {
	var (
		p   string
		err error
	)
	if p, err = treeOfBranch(dir, cfg.Trunk); err != nil {
		return "", err
	}
	if p == "" {
		return "", fmt.Errorf("no worktree has %s checked out", cfg.Trunk)
	}
	return gitOut(p, "rev-parse", "--show-toplevel")
}

// mergeTreePath is the merge tree's directory. When no worktree is the
// merge tree yet, it is the path the caller adds one at: a sibling of an
// implementation tree, named by the same rule, or a sibling of the
// current tree as the last resort.
func mergeTreePath(cfg *Config, dir string) (string, error) {
	var (
		p     string
		top   string
		trees []worktree
		err   error
	)
	if p, err = treeOfRole(dir, cfg.Merge); err != nil {
		return "", err
	}
	if p != "" {
		return p, nil
	}
	if trees, err = worktrees(dir); err != nil {
		return "", err
	}
	for _, t := range trees {
		var base = filepath.Base(t.path)
		for _, r := range cfg.Implementation {
			if strings.HasSuffix(base, "-"+r) {
				return filepath.Join(filepath.Dir(t.path),
					strings.TrimSuffix(base, r)+cfg.Merge), nil
			}
		}
	}
	if top, err = gitOut(dir, "rev-parse", "--show-toplevel"); err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(top), filepath.Base(top)+"-"+cfg.Merge), nil
}

// currentBranch is the branch dir's worktree is on; empty when the head
// is detached.
func currentBranch(dir string) (string, error) {
	return gitOut(dir, "branch", "--show-current")
}

// branchExists reports whether the local branch exists.
func branchExists(dir, name string) bool {
	var _, err = gitOut(dir, "rev-parse", "-q", "--verify", "refs/heads/"+name)
	return err == nil
}

// tagExists reports whether the tag exists.
func tagExists(dir, name string) bool {
	var _, err = gitOut(dir, "rev-parse", "-q", "--verify", "refs/tags/"+name)
	return err == nil
}

// isClean reports whether no tracked file of dir's worktree has changed.
func isClean(dir string) (bool, error) {
	var out, err = gitOut(dir, "status", "--porcelain", "--untracked-files=no")
	return out == "", err
}

// mergeConflicts lists the files a merge of ours and theirs would
// conflict in, with no tree touched. Empty means the merge is clean.
func mergeConflicts(dir, ours, theirs string) ([]string, error) {
	var (
		cmd = exec.Command("git", "-C", dir, "merge-tree", "--write-tree",
			"--name-only", "--no-messages", ours, theirs)
		exit  *exec.ExitError
		out   []byte
		lines []string
		err   error
	)
	if out, err = cmd.Output(); err == nil {
		return nil, nil
	}
	// Exit 1 means conflicts: the first line is the tree, the rest the
	// files. Any other exit is a real failure.
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		return nil, fmt.Errorf("git merge-tree %s %s: %v", ours, theirs, err)
	}
	if lines = strings.Split(strings.TrimSpace(string(out)), "\n"); len(lines) < 2 {
		return nil, nil
	}
	return lines[1:], nil
}

// stateDir is the directory of the merge state and the merge message
// files. It is under the common git dir, so every worktree sees it.
func stateDir(dir string) (string, error) {
	var common, err = gitOut(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return filepath.Join(common, "feature-merge"), nil
}

// readState reads the state file the finish stages share: branch=,
// merged=, and verified= on a green verify. A missing file is an empty
// state.
func readState(dir string) (map[string]string, error) {
	var (
		sd    string
		src   []byte
		state = map[string]string{}
		err   error
	)
	if sd, err = stateDir(dir); err != nil {
		return nil, err
	}
	if src, err = os.ReadFile(filepath.Join(sd, "state")); err != nil {
		return state, nil
	}
	for _, line := range strings.Split(string(src), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			state[k] = v
		}
	}
	return state, nil
}

// writeState writes the state file, replacing what was there.
func writeState(dir string, pairs ...string) error {
	var (
		sd  string
		b   strings.Builder
		err error
	)
	if sd, err = stateDir(dir); err != nil {
		return err
	}
	if err = os.MkdirAll(sd, 0o755); err != nil {
		return err
	}
	for i := 0; i+1 < len(pairs); i += 2 {
		fmt.Fprintf(&b, "%s=%s\n", pairs[i], pairs[i+1])
	}
	return os.WriteFile(filepath.Join(sd, "state"), []byte(b.String()), 0o644)
}

// removeState removes the state file, and the branch's default merge
// message file when branch is not empty.
func removeState(dir, branch string) error {
	var sd, err = stateDir(dir)
	if err != nil {
		return err
	}
	os.Remove(filepath.Join(sd, "state"))
	if branch != "" {
		os.Remove(filepath.Join(sd, branch+".msg"))
	}
	return nil
}

// envPairValue reads one export of the pair file: the value of
// `export <name>=` in file, or "" when the file or the line is absent.
func envPairValue(file, name string) string {
	var src, err = os.ReadFile(file)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(src), "\n") {
		if v, ok := strings.CutPrefix(line, "export "+name+"="); ok {
			return v
		}
	}
	return ""
}

// writeEnvPair writes the pair into the tree's pair file, replacing its
// own two lines only, as attach does.
func writeEnvPair(file string, cfg *Config, number, name string) error {
	var (
		b   strings.Builder
		src []byte
		err error
	)
	if src, err = os.ReadFile(file); err == nil && len(src) > 0 {
		for _, line := range strings.Split(strings.TrimRight(string(src), "\n"), "\n") {
			if strings.HasPrefix(line, "export "+cfg.EnvNumber+"=") ||
				strings.HasPrefix(line, "export "+cfg.EnvName+"=") {
				continue
			}
			b.WriteString(line + "\n")
		}
	}
	fmt.Fprintf(&b, "export %s=%s\nexport %s=%s\n", cfg.EnvNumber, number, cfg.EnvName, name)
	return os.WriteFile(file, []byte(b.String()), 0o644)
}

// stripEnvPair removes the pair's two lines from the tree's pair file,
// as land does when it parks the tree.
func stripEnvPair(file string, cfg *Config) error {
	var (
		b   strings.Builder
		src []byte
		err error
	)
	if src, err = os.ReadFile(file); err != nil || len(src) == 0 {
		return nil
	}
	for _, line := range strings.Split(strings.TrimRight(string(src), "\n"), "\n") {
		if strings.HasPrefix(line, "export "+cfg.EnvNumber+"=") ||
			strings.HasPrefix(line, "export "+cfg.EnvName+"=") {
			continue
		}
		b.WriteString(line + "\n")
	}
	return os.WriteFile(file, []byte(b.String()), 0o644)
}
