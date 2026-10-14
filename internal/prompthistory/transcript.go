package prompthistory

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// transcript.go reads a session transcript: the loosely typed JSON it holds,
// the text helpers over it, and the split into turns.

// Tool -> phase category. A single assistant message may issue several tool calls
// in parallel; the highest-priority category present wins.
var (
	editTools = map[string]bool{
		"Write": true, "Edit": true, "MultiEdit": true, "NotebookEdit": true,
	}
	execTools = map[string]bool{
		"Bash": true,
	}
	researchTools = map[string]bool{
		"Read": true, "Grep": true, "Glob": true, "LS": true, "WebFetch": true,
		"WebSearch": true, "Task": true, "Agent": true, "ToolSearch": true,
		"NotebookRead": true, "BashOutput": true, "Skill": true,
	}
	// agentTools launch a subagent. Their result carries the agent id the agent's
	// own transcript is filed under.
	agentTools = map[string]bool{
		"Agent": true, "Task": true,
	}
)

// How long an agent's transcript must have gone untouched before a turn that
// merely mentions the agent records it anyway. Long enough that a working agent
// is never cut short, short enough that a killed one is still picked up.
const agentStaleAfter = 10 * time.Minute

// Caps on the rendered agent transcript. A whole agent run is often megabytes of
// tool output; these keep a readable record without copying the lot.
const (
	transcriptTextCap     = 6000
	transcriptThinkingCap = 1500
	transcriptResultCap   = 1200
	transcriptEntryCap    = 600
)

// ---- small dynamic-JSON accessors (the transcript is loosely typed) ----

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func asSlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return 0
}

func ptr(n int) *int { return &n }

func logSkip(string) {
	// Silent by design.
}

func git(projectDir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = projectDir
	var out strings.Builder
	cmd.Stdout = &out
	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return ""
	}
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		return out.String()
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		return ""
	}
}

func textOf(message map[string]any) string {
	c := message["content"]
	if s, ok := c.(string); ok {
		return s
	}
	var parts []string
	for _, b := range asSlice(c) {
		if bm := asMap(b); bm != nil && asString(bm["type"]) == "text" {
			parts = append(parts, asString(bm["text"]))
		}
	}
	return strings.Join(parts, "\n")
}

func blocksOf(message map[string]any) []any {
	return asSlice(message["content"])
}

func hasToolResult(message map[string]any) bool {
	for _, b := range blocksOf(message) {
		if bm := asMap(b); bm != nil && asString(bm["type"]) == "tool_result" {
			return true
		}
	}
	return false
}

func isRealUserPrompt(entry map[string]any) bool {
	if asString(entry["type"]) != "user" {
		return false
	}
	if meta, ok := entry["isMeta"].(bool); ok && meta {
		return false
	}
	msg := asMap(entry["message"])
	if msg == nil {
		msg = map[string]any{}
	}
	if hasToolResult(msg) {
		return false
	}
	// A message that is nothing but tags -- a slash command's own record, its
	// output, an IDE notice -- is not a prompt, so cleaning it decides.
	return cleanPrompt(textOf(msg)) != ""
}

func parseTs(entry map[string]any) *time.Time {
	ts := asString(entry["timestamp"])
	if ts == "" {
		return nil
	}
	ts = strings.Replace(ts, "Z", "+00:00", 1)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05-07:00"} {
		if t, err := time.Parse(layout, ts); err == nil {
			return &t
		}
	}
	return nil
}

func fmtDuration(seconds *float64) string {
	if seconds == nil || *seconds < 0 {
		return "—"
	}
	total := int(*seconds + 0.5)
	if total < 60 {
		return fmt.Sprintf("%ds", total)
	}
	m, s := total/60, total%60
	if m < 60 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	h := m / 60
	m = m % 60
	return fmt.Sprintf("%dh %dm %ds", h, m, s)
}

// secs rounds a duration to whole seconds for the CSV, which carries no units.
func secs(seconds *float64) *int {
	if seconds == nil || *seconds < 0 {
		return nil
	}
	return ptr(int(*seconds + 0.5))
}

var tagRe = map[string]*regexp.Regexp{}

func cleanPrompt(text string) string {
	tags := []string{"system-reminder", "ide_selection", "ide_opened_file",
		"command-message", "command-name", "command-args",
		"local-command-stdout", "local-command-caveat"}
	for _, tag := range tags {
		re := tagRe[tag]
		if re == nil {
			re = regexp.MustCompile(`(?s)<` + tag + `>.*?</` + tag + `>`)
			tagRe[tag] = re
		}
		text = re.ReplaceAllString(text, "")
	}
	return strings.TrimSpace(text)
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(text string, maxlen int) string {
	first := ""
	for _, ln := range strings.Split(text, "\n") {
		if s := strings.TrimSpace(ln); s != "" {
			first = s
			break
		}
	}
	slug := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(first), "-"), "-")
	if len(slug) > maxlen {
		slug = slug[:maxlen]
	}
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "changes"
	}
	return slug
}

// firstLine is the one-line description a row and a title carry.
func firstLine(text string, maxlen int) string {
	for _, ln := range strings.Split(text, "\n") {
		if s := strings.TrimSpace(ln); s != "" {
			if len(s) > maxlen {
				s = s[:maxlen]
			}
			return s
		}
	}
	return ""
}

// clip cuts a string to at most n bytes without splitting a rune.
func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8Start(s[n]) {
		n--
	}
	return strings.TrimSpace(s[:n]) + " …"
}

func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

// fenceFor returns a code fence long enough to quote text that may itself contain
// one: a prompt written in markdown, or a tool result that printed a file with a
// fenced block in it. Without this a single ``` inside the quoted text ends the
// block early and the rest of the entry renders as prose.
func fenceFor(text string) string {
	longest, run := 0, 0
	for i := 0; i < len(text); i++ {
		if text[i] == '`' {
			run++
			if run > longest {
				longest = run
			}
			continue
		}
		run = 0
	}
	if longest < 3 {
		return "```"
	}
	return strings.Repeat("`", longest+1)
}

// addFenced appends text as a fenced block that the text cannot break out of.
func addFenced(lines []string, text string) []string {
	f := fenceFor(text)
	return append(lines, f, text, f)
}

func categoryFor(tools []string) string {
	has := func(set map[string]bool) bool {
		for _, t := range tools {
			if set[t] {
				return true
			}
		}
		return false
	}
	if has(editTools) {
		return "Editing"
	}
	if has(execTools) {
		return "Executing"
	}
	if has(researchTools) {
		return "Researching"
	}
	if len(tools) > 0 {
		return "Tooling"
	}
	return "Thinking"
}

func loadTranscript(path string) []map[string]any {
	var entries []map[string]any
	f, err := os.Open(path)
	if err != nil {
		return entries
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err == nil {
			entries = append(entries, e)
		}
	}
	return entries
}

// splitTurns cuts the transcript into turns, each starting at a real user prompt
// and running up to the next one. The last turn runs to the end of the file.
func splitTurns(entries []map[string]any) [][]map[string]any {
	var starts []int
	for i, e := range entries {
		if isRealUserPrompt(e) {
			starts = append(starts, i)
		}
	}
	var turns [][]map[string]any
	for n, start := range starts {
		end := len(entries)
		if n+1 < len(starts) {
			end = starts[n+1]
		}
		turns = append(turns, entries[start:end])
	}
	return turns
}

// repoFor resolves the repository a turn worked in: the top level of the first
// file it wrote or edited, then of the session's current directory, then
// CLAUDE_PROJECT_DIR, then the hook's own working directory. The first two
// follow a session into a git worktree; the last two are what it was opened in.
func repoFor(info turnInfo, payload map[string]any) string {
	for _, f := range info.editedFiles {
		if filepath.IsAbs(f) {
			if t := gitTop(filepath.Dir(f)); t != "" {
				return t
			}
		}
	}
	if t := gitTop(asString(payload["cwd"])); t != "" {
		return t
	}
	if d := os.Getenv("CLAUDE_PROJECT_DIR"); d != "" {
		return d
	}
	if d := asString(payload["cwd"]); d != "" {
		return d
	}
	d, _ := os.Getwd()
	return d
}

func gitTop(dir string) string {
	if dir == "" {
		return ""
	}
	return strings.TrimSpace(git(dir, "rev-parse", "--show-toplevel"))
}
