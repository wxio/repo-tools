package prompthistory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// record.go writes an entry: the turn's own file, an agent's file beside it,
// and the row each one adds to the ledger.

// agentEntry is one agent, recorded: where its file went and what the file says.
type agentEntry struct {
	ref  *agentRef
	info turnInfo
	path string // absolute
	rel  string // repo-relative, for the turn's own entry to point at
	wall *float64
	row  metaRow
}

// record writes one turn's entry. backfill says the turn ended some time ago:
// its clock is then the transcript's, and the worktree's status is left out.
func record(payload map[string]any, turn []map[string]any, backfill bool) {
	promptEntry := turn[0]
	info := summarizeTurn(turn)
	projectDir := repoFor(info, payload)
	historyRoot := filepath.Join(projectDir, ".prompt-history")

	// Post-turn worktree state, rendered into the entry. Our own output is
	// filtered out so the history never lists itself as a change.
	var changed []string
	if !backfill {
		status := git(projectDir, "-c", "core.quotepath=false", "status", "--porcelain", "--untracked-files=no")
		for _, line := range strings.Split(status, "\n") {
			if len(line) > 3 && !strings.Contains(line, ".prompt-history/") {
				changed = append(changed, strings.TrimRight(line, " \t\r\n"))
			}
		}
	}

	// Branch/HEAD are opportunistic git metadata. When git is unavailable (or this
	// isn't a repo, or HEAD is detached) we fall back to a fixed directory name so
	// the hook still records — it just groups the entry under "no-branch".
	branch := branchName(projectDir)
	if branch == "" {
		branch = "no-branch"
	}
	branchDirName := strings.Trim(regexp.MustCompile(`[^A-Za-z0-9._-]+`).ReplaceAllString(branch, "-"), "-")
	if branchDirName == "" {
		branchDirName = "no-branch"
	}
	head := strings.TrimSpace(git(projectDir, "rev-parse", "--short", "HEAD"))
	if head == "" {
		head = "(none)"
	}

	promptText := cleanPrompt(textOf(asMap(promptEntry["message"])))
	slug := slugify(promptText, 48)

	outDir := filepath.Join(historyRoot, branchDirName)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		logSkip("mkdir failed")
		return
	}

	// Claim the filename atomically with O_CREATE|O_EXCL: nextSeq() only picks a
	// starting point, and if a concurrent session already took it the create fails
	// with EEXIST and we bump to the next number. This makes numbering safe across
	// simultaneous Claude sessions without any lock file — the kernel guarantees
	// the exclusive create on a local filesystem.
	var (
		f     *os.File
		fpath string
		err   error
	)
	seq := nextSeq(outDir)
	for {
		fpath = filepath.Join(outDir, fmt.Sprintf("%03d-%s.md", seq, slug))
		if f, err = os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644); err == nil {
			break
		}
		if os.IsExist(err) {
			seq++
			if seq < 100000 { // sanity backstop against a pathological loop
				continue
			}
		}
		logSkip("open failed")
		return
	}
	defer f.Close()

	// Timing. The turn ends "now", but guard against a stale/replayed transcript
	// (clock skew or an old file) by falling back to the last message timestamp
	// when "now" is implausibly far past it.
	startTs := parseTs(promptEntry)
	now := time.Now().UTC()
	var lastTs *time.Time
	for _, e := range turn {
		if t := parseTs(e); t != nil && (lastTs == nil || t.After(*lastTs)) {
			lastTs = t
		}
	}
	endRef := now
	if lastTs != nil && (backfill || now.Sub(*lastTs).Seconds() > 6*3600) {
		endRef = *lastTs
		now = *lastTs
	}
	var totalSecs *float64
	if startTs != nil {
		d := endRef.Sub(*startTs).Seconds()
		totalSecs = &d
	}

	phases := info.phases
	for i, p := range phases {
		start := p.start
		var end *time.Time
		if i+1 < len(phases) {
			end = phases[i+1].start
		} else {
			end = &endRef
		}
		if start != nil && end != nil {
			d := end.Sub(*start).Seconds()
			p.duration = &d
		}
	}

	entryName := filepath.Base(fpath)
	entryRel := filepath.ToSlash(filepath.Join(".prompt-history", branchDirName, entryName))

	// Agents. Everything an agent did lives in its own transcript, so none of it
	// is in the numbers above: the entry counts the main thread, each agent gets
	// its own file and its own CSV row, and nothing is counted twice.
	agents := recordAgents(agentRecordCtx{
		payload:       payload,
		turn:          turn,
		historyRoot:   historyRoot,
		entryDir:      strings.TrimSuffix(fpath, ".md"),
		entryName:     entryName,
		entryRel:      entryRel,
		projectDir:    projectDir,
		branch:        branch,
		head:          head,
		branchDirName: branchDirName,
		now:           now,
	})

	// ---- Render markdown ----
	var lines []string
	add := func(s string) { lines = append(lines, s) }

	title := "(no prompt text)"
	if promptText != "" {
		title = firstLine(promptText, 80)
	}
	add(fmt.Sprintf("# %03d — %s", seq, title))
	add("")
	add(fmt.Sprintf("- **When:** %s", now.Local().Format("2006-01-02 15:04:05 MST")))
	add(fmt.Sprintf("- **Branch:** `%s`  ·  **HEAD:** `%s`", branch, head))
	if info.model != "" {
		add(fmt.Sprintf("- **Model:** `%s`", info.model))
	}
	add(fmt.Sprintf("- **Wall clock:** %s", fmtDuration(totalSecs)))
	add(fmt.Sprintf("- **Tokens:** %s output  ·  ~%s peak context",
		comma(info.totalOut), comma(info.peakCtx)))
	add(fmt.Sprintf("- **Agents:** %d", len(agents)))
	if sid := asString(payload["session_id"]); sid != "" {
		add(fmt.Sprintf("- **Session:** `%s`", sid))
	}
	add("")

	add("## Phases")
	add("")
	add("| # | Phase | Duration | Assistant steps | Tool calls | Output tokens |")
	add("|---|-------|---------:|----------------:|-----------:|--------------:|")
	sumSteps, sumTools := 0, 0
	for i, p := range phases {
		add(fmt.Sprintf("| %d | %s | %s | %d | %d | %s |",
			i+1, p.category, fmtDuration(p.duration), p.steps, p.toolCalls, comma(p.out)))
		sumSteps += p.steps
		sumTools += p.toolCalls
	}
	add(fmt.Sprintf("| | **Total** | **%s** | **%d** | **%d** | **%s** |",
		fmtDuration(totalSecs), sumSteps, sumTools, comma(info.totalOut)))
	add("")

	add("## Prompt")
	add("")
	if promptText != "" {
		lines = addFenced(lines, promptText)
	} else {
		lines = addFenced(lines, "(empty)")
	}
	add("")

	if len(agents) > 0 {
		add("## Agents")
		add("")
		add("Their tokens, tool calls and wall clock are **not** in the totals above —")
		add("each agent ran in its own transcript and is recorded in its own file.")
		add("")
		add("| Agent | Type | Model | Wall clock | Output tokens | Files | Commands | Entry |")
		add("|-------|------|-------|-----------:|--------------:|------:|---------:|-------|")
		for _, a := range agents {
			add(fmt.Sprintf("| %s | %s | `%s` | %s | %s | %d | %d | [%s](%s) |",
				a.row.description, dashIfEmpty(a.ref.agentType), a.ref.model,
				fmtDuration(a.wall), comma(a.info.totalOut),
				len(a.info.editedFiles), len(a.info.bashCmds),
				filepath.Base(a.path), relLink(fpath, a.path)))
		}
		add("")
	}

	add("## What was done")
	add("")
	if len(info.editedFiles) == 0 && len(info.bashCmds) == 0 {
		if len(agents) > 0 {
			add(fmt.Sprintf("Nothing directly: the work was delegated to %d agent(s), above.", len(agents)))
		} else {
			add("No files written and no commands run.")
		}
		add("")
	}
	if len(info.editedFiles) > 0 {
		add("**Files written/edited this turn:**")
		add("")
		for _, fp := range info.editedFiles {
			add(fmt.Sprintf("- `%s`", relTo(projectDir, fp)))
		}
		add("")
	}
	if len(info.bashCmds) > 0 {
		add("**Commands run:**")
		add("")
		for _, c := range info.bashCmds[:min(len(info.bashCmds), 20)] {
			add(fmt.Sprintf("- `%s`", c))
		}
		if len(info.bashCmds) > 20 {
			add(fmt.Sprintf("- … and %d more", len(info.bashCmds)-20))
		}
		add("")
	}
	if len(changed) > 0 {
		add("**Worktree status after turn:**")
		add("")
		shown := changed[:min(len(changed), 60)]
		if len(changed) > 60 {
			shown = append(append([]string{}, shown...),
				fmt.Sprintf("… and %d more", len(changed)-60))
		}
		lines = addFenced(lines, strings.Join(shown, "\n"))
		add("")
	}
	if info.finalText != "" {
		add("**Summary:**")
		add("")
		add(info.finalText)
		add("")
	}

	if _, err := f.WriteString(strings.Join(lines, "\n")); err != nil {
		logSkip("write failed")
		return
	}

	// The index last, so a row never points at a file that isn't there yet.
	rows := []metaRow{{
		kind:        "prompt",
		when:        &now,
		branch:      branch,
		head:        head,
		sessionID:   asString(payload["session_id"]),
		model:       info.model,
		wallSecs:    secs(totalSecs),
		outTokens:   ptr(info.totalOut),
		peakCtx:     ptr(info.peakCtx),
		steps:       ptr(sumSteps),
		toolCalls:   ptr(sumTools),
		filesEdited: ptr(len(info.editedFiles)),
		cmdsRun:     ptr(len(info.bashCmds)),
		agents:      ptr(len(agents)),
		entry:       entryRel,
		description: title,
	}}
	for _, a := range agents {
		rows = append(rows, a.row)
	}
	saveMeta(historyRoot, rows)

	msg := "📝 Wrote " + entryRel
	if len(agents) > 0 {
		msg += fmt.Sprintf(" (+%d agent)", len(agents))
	}
	if out, err := json.Marshal(map[string]string{"systemMessage": msg}); err == nil {
		fmt.Println(string(out))
	}
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func relTo(base, path string) string {
	if !filepath.IsAbs(path) {
		return path
	}
	rel, err := filepath.Rel(base, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path // outside the repo — show as given
	}
	return filepath.ToSlash(rel)
}

// relLink is the markdown link from one entry to another file beside it.
func relLink(from, to string) string {
	rel, err := filepath.Rel(filepath.Dir(from), to)
	if err != nil {
		return filepath.ToSlash(to)
	}
	return filepath.ToSlash(rel)
}

type agentRecordCtx struct {
	payload       map[string]any
	turn          []map[string]any
	historyRoot   string
	entryDir      string // <branch>/NNN-<description>, the directory agents go in
	entryName     string
	entryRel      string
	projectDir    string
	branch        string
	head          string
	branchDirName string
	now           time.Time
}

// recordAgents writes a file per subagent this turn finished, and returns what it
// wrote so the turn's own entry can point at it.
//
// Three rules decide what gets written, and together they are what stops an
// agent being counted twice:
//
//   - only a FINISHED agent is recorded, so its transcript is whole when read.
//     A launch alone is not enough — the turn that sees the hand-back records it.
//   - an agent already in meta-info.csv is updated in place, file and row both,
//     so continuing one with SendMessage adds to its record rather than making
//     a second one.
//   - nothing an agent did is added to the turn's own totals; the two are kept
//     side by side so either can be summed on its own.
func recordAgents(c agentRecordCtx) []agentEntry {
	var (
		refs           = agentsIn(c.turn)
		transcriptPath = asString(c.payload["transcript_path"])
		out            []agentEntry
		known          *metaFile
	)
	if len(refs) == 0 {
		return nil
	}
	known = readMeta(c.historyRoot)

	for _, ref := range refs {
		var tpath = agentTranscript(transcriptPath, ref.id)
		if tpath == "" {
			continue
		}
		st, err := os.Stat(tpath)
		if err != nil {
			continue // no transcript — a launch we cannot read, or a foreign agent
		}
		// An agent that has not handed back may still be working. Record it anyway
		// once its transcript has gone quiet for long enough that it plainly isn't.
		if !ref.completed && time.Since(st.ModTime()) < agentStaleAfter {
			continue
		}

		var entries = loadTranscript(tpath)
		if len(entries) == 0 {
			continue
		}
		if m := agentMetaSidecar(transcriptPath, ref.id); m != nil {
			if ref.description == "" {
				ref.description = asString(m["description"])
			}
			if ref.agentType == "" {
				ref.agentType = asString(m["agentType"])
			}
		}
		var (
			info   = summarizeTurn(entries)
			prompt = ""
		)
		if len(entries) > 0 {
			prompt = cleanPrompt(textOf(asMap(entries[0]["message"])))
		}
		if ref.description == "" {
			ref.description = firstLine(prompt, 60)
		}
		if ref.model == "" {
			ref.model = info.model
		}
		if ref.status == "" {
			ref.status = "unknown"
		}
		var wall *float64
		if info.firstTs != nil && info.lastTs != nil {
			d := info.lastTs.Sub(*info.firstTs).Seconds()
			wall = &d
		}

		// An agent seen before keeps the file it already has, and the entry it was
		// first filed under: one agent, one record, wherever it is continued from.
		var (
			apath  string
			parent = c.entryRel
		)
		if rec, prevParent := agentRowFor(known, ref.id); rec != "" {
			apath = filepath.Join(filepath.Dir(c.historyRoot), filepath.FromSlash(rec))
			if _, err := os.Stat(filepath.Dir(apath)); err != nil {
				apath = ""
			} else if prevParent != "" {
				parent = prevParent
			}
		}
		if apath == "" {
			if err := os.MkdirAll(c.entryDir, 0o755); err != nil {
				continue
			}
			apath = claimAgentFile(c.entryDir, slugify(ref.description, 48))
			if apath == "" {
				continue
			}
		}

		var when = c.now
		if info.lastTs != nil {
			when = info.lastTs.UTC()
		}
		body := renderAgent(agentDoc{
			ref:       ref,
			info:      info,
			prompt:    prompt,
			entries:   entries,
			wall:      wall,
			when:      when,
			branch:    c.branch,
			head:      c.head,
			parent:    filepath.Base(parent),
			sessionID: asString(c.payload["session_id"]),
			project:   c.projectDir,
		})
		if err := os.WriteFile(apath, []byte(strings.Join(body, "\n")), 0o644); err != nil {
			continue
		}

		arel, err := filepath.Rel(filepath.Dir(c.historyRoot), apath)
		if err != nil {
			arel = apath
		}
		out = append(out, agentEntry{
			ref:  ref,
			info: info,
			path: apath,
			rel:  filepath.ToSlash(arel),
			wall: wall,
			row: metaRow{
				kind:        "agent",
				when:        &when,
				branch:      c.branch,
				head:        c.head,
				sessionID:   asString(c.payload["session_id"]),
				agentID:     ref.id,
				agentType:   ref.agentType,
				model:       ref.model,
				wallSecs:    secs(wall),
				outTokens:   ptr(info.totalOut),
				peakCtx:     ptr(info.peakCtx),
				steps:       ptr(info.totalSteps),
				toolCalls:   ptr(info.totalTools),
				filesEdited: ptr(len(info.editedFiles)),
				cmdsRun:     ptr(len(info.bashCmds)),
				entry:       filepath.ToSlash(arel),
				parent:      parent,
				description: ref.description,
			},
		})
	}
	return out
}

// agentRowFor returns the entry an agent's file was written to and the turn that
// first recorded it, or "" for each it cannot find.
func agentRowFor(mf *metaFile, id string) (entry, parent string) {
	_, rec, ok := mf.agentRow(id)
	if !ok {
		return "", ""
	}
	at := func(name string) string {
		c := mf.col(name)
		if c < 0 || c >= len(rec) {
			return ""
		}
		return rec[c]
	}
	return at("entry"), at("parent")
}

// claimAgentFile takes agent_<slug>.md in dir, numbering past a collision the way
// the turn entries do.
func claimAgentFile(dir, slug string) string {
	for n := 1; n < 1000; n++ {
		name := "agent_" + slug + ".md"
		if n > 1 {
			name = fmt.Sprintf("agent_%s-%d.md", slug, n)
		}
		path := filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			f.Close()
			return path
		}
		if !os.IsExist(err) {
			return ""
		}
	}
	return ""
}

type agentDoc struct {
	ref       *agentRef
	info      turnInfo
	prompt    string
	entries   []map[string]any
	wall      *float64
	when      time.Time
	branch    string
	head      string
	parent    string
	sessionID string
	project   string
}

// renderAgent writes one agent's file: the same header and phase table a turn
// gets, then the task it was given, what it touched, its report, and its whole
// transcript.
func renderAgent(d agentDoc) []string {
	var lines []string
	add := func(s string) { lines = append(lines, s) }

	add(fmt.Sprintf("# Agent — %s", firstLine(d.ref.description, 80)))
	add("")
	add(fmt.Sprintf("- **When:** %s", d.when.Local().Format("2006-01-02 15:04:05 MST")))
	add(fmt.Sprintf("- **Branch:** `%s`  ·  **HEAD:** `%s`", d.branch, d.head))
	if d.ref.model != "" {
		add(fmt.Sprintf("- **Model:** `%s`", d.ref.model))
	}
	add(fmt.Sprintf("- **Agent:** `%s`  ·  **Type:** `%s`  ·  **Status:** `%s`",
		d.ref.id, dashIfEmpty(d.ref.agentType), d.ref.status))
	add(fmt.Sprintf("- **Parent:** `%s`", d.parent))
	add(fmt.Sprintf("- **Wall clock:** %s", fmtDuration(d.wall)))
	add(fmt.Sprintf("- **Tokens:** %s output  ·  ~%s peak context",
		comma(d.info.totalOut), comma(d.info.peakCtx)))
	if d.sessionID != "" {
		add(fmt.Sprintf("- **Session:** `%s`", d.sessionID))
	}
	add("")

	add("## Phases")
	add("")
	add("| # | Phase | Duration | Assistant steps | Tool calls | Output tokens |")
	add("|---|-------|---------:|----------------:|-----------:|--------------:|")
	phases := d.info.phases
	for i, p := range phases {
		var end *time.Time
		if i+1 < len(phases) {
			end = phases[i+1].start
		} else {
			end = d.info.lastTs
		}
		if p.start != nil && end != nil {
			secs := end.Sub(*p.start).Seconds()
			p.duration = &secs
		}
		add(fmt.Sprintf("| %d | %s | %s | %d | %d | %s |",
			i+1, p.category, fmtDuration(p.duration), p.steps, p.toolCalls, comma(p.out)))
	}
	add(fmt.Sprintf("| | **Total** | **%s** | **%d** | **%d** | **%s** |",
		fmtDuration(d.wall), d.info.totalSteps, d.info.totalTools, comma(d.info.totalOut)))
	add("")

	add("## Task")
	add("")
	if d.prompt != "" {
		lines = addFenced(lines, d.prompt)
	} else {
		lines = addFenced(lines, "(empty)")
	}
	add("")

	add("## What was done")
	add("")
	if len(d.info.editedFiles) == 0 && len(d.info.bashCmds) == 0 {
		add("No files written and no commands run.")
		add("")
	}
	if len(d.info.editedFiles) > 0 {
		add("**Files written/edited this turn:**")
		add("")
		for _, fp := range d.info.editedFiles {
			add(fmt.Sprintf("- `%s`", relTo(d.project, fp)))
		}
		add("")
	}
	if len(d.info.bashCmds) > 0 {
		add("**Commands run:**")
		add("")
		for _, c := range d.info.bashCmds[:min(len(d.info.bashCmds), 20)] {
			add(fmt.Sprintf("- `%s`", c))
		}
		if len(d.info.bashCmds) > 20 {
			add(fmt.Sprintf("- … and %d more", len(d.info.bashCmds)-20))
		}
		add("")
	}
	if d.info.finalText != "" {
		add("**Report:**")
		add("")
		add(d.info.finalText)
		add("")
	}

	add("## Transcript")
	add("")
	lines = append(lines, renderTranscript(d.entries)...)
	return lines
}
