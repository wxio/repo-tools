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
//	--dry-run       —                    build the message, send nothing, print it
//	--timeout       —                    the HTTP timeout, in seconds; 10
//
// --dry-run is how the early phases of this command are built: it builds
// the whole message and prints it instead of sending it, so the message
// and the quiet rule are finished and tested before any transport exists
// and before any key does. --mark-start is accepted here and does
// nothing yet; a later change writes the start mark it names.
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
	DryRun     bool   `help:"build the message, send nothing, print it"`
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
		payload    = map[string]any{}
		raw        []byte
		err        error
		line       string
		transports []string
	)
	// A panic is swallowed too. Run then returns nil and the hook exits 0.
	defer func() {
		_ = recover()
	}()

	if c.MarkStart {
		// A later change writes the start mark here, keyed by the
		// payload's session_id, and returns. Until then --mark-start
		// is accepted and does nothing.
		return nil
	}

	raw, _ = io.ReadAll(os.Stdin)
	if strings.TrimSpace(string(raw)) != "" {
		if err = json.Unmarshal(raw, &payload); err != nil {
			logFailure(fmt.Sprintf("reading the hook's JSON: %v", err))
			return nil
		}
	}

	line = internalnotify.Message(payload, internalnotify.Options{
		Label: c.Label,
	})

	if c.DryRun {
		fmt.Println(line)
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
