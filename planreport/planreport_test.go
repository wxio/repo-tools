package planreport

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestCount(t *testing.T) {
	var cases = []struct {
		name     string
		src      string
		open     int
		progress int
		done     int
	}{
		{
			name: "markers",
			src:  "- [ ] a\n* [x] b\n+ [X] c\n1. [ ] d\n2) [x] e\n",
			open: 2,
			done: 3,
		},
		{
			name: "nested",
			src:  "- [ ] a\n  - [x] b\n    - [ ] c\n",
			open: 2,
			done: 1,
		},
		{
			name: "not a task",
			src:  "[ ] a\n- [] b\n- [ ]c\n-[x] d\ntext [x] e\n",
			open: 0,
			done: 0,
		},
		{
			name:     "in progress",
			src:      "- [~] a\n- [ ] b\n  - [~] c\n- [x] d\n- [~]e\n",
			open:     1,
			progress: 2,
			done:     1,
		},
		{
			name: "fence",
			src:  "- [ ] a\n```\n- [ ] b\n- [~] c\n```\n- [x] d\n~~~md\n- [ ] e\n~~~\n",
			open: 1,
			done: 1,
		},
	}
	for _, c := range cases {
		var open, progress, done = Count(c.src)
		if open != c.open || progress != c.progress || done != c.done {
			t.Errorf("%s: Count = %d open, %d in progress, %d done; want %d, %d, %d",
				c.name, open, progress, done, c.open, c.progress, c.done)
		}
	}
}

func TestPercent(t *testing.T) {
	var cases = []struct {
		row  Row
		want int
	}{
		{
			row: Row{
				Open: 0,
				Done: 0,
			},
			want: 0,
		},
		{
			row: Row{
				Open: 2,
				Done: 1,
			},
			want: 33,
		},
		{
			row: Row{
				Open: 1,
				Done: 2,
			},
			want: 67,
		},
		{
			row: Row{
				Open: 0,
				Done: 4,
			},
			want: 100,
		},
		{
			row: Row{
				Open:     1,
				Progress: 2,
				Done:     1,
			},
			want: 25,
		},
	}
	for _, c := range cases {
		if got := c.row.Percent(); got != c.want {
			t.Errorf("%+v: Percent = %d, want %d", c.row, got, c.want)
		}
	}
}

func TestReport(t *testing.T) {
	var (
		dir   = t.TempDir()
		files = map[string]string{
			"a/plan.md":     "- [ ] one\n- [x] two\n",
			"a/plan-ui.md":  "- [~] three\n",
			"a/prd.md":      "- [ ] not matched\n",
			"b/plan.md":     "- [x] one\n- [x] two\n- [x] three\n",
			"b/sub/plan.md": "- [ ] nested\n",
			"plan.md":       "- [ ] root\n",
			"README.md":     "- [ ] not matched\n",
		}
		rows []Row
		want = []Row{
			{
				Feature:  "a",
				Link:     "a/plan-ui.md",
				Open:     1,
				Progress: 1,
				Done:     1,
			},
			{
				Feature: "b",
				Link:    "b/plan.md",
				Open:    0,
				Done:    3,
			},
			{
				Feature: "b/sub",
				Link:    "b/sub/plan.md",
				Open:    1,
				Done:    0,
			},
			{
				Feature: "plan.md",
				Link:    "plan.md",
				Open:    1,
				Done:    0,
			},
		}
		err error
	)
	for name, src := range files {
		var path = filepath.Join(dir, name)
		if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if rows, err = Report(dir, regexp.MustCompile(`^plan.*\.md$`)); err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(want) {
		t.Fatalf("Report = %+v, want %+v", rows, want)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], want[i])
		}
	}
}

func TestTable(t *testing.T) {
	var (
		got = Table([]Row{
			{
				Feature:  "a",
				Link:     "a/plan.md",
				Open:     1,
				Progress: 2,
				Done:     1,
			},
			{
				Feature: "longer-name",
				Link:    "longer-name/plan.md",
				Open:    0,
				Done:    3,
			},
		})
		want = "| feature                              | open | in progress | done | total | % complete |\n" +
			"|--------------------------------------|------|-------------|------|-------|------------|\n" +
			"| [a](./a/plan.md)                     | 1    | 2           | 1    | 4     | 25%        |\n" +
			"| [longer-name](./longer-name/plan.md) | 0    | 0           | 3    | 3     | 100%       |\n"
	)
	if got != want {
		t.Errorf("Table =\n%s\nwant\n%s", got, want)
	}
}
