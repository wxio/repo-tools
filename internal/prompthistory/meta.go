package prompthistory

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// meta.go is the meta-info.csv ledger: the row shape, the file, and the lock
// that keeps two sessions from writing it at once.

// csvHeader is the one row shape in .prompt-history/meta-info.csv. Turns and
// agents share it — `kind` says which, and a column that belongs to only one of
// them is empty on the other — because almost everything worth comparing (when,
// branch, model, wall clock, tokens, steps, tool calls) means the same for both.
//
// Every numeric column holds a bare integer: no thousands separators, no "~", no
// "3m 2s". The file is meant to be summed and plotted without being cleaned up
// first, so a number that isn't known is an empty cell rather than a zero or a
// dash. Durations are whole seconds and `when_epoch` is Unix seconds.
var csvHeader = []string{
	"kind", "when", "when_epoch", "branch", "head", "session_id",
	"agent_id", "agent_type", "model",
	"wall_clock_s", "output_tokens", "peak_context", "steps", "tool_calls",
	"files_edited", "commands_run", "agents",
	"entry", "parent", "description",
}

// metaRow is one line of meta-info.csv. The *int fields are "unknown" when nil,
// which an old entry's markdown can be missing but a live one never is.
type metaRow struct {
	kind        string
	when        *time.Time
	branch      string
	head        string
	sessionID   string
	agentID     string
	agentType   string
	model       string
	wallSecs    *int
	outTokens   *int
	peakCtx     *int
	steps       *int
	toolCalls   *int
	filesEdited *int
	cmdsRun     *int
	agents      *int
	entry       string // repo-relative path of the markdown this row indexes
	parent      string // for an agent row, the entry whose turn recorded it
	description string
}

func num(n *int) string {
	if n == nil {
		return ""
	}
	return strconv.Itoa(*n)
}

func (r metaRow) cells() []string {
	when, epoch := "", ""
	if r.when != nil {
		when = r.when.Local().Format(time.RFC3339)
		epoch = strconv.FormatInt(r.when.Unix(), 10)
	}
	return []string{
		r.kind, when, epoch, r.branch, r.head, r.sessionID,
		r.agentID, r.agentType, r.model,
		num(r.wallSecs), num(r.outTokens), num(r.peakCtx), num(r.steps), num(r.toolCalls),
		num(r.filesEdited), num(r.cmdsRun), num(r.agents),
		r.entry, r.parent, r.description,
	}
}

func metaPath(historyRoot string) string { return filepath.Join(historyRoot, "meta-info.csv") }

// metaFile is meta-info.csv in memory. It is small — one line per turn — so it is
// read, amended and rewritten whole rather than appended to, which is also what
// lets an agent's row be updated in place.
type metaFile struct {
	rows [][]string
}

// readMeta loads the CSV, remapping by column name if the header on disk is an
// older or newer shape than csvHeader, so a column added here never discards the
// history already recorded.
func readMeta(historyRoot string) *metaFile {
	var mf = &metaFile{}
	f, err := os.Open(metaPath(historyRoot))
	if err != nil {
		return mf
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil || len(records) == 0 {
		return mf
	}
	head := records[0]
	same := len(head) == len(csvHeader)
	if same {
		for i := range head {
			if head[i] != csvHeader[i] {
				same = false
				break
			}
		}
	}
	for _, rec := range records[1:] {
		if same {
			mf.rows = append(mf.rows, rec)
			continue
		}
		out := make([]string, len(csvHeader))
		for i, name := range csvHeader {
			for j, h := range head {
				if h == name && j < len(rec) {
					out[i] = rec[j]
					break
				}
			}
		}
		mf.rows = append(mf.rows, out)
	}
	return mf
}

func (mf *metaFile) col(name string) int {
	for i, h := range csvHeader {
		if h == name {
			return i
		}
	}
	_ = name
	return -1
}

// agentRow finds the row an agent was already recorded in, so a second sighting
// updates it rather than adding a duplicate.
func (mf *metaFile) agentRow(id string) (int, []string, bool) {
	kind, aid := mf.col("kind"), mf.col("agent_id")
	for i, rec := range mf.rows {
		if kind < len(rec) && aid < len(rec) && rec[kind] == "agent" && rec[aid] == id && id != "" {
			return i, rec, true
		}
	}
	return -1, nil, false
}

// upsert puts a row in: replacing an agent's existing row, appending otherwise.
func (mf *metaFile) upsert(r metaRow) {
	if r.kind == "agent" {
		if i, _, ok := mf.agentRow(r.agentID); ok {
			mf.rows[i] = r.cells()
			return
		}
	}
	mf.rows = append(mf.rows, r.cells())
}

// write replaces meta-info.csv atomically: a temp file in the same directory, then
// a rename, so a reader never sees a half-written index.
func (mf *metaFile) write(historyRoot string) error {
	var (
		tmp *os.File
		err error
	)
	if err = os.MkdirAll(historyRoot, 0o755); err != nil {
		return err
	}
	if tmp, err = os.CreateTemp(historyRoot, ".meta-info-*.csv"); err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	w := csv.NewWriter(tmp)
	if err = w.Write(csvHeader); err != nil {
		tmp.Close()
		return err
	}
	if err = w.WriteAll(mf.rows); err != nil {
		tmp.Close()
		return err
	}
	w.Flush()
	if err = w.Error(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), metaPath(historyRoot))
}

// withMetaLock serialises the read-amend-write of meta-info.csv across the
// concurrent Claude sessions this repo runs. The lock is advisory and bounded:
// after lockWait we proceed anyway, because losing a row is better than wedging
// a session's Stop hook on a lock some killed process left behind.
func withMetaLock(historyRoot string, fn func()) {
	var (
		lock = filepath.Join(historyRoot, ".meta-info.lock")
		f    *os.File
		err  error
	)
	if err = os.MkdirAll(historyRoot, 0o755); err != nil {
		fn()
		return
	}
	const lockWait = 5 * time.Second
	deadline := time.Now().Add(lockWait)
	for {
		if f, err = os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644); err == nil {
			break
		}
		// A lock older than the wait is a leftover; take it.
		if st, serr := os.Stat(lock); serr == nil && time.Since(st.ModTime()) > lockWait {
			_ = os.Remove(lock)
		}
		if time.Now().After(deadline) {
			fn()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	f.Close()
	defer os.Remove(lock)
	fn()
}

// saveMeta amends meta-info.csv with the rows this run produced.
func saveMeta(historyRoot string, rows []metaRow) {
	if len(rows) == 0 {
		return
	}
	withMetaLock(historyRoot, func() {
		mf := readMeta(historyRoot)
		for _, r := range rows {
			mf.upsert(r)
		}
		_ = mf.write(historyRoot)
	})
}
