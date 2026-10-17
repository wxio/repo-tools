package prompthistoryguard

import (
	"fmt"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	var cases = []struct {
		name  string
		paths []string
		mixed bool
	}{
		{
			name:  "nothing staged",
			paths: nil,
			mixed: false,
		},
		{
			name: "transcripts only",
			paths: []string{
				".prompt-history/main/001-a.md",
				".prompt-history/main/002-b.md",
			},
			mixed: false,
		},
		{
			name: "code only",
			paths: []string{
				"flake.nix",
				"samples/goohm/main.go",
			},
			mixed: false,
		},
		{
			name: "transcripts mixed with code",
			paths: []string{
				".prompt-history/main/001-a.md",
				"flake.nix",
			},
			mixed: true,
		},
		{
			name: "a path that merely starts with the directory name",
			paths: []string{
				".prompt-historyrc",
				"flake.nix",
			},
			mixed: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var report = check(c.paths)
			if (report != "") != c.mixed {
				t.Fatalf("check(%q) = %q, want mixed=%v", c.paths, report, c.mixed)
			}
		})
	}
}

// A mixed report has to name both sides, or it does not explain itself.
func TestCheckReportNamesBothSides(t *testing.T) {
	var report = check([]string{
		".prompt-history/main/001-a.md",
		"flake.nix",
	})
	for _, want := range []string{".prompt-history/main/001-a.md", "flake.nix", "git restore --staged"} {
		if !strings.Contains(report, want) {
			t.Errorf("report does not mention %q:\n%s", want, report)
		}
	}
}

func TestCheckReportTruncatesLongLists(t *testing.T) {
	var paths = []string{"flake.nix"}
	for i := range listLimit + 5 {
		paths = append(paths, fmt.Sprintf(".prompt-history/main/%03d-a.md", i))
	}

	var report = check(paths)
	if !strings.Contains(report, "... and 5 more") {
		t.Errorf("report does not truncate the transcript list:\n%s", report)
	}
}

func TestSplitNUL(t *testing.T) {
	var (
		out  = []byte("flake.nix\x00tools/go.mod\x00")
		want = []string{"flake.nix", "tools/go.mod"}
		got  = splitNUL(out)
	)
	if len(got) != len(want) {
		t.Fatalf("splitNUL(%q) = %q, want %q", out, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("splitNUL(%q)[%d] = %q, want %q", out, i, got[i], want[i])
		}
	}
}

// The unstage half reports its counts from partition, so both sides have to
// come back whole.
func TestPartition(t *testing.T) {
	var history, other = partition([]string{
		".prompt-history/main/001-a.md",
		"flake.nix",
		".prompt-history/meta-info.csv",
		".prompt-historyrc",
	})
	if len(history) != 2 {
		t.Errorf("history = %q, want the 2 paths under %s", history, promptHistoryPrefix)
	}
	if len(other) != 2 {
		t.Errorf("other = %q, want flake.nix and .prompt-historyrc", other)
	}
}
