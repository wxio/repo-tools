package mdtable

import (
	"strings"
	"testing"
)

// fmt70 is the formatter the command runs with by default.
var fmt70 = Formatter{
	Wide: 70,
}

func TestFormat_PadsEveryCellToItsColumn(t *testing.T) {
	var (
		src = strings.Join([]string{
			"| file | what it computes |",
			"|---|---|",
			"| `lower.go` | `Lower` |",
			"| `prim.go` | `Apply` and `Inverse` |",
		}, "\n")
		want = strings.Join([]string{
			"| file       | what it computes      |",
			"|------------|-----------------------|",
			"| `lower.go` | `Lower`               |",
			"| `prim.go`  | `Apply` and `Inverse` |",
		}, "\n")
		res = fmt70.Format(src)
	)
	if res.Text != want {
		t.Errorf("Format():\n%s\nwant:\n%s", res.Text, want)
	}
	if !res.Changed || res.Tables != 1 {
		t.Errorf("Changed = %v, Tables = %d; want true, 1", res.Changed, res.Tables)
	}
}

// Every row of one table is the same length. That is the property the
// rule is for, and the one a reader of the raw file sees.
func TestFormat_EveryRowIsTheSameLength(t *testing.T) {
	var (
		src = strings.Join([]string{
			"| a | bbbbbbbb |",
			"|:--|---:|",
			"| cccccccccc | d |",
			"| e | f |",
		}, "\n")
		res   = fmt70.Format(src)
		lines = strings.Split(res.Text, "\n")
	)
	for i := range lines {
		if len([]rune(lines[i])) != len([]rune(lines[0])) {
			t.Errorf("line %d is %d runes, line 0 is %d:\n%s",
				i, len([]rune(lines[i])), len([]rune(lines[0])), res.Text)
		}
	}
}

// An em dash is three bytes and one column. A page of this repo is full
// of them, so a width counted in bytes would misalign most tables here.
func TestFormat_CountsRunesNotBytes(t *testing.T) {
	var (
		src = strings.Join([]string{
			"| a | b |",
			"|---|---|",
			"| — | x |",
			"| yy | z |",
		}, "\n")
		res   = fmt70.Format(src)
		lines = strings.Split(res.Text, "\n")
	)
	for i := range lines {
		if len([]rune(lines[i])) != len([]rune(lines[0])) {
			t.Fatalf("the em dash row is not the same width:\n%s", res.Text)
		}
	}
}

func TestFormat_KeepsAlignmentColons(t *testing.T) {
	var (
		src = strings.Join([]string{
			"| a | b | c |",
			"|:---|---:|:---:|",
			"| 1 | 2 | 3 |",
		}, "\n")
		res = fmt70.Format(src)
		sep = strings.Split(res.Text, "\n")[1]
	)
	if !strings.HasPrefix(sep, "|:-") {
		t.Errorf("the left colon is gone: %s", sep)
	}
	if !strings.Contains(sep, "-:|:-") {
		t.Errorf("the right and the centre colons are gone: %s", sep)
	}
	if !strings.HasSuffix(sep, "-:|") {
		t.Errorf("the centre cell lost its right colon: %s", sep)
	}
}

// A fenced block holds the example of a table in AGENTS.md. An example
// the formatter rewrote would stop being the example.
func TestFormat_LeavesAFencedTableAlone(t *testing.T) {
	var (
		src = strings.Join([]string{
			"```markdown",
			"| a | b |",
			"|---|---|",
			"| 1 | 2 |",
			"```",
		}, "\n")
		res = fmt70.Format(src)
	)
	if res.Changed || res.Tables != 0 {
		t.Errorf("Changed = %v, Tables = %d; want false, 0:\n%s", res.Changed, res.Tables, res.Text)
	}
}

func TestFormat_RefusesAnEscapedPipeAndWritesNothing(t *testing.T) {
	var (
		src = strings.Join([]string{
			"| a | b |",
			"|---|---|",
			"| x \\| y | 2 |",
		}, "\n")
		res = fmt70.Format(src)
	)
	if len(res.Pipes) != 1 {
		t.Fatalf("Pipes = %v; want one finding", res.Pipes)
	}
	if res.Pipes[0].Line != 3 {
		t.Errorf("Line = %d; want 3", res.Pipes[0].Line)
	}
	if res.Changed || res.Text != src {
		t.Errorf("a refused file was rewritten:\n%s", res.Text)
	}
}

func TestFormat_ReportsAWideCellAndStillAlignsIt(t *testing.T) {
	var (
		long = strings.Repeat("x", 80)
		src  = strings.Join([]string{
			"| a | b |",
			"|---|---|",
			"| " + long + " | 2 |",
		}, "\n")
		res = fmt70.Format(src)
	)
	if len(res.Wides) != 1 {
		t.Fatalf("Wides = %d findings; want 1", len(res.Wides))
	}
	if !res.Changed {
		t.Error("a wide cell stopped the alignment; it should only be reported")
	}
}

func TestFormat_WideOffReportsNothing(t *testing.T) {
	var (
		src = "| " + strings.Repeat("x", 80) + " | b |\n|---|---|\n| 1 | 2 |"
		res = Formatter{}.Format(src)
	)
	if len(res.Wides) != 0 {
		t.Errorf("Wides = %d findings; want none with Wide = 0", len(res.Wides))
	}
}

// Prose that happens to start with a pipe is not a table, and a second
// pass over an aligned table changes nothing.
func TestFormat_Idempotent(t *testing.T) {
	var (
		src   = "| a | bbb |\n|---|---|\n| cc | d |"
		once  = fmt70.Format(src)
		twice = fmt70.Format(once.Text)
	)
	if twice.Changed {
		t.Errorf("a second pass changed the file:\n%s\n->\n%s", once.Text, twice.Text)
	}
}

func TestFormat_NotATableWithoutASeparator(t *testing.T) {
	var (
		src = "| not a table\n| nor this"
		res = fmt70.Format(src)
	)
	if res.Changed || res.Tables != 0 {
		t.Errorf("Tables = %d; want 0:\n%s", res.Tables, res.Text)
	}
}

func TestFormat_PadsAShortRow(t *testing.T) {
	var (
		src = "| a | b | c |\n|---|---|---|\n| 1 |"
		res = fmt70.Format(src)
		got = strings.Split(res.Text, "\n")[2]
	)
	if strings.Count(got, "|") != 4 {
		t.Errorf("the short row did not gain its missing cells: %q", got)
	}
}

func TestFormat_KeepsIndentation(t *testing.T) {
	var (
		src = "  | a | bb |\n  |---|---|\n  | ccc | d |"
		res = fmt70.Format(src)
	)
	for _, line := range strings.Split(res.Text, "\n") {
		if !strings.HasPrefix(line, "  |") {
			t.Errorf("a row lost its indentation: %q", line)
		}
	}
}
