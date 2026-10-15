# prompt-history

Writes one Markdown entry per turn of a Claude Code session, and one per
subagent, into the repository the turn worked in.

```sh
sheafdb-tools prompt-history-meta                          # rebuild the index
sheafdb-tools prompt-history-meta -root /path/to/repo      # ... for another repository
sheafdb-tools prompt-history-record -all < hook.json       # backfill a missed session
sheafdb-tools prompt-history-record -meta=false < hook.json  # an entry, and no CSV row
prek run prompt-history-guard --all-files
```

Three subcommands of `sheafdb-tools`, and one library:

| program                  | what it does                                                        |
|--------------------------|---------------------------------------------------------------------|
| `prompt-history-record`  | the Claude Code `Stop` hook: writes an entry per turn and per agent |
| `prompt-history-meta`    | rebuilds `.prompt-history/meta-info.csv` from the entries on disk   |
| `prompt-history-guard`   | the prek hook: keeps `.prompt-history/` in commits of its own       |
| `internal/prompthistory` | the library behind the first two                                    |

## 1. What it writes

```
<repo>/.prompt-history/<branch>/NNN-<description>.md
<repo>/.prompt-history/<branch>/NNN-<description>/agent_<description>.md
<repo>/.prompt-history/meta-info.csv
```

An entry holds a header, a phase breakdown, the prompt, and a summary of
what the turn did. An agent's file adds its whole transcript. The CSV is
an index over the entries, one row per turn and per agent, every numeric
column a bare integer.

Every turn is recorded, whether or not it changed anything. The history
is of the prompts.

## 2. Where the entry lands

`<repo>` is the repository the turn worked in, which is not always the
one the session was opened in. The order is:

1. the first file the turn wrote or edited,
2. the session's current directory,
3. `CLAUDE_PROJECT_DIR`.

A session that `cd`s into a git worktree and edits there is recorded in
that worktree, under that worktree's branch.

**This is why the tools moved out of `sheafdb`.** The commands name no
path above the repository they are given, so they work for any repository
that installs them. Nothing in them is about sheafdb.

## 3. Install in a repository

Take the flake, so the commands are on `PATH`. Then register the `Stop`
hook in `.claude/settings.json`:

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "sheafdb-tools prompt-history-record",
            "timeout": 30,
            "statusMessage": "Recording prompt history"
          }
        ]
      }
    ]
  }
}
```

And the guard in `prek.toml`:

```toml
{
  id = "prompt-history-guard",
  name = ".prompt-history is committed on its own",
  entry = "sheafdb-tools prompt-history-guard",
  language = "system",
  pass_filenames = false,
  always_run = true
}
```

Exclude `.prompt-history` from the other hooks. The transcripts are
generated and reformatting them is noise.

## 4. The guard

A commit holds the transcripts, or it holds everything else. It does not
hold both.

The guard takes the `.prompt-history/` paths back out of the index when
the commit stages anything else, and refuses the commit if the mix is
still there. A commit of transcripts alone is left alone.

By hand:

```sh
git restore --staged .prompt-history
git commit -m "the code"
git add .prompt-history && git commit -m "the transcripts"
```

## 5. Gotchas

- **The record hook never fails.** Any error is swallowed and it exits 0,
  so it can never block or break a session. A missing entry is therefore
  silent; `-all` with the session's hook JSON on stdin backfills it.
- **An agent is recorded where it finished, not where it started.** At
  launch its transcript is still being written. A background agent's
  hand-back can arrive several turns later.
- **An agent's tokens are not in the turn's numbers.** Each agent gets a
  row of its own, so summing a column of the CSV counts every unit of
  work exactly once.
- **The CSV is derived.** Every column it holds is also in the Markdown.
  `prompt-history-meta` reconstructs it, which is also the backfill for a
  history written before the CSV existed.
- **`-meta=false` turns the CSV off.** The record hook then writes the
  entries and leaves `meta-info.csv` as it is. `prompt-history-meta`
  builds the CSV later. The CSV is also how a continued agent finds its
  file, so with the CSV off a continued agent gets a second file.
- **Do not read or edit the transcripts.** They are generated. An agent
  that reads them burns context on its own past output.

## 6. Experiences

**2026-10-14.** Moved out of `sheafdb/tools/` into this repository. The
import path changed from `tools/internal/prompthistory` to
`github.com/wxio/sheafdb-tools/internal/prompthistory`. The repository
resolution in `api.go` and `transcript.go` already read `git rev-parse`,
`CLAUDE_PROJECT_DIR` and the working directory in that order, so the
commands needed no change to work from outside the repository they
record. That was the test of whether the split was possible.

**2026-10-15.** The four commands became subcommands of one binary,
`sheafdb-tools`, on `github.com/jpillora/opts`. A hook now names
`sheafdb-tools prompt-history-record`, not `prompt-history-record`. The
record hook got `-meta`, on by default, to turn the CSV row off.
