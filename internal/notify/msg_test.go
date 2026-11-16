package notify

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

// gitInitBranch makes an empty repository in dir, on the named branch, so
// branchOf has something real to read. A fresh repository's HEAD already
// names a branch, so this needs no commit.
//
// It scrubs GIT_* from the environment for the same reason branchOf does.
// These tests run from a git hook in this repository, and the hook's GIT_DIR
// outranks cmd.Dir: without the scrub, `git init` reports that core.bare and
// core.worktree do not make sense, and the test fails only under the hook.
func gitInitBranch(t *testing.T, dir, branch string) {
	t.Helper()

	var cmds = [][]string{
		{"init", "-q"},
		{"checkout", "-q", "-b", branch},
	}
	for _, args := range cmds {
		var cmd = exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = withoutGitEnv(os.Environ())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// TestWithoutGitEnv is the unit of the scrub above.
func TestWithoutGitEnv(t *testing.T) {
	var got = withoutGitEnv([]string{
		"PATH=/usr/bin",
		"GIT_DIR=/somewhere/.git",
		"GIT_WORK_TREE=/somewhere",
		"HOME=/home/someone",
	})
	if len(got) != 2 || got[0] != "PATH=/usr/bin" || got[1] != "HOME=/home/someone" {
		t.Fatalf("withoutGitEnv kept %q", got)
	}
}

func TestMessage(t *testing.T) {
	var (
		repoDir    = t.TempDir()
		missingDir = t.TempDir() + "/does-not-exist"
	)
	gitInitBranch(t, repoDir, "demo-branch")

	var cases = []struct {
		name    string
		payload map[string]any
		opts    Options
		want    string
	}{
		{
			name: "full payload",
			payload: map[string]any{
				"cwd": repoDir,
			},
			opts: Options{
				Label:   "kite",
				Elapsed: 4*time.Minute + 12*time.Second,
			},
			want: "[kite] demo-branch · done in 4m12s",
		},
		{
			name:    "empty payload",
			payload: map[string]any{},
			opts:    Options{},
			want:    "done",
		},
		{
			name: "unreadable cwd",
			payload: map[string]any{
				"cwd": missingDir,
			},
			opts: Options{
				Label:   "kite",
				Elapsed: 45 * time.Second,
			},
			want: "[kite] · done in 45s",
		},
		{
			name: "missing cwd key",
			payload: map[string]any{
				"hook_event_name": "Stop",
			},
			opts: Options{
				Label: "kite",
			},
			want: "[kite] · done",
		},
		{
			name: "failure by hook_event_name",
			payload: map[string]any{
				"cwd":             repoDir,
				"hook_event_name": "StopFailure",
			},
			opts: Options{
				Label:   "kite",
				Elapsed: time.Hour + 3*time.Minute,
			},
			want: "[kite] demo-branch · failed in 1h03m",
		},
		{
			name: "failure by error fields",
			payload: map[string]any{
				"error_type":    "Timeout",
				"error_message": "deadline exceeded",
			},
			opts: Options{
				Label: "kite",
			},
			want: "[kite] · failed",
		},
		{
			name: "no label, no elapsed",
			payload: map[string]any{
				"cwd": repoDir,
			},
			opts: Options{},
			want: "demo-branch · done",
		},
		{
			name:    "nil payload",
			payload: nil,
			opts: Options{
				Label: "kite",
			},
			want: "[kite] · done",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got = Message(c.payload, c.opts)
			if got != c.want {
				t.Errorf("Message() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestFormatElapsed(t *testing.T) {
	var cases = []struct {
		d    time.Duration
		want string
	}{
		{45 * time.Second, "45s"},
		{4*time.Minute + 12*time.Second, "4m12s"},
		{time.Hour + 3*time.Minute, "1h03m"},
		{0, "0s"},
	}

	for _, c := range cases {
		if got := formatElapsed(c.d); got != c.want {
			t.Errorf("formatElapsed(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestMessageNeverPanics(t *testing.T) {
	var payloads = []map[string]any{
		nil,
		{},
		{"cwd": 42},
		{"cwd": ""},
		{"hook_event_name": 7},
		{"error_type": []string{"not", "a", "string"}},
	}

	for _, p := range payloads {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Message panicked on payload %#v: %v", p, r)
				}
			}()
			Message(p, Options{Label: "kite"})
		}()
	}
}
