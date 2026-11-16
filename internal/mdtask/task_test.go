package mdtask

import "testing"

func TestParse(t *testing.T) {
	var cases = []struct {
		name   string
		line   string
		state  State
		isTask bool
	}{
		{
			name:   "open, dash",
			line:   "- [ ] a",
			state:  Open,
			isTask: true,
		},
		{
			name:   "done, lowercase x",
			line:   "- [x] b",
			state:  Done,
			isTask: true,
		},
		{
			name:   "done, uppercase X",
			line:   "- [X] c",
			state:  Done,
			isTask: true,
		},
		{
			name:   "in progress, tilde",
			line:   "- [~] d",
			state:  Doing,
			isTask: true,
		},
		{
			name:   "star marker",
			line:   "* [ ] e",
			state:  Open,
			isTask: true,
		},
		{
			name:   "plus marker",
			line:   "+ [ ] f",
			state:  Open,
			isTask: true,
		},
		{
			name:   "numbered, dot",
			line:   "1. [ ] g",
			state:  Open,
			isTask: true,
		},
		{
			name:   "numbered, paren",
			line:   "1) [ ] h",
			state:  Open,
			isTask: true,
		},
		{
			name:   "leading whitespace, nested",
			line:   "    - [x] i",
			state:  Done,
			isTask: true,
		},
		{
			name:   "checkbox at end of line, no trailing space",
			line:   "- [ ]",
			state:  Open,
			isTask: true,
		},
		{
			name:   "empty brackets",
			line:   "- [] x",
			isTask: false,
		},
		{
			name:   "no space before bracket",
			line:   "-[ ] a",
			isTask: false,
		},
		{
			name:   "plain prose",
			line:   "this is not a task",
			isTask: false,
		},
		{
			name:   "bare brackets, no list marker",
			line:   "[x] a",
			isTask: false,
		},
		{
			name:   "no space or end after bracket",
			line:   "- [ ]c",
			isTask: false,
		},
		{
			name:   "marker buried in text",
			line:   "text [x] e",
			isTask: false,
		},
	}
	for _, c := range cases {
		var (
			state  State
			isTask bool
		)
		state, isTask = Parse(c.line)
		if isTask != c.isTask {
			t.Errorf("%s: Parse(%q) isTask = %v, want %v", c.name, c.line, isTask, c.isTask)
			continue
		}
		if isTask && state != c.state {
			t.Errorf("%s: Parse(%q) state = %v, want %v", c.name, c.line, state, c.state)
		}
	}
}
