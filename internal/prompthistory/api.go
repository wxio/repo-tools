package prompthistory

import (
	"os"
	"path/filepath"
)

// Record writes an entry for the last turn of the transcript the hook payload
// names, or for every turn of it when all is true.
//
// all is the backfill, for a session the hook missed. In that mode an entry's
// time is the turn's own last timestamp and no worktree status is recorded,
// since the tree's state now says nothing about a turn that ended earlier.
//
// A payload naming no readable transcript, or one holding no user prompt, is
// skipped rather than reported: the caller is a Stop hook and must not fail.
func Record(payload map[string]any, all bool) {
	var (
		transcriptPath = asString(payload["transcript_path"])
		turns          [][]map[string]any
		err            error
	)
	if transcriptPath == "" {
		logSkip("no transcript")
		return
	}
	if _, err = os.Stat(transcriptPath); err != nil {
		logSkip("no transcript")
		return
	}

	if turns = splitTurns(loadTranscript(transcriptPath)); len(turns) == 0 {
		logSkip("no user prompt in transcript")
		return
	}
	if !all {
		turns = turns[len(turns)-1:]
	}
	for _, turn := range turns {
		record(payload, turn, all)
	}
}

// Rebuild writes meta-info.csv again from the entries already on disk, and
// reports the file it wrote and how many rows it holds. This is the backfill
// for the CSV: every column it holds is also written into the markdown, so the
// file can always be reconstructed from the history it indexes.
func Rebuild(root string) (path string, rows int, err error) {
	var historyRoot = filepath.Join(repoDir(root), ".prompt-history")
	rows, err = rebuildMeta(historyRoot)
	return metaPath(historyRoot), rows, err
}

// repoDir is the repository a command works in: root if it was given, then the
// git top level of the working directory, then the project directory the hook
// was configured with, then the working directory as it stands.
func repoDir(root string) string {
	var dir = root
	if dir == "" {
		if dir = gitTop(cwd()); dir == "" {
			if dir = os.Getenv("CLAUDE_PROJECT_DIR"); dir == "" {
				dir = cwd()
			}
		}
	}
	return dir
}

// cwd is the working directory, or "" when it cannot be read.
func cwd() string {
	var d, _ = os.Getwd()
	return d
}
