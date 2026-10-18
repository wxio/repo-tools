package prompthistory

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEntryFileName(t *testing.T) {
	var (
		when = time.Date(2026, 10, 18, 9, 5, 0, 0, time.Local)
		got  = entryFileName(7, when, "fix-the-guard")
		want = "007-20261018-0905-fix-the-guard.md"
	)
	if got != want {
		t.Errorf("entryFileName = %q, want %q", got, want)
	}
}

// The time in the name must not read as part of the sequence number, and an
// entry in the old NNN-<slug>.md form still counts.
func TestNextSeqReadsBothForms(t *testing.T) {
	var dir = t.TempDir()
	for _, name := range []string{
		"001-old-form.md",
		"002-20261018-0905-new-form.md",
		"002-20261018-0905-new-form",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := nextSeq(dir); got != 3 {
		t.Errorf("nextSeq = %d, want 3", got)
	}
}
