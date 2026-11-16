package notify

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// withMarkDir points markBaseDir at a fresh t.TempDir() for the test, and
// restores the previous value when the test ends, so no test writes into
// the real system temporary directory.
func withMarkDir(t *testing.T) string {
	t.Helper()

	var (
		dir  = t.TempDir()
		prev = markBaseDir
	)
	markBaseDir = dir
	t.Cleanup(func() {
		markBaseDir = prev
	})
	return dir
}

func TestWriteReadMark_RoundTrip(t *testing.T) {
	withMarkDir(t)

	var (
		want = Mark{
			Start:      time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC),
			DonePhases: nil,
		}
		got Mark
		ok  bool
		err error
	)
	if err = WriteMark("abc123", want); err != nil {
		t.Fatalf("WriteMark: %v", err)
	}
	if got, ok = ReadMark("abc123"); !ok {
		t.Fatalf("ReadMark: ok = false, want true")
	}
	if !got.Start.Equal(want.Start) {
		t.Errorf("Start = %v, want %v", got.Start, want.Start)
	}
}

func TestReadMark_Missing(t *testing.T) {
	withMarkDir(t)

	var (
		got Mark
		ok  bool
	)
	if got, ok = ReadMark("no-such-session"); ok {
		t.Errorf("ReadMark: ok = true, want false; got %#v", got)
	}
}

func TestReadMark_Corrupt(t *testing.T) {
	var (
		dir  = withMarkDir(t)
		path string
		ok   bool
		err  error
	)
	if path, ok = markPath("corrupt-session"); !ok {
		t.Fatalf("markPath refused a safe session id")
	}
	if err = os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}
	if _, ok = ReadMark("corrupt-session"); ok {
		t.Errorf("ReadMark: ok = true, want false, for a corrupt file in %s", dir)
	}
}

func TestDeleteMark(t *testing.T) {
	withMarkDir(t)

	var (
		path string
		ok   bool
		err  error
	)
	if err = WriteMark("to-delete", Mark{Start: time.Now()}); err != nil {
		t.Fatalf("WriteMark: %v", err)
	}
	if path, ok = markPath("to-delete"); !ok {
		t.Fatalf("markPath refused a safe session id")
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatalf("mark file was not written: %v", err)
	}

	DeleteMark("to-delete")

	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("mark file still exists after DeleteMark: err = %v", err)
	}
}

func TestDeleteMark_MissingIsQuiet(t *testing.T) {
	withMarkDir(t)

	// Nothing to assert beyond: this must not panic and must not block.
	DeleteMark("never-written")
}

func TestSweepMarks(t *testing.T) {
	var (
		dir     = withMarkDir(t)
		oldPath = filepath.Join(dir, markFilePrefix+"old-session"+markFileSuffix)
		long    = 48 * time.Hour
		err     error
		oldOk   bool
		freshOk bool
	)
	if err = WriteMark("old-session", Mark{Start: time.Now()}); err != nil {
		t.Fatalf("WriteMark(old): %v", err)
	}
	if err = WriteMark("fresh-session", Mark{Start: time.Now()}); err != nil {
		t.Fatalf("WriteMark(fresh): %v", err)
	}
	if err = os.Chtimes(oldPath, time.Now().Add(-long), time.Now().Add(-long)); err != nil {
		t.Fatalf("os.Chtimes: %v", err)
	}

	SweepMarks(24 * time.Hour)

	if _, oldOk = ReadMark("old-session"); oldOk {
		t.Errorf("ReadMark(old-session): ok = true after SweepMarks, want false")
	}
	if _, freshOk = ReadMark("fresh-session"); !freshOk {
		t.Errorf("ReadMark(fresh-session): ok = false after SweepMarks, want true")
	}
}

func TestSweepMarks_EmptyDirIsQuiet(t *testing.T) {
	withMarkDir(t)

	// Nothing written. Must not panic and must not block.
	SweepMarks(24 * time.Hour)
}

func TestMarkPath_RejectsMaliciousSessionID(t *testing.T) {
	var (
		dir   = withMarkDir(t)
		cases = []string{
			"../../etc/passwd",
			"..",
			"a/b",
			`a\b`,
			"",
			"session with spaces",
			"/etc/passwd",
		}
		path string
		ok   bool
	)

	for _, sessionID := range cases {
		if path, ok = markPath(sessionID); ok {
			t.Errorf("markPath(%q) = %q, ok=true; want ok=false", sessionID, path)
		}
	}

	// WriteMark and ReadMark must agree with markPath: a rejected session
	// id writes and reads nothing, and the directory this test is
	// confined to gains no new entry.
	var (
		before []os.DirEntry
		after  []os.DirEntry
		err    error
	)
	if before, err = os.ReadDir(dir); err != nil {
		t.Fatalf("os.ReadDir(%s): %v", dir, err)
	}
	if err = WriteMark("../../etc/passwd", Mark{Start: time.Now()}); err == nil {
		t.Errorf("WriteMark(%q) = nil error, want a refusal", "../../etc/passwd")
	}
	if _, ok = ReadMark("../../etc/passwd"); ok {
		t.Errorf("ReadMark(%q): ok = true, want false", "../../etc/passwd")
	}
	if after, err = os.ReadDir(dir); err != nil {
		t.Fatalf("os.ReadDir(%s): %v", dir, err)
	}
	if len(after) != len(before) {
		t.Errorf("WriteMark on a malicious session id changed %s: %d entries before, %d after", dir, len(before), len(after))
	}
}
