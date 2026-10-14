// Package mdtable aligns the tables of a Markdown file, so that a page
// reads in the raw file and not only in a viewer.
//
// The rule is the one in AGENTS.md, "Markdown style": every cell
// is padded with spaces to the width of the widest cell in its column,
// the separator row is filled with dashes to the same width, and so every
// row of one table is the same length.
//
// Two things the package does NOT do, each for its own reason:
//
//   - A pipe inside a cell, written `\|`, is refused rather than padded.
//     It is two characters in the file and one on screen, so no padding
//     lines a column up in both. The fix is to rewrite the cell, which is
//     a decision about words and not about spaces.
//   - A cell past the width limit is reported and left alone. A cell
//     cannot wrap, so one long cell makes every row long; cutting it is
//     an edit to the prose, which the author makes.
//
// Width is counted in runes, not in bytes. An em dash is one column on
// screen and three bytes in the file, and this repo's pages are full of
// them.
package mdtable

import (
	"strings"
	"unicode/utf8"
)

// minCell is the narrowest a column is written, so that the separator row
// always holds a run of dashes a reader recognises.
const minCell = 3

// Formatter holds the one setting the rule has.
type Formatter struct {
	// Wide reports a cell wider than this many runes. 0 turns the report
	// off. AGENTS.md says "about 70 characters".
	Wide int
}

// Result is what one pass over a file produced.
type Result struct {
	// Text is the file, with every table aligned. It is the input
	// unchanged when Pipes is not empty, because a refused file is not
	// rewritten.
	Text string

	// Tables counts the tables found, aligned or not.
	Tables int

	// Changed reports whether Text differs from the input.
	Changed bool

	// Pipes are the cells holding `\|`. Each one is a refusal.
	Pipes []Finding

	// Wides are the cells past Formatter.Wide. Each one is a note.
	Wides []Finding
}

// Finding is one cell worth reporting, with the line it is on. Line is
// 1-based, so it pairs with an editor and with `file:line`.
type Finding struct {
	Line int
	Cell string
}

// Format aligns every table in src.
//
// A table is a run of lines that begin with a pipe, the second of which
// is a separator row. A run inside a fenced code block is left alone: the
// fences in this repo's pages hold examples of tables, and an example
// that the formatter rewrote would stop being the example.
func (f Formatter) Format(src string) Result {
	var (
		lines  = strings.Split(src, "\n")
		out    = make([]string, 0, len(lines))
		res    Result
		fenced bool
		i      int
		j      int
	)
	for i = 0; i < len(lines); {
		if isFence(lines[i]) {
			fenced = !fenced
		}
		if fenced || !isRow(lines[i]) || i+1 >= len(lines) || !isSeparator(lines[i+1]) {
			out = append(out, lines[i])
			i++
			continue
		}
		for j = i; j < len(lines) && isRow(lines[j]); j++ {
		}
		res.Tables++
		out = append(out, f.table(lines[i:j], i+1, &res)...)
		i = j
	}
	res.Text = strings.Join(out, "\n")
	if len(res.Pipes) > 0 {
		res.Text = src
	}
	res.Changed = res.Text != src
	return res
}

// table aligns one block of rows. at is the 1-based line of its first
// row, which is what a finding is reported against.
func (f Formatter) table(block []string, at int, res *Result) []string {
	var (
		indent = leading(block[0])
		rows   = make([][]string, len(block))
		widths []int
		out    = make([]string, 0, len(block))
		n      int
		w      int
		r      int
		c      int
	)
	for r = range block {
		rows[r] = cells(block[r])
		if len(rows[r]) > n {
			n = len(rows[r])
		}
	}
	widths = make([]int, n)
	for r = range rows {
		for len(rows[r]) < n {
			rows[r] = append(rows[r], "")
		}
		// The separator row is written to fit the others, so its own
		// dashes never set a width.
		if r == 1 {
			continue
		}
		for c = range rows[r] {
			w = utf8.RuneCountInString(rows[r][c])
			if w > widths[c] {
				widths[c] = w
			}
			if strings.Contains(rows[r][c], `\|`) {
				res.Pipes = append(res.Pipes, Finding{
					Line: at + r,
					Cell: rows[r][c],
				})
			}
			if f.Wide > 0 && w > f.Wide {
				res.Wides = append(res.Wides, Finding{
					Line: at + r,
					Cell: rows[r][c],
				})
			}
		}
	}
	for c = range widths {
		if widths[c] < minCell {
			widths[c] = minCell
		}
	}
	for r = range rows {
		if r == 1 {
			out = append(out, indent+separator(rows[r], widths))
			continue
		}
		out = append(out, indent+row(rows[r], widths))
	}
	return out
}

// row writes one data row: a space either side of every cell, and each
// cell padded on the right to its column's width.
func row(cs []string, widths []int) string {
	var b strings.Builder
	b.WriteString("|")
	for c := range cs {
		b.WriteString(" ")
		b.WriteString(cs[c])
		b.WriteString(strings.Repeat(" ", widths[c]-utf8.RuneCountInString(cs[c])))
		b.WriteString(" |")
	}
	return b.String()
}

// separator writes the dashes, keeping each cell's alignment colons. A
// column of width w is w+2 wide between its pipes, which is what makes
// the separator the same length as every other row.
func separator(cs []string, widths []int) string {
	var (
		b    strings.Builder
		left bool
		rght bool
		fill int
	)
	b.WriteString("|")
	for c := range cs {
		left = strings.HasPrefix(cs[c], ":")
		rght = strings.HasSuffix(cs[c], ":") && cs[c] != ":"
		fill = widths[c] + 2
		if left {
			b.WriteString(":")
			fill--
		}
		if rght {
			fill--
		}
		b.WriteString(strings.Repeat("-", fill))
		if rght {
			b.WriteString(":")
		}
		b.WriteString("|")
	}
	return b.String()
}

// cells splits one row on its unescaped pipes and trims each cell. The
// outer pipes are delimiters rather than cells, so a leading and a
// trailing one are dropped.
//
// A `\|` is carried through whole. The caller reports it; this is not the
// place that refuses it, because the separator row has to be split the
// same way to be recognised at all.
func cells(line string) []string {
	var (
		s   = strings.TrimSpace(line)
		out []string
		cur strings.Builder
		i   int
	)
	s = strings.TrimPrefix(s, "|")
	if strings.HasSuffix(s, "|") && !strings.HasSuffix(s, `\|`) {
		s = s[:len(s)-1]
	}
	for i = 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			cur.WriteByte(s[i])
			cur.WriteByte(s[i+1])
			i++
			continue
		}
		if s[i] == '|' {
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
			continue
		}
		cur.WriteByte(s[i])
	}
	return append(out, strings.TrimSpace(cur.String()))
}

// isRow reports whether a line is part of a table.
func isRow(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "|")
}

// isSeparator reports whether a line is the dashes under a header row.
// Every cell of it has to be dashes, with an optional colon at either
// end; one cell that is not makes the whole line an ordinary row.
func isSeparator(line string) bool {
	var cs = cells(line)
	if !isRow(line) || len(cs) == 0 {
		return false
	}
	for i := range cs {
		if !dashes(cs[i]) {
			return false
		}
	}
	return true
}

// dashes reports whether one separator cell is well formed.
func dashes(s string) bool {
	s = strings.TrimPrefix(s, ":")
	s = strings.TrimSuffix(s, ":")
	if s == "" {
		return false
	}
	return strings.Trim(s, "-") == ""
}

// isFence reports whether a line opens or closes a fenced code block.
func isFence(line string) bool {
	var s = strings.TrimSpace(line)
	return strings.HasPrefix(s, "```") || strings.HasPrefix(s, "~~~")
}

// leading returns the indentation of a line, which a table inside a list
// item carries and which every row of that table is written with.
func leading(line string) string {
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}
