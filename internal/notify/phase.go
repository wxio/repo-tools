package notify

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/wxio/repo-tools/internal/mdtask"
)

// A phase, to this command, is one `## ` section of a plan file that holds
// at least one task bullet (internal/mdtask decides what a bullet is). A
// section is complete when every bullet in it is Done. A section with an
// Open or a Doing bullet is not complete, and neither is a section with no
// bullet at all.
//
// DonePhases returns the complete sections of every file glob matches
// under dir, each named as:
//
//	<path relative to dir>: <heading, trimmed, as written>
//
// The file is part of the name because one glob commonly matches several
// plan files, and two of them can hold a section with the same heading,
// such as "Phase 3". Without the file, those two would collide into one
// entry on both sides of a before/after comparison. The caller that builds
// the chat message strips the file back off before it shows the heading,
// because the file's path can hold the same private name the rest of this
// package is built to keep out of a message; see msg.go.
//
// DonePhases never errors. A glob that matches nothing, a file it cannot
// read, and a file whose content is not what it expects all leave that
// file out, rather than fail the caller. A hook that calls this must never
// block or fail a session over a plan file it could not read.
func DonePhases(dir, glob string) (names []string) {
	defer func() {
		_ = recover()
	}()

	var (
		pattern string
		matches []string
		err     error
	)
	if filepath.IsAbs(glob) {
		pattern = glob
	} else {
		pattern = filepath.Join(dir, glob)
	}
	if matches, err = filepath.Glob(pattern); err != nil {
		return nil
	}
	for _, path := range matches {
		names = append(names, donePhasesOfFile(dir, path)...)
	}
	return names
}

// NewlyComplete returns the entries of after that are not in before,
// in the order they appear in after. It is how a Stop hook turns the
// mark's recorded set and a fresh call to DonePhases into "what became
// complete during this turn": every name on both sides is not news, and
// every name added since the mark was written is.
func NewlyComplete(before, after []string) []string {
	var (
		seen = make(map[string]bool, len(before))
		out  []string
	)
	for _, name := range before {
		seen[name] = true
	}
	for _, name := range after {
		if !seen[name] {
			out = append(out, name)
		}
	}
	return out
}

// PhaseHeading strips the file qualifier DonePhases adds and returns the
// heading alone, for display. A name with no qualifier, which should not
// happen from DonePhases but must not panic if it ever does, comes back
// unchanged.
func PhaseHeading(qualified string) string {
	if i := strings.Index(qualified, ": "); i >= 0 {
		return qualified[i+2:]
	}
	return qualified
}

// donePhasesOfFile reads one file and returns its complete sections,
// each qualified with path's name relative to dir. A file it cannot read
// gives no sections, not an error.
func donePhasesOfFile(dir, path string) []string {
	var (
		data []byte
		err  error
		rel  string
	)
	if data, err = os.ReadFile(path); err != nil {
		return nil
	}
	if rel, err = filepath.Rel(dir, path); err != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)
	return sectionsOf(rel, string(data))
}

// sectionsOf splits src on `## ` headings and returns the complete ones,
// each as "qualifier: heading". It tracks a fenced code block the same
// way planreport.Count does, so a checkbox inside a fence is never
// counted as a task: fencing is the business of whoever scans the file
// one line at a time, not of mdtask.Parse, which reads one line with no
// memory of the ones before it.
func sectionsOf(qualifier, src string) []string {
	var (
		sc      = bufio.NewScanner(strings.NewReader(src))
		fence   string
		heading string
		total   int
		doneCnt int
		out     []string
		state   mdtask.State
		isTask  bool
	)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)

	// finish closes out the section being read, and records it when it
	// held at least one bullet and every one of them was Done.
	var finish = func() {
		if heading != "" && total > 0 && total == doneCnt {
			out = append(out, qualifier+": "+heading)
		}
	}

	for sc.Scan() {
		var line = sc.Text()
		if f := fenceOf(line); f != "" {
			if fence == "" {
				fence = f
			} else if strings.HasPrefix(f, fence) {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			finish()
			heading = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			total = 0
			doneCnt = 0
			continue
		}
		if heading == "" {
			// Before the first heading, or between the start of the file
			// and its first "## " line. Nothing here belongs to a phase.
			continue
		}
		if state, isTask = mdtask.Parse(line); !isTask {
			continue
		}
		total++
		if state == mdtask.Done {
			doneCnt++
		}
	}
	finish()
	if sc.Err() != nil {
		// A line longer than the scanner's buffer, or another read
		// failure partway through. Whatever was found so far might be
		// missing the rest of its own section, so none of it is safe to
		// call complete.
		return nil
	}
	return out
}

// fenceOf returns the fence a line opens or closes: three or more
// backticks or tildes, after up to three spaces. It returns "" for a
// line that is not a fence. This is the same rule planreport.Count
// applies, kept here as its own copy: internal/mdtask's package comment
// says fencing is each caller's job, not a shared one.
func fenceOf(line string) string {
	var (
		s = strings.TrimLeft(line, " ")
		n int
		c byte
	)
	if len(line)-len(s) > 3 || s == "" {
		return ""
	}
	c = s[0]
	if c != '`' && c != '~' {
		return ""
	}
	for n = 0; n < len(s) && s[n] == c; n++ {
	}
	if n < 3 {
		return ""
	}
	return s[:n]
}
