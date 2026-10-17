`main.go` now follows the go-cli skill, and each subcommand is in a package of its own. The change is in the working tree and is not staged. Do not run `nix build .#repo-tools` before the commit: the flake sees only tracked files, so it fails on the new packages.

**The packages.** Each package exports `<Name>Cmd` and `New<Name>Cmd()`, and the constructor sets the flag defaults.

| package               | type                     | was                        |
|-----------------------|--------------------------|----------------------------|
| `mdtablefmt`          | `MdTableFmtCmd`          | `md_table_fmt.go`          |
| `prompthistoryrecord` | `PromptHistoryRecordCmd` | `prompt_history_record.go` |
| `prompthistorymeta`   | `PromptHistoryMetaCmd`   | `prompt_history_meta.go`   |
| `prompthistoryguard`  | `PromptHistoryGuardCmd`  | `prompt_history_guard.go`  |
| `internal/exitcode`   | `Code`                   | `exitCode` in `main.go`    |

The guard test moved with its package. The file comments are now package doc comments.

**`main.go`.** It follows the skill: `rootCmd` and `builder` in a `var` block, with `EmbedGlobalFlagSet()` and `Complete()`. `init()` holds one `AddCommand` for each subcommand, with its name and summary. A `Run` error prints the help and the error, then exits 2. I made two changes to the skill pattern:

- **`Parse`, not `ParseArgsError`.** opts returns `--help` and `--install` as errors of unexported types. The skill's `main` would exit 1 on them. `Parse` exits 0.
- **`exitcode.Code` first.** The code goes to prek with no help text. md-table-fmt and the guard depend on 1 and 2.

**Why `internal/exitcode`.** Two packages and `main` share the exit-code type. It cannot stay in `package main`.

| check                                    | result                               |
|------------------------------------------|--------------------------------------|
| `go vet ./...`, `go test ./...`, `gofmt` | pass                                 |
| `nix build "path:$PWD#repo-tools"`       | builds; `vendorHash` does not change |
| `repo-tools --help`                      | exit 0                               |
| `repo-tools` with no subcommand          | help and error, exit 2               |
| `md-table-fmt` with no file              | exit 2                               |
| `prompt-history-meta -root /nonexistent` | help and error, exit 2 (was 1)       |
| `md-table-fmt -n` over every `.md`       | clean                                |
| the separability greps                   | nothing found                        |

**Docs.** `AGENTS.md` has a new layout table and now says "Three libraries". `README.md` says "Three libraries" too. The table re-alignment also fixed the old padding on the `main.go` row.

Two things to know:

- An unknown flag, for example `repo-tools --bogus`, prints the help and exits 0. The old `main.go` did the same, because opts `Parse` does this.
- `.claude/` is untracked and holds the go-cli skill. I did not change it. Its style link, `docs/kb/go-style.md`, points to a page that is not in this repository.

20261017 - 22:29 **Next**: say "commit". That stages the new packages, and then `nix build .#repo-tools` works.
