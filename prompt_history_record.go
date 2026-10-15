package main

// prompt_history_record.go is the prompt-history-record subcommand: the Claude
// Code Stop hook that writes a prompt-history entry for every turn, and one for
// every subagent the session delegated to.
//
// It reads the hook JSON on stdin (transcript_path, cwd, ...) and hands it to
// prompthistory.Record, which does the work; see that package for what an entry
// holds and where it lands. It is registered in .claude/settings.json.
//
//	-all   record every turn of the transcript rather than the last one, in
//	       order: a backfill, for a session the hook missed.
//	-meta  add a row to .prompt-history/meta-info.csv for each entry. It is on
//	       by default; -meta=false turns it off.
//
// The hook is best-effort: any failure is swallowed and it always exits 0, so it
// can never block or break a session.

import (
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/wxio/sheafdb-tools/internal/prompthistory"
)

// promptHistoryRecord is the command line of prompt-history-record.
type promptHistoryRecord struct {
	All  bool `help:"record every turn of the transcript, not only the last"`
	Meta bool `help:"add a row to .prompt-history/meta-info.csv for each entry; -meta=false turns it off"`
}

func (c *promptHistoryRecord) Run() error {
	var (
		payload = map[string]any{}
		raw     []byte
	)
	// A panic is swallowed too. Run then returns nil and the hook exits 0.
	defer func() {
		_ = recover()
	}()

	raw, _ = io.ReadAll(os.Stdin)
	if strings.TrimSpace(string(raw)) != "" {
		_ = json.Unmarshal(raw, &payload)
	}
	prompthistory.Record(payload, prompthistory.Options{
		All:  c.All,
		Meta: c.Meta,
	})
	return nil
}
