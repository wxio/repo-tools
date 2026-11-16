// Package notify is the notify subcommand: the Claude Code hook that turns
// the end of a turn into a line in a chat app. It reads the hook's JSON on
// stdin, as prompthistoryrecord does, builds the line with
// internal/notify.Message, and sends it to the transports --to or
// NOTIFY_TO name.
//
// Flags, each defaulted from an environment variable where one exists. A
// flag given on the command line wins over its variable:
//
//	flag            variable             what it does
//	--to            NOTIFY_TO            the transports: --to slack,telegram
//	--min-seconds   NOTIFY_MIN_SECONDS   send nothing for a turn shorter than this; 60
//	--text          —                    add the turn's last line to the message; off
//	--mark-start    —                    write the start mark and exit; for UserPromptSubmit
//	--label         NOTIFY_LABEL         the name shown in the chat
//	--plan-glob     NOTIFY_PLAN_GLOB     which files are plans, for the phase line
//	--dry-run       —                    build the message, print it, ignoring --min-seconds
//	--timeout       —                    the HTTP timeout, in seconds; 10
//
// --dry-run is how the early phases of this command are built: it builds
// the whole message and prints it instead of sending it, so the message
// and the quiet rule are finished and tested before any transport exists
// and before any key does. --mark-start writes the hook's session_id and
// the current time to internal/notify's start mark, then returns: it does
// nothing else, and a payload with no session_id makes it do nothing at
// all.
//
// A normal run (anything but --mark-start) reads that mark back, by the
// payload's session_id, to get how long the turn took. internal/notify's
// ShouldSend then decides whether the turn is worth a message: shorter
// than --min-seconds and quiet, at or above it and worth sending, and a
// missing mark always worth sending, because a session that began before
// the hook was installed cannot be told apart from a short one any other
// way. --dry-run ignores that decision and always prints the line, so a
// person can see what would have gone out even on a turn the quiet rule
// would otherwise drop. Either way, a normal run deletes the mark it
// read and sweeps whatever else is stale, so the temporary directory
// does not grow by one file per turn.
//
// --mark-start also records which sections of the plan files --plan-glob
// matches are complete right now, so a normal run can name every section
// that became complete since. A phase landing is sent whatever
// --min-seconds says: PRD section 8 calls a phase that lands in 20
// seconds still news, so ShouldSend's quiet verdict is overridden
// whenever a phase landed. NOTIFY_PHASES=0 turns the whole phase line
// off, in both --mark-start and a normal run; there is no flag for it,
// only the variable, so the setting cannot be repeated in the committed
// hook line.
//
// It is best-effort, like prompthistoryrecord: any failure is swallowed,
// a panic with it, and it always exits 0. The hook cannot block a
// session and cannot fail one. A failure is written to one line of a log
// file under os.TempDir(), never to standard error, because a hook's
// standard error reaches the session.
package notify

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	internalnotify "github.com/wxio/repo-tools/internal/notify"
)

// defaultMinSeconds is the quiet threshold when NOTIFY_MIN_SECONDS is
// unset or does not parse as a whole number.
const defaultMinSeconds = 60

// defaultPlanGlob is the glob NewNotifyCmd uses when NOTIFY_PLAN_GLOB is
// unset. It is a convention of this command, not a path above the
// repository it is run in: any repository may keep its plans elsewhere
// and set the variable.
const defaultPlanGlob = "docs/features/*/plan.md"

// markSweepAge is how old a start mark has to be before a normal run
// removes it as stale, on the suspicion that its session never reached
// its Stop hook. A run that does find and read its own mark deletes that
// one directly; this sweep is only for the ones left behind.
const markSweepAge = 24 * time.Hour

// logPath is where a failure is written: one line, appended, under the
// system temporary directory, beside the per-session start marks a later
// change adds there. Never standard error — a hook's standard error
// reaches the Claude Code session, and this command must never put a
// line in front of the person who just stopped reading.
var logPath = filepath.Join(os.TempDir(), "repo-tools-notify.log")

// NotifyCmd is the command line of notify.
type NotifyCmd struct {
	To         string `help:"the transports: --to slack,telegram"`
	MinSeconds int    `help:"send nothing for a turn shorter than this, in seconds"`
	Text       bool   `help:"add the turn's last line to the message; off"`
	MarkStart  bool   `help:"write the start mark and exit; for UserPromptSubmit"`
	Label      string `help:"the name shown in the chat"`
	PlanGlob   string `help:"which files are plans, for the phase line"`
	DryRun     bool   `help:"build the message, print it, ignoring --min-seconds"`
	Timeout    int    `help:"the HTTP timeout, in seconds"`
}

// NewNotifyCmd returns the command line of notify, with its flags
// defaulted from the environment variables a repository may set. A flag
// given on the command line still wins: opts only reaches this default
// when the flag is absent.
func NewNotifyCmd() *NotifyCmd {
	var (
		cmd        *NotifyCmd
		minSeconds = defaultMinSeconds
		planGlob   = defaultPlanGlob
		raw        string
		n          int
		err        error
	)
	if raw = os.Getenv("NOTIFY_MIN_SECONDS"); raw != "" {
		if n, err = strconv.Atoi(raw); err == nil {
			minSeconds = n
		}
	}
	if raw = os.Getenv("NOTIFY_PLAN_GLOB"); raw != "" {
		planGlob = raw
	}
	cmd = &NotifyCmd{
		To:         os.Getenv("NOTIFY_TO"),
		MinSeconds: minSeconds,
		Label:      os.Getenv("NOTIFY_LABEL"),
		PlanGlob:   planGlob,
		Timeout:    10,
	}
	return cmd
}

// Run reads the hook's JSON from stdin, builds the message, and either
// prints it (--dry-run) or hands it to the configured transports. It
// always returns nil: a panic or an error here must never block or fail
// the hook that calls it.
func (c *NotifyCmd) Run() error {
	var (
		payload     = map[string]any{}
		raw         []byte
		err         error
		line        string
		transports  []string
		sessionID   string
		haveMark    bool
		mark        internalnotify.Mark
		elapsed     time.Duration
		phasesOn    = os.Getenv("NOTIFY_PHASES") != "0"
		dir         string
		donePhases  []string
		newlyPhases []string
		phaseHeads  []string
	)
	// A panic is swallowed too. Run then returns nil and the hook exits 0.
	defer func() {
		_ = recover()
	}()

	raw, _ = io.ReadAll(os.Stdin)
	if strings.TrimSpace(string(raw)) != "" {
		if err = json.Unmarshal(raw, &payload); err != nil {
			logFailure(fmt.Sprintf("reading the hook's JSON: %v", err))
			return nil
		}
	}

	dir, _ = payload["cwd"].(string)

	if c.MarkStart {
		var ok bool
		if sessionID, ok = payload["session_id"].(string); !ok || sessionID == "" {
			// No session_id, nothing to key the mark on. The
			// UserPromptSubmit hook always sends one; a payload
			// without it does nothing rather than guess.
			return nil
		}
		if phasesOn {
			donePhases = internalnotify.DonePhases(dir, c.PlanGlob)
		}
		if err = internalnotify.WriteMark(sessionID, internalnotify.Mark{
			Start:      time.Now(),
			DonePhases: donePhases,
		}); err != nil {
			logFailure(fmt.Sprintf("writing the start mark: %v", err))
		}
		return nil
	}

	// A normal run: read the mark the UserPromptSubmit hook wrote for
	// this session, so the message and the quiet rule below both have
	// an elapsed time to work from. haveMark stays false, and elapsed
	// stays zero, when there is no session_id or no mark for it —
	// ShouldSend treats that the same as a long turn, not a short one.
	if sessionID, _ = payload["session_id"].(string); sessionID != "" {
		if mark, haveMark = internalnotify.ReadMark(sessionID); haveMark {
			elapsed = time.Since(mark.Start)
		}
		// The mark has done its job. Remove it so the temporary
		// directory does not gain one file per turn.
		internalnotify.DeleteMark(sessionID)
	}
	// Best-effort housekeeping for a session whose Stop hook never ran
	// at all, so its mark is never read and never deleted above.
	internalnotify.SweepMarks(markSweepAge)

	if phasesOn {
		donePhases = internalnotify.DonePhases(dir, c.PlanGlob)
		newlyPhases = internalnotify.NewlyComplete(mark.DonePhases, donePhases)
		for _, name := range newlyPhases {
			phaseHeads = append(phaseHeads, internalnotify.PhaseHeading(name))
		}
	}

	line = internalnotify.Message(payload, internalnotify.Options{
		Label:   c.Label,
		Elapsed: elapsed,
		Phases:  phaseHeads,
	})

	if c.DryRun {
		// Always prints, whatever ShouldSend would say: --dry-run
		// exists so a person can see the message a quiet turn would
		// otherwise drop.
		fmt.Println(line)
		return nil
	}

	// A phase landing overrides the quiet rule: PRD section 8 says a
	// phase that lands in 20 seconds is still news, so len(phaseHeads) > 0
	// sends even when ShouldSend would otherwise say the turn was too
	// short.
	if !internalnotify.ShouldSend(elapsed, c.MinSeconds, haveMark) && len(phaseHeads) == 0 {
		// The turn was shorter than --min-seconds, and a start mark
		// said so with confidence, and no phase landed either. Say
		// nothing, send nothing.
		return nil
	}

	transports = internalnotify.Transports(c.To)
	if len(transports) == 0 {
		// No transport is configured. Nothing is sent, and nothing is
		// printed: --dry-run is the only way this command speaks.
		return nil
	}

	// A later change sends line to each of transports: SendSlack for
	// "slack", with SLACK_WEBHOOK_URL, and SendTelegram for "telegram",
	// with TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID. Neither exists yet,
	// so a configured transport still sends nothing this phase.

	return nil
}

// logFailure appends one line to logPath: the time, and msg. It never
// writes to standard error, and it swallows its own failure — a log file
// this command cannot open must not be a second way for it to misbehave.
func logFailure(msg string) {
	var (
		f   *os.File
		err error
	)
	if f, err = os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s notify: %s\n", time.Now().Format(time.RFC3339), msg)
}
