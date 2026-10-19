Transcript files are now named `NNN-YYYYMMDD-HHmm-<descr>.md`. The change is in the working tree and is not staged. You staged the guard change from the last turn, and I left the index as it was.

**What changed**

- `internal/prompthistory/record.go`: the new `entryFileName` makes the name. The time is the local time at the end of the turn. A backfill uses the time of the last message in the turn.
- The timing block now comes before the file claim, because the name needs the time.
- Each agent directory gets the same name as its entry: `NNN-YYYYMMDD-HHmm-<descr>/agent_<descr>.md`.
- `nextSeq` still reads only the leading `NNN-`, so the numbers continue after old `NNN-<descr>.md` files.
- `prompt-history-meta` reads the entry content, not the file name, so it needs no change.
- The name format is updated in `internal/prompthistory/doc.go` and `docs/prompt-history.md`, section 1.
- `internal/prompthistory/record_test.go` is new. It tests the name, and `nextSeq` over old and new names.

**Checks**

- `go test ./...`, `go vet ./...` and `md-table-fmt -n` pass.
- I recorded a small transcript twice in a scratch repository. The output was `001-20261019-0641-rename-the-history-files.md` and then `002-...`. 19:41 UTC is 06:41 AEDT.
- `prompt-history-meta` rebuilt the index from the new files: 2 rows.

**Not in this change**

- The code still writes to `.prompt-history/`, not `.prompts/history/`. I changed the file name only. Moving the directory is a separate change.
- `docs/prompt-history.md` holds both changes: your staged guard edits and my unstaged name edits. `git add -p` can keep them apart, if necessary.

20261018 - 21:17 **Next**: say "commit" to commit the staged guard change. Then say "commit" again for the name change.
