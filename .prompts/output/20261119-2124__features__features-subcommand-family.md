# The features subcommand family

The family is built, tested and documented. Nothing is staged; the work
is in the working tree.

## Files created or changed

- `features/config.go` — new: `Config`, `Load` (walks up to
  `.repo-tools-features.json`), validation that names the file and key.
- `features/git.go` — new: the git helpers, worktree discovery, the
  merge-state file, the env-pair file.
- `features/features.go` — new: the package doc, the bare command and
  its loop text, `refuse`, `reference`, the `would:` helpers.
- `features/resolve.go` — new: number/name resolution on the trunk, the
  free-tree pick, the scenario helpers, the merge tag.
- `features/new.go`, `attach.go`, `scenario.go`, `refresh.go`,
  `merge.go`, `verify.go`, `land.go`, `finish.go` — new: the eight
  subcommands, each with its short reference beside it.
- `features/complete.go` — new: the `Branch` and `AttachArgs`
  completers.
- `features/fixture_test.go`, `features/features_test.go` — new: the
  unit tests on `t.TempDir()` fixture repositories.
- `internal/argcomplete/argcomplete.go` — extended: the `ArgsCompleter`
  interface and longest-path matching, so a second positional argument
  completes; a word that starts with a dash goes to opts.
- `internal/argcomplete/argcomplete_test.go` — the second-positional
  cases.
- `main.go` — the features registration, the five `argCompleters`
  rows, and the package comment and Summary reworded to "a repository".
- `docs/features.md` — new: the page.
- `README.md`, `AGENTS.md`, `docs/AGENTS.md` — the new rows, the
  count, and the consumer-name rewording.

## The configuration

`.repo-tools-features.json` at the host repository's root; see
`docs/features.md` section 1 for the example and the key table. No
path lives in the file: the trees come from
`git worktree list --porcelain`, mapped by directory name (`<x>-alpha`
is the `alpha` tree).

## Design choices

- The second positional completes through a new
  `argcomplete.ArgsCompleter` interface; `Complete` now matches the
  longest registered path and passes the prior arguments, flags
  dropped. A plain `Completer` still owns the first argument only.
- The merge state is the ported `<git common dir>/feature-merge/state`
  file (`branch=`, `merged=`, `verified=`), shared by every worktree.
- The env-pair check of refresh runs only in an implementation tree,
  and reads the pair from the tree's `envFile`, not the environment.
- The check commands (`branchGate`, `verify`) are argv lists, run
  verbatim and in order.

## Checks

`go test ./...` green across every package, `go vet ./...` clean,
`gofmt -l` clean. The loop was smoke-tested end to end on a fixture
with a bare origin, and the completion through `COMP_LINE`.

The closed-source grep is clean in every touched file; the remaining
hits are in untouched files: `docs/md-table-fmt.md`,
`docs/plan-report.md`, `docs/prompt-history.md`, `flake.nix`,
`.claude/skills/go-cli/SKILL.md`, and the generated `.prompts/` files.

20261119 - 21:24 **Next**: write the host's `.repo-tools-features.json`
and try `go run /path/to/tools features attach <tab>` there.
