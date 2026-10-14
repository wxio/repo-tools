package prompthistory

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// summary.go reduces one turn to what an entry reports: its phases, its tools,
// the files it touched and the numbers that go in a row.

type step struct {
	ts       *time.Time
	category string
	tools    []string
	out      int
	model    string
}

type phase struct {
	category  string
	steps     int
	toolCalls int
	out       int
	start     *time.Time
	end       *time.Time
	duration  *float64
}

type turnInfo struct {
	phases       []*phase
	editedFiles  []string
	bashCmds     []string
	totalOut     int
	peakCtx      int
	totalSteps   int
	totalTools   int
	finalText    string
	model        string
	usedEditTool bool
	usedBash     bool
	firstTs      *time.Time
	lastTs       *time.Time
}

type group struct {
	ts    *time.Time
	tools []string
	out   int
	ctx   int
	model string
}

// summarizeTurn collects steps, phases, tool usage and token totals from the turn.
//
// Claude Code often splits one assistant API message across several transcript
// entries that each repeat the same `usage`. We group entries by message id so
// one API message = one logical step (tokens counted once), and dedup tool_use
// blocks by their block id so a repeated block is not double-listed.
//
// It is used on a session turn and, unchanged, on a whole agent transcript: an
// agent run is one prompt and the steps that answered it, which is the same shape.
func summarizeTurn(turn []map[string]any) turnInfo {
	var (
		editedFiles []string
		bashCmds    []string
		finalText   string
		firstTs     *time.Time
		lastTs      *time.Time
	)
	seenBlocks := map[string]bool{}
	groups := map[string]*group{}
	var order []string

	for idx, e := range turn {
		if ts := parseTs(e); ts != nil {
			if firstTs == nil || ts.Before(*firstTs) {
				firstTs = ts
			}
			if lastTs == nil || ts.After(*lastTs) {
				lastTs = ts
			}
		}
		if asString(e["type"]) != "assistant" {
			continue
		}
		msg := asMap(e["message"])
		if msg == nil {
			msg = map[string]any{}
		}
		mid := asString(msg["id"])
		if mid == "" {
			mid = asString(e["uuid"])
		}
		if mid == "" {
			mid = fmt.Sprintf("idx%d", idx)
		}
		g := groups[mid]
		if g == nil {
			g = &group{ts: parseTs(e), model: asString(msg["model"])}
			groups[mid] = g
			order = append(order, mid)
		}

		if ts := parseTs(e); ts != nil && (g.ts == nil || ts.Before(*g.ts)) {
			g.ts = ts
		}

		usage := asMap(msg["usage"])
		// Split entries repeat identical usage — take the value, don't sum.
		if o := asInt(usage["output_tokens"]); o > g.out {
			g.out = o
		}
		ctx := asInt(usage["input_tokens"]) + asInt(usage["cache_read_input_tokens"]) +
			asInt(usage["cache_creation_input_tokens"])
		if ctx > g.ctx {
			g.ctx = ctx
		}

		for _, b := range blocksOf(msg) {
			bm := asMap(b)
			if bm == nil {
				continue
			}
			switch asString(bm["type"]) {
			case "tool_use":
				bid := asString(bm["id"])
				if bid != "" && seenBlocks[bid] {
					continue
				}
				if bid != "" {
					seenBlocks[bid] = true
				}
				name := asString(bm["name"])
				g.tools = append(g.tools, name)
				inp := asMap(bm["input"])
				switch name {
				case "Write", "Edit", "MultiEdit":
					if fp := asString(inp["file_path"]); fp != "" {
						editedFiles = append(editedFiles, fp)
					}
				case "NotebookEdit":
					if fp := asString(inp["notebook_path"]); fp != "" {
						editedFiles = append(editedFiles, fp)
					}
				case "Bash":
					cmd := strings.Split(strings.TrimSpace(asString(inp["command"])), "\n")
					if len(cmd) > 0 && cmd[0] != "" {
						first := cmd[0]
						if len(first) > 100 {
							first = first[:100]
						}
						bashCmds = append(bashCmds, first)
					}
				}
			case "text":
				if t := strings.TrimSpace(asString(bm["text"])); t != "" {
					finalText = t
				}
			}
		}
	}

	var (
		steps    []step
		totalOut int
		peakCtx  int
	)
	for _, mid := range order {
		g := groups[mid]
		totalOut += g.out
		if g.ctx > peakCtx {
			peakCtx = g.ctx
		}
		steps = append(steps, step{
			ts:       g.ts,
			category: categoryFor(g.tools),
			tools:    g.tools,
			out:      g.out,
			model:    g.model,
		})
	}

	// Merge consecutive same-category steps into phases.
	var phases []*phase
	for _, st := range steps {
		if len(phases) > 0 && phases[len(phases)-1].category == st.category {
			p := phases[len(phases)-1]
			p.steps++
			p.toolCalls += len(st.tools)
			p.out += st.out
			if st.ts != nil {
				p.end = st.ts
			}
		} else {
			phases = append(phases, &phase{
				category:  st.category,
				steps:     1,
				toolCalls: len(st.tools),
				out:       st.out,
				start:     st.ts,
				end:       st.ts,
			})
		}
	}

	// Dedup preserving order.
	editedFiles = dedup(editedFiles)

	model := ""
	usedEdit := false
	usedBash := false
	totalSteps, totalTools := 0, 0
	for _, s := range steps {
		totalSteps++
		totalTools += len(s.tools)
		if model == "" && s.model != "" {
			model = s.model
		}
		for _, t := range s.tools {
			if editTools[t] {
				usedEdit = true
			}
			if execTools[t] {
				usedBash = true
			}
		}
	}

	return turnInfo{
		phases:       phases,
		editedFiles:  editedFiles,
		bashCmds:     bashCmds,
		totalOut:     totalOut,
		peakCtx:      peakCtx,
		totalSteps:   totalSteps,
		totalTools:   totalTools,
		finalText:    finalText,
		model:        model,
		usedEditTool: usedEdit,
		usedBash:     usedBash,
		firstTs:      firstTs,
		lastTs:       lastTs,
	}
}

func dedup(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// branchName returns the current git branch, or "" if git is unavailable, this
// isn't a repo, or HEAD is detached. Purely best-effort metadata — it never
// gates the run.
func branchName(projectDir string) string {
	b := strings.TrimSpace(git(projectDir, "rev-parse", "--abbrev-ref", "HEAD"))
	if b == "" || b == "HEAD" { // "HEAD" == detached
		return ""
	}
	return b
}

var seqRe = regexp.MustCompile(`^(\d+)-`)

func nextSeq(dirPath string) int {
	n := 0
	if entries, err := os.ReadDir(dirPath); err == nil {
		for _, ent := range entries {
			if m := seqRe.FindStringSubmatch(ent.Name()); m != nil {
				if v, err := strconv.Atoi(m[1]); err == nil && v > n {
					n = v
				}
			}
		}
	}
	return n + 1
}

// comma formats an integer with thousands separators, mirroring Python's "{:,}".
func comma(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}
