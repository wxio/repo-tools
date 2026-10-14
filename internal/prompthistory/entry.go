package prompthistory

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// entry.go reads an entry back out of its markdown, which is what lets
// meta-info.csv be rebuilt from the history it indexes.

var (
	backtickRe = regexp.MustCompile("`([^`]*)`")
	durPartRe  = regexp.MustCompile(`(\d+)\s*([hms])`)
	intRe      = regexp.MustCompile(`\d[\d,]*`)
)

func backticked(line string) []string {
	var out []string
	for _, m := range backtickRe.FindAllStringSubmatch(line, -1) {
		out = append(out, m[1])
	}
	return out
}

func nth(xs []string, i int) string {
	if i < len(xs) {
		return xs[i]
	}
	return ""
}

// parseDurationSecs reads back what fmtDuration wrote: "51m 33s", "17s",
// "1h 2m 3s", or "—" for unknown.
func parseDurationSecs(s string) *int {
	total, found := 0, false
	for _, m := range durPartRe.FindAllStringSubmatch(s, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		found = true
		switch m[2] {
		case "h":
			total += n * 3600
		case "m":
			total += n * 60
		default:
			total += n
		}
	}
	if !found {
		return nil
	}
	return &total
}

// parseInts pulls every integer out of a line, dropping the thousands separators
// the markdown is written with.
func parseInts(s string) []int {
	var out []int
	for _, m := range intRe.FindAllString(s, -1) {
		if n, err := strconv.Atoi(strings.ReplaceAll(m, ",", "")); err == nil {
			out = append(out, n)
		}
	}
	return out
}

func intAt(xs []int, i int) *int {
	if i < len(xs) {
		return ptr(xs[i])
	}
	return nil
}

// countList counts the bullets under a bold heading, following the "… and N more"
// line the renderer writes when it truncates a long list. A heading that isn't
// there means none of that kind, not "unknown" — but only in an entry this hook
// wrote, which the "What was done" section identifies.
func countList(lines []string, heading string) *int {
	found := false
	for i, ln := range lines {
		if strings.TrimSpace(ln) == "## What was done" {
			found = true
		}
		if strings.TrimSpace(ln) != heading {
			continue
		}
		n := 0
		for _, item := range lines[i+1:] {
			s := strings.TrimSpace(item)
			if s == "" {
				continue
			}
			if !strings.HasPrefix(s, "- ") {
				break
			}
			if strings.Contains(s, "… and ") {
				if more := parseInts(s); len(more) > 0 {
					n += more[0]
				}
				continue
			}
			n++
		}
		return ptr(n)
	}
	if found {
		return ptr(0)
	}
	return nil
}

// parseEntry reads one markdown entry back into the row that indexes it. Every
// CSV column is written into the markdown, so this is enough to rebuild the whole
// index from the history alone — which is what -csv does.
func parseEntry(path, historyRoot string) (metaRow, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return metaRow{}, false
	}
	var (
		lines = strings.Split(string(raw), "\n")
		row   = metaRow{
			kind: "prompt",
		}
	)
	rel, err := filepath.Rel(filepath.Dir(historyRoot), path)
	if err != nil {
		rel = path
	}
	row.entry = filepath.ToSlash(rel)

	for _, ln := range lines {
		s := strings.TrimSpace(ln)
		switch {
		case strings.HasPrefix(s, "# "):
			if row.description == "" {
				title := strings.TrimPrefix(s, "# ")
				if _, after, ok := strings.Cut(title, " — "); ok {
					title = after
				}
				row.description = title
			}
		case strings.HasPrefix(s, "- **When:** "):
			txt := strings.TrimPrefix(s, "- **When:** ")
			if t, err := time.ParseInLocation("2006-01-02 15:04:05 MST", txt, time.Local); err == nil {
				row.when = &t
			}
		case strings.HasPrefix(s, "- **Branch:**"):
			b := backticked(s)
			row.branch, row.head = nth(b, 0), nth(b, 1)
		case strings.HasPrefix(s, "- **Model:**"):
			row.model = nth(backticked(s), 0)
		case strings.HasPrefix(s, "- **Agent:**"):
			b := backticked(s)
			row.kind, row.agentID, row.agentType = "agent", nth(b, 0), nth(b, 1)
		case strings.HasPrefix(s, "- **Parent:**"):
			row.parent = nth(backticked(s), 0)
		case strings.HasPrefix(s, "- **Session:**"):
			row.sessionID = nth(backticked(s), 0)
		case strings.HasPrefix(s, "- **Wall clock:**"):
			row.wallSecs = parseDurationSecs(strings.TrimPrefix(s, "- **Wall clock:**"))
		case strings.HasPrefix(s, "- **Tokens:**"):
			n := parseInts(s)
			row.outTokens, row.peakCtx = intAt(n, 0), intAt(n, 1)
		case strings.HasPrefix(s, "- **Agents:**"):
			row.agents = intAt(parseInts(s), 0)
		case strings.HasPrefix(s, "| | **Total** |"):
			// | | **Total** | **51m 33s** | **15** | **25** | **54,158** |
			cells := strings.Split(strings.Trim(s, "|"), "|")
			if len(cells) >= 6 {
				row.steps = intAt(parseInts(cells[3]), 0)
				row.toolCalls = intAt(parseInts(cells[4]), 0)
			}
		}
	}
	row.filesEdited = countList(lines, "**Files written/edited this turn:**")
	row.cmdsRun = countList(lines, "**Commands run:**")
	if row.kind == "prompt" && row.agents == nil {
		row.agents = ptr(countAgentFiles(strings.TrimSuffix(path, ".md")))
	}
	return row, true
}

func countAgentFiles(dir string) int {
	n := 0
	if ents, err := os.ReadDir(dir); err == nil {
		for _, e := range ents {
			if !e.IsDir() && strings.HasPrefix(e.Name(), "agent_") && strings.HasSuffix(e.Name(), ".md") {
				n++
			}
		}
	}
	return n
}

// rebuildMeta writes meta-info.csv from scratch out of every entry under
// .prompt-history: the backfill, and the proof that the CSV is derived rather
// than a second source of truth.
func rebuildMeta(historyRoot string) (int, error) {
	var (
		rows []metaRow
		err  error
	)
	err = filepath.WalkDir(historyRoot, func(path string, d os.DirEntry, werr error) error {
		if werr != nil {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		if r, ok := parseEntry(path, historyRoot); ok {
			rows = append(rows, r)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.when != nil && b.when != nil && !a.when.Equal(*b.when) {
			return a.when.Before(*b.when)
		}
		if (a.when == nil) != (b.when == nil) {
			return b.when == nil
		}
		return a.entry < b.entry
	})
	var mf = &metaFile{}
	for _, r := range rows {
		mf.rows = append(mf.rows, r.cells())
	}
	var werr error
	withMetaLock(historyRoot, func() { werr = mf.write(historyRoot) })
	return len(rows), werr
}
