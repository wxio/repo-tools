// Package mdtask is the single definition of a Markdown task line, shared
// by the commands that count or watch tasks.
//
// A task line is a list item that starts with a checkbox: `- [ ] text`,
// `* [x] text`, `+ [~] text`, `1. [ ] text` or `1) [ ] text`, each with
// any amount of leading whitespace. The mark inside the brackets is one
// of three states: a space is Open, a tilde is Doing, and an `x` or `X`
// is Done. A space or the end of the line must follow the checkbox, as
// GitHub requires.
//
// This package does not decide whether a line sits inside a fenced code
// block. A checkbox there is an example, not a task, but the fence runs
// across several lines, and this package reads one line at a time with
// no memory between calls. The caller that scans a file keeps the fence
// state and skips a line inside one before it reaches Parse; see
// planreport.Count for the caller that does.
package mdtask

import "regexp"

// State is the state of a task's checkbox.
type State int

const (
	// Open is a task whose checkbox holds a space: `[ ]`.
	Open State = iota
	// Doing is a task whose checkbox holds a tilde: `[~]`.
	Doing
	// Done is a task whose checkbox holds an x or an X: `[x]` or `[X]`.
	Done
)

// pattern matches a list item that starts with a checkbox. Group 1 is the
// mark inside the brackets: a space is Open, a tilde is Doing, and an x
// or X is Done. A space or the end of the line must follow the checkbox,
// as GitHub requires.
var pattern = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+\[( |~|[xX])\](?:\s|$)`)

// Parse reads one line and returns its State and whether the line is a
// task at all. A line that is not a task returns false, and its State is
// meaningless.
func Parse(line string) (State, bool) {
	var m = pattern.FindStringSubmatch(line)
	if m == nil {
		return Open, false
	}
	switch m[1] {
	case " ":
		return Open, true
	case "~":
		return Doing, true
	default:
		return Done, true
	}
}
