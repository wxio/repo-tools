// Package notify builds the one-line message a Claude Code hook turns into a
// chat message: the label, the branch, the outcome and how long the turn
// took, and — once a phase lands — the phase's name.
//
// It never names its caller. The label is the whole privacy design: it is
// read from [Options.Label] alone, never from a directory name or a git
// remote, because the directory a hook runs in is often the private name of
// the thing being worked on. This package has no way to read that name by
// accident, because it never looks at anything but the hook's own payload
// and the options it is given.
package notify

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Options carries what Message needs beyond the hook's own payload. It is
// kept small and additive on purpose: a later addition, such as the name of
// a plan phase that just completed, is a new field here, and it changes no
// caller that only sets Label and Elapsed.
type Options struct {
	// Label is the name shown in the chat. It is the only source Message
	// ever reads for that name.
	Label string

	// Elapsed is how long the turn took. Zero omits the "in ..." clause.
	Elapsed time.Duration

	// Phases names the plan sections that became complete during the
	// turn, already stripped of the file each came from (PhaseHeading
	// does that stripping; phase.go is why the names need stripping
	// before they reach this struct). An empty slice omits the clause.
	Phases []string
}

// Message builds one line from the hook's payload and opts: the label, the
// branch, the outcome, and the elapsed time.
//
//	[kite] add-retries · done in 4m12s
//
// The label comes ONLY from opts.Label, never from the payload, a directory
// name, or a git remote. A caller's repository name may be a confidential
// project name, and this package must never let that name reach a chat
// message by reading it out of the environment it happens to run in.
//
// Message never returns an error and never panics: a missing or malformed
// field simply drops its part of the line, and an empty payload still
// produces a harmless line.
func Message(payload map[string]any, opts Options) (line string) {
	defer func() {
		_ = recover()
	}()

	var (
		head  string
		parts []string
		tail  string
	)
	if opts.Label != "" {
		head = "[" + opts.Label + "]"
	}
	if branch := branchOf(payload); branch != "" {
		if head != "" {
			head = head + " " + branch
		} else {
			head = branch
		}
	}
	if head != "" {
		parts = append(parts, head)
	}

	tail = outcomeOf(payload)
	if opts.Elapsed > 0 {
		tail = tail + " in " + formatElapsed(opts.Elapsed)
	}
	parts = append(parts, tail)

	if phase := phasesClause(opts.Phases); phase != "" {
		parts = append(parts, phase)
	}

	line = strings.Join(parts, " · ")
	return line
}

// maxPhaseItems and maxPhaseChars bound the clause phasesClause renders,
// so a turn that completes many phases at once, or one whose heading runs
// long, cannot make the line unreadable or unbounded. A Slack or a
// Telegram message is read on a phone; a one-line summary that grows past
// a phone's width stops being one line in practice.
const (
	maxPhaseItems = 3
	maxPhaseChars = 160
)

// phasesClause renders the phase part of the line: the headings in names,
// joined, and capped. Three or fewer are listed in full; more collapse
// the rest into "and N more". Either form is then cut to maxPhaseChars,
// with "…" marking the cut.
func phasesClause(names []string) string {
	if len(names) == 0 {
		return ""
	}

	var (
		shown = names
		extra = 0
		text  string
	)
	if len(shown) > maxPhaseItems {
		extra = len(shown) - maxPhaseItems
		shown = shown[:maxPhaseItems]
	}
	text = strings.Join(shown, ", ")
	if extra > 0 {
		text = text + fmt.Sprintf(" and %d more", extra)
	}
	if runes := []rune(text); len(runes) > maxPhaseChars {
		text = string(runes[:maxPhaseChars]) + "…"
	}
	return text
}

// branchOf runs `git branch --show-current` in the payload's cwd. A missing
// cwd, a cwd that does not exist, or a cwd that is not a git repository all
// give "", never an error.
func branchOf(payload map[string]any) string {
	var (
		dir string
		ok  bool
		cmd *exec.Cmd
		out []byte
		err error
	)
	if dir, ok = payload["cwd"].(string); !ok || dir == "" {
		return ""
	}

	cmd = exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	cmd.Env = withoutGitEnv(os.Environ())
	if out, err = cmd.Output(); err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// withoutGitEnv drops every GIT_* variable from env.
//
// git exports GIT_DIR, GIT_WORK_TREE and GIT_INDEX_FILE to the commands it
// runs, and they outrank cmd.Dir. A process started from a git hook inherits
// them, so `git branch --show-current` would answer for the hook's own
// repository and not for the directory it was asked about. Dropping them
// leaves cmd.Dir as the only thing that decides.
func withoutGitEnv(env []string) []string {
	var kept = make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, "GIT_") {
			continue
		}
		kept = append(kept, kv)
	}
	return kept
}

// outcomeOf is "failed" when the payload names a StopFailure event or carries
// a non-empty error_type or error_message, and "done" otherwise.
func outcomeOf(payload map[string]any) string {
	if asString(payload["hook_event_name"]) == "StopFailure" {
		return "failed"
	}
	if asString(payload["error_type"]) != "" || asString(payload["error_message"]) != "" {
		return "failed"
	}
	return "done"
}

// asString reads a string field of the payload, or "" when it is absent or
// holds a value of another type.
func asString(v any) string {
	var s, _ = v.(string)
	return s
}

// formatElapsed renders d compactly: seconds alone under a minute, minutes
// and seconds under an hour, hours and minutes at an hour or more. Each form
// drops the unit below the one it carries, rather than show a zero.
func formatElapsed(d time.Duration) string {
	var (
		total = int(d.Round(time.Second).Seconds())
		h     = total / 3600
		m     = (total % 3600) / 60
		s     = total % 60
	)
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
