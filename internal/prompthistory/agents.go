package prompthistory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// agents.go finds the subagents a turn finished, and renders an agent transcript.

// agentRef is one subagent seen in a turn. The id is the key: the Agent tool's
// result and the hand-back message both carry it, and the agent's own transcript
// is filed under it.
type agentRef struct {
	id          string
	description string
	agentType   string
	model       string
	status      string
	launched    bool
	completed   bool
}

// handbackRe matches the frame a background agent's report arrives in. It is the
// only signal that such an agent has finished, and it may land several turns after
// the launch.
var handbackRe = regexp.MustCompile(`<agent-message from="([A-Za-z0-9_-]+)"`)

// agentsIn collects every subagent this turn mentions, in first-appearance order,
// merging what the launch result, the finish result and the hand-back each know.
func agentsIn(turn []map[string]any) []*agentRef {
	var (
		byID  = map[string]*agentRef{}
		order []string
	)
	ref := func(id string) *agentRef {
		if id == "" {
			return nil
		}
		if a := byID[id]; a != nil {
			return a
		}
		var a = &agentRef{
			id: id,
		}
		byID[id] = a
		order = append(order, id)
		return a
	}
	for _, e := range turn {
		// What the Agent tool itself recorded: `async_launched` for a background
		// agent (it is still running), anything else for one that has finished.
		if r := asMap(e["toolUseResult"]); r != nil {
			if a := ref(asString(r["agentId"])); a != nil {
				a.launched = true
				if d := asString(r["description"]); d != "" {
					a.description = d
				}
				if t := asString(r["agentType"]); t != "" {
					a.agentType = t
				}
				if m := asString(r["resolvedModel"]); m != "" {
					a.model = m
				}
				if s := asString(r["status"]); s != "" {
					a.status = s
					if s != "async_launched" {
						a.completed = true
					}
				}
			}
		}
		// The hand-back that delivers a background agent's final report.
		if asString(e["type"]) == "user" {
			for _, m := range handbackRe.FindAllStringSubmatch(textOf(asMap(e["message"])), -1) {
				if a := ref(m[1]); a != nil {
					a.completed = true
					if a.status == "" || a.status == "async_launched" {
						a.status = "completed"
					}
				}
			}
		}
	}
	var out []*agentRef
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out
}

// agentTranscript is where Claude Code files a subagent's own JSONL: beside the
// session transcript, under <session id>/subagents/agent-<agent id>.jsonl.
func agentTranscript(transcriptPath, id string) string {
	if transcriptPath == "" || id == "" {
		return ""
	}
	base := strings.TrimSuffix(filepath.Base(transcriptPath), ".jsonl")
	return filepath.Join(filepath.Dir(transcriptPath), base, "subagents", "agent-"+id+".jsonl")
}

// agentMetaSidecar is the small JSON Claude Code writes beside a subagent's
// transcript. It names the agent type and the description, which the launch
// result of a background agent does not carry and a hand-back never does.
func agentMetaSidecar(transcriptPath, id string) map[string]any {
	var t = agentTranscript(transcriptPath, id)
	if t == "" {
		return nil
	}
	raw, err := os.ReadFile(strings.TrimSuffix(t, ".jsonl") + ".meta.json")
	if err != nil {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return m
}

// toolSummary is the one-line gist of a tool call, for the rendered transcript.
func toolSummary(name string, input map[string]any) string {
	pick := func(keys ...string) string {
		for _, k := range keys {
			if v := asString(input[k]); v != "" {
				return v
			}
		}
		return ""
	}
	s := ""
	switch name {
	case "Bash":
		s = firstLine(asString(input["command"]), 1<<30)
	case "Agent", "Task":
		s = pick("description", "subagent_type")
	default:
		s = pick("file_path", "notebook_path", "pattern", "path", "url", "query",
			"skill", "description", "command")
	}
	if s == "" {
		if raw, err := json.Marshal(input); err == nil && len(raw) > 2 {
			s = string(raw)
		}
	}
	return strings.ReplaceAll(clip(s, 160), "`", "'")
}

// resultText pulls the readable part out of a tool_result block, whose content is
// either a plain string or a list of blocks.
func resultText(block map[string]any) string {
	c := block["content"]
	if s, ok := c.(string); ok {
		return s
	}
	var parts []string
	for _, b := range asSlice(c) {
		if bm := asMap(b); bm != nil {
			if t := asString(bm["text"]); t != "" {
				parts = append(parts, t)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// renderTranscript writes an agent's run out as markdown: every assistant message
// with its thinking, its text and the tool calls it made, and every tool result,
// each clipped so one `go test` run cannot drown the file.
func renderTranscript(entries []map[string]any) []string {
	var lines []string
	add := func(s string) { lines = append(lines, s) }
	shown := 0
	for i, e := range entries {
		if shown >= transcriptEntryCap {
			add(fmt.Sprintf("_… and %d more transcript entries._", len(entries)-i))
			add("")
			break
		}
		msg := asMap(e["message"])
		if msg == nil {
			continue
		}
		kind := asString(e["type"])
		stamp := ""
		if t := parseTs(e); t != nil {
			stamp = t.Local().Format("15:04:05")
		}
		var body []string
		switch kind {
		case "assistant":
			for _, b := range blocksOf(msg) {
				bm := asMap(b)
				if bm == nil {
					continue
				}
				switch asString(bm["type"]) {
				case "thinking":
					if t := clip(asString(bm["thinking"]), transcriptThinkingCap); t != "" {
						body = append(body, "> "+strings.ReplaceAll(t, "\n", "\n> "), "")
					}
				case "text":
					if t := clip(asString(bm["text"]), transcriptTextCap); t != "" {
						body = append(body, t, "")
					}
				case "tool_use":
					body = append(body, fmt.Sprintf("→ **%s** `%s`", asString(bm["name"]),
						toolSummary(asString(bm["name"]), asMap(bm["input"]))), "")
				}
			}
		case "user":
			if i == 0 {
				continue // the task prompt, rendered above under "## Task"
			}
			if hasToolResult(msg) {
				kind = "tool result"
			}
			for _, b := range blocksOf(msg) {
				bm := asMap(b)
				if bm == nil || asString(bm["type"]) != "tool_result" {
					continue
				}
				if t := clip(resultText(bm), transcriptResultCap); t != "" {
					body = append(addFenced(body, t), "")
				}
			}
			if body == nil {
				if t := clip(textOf(msg), transcriptResultCap); t != "" {
					body = append(body, t, "")
				}
			}
		default:
			continue
		}
		if len(body) == 0 {
			continue
		}
		shown++
		add(fmt.Sprintf("**%s · %s**", stamp, kind))
		add("")
		lines = append(lines, body...)
	}
	return lines
}
