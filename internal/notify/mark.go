package notify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// markBaseDir is the directory a session's start mark lives under. It
// starts as os.TempDir(), but it is a variable, not a call baked into
// every function below, so a test can point it at t.TempDir() and never
// touch the real temporary directory.
var markBaseDir = os.TempDir()

// markFilePrefix and markFileSuffix bound the name of a mark file, so
// SweepMarks can find them again with a glob and nothing else in the
// directory is mistaken for one.
const (
	markFilePrefix = "repo-tools-notify-mark-"
	markFileSuffix = ".json"
)

// sessionIDPattern is every character a session id is allowed to hold.
// A session id arrives from outside this package, on a hook's JSON, so
// it is untrusted: reject anything that is not a letter, a digit, an
// underscore or a hyphen. Rejecting a character outside that set also
// rejects a path separator and a "..", which is the property that keeps
// a mark file inside markBaseDir whatever a caller sends.
var sessionIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Mark is the state a UserPromptSubmit hook writes at the start of a
// turn, keyed by the hook's session_id, for a Stop hook to read at the
// end of the same turn.
type Mark struct {
	// Start is when the turn began. A Stop hook subtracts this from now
	// to get how long the turn took.
	Start time.Time

	// DonePhases is the set of plan sections that were already complete
	// when the mark was written. Nothing fills this in yet: the field is
	// added now so that a later change, which names each section that
	// became complete during a turn, needs no migration of a mark
	// already on disk. A Stop hook only ever compares against a mark
	// that a UserPromptSubmit hook from the same or a later build wrote.
	DonePhases []string
}

// markPath returns the file Mark data for sessionID lives in, and
// whether sessionID is safe to use at all. A sessionID that fails
// sessionIDPattern never reaches filepath.Join, so it can never carry a
// path separator or a ".." out of markBaseDir.
func markPath(sessionID string) (path string, ok bool) {
	if !sessionIDPattern.MatchString(sessionID) {
		return "", false
	}
	path = filepath.Join(markBaseDir, markFilePrefix+sessionID+markFileSuffix)
	return path, true
}

// WriteMark writes m as the start mark for sessionID. It errors on an
// unsafe sessionID or a write it cannot make; the caller, a hook that
// must never fail a session, chooses to swallow that error.
func WriteMark(sessionID string, m Mark) error {
	var (
		path string
		ok   bool
		data []byte
		err  error
	)
	if path, ok = markPath(sessionID); !ok {
		return fmt.Errorf("notify: unsafe session id %q", sessionID)
	}
	if data, err = json.Marshal(m); err != nil {
		return err
	}
	if err = os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return nil
}

// ReadMark reads the start mark for sessionID. It never errors: a
// missing file, an unsafe sessionID, and a file that does not parse as
// a Mark all give ok=false and a zero Mark.
func ReadMark(sessionID string) (mark Mark, ok bool) {
	var (
		path string
		data []byte
		err  error
	)
	if path, ok = markPath(sessionID); !ok {
		return Mark{}, false
	}
	if data, err = os.ReadFile(path); err != nil {
		return Mark{}, false
	}
	if err = json.Unmarshal(data, &mark); err != nil {
		return Mark{}, false
	}
	return mark, true
}

// DeleteMark removes the start mark for sessionID, once a Stop hook has
// read it. It is best-effort: an unsafe sessionID, a missing file, or a
// file it cannot remove are all left in silence, never an error the
// caller has to handle.
func DeleteMark(sessionID string) {
	var (
		path string
		ok   bool
	)
	if path, ok = markPath(sessionID); !ok {
		return
	}
	_ = os.Remove(path)
}

// SweepMarks removes every mark file under markBaseDir whose last write
// is older than olderThan, so a session that never reached its Stop
// hook does not leave a file behind forever. It is best-effort: a glob
// that matches nothing, a file it cannot stat, and a file it cannot
// remove are all left in silence, never an error the caller has to
// handle.
func SweepMarks(olderThan time.Duration) {
	var (
		pattern = filepath.Join(markBaseDir, markFilePrefix+"*"+markFileSuffix)
		matches []string
		now     = time.Now()
		err     error
		info    os.FileInfo
	)
	if matches, err = filepath.Glob(pattern); err != nil {
		return
	}
	for _, path := range matches {
		if info, err = os.Stat(path); err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > olderThan {
			_ = os.Remove(path)
		}
	}
}
