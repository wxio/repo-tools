// Command prompt-history-record is the Claude Code Stop hook that writes a
// prompt-history entry for every turn, and one for every subagent the session
// delegated to.
//
// It reads the hook JSON on stdin (transcript_path, cwd, ...) and hands it to
// prompthistory.Record, which does the work; see that package for what an entry
// holds and where it lands. It is registered in .claude/settings.json.
//
//	-all   record every turn of the transcript rather than the last one, in
//	       order: a backfill, for a session the hook missed.
//
// The hook is best-effort: any failure is swallowed and it always exits 0, so it
// can never block or break a session.
package main

import (
	"encoding/json"
	"flag"
	"io"
	"os"
	"strings"

	"github.com/wxio/sheafdb-tools/internal/prompthistory"
)

func main() {
	defer os.Exit(0)
	run()
}

func run() {
	var (
		all     = flag.Bool("all", false, "record every turn of the transcript, not only the last")
		payload = map[string]any{}
		raw     []byte
	)
	flag.Parse()

	raw, _ = io.ReadAll(os.Stdin)
	if strings.TrimSpace(string(raw)) != "" {
		_ = json.Unmarshal(raw, &payload)
	}
	prompthistory.Record(payload, *all)
}
