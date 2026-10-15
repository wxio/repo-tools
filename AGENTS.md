# repo-tools

The small programs the sheafdb repositories run on themselves. One
binary, `repo-tools`, with four subcommands. Two libraries. One
dependency outside the Go standard library: `github.com/jpillora/opts`,
which parses the command line. Add no other.

Nothing here is about sheafdb. A command reads the repository it is given
and names no path above it, so any repository can install the flake and
get the same hooks.

## How to work with me

These rules cover replies, plans, commit messages and code comments.

### Simplified Technical English

Write to the ASD-STE100 writing rules. Use the rules, not the approved
word list: 900 words cannot carry Go or prek, and technical names and
technical verbs are exempt in STE itself.

- One idea per sentence. One instruction per sentence.
- 20 words maximum in an instruction, 25 in a description.
- 6 sentences maximum in a paragraph.
- Active voice. Imperative mood for an instruction.
- Present tense, unless the past is the point.
- One word, one meaning. Repeat a term; do not vary it for style.
- Keep articles and helping verbs. Write "the walk cache", not "walk
  cache".
- 3 words maximum in a noun cluster.
- Use a relative clause where an -ing form is ambiguous.
- Say the thing. Cut the preamble and the hedge.

### Executive function support

Working memory and task switching are the expensive parts. Carry that
load.

- Lead with the answer, or with the next action. Put a blocker in the
  first line, never at the end.
- Give one next action, not a menu. Where there is a choice, choose it,
  then say what you chose and why in one line.
- Before a long task, repeat the request back in one sentence.
- Work in chunks. After each chunk, write one line: what changed, what is
  next.
- When the work grows past what was asked, stop and name it.
- Do not stage or commit until told to. Leave the change in the working
  tree and report that it is ready. "Commit" in the request is the
  signal; "done", "ship" or "looks good" is not.
- Avoid an open question. Propose a default and ask for confirmation.
- Say the size before you start: minutes, or an afternoon.
- End every turn with one line that starts "YYYYMMDD - HH:MM **Next**:".

## Development environment

`flake.nix` pins the toolchain. Enter the shell with `direnv allow`
(`.envrc` is `use flake`) or `nix develop`. A git worktree needs its own
`direnv allow`.

| in the shell | why                               |
|--------------|-----------------------------------|
| `go` (1.27)  | the module                        |
| `prek`       | the git hook runner               |
| `gh`         | the GitHub CLI, for pull requests |

**If `prek` is not found, you are not in the shell.**

## Go style

Read [docs/go-style.md](docs/go-style.md) before writing Go here: locals
in a `var (...)` block at the top of the scope, assigned with `=` inside
`if`; every struct literal multi-line with a trailing comma.

## Markdown style

A page is read in the raw file as often as in a viewer. `gofmt` decides
Go, and nothing decides Markdown, so these rules are written down.

**Align every table.** Pad each cell with spaces to the width of the
widest cell in its column. Fill the separator row with dashes to the
same width. Every row of one table is then the same length.

```markdown
| file       | what it computes                  |
|------------|-----------------------------------|
| `lower.go` | `Lower`: an operation as a script |
| `prim.go`  | `Apply` and `Inverse`             |
```

Three rules follow from it:

- **Keep a cell short.** A cell cannot wrap, so one long cell makes
  every row long. Cut a cell that runs past about 70 characters, or
  split the row.
- **Do not put a pipe in a cell, even as `\|`.** It is two characters in
  the file and one on screen, so no padding lines up in both.
- **Re-align after an edit.** A new row that is wider re-pads the whole
  table.

Do not do this by hand. `md-table-fmt` does it, and a prek hook runs it
over the Markdown files a commit stages:

```sh
go run . md-table-fmt docs/md-table-fmt.md
go run . md-table-fmt -n $(git ls-files '*.md')   # report only
```

This repository is the source of that program, so it runs it with `go
run .`. A repository that takes the flake runs `repo-tools
md-table-fmt`.

This applies to every `.md` here, this file included.

## The layout

| path                       | what it is                                                    |
|----------------------------|---------------------------------------------------------------|
| `main.go`                  | `repo-tools`: the opts command line and the exit codes     |
| `md_table_fmt.go`          | `md-table-fmt`: aligns the tables of a `.md` file             |
| `prompt_history_record.go` | `prompt-history-record`: the Claude Code `Stop` hook          |
| `prompt_history_meta.go`   | `prompt-history-meta`: rebuilds the prompt-history index      |
| `prompt_history_guard.go`  | `prompt-history-guard`: the prek hook over `.prompt-history/` |
| `internal/mdtable`         | the library behind `md-table-fmt`                             |
| `internal/prompthistory`   | the library behind `record` and `meta`                        |
| `docs/`                    | one page per tool; `docs/AGENTS.md` is first                  |

## Commands

```sh
go test ./... && go vet ./...
go run . md-table-fmt -n $(git ls-files '*.md')
go run . prompt-history-meta -root /path/to/repo

nix build .#repo-tools     # the binary, as a consumer gets it
prek install                  # the git hooks, once per clone and worktree
prek run --all-files
```

**Do not run `git add` or `git commit` until told "commit".** Edit, test
and report. On "commit", stage only the files changed for the task, then
commit. One request is one commit; the next commit needs the next
request.

## The rule that keeps this repository separable

A command takes the repository as an argument, an environment variable or
the working directory. It never reads a path above the file it was given,
and it never names a sheafdb directory.

Check it before a release:

```sh
grep -rn '\.\./\.\.' --include='*.go' .     # expect nothing
grep -rln 'sheafdb/' --include='*.go' .     # expect nothing
```

A tool that needs to know about sheafdb belongs in sheafdb.
