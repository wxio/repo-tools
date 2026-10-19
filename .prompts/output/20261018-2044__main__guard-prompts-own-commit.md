`prompt-history-guard` now keeps everything under `.prompts/` out of a commit that holds other files. The change is in the working tree and is not staged.

**What it does now**

- The record directories are `.prompts/` and `.prompt-history/`. The guard keeps `.prompt-history/` because `prompt-history-record` still writes there.
- The two record directories can share a commit. A commit of records alone passes.
- A mixed commit: the guard unstages only the record directories that are staged, then commits the rest.
- The report and its `git restore --staged` hint name only the staged directories, because `git restore` rejects a path that matches nothing.

**Files**

| file                                            | change                                                     |
|-------------------------------------------------|------------------------------------------------------------|
| `prompthistoryguard/prompthistoryguard.go`      | `recordPrefixes`, `recordDir`, `recordDirs`; reworded docs |
| `prompthistoryguard/prompthistoryguard_test.go` | cases for `.prompts/output/`, mixed records, `recordDirs`  |
| `main.go`, `README.md`, `AGENTS.md`             | the guard summary names `.prompts/`                        |
| `docs/prompt-history.md`                        | section 4 describes both directories; table re-aligned     |

**Checks**

- `go test ./...`, `go vet ./...` and `md-table-fmt -n` pass.
- The separability greps find nothing.
- I ran the built binary in a scratch repository. It handled a mixed commit with an unborn HEAD, a records-only commit, and a `.prompts/output/` plus code commit.

**Outside this change, not done**

- `docs/prompt-history.md` says the transcripts go in `.prompts/history/`. The code still writes `.prompt-history/`. Moving `internal/prompthistory` to the new path is a separate change.
- The 5 old replies in `docs/output/` are still tracked there. They are not under `.prompts/`, so the guard does not see them.

20261018 - 20:44 **Next**: review the diff, then say "commit" to commit the guard change on its own.
