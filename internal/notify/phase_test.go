package notify

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writePlan writes name under dir, with contents, and returns the
// directory, so the tests below can pass t.TempDir() straight to
// DonePhases as dir.
func writePlan(t *testing.T, dir, name, contents string) {
	t.Helper()

	var (
		path = filepath.Join(dir, name)
		err  error
	)
	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll: %v", err)
	}
	if err = os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}
}

func TestDonePhases_OneCompleteOneHalfDone(t *testing.T) {
	var dir = t.TempDir()
	writePlan(t, dir, "plan.md", ""+
		"## Phase 1 — a complete one\n"+
		"\n"+
		"- [x] first\n"+
		"- [x] second\n"+
		"\n"+
		"## Phase 2 — a half-done one\n"+
		"\n"+
		"- [x] first\n"+
		"- [ ] second\n")

	var got = DonePhases(dir, "plan.md")
	var want = []string{"plan.md: Phase 1 — a complete one"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DonePhases = %#v, want %#v", got, want)
	}
}

func TestDonePhases_DoingBulletIsNotComplete(t *testing.T) {
	var dir = t.TempDir()
	writePlan(t, dir, "plan.md", ""+
		"## Phase 1 — in progress\n"+
		"\n"+
		"- [x] first\n"+
		"- [~] second, still missing a bit\n")

	var got = DonePhases(dir, "plan.md")
	if len(got) != 0 {
		t.Errorf("DonePhases = %#v, want none", got)
	}
}

func TestDonePhases_NoBulletIsNotComplete(t *testing.T) {
	var dir = t.TempDir()
	writePlan(t, dir, "plan.md", ""+
		"## Phase 1 — no checklist at all\n"+
		"\n"+
		"Just prose, no bullet in sight.\n")

	var got = DonePhases(dir, "plan.md")
	if len(got) != 0 {
		t.Errorf("DonePhases = %#v, want none", got)
	}
}

func TestDonePhases_FencedBulletIsIgnored(t *testing.T) {
	var dir = t.TempDir()
	writePlan(t, dir, "plan.md", ""+
		"## Phase 1 — a fenced example\n"+
		"\n"+
		"Here is what a task bullet looks like:\n"+
		"\n"+
		"```\n"+
		"- [ ] an example, not a task\n"+
		"```\n")

	// The one bullet in this section is inside a fence, so the section
	// holds no real bullet at all, and is not complete.
	var got = DonePhases(dir, "plan.md")
	if len(got) != 0 {
		t.Errorf("DonePhases = %#v, want none", got)
	}
}

func TestDonePhases_FencedBulletDoesNotCountTowardsCompletion(t *testing.T) {
	var dir = t.TempDir()
	writePlan(t, dir, "plan.md", ""+
		"## Phase 1 — a real bullet beside a fenced one\n"+
		"\n"+
		"- [x] the only real bullet\n"+
		"\n"+
		"An example of what a task looked like before:\n"+
		"\n"+
		"```\n"+
		"- [ ] an example, open, inside the fence\n"+
		"```\n")

	// The fenced "- [ ]" must not count as an open bullet that would
	// keep this section from being complete.
	var got = DonePhases(dir, "plan.md")
	var want = []string{"plan.md: Phase 1 — a real bullet beside a fenced one"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DonePhases = %#v, want %#v", got, want)
	}
}

func TestDonePhases_GlobMatchingNothing(t *testing.T) {
	var dir = t.TempDir()

	var got = DonePhases(dir, "no-such-*/plan.md")
	if len(got) != 0 {
		t.Errorf("DonePhases = %#v, want none", got)
	}
}

func TestDonePhases_UnreadableFileYieldsNone(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores file permissions")
	}

	var (
		dir  = t.TempDir()
		path = filepath.Join(dir, "plan.md")
		err  error
	)
	writePlan(t, dir, "plan.md", "## Phase 1 — unreachable\n\n- [x] one\n")
	if err = os.Chmod(path, 0o000); err != nil {
		t.Fatalf("os.Chmod: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(path, 0o644)
	})

	var got = DonePhases(dir, "plan.md")
	if len(got) != 0 {
		t.Errorf("DonePhases = %#v, want none, for an unreadable file", got)
	}
}

func TestDonePhases_MalformedFileYieldsNone(t *testing.T) {
	var (
		dir  = t.TempDir()
		path = filepath.Join(dir, "plan.md")
		err  error
	)
	// A directory where a plan file was expected: os.ReadFile refuses
	// it, the same shape of failure as a file this command cannot open.
	if err = os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("os.MkdirAll: %v", err)
	}

	var got = DonePhases(dir, "plan.md")
	if len(got) != 0 {
		t.Errorf("DonePhases = %#v, want none, for a file that will not read", got)
	}
}

func TestDonePhases_QualifiesWithTheFile(t *testing.T) {
	var dir = t.TempDir()
	writePlan(t, dir, "a/plan.md", "## Phase 1 — shared name\n\n- [x] one\n")
	writePlan(t, dir, "b/plan.md", "## Phase 1 — shared name\n\n- [x] one\n")

	var got = DonePhases(dir, "*/plan.md")
	var want = []string{
		"a/plan.md: Phase 1 — shared name",
		"b/plan.md: Phase 1 — shared name",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DonePhases = %#v, want %#v", got, want)
	}
}

func TestNewlyComplete(t *testing.T) {
	var cases = []struct {
		name   string
		before []string
		after  []string
		want   []string
	}{
		{
			name:   "nothing before, something after",
			before: nil,
			after:  []string{"plan.md: Phase 1", "plan.md: Phase 2"},
			want:   []string{"plan.md: Phase 1", "plan.md: Phase 2"},
		},
		{
			name:   "one was already done, one is new",
			before: []string{"plan.md: Phase 1"},
			after:  []string{"plan.md: Phase 1", "plan.md: Phase 2"},
			want:   []string{"plan.md: Phase 2"},
		},
		{
			name:   "nothing new",
			before: []string{"plan.md: Phase 1"},
			after:  []string{"plan.md: Phase 1"},
			want:   nil,
		},
		{
			name:   "both empty",
			before: nil,
			after:  nil,
			want:   nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got = NewlyComplete(c.before, c.after)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("NewlyComplete(%#v, %#v) = %#v, want %#v", c.before, c.after, got, c.want)
			}
		})
	}
}

func TestPhaseHeading(t *testing.T) {
	var cases = []struct {
		in   string
		want string
	}{
		{"plan.md: Phase 1 — a thing", "Phase 1 — a thing"},
		{"docs/features/foo/plan.md: Phase 3 — two words: a title", "Phase 3 — two words: a title"},
		{"no qualifier here", "no qualifier here"},
		{"", ""},
	}
	for _, c := range cases {
		if got := PhaseHeading(c.in); got != c.want {
			t.Errorf("PhaseHeading(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestDonePhases_AbsoluteGlob checks that an absolute glob is used as
// given, and not joined onto dir, so a caller that already has a full
// path does not end up with dir prefixed twice. dir is still used to
// make the qualifier readable, so an absolute glob that happens to sit
// under dir still names its file as "plan.md", not as the full path.
func TestDonePhases_AbsoluteGlob(t *testing.T) {
	var dir = t.TempDir()
	writePlan(t, dir, "plan.md", "## Phase 1 — absolute\n\n- [x] one\n")

	var got = DonePhases(dir, filepath.Join(dir, "plan.md"))
	var want = []string{"plan.md: Phase 1 — absolute"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DonePhases = %#v, want %#v", got, want)
	}
}
